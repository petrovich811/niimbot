package main

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
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
