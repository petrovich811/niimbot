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
	"regexp"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Element — элемент этикетки: надпись или картинка.
type Element struct {
	Kind string  `json:"kind"`        // text | image | line | frame
	X    float64 `json:"x"`           // мм вдоль этикетки
	Y    float64 `json:"y"`           // мм поперёк этикетки
	W    float64 `json:"w,omitempty"` // мм: ширина элемента
	H    float64 `json:"h,omitempty"` // мм: высота элемента

	// Надпись
	Text   string  `json:"text,omitempty"`
	FontMM float64 `json:"font,omitempty"`   // высота шрифта в мм
	LineMM float64 `json:"line,omitempty"`   // высота строки в мм; 0 — по шрифту
	Family string  `json:"family,omitempty"` // семейство шрифта; пусто — по умолчанию
	Bold   bool    `json:"bold,omitempty"`   // жирное начертание
	Rotate int     `json:"rotate,omitempty"` // поворот: 0, 90, 180 или 270
	Align  string  `json:"align,omitempty"`  // left | center | right (внутри рамки X..X+W)

	// Структура: строка SMILES; может содержать поля {1}, {2}… из данных
	Smiles string `json:"smiles,omitempty"`

	// Толщина линий: для рамки — линий, для структуры — связей
	Thickness float64 `json:"thickness,omitempty"`

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
	return RenderTemplateAt(tpl, row, time.Now())
}

