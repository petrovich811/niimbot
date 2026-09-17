// Системные шрифты: поиск, разбор и выбор для элементов этикетки.
//
// Шрифты ищутся там, где их держат Linux и Windows. Каждый файл разбирается
// и попадает в список с названием семейства: в шаблоне хранится именно
// семейство, а не путь к файлу, — тогда шаблон переживёт перенос на другую
// машину, где тот же шрифт лежит в другом месте.
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// FontInfo — шрифт, найденный в системе.
type FontInfo struct {
	Family   string `json:"family"`   // название семейства, например «DejaVu Sans»
	Style    string `json:"style"`    // Regular, Bold, Italic…
	Path     string `json:"path"`     // файл, откуда взят
	Bold     bool   `json:"bold"`
	Italic   bool   `json:"italic"`
	Cyrillic bool   `json:"cyrillic"` // есть ли буквы кириллицы
}

var (
	fontsOnce sync.Once
	fontsList []FontInfo
	fontsErr  error
)

// SystemFonts возвращает шрифты системы (список считается один раз).
func SystemFonts() ([]FontInfo, error) {
	fontsOnce.Do(func() {
		fontsList, fontsErr = scanFonts()
	})
	return fontsList, fontsErr
}

// fontDirs — где искать шрифты в разных системах.
func fontDirs() []string {
	if runtime.GOOS == "windows" {
		win := os.Getenv("WINDIR")
		if win == "" {
			win = `C:\Windows`
		}
		dirs := []string{filepath.Join(win, "Fonts")}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	}
	home, _ := os.UserHomeDir()
	dirs := []string{
		"/usr/share/fonts",
		"/usr/local/share/fonts",
		"/usr/share/fonts/truetype",
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".fonts"),
			filepath.Join(home, ".local", "share", "fonts"))
	}
	return dirs
}

// scanFonts обходит каталоги со шрифтами и разбирает каждый файл.
func scanFonts() ([]FontInfo, error) {
	seen := map[string]bool{} // у одного шрифта бывает несколько имён файлов
	var out []FontInfo

	for _, dir := range fontDirs() {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc":
			default:
				return nil
			}
			info, err := readFontInfo(path)
			if err != nil {
				return nil // битый или нечитаемый файл пропускаем
			}
			key := info.Family + "|" + info.Style
			if seen[key] {
				return nil
			}
			seen[key] = true
			out = append(out, info)
			return nil
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Family != out[j].Family {
			return out[i].Family < out[j].Family
		}
		return out[i].Style < out[j].Style
	})
	return out, nil
}

// readFontInfo разбирает файл шрифта: название семейства, начертание
// и наличие кириллицы.
func readFontInfo(path string) (FontInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FontInfo{}, err
	}
	f, err := sfnt.Parse(data)
	if err != nil {
		// .ttc — набор шрифтов; берём первый
		coll, err2 := sfnt.ParseCollection(data)
		if err2 != nil {
			return FontInfo{}, err
		}
		f, err = coll.Font(0)
		if err != nil {
			return FontInfo{}, err
		}
	}

	var buf sfnt.Buffer
	family, err := f.Name(&buf, sfnt.NameIDFamily)
	if err != nil || strings.TrimSpace(family) == "" {
		family = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	style, _ := f.Name(&buf, sfnt.NameIDSubfamily)
	style = strings.TrimSpace(style)
	if style == "" {
		style = "Regular"
	}

	lower := strings.ToLower(style)
	info := FontInfo{
		Family: family,
		Style:  style,
		Path:   path,
		Bold:   strings.Contains(lower, "bold") || strings.Contains(lower, "black"),
		Italic: strings.Contains(lower, "italic") || strings.Contains(lower, "oblique"),
	}

	// Кириллица: пробуем найти заглавную «А» и строчную «я».
	if idx, err := f.GlyphIndex(&buf, 'А'); err == nil && idx != 0 {
		if idx2, err := f.GlyphIndex(&buf, 'я'); err == nil && idx2 != 0 {
			info.Cyrillic = true
		}
	}
	return info, nil
}

// resolveFontFamily находит файл шрифта по названию семейства.
//
// Предпочтение: не курсив, затем обычное начертание, затем жирное.
// Если семейства нет в системе — берём шрифт по умолчанию.
func resolveFontFamily(family string, bold bool) (string, bool) {
	family = strings.TrimSpace(family)
	if family == "" {
		return "", false
	}
	list, err := SystemFonts()
	if err != nil {
		return "", false
	}

	var regular, boldFile, other string
	for _, f := range list {
		if !strings.EqualFold(f.Family, family) || f.Italic {
			continue
		}
		switch {
		case f.Bold:
			if boldFile == "" {
				boldFile = f.Path
			}
		default:
			if regular == "" {
				regular = f.Path
			}
		}
		if other == "" {
			other = f.Path
		}
	}

	if bold && boldFile != "" {
		return boldFile, true
	}
	if regular != "" {
		return regular, true
	}
	if boldFile != "" {
		return boldFile, true
	}
	if other != "" {
		return other, true
	}
	return "", false
}

// fontFamiliesForPicker — список семейств для выпадающего списка:
// только те, где есть кириллица, — на этикетках почти всегда русский текст.
func fontFamiliesForPicker() []string {
	list, err := SystemFonts()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range list {
		if !f.Cyrillic || seen[f.Family] {
			continue
		}
		seen[f.Family] = true
		out = append(out, f.Family)
	}
	return out
}
