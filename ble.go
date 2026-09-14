// Подключение к принтеру по Bluetooth LE.
//
// Принтер N1 отдаёт ДВА сервиса с похожими свойствами:
//   - e7810a71-73ae-499d-8c15-faa9aef0c3f2 — «родной» сервис NIIMBOT,
//     именно его слушает прошивка (характеристика bef8d6c9-…);
//   - 49535343-fe7d-4ae5-8fa9-9fafd205e455 — прозрачный UART, на команды
//     не отвечает.
//
// Проверено на живом N1: если подключиться к UART, ответов нет вообще.
package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const niimbotCharUUID = "bef8d6c9-9c21-4c9e-b632-bd58c1009f9f"

type waiter struct {
	want []byte
	ch   chan Packet
}

// Printer — подключённый принтер.
type Printer struct {
	device bluetooth.Device
	char   bluetooth.DeviceCharacteristic

	mu        sync.Mutex
	buf       []byte
	waiters   []waiter
	lastError byte // код из ответа 0xdb, 0 если ошибки не было

	verbose bool
}

// printErrors — расшифровка кодов ошибки печати (ответ 0xdb).
var printErrors = map[byte]string{
	0x01: "открыта крышка",
	0x02: "нет бумаги",
	0x03: "низкий заряд батареи",
	0x04: "исключение батареи",
	0x05: "отменено пользователем",
	0x06: "ошибка данных (принтер не успел принять — добавьте паузу между строками)",
	0x07: "перегрев",
	0x08: "бумага закончилась",
	0x09: "принтер занят",
	0x0a: "нет печатающей головки",
	0x0b: "низкая температура",
	0x0c: "головка не закреплена",
	0x0d: "нет ленты (риббона)",
	0x0e: "неподходящая лента",
}

func (p *Printer) log(format string, args ...any) {
	if p.verbose {
		fmt.Printf(format+"\n", args...)
	}
}

// ConnectMAC подключается по MAC-адресу.
//
// Перед подключением обязательно сканируем: BlueZ должен «увидеть»
// устройство, иначе он отвечает ошибкой D-Bus
// («Method "Get" … doesn't exist») — проверено.
func ConnectMAC(mac string, verbose bool) (*Printer, error) {
	addr, err := makeAddress(mac)
	if err != nil {
		return nil, fmt.Errorf("не разобрать адрес принтера %q: %w", mac, err)
	}
	var adapter = bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, fmt.Errorf("не удалось включить Bluetooth-адаптер: %w", err)
	}
	p := &Printer{verbose: verbose}

	if err := p.warmUp(adapter, mac); err != nil {
		return nil, err
	}

	p.log("Подключаюсь к %s …", mac)

	// Подключение выносим в горутину с таймаутом: у Linux-бэкенда вызов
	// может «зависнуть» на D-Bus, если устройство не отвечает (например,
	// принтер не в режиме подключения).
	type res struct {
		dev bluetooth.Device
		err error
	}
	ch := make(chan res, 1)
	go func() {
		d, e := adapter.Connect(addr, bluetooth.ConnectionParams{
			ConnectionTimeout: bluetooth.NewDuration(20 * time.Second),
		})
		ch <- res{d, e}
	}()

	var dev bluetooth.Device
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("подключение: %w", r.err)
		}
		dev = r.dev
	case <-time.After(30 * time.Second):
		go adapter.StopScan() // не даём сканированию держать адаптер
		return nil, fmt.Errorf("подключение к %s не удалось за 30 с — "+
			"принтер включён и не занят другим устройством?", mac)
	}

	p.device = dev
	p.log("  соединение установлено")
	if err := p.setup(); err != nil {
		dev.Disconnect()
		return nil, err
	}
	return p, nil
}

