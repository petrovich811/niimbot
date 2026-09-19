package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// Пакет рукопожатия, снятый с живого принтера N1:
//
//	→ 0xc1 → 5555 c1 01 01 c1 aaaa
//	← 0xc2 → 5555 c2 01 02 c1 aaaa
func TestBuildPacketMatchesLivePrinter(t *testing.T) {
	got := buildPacket(cmdConnect, []byte{1})
	want := []byte{0x55, 0x55, 0xC1, 0x01, 0x01, 0xC1, 0xAA, 0xAA}
	if !bytes.Equal(got, want) {
		t.Fatalf("пакет Connect = %x, ожидался %x", got, want)
	}

	// Запрос модели: 555540010849aaaa (тоже снято с принтера)
	got = buildPacket(cmdPrinterInfo, []byte{infoModelID})
	want = []byte{0x55, 0x55, 0x40, 0x01, 0x08, 0x49, 0xAA, 0xAA}
	if !bytes.Equal(got, want) {
		t.Fatalf("пакет PrinterInfo = %x, ожидался %x", got, want)
	}
}

// Ответ принтера с моделью N1 (0x0e02 = 3586) должен разбираться.
func TestParsePacketsModelResponse(t *testing.T) {
	buf := []byte{0x55, 0x55, 0x48, 0x02, 0x0E, 0x02, 0x46, 0xAA, 0xAA}
	pkts := parsePackets(&buf)
	if len(pkts) != 1 {
		t.Fatalf("разобрано пакетов: %d, ожидался 1", len(pkts))
	}
	if pkts[0].Cmd != 0x48 {
		t.Fatalf("команда = 0x%02x, ожидалась 0x48", pkts[0].Cmd)
	}
	model := int(pkts[0].Data[0])<<8 | int(pkts[0].Data[1])
	if model != modelIDN1 {
		t.Fatalf("id модели = %d, ожидался %d (N1)", model, modelIDN1)
	}
	if len(buf) != 0 {
		t.Fatalf("в буфере остался мусор: %x", buf)
	}
}

// Разорванный пакет не должен теряться: хвост остаётся в буфере до прихода
// остатка.
func TestParsePacketsPartial(t *testing.T) {
	part1 := []byte{0x55, 0x55, 0x48, 0x02, 0x0E}
	buf := append([]byte{}, part1...)
	if pkts := parsePackets(&buf); len(pkts) != 0 {
		t.Fatalf("из неполного пакета разобрано %d штук", len(pkts))
	}
	if !bytes.Equal(buf, part1) {
		t.Fatalf("хвост сохранён неверно: %x", buf)
	}
	buf = append(buf, 0x02, 0x46, 0xAA, 0xAA)
	pkts := parsePackets(&buf)
	if len(pkts) != 1 {
		t.Fatalf("после достройки разобрано %d пакетов", len(pkts))
	}
}

// Разбор статуса печати: page(2) + печать + подача.
func TestPrintStatusParsing(t *testing.T) {
	// 00016464 = страница 1, печать 100%, подача 100%
	data := []byte{0x00, 0x01, 0x64, 0x64}
	page := int(data[0])<<8 | int(data[1])
	if page != 1 || data[2] != 100 || data[3] != 100 {
		t.Fatalf("разбор статуса неверен: page=%d print=%d feed=%d", page, data[2], data[3])
	}
}

// Строка битмапа должна быть ровно 12 байт (96 точек), а счётчики —
// разложены по трём частям, как в оригинальном протоколе.
func TestCountPixels(t *testing.T) {
	row := make(Row, 12)
	row[0] = 0b1000_0001 // 2 точки в первой части
	row[5] = 0b0000_0011 // 2 точки во второй части
	total, parts := countPixels(row, printheadPixels)
	if total != 4 {
		t.Fatalf("всего точек = %d, ожидалось 4", total)
	}
	if parts[0] != 2 || parts[1] != 2 || parts[2] != 0 {
		t.Fatalf("счётчики по частям = %v, ожидалось [2 2 0]", parts)
	}
}

// Ориентация. Изображение в ориентации чтения (ширина = подача,
// высота = головка) транспонируется в строки принтера, причём ось головки
// идёт в обратном порядке — иначе этикетка выходит зеркальной
// (проверено печатью на живом принтере).
func TestImageToRowsOrientation(t *testing.T) {
	img := NewLabelImage(2) // 2 мм ≈ 16 строк подачи
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() != printheadPixels {
		t.Fatalf("размер изображения %v, ожидалась высота %d", img.Bounds(), printheadPixels)
	}
	// белый фон обязателен: чёрный холст ушёл бы на печать сплошным листом
	if img.GrayAt(0, 0).Y != 255 {
		t.Fatalf("фон не белый: %d", img.GrayAt(0, 0).Y)
	}
	if len(ImageToRows(img, false)) != img.Bounds().Dx() {
		t.Fatalf("строк %d, ожидалось %d (по длине этикетки)",
			len(ImageToRows(img, false)), img.Bounds().Dx())
	}

	// Точка в начале координат изображения (x=0 — начало подачи,
	// y=0 — один край головки) должна попасть в первую строку и в последнюю
	// точку головки (c = 95), то есть в младший бит последнего байта.
	img.SetGray(0, 0, color.Gray{Y: 0})
	rows := ImageToRows(img, false)
	if rows[0] == nil {
		t.Fatal("первая строка пустая, хотя в ней есть точка")
	}
	last := printheadPixels/8 - 1
	if rows[0][last] != 0x01 {
		t.Fatalf("последний байт первой строки = 0x%02x, ожидался 0x01 "+
			"(точка головки 95 = младший бит)", rows[0][last])
	}
	for i, b := range rows[0] {
		if i != last && b != 0 {
			t.Fatalf("байт %d = 0x%02x, ожидался 0 — точка должна быть одна", i, b)
		}
	}

	// Обратная проверка: точка у противоположного края головки (y = 95)
	// должна оказаться в первом байте, в старшем бите.
	img2 := NewLabelImage(2)
	img2.SetGray(0, printheadPixels-1, color.Gray{Y: 0})
	rows2 := ImageToRows(img2, false)
	if rows2[0] == nil || rows2[0][0] != 0x80 {
		t.Fatalf("край головки: первый байт = 0x%02x, ожидался 0x80",
			func() byte {
				if rows2[0] == nil {
					return 0
				}
				return rows2[0][0]
			}())
	}
}

