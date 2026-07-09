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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/miz"
)

//go:embed ui.html
var uiFS embed.FS

var (
	flagListen      = flag.String("listen", ":7000", "HTTP listen address (avoid 6000 — blocked by Chrome as X11)")
	flagRoot        = flag.String("root", "", "SkyeyeATC root dir (default: directory of this binary)")
	flagSRSAddr     = flag.String("srs-addr", "192.168.1.221:5004", "SRS address for health probe")
	flagTacviewAddr = flag.String("tacview-addr", "192.168.1.221:42676", "Tacview address for health probe")
	flagMizDir      = flag.String("miz-dir", `C:\Users\Administrator\Saved Games\DCS.dcs_serverrelease\Missions`, "Dir scanned for newest .miz when --miz-path is empty")
	flagMizPath     = flag.String("miz-path", "", "Path to a specific .miz for /api/miz-weather (overrides --miz-dir; keep in sync with the roles' SKYEYE_MIZ)")
)

// Role is one spawned vSFG-7 process: a single `start "Title" cmd /k "..."`
// line inside a .bat file. Multi-role bats (start_towers.bat) produce several
// Role entries, all pointing back at the same Bat.
type Role struct {
	Name          string `json:"name"`
	Bat           string `json:"bat"`
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
	go healthLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveUI)
	mux.HandleFunc("/api/roles", handleRoles)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/start", handleStart)
	mux.HandleFunc("/api/stop", handleStop)
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
	r := Role{Name: title, Bat: bat, Airfield: "OMDM"} // atc.exe default
	if m := reAirfield.FindStringSubmatch(cmd); m != nil {
		r.Airfield = strings.ToUpper(m[1])
	}
	if m := reDashboardPort.FindStringSubmatch(cmd); m != nil {
		fmt.Sscanf(m[1], "%d", &r.DashboardPort)
	}
	r.LogFile = "atc-" + strings.ToLower(r.Airfield) + ".log"
	return r
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
// theatres: PG towers 6001-6003 + Marshal 6004 + Deckboss 6005, and Caucasus
// towers 6011-6014.
var proxyPorts = map[int]bool{
	6001: true, 6002: true, 6003: true, 6004: true, 6005: true,
	6011: true, 6012: true, 6013: true, 6014: true,
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
