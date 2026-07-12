// Command launcher is the vSFG-7 ops panel — a tiny HTTP app that discovers
// every run_*.bat / start_*.bat next to it, parses the roles each spawns,
// and gives you start / stop / restart / log-tail / health via a browser.
//
// Built portable on purpose: the launcher resolves its root from os.Executable
// so it can be dropped into training-1 (or any box) without path edits. All
// external endpoints are flags, not constants.
package main

import (
	"context"
	"embed"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/miz"
)

//go:embed ui.html fleet.html logs.html
var uiFS embed.FS

var (
	flagListen      = flag.String("listen", ":7000", "HTTP listen address (avoid 6000 — blocked by Chrome as X11)")
	flagRoot        = flag.String("root", "", "SkyeyeATC root dir (default: directory of this binary)")
	flagSRSAddr     = flag.String("srs-addr", "192.168.1.221:5004", "SRS address for health probe")
	flagTacviewAddr = flag.String("tacview-addr", "192.168.1.221:42676", "Tacview address for health probe")
	flagMizDir      = flag.String("miz-dir", `C:\Users\Administrator\Saved Games\DCS.dcs_serverrelease\Missions`, "Dir scanned for newest .miz when --miz-path is empty")
	flagMizPath     = flag.String("miz-path", "", "Path to a specific .miz for /api/miz-weather (overrides --miz-dir; keep in sync with the roles' SKYEYE_MIZ)")
	flagFleet       = flag.String("fleet", "host@192.168.1.231:7000,dev@192.168.1.221:7000,training1@192.168.1.220:7000,foothold@192.168.1.222:7000", "Rigs the /fleet monitor polls: name@host:port,...")
)

var fleetRigs []Rig

// Role is one spawned vSFG-7 process: a single `start "Title" cmd /k "..."`
// line inside a .bat file. Multi-role bats (start_towers.bat) produce several
// Role entries, all pointing back at the same Bat.
type Role struct {
	Name          string `json:"name"`
	Bat           string `json:"bat"`
	Region        string `json:"region"` // pg | caucasus | germany (from bat name)
	Airfield      string `json:"airfield"`
	LogFile       string `json:"logFile"`
	DashboardPort int    `json:"dashboardPort,omitempty"`
	Status        string `json:"status"`
	PID           int    `json:"pid,omitempty"`
}

type Health struct {
	SRS       bool      `json:"srs"`
	Tacview   bool      `json:"tacview"`
	Tacview64 bool      `json:"tacview64"`
	OpenAI    bool      `json:"openai"`
	At        time.Time `json:"at"`
}

var (
	rootDir      string
	roles        []Role
	rolesMu      sync.Mutex
	cachedHealth Health
	healthMu     sync.Mutex
)

