// Конструктор этикеток: шаблон из размещённых элементов.
//
// Этикетка описывается не текстом с переносами, а списком элементов —
// надписей и картинок, у каждого свои координаты в миллиметрах. Координаты
// идут в ориентации чтения:
//
//	X — вдоль этикетки (по подаче), 0 слева
//	Y — поперёк этикетки (по головке), 0 сверху
//
// Ширина этикетки задаётся длиной (X), высота всегда 12 мм — это 96 точек
// печатающей головки.
package main

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Element — элемент этикетки: надпись или картинка.
type Element struct {
	Kind string  `json:"kind"`        // "text" или "image"
	X    float64 `json:"x"`           // мм вдоль этикетки
	Y    float64 `json:"y"`           // мм поперёк этикетки
	W    float64 `json:"w,omitempty"` // мм: ширина рамки (для выравнивания и картинок)
	H    float64 `json:"h,omitempty"` // мм: высота (для картинок)

	// Надпись
	Text   string  `json:"text,omitempty"`
	FontMM float64 `json:"font,omitempty"`  // высота строки в мм
	Align  string  `json:"align,omitempty"` // left | center | right (внутри рамки X..X+W)

	// Картинка: путь к файлу или data:URL с base64
	Image string `json:"image,omitempty"`
}

// LabelTemplate — шаблон этикетки для конструктора.
type LabelTemplate struct {
	LengthMM float64   `json:"length"` // длина этикетки в мм
	Elements []Element `json:"elements"`
}

// labelHeightMM — высота этикетки: 12 мм по печатающей головке.
const labelHeightMM = 12.0

// RenderTemplate рисует этикетку по шаблону, подставляя поля из строки данных.
//
// row — столбцы строки данных: {1} — первый, {2} — второй и так далее.
func RenderTemplate(tpl LabelTemplate, row []string) (*image.Gray, error) {
	img := NewLabelImage(tpl.LengthMM)

	for i, el := range tpl.Elements {
		var err error
		switch el.Kind {
		case "text":
			err = drawTextElement(img, el, row)
		case "image":
			err = drawImageElement(img, el)
		case "":
			err = fmt.Errorf("у элемента %d не указан вид (kind)", i+1)
		default:
			err = fmt.Errorf("элемент %d: неизвестный вид %q", i+1, el.Kind)
		}
		if err != nil {
			return nil, err
		}
	}
	return img, nil
}

// drawTextElement рисует надпись с учётом координат, кегля и выравнивания.
func drawTextElement(img *image.Gray, el Element, row []string) error {
	text, err := substituteFields(el.Text, row)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	fontMM := el.FontMM
	if fontMM <= 0 {
		fontMM = 2.5 // разумное умолчание, если кегль не задан
	}
	fontPath, err := findFontFile()
	if err != nil {
		return err
	}
	// 1 мм = 8 точек, поэтому кегль в точках изображения — это мм × 8.
	face, err := loadFace(fontPath, fontMM*dotsPerMM)
	if err != nil {
		return err
	}
	defer face.Close()

	lines := strings.Split(text, "\n")
	metrics := face.Metrics()
	lineH := metrics.Height.Ceil()
	if lineH <= 0 {
		lineH = int(fontMM*dotsPerMM) + 2
	}
	ascent := metrics.Ascent.Ceil()

	x0 := int(math.Round(el.X * dotsPerMM))
	y0 := int(math.Round(el.Y * dotsPerMM))
	boxW := int(math.Round(el.W * dotsPerMM))

	for i, line := range lines {
		lineW := font.MeasureString(face, line).Ceil()
		x := x0
		switch strings.ToLower(el.Align) {
		case "center":
			if boxW > 0 {
				x = x0 + (boxW-lineW)/2
			} else {
				x = x0 - lineW/2
			}
		case "right":
			if boxW > 0 {
				x = x0 + boxW - lineW
			} else {
				x = x0 - lineW
			}
		}
		y := y0 + ascent + i*lineH
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(color.Black),
			Face: face,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(line)
	}
	return nil
}

// drawImageElement рисует картинку в её рамке.
func drawImageElement(img *image.Gray, el Element) error {
	if el.Image == "" {
		return fmt.Errorf("у картинки не указан источник")
	}
	src, err := loadImageSource(el.Image)
	if err != nil {
		return err
	}
	wMM, hMM := el.W, el.H
	if wMM <= 0 || hMM <= 0 {
		return fmt.Errorf("у картинки не заданы размеры (w и h в мм)")
	}
	dw := int(math.Round(wMM * dotsPerMM))
	dh := int(math.Round(hMM * dotsPerMM))
	if dw < 1 || dh < 1 {
		return fmt.Errorf("картинка получается меньше точки")
	}

	scaled := image.NewGray(image.Rect(0, 0, dw, dh))
	drawScaled(scaled, src)

	x0 := int(math.Round(el.X * dotsPerMM))
	y0 := int(math.Round(el.Y * dotsPerMM))
	draw.Draw(img, image.Rect(x0, y0, x0+dw, y0+dh), scaled, image.Point{}, draw.Src)
	return nil
}

// drawScaled уменьшает или увеличивает картинку в подготовленный холст.
func drawScaled(dst *image.Gray, src image.Image) {
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
}

// loadImageSource читает картинку из файла или из data:URL с base64.
//
// data:URL удобен конструктору: картинку можно вставить прямо в шаблон,
// и он останется самодостаточным — одним файлом.
func loadImageSource(src string) (image.Image, error) {
	if strings.HasPrefix(src, "data:") {
		comma := strings.Index(src, ",")
		if comma < 0 {
			return nil, fmt.Errorf("повреждённый data:URL картинки")
		}
		raw, err := base64.StdEncoding.DecodeString(src[comma+1:])
		if err != nil {
			return nil, fmt.Errorf("разбор base64 картинки: %w", err)
		}
		img, _, err := image.Decode(strings.NewReader(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("разбор картинки из шаблона: %w", err)
		}
		return img, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("разбор картинки %s: %w", src, err)
	}
	return img, nil
}

// substituteFields заменяет {1}, {2}… на столбцы строки данных.
//
// Ссылка на столбец, которого нет, — ошибка: иначе на этикетке напечаталось
// бы литеральное «{3}», и брак был бы виден только на бумаге.
func substituteFields(text string, row []string) (string, error) {
	for i, v := range row {
		text = strings.ReplaceAll(text, fmt.Sprintf("{%d}", i+1), v)
	}
	if bad := rePlaceholder.FindString(text); bad != "" {
		return "", fmt.Errorf(
			"элемент использует %s, а в данных только %d столбц(а/ов)", bad, len(row))
	}
	return text, nil
}
