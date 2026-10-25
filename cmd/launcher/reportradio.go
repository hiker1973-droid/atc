package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/debrief"
)

// Reports' radio logs from the rig that flew the session (operator
// 2026-10-24: "The comms report feature is not working does it need to pull
// log from the server", "go ahead and fix aar"). A report made on the dev box
// read only that PC's logs\ - no Readback roles run there, so the Comms page
// and the Marshal and tanker calls came out empty. Each --fleet rig's role logs
// come through its read-only /api/roles and /api/day-log (live on every rig
// since 666c26c, so no Update is needed); "auto" reads them all and keeps
// whichever heard the most.

// radioRigs lists where the radio logs can come from: this PC, then the
// fleet's other rigs.
func radioRigs() []map[string]string {
	out := []map[string]string{{"name": "local", "label": "This PC"}}
	for _, rig := range fleetRigs {
		if isSelf(rig) {
			continue
		}
		label := rig.Label
		if label == "" {
			label = rig.Name
		}
		out = append(out, map[string]string{"name": rig.Name, "label": label})
	}
	return out
}

// rigRadio is one place's radio lines, or why there are none.
type rigRadio struct {
	label string
	rad   *debrief.Radio
	err   error
}

// readReportRadio reads Readback's radio lines between from and to from
// "local" (this PC), a fleet rig by name, or "auto": every one, keeping the
// most. It returns the lines (nil if none), the label for the sources list,
// and when nothing was found, a note saying where it looked.
func readReportRadio(which string, from, to time.Time) (*debrief.Radio, string, string) {
	var tries []func() rigRadio
	local := func() rigRadio {
		rad, err := debrief.ReadRadio(filepath.Join(rootDir, "logs"), from, to)
		return rigRadio{label: "this PC", rad: rad, err: err}
	}
	if which == "" || which == "local" || which == "auto" {
		tries = append(tries, local)
	}
	for _, rig := range fleetRigs {
		if isSelf(rig) || (which != "auto" && which != rig.Name) {
			continue
		}
		rig := rig
		tries = append(tries, func() rigRadio {
			label := rig.Label
			if label == "" {
				label = rig.Name
			}
			rad, err := fetchRigRadio(rig, from, to)
			return rigRadio{label: label, rad: rad, err: err}
		})
	}
	if len(tries) == 0 {
		return nil, "", fmt.Sprintf("radio logs: no rig called %q", which)
	}
	got := make([]rigRadio, len(tries))
	var wg sync.WaitGroup
	for i, try := range tries {
		wg.Add(1)
		go func(i int, try func() rigRadio) {
			defer wg.Done()
			got[i] = try()
		}(i, try)
	}
	wg.Wait()
	best := -1
	for i, g := range got {
		if g.err == nil && g.rad != nil && len(g.rad.Lines) > 0 && (best < 0 || len(g.rad.Lines) > len(got[best].rad.Lines)) {
			best = i
		}
	}
	if best >= 0 {
		g := got[best]
		return g.rad, fmt.Sprintf("Readback radio logs from %s (%d lines)", g.label, len(g.rad.Lines)), ""
	}
	var looked []string
	for _, g := range got {
		if g.err != nil {
			looked = append(looked, fmt.Sprintf("%s (%v)", g.label, g.err))
		} else {
			looked = append(looked, g.label)
		}
	}
	return nil, "", fmt.Sprintf("no Readback radio calls %s - %s on %s",
		from.Local().Format("2 Jan 15:04"), to.Local().Format("15:04"), strings.Join(looked, ", "))
}

// A whole day of a role's log can be several MB.
var radioClient = &http.Client{Timeout: 90 * time.Second}

// fetchRigRadio copies each of a rig's role logs for the days from..to spans
// into a temporary folder, under the log's own name (the station comes from
// it), and reads that.
func fetchRigRadio(rig Rig, from, to time.Time) (*debrief.Radio, error) {
	base := fmt.Sprintf("http://%s:%d", rig.Host, rig.Port)
	var roles []Role
	if err := fleetGetJSON(base+"/api/roles", &roles); err != nil {
		return nil, fmt.Errorf("launcher not reachable: %v", err)
	}
	var dates []string
	for d := from.Local(); !d.After(to.Local()) && len(dates) < 4; d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	if last := to.Local().Format("2006-01-02"); len(dates) > 0 && dates[len(dates)-1] != last && len(dates) < 4 {
		dates = append(dates, last)
	}
	dir, err := os.MkdirTemp("", "readback-radio-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	seen := map[string]bool{} // two roles can share a log (Deckboss runs in the Marshal's)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	slots := make(chan struct{}, 6) // a rig lists every theatre's roles: go easy on its launcher
	for _, role := range roles {
		file := filepath.Base(role.LogFile)
		if !strings.HasPrefix(file, "atc-") || seen[file] {
			continue
		}
		seen[file] = true
		wg.Add(1)
		go func(name, file string) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			if err := fetchDayLogs(base, name, dates, filepath.Join(dir, file)); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(role.Name, file)
	}
	wg.Wait()
	rad, err := debrief.ReadRadio(dir, from, to)
	if err == nil && len(rad.Lines) == 0 && firstErr != nil {
		err = firstErr
	}
	return rad, err
}

// fetchDayLogs writes one role's lines for each date to path.
func fetchDayLogs(base, role string, dates []string, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, date := range dates {
		resp, err := radioClient.Get(base + "/api/day-log?name=" + url.QueryEscape(role) + "&date=" + date)
		if err != nil {
			return err
		}
		ok := resp.StatusCode == http.StatusOK && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain")
		if ok {
			_, err = io.Copy(f, resp.Body)
		}
		resp.Body.Close()
		if !ok {
			return fmt.Errorf("%s: HTTP %d (launcher too old for day logs?)", role, resp.StatusCode)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