// warmUp сканирует эфир, пока не найдёт нужный адрес (или не истечёт таймаут).
//
// Важно: Scan у Linux-бэкенда блокируется до StopScan, поэтому запускаем
// его в отдельной горутине, а выходим по сигналу из колбэка.
func (p *Printer) warmUp(adapter *bluetooth.Adapter, mac string) error {
	want := strings.ToUpper(mac)
	found := make(chan string, 1)
	var once sync.Once
	p.log("Сканирую, чтобы BlueZ увидел принтер …")

	go func() {
		// Ошибку здесь не показываем: сканирование останавливаем мы сами,
		// и библиотека сообщает об этом как о неожиданном завершении.
		// Раньше это выглядело как сбой — «сканирование прервано».
		_ = adapter.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
			if strings.EqualFold(r.Address.String(), want) {
				once.Do(func() {
					p.log("  принтер на связи: %s (%s)", r.LocalName(), r.Address.String())
					found <- r.Address.String()
				})
			}
		})
	}()

	select {
	case <-found:
	case <-time.After(12 * time.Second):
		stopScan(adapter)
		return fmt.Errorf("принтер %s не найден в эфире — включите его и повторите", mac)
	}
	stopScan(adapter)
	time.Sleep(300 * time.Millisecond) // дать BlueZ дозаписать свойства
	return nil
}

// stopScan останавливает сканирование, не блокируя вызывающего:
// у Linux-бэкенда StopScan может ждать завершения цикла D-Bus.
func stopScan(adapter *bluetooth.Adapter) {
	done := make(chan struct{})
	go func() {
		adapter.StopScan()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// ScanPrinter ищет принтер по имени (например, «N1-»).
func ScanPrinter(nameHint string, timeout time.Duration, verbose bool) (string, error) {
	var adapter = bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return "", fmt.Errorf("не удалось включить Bluetooth-адаптер: %w", err)
	}
	type found struct {
		addr string
		rssi int16
		name string
	}
	best := found{rssi: -32768}
	done := make(chan struct{})
	var once sync.Once

	// Scan блокируется до StopScan — держим его в горутине.
	go func() {
		err := adapter.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
			name := r.LocalName()
			if name == "" {
				return
			}
			if strings.Contains(strings.ToLower(name), strings.ToLower(nameHint)) {
				if r.RSSI > best.rssi {
					best = found{addr: r.Address.String(), rssi: r.RSSI, name: name}
				}
				once.Do(func() { close(done) })
			}
		})
		if verbose && err != nil {
			fmt.Fprintf(os.Stderr, "  сканирование прервано: %v\n", err)
		}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
	}
	stopScan(adapter)

	if best.addr == "" {
		return "", fmt.Errorf("принтер %q не найден", nameHint)
	}
	if verbose {
		fmt.Printf("  найден %s  %s  RSSI %d\n", best.name, best.addr, best.rssi)
	}
	return best.addr, nil
}

// setup находит сервис Niimbot и включает уведомления.
func (p *Printer) setup() error {
	services, err := p.device.DiscoverServices(nil)
	if err != nil {
		return fmt.Errorf("поиск сервисов: %w", err)
	}

	var target *bluetooth.DeviceCharacteristic
	for i := range services {
		uuid := strings.ToLower(services[i].UUID().String())
		chars, err := services[i].DiscoverCharacteristics(nil)
		if err != nil {
			continue
		}
		for j := range chars {
			cu := strings.ToLower(chars[j].UUID().String())
			if cu == niimbotCharUUID {
				target = &chars[j]
				break
			}
			// запасной вариант — первый же характеристика в родном сервисе
			if target == nil && strings.HasPrefix(uuid, bleServiceUUID[:8]) {
				target = &chars[j]
			}
		}
		if target != nil && strings.ToLower(target.UUID().String()) == niimbotCharUUID {
			break
		}
	}
	if target == nil {
		return fmt.Errorf("не нашёл характеристику принтера (нужен сервис %s)", bleServiceUUID)
	}
	p.char = *target
	p.log("  характеристика: %s", p.char.UUID().String())

	return p.char.EnableNotifications(func(buf []byte) {
		p.mu.Lock()
		p.buf = append(p.buf, buf...)
		pkts := parsePackets(&p.buf)
		p.mu.Unlock()
		for _, pkt := range pkts {
			p.log("  ← %s", pkt.String())
			// Ошибку печати (0xdb) запоминаем: иначе она проходит незаметно
			// и драйвер молча ждёт до таймаута.
			if pkt.Cmd == respPrintError && len(pkt.Data) > 0 {
				p.mu.Lock()
				p.lastError = pkt.Data[0]
				p.mu.Unlock()
				why := printErrors[pkt.Data[0]]
				if why == "" {
					why = "неизвестный код"
				}
				p.log("  ⚠️ принтер сообщил об ошибке: %s (0x%02x)", why, pkt.Data[0])
			}
			p.mu.Lock()
			ws := p.waiters
			p.mu.Unlock()
			for i, w := range ws {
				if w.want == nil || containsByte(w.want, pkt.Cmd) {
					p.mu.Lock()
					if i < len(p.waiters) {
						p.waiters = append(p.waiters[:i], p.waiters[i+1:]...)
					}
					p.mu.Unlock()
					select {
					case w.ch <- pkt:
					default:
					}
					break
				}
			}
		}
	})
}

