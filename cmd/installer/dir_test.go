package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChosenDir(t *testing.T) {
	parent := t.TempDir()
	if got := chosenDir(parent); got != filepath.Join(parent, "Readback ATC") {
		t.Fatalf("a parent folder: %q", got)
	}
	own := filepath.Join(parent, "Readback ATC")
	if got := chosenDir(own); got != own {
		t.Fatalf("a folder named for it: %q", got)
	}
	existing := filepath.Join(parent, "RB")
	os.MkdirAll(existing, 0o755)
	os.WriteFile(filepath.Join(existing, "launcher.exe"), nil, 0o644)
	if got := chosenDir(existing); got != existing {
		t.Fatalf("a folder already holding Readback: %q", got)
	}
}

func TestCheckDir(t *testing.T) {
	ok := filepath.Join(t.TempDir(), "Games", "Readback ATC")
	if err := checkDir(ok); err != nil {
		t.Fatalf("a writable folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ok, ".readback-write-test")); !os.IsNotExist(err) {
		t.Fatal("the write test file was left behind")
	}
	for _, bad := range []string{`relative\path`, `C:\`, filepath.Join(os.Getenv("SystemRoot"), "Readback ATC")} {
		if err := checkDir(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		} else if strings.Contains(bad, "Windows") && !strings.Contains(err.Error(), "Windows") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
