package main

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hideWindow keeps a helper program (PowerShell, winget, cmd) from flashing
// a console window: the installer has none of its own.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// screenSize is the primary screen in pixels, to centre the setup window.
func screenSize() (w, h int) {
	gsm := syscall.NewLazyDLL("user32.dll").NewProc("GetSystemMetrics")
	x, _, _ := gsm.Call(0) // SM_CXSCREEN
	y, _, _ := gsm.Call(1) // SM_CYSCREEN
	return int(x), int(y)
}
