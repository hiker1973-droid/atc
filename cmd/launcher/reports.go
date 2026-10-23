package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/debrief"
	"github.com/vsfg7/atc/pkg/miz"
)

// Reports page (operator 2026-10-23: "lets add reporting functions",
// "probably for reporting need a section to add their claude key", "load dcs
// log and tacview", "also a way to upload the log and tacview file"). The
// person picks a session - a Tacview recording and/or a dcs.log, uploaded or
// found on this PC, plus Readback's own radio logs for the same hours - and
// pkg/debrief boils them down to a few KB of text. The page shows exactly
// that text before anything is sent; Generate sends it to Claude with the
// customer's own key (Settings, ANTHROPIC_API_KEY) and keeps the debrief in
// reports\ beside the launcher.

func reportsDir() string { return filepath.Join(rootDir, "reports") }

// A prepared debrief waits here between Preview and Generate.
type preparedReport struct {
	Records  string
	Title    string
	From     time.Time
	To       time.Time
	Sources  []string
	At       time.Time
	ReportNo string
	Mission  string
	Facts    []debrief.Fact
	Report   *debrief.Report // the pages; Generate adds Claude's words
	Preview  []byte          // the pages before Claude, for the Preview frame
}

var (
	preparedMu sync.Mutex
	prepared   = map[string]*preparedReport{}
)

// reportMeta is saved beside each report (<name>.json next to <name>.md).
// An AAR (ReportNo set) keeps only Claude's sections in the .md; its header
// and data block come from here. Earlier debriefs kept a whole page there.
type reportMeta struct {
	Name     string         `json:"name"`
	Title    string         `json:"title"`
	Created  time.Time      `json:"created"`
	From     time.Time      `json:"from"`
	To       time.Time      `json:"to"`
	Model    string         `json:"model"`
	Usage    debrief.Usage  `json:"usage"`
	Sources  []string       `json:"sources"`
	ReportNo string         `json:"reportNo,omitempty"`
	Mission  string         `json:"mission,omitempty"`
	Facts    []debrief.Fact `json:"facts,omitempty"`
	Discord  string         `json:"discord,omitempty"`      // one post (reports before the squadron's format)
	Posts    []string       `json:"discordPosts,omitempty"` // the Discord post, as messages under 2,000 characters
	HTML     bool           `json:"html,omitempty"` // the full report is <name>.html (the AAR in the debrief style)
}

