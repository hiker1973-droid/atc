package main

import (
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
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

// regKey is where the installed copy is remembered (the same key as the
// trial record, pkg/license).
const regKey = `Software\Readback ATC`

// rememberedDir is where Readback was last installed, or "".
func rememberedDir() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("InstallDir")
	return v
}

// rememberDir records the install folder ("" forgets it).
func rememberDir(dir string) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	if dir == "" {
		k.DeleteValue("InstallDir")
		return
	}
	k.SetStringValue("InstallDir", dir)
}

// pickFolder shows Windows' folder picker, on top of the setup window, and
// returns the folder chosen ("" if cancelled).
func pickFolder(start string) string {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := `Add-Type -AssemblyName System.Windows.Forms; ` +
		`$o = New-Object System.Windows.Forms.Form -Property @{TopMost = $true; ShowInTaskbar = $false}; ` +
		`$d = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
		`$d.Description = 'Choose where to install Readback ATC'; $d.ShowNewFolderButton = $true; ` +
		`$d.SelectedPath = ` + q(start) + `; ` +
		`if ($d.ShowDialog($o) -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($d.SelectedPath) }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", script)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
