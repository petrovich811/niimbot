// Драйвер принтера этикеток NIIMBOT N1 (Bluetooth LE) для Linux.
//
// Использование:
//
//	niimbot info                 # подключиться и показать состояние
//	niimbot text "Насос" "12А"   # напечатать текст
//	niimbot image file.png       # напечатать картинку
//	niimbot testpage             # встроенная тестовая страница
//	niimbot scan                 # найти принтер
//
// По умолчанию печатается этикетка 14×30 мм (12 мм поперёк головки),
// текст идёт вдоль этикетки. Флаг --flip переворачивает содержимое,
// если этикетка выходит «вверх ногами».
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"time"
)

const defaultMAC = "24:0A:11:D9:EC:C3" // N1-GA24110447

// Известные флаги с указанием, принимают ли они значение.
var flagSpec = map[string]bool{
	"address": true, "v": true, "length": true, "font": true, "flip": false,
	"density": true, "label": true, "copies": true, "no-wait": false,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	// Разделяем флаги и позиционные аргументы вручную: флаги могут стоять
	// в любом месте, а подкоманда — первый позиционный аргумент
	// (пакет flag на это не способен).
	flagArgs, posArgs := splitArgs(os.Args[1:])
	if len(posArgs) == 0 {
		usage()
		os.Exit(2)
	}
	cmd := posArgs[0]
	posArgs = posArgs[1:]

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	addr := fs.String("address", "", "MAC принтера (по умолчанию — поиск по имени N1-)")
	verbose := fs.Bool("v", true, "подробный вывод")
	length := fs.Float64("length", 30.0, "длина этикетки, мм")
	fontPt := fs.Float64("font", 0, "кегль в точках (0 = подобрать автоматически)")
	flip := fs.Bool("flip", false, "перевернуть содержимое на 180°")
	density := fs.Int("density", 2, "плотность 1..3")
	label := fs.String("label", "", "тип этикетки; пусто — определить по метке рулона")
	copies := fs.Int("copies", 1, "количество копий")
	if err := fs.Parse(flagArgs); err != nil {
		fatal(err)
	}

	// Тип этикетки проверяем ДО подключения: незачем будить принтер,
	// если задача заведомо невыполнима.
	if (cmd == "text" || cmd == "image") && *label != "" {
		if err := checkLabelType(*label); err != nil {
			fatal(err)
		}
	}

	switch cmd {
	case "scan":
		mac, err := ScanPrinter("N1-", 12*time.Second, *verbose)
		if err != nil {
			fatal(err)
		}
		fmt.Println(mac)
		return
	case "preview":
		// Рендер без принтера: удобно проверить макет до печати.
		if len(posArgs) == 0 {
			fatal(fmt.Errorf("не задан текст"))
		}
		img, err := RenderText(posArgs, *length, *fontPt)
		if err != nil {
			fatal(err)
		}
		savePreview(img)
		rows := ImageToRows(img, *flip)
		ink := 0
		for _, r := range rows {
			if r != nil {
				ink++
			}
		}
		fmt.Printf("строк с печатью: %d из %d\n", ink, len(rows))
		return
	case "info", "rfid", "testpage", "text", "image":
	default:
		usage()
		os.Exit(2)
	}

	mac := *addr
	if mac == "" {
		found, err := ScanPrinter("N1-", 12*time.Second, *verbose)
		if err == nil {
			mac = found
		} else {
			mac = defaultMAC
			fmt.Fprintf(os.Stderr, "поиск не удался (%v), пробую %s\n", err, mac)
		}
	}

	p, err := ConnectMAC(mac, *verbose)
	if err != nil {
		fatal(err)
	}
	defer p.Close()

	info, err := p.Handshake()
	if err != nil {
		fatal(err)
	}

	switch cmd {
	case "info":
		printInfo(info)
		printRfid(p, false)
		printRfid(p, true)

	case "rfid":
		printRfid(p, false)
		printRfid(p, true)

	case "testpage":
		if err := p.PrintTestPage(); err != nil {
			fatal(err)
		}
		fmt.Println("тестовая страница отправлена")

	case "text":
		if len(posArgs) == 0 {
			fatal(fmt.Errorf("не задан текст"))
		}
		labelName, err := resolveLabel(p, *label)
		if err != nil {
			fatal(err)
		}
		img, err := RenderText(posArgs, *length, *fontPt)
		if err != nil {
			fatal(err)
		}
		savePreview(img)
		if err := printRows(p, ImageToRows(img, *flip), *density, labelName, *copies); err != nil {
			fatal(err)
		}

	case "image":
		if len(posArgs) < 1 {
			fatal(fmt.Errorf("не задан файл картинки"))
		}
		labelName, err := resolveLabel(p, *label)
		if err != nil {
			fatal(err)
		}
		img, err := LoadImageFile(posArgs[0])
		if err != nil {
			fatal(err)
		}
		if err := printRows(p, ImageToRows(img, *flip), *density, labelName, *copies); err != nil {
			fatal(err)
		}
	}
}

