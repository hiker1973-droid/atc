package main

import (
	"os"
	"path/filepath"
	"testing"
)

// An installed copy with nothing saved has no SRS or Tacview address; only a
// vSFG-7 rig (a git checkout) falls back to the built-in LAN defaults.
func TestProbeAddrBlankWhenInstalled(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	oldEnv := userEnv
	defer func() { userEnv = oldEnv }()
	saved := ""
	userEnv = func(string) string { return saved }

	rootDir = t.TempDir() // an installed copy: no .git
	if a := probeAddr("tacview-addr", "SKYEYE_TACVIEW"); a != "" {
		t.Fatalf("installed, nothing saved: %q, want blank", a)
	}
	if tcpProbe("", 0) {
		t.Fatal("a blank address probed as up")
	}
	saved = "192.168.50.10:42674"
	if a := probeAddr("tacview-addr", "SKYEYE_TACVIEW"); a != saved {
		t.Fatalf("installed, saved: %q", a)
	}
	saved = ""
	os.MkdirAll(filepath.Join(rootDir, ".git"), 0o755) // a rig
	if a := probeAddr("tacview-addr", "SKYEYE_TACVIEW"); a != *flagTacviewAddr {
		t.Fatalf("rig: %q, want the default %q", a, *flagTacviewAddr)
	}
}
