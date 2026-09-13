// Рендер этикетки и превращение изображения в строки для принтера.
//
// Ориентация. Этикетка N1 — 14×30 мм: 12 мм поперёк головки (96 точек)
// и 30 мм вдоль подачи (240 точек). Текст должен идти ВДОЛЬ этикетки,
// поэтому содержимое рисуется в «ориентации чтения»:
//
//	ширина = длина этикетки (подача), высота = ширина головки
//
// а затем транспонируется: строка r для принтера — это столбец c
// изображения при x=r. Иначе текст ложится поперёк этикетки
// (проверено на живом принтере).
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const feedMargin = 6 // отступ в точках

var fontCandidates = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
}

func findFontFile() (string, error) {
	for _, p := range fontCandidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("не нашёл TTF-шрифт с кириллицей")
}

func loadFace(path string, sizePt float64) (font.Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size: sizePt,
		DPI:  72, // кегль задаём в точках изображения напрямую
		Hinting: font.HintingFull,
	})
}

// NewLabelImage создаёт изображение этикетки в ориентации чтения.
//
// ВАЖНО: image.NewGray даёт чёрный холст (нули = чёрный), поэтому фон
// сразу заливается белым — иначе на печать уйдёт сплошной чёрный лист.
func NewLabelImage(lengthMM float64) *image.Gray {
	w := int(lengthMM*dotsPerMM + 0.5) // вдоль подачи
	img := image.NewGray(image.Rect(0, 0, w, printheadPixels))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

// RenderText рисует строки текста на изображении этикетки.
//
// Изображение — в ориентации чтения: ширина = длина этикетки (текст идёт
// слева направо вдоль неё), высота = 96 точек поперёк головки.
// Если fontPt <= 0, кегль подбирается так, чтобы всё влезло.
func RenderText(lines []string, lengthMM float64, fontPt float64) (*image.Gray, error) {
	fontPath, err := findFontFile()
	if err != nil {
		return nil, err
	}
	img := NewLabelImage(lengthMM)
	availW := img.Bounds().Dx() - 2*feedMargin
	availH := img.Bounds().Dy() - 2*feedMargin

	if fontPt <= 0 {
		for size := 40.0; size >= 6; size -= 1 {
			face, err := loadFace(fontPath, size)
			if err != nil {
				return nil, err
			}
			w, h := measureBlock(face, lines)
			face.Close()
			if w <= availW && h <= availH {
				fontPt = size
				break
			}
		}
		if fontPt <= 0 {
			fontPt = 6
		}
	}

	face, err := loadFace(fontPath, fontPt)
	if err != nil {
		return nil, err
	}
	defer face.Close()

	_, blockH := measureBlock(face, lines)
	y := feedMargin
	if blockH < availH { // центрируем блок по высоте (поперёк головки)
		y = (img.Bounds().Dy() - blockH) / 2
	}
	lineHeight := blockH / len(lines)

	for _, line := range lines {
		lw := font.MeasureString(face, line).Round()
		x := (img.Bounds().Dx() - lw) / 2 // центрируем строку по длине
		if x < 1 {
			x = 1
		}
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(color.Black),
			Face: face,
			Dot:  fixed.P(x, y+face.Metrics().Ascent.Round()),
		}
		d.DrawString(line)
		y += lineHeight
	}
	return img, nil
}

func measureBlock(face font.Face, lines []string) (int, int) {
	maxW := 0
	for _, l := range lines {
		if w := font.MeasureString(face, l).Round(); w > maxW {
			maxW = w
		}
	}
	m := face.Metrics()
	lineH := (m.Ascent + m.Descent).Round() + 2
	return maxW, lineH * len(lines)
}

// Row — одна строка для принтера: 12 байт (96 точек) или nil для пустой.
type Row []byte

// ImageToRows транспонирует изображение в строки для принтера.
//
// flip переворачивает содержимое на 180° — на случай, если этикетка
// выходит «вверх ногами» при том же направлении подачи.
func ImageToRows(img *image.Gray, flip bool) []Row {
	b := img.Bounds()
	feedLen := b.Dx()   // строк подачи
	across := b.Dy()    // точек поперёк головки
	bytesPerRow := across / 8

	rows := make([]Row, 0, feedLen)
	for r := 0; r < feedLen; r++ {
		row := make([]byte, bytesPerRow)
		blank := true
		for c := 0; c < across; c++ {
			// Ось головки идёт В ОБРАТНОМ порядке: точка головки c берётся
			// из строки изображения across-1-c. Так устроен протокол
			// (в NiimBlueLib это `idx = (height-1-col)*width + row`).
			// Без этого разворота этикетка выходит зеркальной — проверено
			// печатью на живом принтере.
			x, y := r, across-1-c
			if flip {
				x = feedLen - 1 - r
				y = c
			}
			if img.GrayAt(b.Min.X+x, b.Min.Y+y).Y < 128 {
				row[c/8] |= 1 << (7 - uint(c%8))
				blank = false
			}
		}
		if blank {
			rows = append(rows, nil)
		} else {
			rows = append(rows, row)
		}
	}
	return rows
}

// countPixels считает чёрные точки: всего и по трём частям.
// Разбиение — как в оригинальном протоколе: данные строки делятся на три
// равные части, и в пакет уходят три счётчика.
func countPixels(row Row, across int) (total int, parts [3]byte) {
	chunk := across / 8 / 3
	if chunk < 1 {
		chunk = 1
	}
	for i, v := range row {
		n := popcount(v)
		total += n
		idx := i / chunk
		if idx > 2 {
			idx = 2
		}
		parts[idx] += byte(n)
	}
	return
}

func popcount(b byte) int {
	n := 0
	for b != 0 {
		n += int(b & 1)
		b >>= 1
	}
	return n
}

// LoadImageFile открывает PNG/JPEG и приводит к нужному размеру.
func LoadImageFile(path string) (*image.Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst, nil
}