func containsByte(set []byte, v byte) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

// Send отправляет пакет. want — список ожидаемых кодов ответа (nil = не ждать).
func (p *Printer) Send(cmd byte, data []byte, want []byte, timeout time.Duration) (*Packet, error) {
	if data == nil {
		data = []byte{1}
	}
	var ch chan Packet
	if want != nil {
		ch = make(chan Packet, 1)
		p.mu.Lock()
		p.waiters = append(p.waiters, waiter{want: want, ch: ch})
		p.mu.Unlock()
	}
	p.log("  → 0x%02x %x", cmd, data)
	if _, err := p.char.WriteWithoutResponse(buildPacket(cmd, data)); err != nil {
		return nil, fmt.Errorf("отправка 0x%02x: %w", cmd, err)
	}
	if want == nil {
		return nil, nil
	}
	select {
	case pkt := <-ch:
		return &pkt, nil
	case <-time.After(timeout):
		p.log("  ! нет ответа на 0x%02x за %s", cmd, timeout)
		return nil, nil
	}
}

func (p *Printer) Close() error {
	p.Send(cmdPrintEnd, nil, nil, 0)
	return p.device.Disconnect()
}

// Info — сведения о принтере.
type Info struct {
	ConnectResult byte
	ModelID       int
	Serial        string
	Density       int
	LabelType     int
	Battery       int
	Firmware      string
}

// Handshake — рукопожатие и чтение параметров.
func (p *Printer) Handshake() (Info, error) {
	var info Info
	pkt, err := p.Send(cmdConnect, nil, []byte{respConnect}, 5*time.Second)
	if err != nil {
		return info, err
	}
	if pkt != nil && len(pkt.Data) > 0 {
		info.ConnectResult = pkt.Data[0]
		p.log("  рукопожатие: %d (2 = ConnectedNew)", info.ConnectResult)
	}

	get := func(t byte) *Packet {
		resp := infoResponse[t]
		pkt, _ := p.Send(cmdPrinterInfo, []byte{t}, []byte{resp}, 3*time.Second)
		return pkt
	}
	if pkt := get(infoModelID); pkt != nil && len(pkt.Data) >= 2 {
		info.ModelID = int(pkt.Data[0])<<8 | int(pkt.Data[1])
	}
	if pkt := get(infoSerial); pkt != nil {
		info.Serial = strings.Trim(string(pkt.Data), "\x00")
	}
	if pkt := get(infoDensity); pkt != nil && len(pkt.Data) > 0 {
		info.Density = int(pkt.Data[0])
	}
	if pkt := get(infoLabelType); pkt != nil && len(pkt.Data) > 0 {
		info.LabelType = int(pkt.Data[0])
	}
	if pkt := get(infoBattery); pkt != nil && len(pkt.Data) > 0 {
		info.Battery = int(pkt.Data[0])
	}
	if pkt := get(infoFirmware); pkt != nil {
		parts := make([]string, 0, len(pkt.Data))
		for _, b := range pkt.Data {
			parts = append(parts, fmt.Sprintf("%d", b))
		}
		info.Firmware = strings.Join(parts, ".")
	}
	return info, nil
}

// PrintTestPage печатает встроенную тестовую страницу.
func (p *Printer) PrintTestPage() error {
	_, err := p.Send(cmdPrintTestPage, nil, nil, 0)
	return err
}

// takeError возвращает код ошибки печати и сбрасывает его.
func (p *Printer) takeError() byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.lastError
	p.lastError = 0
	return e
}

// errorText расшифровывает код ошибки печати.
func errorText(code byte) string {
	if t := printErrors[code]; t != "" {
		return t
	}
	return fmt.Sprintf("неизвестная ошибка (код %d)", code)
}

