//go:build webview

// Сборка с нативным окном: go build -tags webview
//
// Интерфейс открывается собственным окном программы, без браузера вообще.
// Внутри — системный движок: WebKitGTK в Linux, WebView2 в Windows, WKWebView
// в macOS. Наш HTML отсюда не меняется: окно просто загружает тот же
// встроенный сервер.
//
// Требует CGO и заголовков:
//
//	Debian/Ubuntu: sudo apt install libwebkit2gtk-4.1-dev
//
// Поэтому нативное окно — отдельная сборка, а не основная: без него программа
// собирается без CGO и легко кросс-компилируется под Windows.
package main

import (
	"fmt"

	webview "github.com/webview/webview_go"
)

// nativeWindowAvailable сообщает, умеет ли эта сборка открывать своё окно.
func nativeWindowAvailable() bool { return true }

// runGUIWindow открывает собственное окно программы и работает, пока его
// не закроют. Блокирует вызывающего: так требуют оконные библиотеки.
func runGUIWindow(url string, appMode bool) error {
	fmt.Println("Открываю собственным окном (движок системы)")

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("NIIMBOT N1 — печать этикеток")
	w.SetSize(1280, 900, webview.HintNone)
	// Минимум побольше: конструктор с холстом в узком окне неудобен.
	w.SetSize(900, 700, webview.HintMin)
	w.Navigate(url)
	w.Run() // возвращается, когда окно закрыли

	fmt.Println("Окно закрыто")
	return nil
}