func main() {
	flag.Parse()

	if *flagRoot != "" {
		rootDir = *flagRoot
	} else if exe, err := os.Executable(); err == nil {
		rootDir = filepath.Dir(exe)
	} else {
		rootDir, _ = os.Getwd()
	}
	fmt.Printf("vSFG-7 Launcher — root=%s listen=%s\n", rootDir, *flagListen)
	fmt.Printf("Open http://localhost%s/ in a browser.\n", *flagListen)

	discoverRoles()
	fleetRigs = parseFleet(*flagFleet)
	cachedVersion = computeVersion()
	go healthLoop()
	go alertLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveUI)
	mux.HandleFunc("/fleet", serveFleetUI)
	mux.HandleFunc("/logs", serveLogsUI)
	mux.HandleFunc("/api/fleet", handleFleet)
	mux.HandleFunc("/api/version", handleVersion)
	mux.HandleFunc("/api/alerts", handleAlerts)
	mux.HandleFunc("/api/rig-log", handleRigLog)
	mux.HandleFunc("/api/roles", handleRoles)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/start", handleStart)
	mux.HandleFunc("/api/stop", handleStop)
	mux.HandleFunc("/api/start-region", handleStartRegion)
	mux.HandleFunc("/api/stop-region", handleStopRegion)
	mux.HandleFunc("/api/restart", handleRestart)
	mux.HandleFunc("/api/log", handleLog)
	mux.HandleFunc("/api/rescan", handleRescan)
	mux.HandleFunc("/api/miz-weather", handleMizWeather)
	mux.HandleFunc("/tower/", handleTowerProxy)

	if err := http.ListenAndServe(*flagListen, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ── Bat discovery ─────────────────────────────────────────────────────────────

var (
	reStart         = regexp.MustCompile(`(?i)start\s+"([^"]+)"\s+cmd\s+/[kc]\s+"([^"]+)"`)
	reAirfield      = regexp.MustCompile(`--airfield\s+(\S+)`)
	reDashboardPort = regexp.MustCompile(`--dashboard-port\s+(\d+)`)
)

// discoverRoles scans rootDir for bats that use the `start "Title" cmd /k`
// pattern and builds a flat Role list. Old-style bats with a direct atc.exe
// call (no start wrapper) are skipped — they're unmanageable by title-match
// and were flagged for deletion anyway.
func discoverRoles() {
	rolesMu.Lock()
	defer rolesMu.Unlock()
	roles = nil

	ignore := map[string]bool{
		"build.bat":          true,
		"launcher.bat":       true,
		"start_launcher.bat": true,
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".bat") || ignore[lower] {
			continue
		}
		if !strings.HasPrefix(lower, "run_") && !strings.HasPrefix(lower, "start_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, name))
		if err != nil {
			continue
		}
		for _, m := range reStart.FindAllStringSubmatch(string(data), -1) {
			roles = append(roles, roleFromCmd(name, m[1], m[2]))
		}
	}
}

func roleFromCmd(bat, title, cmd string) Role {
	r := Role{Name: title, Bat: bat, Region: regionForBat(bat), Airfield: "OMDM"} // atc.exe default
	if m := reAirfield.FindStringSubmatch(cmd); m != nil {
		r.Airfield = strings.ToUpper(m[1])
	}
	if m := reDashboardPort.FindStringSubmatch(cmd); m != nil {
		fmt.Sscanf(m[1], "%d", &r.DashboardPort)
	}
	// --airfield defaults to OMDM even in ATIS/Command/Marshal-only modes, so
	// the mode flag has to win or these roles all read atc-omdm.log. Mirrors the
	// log-slug switch in cmd/atc/main.go.
	switch {
	case strings.Contains(cmd, "--atis-only"):
		r.LogFile = "atc-atis.log"
	case strings.Contains(cmd, "--command-only"):
		r.LogFile = "atc-command.log"
	case strings.Contains(cmd, "--marshal-only"):
		r.LogFile = "atc-marshal.log"
	default:
		r.LogFile = "atc-" + strings.ToLower(r.Airfield) + ".log"
	}
	return r
}

// regionForBat maps a role's source bat filename to its theatre. Region-specific
// bats carry a suffix (_caucasus / _germany); the un-suffixed PG bats
// (start_towers.bat, start_atis.bat, start_marshal.bat, ...) default to "pg".
// Keep the keys in sync with regionBats and the ui.html THEATRES config.
func regionForBat(bat string) string {
	l := strings.ToLower(bat)
	switch {
	case strings.Contains(l, "caucasus"):
		return "caucasus"
	case strings.Contains(l, "germany"):
		return "germany"
	default:
		return "pg"
	}
}

// ── Process detection (Windows tasklist) ─────────────────────────────────────

// enumerateCmdWindows returns a map of lowercased window title → PID for
// every cmd.exe instance currently showing a window. One tasklist call per
// /api/roles request — much cheaper than one per role.
func enumerateCmdWindows() map[string]int {
	out, err := exec.Command("tasklist", "/v", "/fi", "IMAGENAME eq cmd.exe", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil
	}
	m := make(map[string]int)
	r := csv.NewReader(strings.NewReader(string(out)))
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 9 {
			continue
		}
		title := strings.TrimSpace(rec[len(rec)-1])
		var pid int
		fmt.Sscanf(rec[1], "%d", &pid)
		m[strings.ToLower(title)] = pid
	}
	return m
}

// findWindowPID matches `name` against window titles, accepting either an
// exact match or a "<name> - <cmdline>" prefix. Newer Windows builds append
// the spawned command to the console title, so a strict equality check
// reports running roles as stopped.
func findWindowPID(wins map[string]int, name string) (int, bool) {
	key := strings.ToLower(name)
	if pid, ok := wins[key]; ok {
		return pid, true
	}
	prefix := key + " - "
	for title, pid := range wins {
		if strings.HasPrefix(title, prefix) {
			return pid, true
		}
	}
	return 0, false
}