// RenderTemplateAt — то же, но с заданным моментом времени: нужно тестам,
// чтобы дата и время на этикетке были предсказуемы.
func RenderTemplateAt(tpl LabelTemplate, row []string, now time.Time) (*image.Gray, error) {
	img := NewLabelImage(tpl.LengthMM)

	for i, el := range tpl.Elements {
		var err error
		switch el.Kind {
		case "text":
			err = drawTextElement(img, el, row, now)
		case "image":
			err = drawImageElement(img, el)
		case "line":
			err = drawLineElement(img, el)
		case "frame":
			err = drawFrameElement(img, el)
		case "smiles":
			err = drawSmilesElement(img, el, row, now)
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
func drawTextElement(img *image.Gray, el Element, row []string, now time.Time) error {
	text, err := substituteFields(el.Text, row, now)
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
	fontPath, err := pickFont(el.Family, el.Bold)
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
	ascent := metrics.Ascent.Ceil()

	// Высота строки: заданная в мм или по метрикам шрифта.
	lineH := metrics.Height.Ceil()
	if el.LineMM > 0 {
		lineH = int(math.Round(el.LineMM * dotsPerMM))
	}
	if lineH <= 0 {
		lineH = int(fontMM*dotsPerMM) + 2
	}

	x0 := int(math.Round(el.X * dotsPerMM))
	y0 := int(math.Round(el.Y * dotsPerMM))
	boxW := int(math.Round(el.W * dotsPerMM))

	// Поворот: надпись рисуется на отдельном холсте и поворачивается целиком.
	// Так текст идёт поперёк этикетки — нужно для кабельных бирок.
	if angle := ((el.Rotate % 360) + 360) % 360; angle != 0 {
		blockW, blockH := 0, lineH*len(lines)
		for _, line := range lines {
			if w := font.MeasureString(face, line).Ceil(); w > blockW {
				blockW = w
			}
		}
		if blockW < 1 {
			blockW = 1
		}
		tmp := whiteCanvas(blockW, blockH)
		drawTextLines(tmp, face, lines, 0, 0, lineH, blockW, ascent, "left", 0)
		rot := rotateGray(tmp, angle)

		// Выравнивание считаем по повёрнутому блоку.
		rw, rh := rot.Bounds().Dx(), rot.Bounds().Dy()
		px := x0
		switch strings.ToLower(el.Align) {
		case "center":
			if boxW > 0 {
				px = x0 + (boxW-rw)/2
			} else {
				px = x0 - rw/2
			}
		case "right":
			if boxW > 0 {
				px = x0 + boxW - rw
			} else {
				px = x0 - rw
			}
		}
		draw.Draw(img, image.Rect(px, y0, px+rw, y0+rh), rot, image.Point{}, draw.Src)
		return nil
	}

	drawTextLines(img, face, lines, x0, y0, lineH, boxW, ascent, el.Align, 1)
	return nil
}

// drawTextLines рисует строки надписи с выравниванием внутри рамки boxW.
// step — шаг между строками в единицах lineH (нужен, когда строка одна
// и рисовать её надо на своём холсте).
func drawTextLines(dst *image.Gray, face font.Face, lines []string,
	x0, y0, lineH, boxW, ascent int, align string, step int) {
	if step < 1 {
		step = 1
	}
	for i, line := range lines {
		lineW := font.MeasureString(face, line).Ceil()
		x := x0
		switch strings.ToLower(align) {
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
		y := y0 + ascent + i*lineH*step
		d := &font.Drawer{
			Dst:  dst,
			Src:  image.NewUniform(color.Black),
			Face: face,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(line)
	}
}

// mmToDots переводит миллиметры в точки печати.
func mmToDots(mm float64) int {
	return int(math.Round(mm * dotsPerMM))
}

// mmToDotsMin переводит миллиметры в точки, но не меньше одной:
// тонкая рамка 0,2 мм должна остаться видимой.
func mmToDotsMin(mm float64) int {
	n := mmToDots(mm)
	if n < 1 {
		n = 1
	}
	return n
}

// fillRect заливает прямоугольник чёрным, обрезая по краям холста.
func fillRect(img *image.Gray, x, y, w, h int) {
	r := image.Rect(x, y, x+w, y+h).Intersect(img.Bounds())
	if r.Empty() {
		return
	}
	draw.Draw(img, r, image.NewUniform(color.Black), image.Point{}, draw.Src)
}

// drawLineElement рисует залитый прямоугольник: разделитель между строками
// или сплошную плашку. Тонкая линия — это тот же прямоугольник с малой высотой.
func drawLineElement(img *image.Gray, el Element) error {
	w, h := mmToDotsMin(el.W), mmToDotsMin(el.H)
	if el.W <= 0 || el.H <= 0 {
		return fmt.Errorf("у линии не заданы размеры (w и h в мм)")
	}
	fillRect(img, mmToDots(el.X), mmToDots(el.Y), w, h)
	return nil
}

// drawFrameElement рисует рамку: четыре полосы по сторонам прямоугольника.
func drawFrameElement(img *image.Gray, el Element) error {
	if el.W <= 0 || el.H <= 0 {
		return fmt.Errorf("у рамки не заданы размеры (w и h в мм)")
	}
	t := el.Thickness
	if t <= 0 {
		t = 0.3
	}
	x, y := mmToDots(el.X), mmToDots(el.Y)
	w, h, tw := mmToDots(el.W), mmToDots(el.H), mmToDotsMin(t)

	fillRect(img, x, y, w, tw)      // верх
	fillRect(img, x, y+h-tw, w, tw) // низ
	fillRect(img, x, y, tw, h)      // левая
	fillRect(img, x+w-tw, y, tw, h) // правая
	return nil
}

// rotateGray поворачивает изображение на 90, 180 или 270 градусов.
func rotateGray(src *image.Gray, angle int) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	switch ((angle % 360) + 360) % 360 {
	case 90:
		dst := image.NewGray(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				// по часовой стрелке
				dst.SetGray(h-1-y, x, src.GrayAt(x, y))
			}
		}
		return dst
	case 180:
		dst := image.NewGray(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetGray(w-1-x, h-1-y, src.GrayAt(x, y))
			}
		}
		return dst
	case 270:
		dst := image.NewGray(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetGray(y, w-1-x, src.GrayAt(x, y))
			}
		}
		return dst
	default:
		return src
	}
}

// pickFont выбирает файл шрифта для элемента.
//
// Пустое семейство — встроенный шрифт по умолчанию. Если названного
// семейства в системе нет, это ошибка: молча подменить шрифт нельзя,
// иначе этикетка выйдет не такой, как задумано, и заметит это только
// человек, глядя на бумагу.
func pickFont(family string, bold bool) (string, error) {
	if strings.TrimSpace(family) == "" {
		return findFontFile()
	}
	path, ok := resolveFontFamily(family, bold)
	if !ok {
		return "", fmt.Errorf("шрифт %q не найден в системе — выберите другой в конструкторе", family)
	}
	return path, nil
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

	angle := ((el.Rotate % 360) + 360) % 360
	if angle == 90 || angle == 270 {
		dw, dh = dh, dw // поворот меняет местами ширину и высоту
	}
	scaled := image.NewGray(image.Rect(0, 0, dw, dh))
	drawScaled(scaled, src)
	if angle != 0 {
		scaled = rotateGray(scaled, angle)
	}

	x0 := int(math.Round(el.X * dotsPerMM))
	y0 := int(math.Round(el.Y * dotsPerMM))
	draw.Draw(img, image.Rect(x0, y0, x0+scaled.Bounds().Dx(), y0+scaled.Bounds().Dy()),
		scaled, image.Point{}, draw.Src)
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

// Подстановки даты и времени. Порядок важен: сначала самая длинная,
// иначе «{дата-время}» превратится в «18.09.2026-время».
var (
	reDateTime = regexp.MustCompile(`(?i)\{дата-время\}`)
	reDate     = regexp.MustCompile(`(?i)\{дата\}`)
	reTime     = regexp.MustCompile(`(?i)\{время\}`)
)

// substituteFields заменяет поля данных и дату со временем.
//
// Столбцы данных: {1}, {2}… Дата и время подставляются в момент печати:
// {дата} → 18.09.2026, {время} → 00:20, {дата-время} → 18.09.2026 00:20.
//
// Ссылка на столбец, которого нет, — ошибка: иначе на этикетке напечаталось
// бы литеральное «{3}», и брак был бы виден только на бумаге.
func substituteFields(text string, row []string, now time.Time) (string, error) {
	text = reDateTime.ReplaceAllString(text, now.Format("02.01.2006 15:04"))
	text = reDate.ReplaceAllString(text, now.Format("02.01.2006"))
	text = reTime.ReplaceAllString(text, now.Format("15:04"))

	for i, v := range row {
		text = strings.ReplaceAll(text, fmt.Sprintf("{%d}", i+1), v)
	}
	if bad := rePlaceholder.FindString(text); bad != "" {
		return "", fmt.Errorf(
			"элемент использует %s, а в данных только %d столбц(а/ов)", bad, len(row))
	}
	return text, nil
}

// drawSmilesElement рисует химическую структуру по строке SMILES.
//
// Поворот на 90° нужен не для красоты: высота этикетки всего 12 мм, и
// квадратная структура упирается в неё. Повёрнутая может занять ВСЮ длину
// этикетки — то есть стать в два с лишним раза крупнее.
func drawSmilesElement(img *image.Gray, el Element, row []string, now time.Time) error {
	if el.W <= 0 || el.H <= 0 {
		return fmt.Errorf("у структуры не заданы размеры (w и h в мм)")
	}
	smiles, err := substituteFields(el.Smiles, row, now)
	if err != nil {
		return err
	}
	bond := el.Thickness
	if bond <= 0 {
		bond = 2 // тонкие связи при 96 точках пропадают — проверено
	}

	// Поворот делает сам RDKit: он раскладывает молекулу под нужную рамку,
	// а не вписывает её в квадрат и потом крутит. Так структура занимает
	// всю отведённую площадь — важно, когда высота этикетки всего 12 мм.
	angle := ((el.Rotate % 360) + 360) % 360
	st, err := RenderSmiles(smiles, mmToDotsMin(el.W), mmToDotsMin(el.H), angle, bond)
	if err != nil {
		return err
	}

	// Размещаем по фактическому размеру с учётом выравнивания.
	x0, y0 := mmToDots(el.X), mmToDots(el.Y)
	stW, stH := st.Bounds().Dx(), st.Bounds().Dy()
	switch strings.ToLower(el.Align) {
	case "center":
		x0 += (mmToDots(el.W) - stW) / 2
	case "right":
		x0 += mmToDots(el.W) - stW
	}
	draw.Draw(img, image.Rect(x0, y0, x0+stW, y0+stH), st, image.Point{}, draw.Src)
	return nil
}
