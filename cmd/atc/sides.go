package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Field sides (operator 2026-10-23: "for the training miz I want to make
// Mozdok a blue airfield", then "add a option gui to have a tower be red or
// blue"). The launcher's tower card has a Side switch that writes sides.json
// next to atc.exe and restarts that tower. Every field is blue unless the
// file says red.
//
//   - A tower reads its side at startup: a red tower registers and transmits
//     on SRS as red and hands departures to Red Command.
//   - Blue Command never recovers or diverts a jet to a red field; Red
//     Command (--coalition red, no --command-fields) uses only the red fields.
//     Command re-reads the file when it changes, so a flip takes effect
//     without restarting it.

// sidesFile is sides.json. Fields maps an ICAO to "red"; anything else, or a
// field not listed, is blue.
type sidesFile struct {
	Fields map[string]string `json:"fields"`
}

// redCommandFreqMHz and redCommandName are what a red tower hands departures
// to: Red Command in start_red_caucasus.bat, or where the Settings page moved
// it (roles.go).
var (
	redCommandFreqMHz = 280.0
	redCommandName    = "Red Command"
)

var (
	sidesMu    sync.Mutex
	sidesCache sidesFile
	sidesMod   time.Time
	sidesPath  = func() string {
		exe, err := os.Executable()
		if err != nil {
			return "sides.json"
		}
		return filepath.Join(filepath.Dir(exe), "sides.json")
	}
)

// fieldIsRed reports whether sides.json makes icao a red field. The file is
// re-read only when it changes.
func fieldIsRed(icao string) bool {
	sidesMu.Lock()
	defer sidesMu.Unlock()
	p := sidesPath()
	st, err := os.Stat(p)
	if err != nil {
		sidesCache, sidesMod = sidesFile{}, time.Time{}
		return false
	}
	if !st.ModTime().Equal(sidesMod) {
		var f sidesFile
		if b, err := os.ReadFile(p); err == nil {
			if err := json.Unmarshal(b, &f); err != nil {
				log.Warn().Err(err).Str("file", p).Msg("sides.json unreadable - every field blue")
				f = sidesFile{}
			}
		}
		sidesCache, sidesMod = f, st.ModTime()
	}
	return strings.EqualFold(sidesCache.Fields[strings.ToUpper(icao)], "red")
}

// sideFields keeps the fields on Command's side: blue Command the blue ones,
// red Command the red ones.
func sideFields(fields []*airfield.Airfield, red bool) []*airfield.Airfield {
	var out []*airfield.Airfield
	for _, f := range fields {
		if fieldIsRed(f.ICAO) == red {
			out = append(out, f)
		}
	}
	return out
}

// applyTowerSide sets a tower's side from sides.json when --coalition was not
// given, and points a red tower's departure handoff at Red Command. af is the
// registry entry; a tower process runs one field, so changing it here changes
// nothing else.
func applyTowerSide(af *airfield.Airfield, coalitionGiven bool) {
	if !coalitionGiven && fieldIsRed(af.ICAO) {
		flagCoalition = "red"
	}
	if redSide() {
		af.HandoffCallsign = redCommandName
		af.HandoffFreqMHz = redCommandFreqMHz
		af.HandoffPreset = "" // a different button on each jet in the BFM mission
	}
}