// killTree force-kills pid and any descendants. Needed because each .bat
// window owns a child atc.exe process — /t walks the tree.
func killTree(pid int) error {
	return exec.Command("taskkill", "/f", "/t", "/pid", fmt.Sprintf("%d", pid)).Run()
}

// ── Health probes ─────────────────────────────────────────────────────────────

func healthLoop() {
	for {
		updateHealth()
		time.Sleep(15 * time.Second)
	}
}

func updateHealth() {
	h := Health{
		At:        time.Now(),
		SRS:       tcpProbe(*flagSRSAddr, 2*time.Second),
		Tacview:   tcpProbe(*flagTacviewAddr, 2*time.Second),
		Tacview64: tacview64Probe(),
		OpenAI:    openaiProbe(3 * time.Second),
	}
	healthMu.Lock()
	cachedHealth = h
	healthMu.Unlock()
}

// tacview64Probe checks whether the standalone Tacview64.exe viewer is
// running. The viewer is operator-launched (not auto-started with DCS) and
// easy to forget after a VM reboot. This is Windows-only and harmless if
// tasklist is unavailable — returns false.
func tacview64Probe() bool {
	cmd := exec.Command("tasklist", "/fi", "imagename eq Tacview64.exe", "/nh")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "tacview64.exe")
}

func tcpProbe(addr string, timeout time.Duration) bool {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// openaiProbe treats any sub-500 response as "up" — 401 (no key) still means
// the API is reachable, which is what the health dot is really asking.
func openaiProbe(timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// ── Build version ───────────────────────────────────────────────────────────

// VersionInfo is the deployed build, read from the VCS stamps Go embeds when
// `go build` runs inside the git tree (no ldflags needed). Lets the fleet
// monitor show exactly which commit each rig is running.
type VersionInfo struct {
	Version  string `json:"version"` // human: git-describe (e.g. "v1.5.0-18-g6603c5b")
	Revision string `json:"revision"`
	Short    string `json:"short"`
	Time     string `json:"time"`
	Modified bool   `json:"modified"`
	Go       string `json:"go"`
}

// cachedVersion is computed once at startup (computeVersion shells out to git).
var cachedVersion VersionInfo

// buildVersion reads the VCS stamps Go embeds during `go build` — accurate to
// the exact commit the running binary was built from.
func buildVersion() VersionInfo {
	v := VersionInfo{Revision: "unknown", Short: "unknown"}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return v
	}
	v.Go = bi.GoVersion
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			v.Revision = s.Value
			if len(s.Value) >= 7 {
				v.Short = s.Value[:7]
			} else if s.Value != "" {
				v.Short = s.Value
			}
		case "vcs.time":
			v.Time = s.Value
		case "vcs.modified":
			v.Modified = s.Value == "true"
		}
	}
	return v
}

// computeVersion resolves the binary's build commit to the nearest semver tag
// via `git describe` (run in rootDir, the repo checkout). Gives a readable
// version like "v1.5.0-18-g6603c5b"; falls back to the short SHA if git or the
// tag isn't available. Describing the *build* revision (not HEAD) keeps this
// honest even when the repo was pulled but not yet rebuilt.
func computeVersion() VersionInfo {
	v := buildVersion()
	ref := v.Revision
	if ref == "" || ref == "unknown" {
		ref = "HEAD"
	}
	if out, err := exec.Command("git", "-C", rootDir, "describe", "--tags", "--always", ref).Output(); err == nil {
		if d := strings.TrimSpace(string(out)); d != "" {
			v.Version = d
		}
	}
	if v.Version == "" {
		v.Version = v.Short
	}
	return v
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, cachedVersion)
}

// ── Fault alerts ────────────────────────────────────────────────────────────

