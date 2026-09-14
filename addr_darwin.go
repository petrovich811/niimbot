//go:build darwin

package main

import "tinygo.org/x/bluetooth"

// makeAddress разбирает адрес принтера.
//
// На macOS настоящие Bluetooth-адреса устройств системе недоступны: вместо MAC
// CoreBluetooth выдаёт UUID, и он же возвращается при сканировании. Поэтому
// здесь принимаем именно UUID, а не MAC.
func makeAddress(s string) (bluetooth.Address, error) {
	u, err := bluetooth.ParseUUID(s)
	if err != nil {
		return bluetooth.Address{}, err
	}
	return bluetooth.Address{UUID: u}, nil
}
