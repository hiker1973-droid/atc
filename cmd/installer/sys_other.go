//go:build !windows

package main

import "os/exec"

// The installer is for Windows; these let the package build elsewhere.
func hideWindow(*exec.Cmd)           {}
func screenSize() (w, h int)         { return 0, 0 }
func rememberedDir() string          { return "" }
func rememberDir(string)             {}
func pickFolder(start string) string { return "" }