// Alert is one active fault surfaced by the fleet monitor. Sources are a log
// filename (log-scan alerts), a role name (duplicate process), or a subsystem
// (SRS/Tacview health while roles run).
type Alert struct {
	Level   string    `json:"level"` // "error" | "warn"
	Source  string    `json:"source"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// faultSignatures are the known bad log messages from CLAUDE.md's fault list.
// Any level=error line also alerts regardless of message.
var faultSignatures = []string{
	"SRS disconnected", "SRS TCP failed", "ExternalAudio", "TTS prewarm failed",
	"Whisper returned empty", "Dashboard server error",
}

var (
	cachedAlerts []Alert
	alertsMu     sync.Mutex
)

func alertLoop() {
	for {
		updateAlerts()
		time.Sleep(20 * time.Second)
	}
}

// updateAlerts recomputes the active-alert set: recent log faults, duplicate
// role windows, and SRS/Tacview down while roles are running.
func updateAlerts() {
	alerts := scanLogAlerts()

	titles := enumerateCmdWindowTitles()
	rolesMu.Lock()
	rs := append([]Role(nil), roles...)
	rolesMu.Unlock()
	running := 0
	for _, role := range rs {
		n := countRoleWindows(titles, role.Name)
		if n >= 1 {
			running++
		}
		if n > 1 {
			alerts = append(alerts, Alert{Level: "warn", Source: role.Name,
				Message: fmt.Sprintf("duplicate process — %d windows for this role", n), Time: time.Now()})
		}
	}
	if running > 0 {
		healthMu.Lock()
		h := cachedHealth
		healthMu.Unlock()
		if !h.SRS {
			alerts = append(alerts, Alert{Level: "error", Source: "SRS",
				Message: "SRS unreachable while roles are running", Time: time.Now()})
		}
		if !h.Tacview {
			alerts = append(alerts, Alert{Level: "warn", Source: "Tacview",
				Message: "Tacview telemetry unreachable while roles are running", Time: time.Now()})
		}
	}

	alertsMu.Lock()
	cachedAlerts = alerts
	alertsMu.Unlock()
}

type logLine struct {
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// scanLogAlerts tails every logs/*.log and returns recent (last 5 min) fault
// lines, deduped by source+message (keeping the newest), newest-first, capped.
func scanLogAlerts() []Alert {
	dir := filepath.Join(rootDir, "logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	cutoff := time.Now().Add(-5 * time.Minute)
	seen := map[string]Alert{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		for _, ln := range tailLines(filepath.Join(dir, e.Name()), 16*1024) {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			var l logLine
			if json.Unmarshal([]byte(ln), &l) != nil {
				continue
			}
			if l.Time.Before(cutoff) {
				continue
			}
			fault := l.Level == "error"
			if !fault {
				for _, sig := range faultSignatures {
					if strings.Contains(l.Message, sig) {
						fault = true
						break
					}
				}
			}
			if !fault {
				continue
			}
			key := e.Name() + "|" + l.Message
			if prev, ok := seen[key]; !ok || l.Time.After(prev.Time) {
				lvl := l.Level
				if lvl == "" {
					lvl = "warn"
				}
				seen[key] = Alert{Level: lvl, Source: e.Name(), Message: l.Message, Time: l.Time}
			}
		}
	}
	out := make([]Alert, 0, len(seen))
	for _, a := range seen {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// tailLines returns the lines in the last n bytes of a file (dropping the
// partial first line when the file is larger than n).
func tailLines(path string, n int64) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	start := int64(0)
	if info.Size() > n {
		start = info.Size() - n
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	return lines
}

// enumerateCmdWindowTitles returns every cmd.exe window title (lowercased),
// duplicates included — enumerateCmdWindows dedupes, which hides zombies.
func enumerateCmdWindowTitles() []string {
	out, err := exec.Command("tasklist", "/v", "/fi", "IMAGENAME eq cmd.exe", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil
	}
	var titles []string
	r := csv.NewReader(strings.NewReader(string(out)))
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 9 {
			continue
		}
		titles = append(titles, strings.ToLower(strings.TrimSpace(rec[len(rec)-1])))
	}
	return titles
}

// countRoleWindows counts windows matching a role title (exact or the
// "<title> - <cmdline>" form newer Windows builds produce).
func countRoleWindows(titles []string, name string) int {
	key := strings.ToLower(name)
	prefix := key + " - "
	n := 0
	for _, t := range titles {
		if t == key || strings.HasPrefix(t, prefix) {
			n++
		}
	}
	return n
}

func handleAlerts(w http.ResponseWriter, _ *http.Request) {
	alertsMu.Lock()
	a := append([]Alert(nil), cachedAlerts...)
	alertsMu.Unlock()
	writeJSON(w, a)
}

// ── Fleet monitor ─────────────────────────────────────────────────────────────

// Rig is one box in the vSFG-7 fleet the /fleet monitor polls.
type Rig struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// parseFleet parses "name@host:port,name@host:port,..." into a rig list.
// A bare "host" or "host:port" (no name) uses the host as the name; a missing
// port defaults to 7000 (the launcher port).
func parseFleet(s string) []Rig {
	var rigs []Rig
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, addr := part, part
		if at := strings.IndexByte(part, '@'); at >= 0 {
			name, addr = part[:at], part[at+1:]
		}
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			host, portStr = addr, "7000"
		}
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			port = 7000
		}
		if name == part { // "host:port" with no name → name after the port strip
			name = host
		}
		rigs = append(rigs, Rig{Name: name, Host: host, Port: port})
	}
	return rigs
}

// RigStatus is a point-in-time health snapshot of one rig for the fleet view.
type RigStatus struct {
	Name       string    `json:"name"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	HostUp     bool      `json:"hostUp"`
	LauncherUp bool      `json:"launcherUp"`
	Health     *Health      `json:"health,omitempty"`
	Version    *VersionInfo `json:"version,omitempty"`
	RolesTotal int          `json:"rolesTotal"`
	Running    []Role       `json:"running,omitempty"`
	Alerts     []Alert      `json:"alerts,omitempty"`
	Err        string       `json:"err,omitempty"`
	At         time.Time    `json:"at"`
}

