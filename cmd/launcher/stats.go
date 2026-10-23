package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/debrief"
)

// Stats tab (operator 2026-10-23: "we should be able to do a greenieboard,
// flight stats", "maybe do a separate page/tab and export function"). The
// squadron's numbers over time, from every mission run in the DCS server's
// dcs.log files on this PC (the live log, dcs.log.old and the dated .zip
// copies DCS rotates out) plus any logs uploaded here: per-pilot sorties,
// flight hours, landings, traps, kills, losses, and the greenie board.
// Export as CSV (pilots, passes) or a PDF.

func statsUploadDir() string { return filepath.Join(rootDir, "stats", "logs") }

type statsLog struct {
	Path     string    `json:"-"`
	Label    string    `json:"label"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Runs     int       `json:"runs"`
	Uploaded bool      `json:"uploaded"`
}

type statsCacheEntry struct {
	mod  time.Time
	size int64
	runs []debrief.EventSession
}

var (
	statsMu    sync.Mutex
	statsCache = map[string]statsCacheEntry{}
)

// statsLogFiles: the server logs DCS keeps under Saved Games, and the
// uploads.
func statsLogFiles() []statsLog {
	var out []statsLog
	add := func(p string, uploaded bool) {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			label := filepath.Base(filepath.Dir(filepath.Dir(p))) + `\` + filepath.Base(p)
			if uploaded {
				label = filepath.Base(p) + " (uploaded)"
			}
			out = append(out, statsLog{Path: p, Label: label, Size: st.Size(), Modified: st.ModTime(), Uploaded: uploaded})
		}
	}
	home, _ := os.UserHomeDir()
	for _, pat := range []string{"dcs.log", "dcs.log.old", "dcs.log-*.zip"} {
		m, _ := filepath.Glob(filepath.Join(home, "Saved Games", "DCS*", "Logs", pat))
		for _, p := range m {
			add(p, false)
		}
	}
	up, _ := filepath.Glob(filepath.Join(statsUploadDir(), "*"))
	for _, p := range up {
		add(p, true)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out
}

// statsRuns reads (or reuses) every log's mission runs.
func statsRuns() ([]debrief.EventSession, []statsLog) {
	files := statsLogFiles()
	statsMu.Lock()
	defer statsMu.Unlock()
	var all []debrief.EventSession
	for i, f := range files {
		c, ok := statsCache[f.Path]
		if !ok || !c.mod.Equal(f.Modified) || c.size != f.Size {
			c = statsCacheEntry{mod: f.Modified, size: f.Size, runs: readRuns(f.Path)}
			statsCache[f.Path] = c
		}
		files[i].Runs = len(c.runs)
		all = append(all, c.runs...)
	}
	return all, files
}

func readRuns(path string) []debrief.EventSession {
	var r io.ReadCloser
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil || len(zr.File) == 0 {
			return nil
		}
		defer zr.Close()
		rc, err := zr.File[0].Open()
		if err != nil {
			return nil
		}
		r = rc
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		r = f
	}
	defer r.Close()
	s, err := debrief.ParseDCSLog(r)
	if err != nil && s == nil {
		return nil
	}
	return s.Sessions
}

// statsPeriod reads ?period=month|lastmonth|30d|year|all|custom&from=&to=
// (dates as YYYY-MM-DD, "to" inclusive) in this PC's time.
func statsPeriod(r *http.Request) (time.Time, time.Time, string) {
	now := time.Now()
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local) }
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	switch r.URL.Query().Get("period") {
	case "lastmonth":
		return month.AddDate(0, -1, 0), month, month.AddDate(0, -1, 0).Format("January 2006")
	case "30d":
		return day(now).AddDate(0, 0, -29), day(now).AddDate(0, 0, 1), "the last 30 days"
	case "year":
		y := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		return y, y.AddDate(1, 0, 0), now.Format("2006")
	case "all":
		return time.Time{}, time.Time{}, "all time"
	case "custom":
		from, err1 := time.ParseInLocation("2006-01-02", r.URL.Query().Get("from"), time.Local)
		to, err2 := time.ParseInLocation("2006-01-02", r.URL.Query().Get("to"), time.Local)
		if err1 == nil && err2 == nil && !to.Before(from) {
			return from, to.AddDate(0, 0, 1), from.Format("2 Jan 2006") + " - " + to.Format("2 Jan 2006")
		}
	}
	return month, month.AddDate(0, 1, 0), month.Format("January 2006")
}

func statsTotals(r *http.Request) (*debrief.Totals, []statsLog, string) {
	runs, files := statsRuns()
	from, to, label := statsPeriod(r)
	return debrief.AddUp(runs, from, to, time.Local), files, label
}

type statsPilot struct {
	Name      string   `json:"name"`
	Side      string   `json:"side"`
	Aircraft  []string `json:"aircraft"`
	Sorties   int      `json:"sorties"`
	FlightMin int      `json:"flightMin"`
	Landings  int      `json:"landings"`
	Traps     int      `json:"traps"`
	Kills     int      `json:"kills"`
	Ground    int      `json:"ground"`
	ShotDown  int      `json:"shotDown"`
	Crashes   int      `json:"crashes"`
	Ejections int      `json:"ejections"`
	Shots     int      `json:"shots"`
	Hits      int      `json:"hits"`
}

type statsPass struct {
	At    time.Time `json:"at"`
	Grade string    `json:"grade"`
	Wire  int       `json:"wire"`
	Raw   string    `json:"raw"`
	Place string    `json:"place"`
	Fill  string    `json:"fill"`
	Ink   string    `json:"ink"`
}

type statsBoard struct {
	Pilot  string      `json:"pilot"`
	Avg    float64     `json:"avg"`
	Traps  int         `json:"traps"`
	Passes []statsPass `json:"passes"`
}

func pilotRows(t *debrief.Totals) []statsPilot {
	out := []statsPilot{}
	for _, p := range t.Pilots {
		out = append(out, statsPilot{Name: p.Name, Side: p.Side, Aircraft: p.Aircraft, Sorties: p.Sorties, FlightMin: int(p.Flight.Minutes()),
			Landings: p.Landings, Traps: p.Traps, Kills: p.Kills, Ground: p.Ground, ShotDown: p.ShotDown, Crashes: p.Crashes,
			Ejections: p.Ejections, Shots: p.Shots, Hits: p.Hits})
	}
	return out
}

func boardRows(t *debrief.Totals) []statsBoard {
	out := []statsBoard{}
	for _, b := range t.GreenieBoard() {
		row := statsBoard{Pilot: b.Pilot, Avg: b.Avg, Traps: b.Traps}
		for _, p := range b.Passes {
			fill, ink := debrief.BoardColours(p.Grade.Grade)
			row.Passes = append(row.Passes, statsPass{At: p.At, Grade: p.Grade.Grade, Wire: p.Wire, Raw: p.Raw, Place: p.Place, Fill: fill, Ink: ink})
		}
		out = append(out, row)
	}
	return out
}

func serveStatsUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("stats.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleStats: GET the totals for a period.
func handleStats(w http.ResponseWriter, r *http.Request) {
	t, files, label := statsTotals(r)
	writeJSON(w, map[string]any{"period": label, "from": t.From, "to": t.To, "sessions": t.Sessions, "sources": files,
		"pilots": pilotRows(t), "board": boardRows(t), "canUpload": who(r).Admin})
}

// handleStatsCSV: GET ?kind=pilots|passes, a spreadsheet of the period.
func handleStatsCSV(w http.ResponseWriter, r *http.Request) {
	t, _, label := statsTotals(r)
	kind := r.URL.Query().Get("kind")
	name := "Readback pilot stats"
	if kind == "passes" {
		name = "Readback greenie board"
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s - %s.csv"`, name, strings.NewReplacer("/", "-", ":", "").Replace(label)))
	io.WriteString(w, "\ufeff") // so Excel reads the names right
	cw := csv.NewWriter(w)
	if kind == "passes" {
		cw.Write([]string{"Date", "Time", "Pilot", "Grade", "Wire", "Points", "Ship", "LSO comment"})
		for _, b := range t.GreenieBoard() {
			for _, p := range b.Passes {
				pts := ""
				if v, ok := debrief.GradePoints(p.Grade.Grade); ok {
					pts = strconv.FormatFloat(v, 'f', -1, 64)
				}
				wire := ""
				if p.Wire > 0 {
					wire = strconv.Itoa(p.Wire)
				}
				at := p.At.In(time.Local)
				cw.Write([]string{at.Format("2006-01-02"), at.Format("15:04"), p.Pilot, p.Grade.Grade, wire, pts, p.Place, p.Raw})
			}
		}
	} else {
		cw.Write([]string{"Pilot", "Side", "Aircraft", "Sorties", "Flight hours", "Landings", "Traps", "Air kills", "Ground kills",
			"Shot down", "Crashes", "Ejections", "Shots", "Hits"})
		for _, p := range t.Pilots {
			cw.Write([]string{p.Name, p.Side, strings.Join(p.Aircraft, " / "), strconv.Itoa(p.Sorties), fmt.Sprintf("%.1f", p.Flight.Hours()),
				strconv.Itoa(p.Landings), strconv.Itoa(p.Traps), strconv.Itoa(p.Kills), strconv.Itoa(p.Ground), strconv.Itoa(p.ShotDown),
				strconv.Itoa(p.Crashes), strconv.Itoa(p.Ejections), strconv.Itoa(p.Shots), strconv.Itoa(p.Hits)})
		}
	}
	cw.Flush()
}

