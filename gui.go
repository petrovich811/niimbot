// Графический интерфейс: встроенный веб-сервер в самом бинарнике.
//
// Тот же подход, что в OCRTAG: ресурсы вшиты через embed, сервер на net/http,
// никаких GUI-фреймворков и внешних зависимостей. Логика печати не
// дублируется — интерфейс вызывает те же функции, что и командная строка.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"image/png"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webAssets embed.FS

// Принтер держит ОДНО BLE-соединение, поэтому все обращения к нему
// выстраиваем в очередь.
var bleMu sync.Mutex

// guiVerbose включает вывод протокола в интерфейсе (флаг -v).
var guiVerbose = true

// --------------------------------------------------------------------------
// Задания печати
// --------------------------------------------------------------------------

type job struct {
	mu      sync.Mutex
	Total   int       `json:"total"`
	Done    int       `json:"done"`
	State   string    `json:"state"` // running | done | error
	Err     string    `json:"error,omitempty"`
	Started time.Time `json:"-"`
}

var (
	jobsMu sync.Mutex
	jobs   = map[string]*job{}
	jobSeq int
)

func newJob(total int) (string, *job) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	jobSeq++
	id := fmt.Sprintf("job-%d", jobSeq)
	j := &job{Total: total, State: "running", Started: time.Now()}
	jobs[id] = j
	return id, j
}

func (j *job) set(done int) {
	j.mu.Lock()
	j.Done = done
	j.mu.Unlock()
}

func (j *job) finish(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err != nil {
		j.State, j.Err = "error", err.Error()
		return
	}
	j.State, j.Done = "done", j.Total
}

func (j *job) snapshot() job {
	j.mu.Lock()
	defer j.mu.Unlock()
	return job{Total: j.Total, Done: j.Done, State: j.State, Err: j.Err}
}

// --------------------------------------------------------------------------
// Шаблоны настроек
// --------------------------------------------------------------------------

// Template — сохранённый набор настроек под типовую задачу.
type Template struct {
	Name    string  `json:"name"`
	Length  float64 `json:"length"`
	Font    float64 `json:"font"`
	Density int     `json:"density"`
	Label   string  `json:"label"`
	Copies   int    `json:"copies"`
	Flip     bool   `json:"flip"`
	Template string `json:"template"`
}

func templatesPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "niimbot", "templates.json")
}

func loadTemplates() []Template {
	data, err := os.ReadFile(templatesPath())
	if err != nil {
		return nil
	}
	var list []Template
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return list
}