// splitArgs раскладывает аргументы на флаги и позиционные значения.
func splitArgs(args []string) (flags []string, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" || a == "--" {
			positional = append(positional, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		hasInline := strings.Contains(name, "=")
		base := strings.SplitN(name, "=", 2)[0]
		flags = append(flags, "-"+name)
		if !hasInline && flagSpec[base] {
			// значение идёт следующим аргументом
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		}
	}
	return
}

// resolveLabel определяет тип этикетки для печати.
//
// Если флаг --label задан явно — берём его (и предупреждаем, если метка
// рулона говорит другое). Если не задан — читаем метку рулона: принтер
// сам знает, что в него вставлено, и это надёжнее любой догадки.
func resolveLabel(p *Printer, flagValue string) (string, error) {
	if flagValue != "" {
		if err := checkLabelType(flagValue); err != nil {
			return "", err
		}
		if detected, ok := detectLabel(p); ok {
			if labelTypes[flagValue] != detected {
				fmt.Fprintf(os.Stderr,
					"внимание: в принтере этикетки типа %s, а печатаем как %s\n",
					labelTypeName(detected), flagValue)
			}
		}
		return flagValue, nil
	}

	if detected, ok := detectLabel(p); ok {
		name := labelTypeName(detected)
		if name == "" {
			return "", fmt.Errorf("в метке рулона неизвестный тип этикетки (%d) — задайте --label вручную", detected)
		}
		if _, supported := labelTypesN1[detected]; !supported {
			return "", fmt.Errorf("в метке рулона тип %s, который N1 не поддерживает — задайте --label вручную", name)
		}
		fmt.Printf("тип этикетки определён по метке рулона: %s\n", name)
		return name, nil
	}

	fmt.Fprintln(os.Stderr, "метка рулона не прочитана — печатаю как withgaps (обычные этикетки)")
	return "withgaps", nil
}

// detectLabel читает тип этикетки из метки рулона.
func detectLabel(p *Printer) (byte, bool) {
	info, err := p.ReadRfid(false)
	if err != nil || !info.TagPresent || !info.LabelTypeOK {
		return 0, false
	}
	return info.LabelType, true
}

// labelTypeName возвращает имя типа по коду.
func labelTypeName(id byte) string {
	for name, v := range labelTypes {
		if v == id {
			return name
		}
	}
	return ""
}

// checkLabelType проверяет имя типа этикетки и поддержку его моделью N1.
func checkLabelType(label string) error {
	id, ok := labelTypes[label]
	if !ok {
		return fmt.Errorf("неизвестный тип этикетки %q.\nПоддерживаются:%s", label, labelListN1())
	}
	// Протокол знает восемь типов, но N1 заявляет только пять.
	if _, ok := labelTypesN1[id]; !ok {
		return fmt.Errorf("N1 не поддерживает тип %q.\nПоддерживаются:%s", label, labelListN1())
	}
	return nil
}

func printRows(p *Printer, rows []Row, density int, label string, copies int) error {
	if err := checkLabelType(label); err != nil {
		return err
	}
	if err := p.PrintRows(rows, byte(density), labelTypes[label], copies); err != nil {
		return err
	}
	fmt.Println("готово")
	return nil
}