// localSource is a dcs.log or Tacview recording found on this PC.
type localSource struct {
	Path     string    `json:"path"`
	Label    string    `json:"label"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// localDCSLogs: every DCS install's dcs.log under Saved Games (the server's
// and the game's), newest first.
func localDCSLogs() []localSource {
	home, _ := os.UserHomeDir()
	out := []localSource{} // [] not null in the JSON
	for _, pat := range []string{"dcs.log", "dcs.log.old"} {
		m, _ := filepath.Glob(filepath.Join(home, "Saved Games", "DCS*", "Logs", pat))
		for _, p := range m {
			if st, err := os.Stat(p); err == nil {
				out = append(out, localSource{Path: p, Label: filepath.Base(filepath.Dir(filepath.Dir(p))) + `\` + filepath.Base(p), Size: st.Size(), Modified: st.ModTime()})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out
}

// localTacview: the newest recordings in Tacview's default folder.
func localTacview() []localSource {
	home, _ := os.UserHomeDir()
	dirs := []string{filepath.Join(home, "Documents", "Tacview")}
	if od := os.Getenv("OneDrive"); od != "" {
		dirs = append(dirs, filepath.Join(od, "Documents", "Tacview"))
	}
	out := []localSource{} // [] not null in the JSON
	for _, d := range dirs {
		m, _ := filepath.Glob(filepath.Join(d, "*.acmi"))
		for _, p := range m {
			if st, err := os.Stat(p); err == nil {
				out = append(out, localSource{Path: p, Label: filepath.Base(p), Size: st.Size(), Modified: st.ModTime()})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	if len(out) > 15 {
		out = out[:15]
	}
	return out
}

// localMissions: the mission the DCS server has loaded (the hook's
// current_mission.txt), then the newest .miz files under the Missions
// folder and its subfolders.
func localMissions() []localSource {
	out := []localSource{}
	seen := map[string]bool{}
	addFile := func(p, label string) {
		if seen[strings.ToLower(p)] {
			return
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			seen[strings.ToLower(p)] = true
			out = append(out, localSource{Path: p, Label: label, Size: st.Size(), Modified: st.ModTime()})
		}
	}
	watch := *flagMizWatch
	if watch == "" {
		watch = filepath.Join(rootDir, "current_mission.txt")
	}
	if p := currentMissionFromHook(watch); p != "" {
		addFile(p, "Loaded on the server now: "+filepath.Base(p))
	}
	if p := *flagMizPath; p != "" {
		addFile(p, filepath.Base(p))
	}
	dir, _ := resolveMizDir()
	if dir == "" {
		return out
	}
	var found []localSource
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.Count(strings.TrimPrefix(p, dir), string(os.PathSeparator)) > 4 {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".miz") {
			if st, err := d.Info(); err == nil {
				found = append(found, localSource{Path: p, Label: filepath.Base(p), Size: st.Size(), Modified: st.ModTime()})
			}
		}
		return nil
	})
	sort.Slice(found, func(i, j int) bool { return found[i].Modified.After(found[j].Modified) })
	for _, f := range found {
		if len(out) >= 20 {
			break
		}
		addFile(f.Path, f.Label)
	}
	return out
}

// pickLocal returns path only if it is one of the listed files: the page
// cannot name any other file on the PC.
func pickLocal(path string, list []localSource) (string, error) {
	if path == "" {
		return "", nil
	}
	for _, s := range list {
		if strings.EqualFold(s.Path, path) {
			return s.Path, nil
		}
	}
	return "", fmt.Errorf("that file is not one of the ones listed")
}

func serveReportsUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("reports.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleReports: GET the page's state - the key, the models, the files on
// this PC and the saved debriefs.
func handleReports(w http.ResponseWriter, r *http.Request) {
	var models []map[string]string
	for _, m := range debrief.Models {
		models = append(models, map[string]string{"id": m.ID, "label": m.Label})
	}
	writeJSON(w, map[string]any{
		"canGenerate": who(r).Admin,
		"keySet":      userEnv("ANTHROPIC_API_KEY") != "",
		"models":      models,
		"dcsLogs":     localDCSLogs(),
		"tacview":     localTacview(),
		"missions":    localMissions(),
		"saved":       savedReports(),
	})
}

func savedReports() []reportMeta {
	m, _ := filepath.Glob(filepath.Join(reportsDir(), "*.json"))
	out := []reportMeta{}
	for _, p := range m {
		var meta reportMeta
		if b, err := os.ReadFile(p); err == nil && json.Unmarshal(b, &meta) == nil {
			out = append(out, meta)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

const maxUpload = 2 << 30 // a long Tacview recording, unzipped, can be over a GB

// handleReportPrepare (control, multipart): summarise the chosen sources and
// return exactly what Generate would send.
func handleReportPrepare(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "could not read the upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	name, _ := brandingSnapshot()["name"].(string)
	in := debrief.Inputs{Squadron: name, Focus: r.FormValue("focus")}
	var sources []string
	title := ""

	acmiPath, cleanup, err := reportSource(r, "acmi", "acmiPath", localTacview())
	defer cleanup()
	if err != nil {
		http.Error(w, "Tacview: "+err.Error(), http.StatusBadRequest)
		return
	}
	if acmiPath != "" {
		rec, err := debrief.ParseACMIFile(acmiPath)
		if err != nil {
			http.Error(w, "Tacview recording: "+err.Error(), http.StatusBadRequest)
			return
		}
		in.Tacview, title = rec, rec.Title
		sources = append(sources, "Tacview "+sourceName(r, "acmi", "acmiPath"))
	}
	logPath, cleanup2, err := reportSource(r, "dcslog", "dcsPath", localDCSLogs())
	defer cleanup2()
	if err != nil {
		http.Error(w, "dcs.log: "+err.Error(), http.StatusBadRequest)
		return
	}
	if logPath != "" {
		s, err := debrief.ParseDCSLogFile(logPath)
		if err != nil {
			http.Error(w, "dcs.log: "+err.Error(), http.StatusBadRequest)
			return
		}
		in.Server = s
		sources = append(sources, "dcs.log "+sourceName(r, "dcslog", "dcsPath"))
	}
	mizPath, cleanup3, err := reportSource(r, "miz", "mizPath", localMissions())
	defer cleanup3()
	if err != nil {
		http.Error(w, "Mission file: "+err.Error(), http.StatusBadRequest)
		return
	}
	if mizPath != "" {
		b, err := miz.ReadBriefing(mizPath)
		if err != nil {
			http.Error(w, "Mission file: "+err.Error(), http.StatusBadRequest)
			return
		}
		if r.FormValue("mizPath") == "" { // an upload: name it after the file, not the temp copy
			if _, h, err := r.FormFile("miz"); err == nil {
				if b.Name == strings.TrimSuffix(filepath.Base(mizPath), filepath.Ext(mizPath)) { // no sortie name: the temp file's
					b.Name = strings.TrimSuffix(h.Filename, filepath.Ext(h.Filename))
				}
				b.File = h.Filename
			}
		}
		in.Mission = b
		sources = append(sources, "mission "+b.File)
	}

	// The window for Readback's radio logs: the Tacview recording's hours
	// when asked, else the times given, else the dcs.log's.
	from, _ := time.Parse(time.RFC3339, r.FormValue("from"))
	to, _ := time.Parse(time.RFC3339, r.FormValue("to"))
	switch {
	case r.FormValue("matchTacview") == "1" && in.Tacview != nil && !in.Tacview.Start.IsZero():
		from, to = in.Tacview.Start.Add(-15*time.Minute), in.Tacview.End().Add(15*time.Minute)
	case !from.IsZero() && to.After(from):
	case in.Server != nil && !in.Server.Start.IsZero():
		from, to = in.Server.Start, in.Server.End
	}
	in.From, in.To = from, to
	if r.FormValue("radio") == "1" && !from.IsZero() {
		rad, err := debrief.ReadRadio(filepath.Join(rootDir, "logs"), from, to)
		if err == nil && len(rad.Lines) > 0 {
			in.Radio = rad
			sources = append(sources, fmt.Sprintf("Readback radio logs (%d lines)", len(rad.Lines)))
		}
	}
	if len(sources) == 0 {
		http.Error(w, "nothing to report from: choose a Tacview recording or a dcs.log, or a time window with Readback radio traffic", http.StatusBadRequest)
		return
	}
	var rec *debrief.Recording
	if in.Tacview != nil {
		rec = in.Tacview
	}
	in.Session = debrief.Analyze(rec, in.Server, from, to, time.Local)
	if !in.Session.HasEvents && in.Server != nil && len(in.Server.Sessions) == 0 {
		sources = append(sources, "(that dcs.log has no server events: use the DCS server's own log)")
	}
	title = debrief.MissionName(in)
	switch {
	case in.Tacview != nil && !in.Tacview.Start.IsZero():
		title = fmt.Sprintf("%s - %s", title, in.Tacview.Start.UTC().Format("2 Jan 2006 15:04Z"))
	case !from.IsZero():
		title = fmt.Sprintf("%s - %s", title, from.UTC().Format("2 Jan 2006 15:04Z"))
	}
	records := debrief.Build(in)
	id := newReportID()
	preparedMu.Lock()
	facts := debrief.Facts(in, sources)
	rpt := &debrief.Report{Session: in.Session, Facts: facts, ReportNo: debrief.ReportNumber(in), Squadron: name,
		LogoURI: string(squadronLogoDataURI()), Mission: debrief.MissionName(in), Sources: sources, Radio: in.Radio}
	for k, p := range prepared { // tracks are large: keep the newest few
		if len(prepared) >= 3 || time.Since(p.At) > time.Hour {
			delete(prepared, k)
		}
	}
	prepared[id] = &preparedReport{Records: records, Title: title, From: from, To: to, Sources: sources, At: time.Now(),
		ReportNo: rpt.ReportNo, Mission: rpt.Mission, Facts: facts, Report: rpt, Preview: []byte(rpt.HTML())}
	preparedMu.Unlock()

	stats := map[string]any{"sources": sources, "chars": len(records), "approxTokens": len(records) / 4, "facts": facts}
	if in.Tacview != nil {
		stats["aircraft"] = len(in.Tacview.Aircraft)
	}
	if in.Radio != nil {
		stats["radioLines"] = len(in.Radio.Lines)
	}
	if in.Server != nil {
		stats["serverProblems"] = len(in.Server.Problems)
	}
	stats["engagements"], stats["players"], stats["events"] = len(in.Session.Engagements), len(in.Session.Pilots), in.Session.HasEvents
	writeJSON(w, map[string]any{"id": id, "title": title, "records": records, "stats": stats,
		"from": from, "to": to})
}

// reportSource is the file for one source: an upload (saved to a temporary
// file, which cleanup removes) or one of the files listed on this PC.
func reportSource(r *http.Request, field, pathField string, local []localSource) (string, func(), error) {
	none := func() {}
	if f, h, err := r.FormFile(field); err == nil {
		defer f.Close()
		return saveUpload(f, h)
	}
	p, err := pickLocal(r.FormValue(pathField), local)
	return p, none, err
}

func saveUpload(f multipart.File, h *multipart.FileHeader) (string, func(), error) {
	if osf, ok := f.(*os.File); ok { // a large upload is already on disk: no second copy
		return osf.Name(), func() {}, nil
	}
	tmp, err := os.CreateTemp("", "readback-report-*-"+safeName(h.Filename))
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, err := io.Copy(tmp, f); err != nil {
		tmp.Close()
		cleanup()
		return "", func() {}, err
	}
	tmp.Close()
	return tmp.Name(), cleanup, nil
}

func sourceName(r *http.Request, field, pathField string) string {
	if _, h, err := r.FormFile(field); err == nil {
		return h.Filename + " (uploaded)"
	}
	return filepath.Base(r.FormValue(pathField))
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeName(s string) string {
	s = unsafeChars.ReplaceAllString(filepath.Base(s), "_")
	if len(s) > 60 {
		s = s[len(s)-60:]
	}
	return s
}

func newReportID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// handleReportGenerate (control, JSON {id, model}): send the prepared records
// to Claude and keep the debrief.
func handleReportGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct{ ID, Model string }
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	key := userEnv("ANTHROPIC_API_KEY")
	if key == "" {
		http.Error(w, "no Claude API key: add one in Settings, under Reports", http.StatusBadRequest)
		return
	}
	if !debrief.ValidModel(body.Model) {
		body.Model = debrief.Models[0].ID
	}
	preparedMu.Lock()
	p := prepared[body.ID]
	preparedMu.Unlock()
	if p == nil {
		http.Error(w, "that preview has expired: press Preview again", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Minute)
	defer cancel()
	text, usage, err := debrief.Write(ctx, key, body.Model, p.Records)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	meta := reportMeta{Title: p.Title, Created: time.Now(), From: p.From, To: p.To, Model: body.Model, Usage: usage,
		Sources: p.Sources, ReportNo: p.ReportNo, Mission: p.Mission, Facts: p.Facts, HTML: true}
	meta.Name = meta.Created.Format("2006-01-02_150405")
	if err := os.MkdirAll(reportsDir(), 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	ai := debrief.ParseAI(text)
	preparedMu.Lock()
	p.Report.AI, p.Report.Written, p.Report.Model = &ai, meta.Created, body.Model
	page := p.Report.HTML()
	meta.Posts = p.Report.DiscordPosts()
	preparedMu.Unlock()
	os.WriteFile(filepath.Join(reportsDir(), meta.Name+".html"), []byte(page), 0o644)
	os.WriteFile(filepath.Join(reportsDir(), meta.Name+".md"), []byte(aiMarkdown(ai)), 0o644)
	os.WriteFile(filepath.Join(reportsDir(), meta.Name+".records.txt"), []byte(p.Records), 0o644)
	mb, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(reportsDir(), meta.Name+".json"), mb, 0o644)
	writeJSON(w, map[string]any{"meta": meta})
}

// loadReport reads a saved report's meta and Markdown.
func loadReport(name string) (reportMeta, string, error) {
	var meta reportMeta
	if !reportName.MatchString(name) {
		return meta, "", fmt.Errorf("bad name")
	}
	md, err := os.ReadFile(filepath.Join(reportsDir(), name+".md"))
	if err != nil {
		return meta, "", fmt.Errorf("not found")
	}
	if b, err := os.ReadFile(filepath.Join(reportsDir(), name+".json")); err == nil {
		json.Unmarshal(b, &meta)
	}
	meta.Name = name
	return meta, string(md), nil
}

// fullMarkdown is the whole AAR as one Markdown file: header, data block,
// sections (an older debrief's file already is).
func fullMarkdown(meta reportMeta, md string) string {
	if meta.ReportNo == "" {
		return md
	}
	var b strings.Builder
	squadron, _ := brandingSnapshot()["name"].(string)
	fmt.Fprintf(&b, "# AFTER ACTION REPORT\n\n**%s** - %s - %s\n\n| | |\n|---|---|\n", squadron, meta.ReportNo, meta.Mission)
	for _, f := range meta.Facts {
		fmt.Fprintf(&b, "| %s | %s |\n", f.Label, strings.ReplaceAll(f.Value, "|", "/"))
	}
	b.WriteString("\n" + md)
	return b.String()
}

var reportName = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{6}$`)