// sendRows отправляет строки одной этикетки.
//
// Между строками обязательна пауза: writeWithoutResponse отдаёт данные
// быстрее, чем принтер успевает их принять, и он отвечает ошибкой
// PrinterErrorCode.DataError (0xdb, код 6). Проверено на живом N1 — без
// паузы серия срывается. В черновике на Python пауза была 4 мс.
func (p *Printer) sendRows(rows []Row) error {
	const rowDelay = 4 * time.Millisecond
	for i, row := range rows {
		if row == nil {
			p.Send(cmdPrintEmptyRow, append(u16(i), 1), nil, 0)
			time.Sleep(rowDelay)
			continue
		}
		_, parts := countPixels(row, printheadPixels)
		payload := append(u16(i), parts[:]...)
		payload = append(payload, 1) // repeats
		payload = append(payload, row...)
		if err := p.sendBitmapRow(payload); err != nil {
			return err
		}
		time.Sleep(rowDelay)
	}
	return nil
}

// beginJob готовит задание печати на total страниц.
func (p *Printer) beginJob(total int, density, labelType byte) {
	p.Send(cmdSetDensity, []byte{density}, []byte{respSetDensity}, 2*time.Second)
	p.Send(cmdSetLabelType, []byte{labelType}, nil, 0)
	// printStart: страницы(2) + 4 нуля + цвет
	p.Send(cmdPrintStart, append(u16(total), 0, 0, 0, 0, 0), nil, 0)
	time.Sleep(150 * time.Millisecond)
}

// beginPage открывает страницу и задаёт её размер. copies — копий на странице.
func (p *Printer) beginPage(height, copies int) {
	p.Send(cmdPageStart, nil, []byte{respPageStart}, 2*time.Second)
	pageSize := append(u16(height), u16(printheadPixels)...)
	pageSize = append(pageSize, u16(copies)...)
	p.Send(cmdSetPageSize, pageSize, nil, 0)
	time.Sleep(60 * time.Millisecond)
}

// PrintRows печатает одну этикетку в заданном числе копий.
func (p *Printer) PrintRows(rows []Row, density, labelType byte, copies int) error {
	height := len(rows)
	p.log("  печать: %d строк, головка %d точек, плотность %d, тип этикетки %d",
		height, printheadPixels, density, labelType)

	p.beginJob(copies, density, labelType)
	p.beginPage(height, copies)
	if err := p.sendRows(rows); err != nil {
		return err
	}
	p.Send(cmdPageEnd, nil, []byte{respPageEnd}, 3*time.Second)
	return p.waitFinished(copies)
}

// PrintPages печатает серию этикеток ОДНИМ заданием: одна страница на этикетку.
//
// Так быстрее и аккуратнее, чем отдельное задание на каждую этикетку:
// принтер не останавливается между ними. Копии разворачиваются в страницы —
// то есть при copies=2 этикетки идут парами: A, A, B, B.
//
// progress вызывается после каждой отправленной страницы (done, total).
func (p *Printer) PrintPages(pages [][]Row, density, labelType byte, copies int,
	progress func(done, total int)) error {
	if len(pages) == 0 {
		return nil
	}
	if copies < 1 {
		copies = 1
	}

	var flat [][]Row
	for _, page := range pages {
		for c := 0; c < copies; c++ {
			flat = append(flat, page)
		}
	}
	total := len(flat)
	p.log("  серия: %d этикеток (%d шаблонов × %d копий), плотность %d, тип %d",
		total, len(pages), copies, density, labelType)

	p.beginJob(total, density, labelType)
	t0 := time.Now()
	for i, rows := range flat {
		p.beginPage(len(rows), 1)
		if err := p.sendRows(rows); err != nil {
			return fmt.Errorf("этикетка %d из %d: %w", i+1, total, err)
		}
		p.Send(cmdPageEnd, nil, []byte{respPageEnd}, 3*time.Second)
		// Ждём, пока принтер допечатает эту этикетку: иначе следующая
		// уйдёт в занятый принтер и он ответит ошибкой данных.
		if err := p.waitPageDone(20 * time.Second); err != nil {
			return fmt.Errorf("этикетка %d из %d: %w", i+1, total, err)
		}
		if progress != nil {
			progress(i+1, total)
		}
	}
	p.log("  данные серии отправлены за %.1fs", time.Since(t0).Seconds())
	return p.waitFinished(total)
}