func saveTemplates(list []Template) error {
	path := templatesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// --------------------------------------------------------------------------
// Ответы
// --------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// withPrinter подключается к принтеру, подержав очередь, и отдаёт его обработчику.
func withPrinter(address string, fn func(*Printer) error) error {
	bleMu.Lock()
	defer bleMu.Unlock()

	if address == "" {
		found, err := ScanPrinter("N1-", 12*time.Second, guiVerbose)
		if err != nil {
			return err
		}
		address = found
	}
	p, err := ConnectMAC(address, guiVerbose)
	if err != nil {
		return err
	}
	defer p.Close()
	if _, err := p.Handshake(); err != nil {
		return err
	}
	return fn(p)
}

// --------------------------------------------------------------------------
// API
// --------------------------------------------------------------------------

// apiStatus — состояние принтера и расходников.
func apiStatus(address string) (map[string]any, error) {
	var out = map[string]any{}
	err := withPrinter(address, func(p *Printer) error {
		info, err := p.Handshake()
		if err != nil {
			return err
		}
		out["printer"] = info
		for key, ribbon := range map[string]bool{"paper": false, "ribbon": true} {
			rf, err := p.ReadRfid(ribbon)
			if err != nil {
				continue
			}
			out["rfid_"+key] = map[string]any{
				"tag":       rf.TagPresent,
				"serial":    rf.Serial,
				"barcode":   rf.Barcode,
				"labelType": rf.LabelTypeName(),
				"typeID":    rf.LabelType,
				"total":     rf.AllPaper,
				"used":      rf.UsedPaper,
				"left":      rf.Leftover(),
			}
		}
		return nil
	})
	return out, err
}

// printRequest — тело запросов на печать.
type printRequest struct {
	Items    []string `json:"items"`    // строки данных: одна запись на этикетку
	Template string   `json:"template"` // шаблон этикетки с {1}, {2}… (необязательно)
	Length  float64  `json:"length"`
	Font    float64  `json:"font"`
	Density int      `json:"density"`
	Label   string   `json:"label"`
	Copies  int      `json:"copies"`
	Flip    bool     `json:"flip"`
	Address string   `json:"address"`
}

// splitFields разбирает строку данных на столбцы: ; , или табуляция.
func splitFields(s string) []string {
	s = strings.TrimRight(s, "\r")
	sep := ";"
	switch {
	case strings.Contains(s, "\t"):
		sep = "\t"
	case strings.Contains(s, ";"):
		sep = ";"
	case strings.Contains(s, ","):
		sep = ","
	}
	return splitQuoted(s, sep)
}

// splitQuoted делит строку по разделителю, не трогая разделители внутри
// кавычек, и снимает сами кавычки: "Насос; старый";12А-5 → 2 столбца.
func splitQuoted(s, sep string) []string {
	var out []string
	var cur strings.Builder
	inQuotes := false
	runes := []rune(s)
	sepRunes := []rune(sep)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case inQuotes && c == '"':
			// удвоенная кавычка внутри поля — это одна кавычка
			if i+1 < len(runes) && runes[i+1] == '"' {
				cur.WriteRune('"')
				i++
			} else {
				inQuotes = false
			}
		case !inQuotes && c == '"':
			inQuotes = true
		case !inQuotes && len(sepRunes) == 1 && c == sepRunes[0]:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	out = append(out, strings.TrimSpace(cur.String()))
	return out
}

// rePlaceholder находит неподставленные {1}, {2}… в строке этикетки.
var rePlaceholder = regexp.MustCompile(`\{\d+\}`)

// expandTemplate подставляет столбцы строки данных в шаблон этикетки.
//
// Каждая строка шаблона становится строкой этикетки — включая пустые:
// так раскладка одинакова у всех этикеток серии, даже если какое-то поле
// пустое. Пустые строки в конце шаблона отбрасываются.
//
// Если шаблон ссылается на столбец, которого в данных нет, возвращается
// ошибка: иначе на этикетке напечаталось бы литеральное «{3}» — брак,
// который на бумаге уже не исправить.
func expandTemplate(tpl string, row []string) ([]string, error) {
	lines := strings.Split(tpl, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		for i, v := range row {
			line = strings.ReplaceAll(line, fmt.Sprintf("{%d}", i+1), v)
		}
		if bad := rePlaceholder.FindString(line); bad != "" {
			return nil, fmt.Errorf(
				"шаблон использует %s, а в данных только %d столбц(а/ов). "+
					"Проверьте, что в CSV столько же столбцов, сколько в шаблоне", bad, len(row))
		}
		out = append(out, strings.TrimRight(line, " \t"))
	}
	return out, nil
}

// splitLabel превращает текст этикетки в строки: «A|B» → две строки.
func splitLabel(s string) []string {
	s = strings.TrimRight(s, "\r")
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	// если разделителей нет — это одна строка
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}

// renderPages готовит страницы битмапа по списку этикеток.
func renderPages(req printRequest) ([][]Row, error) {
	length := req.Length
	if length <= 0 {
		length = 30
	}
	var pages [][]Row
	for _, item := range req.Items {
		var lines []string
		if strings.TrimSpace(req.Template) != "" {
			var err error
			lines, err = expandTemplate(req.Template, splitFields(item))
			if err != nil {
				return nil, fmt.Errorf("строка данных %q: %w", item, err)
			}
		} else {
			lines = splitLabel(item)
		}
		if len(lines) == 0 {
			continue
		}
		img, err := RenderText(lines, length, req.Font)
		if err != nil {
			return nil, err
		}
		rows := ImageToRows(img, req.Flip)
		pages = append(pages, rows)
	}
	return pages, nil
}

// apiPrint запускает серию в фоне и возвращает идентификатор задания.
func apiPrint(w http.ResponseWriter, req printRequest) {
	pages, err := renderPages(req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if len(pages) == 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("пустой список этикеток"))
		return
	}

	// Тип этикетки проверяем сразу, до похода к принтеру.
	if req.Label != "" {
		if err := checkLabelName(req.Label); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}

	density := req.Density
	if density < 1 || density > 3 {
		density = 2
	}
	copies := req.Copies
	if copies < 1 {
		copies = 1
	}

	id, j := newJob(len(pages) * copies)
	go func() {
		err := withPrinter(req.Address, func(p *Printer) error {
			labelName, err := resolveLabel(p, req.Label)
			if err != nil {
				return err
			}
			lt := labelTypes[labelName]
			return p.PrintPages(pages, byte(density), lt, copies,
				func(done, total int) {
					j.set(done)
					fmt.Printf("  отправлено %d из %d этикеток\n", done, total)
				})
		})
		if err != nil {
			fmt.Printf("серия не напечатана: %v\n", err)
		} else {
			fmt.Printf("серия напечатана: %d этикеток\n", j.Total)
		}
		j.finish(err)
	}()

	writeJSON(w, map[string]any{"job": id, "total": j.Total})
}

