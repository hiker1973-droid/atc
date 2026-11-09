package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTiwParse(t *testing.T) {
	if n, ok := tiwParse("2026-11-08 17:00:00.000 INFO SCRIPTING: [CarrierGUI Bridge] TIW flag 320=60"); !ok || n != 60 {
		t.Fatalf("got %d %v", n, ok)
	}
	if n, ok := tiwParse("... [CarrierGUI Bridge] TIW flag 320=0"); !ok || n != 0 {
		t.Fatalf("got %d %v", n, ok)
	}
	if _, ok := tiwParse("something else"); ok {
		t.Fatal("matched")
	}
}

func TestTiwTail(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dcs.log")
	first := "old line TIW flag 320=30\n"
	os.WriteFile(p, []byte(first+"[CarrierGUI Bridge] TIW flag 320=60\n[CarrierGUI Bridge] TIW flag 320="), 0o644)
	got, off2 := tiwTail(p, int64(len(first)))
	if len(got) != 1 || got[0] != 60 {
		t.Fatalf("got %v", got)
	}
	// the half-written line is picked up once finished
	os.WriteFile(p, []byte(first+"[CarrierGUI Bridge] TIW flag 320=60\n[CarrierGUI Bridge] TIW flag 320=0\n"), 0o644)
	got, off3 := tiwTail(p, off2)
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("got %v", got)
	}
	// a restarted log (smaller file) is read from the top
	os.WriteFile(p, []byte("[CarrierGUI Bridge] TIW flag 320=90\n"), 0o644)
	if got, _ = tiwTail(p, off3); len(got) != 1 || got[0] != 90 {
		t.Fatalf("got %v", got)
	}
}