// Переворот меняет порядок строк.
func TestImageToRowsFlip(t *testing.T) {
	img := NewLabelImage(2)
	img.SetGray(0, 0, color.Gray{Y: 0})
	plain := ImageToRows(img, false)
	flipped := ImageToRows(img, true)
	if plain[0] == nil {
		t.Fatal("без переворота первая строка должна быть с точкой")
	}
	if flipped[len(flipped)-1] == nil {
		t.Fatal("с переворотом точка должна уехать в последнюю строку")
	}
}

// Замер скорости не нужен, но проверяем, что печать строк не блокируется
// на пустых строках (они уходят командой 0x84).
func TestRowTypeForEmptyLine(t *testing.T) {
	img := NewLabelImage(5)
	rows := ImageToRows(img, false)
	for i, r := range rows {
		if r != nil {
			t.Fatalf("строка %d не пустая на белом листе", i)
		}
	}
}

// N1 поддерживает только пять типов этикеток из восьми, известных протоколу.
// Остальные должны отсекаться до отправки на принтер.
func TestLabelTypesSupportedByN1(t *testing.T) {
	supported := []string{"withgaps", "continuous", "transparent", "blackmarkgap", "heatshrink"}
	for _, name := range supported {
		id, ok := labelTypes[name]
		if !ok {
			t.Fatalf("тип %q отсутствует в таблице протокола", name)
		}
		if _, ok := labelTypesN1[id]; !ok {
			t.Fatalf("N1 должен поддерживать %q (id %d)", name, id)
		}
	}
	// Эти три протокол знает, но N1 их не поддерживает.
	for _, name := range []string{"black", "perforated", "pvctag"} {
		id, ok := labelTypes[name]
		if !ok {
			t.Fatalf("тип %q отсутствует в таблице протокола", name)
		}
		if _, ok := labelTypesN1[id]; ok {
			t.Fatalf("N1 не должен заявлять тип %q (id %d)", name, id)
		}
	}
	// Коды типов должны совпадать с описанием протокола.
	want := map[string]byte{
		"withgaps": 1, "black": 2, "continuous": 3, "perforated": 4,
		"transparent": 5, "pvctag": 6, "blackmarkgap": 10, "heatshrink": 11,
	}
	for name, id := range want {
		if labelTypes[name] != id {
			t.Fatalf("%s: код %d, ожидался %d", name, labelTypes[name], id)
		}
	}
	if n := len(labelTypesN1); n != 5 {
		t.Fatalf("у N1 должно быть 5 типов, а не %d", n)
	}
}

// Разбор RFID-метки на реальных байтах, снятых с живого N1.
func TestParseRfidInfoRealData(t *testing.T) {
	// Рулон этикеток: ответ 0x1b
	paper := []byte{
		0x88, 0x1d, 0xcc, 0xe9, 0x46, 0x12, 0x10, 0x80, // uuid
		0x08, '1', '2', '2', '4', '2', '1', '1', '7', // штрихкод
		0x10, 'P', 'C', '0', 'H', '9', '0', '2', '3', '8', '4', '0', '0', '2', '3', '7', '8', // серийник
		0x00, 0xe4, // всего 228
		0x00, 0x2b, // израсходовано 43
		0x01, // тип: withgaps
	}
	info := parseRfidInfo(paper)
	if !info.TagPresent {
		t.Fatal("метка рулона не распознана")
	}
	if info.UUID != "881dcce946121080" {
		t.Fatalf("uuid = %q", info.UUID)
	}
	if info.Barcode != "12242117" {
		t.Fatalf("штрихкод = %q", info.Barcode)
	}
	if info.Serial != "PC0H902384002378" {
		t.Fatalf("серийный номер = %q", info.Serial)
	}
	if info.AllPaper != 228 || info.UsedPaper != 43 || info.Leftover() != 185 {
		t.Fatalf("ресурс: всего %d, израсходовано %d, осталось %d",
			info.AllPaper, info.UsedPaper, info.Leftover())
	}
	if info.LabelType != 1 || info.LabelTypeName() != "withgaps" {
		t.Fatalf("тип этикетки = %d (%q)", info.LabelType, info.LabelTypeName())
	}
	// Тип из метки обязан быть из числа поддерживаемых N1.
	if _, ok := labelTypesN1[info.LabelType]; !ok {
		t.Fatal("тип из метки не поддерживается N1")
	}
}

