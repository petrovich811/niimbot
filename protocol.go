// Протокол принтера этикеток NIIMBOT N1.
//
// Восстановлен по открытой библиотеке NiimBlueLib
// (https://github.com/MultiMote/niimbluelib, MIT) и проверен на живом
// принтере N1 (id модели 3586, серийный GA24110447).
//
// Пакет: 55 55 | команда | длина | данные | XOR-контроль | AA AA
// Многобайтовые числа — big-endian. Контрольная сумма — XOR команды,
// длины и всех байт данных.
package main

import "fmt"

const (
	head0 = 0x55
	head1 = 0x55
	tail0 = 0xAA
	tail1 = 0xAA
)

// Команды: клиент → принтер
const (
	cmdConnect           = 0xC1
	cmdPrintStart        = 0x01
	cmdPageStart         = 0x03
	cmdPageEnd           = 0xE3
	cmdPrintEnd          = 0xF3
	cmdPrintStatus       = 0xA3
	cmdSetDensity        = 0x21
	cmdSetLabelType      = 0x23
	cmdSetPageSize       = 0x13
	cmdPrintBitmapRow    = 0x85
	cmdPrintBitmapRowIdx = 0x83
	cmdPrintEmptyRow     = 0x84
	cmdPrinterInfo       = 0x40
	cmdPrinterStatusData = 0xA5
	cmdHeartbeat         = 0xDC
	cmdPrintTestPage     = 0x5A
	cmdCancelPrint       = 0xDA
	cmdRfidInfo          = 0x1A // метка рулона этикеток
	cmdRfidInfo2         = 0x1C // метка ленты (риббона)
)

// Ответы: принтер → клиент (проверено на живом N1)
const (
	respConnect     = 0xC2
	respPrintStart  = 0x02
	respPageStart   = 0x04
	respPageEnd     = 0xE4
	respPrintEnd    = 0xF4
	respPrintStatus = 0xB3
	respSetDensity  = 0x31
	respPrintError  = 0xDB
	respRfidInfo    = 0x1B // ответ на cmdRfidInfo
	respRfidInfo2   = 0x1D // ответ на cmdRfidInfo2
)

// Статус принтера у N1 приходит кодом 0xB4 (в библиотеке указан 0xB5) —
// принимаем оба.
var respStatusData = []byte{0xB4, 0xB5}

// Типы запроса PrinterInfo (cmdPrinterInfo) и соответствующие ответы.
const (
	infoDensity   = 1
	infoLabelType = 3
	infoModelID   = 8
	infoFirmware  = 9
	infoBattery   = 10
	infoSerial    = 11
)

var infoResponse = map[byte]byte{
	infoDensity:   0x41,
	2:             0x42, // скорость
	infoLabelType: 0x43,
	6:             0x46, // язык
	infoModelID:   0x48,
	infoFirmware:  0x49,
	infoBattery:   0x4A,
	infoSerial:    0x4B,
}

// Типы этикеток протокола NIIMBOT.
//
// Описание типов: https://printers.niim.blue/other/label-types/
// Полный набор из восьми типов поддерживают разные модели; у N1 их пять —
// см. labelTypesN1.
var labelTypes = map[string]byte{
	"withgaps":     1,  // с зазорами — обычные этикетки
	"black":        2,  // чёрные (термочувствительные)
	"continuous":   3,  // непрерывная лента без зазоров
	"perforated":   4,  // перфорированные
	"transparent":  5,  // прозрачные
	"pvctag":       6,  // ПВХ-бирки
	"blackmarkgap": 10, // с чёрной меткой
	"heatshrink":   11, // термоусадочная трубка
}

// labelTypesN1 — какие типы этикеток поддерживает именно N1.
// Источник: таблица моделей NiimBlueLib (paperTypes для id 3586).
//
// Остальные три типа (black, perforated, pvctag) протокол знает, но
// прошивка N1 их не поддерживает — печатать на них не стоит.
var labelTypesN1 = map[byte]string{
	1:  "withgaps — этикетки с зазорами (обычные)",
	3:  "continuous — непрерывная лента",
	5:  "transparent — прозрачные",
	10: "blackmarkgap — с чёрной меткой",
	11: "heatshrink — термоусадочная трубка",
}

// labelListN1 возвращает список поддерживаемых N1 типов для сообщения об ошибке.
func labelListN1() string {
	out := ""
	for _, name := range []byte{1, 3, 5, 10, 11} {
		out += "\n  --label " + labelTypesN1[name]
	}
	return out
}