// handleStatsPDF: GET the period's stats and greenie board as a PDF.
func handleStatsPDF(w http.ResponseWriter, r *http.Request) {
	t, _, label := statsTotals(r)
	pdf, err := printPDF(r.Context(), []byte(statsHTML(t, label)))
	if err != nil {
		http.Error(w, "could not make the PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Readback squadron stats - %s.pdf"`, strings.NewReplacer("/", "-", ":", "").Replace(label)))
	w.Write(pdf)
}

// handleStatsUpload (control, multipart "log"): keep a server dcs.log (or a
// rotated .zip of one) here, so its runs count.
func handleStatsUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	f, h, err := r.FormFile("log")
	if err != nil {
		http.Error(w, "choose a dcs.log first", http.StatusBadRequest)
		return
	}
	defer f.Close()
	if err := os.MkdirAll(statsUploadDir(), 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	ext := ".log"
	if strings.EqualFold(filepath.Ext(h.Filename), ".zip") {
		ext = ".zip"
	}
	base := strings.TrimSuffix(safeName(h.Filename), filepath.Ext(safeName(h.Filename)))
	out, err := os.CreateTemp(statsUploadDir(), time.Now().Format("2006-01-02_150405")+" "+base+" *"+ext) // unique: two uploads in a second
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	dst := out.Name()
	_, err = io.Copy(out, f)
	out.Close()
	if err != nil {
		os.Remove(dst)
		http.Error(w, err.Error(), 500)
		return
	}
	runs := readRuns(dst)
	events := 0
	for _, run := range runs {
		events += len(run.Events)
	}
	if events == 0 {
		os.Remove(dst)
		http.Error(w, "that log has no server events: it must be the DCS server's own dcs.log (Saved Games\\DCS.dcs_serverrelease\\Logs), not a game client's", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "runs": len(runs), "events": events})
}

// statsHTML is the period's stats as a printable page (landscape).
func statsHTML(t *debrief.Totals, label string) string {
	squadron, _ := brandingSnapshot()["name"].(string)
	e := html.EscapeString
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>Squadron stats</title><style>
@page { size: 11in 8.5in; margin: 0.45in; }
body { font-family: "Segoe UI", Arial, sans-serif; color: #1c232b; font-size: 9pt; margin: 0; }
.hd { display: flex; align-items: center; gap: 12px; } .hd img { height: 46px; } h1 { font-size: 17pt; margin: 0; }
.sub { color: #55606b; } h2 { font-size: 11pt; margin: 14px 0 4px; border-bottom: 1.4px solid #1c232b; text-transform: uppercase; letter-spacing: .04em; }
table { border-collapse: collapse; width: 100%; } th { text-align: left; font-size: 7.5pt; color: #55606b; border-bottom: 1.4px solid #333; padding: 2px 4px; }
td { border-bottom: 1px solid #dde2e6; padding: 2px 4px; } td.n, th.n { text-align: right; } tr { break-inside: avoid; }
.sq { display: inline-block; width: 16px; height: 16px; line-height: 16px; text-align: center; font-size: 7.5pt; font-weight: 700; margin: 1px; border-radius: 2px;
  -webkit-print-color-adjust: exact; print-color-adjust: exact; }
.key { color: #55606b; font-size: 8pt; margin-top: 4px; }
</style></head><body>`)
	b.WriteString(`<div class="hd"><img src="` + string(squadronLogoDataURI()) + `" alt=""><div><h1>` + e(squadron) + ` · squadron stats · ` + e(label) +
		`</h1><div class="sub">` + fmt.Sprintf("%d mission runs on the server", t.Sessions) + ` · from the DCS server's own logs · made ` + time.Now().Format("2 Jan 2006 15:04") + `</div></div></div>`)
	b.WriteString(`<h2>Flight stats</h2><table><tr><th>Pilot</th><th>Aircraft</th><th class="n">Sorties</th><th class="n">Hours</th><th class="n">Landings</th><th class="n">Traps</th><th class="n">Air kills</th><th class="n">Ground</th><th class="n">Shot down</th><th class="n">Crashes</th><th class="n">Ejections</th><th class="n">Shots</th><th class="n">Hits</th></tr>`)
	for _, p := range t.Pilots {
		fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td>%s</td><td class="n">%d</td><td class="n">%.1f</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td></tr>`,
			e(p.Name), e(strings.Join(p.Aircraft, ", ")), p.Sorties, p.Flight.Hours(), p.Landings, p.Traps, p.Kills, p.Ground, p.ShotDown, p.Crashes, p.Ejections, p.Shots, p.Hits)
	}
	if len(t.Pilots) == 0 {
		b.WriteString(`<tr><td colspan="13">No flying recorded in this period.</td></tr>`)
	}
	b.WriteString(`</table>`)
	board := t.GreenieBoard()
	b.WriteString(`<h2>Greenie board</h2>`)
	if len(board) == 0 {
		b.WriteString(`<p>No graded carrier passes in this period.</p>`)
	} else {
		b.WriteString(`<table><tr><th>Pilot</th><th class="n">Avg</th><th class="n">Passes</th><th class="n">Traps</th><th>Passes, oldest first</th></tr>`)
		for _, row := range board {
			avg := "-"
			if row.Avg >= 0 {
				avg = fmt.Sprintf("%.2f", row.Avg)
			}
			var sq strings.Builder
			for _, p := range row.Passes {
				fill, ink := debrief.BoardColours(p.Grade.Grade)
				label := ""
				if p.Wire > 0 {
					label = strconv.Itoa(p.Wire)
				}
				fmt.Fprintf(&sq, `<span class="sq" style="background:%s;color:%s">%s</span>`, fill, ink, label)
			}
			fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td class="n">%s</td><td class="n">%d</td><td class="n">%d</td><td>%s</td></tr>`, e(row.Pilot), avg, len(row.Passes), row.Traps, sq.String())
		}
		b.WriteString(`</table><div class="key">`)
		for _, g := range []string{"_OK_", "OK", "(OK)", "---", "B", "WO", "C", "WOFD"} {
			fill, ink := debrief.BoardColours(g)
			name := g
			if g == "WOFD" {
				name = "WO foul deck (not scored)"
			}
			fmt.Fprintf(&b, `<span class="sq" style="background:%s;color:%s">&nbsp;</span>%s &nbsp; `, fill, ink, e(name))
		}
		b.WriteString(`· the number is the wire · ranked by average grade points (_OK_ 5, OK 4, (OK) 3, B 2.5, --- 2, WO 1, C 0) · grades from DCS's LSO</div>`)
	}
	b.WriteString(`</body></html>`)
	return b.String()
}