// printRfid показывает данные метки рулона.
//
// ribbon=false — рулон этикеток, ribbon=true — лента (риббон).
// Поле типа у ленты — не тип этикетки, а тип расходника, поэтому его
// показываем отдельно и не сверяем со списком типов для N1.
func printRfid(p *Printer, ribbon bool) {
	title := "рулон этикеток"
	if ribbon {
		title = "лента (риббон)"
	}
	fmt.Printf("\n=== %s ===\n", title)

	info, err := p.ReadRfid(ribbon)
	if err != nil {
		fmt.Printf("  метка не прочитана: %v\n", err)
		return
	}
	if !info.TagPresent {
		fmt.Println("  метки нет — рулон без RFID либо не вставлен")
		return
	}

	fmt.Printf("  %-22s %s\n", "UUID:", info.UUID)
	if info.Barcode != "" {
		fmt.Printf("  %-22s %s\n", "штрихкод:", info.Barcode)
	}
	if info.Serial != "" {
		fmt.Printf("  %-22s %s\n", "серийный номер:", info.Serial)
	}

	if info.LabelTypeOK {
		name := info.LabelTypeName()
		if !ribbon && name != "" {
			note := "N1 не поддерживает"
			if _, ok := labelTypesN1[info.LabelType]; ok {
				note = "N1 поддерживает"
			}
			fmt.Printf("  %-22s %s (%d) — %s\n", "тип этикетки:", name, info.LabelType, note)
		} else {
			fmt.Printf("  %-22s %d\n", "тип расходника:", info.LabelType)
		}
	}

	if info.AllPaper >= 0 && info.UsedPaper >= 0 {
		fmt.Printf("  %-22s %d, израсходовано %d, осталось %d\n",
			"ресурс:", info.AllPaper, info.UsedPaper, info.Leftover())
	}
	if info.HasCapacity {
		fmt.Printf("  %-22s %d\n", "ёмкость:", info.Capacity)
	}
}

func printInfo(i Info) {
	fmt.Println("\n=== принтер ===")
	fmt.Printf("  %-28s %d (2 = ConnectedNew)\n", "рукопожатие:", i.ConnectResult)
	fmt.Printf("  %-28s %d\n", "id модели:", i.ModelID)
	if i.ModelID == modelIDN1 {
		fmt.Println("  ✅ это N1 — параметры драйвера совпадают")
	} else {
		fmt.Println("  ⚠️  id не совпадает с N1 (3586) — проверьте параметры головки")
	}
	if i.Serial != "" {
		fmt.Printf("  %-28s %s\n", "серийный номер:", i.Serial)
	}
	fmt.Printf("  %-28s %d\n", "плотность:", i.Density)
	fmt.Printf("  %-28s %d\n", "тип этикетки:", i.LabelType)
	fmt.Printf("  %-28s %d\n", "заряд (ступень):", i.Battery)
	if i.Firmware != "" {
		fmt.Printf("  %-28s %s\n", "прошивка:", i.Firmware)
	}
}

// savePreview сохраняет макет — удобно посмотреть, что уйдёт на печать.
func savePreview(img *image.Gray) {
	const path = "/tmp/niimbot_label_preview.png"
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return
	}
	b := img.Bounds()
	fmt.Printf("макет: %s (%d×%d точек = %.1f×%.1f мм)\n",
		path, b.Dx(), b.Dy(), float64(b.Dx())/dotsPerMM, float64(b.Dy())/dotsPerMM)
}

func usage() {
	fmt.Fprint(os.Stderr, `Драйвер NIIMBOT N1

  niimbot scan                  найти принтер по Bluetooth
  niimbot preview "строка"      отрисовать макет без печати
  niimbot info                  показать состояние принтера
  niimbot rfid                  прочитать метки рулона и ленты
  niimbot text "строка" "..."   напечатать текст
  niimbot image file.png        напечатать картинку
  niimbot testpage              встроенная тестовая страница

Флаги: --address MAC   --length мм   --font точек   --density 1..3
       --copies N   --flip   -v=false

Типы этикеток (N1 поддерживает пять из восьми типов протокола):
  --label withgaps       этикетки с зазорами — обычные (по умолчанию)
  --label continuous     непрерывная лента
  --label transparent    прозрачные
  --label blackmarkgap   с чёрной меткой
  --label heatshrink     термоусадочная трубка

Протокол знает ещё black, perforated и pvctag, но прошивка N1 их не поддерживает.
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ошибка:", err)
	os.Exit(1)
}