// Лента: ответ 0x1d. Поле типа здесь — тип расходника, а не этикетки,
// поэтому в набор типов N1 оно попадать не обязано.
func TestParseRfidInfoRibbon(t *testing.T) {
	ribbon := []byte{
		0x88, 0x1d, 0x82, 0x09, 0xfa, 0x90, 0x00, 0x00,
		0x0d, '6', '9', '7', '2', '8', '4', '2', '7', '4', '7', '5', '2', '5',
		0x10, 'P', 'Z', '1', 'G', 'A', '0', '6', '3', '0', '4', '0', '0', '0', '3', '9', '1',
		0x06, 0x40,
		0x03, 0x85,
		0x06,
		0x40, // неполная ёмкость — читаться не должна
	}
	info := parseRfidInfo(ribbon)
	if !info.TagPresent {
		t.Fatal("метка ленты не распознана")
	}
	if info.Serial != "PZ1GA06304000391" {
		t.Fatalf("серийный номер ленты = %q", info.Serial)
	}
	if info.AllPaper != 1600 || info.UsedPaper != 901 || info.Leftover() != 699 {
		t.Fatalf("ресурс ленты: всего %d, израсходовано %d, осталось %d",
			info.AllPaper, info.UsedPaper, info.Leftover())
	}
	if info.HasCapacity {
		t.Fatal("ёмкость не должна читаться: в ответе для неё только один байт")
	}
}

// Пустой ответ означает, что метки нет.
func TestParseRfidInfoNoTag(t *testing.T) {
	for _, data := range [][]byte{{}, {0x01}} {
		info := parseRfidInfo(data)
		if info.TagPresent {
			t.Fatalf("данные %x: метка не должна считаться присутствующей", data)
		}
	}
}

// Шаблон этикетки: столбцы строки данных подставляются в {1}, {2}…
// и каждая строка шаблона становится строкой этикетки.
func TestExpandTemplate(t *testing.T) {
	row := splitFields("Насос центробежный;12А-5;14.09.2026")
	if len(row) != 3 || row[0] != "Насос центробежный" || row[1] != "12А-5" || row[2] != "14.09.2026" {
		t.Fatalf("разбор столбцов: %q", row)
	}

	lines, err := expandTemplate("{1}\n{2}\n{3}", row)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	want := []string{"Насос центробежный", "12А-5", "14.09.2026"}
	if len(lines) != len(want) {
		t.Fatalf("строк %d, ожидалось %d: %q", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("строка %d = %q, ожидалась %q", i, lines[i], want[i])
		}
	}

	// Столбец можно использовать несколько раз и вместе с обычным текстом.
	lines, _ = expandTemplate("ТЕГ {2}\n{1}", row)
	if len(lines) != 2 || lines[0] != "ТЕГ 12А-5" || lines[1] != "Насос центробежный" {
		t.Fatalf("подстановка с текстом: %q", lines)
	}

	// Пустая строка в середине шаблона сохраняется: раскладка должна быть
	// одинаковой у всех этикеток серии.
	lines, _ = expandTemplate("{1}\n\n{3}", row)
	if len(lines) != 3 || lines[1] != "" {
		t.Fatalf("пустая строка шаблона потеряна: %q", lines)
	}

	// Пустое значение столбца оставляет строку пустой, а не съедает её.
	lines, _ = expandTemplate("{1}\n{2}\n{3}", []string{"Насос", "", "14.09.2026"})
	if len(lines) != 3 || lines[1] != "" {
		t.Fatalf("пустое значение сдвинуло раскладку: %q", lines)
	}

	// Пустые строки в конце шаблона отбрасываются.
	lines, _ = expandTemplate("{1}\n\n\n", row)
	if len(lines) != 1 {
		t.Fatalf("хвостовые пустые строки не убраны: %q", lines)
	}
}

// Ссылка на столбец, которого нет в данных, — ошибка, а не литерал «{3}»
// на этикетке. Проверено: раньше печаталось именно «{3}».
func TestExpandTemplateMissingColumn(t *testing.T) {
	_, err := expandTemplate("{1}\n{2}\n{3}", []string{"Насос", "12А-5"})
	if err == nil {
		t.Fatal("ожидалась ошибка про недостающий столбец")
	}
	if !strings.Contains(err.Error(), "{3}") {
		t.Fatalf("в ошибке нет имени столбца: %v", err)
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("в ошибке не сказано, сколько столбцов пришло: %v", err)
	}
}

// Столбцы разбираются по ; , или табуляции — что встретится первым.
func TestSplitFieldsSeparators(t *testing.T) {
	cases := map[string][]string{
		"a;b;c":     {"a", "b", "c"},
		"a,b,c":     {"a", "b", "c"},
		"a\tb\tc":   {"a", "b", "c"},
		`"a";"b"`:   {"a", "b"},
		"один":      {"один"},
		"a; b ; c ": {"a", "b", "c"},
	}
	for in, want := range cases {
		got := splitFields(in)
		if len(got) != len(want) {
			t.Fatalf("%q → %q, ожидалось %q", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q → %q, ожидалось %q", in, got, want)
			}
		}
	}
}

// Кавычки защищают разделитель внутри поля — это нужно для CSV из Excel.
func TestSplitFieldsQuotes(t *testing.T) {
	cases := map[string][]string{
		`"Насос; старый";12А-5`:    {"Насос; старый", "12А-5"},
		`"Он сказал ""да""";12Б-1`: {`Он сказал "да"`, "12Б-1"},
		`"a,b";c`:                  {"a,b", "c"},
		`простой;случай`:           {"простой", "случай"},
		`"весь в кавычках"`:        {"весь в кавычках"},
	}
	for in, want := range cases {
		got := splitFields(in)
		if len(got) != len(want) {
			t.Fatalf("%s → %q, ожидалось %q", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s → %q, ожидалось %q", in, got, want)
			}
		}
	}
}

