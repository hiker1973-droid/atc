package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vsfg7/atc/pkg/miz"
)

func TestReadMissionPointer(t *testing.T) {
	dir := t.TempDir()
	mission := filepath.Join(dir, "Foothold_SY_v1.10_03_VFR.miz")
	if err := os.WriteFile(mission, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	ptr := filepath.Join(dir, "current_mission.txt")

	if got := readMissionPointer(ptr); got != "" {
		t.Errorf("missing pointer file: got %q, want empty", got)
	}
	cases := []struct {
		name, content, want string
	}{
		{"plain", mission, mission},
		{"BOM and newline, as DCS may write it", "\uFEFF" + mission + "\r\n", mission},
		{"empty", "", ""},
		{"not a .miz", filepath.Join(dir, "notes.txt"), ""},
		{"mission no longer on disk", filepath.Join(dir, "gone.miz"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(ptr, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			if got := readMissionPointer(ptr); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// A rotation to a new mission applies its weather once; the same mission, or a
// mission whose weather cannot be read, leaves the current weather alone.
func TestMizWatcherStep(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.miz")
	b := filepath.Join(dir, "B.miz")
	broken := filepath.Join(dir, "broken.miz")
	for _, p := range []string{a, b, broken} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ptr := filepath.Join(dir, "current_mission.txt")
	point := func(p string) {
		if err := os.WriteFile(ptr, []byte(p), 0644); err != nil {
			t.Fatal(err)
		}
	}

	var applied []float64
	w := &mizWatcher{
		pointerFile: ptr,
		current:     a, // booted from A
		read: func(p string) (miz.Weather, error) {
			switch p {
			case a:
				return miz.Weather{WindKts: 1}, nil
			case b:
				return miz.Weather{WindKts: 2}, nil
			}
			return miz.Weather{}, errors.New("no mission table")
		},
		apply: func(wx miz.Weather) { applied = append(applied, wx.WindKts) },
	}

	point(a)
	if w.step() || len(applied) != 0 {
		t.Fatalf("pointer names the boot mission: applied %v, want nothing", applied)
	}
	point(b)
	if !w.step() || len(applied) != 1 || applied[0] != 2 {
		t.Fatalf("rotation to B: applied %v, want [2]", applied)
	}
	if w.step() || len(applied) != 1 {
		t.Fatalf("B again: applied %v, want still [2]", applied)
	}
	point(broken)
	if w.step() || len(applied) != 1 || w.current != broken {
		t.Fatalf("broken mission: applied %v current %q, want weather kept and no retry", applied, w.current)
	}
	point(a)
	if !w.step() || len(applied) != 2 || applied[1] != 1 {
		t.Fatalf("back to A: applied %v, want [2 1]", applied)
	}
}