// apiPreview отдаёт PNG-макет этикетки.
func apiPreview(w http.ResponseWriter, req printRequest) {
	length := req.Length
	if length <= 0 {
		length = 30
	}
	lines := []string{"пусто"}
	if len(req.Items) > 0 {
		var l []string
		if strings.TrimSpace(req.Template) != "" {
			var err error
			l, err = expandTemplate(req.Template, splitFields(req.Items[0]))
			if err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
		} else {
			l = splitLabel(req.Items[0])
		}
		if len(l) > 0 {
			lines = l
		}
	}
	img, err := RenderText(lines, length, req.Font)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	png.Encode(w, img)
}

// --------------------------------------------------------------------------
// Сервер
// --------------------------------------------------------------------------

// startGUI поднимает веб-интерфейс и открывает браузер.
func startGUI(address string, port int, openBrowserFlag, verbose bool) error {
	guiVerbose = verbose
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		data, err := apiStatus(address)
		if err != nil {
			writeErr(w, http.StatusServiceUnavailable, err)
			return
		}
		writeJSON(w, data)
	})

	mux.HandleFunc("/api/job", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		jobsMu.Lock()
		j, ok := jobs[id]
		jobsMu.Unlock()
		if !ok {
			writeErr(w, http.StatusNotFound, fmt.Errorf("задание не найдено"))
			return
		}
		writeJSON(w, j.snapshot())
	})

	readRequest := func(w http.ResponseWriter, r *http.Request) (printRequest, bool) {
		var req printRequest
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("нужен POST"))
			return req, false
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return req, false
		}
		if address == "" {
			req.Address = ""
		} else {
			req.Address = address // адрес из командной строки важнее
		}
		return req, true
	}

	mux.HandleFunc("/api/preview", func(w http.ResponseWriter, r *http.Request) {
		if req, ok := readRequest(w, r); ok {
			apiPreview(w, req)
		}
	})
	mux.HandleFunc("/api/print", func(w http.ResponseWriter, r *http.Request) {
		if req, ok := readRequest(w, r); ok {
			apiPrint(w, req)
		}
	})

	mux.HandleFunc("/api/templates", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, loadTemplates())
		case http.MethodPost:
			var t Template
			if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
			if strings.TrimSpace(t.Name) == "" {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("нужно имя шаблона"))
				return
			}
			list := loadTemplates()
			replaced := false
			for i := range list {
				if list[i].Name == t.Name {
					list[i], replaced = t, true
					break
				}
			}
			if !replaced {
				list = append(list, t)
			}
			if err := saveTemplates(list); err != nil {
				writeErr(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, list)
		case http.MethodDelete:
			name := r.URL.Query().Get("name")
			list := loadTemplates()
			out := list[:0]
			for _, t := range list {
				if t.Name != name {
					out = append(out, t)
				}
			}
			if err := saveTemplates(out); err != nil {
				writeErr(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, out)
		default:
			writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("метод не поддерживается"))
		}
	})

	// Статика из embed
	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := "http://" + addr
	fmt.Printf("Интерфейс: %s\n", url)
	fmt.Println("Остановить — Ctrl+C")

	if openBrowserFlag {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}
	return http.ListenAndServe(addr, mux)
}

// openBrowser открывает браузер (как в OCRTAG).
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	_ = exec.Command(cmd, args...).Start()
}