// Разбор адреса принтера. На Linux и Windows это MAC; на macOS — UUID,
// и там работает своя реализация (addr_darwin.go).
func TestMakeAddress(t *testing.T) {
	addr, err := makeAddress("24:0A:11:D9:EC:C3")
	if err != nil {
		t.Fatalf("адрес не разобран: %v", err)
	}
	if got := strings.ToUpper(addr.String()); got != "24:0A:11:D9:EC:C3" {
		t.Fatalf("адрес разобран как %q", got)
	}

	// Мусор должен давать ошибку, а не молча пустой адрес.
	for _, bad := range []string{"", "не-адрес", "12345"} {
		if _, err := makeAddress(bad); err == nil {
			t.Fatalf("для %q ожидалась ошибка", bad)
		}
	}
}

// Вписывание картинки в область этикетки: любая картинка должна доходить
// до принтера нужного размера, иначе головка получает мусор.
func TestFitToLabel(t *testing.T) {
	const (
		wantW = 240 // 30 мм при 203 dpi
		wantH = printheadPixels
	)

	// Уже точный размер — не трогаем.
	exact := image.NewGray(image.Rect(0, 0, wantW, wantH))
	got := FitToLabel(exact, 30)
	if got.Bounds().Dx() != wantW || got.Bounds().Dy() != wantH {
		t.Fatalf("точный размер изменён: %v", got.Bounds())
	}

	cases := []struct {
		name string
		w, h int
	}{
		{"большая квадратная", 1000, 1000},
		{"широкая", 1000, 50},
		{"высокая", 50, 1000},
		{"крошечная", 10, 10},
	}
	for _, c := range cases {
		src := image.NewGray(image.Rect(0, 0, c.w, c.h))
		out := FitToLabel(src, 30)
		if out.Bounds().Dx() != wantW || out.Bounds().Dy() != wantH {
			t.Fatalf("%s: получилось %v, ожидалось %dx%d",
				c.name, out.Bounds(), wantW, wantH)
		}
	}

	// Пропорции: широкая картинка должна занять всю ширину и стать низкой.
	wide := image.NewGray(image.Rect(0, 0, 1000, 100))
	out := FitToLabel(wide, 30)
	if !isWhiteRow(out, 0) == false && !hasContent(out) {
		t.Fatal("широкая картинка исчезла")
	}

	// Длина этикетки меняет ширину результата, а высота всегда 96.
	for _, mm := range []float64{20, 30, 50} {
		out := FitToLabel(wide, mm)
		if out.Bounds().Dy() != wantH {
			t.Fatalf("длина %v мм: высота %d, ожидалось %d", mm, out.Bounds().Dy(), wantH)
		}
		if want := int(math.Round(mm * dotsPerMM)); out.Bounds().Dx() != want {
			t.Fatalf("длина %v мм: ширина %d, ожидалось %d", mm, out.Bounds().Dx(), want)
		}
	}

	// Нулевая длина не должна ломать расчёт.
	if out := FitToLabel(wide, 0); out.Bounds().Dx() <= 0 {
		t.Fatal("нулевая длина дала пустое полотно")
	}
}

// Фон вокруг вписанной картинки должен быть белым, иначе на этикетке
// появится чёрный прямоугольник.
func TestFitToLabelWhiteBackground(t *testing.T) {
	small := image.NewGray(image.Rect(0, 0, 10, 10))
	out := FitToLabel(small, 30)
	if !isWhitePixel(out, 0, 0) {
		t.Fatal("левый верхний угол не белый")
	}
	if !isWhitePixel(out, out.Bounds().Dx()-1, 0) {
		t.Fatal("правый верхний угол не белый")
	}
}

func isWhitePixel(g *image.Gray, x, y int) bool {
	return g.GrayAt(x, y).Y > 200
}

func isWhiteRow(g *image.Gray, y int) bool {
	for x := 0; x < g.Bounds().Dx(); x++ {
		if !isWhitePixel(g, x, y) {
			return false
		}
	}
	return true
}

func hasContent(g *image.Gray) bool {
	for y := 0; y < g.Bounds().Dy(); y++ {
		for x := 0; x < g.Bounds().Dx(); x++ {
			if g.GrayAt(x, y).Y < 128 {
				return true
			}
		}
	}
	return false
}

// Разбор аргументов: флаги со значением должны забирать следующий аргумент.
//
// Раньше список таких флагов вёлся руками, и новые флаги в него забывали
// добавить: «gui --port 9000» разбирался как булев флаг, а 9000 попадал
// в позиционные аргументы — команда падала с «flag needs an argument».
func TestSplitArgsFlagValues(t *testing.T) {
	flags, positional := splitArgs([]string{"preview", "--file", "/tmp/x.png", "--length", "40"})
	wantFlags := []string{"-file", "/tmp/x.png", "-length", "40"}
	if strings.Join(flags, " ") != strings.Join(wantFlags, " ") {
		t.Fatalf("флаги: %q, ожидалось %q", flags, wantFlags)
	}
	if len(positional) != 1 || positional[0] != "preview" {
		t.Fatalf("позиционные: %q, ожидался [preview]", positional)
	}

	// Булев флаг значение не забирает: следующий аргумент — позиционный.
	flags, positional = splitArgs([]string{"gui", "--no-browser", "лишнее"})
	if len(flags) != 1 || flags[0] != "-no-browser" {
		t.Fatalf("булев флаг забрал значение: %q", flags)
	}
	if len(positional) != 2 {
		t.Fatalf("позиционные: %q", positional)
	}

	// Значение через знак равенства обрабатывается отдельно.
	flags, _ = splitArgs([]string{"--length=40"})
	if len(flags) != 1 || flags[0] != "-length=40" {
		t.Fatalf("значение через = разобрано как %q", flags)
	}
}

