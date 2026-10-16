package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Comms report on the dashboard (operator 2026-10-15: "Should we have a tower
// or command report near miss or inproper comms?" -> "build 1, 2 and 3").
// Every tower appends near misses, departures and landings without a
// clearance, calls with no callsign, changed callsigns and calls not
// understood to <root>\comms_report\YYYY-MM-DD.jsonl (cmd/atc/comms_report.go).
// Each launcher serves its own rig's (/api/comms-report); /api/comms-fleet
// merges every rig, newest first. For debriefs and for finding SkyEye's own
// recognition faults - the report is not spoken on the radio.

// CommsLine is one report line as atc.exe writes it.
type CommsLine struct {
	Time     time.Time `json:"t"`
	Airfield string    `json:"airfield"`
	Tower    string    `json:"tower"`
	Kind     string    `json:"kind"`
	Callsign string    `json:"callsign,omitempty"`
	Other    string    `json:"other,omitempty"`
	Detail   string    `json:"detail"`
	Text     string    `json:"text,omitempty"`
	Rig      string    `json:"rig,omitempty"`
}

// CommsReport is a rig's (or the fleet's) events in a window.
type CommsReport struct {
	Since  time.Time      `json:"since"`
	Events []CommsLine    `json:"events"`
	Counts map[string]int `json:"counts"`
	Rigs   []RigCommsErr  `json:"rigs,omitempty"`
	At     time.Time      `json:"at"`
}

// RigCommsErr is one rig's share of the fleet report.
type RigCommsErr struct {
	Rig    string `json:"rig"`
	Label  string `json:"label"`
	Events int    `json:"events"`
	Err    string `json:"err,omitempty"`
}

const commsReportMaxEvents = 400

func commsReportDir() string { return filepath.Join(rootDir, "comms_report") }

// readCommsReport reads dir's events since since, newest first.
func readCommsReport(dir string, since time.Time) CommsReport {
	rep := CommsReport{Since: since, Counts: map[string]int{}, At: time.Now()}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return rep
	}
	for _, e := range entries {
		day, err := time.Parse("2006-01-02.jsonl", e.Name())
		if err != nil || day.Before(since.UTC().Truncate(24*time.Hour)) {
			continue
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			var l CommsLine
			if json.Unmarshal(sc.Bytes(), &l) != nil || l.Time.Before(since) {
				continue
			}
			rep.Events = append(rep.Events, l)
		}
		f.Close()
	}
	finishCommsReport(&rep)
	return rep
}

// finishCommsReport sorts newest first, caps the list and counts by kind
// (the counts cover every event, not only the ones kept).
func finishCommsReport(rep *CommsReport) {
	sort.Slice(rep.Events, func(i, j int) bool { return rep.Events[i].Time.After(rep.Events[j].Time) })
	for _, e := range rep.Events {
		rep.Counts[e.Kind]++
	}
	if len(rep.Events) > commsReportMaxEvents {
		rep.Events = rep.Events[:commsReportMaxEvents]
	}
}

// commsSince is the window start from ?hours= (default 24, at most a week).
func commsSince(r *http.Request) time.Time {
	h := 24
	if v, err := strconv.Atoi(r.URL.Query().Get("hours")); err == nil && v > 0 {
		h = v
	}
	if h > 168 {
		h = 168
	}
	return time.Now().Add(-time.Duration(h) * time.Hour)
}

func handleCommsReport(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, readCommsReport(commsReportDir(), commsSince(r)))
}

var (
	commsFleetMu    sync.Mutex
	commsFleetCache = map[int64]*CommsReport{}
)

// handleCommsFleet merges every rig's report; cached 30 s per window.
func handleCommsFleet(w http.ResponseWriter, r *http.Request) {
	since := commsSince(r)
	hours := int64(time.Since(since).Round(time.Hour) / time.Hour)
	commsFleetMu.Lock()
	defer commsFleetMu.Unlock()
	if c := commsFleetCache[hours]; c != nil && time.Since(c.At) < 30*time.Second {
		writeJSON(w, c)
		return
	}
	parts := make([]CommsReport, len(fleetRigs))
	errs := make([]RigCommsErr, len(fleetRigs))
	var wg sync.WaitGroup
	for i, rig := range fleetRigs {
		wg.Add(1)
		go func(i int, rig Rig) {
			defer wg.Done()
			errs[i] = RigCommsErr{Rig: rig.Name, Label: rig.Label}
			var rep CommsReport
			if isSelf(rig) {
				rep = readCommsReport(commsReportDir(), since)
			} else if err := fleetGetJSON(fmt.Sprintf("http://%s:%d/api/comms-report?hours=%d", rig.Host, rig.Port, hours), &rep); err != nil {
				errs[i].Err = err.Error()
				return
			}
			label := rig.Label
			if label == "" {
				label = rig.Name
			}
			for j := range rep.Events {
				rep.Events[j].Rig = label
			}
			parts[i] = rep
			errs[i].Events = 0
			for _, n := range rep.Counts {
				errs[i].Events += n
			}
		}(i, rig)
	}
	wg.Wait()
	all := CommsReport{Since: since, Counts: map[string]int{}, At: time.Now(), Rigs: errs}
	for _, p := range parts {
		all.Events = append(all.Events, p.Events...)
	}
	finishCommsReport(&all)
	// Counts from the rigs' own totals: their event lists may be capped.
	all.Counts = map[string]int{}
	for _, p := range parts {
		for k, n := range p.Counts {
			all.Counts[k] += n
		}
	}
	commsFleetCache[hours] = &all
	writeJSON(w, all)
}
