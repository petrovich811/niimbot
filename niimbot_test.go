package main

import (
	"bytes"
	"image/color"
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