// Каждый флаг со значением обязан быть известен разбору аргументов.
// Если кто-то добавит флаг и забудет про разбор — тест это поймает.
func TestFlagNeedsValueMatchesRegistration(t *testing.T) {
	valueFlags := []string{"address", "length", "font", "density", "label", "copies", "file", "port"}
	for _, name := range valueFlags {
		if !flagNeedsValue(name) {
			t.Errorf("флаг %q принимает значение, но разбор считает иначе", name)
		}
	}
	boolFlags := []string{"v", "flip", "no-browser"}
	for _, name := range boolFlags {
		if flagNeedsValue(name) {
			t.Errorf("флаг %q булев, но разбор ждёт значение", name)
		}
	}
	// Неизвестный флаг значением не считается — иначе он съел бы команду.
	if flagNeedsValue("такого-флага-нет") {
		t.Error("неизвестный флаг не должен забирать значение")
	}
}

// --- конструктор этикеток ---

// Границы чернил по горизонтали: где начинается и кончается краска.
func inkColumns(img *image.Gray) (int, int) {
	minX, maxX := -1, -1
	b := img.Bounds()
	for x := 0; x < b.Dx(); x++ {
		for y := 0; y < b.Dy(); y++ {
			if img.GrayAt(x, y).Y < 128 {
				if minX < 0 {
					minX = x
				}
				maxX = x
				break
			}
		}
	}
	return minX, maxX
}

func inkRows(img *image.Gray) (int, int) {
	minY, maxY := -1, -1
	b := img.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if img.GrayAt(x, y).Y < 128 {
				if minY < 0 {
					minY = y
				}
				maxY = y
				break
			}
		}
	}
	return minY, maxY
}

// Размер этикетки из шаблона: длина задаётся, высота всегда 96 точек.
func TestRenderTemplateSize(t *testing.T) {
	for _, mm := range []float64{20, 30, 50} {
		img, err := RenderTemplate(LabelTemplate{LengthMM: mm}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dy() != printheadPixels {
			t.Fatalf("%v мм: высота %d", mm, img.Bounds().Dy())
		}
		// Тот же пересчёт, что и в NewLabelImage: миллиметры в точки
		// с округлением до ближайшей.
		if want := int(mm*dotsPerMM + 0.5); img.Bounds().Dx() != want {
			t.Fatalf("%v мм: ширина %d, ожидалось %d", mm, img.Bounds().Dx(), want)
		}
	}
}

// Координаты элемента: надпись ставится туда, куда её положили.
func TestRenderTemplateTextPosition(t *testing.T) {
	// 1 мм = 8 точек. Элемент в левом верхнем углу.
	left, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "text", X: 1, Y: 1, Text: "ЛОТ", FontMM: 3}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	minX, _ := inkColumns(left)
	if minX < 6 || minX > 14 {
		t.Fatalf("надпись начинается на %d точке, ожидалось около 8 (1 мм)", minX)
	}
	minY, _ := inkRows(left)
	if minY < 3 || minY > 14 {
		t.Fatalf("надпись сверху на %d точке, ожидалось около 8 (1 мм)", minY)
	}

	// Тот же элемент, сдвинутый вправо на 20 мм, должен оказаться справа.
	right, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "text", X: 20, Y: 1, Text: "ЛОТ", FontMM: 3}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	minXr, _ := inkColumns(right)
	if minXr < 155 {
		t.Fatalf("сдвинутая надпись начинается на %d точке, ожидалось около 160 (20 мм)", minXr)
	}
}

// Выравнивание внутри рамки элемента.
func TestRenderTemplateAlign(t *testing.T) {
	render := func(align string) (int, int) {
		img, err := RenderTemplate(LabelTemplate{
			LengthMM: 30,
			Elements: []Element{{
				Kind: "text", X: 0, Y: 1, W: 30, Text: "ЦЕНТР", FontMM: 3, Align: align,
			}},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return inkColumns(img)
	}

	lMin, lMax := render("left")
	cMin, cMax := render("center")
	rMin, rMax := render("right")

	if !(lMin < cMin && cMin < rMin) {
		t.Fatalf("порядок выравнивания нарушен: left %d, center %d, right %d", lMin, cMin, rMin)
	}
	// Все три варианта — одна и та же надпись, ширина должна совпасть.
	if lMax-lMin != cMax-cMin || cMax-cMin != rMax-rMin {
		t.Fatalf("ширина надписи зависит от выравнивания: %d, %d, %d",
			lMax-lMin, cMax-cMin, rMax-rMin)
	}
	// Прижатая вправо должна упираться в правый край рамки.
	if rMax < 230 {
		t.Fatalf("выравнивание вправо не дошло до края: %d", rMax)
	}
}

// Подстановка столбцов и ошибка на недостающий столбец.
func TestRenderTemplateFields(t *testing.T) {
	img, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "text", X: 1, Y: 1, Text: "{1}", FontMM: 3}},
	}, []string{"УТРЕХТ"})
	if err != nil {
		t.Fatal(err)
	}
	if minX, _ := inkColumns(img); minX < 0 {
		t.Fatal("подставленное поле не напечаталось")
	}

	_, err = RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "text", X: 1, Y: 1, Text: "{3}", FontMM: 3}},
	}, []string{"один", "два"})
	if err == nil {
		t.Fatal("ожидалась ошибка про недостающий столбец")
	}
	if !strings.Contains(err.Error(), "{3}") {
		t.Fatalf("в ошибке нет имени столбца: %v", err)
	}
}

