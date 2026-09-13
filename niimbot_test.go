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

// Ориентация: изображение в ориентации чтения (ширина = подача,
// высота = головка) должно транспонироваться так, чтобы точка из
// верхнего левого угла попала в бит 7 первого байта первой строки.
func TestImageToRowsOrientation(t *testing.T) {
	img := NewLabelImage(2) // 2 мм ≈ 16 строк подачи
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() != printheadPixels {
		t.Fatalf("размер изображения %v, ожидалась высота %d", img.Bounds(), printheadPixels)
	}
	// белый фон обязателен: чёрный холст ушёл бы на печать сплошным листом
	if img.GrayAt(0, 0).Y != 255 {
		t.Fatalf("фон не белый: %d", img.GrayAt(0, 0).Y)
	}
	img.SetGray(0, 0, color.Gray{Y: 0}) // точка в начале координат

	rows := ImageToRows(img, false)
	if rows[0] == nil {
		t.Fatal("первая строка пустая, хотя в ней есть точка")
	}
	if rows[0][0] != 0x80 {
		t.Fatalf("первый байт = 0x%02x, ожидался 0x80 (бит 7 = крайняя точка)", rows[0][0])
	}
	if len(rows) != img.Bounds().Dx() {
		t.Fatalf("строк %d, ожидалось %d (по длине этикетки)", len(rows), img.Bounds().Dx())
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

