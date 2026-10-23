package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Field sides: the tower card's Side switch (operator 2026-10-23: "add a
// option gui to have a tower be red or blue"). The launcher keeps the choice
// in sides.json next to atc.exe; atc.exe reads it (cmd/atc/sides.go). A tower
// takes its side at startup, so a flip restarts it; Command re-reads the file
// by itself. Every field is blue unless the file says red.

type sidesFile struct {
	Fields map[string]string `json:"fields"` // ICAO -> "red"
}

var sidesMu sync.Mutex

func sidesPath() string { return filepath.Join(rootDir, "sides.json") }

func readSides() sidesFile {
	f := sidesFile{Fields: map[string]string{}}
	if b, err := os.ReadFile(sidesPath()); err == nil {
		_ = json.Unmarshal(b, &f)
	}
	if f.Fields == nil {
		f.Fields = map[string]string{}
	}
	return f
}

// setFieldSide records icao's side; blue removes it from the file.
func setFieldSide(icao, side string) error {
	sidesMu.Lock()
	defer sidesMu.Unlock()
	f := readSides()
	if side == "red" {
		f.Fields[icao] = "red"
	} else {
		delete(f.Fields, icao)
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := sidesPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, sidesPath())
}

// towerRolesFor are the tower roles (not Deckboss, which shares Batumi's
// --airfield) for a field.
func towerRolesFor(icao string) []string {
	rolesMu.Lock()
	defer rolesMu.Unlock()
	var out []string
	for _, r := range roles {
		if strings.EqualFold(r.Airfield, icao) && strings.HasPrefix(strings.ToLower(r.Bat), "start_towers_") {
			out = append(out, r.Name)
		}
	}
	return out
}

// handleSides: GET the sides; POST ?icao=XRMF&side=red|blue sets one and
// restarts that field's tower if it is running.
func handleSides(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sidesMu.Lock()
		f := readSides()
		sidesMu.Unlock()
		writeJSON(w, f)
		return
	}
	if code, err := controlAllowed(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	icao := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("icao")))
	side := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("side")))
	if icao == "" || (side != "red" && side != "blue") {
		http.Error(w, "want ?icao=<ICAO>&side=red|blue", http.StatusBadRequest)
		return
	}
	if err := setFieldSide(icao, side); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	restarted := []string{}
	for _, name := range runningRoles(towerRolesFor(icao)) {
		_ = stopRole(name)
		time.Sleep(time.Second)
		// The towers bat starts every tower; the ones still up refuse their
		// second copy (ec006bd), so only this one comes back.
		if err := startRole(name); err == nil {
			restarted = append(restarted, name)
		}
	}
	writeJSON(w, map[string]any{"icao": icao, "side": side, "restarted": restarted})
}