// Пустая подстановка не должна ничего рисовать и не должна падать.
func TestRenderTemplateEmptyField(t *testing.T) {
	img, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "text", X: 1, Y: 1, Text: "{1}", FontMM: 3}},
	}, []string{"   "})
	if err != nil {
		t.Fatal(err)
	}
	if minX, _ := inkColumns(img); minX >= 0 {
		t.Fatal("пустое поле что-то напечатало")
	}
}

// Картинка из data:URL встаёт в свою рамку.
func TestRenderTemplateImage(t *testing.T) {
	// Чёрный квадрат 10×10 в base64 (PNG).
	var buf bytes.Buffer
	sq := image.NewGray(image.Rect(0, 0, 10, 10))
	draw.Draw(sq, sq.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	if err := png.Encode(&buf, sq); err != nil {
		t.Fatal(err)
	}
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	img, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "image", X: 2, Y: 2, W: 8, H: 8, Image: url}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	minX, maxX := inkColumns(img)
	minY, maxY := inkRows(img)
	// 2 мм = 16 точек, 8 мм = 64 точки.
	if minX < 14 || minX > 18 {
		t.Fatalf("картинка начинается на %d, ожидалось около 16 (2 мм)", minX)
	}
	if maxX < 76 || maxX > 80 {
		t.Fatalf("картинка кончается на %d, ожидалось около 79 (2+8 мм)", maxX)
	}
	if minY < 14 || maxY > 80 {
		t.Fatalf("картинка по высоте: %d..%d, ожидалось около 16..79", minY, maxY)
	}
}

// Ошибки в описании элементов должны быть понятными, а не молчаливыми.
func TestRenderTemplateBadElements(t *testing.T) {
	cases := []LabelTemplate{
		{LengthMM: 30, Elements: []Element{{Kind: "рамочка", X: 1, Y: 1}}},
		{LengthMM: 30, Elements: []Element{{X: 1, Y: 1}}},
		{LengthMM: 30, Elements: []Element{{Kind: "image", X: 1, Y: 1, W: 5, H: 5}}},
		{LengthMM: 30, Elements: []Element{{Kind: "image", X: 1, Y: 1, Image: "data:image/png;base64,zzz", W: 5, H: 5}}},
	}
	for i, tpl := range cases {
		if _, err := RenderTemplate(tpl, nil); err == nil {
			t.Fatalf("случай %d: ожидалась ошибка", i+1)
		}
	}
}

// --- шрифты ---

// Системные шрифты должны читаться, и кириллица в них распознаваться.
func TestSystemFonts(t *testing.T) {
	list, err := SystemFonts()
	if err != nil {
		t.Fatalf("список шрифтов: %v", err)
	}
	if len(list) == 0 {
		t.Skip("в системе не нашлось шрифтов — проверять нечего")
	}
	var withCyr, withPath int
	for _, f := range list {
		if f.Family == "" {
			t.Fatalf("шрифт без названия семейства: %+v", f)
		}
		if f.Path == "" {
			t.Fatalf("шрифт без пути: %+v", f)
		}
		withPath++
		if f.Cyrillic {
			withCyr++
		}
	}
	if withPath != len(list) {
		t.Fatalf("у части шрифтов нет пути")
	}
	// В списке для выбора кириллица обязательна: на этикетках русский текст.
	for _, fam := range fontFamiliesForPicker() {
		found := false
		for _, f := range list {
			if f.Family == fam && f.Cyrillic {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("в списке для выбора семейство без кириллицы: %q", fam)
		}
	}
}

// Выбор шрифта: найденное семейство даёт путь, несуществующее — нет.
func TestResolveFontFamily(t *testing.T) {
	list, err := SystemFonts()
	if err != nil || len(list) == 0 {
		t.Skip("шрифтов нет — проверять нечего")
	}

	var family string
	for _, f := range list {
		if f.Cyrillic && !f.Italic {
			family = f.Family
			break
		}
	}
	if family == "" {
		t.Skip("семейства с кириллицей не нашлось")
	}

	path, ok := resolveFontFamily(family, false)
	if !ok || path == "" {
		t.Fatalf("семейство %q не разрешилось в файл", family)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("файл шрифта %q недоступен: %v", path, err)
	}

	// Регистр не должен иметь значения.
	if _, ok := resolveFontFamily(strings.ToUpper(family), false); !ok {
		t.Fatalf("семейство %q не найдено в другом регистре", family)
	}

	if _, ok := resolveFontFamily("Такого Шрифта Точно Нет 12345", false); ok {
		t.Fatal("несуществующее семейство разрешилось в файл")
	}
}

// Элемент с чужим шрифтом — понятная ошибка, а не молчаливая подмена.
func TestRenderTemplateMissingFont(t *testing.T) {
	_, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{
			Kind: "text", X: 1, Y: 1, Text: "текст", FontMM: 3,
			Family: "Такого Шрифта Точно Нет 12345",
		}},
	}, nil)
	if err == nil {
		t.Fatal("ожидалась ошибка про отсутствующий шрифт")
	}
	if !strings.Contains(err.Error(), "не найден") {
		t.Fatalf("непонятная ошибка: %v", err)
	}
}

