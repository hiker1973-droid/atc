package main

import (
	"syscall"
	"unsafe"
)

// setTitle names the console window (the taskbar shows it beside the icon).
func setTitle(s string) {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return
	}
	syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleTitleW").Call(uintptr(unsafe.Pointer(p)))
}