// sendBitmapRow отправляет строку с повтором при переполнении MTU.
func (p *Printer) sendBitmapRow(payload []byte) error {
	// MTU у N1 небольшой; если пакет не влезает, шлём как есть —
	// writeWithoutResponse разобьёт сам, но проверяем ошибку.
	_, err := p.Send(cmdPrintBitmapRow, payload, nil, 0)
	return err
}

// waitPageDone ждёт, пока принтер допечатает текущую страницу.
//
// Без этого следующая страница уходит в тот момент, когда принтер ещё занят
// предыдущей, и он отвечает ошибкой данных (0xdb, код 6). Проверено на живом
// N1: серия без ожидания срывается на второй этикетке.
//
// Требуем ДВА подряд замера «100 % печати и подачи»: сразу после конца
// страницы принтер может ещё отдавать значение от предыдущей.
func (p *Printer) waitPageDone(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	time.Sleep(300 * time.Millisecond) // дать странице начаться
	good := 0
	for time.Now().Before(deadline) {
		if code := p.takeError(); code != 0 {
			return fmt.Errorf("принтер сообщил об ошибке: %s", errorText(code))
		}
		pkt, _ := p.Send(cmdPrintStatus, nil, []byte{respPrintStatus}, 2*time.Second)
		if pkt != nil && len(pkt.Data) >= 4 {
			printed, fed := int(pkt.Data[2]), int(pkt.Data[3])
			if printed >= 100 && fed >= 100 {
				good++
				if good >= 2 {
					return nil
				}
			} else {
				good = 0
			}
		} else {
			good = 0 // принтер занят и не отвечает — значит, ещё печатает
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("страница не завершилась за %s", timeout)
}

// waitFinished опрашивает статус печати (0xa3).
//
// Ответ 0xb3: page(2 байта BE), прогресс печати (0..100), прогресс подачи (0..100).
//
// ВАЖНО: поле «страница» НЕ считает напечатанные страницы. Проверено на живом
// N1: и для одной этикетки, и для серии из двух-трёх оно приходит равным 1.
// Поэтому завершением считаем 100 % печати и подачи, а не номер страницы —
// иначе серия никогда не «завершается» и драйвер висит до таймаута.
func (p *Printer) waitFinished(pages int) error {
	p.log("  ожидаю завершения печати (страниц в задании: %d) …", pages)
	deadline := time.Now().Add(120 * time.Second)
	lastPg, lastPct := -1, -1
	for time.Now().Before(deadline) {
		pkt, err := p.Send(cmdPrintStatus, nil, []byte{respPrintStatus}, 2500*time.Millisecond)
		if err != nil {
			return err
		}
		if code := p.takeError(); code != 0 {
			return fmt.Errorf("принтер сообщил об ошибке: %s", errorText(code))
		}
		if pkt != nil && len(pkt.Data) >= 4 {
			pg := int(pkt.Data[0])<<8 | int(pkt.Data[1])
			printed, fed := int(pkt.Data[2]), int(pkt.Data[3])
			if pg != lastPg || printed != lastPct {
				p.log("    статус: страница %d, печать %d%%, подача %d%%", pg, printed, fed)
				lastPg, lastPct = pg, printed
			}
			if pg >= 1 && printed >= 100 && fed >= 100 {
				p.log("  ✅ печать завершена (страница %d, 100%%)", pg)
				return nil
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("не дождался подтверждения печати (этикетка может быть напечатана)")
}

// ReadRfid читает метку рулона: ribbon=false — этикетки, ribbon=true — лента.
func (p *Printer) ReadRfid(ribbon bool) (RfidInfo, error) {
	cmd, resp := byte(cmdRfidInfo), byte(respRfidInfo)
	what := "этикеток"
	if ribbon {
		cmd, resp = cmdRfidInfo2, respRfidInfo2
		what = "ленты"
	}
	pkt, err := p.Send(cmd, nil, []byte{resp}, 4*time.Second)
	if err != nil {
		return RfidInfo{}, err
	}
	if pkt == nil {
		return RfidInfo{}, fmt.Errorf("принтер не ответил на запрос метки %s", what)
	}
	return parseRfidInfo(pkt.Data), nil
}
