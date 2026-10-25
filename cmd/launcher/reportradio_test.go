package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A report made away from the rigs reads the radio logs through a rig's
// /api/roles and /api/day-log; "auto" keeps whichever place heard the most.
func TestReportRadioFromRig(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	rootDir = t.TempDir() // this PC: no logs
	os.MkdirAll(filepath.Join(rootDir, "logs"), 0o755)

	days := map[string]string{
		"Senaki Tower|2026-10-22": `{"level":"info","text":"Senaki tower, Raider 302, ready for taxi.","time":"2026-10-22T17:37:43-06:00","message":"recognized"}
{"level":"info","text":"Raider 302, Senaki Tower, taxi to runway 27.","time":"2026-10-22T17:37:46-06:00","message":"Tower TX"}
{"level":"info","text":"too late","time":"2026-10-22T23:00:00-06:00","message":"recognized"}
`,
		"Deckboss|2026-10-22": `{"level":"info","text":"twice","time":"2026-10-22T17:40:00-06:00","message":"recognized"}
`,
	}
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/roles":
			writeJSON(w, []Role{{Name: "Senaki Tower", LogFile: `C:\SkyeyeATC\logs\atc-ugkn.log`},
				{Name: "Marshal", LogFile: "atc-marshal.log"}, {Name: "Deckboss", LogFile: "atc-marshal.log"}})
		case "/api/day-log":
			name, date := r.URL.Query().Get("name"), r.URL.Query().Get("date")
			asked = append(asked, name+"|"+date)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte(days[name+"|"+date]))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	p, _ := strconv.Atoi(port)
	fleetRigs = []Rig{{Name: "training1", Label: "Day Server", Host: host, Port: p}}
	t.Cleanup(func() { fleetRigs = nil })

	mdt := time.FixedZone("MDT", -6*3600)
	from := time.Date(2026, 10, 22, 17, 30, 0, 0, mdt)
	to := time.Date(2026, 10, 22, 18, 30, 0, 0, mdt)
	for _, which := range []string{"auto", "training1"} {
		rad, src, note := readReportRadio(which, from, to)
		if rad == nil || len(rad.Lines) != 2 || note != "" {
			t.Fatalf("%s: got %v %q %q", which, rad, src, note)
		}
		if src != "Readback radio logs from Day Server (2 lines)" || rad.Lines[0].Station != "UGKN" {
			t.Errorf("%s: source %q, station %q", which, src, rad.Lines[0].Station)
		}
	}
	for _, a := range asked {
		if strings.HasPrefix(a, "Deckboss") {
			t.Errorf("Deckboss shares the Marshal's log and was fetched again: %v", asked)
		}
	}

	// This PC alone: nothing, and the note says where it looked.
	rad, _, note := readReportRadio("local", from, to)
	if rad != nil || !strings.Contains(note, "this PC") {
		t.Errorf("local: %v %q", rad, note)
	}
}
