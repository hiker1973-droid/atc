package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Pilot display names (operator 2026-11-08: "streamline these names for our
// standards and allow me to edit them for the rigs").
//
// The standard is "Callsign NNN | Handle" with a capitalised flight word, e.g.
// "Venom 212 | Dabs". DCS names arrive in whatever form the player typed
// ("RAIDER 031 | BACON", "Raider 302| Jedi", "leeroy-venom 024"), so
// standardPilotName tidies them, and pilot_names.json (next to the launcher,
// UCID -> name) holds any name the operator typed by hand, which always wins.
// Applied in mergePilots, so the Pilots page, the Stats tab, the nightly
// reports and the greenie board all show the same name.

var (
	pilotNamesMu sync.Mutex
	pilotNames   map[string]string
)

func pilotNamesFile() string { return filepath.Join(rootDir, "pilot_names.json") }

func loadPilotNames() map[string]string {
	if pilotNames != nil {
		return pilotNames
	}
	pilotNames = map[string]string{}
	if b, err := os.ReadFile(pilotNamesFile()); err == nil {
		_ = json.Unmarshal(b, &pilotNames)
	}
	return pilotNames
}

func pilotNameOverride(ucid string) string {
	pilotNamesMu.Lock()
	defer pilotNamesMu.Unlock()
	return loadPilotNames()[ucid]
}

var flightRe = regexp.MustCompile(`(?i)\b(venom|raider)[\s-]*(\d[\d-]*)`)

func titleWord(w string) string {
	if w == "" || strings.ContainsAny(w, "-") && strings.ToUpper(w) == w {
		return w // an acronym such as KC-DC stays as typed
	}
	if w == strings.ToUpper(w) || w == strings.ToLower(w) {
		return strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return w // mixed case is deliberate (DaBs, McGee)
}

// standardPilotName turns a DCS player name into "Venom 212 | Dabs". A name
// with no flight/number in it is just tidied.
func standardPilotName(raw string) string {
	raw = strings.Join(strings.Fields(raw), " ")
	if raw == "" {
		return raw
	}
	m := flightRe.FindStringSubmatchIndex(raw)
	if m == nil {
		return strings.Join(mapWords(strings.Fields(raw)), " ")
	}
	flight := titleWord(strings.ToLower(raw[m[2]:m[3]])) + " " + raw[m[4]:m[5]]
	rest := raw[:m[0]] + " " + raw[m[1]:]
	rest = strings.NewReplacer("|", " ", "vSFG-7", " ", "VSFG-7", " ").Replace(rest)
	var handle []string
	for _, w := range strings.Fields(strings.Trim(rest, " -")) {
		w = strings.Trim(w, "-")
		if w == "" || strings.Trim(w, "0123456789") == "" {
			continue // a trailing slot number is not part of the handle
		}
		handle = append(handle, w)
	}
	if len(handle) == 0 {
		return flight
	}
	return flight + " | " + strings.Join(mapWords(handle), " ")
}

func mapWords(ws []string) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = titleWord(w)
	}
	return out
}

var stdNameRe = regexp.MustCompile(`^(Raider|Venom) \d[\d-]* \| \S.*$`)

// isStandardPilotName: exactly "Raider 302 | Jedi" or "Venom 024 | Leeroy".
func isStandardPilotName(n string) bool { return stdNameRe.MatchString(n) }

// displayPilotName is what the pages show: the operator's name, else the tidied DCS one.
func displayPilotName(ucid, raw string) string {
	if n := pilotNameOverride(ucid); n != "" {
		return n
	}
	return standardPilotName(raw)
}

// handlePilotName: POST {"ucid":"...","name":"..."} sets a name; an empty name goes back to the automatic one.
// GET returns every saved name.
func handlePilotName(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pilotNamesMu.Lock()
		defer pilotNamesMu.Unlock()
		writeJSON(w, loadPilotNames())
		return
	}
	requireControl(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ UCID, Name string }
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.UCID == "" {
			http.Error(w, "need ucid", http.StatusBadRequest)
			return
		}
		name := strings.Join(strings.Fields(req.Name), " ")
		if name != "" && !isStandardPilotName(name) {
			http.Error(w, `name must look like "Raider 302 | Jedi" or "Venom 024 | Leeroy"`, http.StatusBadRequest)
			return
		}
		pilotNamesMu.Lock()
		m := loadPilotNames()
		if name == "" {
			delete(m, req.UCID)
		} else {
			m[req.UCID] = name
		}
		b, _ := json.MarshalIndent(m, "", "  ")
		err := os.WriteFile(pilotNamesFile(), b, 0o644)
		pilotNamesMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pilotsCacheMu.Lock()
		pilotsCache = map[string]*PilotsReport{}
		pilotsCacheMu.Unlock()
		writeJSON(w, map[string]string{"ucid": req.UCID, "name": displayPilotName(req.UCID, name)})
	})(w, r)
}