// Высота строки раздвигает строки многострочной надписи.
func TestRenderTemplateLineHeight(t *testing.T) {
	rowsSpan := func(lineMM float64) int {
		img, err := RenderTemplate(LabelTemplate{
			LengthMM: 30,
			Elements: []Element{{
				Kind: "text", X: 1, Y: 0.5, Text: "раз\nдва", FontMM: 2, LineMM: lineMM,
			}},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, maxY := inkRows(img)
		return maxY
	}

	dense := rowsSpan(0)
	sparse := rowsSpan(4.5)
	if sparse <= dense {
		t.Fatalf("высота строки не подействовала: по шрифту до %d, с шагом 4,5 мм до %d",
			dense, sparse)
	}
}

// --- линии, рамки, поворот ---

// Линия — залитая полоса: краска только внутри её прямоугольника.
func TestRenderTemplateLine(t *testing.T) {
	img, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "line", X: 2, Y: 5, W: 20, H: 0.5}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	minX, maxX := inkColumns(img)
	minY, maxY := inkRows(img)
	// 2 мм = 16 точек, 22 мм = 176; 5 мм = 40, 5,5 мм = 44.
	if minX < 15 || minX > 17 {
		t.Fatalf("линия начинается на %d, ожидалось 16", minX)
	}
	if maxX < 174 || maxX > 178 {
		t.Fatalf("линия кончается на %d, ожидалось около 176", maxX)
	}
	if minY < 39 || maxY > 45 {
		t.Fatalf("линия по высоте %d..%d, ожидалось 40..44", minY, maxY)
	}
}

// Рамка: краска по четырём сторонам, а середина пустая.
func TestRenderTemplateFrame(t *testing.T) {
	img, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "frame", X: 1, Y: 1, W: 28, H: 10, Thickness: 0.4}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Середина этикетки должна быть чистой.
	cx, cy := img.Bounds().Dx()/2, img.Bounds().Dy()/2
	if !isWhitePixel(img, cx, cy) {
		t.Fatal("внутри рамки есть краска")
	}
	// Стороны — на месте: верх, низ, лево, право.
	if !hasInkAt(img, cx, 8) {
		t.Fatal("верхняя сторона рамки не найдена")
	}
	if !hasInkAt(img, cx, img.Bounds().Dy()-9) {
		t.Fatal("нижняя сторона рамки не найдена")
	}
	if !hasInkAt(img, 8, cy) {
		t.Fatal("левая сторона рамки не найдена")
	}
	if !hasInkAt(img, img.Bounds().Dx()-9, cy) {
		t.Fatal("правая сторона рамки не найдена")
	}
}

func hasInkAt(g *image.Gray, x, y int) bool {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			px, py := x+dx, y+dy
			if px < 0 || py < 0 || px >= g.Bounds().Dx() || py >= g.Bounds().Dy() {
				continue
			}
			if g.GrayAt(px, py).Y < 128 {
				return true
			}
		}
	}
	return false
}

// Поворот на 90° меняет местами ширину и высоту блока надписи.
func TestRenderTemplateRotate(t *testing.T) {
	// Короткое слово: без поворота блок шире, чем выше.
	word := "ШИРЕ"
	base, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "text", X: 2, Y: 2, Text: word, FontMM: 3}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bMinX, bMaxX := inkColumns(base)
	bMinY, bMaxY := inkRows(base)
	baseW, baseH := bMaxX-bMinX, bMaxY-bMinY

	rot, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "text", X: 2, Y: 2, Text: word, FontMM: 3, Rotate: 90}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rMinX, rMaxX := inkColumns(rot)
	rMinY, rMaxY := inkRows(rot)
	rotW, rotH := rMaxX-rMinX, rMaxY-rMinY

	if baseW <= baseH {
		t.Fatalf("исходная надпись не шире, чем выше: %dx%d", baseW, baseH)
	}
	if rotH <= rotW {
		t.Fatalf("после поворота блок не стал выше, чем шире: %dx%d", rotW, rotH)
	}
	// Размеры должны поменяться местами (с точностью до пары точек).
	if abs(rotW-baseH) > 3 || abs(rotH-baseW) > 3 {
		t.Fatalf("поворот не swapped: было %dx%d, стало %dx%d", baseW, baseH, rotW, rotH)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// У линий и рамок без размеров — понятная ошибка.
func TestRenderTemplateShapeErrors(t *testing.T) {
	for _, el := range []Element{
		{Kind: "line", X: 1, Y: 1},
		{Kind: "frame", X: 1, Y: 1},
	} {
		if _, err := RenderTemplate(LabelTemplate{LengthMM: 30, Elements: []Element{el}}, nil); err == nil {
			t.Fatalf("%s без размеров: ожидалась ошибка", el.Kind)
		}
	}
}

// --- дата, время и структуры ---

func TestSubstituteDateTime(t *testing.T) {
	when := time.Date(2026, 9, 18, 0, 25, 0, 0, time.Local)
	cases := map[string]string{
		"дата {дата}":          "дата 18.09.2026",
		"время {время}":        "время 00:25",
		"сделано {дата-время}": "сделано 18.09.2026 00:25",
		"{дата} в {время}":     "18.09.2026 в 00:25",
		"{ДАТА}":               "18.09.2026", // регистр не важен
		"без подстановок":      "без подстановок",
	}
	for in, want := range cases {
		got, err := substituteFields(in, nil, when)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != want {
			t.Fatalf("%q → %q, ожидалось %q", in, got, want)
		}
	}
}

// Дата и время на этикетке действительно появляются.
func TestRenderTemplateDateTime(t *testing.T) {
	when := time.Date(2026, 9, 18, 0, 25, 0, 0, time.Local)
	img, err := RenderTemplateAt(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{
			{Kind: "text", X: 1, Y: 1, W: 28, Text: "{дата-время}", FontMM: 2},
		},
	}, nil, when)
	if err != nil {
		t.Fatal(err)
	}
	if minX, _ := inkColumns(img); minX < 0 {
		t.Fatal("дата со временем не напечатались")
	}
}