var fleetClient = &http.Client{Timeout: 2500 * time.Millisecond}

func fleetGetJSON(url string, v any) error {
	resp, err := fleetClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// pollRig fetches a rig's launcher /api/health + /api/roles. If the launcher is
// unreachable it falls back to a TCP probe of SMB 445 so we can still tell
// "host up, launcher down" from "host offline".
func pollRig(rig Rig) RigStatus {
	rs := RigStatus{Name: rig.Name, Host: rig.Host, Port: rig.Port, At: time.Now()}
	base := fmt.Sprintf("http://%s:%d", rig.Host, rig.Port)

	var h Health
	if err := fleetGetJSON(base+"/api/health", &h); err == nil {
		rs.LauncherUp, rs.HostUp = true, true
		rs.Health = &h
	} else {
		rs.Err = err.Error()
		if c, e := net.DialTimeout("tcp", net.JoinHostPort(rig.Host, "445"), 1500*time.Millisecond); e == nil {
			c.Close()
			rs.HostUp = true
		}
	}
	if rs.LauncherUp {
		var roles []Role
		if err := fleetGetJSON(base+"/api/roles", &roles); err == nil {
			rs.RolesTotal = len(roles)
			for _, r := range roles {
				if r.Status == "running" {
					rs.Running = append(rs.Running, r)
				}
			}
		}
		var ver VersionInfo
		if err := fleetGetJSON(base+"/api/version", &ver); err == nil {
			rs.Version = &ver
		}
		var alerts []Alert
		if err := fleetGetJSON(base+"/api/alerts", &alerts); err == nil {
			rs.Alerts = alerts
		}
	}
	return rs
}

// pollFleet polls every rig concurrently and preserves flag order.
func pollFleet(rigs []Rig) []RigStatus {
	out := make([]RigStatus, len(rigs))
	var wg sync.WaitGroup
	for i, rig := range rigs {
		wg.Add(1)
		go func(i int, rig Rig) {
			defer wg.Done()
			out[i] = pollRig(rig)
		}(i, rig)
	}
	wg.Wait()
	return out
}

func handleFleet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, pollFleet(fleetRigs))
}

func serveFleetUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("fleet.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func serveLogsUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("logs.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func rigByName(name string) (Rig, bool) {
	for _, r := range fleetRigs {
		if strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return Rig{}, false
}

// handleRigLog proxies a role's recent log (last 32KB) from any fleet rig so the
// /logs page only ever fetches the local launcher — no cross-origin request to
// each rig. rig=<name> resolves against the -fleet list; role=<role name>.
func handleRigLog(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")
	if role == "" {
		http.Error(w, "missing role", http.StatusBadRequest)
		return
	}
	rig, ok := rigByName(r.URL.Query().Get("rig"))
	if !ok {
		http.Error(w, "unknown rig", http.StatusBadRequest)
		return
	}
	target := fmt.Sprintf("http://%s:%d/api/log?name=%s", rig.Host, rig.Port, url.QueryEscape(role))
	resp, err := fleetClient.Get(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ── Role actions ──────────────────────────────────────────────────────────────

func findRole(name string) *Role {
	rolesMu.Lock()
	defer rolesMu.Unlock()
	for i := range roles {
		if roles[i].Name == name {
			return &roles[i]
		}
	}
	return nil
}

// startRole shells out to `cmd /c start "" <bat>` so the bat opens in a new
// console window (inherits normal behavior — the bat's own `start "Title"`
// lines create the actual role windows).
func startRole(name string) error {
	r := findRole(name)
	if r == nil {
		return fmt.Errorf("unknown role: %s", name)
	}
	cmd := exec.Command("cmd", "/c", "start", "", filepath.Join(rootDir, r.Bat))
	cmd.Dir = rootDir
	return cmd.Start()
}

func stopRole(name string) error {
	wins := enumerateCmdWindows()
	pid, ok := findWindowPID(wins, name)
	if !ok {
		return fmt.Errorf("not running: %s", name)
	}
	return killTree(pid)
}

// regionBats maps a theatre to its single-shot launcher bat (ATIS + towers +
// command, and Marshal/Deckboss for PG). These deliberately do NOT call
// start_launcher.bat — the launcher is already running when the dashboard
// triggers a region start. Keep keys in sync with regionForBat + ui.html.
var regionBats = map[string]string{
	"pg":       "start_region_pg.bat",
	"caucasus": "start_region_caucasus.bat",
	"germany":  "start_region_germany.bat",
}

// startRegion runs a theatre's region bat, spawning all of its role windows in
// one shot (same `cmd /c start "" <bat>` mechanism as startRole).
func startRegion(region string) error {
	bat, ok := regionBats[region]
	if !ok {
		return fmt.Errorf("unknown region: %s", region)
	}
	full := filepath.Join(rootDir, bat)
	if _, err := os.Stat(full); err != nil {
		return fmt.Errorf("region bat missing: %s", bat)
	}
	cmd := exec.Command("cmd", "/c", "start", "", full)
	cmd.Dir = rootDir
	return cmd.Start()
}

// stopRegion kills every running role window belonging to the region. Returns
// the number of roles killed so the dashboard can report "nothing was running".
func stopRegion(region string) (int, error) {
	rolesMu.Lock()
	var names []string
	for i := range roles {
		if roles[i].Region == region {
			names = append(names, roles[i].Name)
		}
	}
	rolesMu.Unlock()

	wins := enumerateCmdWindows()
	killed := 0
	var firstErr error
	for _, n := range names {
		if pid, ok := findWindowPID(wins, n); ok {
			if err := killTree(pid); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			killed++
		}
	}
	return killed, firstErr
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func serveUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := uiFS.ReadFile("ui.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// proxyPorts is the allowlist of local dashboard ports the launcher will
// reverse-proxy. Restricting to known role ports keeps /tower/ from being
// abused as an open proxy to arbitrary localhost services. Covers both
// theatres: PG towers 6001-6003 + Marshal 6004 + Deckboss 6005, Caucasus
// towers 6011-6014, and Cold War Germany towers 6021-6028.
var proxyPorts = map[int]bool{
	6001: true, 6002: true, 6003: true, 6004: true, 6005: true,
	6011: true, 6012: true, 6013: true, 6014: true,
	6021: true, 6022: true, 6023: true, 6024: true,
	6025: true, 6026: true, 6027: true, 6028: true,
}

// handleTowerProxy reverse-proxies /tower/<port>/<rest> to http://127.0.0.1:<port>/<rest>
// so a remote browser only ever talks to the launcher (:7000) — the per-tower
// dashboards need not be exposed. This is what makes the dashboard work behind
// a single Cloudflare Tunnel hostname. Streams (SSE /ws/log) pass through via
// FlushInterval=-1. Port is allowlist-checked.
func handleTowerProxy(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/tower/")
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		http.Error(w, "bad tower path — want /tower/<port>/<path>", http.StatusBadRequest)
		return
	}
	port, err := strconv.Atoi(rest[:slash])
	if err != nil || !proxyPorts[port] {
		http.Error(w, "tower port not allowed", http.StatusForbidden)
		return
	}
	target := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", port)}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1 // stream immediately for SSE (/ws/log)
	// A tower whose process isn't running refuses the connection — a normal
	// "role stopped" state, not a launcher fault. httputil's default handler
	// logs "http: proxy error: ..." on every dashboard poll (~every 3s per
	// down tower), which floods the console when a theatre's roles aren't up.
	// Return a quiet 503 instead; the UI already renders a non-OK /status as
	// tower-down.
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	// Rewrite the path to drop the /tower/<port> prefix; NewSingleHostReverseProxy's
	// director keeps r.URL.Path (target.Path is empty) and preserves RawQuery.
	r.URL.Path = "/" + rest[slash+1:]
	proxy.ServeHTTP(w, r)
}

func handleRoles(w http.ResponseWriter, _ *http.Request) {
	rolesMu.Lock()
	out := make([]Role, len(roles))
	copy(out, roles)
	rolesMu.Unlock()

	wins := enumerateCmdWindows()
	for i := range out {
		if pid, ok := findWindowPID(wins, out[i].Name); ok {
			out[i].Status = "running"
			out[i].PID = pid
		} else {
			out[i].Status = "stopped"
		}
	}
	writeJSON(w, out)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	healthMu.Lock()
	h := cachedHealth
	healthMu.Unlock()
	writeJSON(w, h)
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	if err := startRole(r.URL.Query().Get("name")); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "started"})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	if err := stopRole(r.URL.Query().Get("name")); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func handleStartRegion(w http.ResponseWriter, r *http.Request) {
	if err := startRegion(r.URL.Query().Get("region")); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "started"})
}

func handleStopRegion(w http.ResponseWriter, r *http.Request) {
	killed, err := stopRegion(r.URL.Query().Get("region"))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"status": "stopped", "killed": killed})
}

