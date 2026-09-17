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
	"math"
	"os"

	// Декодеры картинок: без них image.Decode не понимает ни один формат.
	// PNG раньше работал случайно — его импортировали ради кодирования макета.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const feedMargin = 6 // отступ в точках

// binarizeThreshold — порог перевода в чёрно-белое, 0..255.
//
// Всё, что темнее порога, становится чёрным. По умолчанию 200, а не 128:
// линии и текст рисуются со сглаживанием, их края светло-серые, и при пороге
// 128 связи структур рвутся пунктиром, а цифры рассыпаются на точки —
// проверено на живом принтере. При 200 тонкие элементы выходят плотными.
// Выше 230 буквы начинают заплывать.
var binarizeThreshold uint8 = 200

// Binarize переводит изображение в чёрно-белое тем же порогом, что идёт
// в принтер. Нужен, чтобы макет показывал ровно то, что напечатается.
func Binarize(img *image.Gray) *image.Gray {
	return BinarizeAt(img, binarizeThreshold)
}

// BinarizeAt — то же с явным порогом.
func BinarizeAt(img *image.Gray, threshold uint8) *image.Gray {
	out := image.NewGray(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if img.GrayAt(x, y).Y < threshold {
				out.SetGray(x, y, color.Gray{Y: 0})
			} else {
				out.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return out
}

// fontCandidates — шрифты с кириллицей для трёх систем.
// Жирные варианты идут первыми: на этикетке мелкий текст читается лучше.
var fontCandidates = []string{
	// Linux
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	// Windows
	`C:\Windows\Fonts\arialbd.ttf`,
	`C:\Windows\Fonts\seguisb.ttf`,
	`C:\Windows\Fonts\arial.ttf`,
	`C:\Windows\Fonts\segoeui.ttf`,
	`C:\Windows\Fonts\tahoma.ttf`,
	// macOS
	"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/Library/Fonts/Arial Bold.ttf",
	"/Library/Fonts/Arial.ttf",
	"/System/Library/Fonts/Helvetica.ttc",
}

// fontEnvVar позволяет указать свой шрифт, если ничего не нашлось.
const fontEnvVar = "NIIMBOT_FONT"

func findFontFile() (string, error) {
	if custom := os.Getenv(fontEnvVar); custom != "" {
		return custom, nil
	}
	for _, p := range fontCandidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("не нашёл TTF-шрифт с кириллицей — укажите свой через %s", fontEnvVar)
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
		Size:    sizePt,
		DPI:     72, // кегль задаём в точках изображения напрямую
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
	return ImageToRowsAt(img, flip, binarizeThreshold)
}

// ImageToRowsAt — то же, но с явным порогом: у разных этикеток он бывает свой.
func ImageToRowsAt(img *image.Gray, flip bool, threshold uint8) []Row {
	b := img.Bounds()
	feedLen := b.Dx() // строк подачи
	across := b.Dy()  // точек поперёк головки
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
			if img.GrayAt(b.Min.X+x, b.Min.Y+y).Y < threshold {
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
func LoadImageFile(path string, lengthMM float64) (*image.Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return FitToLabel(src, lengthMM), nil
}

// FitToLabel вписывает картинку в область этикетки.
//
// Принтер печатает 96 точек поперёк (12 мм), а длина этикетки задаётся
// режимом. Картинка любого размера вписывается целиком с сохранением
// пропорций и центрируется на белом поле. Так её можно готовить в любой
// программе и не подгонять пиксели вручную: раньше размер брался как есть,
// и картинка не по размеру головки ломала печать.
//
// Картинка, уже совпадающая с областью, возвращается без изменений —
// чтобы заранее подготовленный макет не размывался пересчётом.
func FitToLabel(src image.Image, lengthMM float64) *image.Gray {
	if lengthMM <= 0 {
		lengthMM = 30
	}
	targetW := int(math.Round(lengthMM * dotsPerMM))
	targetH := printheadPixels

	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return whiteCanvas(targetW, targetH)
	}

	// Уже точный размер — отдаём как есть.
	if sw == targetW && sh == targetH {
		dst := image.NewGray(image.Rect(0, 0, targetW, targetH))
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		return dst
	}

	// Вписываем с сохранением пропорций.
	scale := math.Min(float64(targetW)/float64(sw), float64(targetH)/float64(sh))
	dw := int(math.Round(float64(sw) * scale))
	dh := int(math.Round(float64(sh) * scale))
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}

	scaled := image.NewGray(image.Rect(0, 0, dw, dh))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, b, draw.Src, nil)

	canvas := whiteCanvas(targetW, targetH)
	offX := (targetW - dw) / 2
	offY := (targetH - dh) / 2
	draw.Draw(canvas, image.Rect(offX, offY, offX+dw, offY+dh), scaled, image.Point{}, draw.Src)
	return canvas
}

// whiteCanvas создаёт белый холст нужного размера.
// image.NewGray сам по себе ЧЁРНЫЙ — это уже однажды ломало печать.
func whiteCanvas(w, h int) *image.Gray {
	c := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(c, c.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	return c
}
