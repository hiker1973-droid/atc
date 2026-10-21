package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func TestSwapExeKeepsRollback(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	rootDir = t.TempDir()
	write := func(n, s string) { os.WriteFile(filepath.Join(rootDir, n), []byte(s), 0o644) }
	read := func(n string) string { b, _ := os.ReadFile(filepath.Join(rootDir, n)); return string(b) }
	write("atc.exe", "v1")
	write("atc.new.exe", "v2")
	if err := swapExe("atc.exe", "atc.new.exe", "atc.prev.exe"); err != nil {
		t.Fatal(err)
	}
	if read("atc.exe") != "v2" || read("atc.prev.exe") != "v1" {
		t.Fatalf("after swap: atc=%q prev=%q", read("atc.exe"), read("atc.prev.exe"))
	}
	if _, err := os.Stat(filepath.Join(rootDir, "atc.new.exe")); !os.IsNotExist(err) {
		t.Error("atc.new.exe left behind")
	}
	// A second update replaces the rollback copy.
	write("atc.new.exe", "v3")
	if err := swapExe("atc.exe", "atc.new.exe", "atc.prev.exe"); err != nil {
		t.Fatal(err)
	}
	if read("atc.exe") != "v3" || read("atc.prev.exe") != "v2" {
		t.Errorf("second swap: atc=%q prev=%q", read("atc.exe"), read("atc.prev.exe"))
	}
}

// A locked atc.exe (Night 2026-10-21: "being used by another process") is
// retried, not given up on at once.
func TestSwapExeWaitsOutALock(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	defer func(n int, w time.Duration) { swapRetries, swapRetryWait = n, w }(swapRetries, swapRetryWait)
	rootDir = t.TempDir()
	swapRetries, swapRetryWait = 20, 50*time.Millisecond
	cur := filepath.Join(rootDir, "atc.exe")
	os.WriteFile(cur, []byte("v1"), 0o644)
	os.WriteFile(filepath.Join(rootDir, "atc.new.exe"), []byte("v2"), 0o644)
	f, err := os.Open(cur) // Go opens without FILE_SHARE_DELETE: a rename fails while it is open
	if err != nil {
		t.Fatal(err)
	}
	go func() { time.Sleep(300 * time.Millisecond); f.Close() }()
	if err := swapExe("atc.exe", "atc.new.exe", "atc.prev.exe"); err != nil {
		t.Fatalf("swap gave up: %v", err)
	}
	if b, _ := os.ReadFile(cur); string(b) != "v2" {
		t.Errorf("atc.exe = %q after the lock cleared", b)
	}

	// A lock that never clears still fails, and atc.exe stays the old build.
	swapRetries = 3
	os.WriteFile(filepath.Join(rootDir, "atc.new.exe"), []byte("v3"), 0o644)
	g, _ := os.Open(cur)
	defer g.Close()
	if err := swapExe("atc.exe", "atc.new.exe", "atc.prev.exe"); err == nil {
		t.Fatal("a permanent lock was reported as swapped")
	}
	if b, _ := os.ReadFile(cur); string(b) != "v2" {
		t.Errorf("atc.exe = %q after a failed swap, want the old build", b)
	}
}

// A key set with setx after the launcher started reaches the roles it starts.
func TestRefreshKeyEnvReadsTheRegistry(t *testing.T) {
	const name = "SKYEYE_UPDATE_TEST_KEY"
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		t.Skip("cannot open HKCU Environment")
	}
	defer k.Close()
	if err := k.SetStringValue(name, "fresh"); err != nil {
		t.Skip(err)
	}
	defer k.DeleteValue(name)
	os.Setenv(name, "stale")
	defer os.Unsetenv(name)
	if n := refreshKeyEnv(); n < 1 {
		t.Fatalf("refreshKeyEnv set %d", n)
	}
	if got := os.Getenv(name); got != "fresh" {
		t.Errorf("%s = %q, want the registry's value", name, got)
	}
}

// A setting deleted from the registry (reg delete) stops reaching the roles.
func TestRefreshKeyEnvDropsDeleted(t *testing.T) {
	if _, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE); err != nil {
		t.Skip("cannot open HKCU Environment")
	}
	const name = "SKYEYE_UPDATE_TEST_GONE"
	os.Setenv(name, "stale")
	defer os.Unsetenv(name)
	refreshKeyEnv()
	if v, ok := os.LookupEnv(name); ok {
		t.Errorf("%s = %q, want it unset", name, v)
	}
}

// The launcher restarts at the end of an update; the new one shows how it went.
func TestUpdateStateSurvivesRestart(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	rootDir = t.TempDir()
	updMu.Lock()
	upd = updateState{Running: true, From: "aaa", To: "bbb", Log: []string{"built atc.new.exe"}}
	updMu.Unlock()
	updLog("launcher.exe updated - restarting the launcher")
	updMu.Lock()
	upd.Done = true
	updMu.Unlock()
	saveUpdateState()

	updMu.Lock()
	upd = updateState{}
	updMu.Unlock()
	loadUpdateState()
	updMu.Lock()
	defer updMu.Unlock()
	if !upd.Done || upd.Running || upd.To != "bbb" || len(upd.Log) != 2 {
		t.Errorf("reloaded state %+v", upd)
	}
}