func handleRestart(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	_ = stopRole(name) // best-effort — may already be down
	time.Sleep(1 * time.Second)
	if err := startRole(name); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "restarted"})
}

func handleLog(w http.ResponseWriter, r *http.Request) {
	role := findRole(r.URL.Query().Get("name"))
	if role == nil {
		http.Error(w, "unknown role", 404)
		return
	}
	path := filepath.Join(rootDir, "logs", role.LogFile)
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	const tailBytes = 32 * 1024
	if start := info.Size() - tailBytes; start > 0 {
		f.Seek(start, io.SeekStart)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.Copy(w, f)
}

func handleRescan(w http.ResponseWriter, _ *http.Request) {
	discoverRoles()
	rolesMu.Lock()
	n := len(roles)
	rolesMu.Unlock()
	writeJSON(w, map[string]int{"roles": n})
}

// handleMizWeather finds the newest .miz in flagMizDir, parses its weather
// block, and returns the values in the same shape the Set-Weather modal
// expects. windDir is true degrees — the dashboard /weather endpoint already
// treats its incoming windDir as true.
func handleMizWeather(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Priority mirrors atc.exe's boot seed (cmd/atc/main.go): an explicit
	// --miz-path (fed SKYEYE_MIZ by start_launcher.bat) wins, so the dashboard
	// weather widget reflects the same mission the roles actually loaded rather
	// than whatever .miz was saved most recently on disk.
	path := *flagMizPath
	if path == "" {
		p, err := miz.FindNewestMiz(*flagMizDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		path = p
	}
	wx, err := miz.ReadMizWeather(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"mizName": filepath.Base(path),
		"windDir": wx.WindDirTrue,
		"windKts": wx.WindKts,
		"ceilFt":  wx.CeilFt,
		"visNm":   wx.VisNm,
		"altInHg": wx.AltInHg,
		"tempC":   wx.TempC,
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
