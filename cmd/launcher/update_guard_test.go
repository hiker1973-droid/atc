package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLastPilotCall(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.Add(-time.Hour).Format(time.RFC3339)
	recent := now.Add(-2 * time.Minute).Format(time.RFC3339)
	os.WriteFile(filepath.Join(dir, "atc-a.log"), []byte(
		`{"level":"info","time":"`+old+`","message":"recognized"}`+"\n"+
			`{"level":"info","time":"`+recent+`","message":"Tacview telemetry connected"}`+"\n"), 0o644)
	since := now.Add(-recentPilotWindow)
	if got := lastPilotCall(dir, since); !got.IsZero() {
		t.Fatalf("old call / non-call counted as activity: %v", got)
	}
	os.WriteFile(filepath.Join(dir, "atc-b.log"), []byte(
		`{"level":"info","time":"`+recent+`","message":"Marshal heard"}`+"\n"), 0o644)
	if got := lastPilotCall(dir, since); got.IsZero() {
		t.Fatal("recent Marshal call not detected")
	}
}