// Параметры N1 (из таблицы моделей NiimBlueLib, id 3586)
const (
	modelIDN1       = 3586
	printheadPixels = 96 // точек в головке = 12 мм при 203 dpi
	dotsPerMM       = 203.0 / 25.4
	bleServiceUUID  = "e7810a71-73ae-499d-8c15-faa9aef0c3f2"
)

// Packet — разобранный пакет
type Packet struct {
	Cmd  byte
	Data []byte
}

func (p Packet) String() string {
	return fmt.Sprintf("0x%02x %x", p.Cmd, p.Data)
}

// buildPacket собирает пакет для отправки.
func buildPacket(cmd byte, data []byte) []byte {
	out := make([]byte, 0, len(data)+7)
	out = append(out, head0, head1, cmd, byte(len(data)))
	out = append(out, data...)
	var cs byte
	cs ^= cmd
	cs ^= byte(len(data))
	for _, b := range data {
		cs ^= b
	}
	out = append(out, cs, tail0, tail1)
	return out
}

func u16(v int) []byte {
	return []byte{byte(v >> 8), byte(v)}
}

// parsePackets выдёргивает целые пакеты из накопленного буфера,
// оставляя в нём незавершённый хвост.
func parsePackets(buf *[]byte) []Packet {
	var out []Packet
	b := *buf
	for {
		// ищем заголовок
		start := -1
		for i := 0; i+1 < len(b); i++ {
			if b[i] == head0 && b[i+1] == head1 {
				start = i
				break
			}
		}
		if start < 0 {
			b = b[:0]
			break
		}
		if len(b) < start+4 {
			b = b[start:]
			break
		}
		length := int(b[start+3])
		total := 2 + 1 + 1 + length + 1 + 2
		if len(b) < start+total {
			b = b[start:]
			break
		}
		chunk := b[start : start+total]
		if chunk[total-2] != tail0 || chunk[total-1] != tail1 {
			b = b[start+2:]
			continue
		}
		data := make([]byte, length)
		copy(data, chunk[4:4+length])
		out = append(out, Packet{Cmd: chunk[2], Data: data})
		b = b[start+total:]
	}
	*buf = b
	return out
}

// --------------------------------------------------------------------------
// RFID-метки рулона
// --------------------------------------------------------------------------

// RfidInfo — данные метки рулона (этикеток или ленты).
//
// Формат ответа (по NiimBlueLib, parseRfidInfoResponse):
//
//	uuid(8) | штрихкод(vstring) | серийник(vstring) |
//	всего(int16) | израсходовано(int16) | тип этикетки(int8) [| ёмкость(int16)]
//
// Если в ответе один байт — метки нет.
type RfidInfo struct {
	TagPresent  bool
	UUID        string
	Barcode     string
	Serial      string
	AllPaper    int
	UsedPaper   int
	LabelType   byte
	LabelTypeOK bool
	Capacity    int
	HasCapacity bool
}

// Leftover — сколько этикеток осталось в рулоне.
func (r RfidInfo) Leftover() int {
	if r.AllPaper < 0 || r.UsedPaper < 0 {
		return -1
	}
	return r.AllPaper - r.UsedPaper
}

// LabelTypeName — имя типа этикетки из метки.
func (r RfidInfo) LabelTypeName() string {
	for name, id := range labelTypes {
		if id == r.LabelType {
			return name
		}
	}
	return ""
}

// parseRfidInfo разбирает данные метки.
func parseRfidInfo(data []byte) RfidInfo {
	var info RfidInfo
	if len(data) <= 1 {
		return info // метки нет
	}
	info.TagPresent = true

	off := 0
	if len(data) < 8 {
		return info
	}
	info.UUID = fmt.Sprintf("%x", data[off:off+8])
	off += 8

	readVString := func() string {
		if off >= len(data) {
			return ""
		}
		n := int(data[off])
		off++
		if off+n > len(data) {
			n = len(data) - off
		}
		s := string(data[off : off+n])
		off += n
		return s
	}
	info.Barcode = readVString()
	info.Serial = readVString()

	readI16 := func() (int, bool) {
		if off+2 > len(data) {
			return 0, false
		}
		v := int(data[off])<<8 | int(data[off+1])
		off += 2
		return v, true
	}
	if v, ok := readI16(); ok {
		info.AllPaper = v
	} else {
		info.AllPaper = -1
	}
	if v, ok := readI16(); ok {
		info.UsedPaper = v
	} else {
		info.UsedPaper = -1
	}

	if off < len(data) {
		info.LabelType = data[off]
		info.LabelTypeOK = true
		off++
	}
	if off+2 <= len(data) {
		info.Capacity = int(data[off])<<8 | int(data[off+1])
		info.HasCapacity = true
	}
	return info
}