// handleReportFile: GET ?name= one saved debrief; POST {op:"delete",name} (control).
func handleReportFile(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if code, err := controlAllowed(r); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		var body struct{ Op, Name string }
		json.NewDecoder(r.Body).Decode(&body)
		if body.Op != "delete" || !reportName.MatchString(body.Name) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		for _, ext := range []string{".md", ".json", ".records.txt", ".pdf", ".html", ".png"} {
			os.Remove(filepath.Join(reportsDir(), body.Name+ext))
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	name := r.URL.Query().Get("name")
	meta, md, err := loadReport(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+reportFileName(meta)+`.md"`)
		io.WriteString(w, fullMarkdown(meta, md))
		return
	}
	writeJSON(w, map[string]any{"meta": meta, "markdown": md})
}

// reportFileName: "AAR-20261023-0015 Caucasus Training" for downloads.
func reportFileName(meta reportMeta) string {
	if meta.ReportNo == "" {
		return "Debrief " + meta.Name
	}
	n := strings.TrimSpace(unsafeChars.ReplaceAllString(meta.Mission, " "))
	if len(n) > 60 {
		n = n[:60]
	}
	return strings.TrimSpace(meta.ReportNo + " " + n)
}

// handleReportPreview: GET /reports/preview?id= the pages of a prepared
// report before Claude writes (the Preview frame).
func handleReportPreview(w http.ResponseWriter, r *http.Request) {
	preparedMu.Lock()
	p := prepared[r.URL.Query().Get("id")]
	preparedMu.Unlock()
	if p == nil {
		http.Error(w, "that preview has expired: press Preview again", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(p.Preview)
}

// aiMarkdown keeps Claude's words as Markdown beside the report (the .md
// download and a record of what was written).
func aiMarkdown(a debrief.AIText) string {
	var b strings.Builder
	w := func(format string, x ...any) { fmt.Fprintf(&b, format+"\n", x...) }
	if a.BLUF != "" {
		w("**BLUF:** %s\n", a.BLUF)
	}
	if len(a.WhatWon) > 0 {
		w("## Key points")
		for i, p := range a.WhatWon {
			w("%d. **%s** %s", i+1, p.Title, p.Text)
		}
		w("")
	}
	for i, n := range a.EngagementNotes {
		w("## Engagement %d\n%s\n", i+1, n)
	}
	for _, sec := range []struct{ h, t string }{{"1. Mission overview", a.Overview}, {"2. What was planned", a.Planned},
		{"6. Air traffic control and comms", a.Comms}, {"Safety", a.Safety}, {"Server", a.Server}} {
		if sec.t != "" {
			w("## %s\n%s\n", sec.h, sec.t)
		}
	}
	if len(a.Sustain)+len(a.Improve) > 0 {
		w("## 7. Sustain / Improve\n**Sustain**")
		for _, x := range a.Sustain {
			w("- %s", x)
		}
		w("\n**Improve**")
		for _, x := range a.Improve {
			w("- %s", x)
		}
		w("")
	}
	if len(a.Recommendations) > 0 {
		w("## 8. Recommendations and follow-up actions")
		for i, x := range a.Recommendations {
			w("%d. %s [%s]", i+1, x.Text, x.Owner)
		}
	}
	return b.String()
}