// Структура по SMILES. Если RDKit не установлен — тест пропускается,
// чтобы сборка не зависела от химического окружения.
func TestRenderSmiles(t *testing.T) {
	if !ChemAvailable() {
		t.Skip("RDKit не установлен — пропускаю")
	}
	st, err := RenderSmiles("CC(=CCCC(C)(C=C)O)C", 96, 96, 0, 2)
	if err != nil {
		t.Fatalf("структура не нарисована: %v", err)
	}
	if st.Bounds().Dx() != 96 || st.Bounds().Dy() != 96 {
		t.Fatalf("размер структуры %v, ожидалось 96x96", st.Bounds())
	}
	// Структура должна быть нарисована, а не пустая.
	dark := 0
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			if st.GrayAt(x, y).Y < 128 {
				dark++
			}
		}
	}
	if dark < 50 {
		t.Fatalf("на структуре всего %d тёмных точек — похоже, пусто", dark)
	}

	// Повторный вызов берётся из кэша и даёт тот же результат.
	again, err := RenderSmiles("CC(=CCCC(C)(C=C)O)C", 96, 96, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if again != st {
		t.Fatal("повторный вызов не попал в кэш")
	}
}

// Мусорная строка SMILES — понятная ошибка, а не пустая этикетка.
func TestRenderSmilesBadInput(t *testing.T) {
	if !ChemAvailable() {
		t.Skip("RDKit не установлен — пропускаю")
	}
	if _, err := RenderSmiles("это не молекула", 96, 96, 0, 2); err == nil {
		t.Fatal("ожидалась ошибка разбора SMILES")
	}
	if _, err := RenderSmiles("   ", 96, 96, 0, 2); err == nil {
		t.Fatal("пустая строка SMILES должна давать ошибку")
	}
}

// Элемент-структура в шаблоне: SMILES можно брать из данных.
func TestRenderTemplateSmilesElement(t *testing.T) {
	if !ChemAvailable() {
		t.Skip("RDKit не установлен — пропускаю")
	}
	img, err := RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{
			{Kind: "smiles", X: 0.5, Y: 1.5, W: 9, H: 9, Smiles: "{1}"},
			{Kind: "text", X: 10, Y: 1, W: 19, Text: "ЛИНАЛООЛ", FontMM: 2.4},
		},
	}, []string{"CC(=CCCC(C)(C=C)O)C"})
	if err != nil {
		t.Fatal(err)
	}
	// Структура слева, текст справа.
	left, _ := inkColumns(img)
	if left < 2 || left > 8 {
		t.Fatalf("структура начинается на %d, ожидалось около 4 (0,5 мм)", left)
	}
	// У структуры без размеров — понятная ошибка.
	_, err = RenderTemplate(LabelTemplate{
		LengthMM: 30,
		Elements: []Element{{Kind: "smiles", X: 1, Y: 1, Smiles: "CCO"}},
	}, nil)
	if err == nil {
		t.Fatal("структура без размеров: ожидалась ошибка")
	}
}

// Шаблон настроек должен сохранять и возвращать элементы конструктора.
// Раньше поля elements в структуре не было, и сохранённые шаблоны выходили
// пустыми: в файле лежали одни настройки без единого элемента.
func TestTemplateRoundTrip(t *testing.T) {
	tpl := Template{
		Name:      "проба",
		Length:    30,
		Density:   2,
		Threshold: 200,
		Elements: []Element{
			{Kind: "text", X: 0.4, Y: 2.8, W: 29.2, Text: "Драхма. Парфия. Артабан II",
				FontMM: 1.8, Align: "center", Family: "DejaVu Sans", Bold: true},
			{Kind: "text", X: 0.4, Y: 6.3, W: 29.2, Text: "12-38 н.э. XF",
				FontMM: 2.6, Align: "center", Family: "Arial"},
		},
	}

	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatal(err)
	}
	var back Template
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}

	if len(back.Elements) != 2 {
		t.Fatalf("после сохранения элементов %d, ожидалось 2", len(back.Elements))
	}
	if back.Elements[0].Text != "Драхма. Парфия. Артабан II" {
		t.Fatalf("текст первого элемента потерялся: %q", back.Elements[0].Text)
	}
	if back.Elements[1].FontMM != 2.6 || back.Elements[1].Family != "Arial" {
		t.Fatalf("настройки второго элемента потерялись: %+v", back.Elements[1])
	}
	if back.Threshold != 200 {
		t.Fatalf("порог потерялся: %d", back.Threshold)
	}

	// И шаблон с этими элементами действительно рисуется.
	img, err := RenderTemplate(LabelTemplate{LengthMM: back.Length, Elements: back.Elements}, nil)
	if err != nil {
		t.Fatalf("шаблон из файла не рисуется: %v", err)
	}
	if minX, _ := inkColumns(img); minX < 0 {
		t.Fatal("на этикетке из сохранённого шаблона ничего не напечаталось")
	}
}
