//go:build !webview

// Сборка по умолчанию: интерфейс открывается в браузере.
//
// Так программа остаётся без CGO — её можно собрать под Windows одной командой
// с любой машины. Нативное окно включается отдельно: go build -tags webview.
package main

import "fmt"

// nativeWindowAvailable сообщает, умеет ли эта сборка открывать своё окно.
func nativeWindowAvailable() bool { return false }

// runGUIWindow открывает интерфейс и ждёт, пока пользователь не остановит
// программу. Блокирует вызывающего.
func runGUIWindow(url string, appMode bool) error {
	if appMode && openAppWindow(url) {
		fmt.Println("Открываю отдельным окном (режим приложения)")
	} else {
		if appMode {
			fmt.Println("Браузера на Chromium не нашлось — открываю обычный")
		}
		openBrowser(url)
	}
	fmt.Println("Остановить — Ctrl+C")
	select {} // ждём прерывания
}
