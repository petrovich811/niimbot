//go:build !darwin

package main

import "tinygo.org/x/bluetooth"

// makeAddress разбирает адрес принтера.
//
// На Linux и Windows Bluetooth-адрес — это MAC, и система сама решает,
// публичный он или случайный.
func makeAddress(s string) (bluetooth.Address, error) {
	m, err := bluetooth.ParseMAC(s)
	if err != nil {
		return bluetooth.Address{}, err
	}
	return bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: m}}, nil
}
