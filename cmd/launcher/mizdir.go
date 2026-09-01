package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/miz"
)

// resolveMizDir finds the DCS Missions directory to scan when neither
// --miz-path nor --miz-dir was given.
//
// The default used to be a literal C:\Users\Administrator\... path — Training
// 1's. On a rig running as any other user the dashboard weather widget failed
// with a confusing "cannot find the path specified". Resolve from the running
// user instead, keeping the old constant as a last resort so the rigs where it
// was correct do not regress.
//
// Returns the directory and every candidate tried, so a failure can say where
// it looked.
func resolveMizDir() (string, []string) {
	if *flagMizDir != "" {
		return *flagMizDir, []string{*flagMizDir}
	}

	var cands []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, variant := range []string{"DCS.dcs_serverrelease", "DCS.release_server", "DCS"} {
			cands = append(cands, filepath.Join(home, "Saved Games", variant, "Missions"))
		}
	}
	cands = append(cands, `C:\Users\Administrator\Saved Games\DCS.dcs_serverrelease\Missions`)

	var tried []string
	for _, c := range cands {
		tried = append(tried, c)
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, tried
		}
	}
	return "", tried
}

// newestParsableMiz returns the newest .miz in dir whose weather actually
// parses, along with its weather.
//
// Taking strictly the newest file is too brittle: a Missions folder collects
// zero-byte stubs and half-written saves, and one of those at the top of the
// list took the whole weather widget down with "zip: not a valid zip file".
// Walk newest-first and use the first that reads.
func newestParsableMiz(dir string) (string, miz.Weather, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", miz.Weather{}, err
	}

	type cand struct {
		path string
		mod  time.Time
	}
	var cands []cand
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".miz") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 { // stubs are never missions
			continue
		}
		cands = append(cands, cand{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	if len(cands) == 0 {
		return "", miz.Weather{}, fmt.Errorf("no readable .miz files in %s", dir)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mod.After(cands[j].mod) })

	var firstErr error
	for _, c := range cands {
		wx, err := miz.ReadMizWeather(c.path)
		if err == nil {
			return c.path, wx, nil
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", filepath.Base(c.path), err)
		}
	}
	return "", miz.Weather{}, firstErr
}
