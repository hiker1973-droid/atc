package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One day's Paddles lines, from the rotated log and the live one, oldest
// first; other days and debug lines left out.
func TestWriteDayLog(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, lines ...string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("atc-lso.log.1",
		`{"level":"info","time":"2026-10-17T23:59:00-06:00","message":"Paddles: pass"}`,
		`{"level":"info","time":"2026-10-18T00:01:00-06:00","message":"Paddles: pass","pass":"a"}`)
	write("atc-lso.log",
		`{"level":"debug","time":"2026-10-18T10:00:00-06:00","message":"noise"}`,
		`{"level":"info","time":"2026-10-18T10:28:19-06:00","message":"Paddles: pass","pass":"b"}`,
		`{"level":"info","time":"2026-10-19T00:00:01-06:00","message":"Paddles: pass"}`)
	var b strings.Builder
	if err := writeDayLog(&b, dir, "atc-lso.log", "2026-10-18"); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(got) != 2 || !strings.Contains(got[0], `"pass":"a"`) || !strings.Contains(got[1], `"pass":"b"`) {
		t.Errorf("lines = %q", got)
	}
}
