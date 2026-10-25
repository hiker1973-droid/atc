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

	"github.com/vsfg7/atc/pkg/license"
	"github.com/vsfg7/atc/pkg/miz"
)

//go:embed ui.html fleet.html logs.html pilots.html settings.html setup.html reports.html stats.html bugreport.html logo.png readback.png help
var uiFS embed.FS

var (
	flagListen      = flag.String("listen", ":7000", "HTTP listen address (avoid 6000 — blocked by Chrome as X11)")
	flagRoot        = flag.String("root", "", "SkyeyeATC root dir (default: directory of this binary)")
	flagSRSAddr     = flag.String("srs-addr", "192.168.1.221:5004", "SRS address for health probe")
	flagTacviewAddr = flag.String("tacview-addr", "192.168.1.221:42676", "Tacview address for health probe")
	flagMizDir      = flag.String("miz-dir", "", "Dir scanned for newest .miz when --miz-path is empty (default: this user's DCS Saved Games)")
	flagMizPath     = flag.String("miz-path", "", "Path to a specific .miz for /api/miz-weather (overrides --miz-dir; keep in sync with the roles' SKYEYE_MIZ)")
	flagMizWatch    = flag.String("miz-watch-file", "", "File the DCS server hook writes the loaded mission's path to; when it names an existing .miz, /api/miz-weather reports that mission (matches atc.exe --miz-watch-file; default current_mission.txt in the root dir)")
	flagTacviewPort = flag.Int("tacview-port", 42676,
		"Tacview real-time telemetry port used when reading a remote rig's air picture")
	flagSkyEyeImage = flag.String("skyeye-image", "skyeye.exe", "SkyEye GCI process image name the health probe looks for")
	flagSkyEyeDir   = flag.String("skyeye-dir", "", "SkyEye install dir, used to tell GCI offline from not installed (default: Skyeye beside the SkyeyeATC root)")
	flagFleet       = flag.String("fleet", "host@192.168.1.231:7000,night=Night Server@192.168.1.221:7000,training1=Day Server@192.168.1.220:7000,foothold@192.168.1.222:7000", "Rigs the /fleet monitor polls: name[=Display Label]@host:port,...")
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

	cmd string // the bat's atc.exe command line, for process-based detection
}

type Health struct {
	SRS       bool      `json:"srs"`
	Tacview   bool      `json:"tacview"`
	Tacview64 bool      `json:"tacview64"`
	OpenAI    bool      `json:"openai"`
	GCI       string    `json:"gci"` // GCI: online | offline | absent
	GCIPID    int       `json:"gciPid,omitempty"`
	// GCIName says which GCI is up: our atc.exe role ("Darkstar (atc.exe)")
	// or "skyeye.exe". GCIBoth: both at once - they talk over each other.
	GCIName string `json:"gciName,omitempty"`
	GCIBoth bool   `json:"gciBoth,omitempty"`
	// From our GCI's status file (cmd/atc/gci_coverage.go): no AWACS airborne,
	// and the AWACS feeding its picture ("E-3 Darkstar 1-1 + E-2 Wizard 1-1").
	GCIOffStation bool   `json:"gciOffStation,omitempty"`
	GCIRadars     string `json:"gciRadars,omitempty"`
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
	if err := initAuth(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fmt.Printf("vSFG-7 Launcher — root=%s listen=%s\n", rootDir, *flagListen)
	fmt.Printf("Open http://localhost%s/ in a browser.\n", *flagListen)
	fmt.Println(authSummary())

	discoverRoles()
	loadUpdateState()
	fleetRigs = parseFleet(*flagFleet)
	cachedVersion = computeVersion()
	go healthLoop()
	go appUpdateLoop()          // SkyEye-Setup installs: daily release check (appupdate.go)
	go restoreRolesAfterUpdate() // roles running before an update come back
	go alertLoop()
	startAirborneSources()

	mux := http.NewServeMux()
	// Read-only — any authenticated caller.
	mux.HandleFunc("/", serveUI)
	mux.HandleFunc("/fleet", serveFleetUI)
	mux.HandleFunc("/logs", serveLogsUI)
	mux.HandleFunc("/pilots", servePilotsUI)
	mux.HandleFunc("/settings", serveSettingsUI)
	mux.HandleFunc("/setup", serveSetupUI)
	mux.HandleFunc("/reports", serveReportsUI)
	mux.HandleFunc("/stats", serveStatsUI) // greenie board and flight stats over time (stats.go)
	mux.HandleFunc("/report-problem", serveBugReportPage) // a zip for #bug-reports (bugreport.go)
	mux.HandleFunc("/reports/aar", handleAARPage)           // one report as its printable page (aar.go)
	mux.HandleFunc("/reports/preview", handleReportPreview) // a prepared report's pages before Claude writes
	mux.HandleFunc("/logo.png", serveLogo)
	mux.HandleFunc("/branding.js", serveBrandingJS)
	mux.HandleFunc("/theme.js", serveThemeJS)   // colour modes (theme.go)
	mux.HandleFunc("/theme.css", serveThemeCSS) // Light and Night for every page
	mux.HandleFunc("/icon.png", serveIcon)
	mux.HandleFunc("/readback-mark.png", serveProductMark)
	mux.Handle("/help/", helpHandler()) // the Installation Guide and User Manual (help/)
	mux.Handle("/help", http.RedirectHandler("/help/", http.StatusFound))
	// GET = squadron name and logo (viewer); POST = change them (control, checked inside).
	mux.HandleFunc("/api/branding", handleBranding)
	mux.HandleFunc("/api/me", handleMe)
	mux.HandleFunc("/api/fleet", handleFleet)
	mux.HandleFunc("/api/version", handleVersion)
	mux.HandleFunc("/api/alerts", handleAlerts)
	mux.HandleFunc("/api/rig-log", handleRigLog)
	mux.HandleFunc("/api/roles", handleRoles)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/log", handleLog)
	mux.HandleFunc("/api/miz-weather", handleMizWeather)
	mux.HandleFunc("/api/airborne", handleAirborne)
	mux.HandleFunc("/api/log-stream", handleLogStream)
	mux.HandleFunc("/api/elevenlabs-usage", handleElevenUsage)
	mux.HandleFunc("/api/elevenlabs-fleet", handleElevenFleet)
	mux.HandleFunc("/api/comms-report", handleCommsReport)
	mux.HandleFunc("/api/comms-fleet", handleCommsFleet)
	mux.HandleFunc("/api/lso-log", handleLSOLog)
	mux.HandleFunc("/api/day-log", handleDayLog)
	mux.HandleFunc("/api/pilots-raw", handlePilotsRaw)
	mux.HandleFunc("/api/pilots", handlePilots)
	mux.HandleFunc("/api/reports", handleReports)
	// GET = one saved debrief (viewer); POST = delete it (control, checked inside).
	mux.HandleFunc("/api/reports/file", handleReportFile)
	mux.HandleFunc("/api/reports/pdf", handleAARPDF)
	mux.HandleFunc("/api/reports/png", handleAARPNG) // the first page as a picture, for Discord
	mux.HandleFunc("/api/stats", handleStats)
	mux.HandleFunc("/api/stats/csv", handleStatsCSV)
	mux.HandleFunc("/api/stats/pdf", handleStatsPDF)

	// State-changing — operators only, POST + same-origin (see controlAllowed).
	mux.HandleFunc("/api/start", requireControl(requireLicence(handleStart)))
	mux.HandleFunc("/api/stop", requireControl(handleStop))
	mux.HandleFunc("/api/start-region", requireControl(requireLicence(handleStartRegion)))
	mux.HandleFunc("/api/stop-region", requireControl(handleStopRegion))
	mux.HandleFunc("/api/restart", requireControl(requireLicence(handleRestart)))
	mux.HandleFunc("/api/rescan", requireControl(handleRescan))
	mux.HandleFunc("/api/reports/prepare", requireControl(handleReportPrepare))   // summarise, show what would be sent
	mux.HandleFunc("/api/reports/generate", requireControl(handleReportGenerate)) // send to Claude with the customer's key
	mux.HandleFunc("/api/stats/upload", requireControl(handleStatsUpload))        // keep a server dcs.log for the Stats tab
	mux.HandleFunc("/api/bug-report", requireControl(handleBugReport))            // the logs and settings, keys removed, as a zip
	// GET = the field sides (viewer); POST = set one (control, checked inside).
	mux.HandleFunc("/api/sides", handleSides)
	// GET = keys (masked), connection and frequencies (viewer); POST = change one (control, checked inside).
	mux.HandleFunc("/api/settings", handleSettings)
	mux.HandleFunc("/api/settings/voices", handleSettingsVoices)
	// GET = setup state (viewer); POST = theatre / test call / done (control, checked inside).
	mux.HandleFunc("/api/setup", handleSetup)
	// GET = trial / licence status (viewer); POST = enter a key (control, checked inside).
	mux.HandleFunc("/api/licence", handleLicence)
	// GET = installed-copy update state (viewer); POST = install the latest release (control, checked inside).
	mux.HandleFunc("/api/app-update", handleAppUpdate)
	// GET = progress (viewer); POST = run (control, checked inside).
	mux.HandleFunc("/api/update", handleUpdate)

	// Mixed — the proxy gates per-method itself: GET /status is a viewer read,
	// POST /runway and /weather are control actions.
	mux.HandleFunc("/tower/", handleTowerProxy)
	mux.HandleFunc("/rig/", handleRigProxy)

	// Retry the listen for a while: after a one-click update the new launcher
	// starts while the old one is still letting go of the port.
	var err error
	for i := 0; i < 20; i++ {
		if err = http.ListenAndServe(*flagListen, withAuth(mux)); err == nil || !strings.Contains(err.Error(), "bind") {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
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
	r := Role{Name: title, Bat: bat, Region: regionForBat(bat), Airfield: "OMDM", cmd: cmd} // atc.exe default
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
	case strings.Contains(cmd, "--command-only") && strings.Contains(cmd, "--coalition red"):
		r.LogFile = "atc-command-red.log"
	case strings.Contains(cmd, "--command-only"):
		r.LogFile = "atc-command.log"
	case strings.Contains(cmd, "--marshal-only"):
		r.LogFile = "atc-marshal.log"
	case strings.Contains(cmd, "--lso-only"):
		r.LogFile = "atc-lso.log"
	case strings.Contains(cmd, "--gci-only"):
		r.LogFile = "atc-gci.log"
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
	case strings.Contains(l, "iraq"):
		return "iraq"
	case strings.Contains(l, "syria"):
		return "syria"
	case strings.Contains(l, "kola"):
		return "kola"
	default:
		return "pg"
	}
}

// ── Process detection (Windows tasklist) ─────────────────────────────────────

// enumerateCmdWindows returns a map of lowercased window title → PID for
// every cmd.exe instance currently showing a window. One tasklist call per
// /api/roles request — much cheaper than one per role.
func enumerateCmdWindows() map[string]int {
	m := make(map[string]int)
	for _, w := range listCmdWindows() {
		m[w.title] = w.pid
	}
	return m
}

type cmdWindow struct {
	title string // lowercased
	pid   int
}

// listCmdWindows is every cmd.exe window, duplicates included: the title map
// above keeps one PID per title, so a role running twice was stopped once and
// the second copy survived every Stop and Update (Day 2026-10-23 06:39: each
// Caucasus tower running 2-3 times).
func listCmdWindows() []cmdWindow {
	out, err := exec.Command("tasklist", "/v", "/fi", "IMAGENAME eq cmd.exe", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil
	}
	var m []cmdWindow
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
		m = append(m, cmdWindow{title: strings.ToLower(title), pid: pid})
	}
	return m
}

// roleWindowPIDs is the PID of every window titled for the role, in the same
// exact or "<name> - <cmdline>" forms findWindowPID accepts.
func roleWindowPIDs(wins []cmdWindow, name string) []int {
	key := strings.ToLower(name)
	prefix := key + " - "
	var pids []int
	for _, w := range wins {
		if w.title == key || strings.HasPrefix(w.title, prefix) {
			pids = append(pids, w.pid)
		}
	}
	return pids
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

// probeAddr is where the health check and the air picture look for SRS or
// Tacview: the --srs-addr / --tacview-addr the rig's start_launcher.bat
// passes, else the address saved in Setup or Settings. Only a vSFG-7 rig (a
// source install) falls back to the flag's default, which is a vSFG-7 LAN
// address: an installed copy with nothing saved has none (operator
// 2026-10-23: the laptop showed the Night rig's live aircraft - "it should
// be blank").
func probeAddr(flagName, envName string) string {
	given := false
	flag.Visit(func(f *flag.Flag) { given = given || f.Name == flagName })
	if !given {
		if v := userEnv(envName); v != "" {
			return v
		}
		if !license.IsSource(rootDir) {
			return ""
		}
	}
	return flag.Lookup(flagName).Value.String()
}

func updateHealth() {
	h := Health{
		At:        time.Now(),
		SRS:       tcpProbe(probeAddr("srs-addr", "SKYEYE_SRS"), 2*time.Second),
		Tacview:   tcpProbe(probeAddr("tacview-addr", "SKYEYE_TACVIEW"), 2*time.Second),
		Tacview64: tacview64Probe(),
		OpenAI:    openaiProbe(3 * time.Second),
	}
	h.GCI, h.GCIPID, h.GCIName, h.GCIBoth = gciProbe()
	if h.GCI == "online" && h.GCIName != "skyeye.exe" {
		h.GCIOffStation, h.GCIRadars = gciStationStatus()
	}
	healthMu.Lock()
	cachedHealth = h
	healthMu.Unlock()
}

// gciProbe reports the rig's GCI: our atc.exe --gci-only role (Darkstar /
// Overlord, cmd/atc/gci.go, since 2026-10-21) or the old skyeye.exe. With
// neither running it is "offline" when the GCI role's theatre has other roles
// running (the rig is flying it, so GCI should be up) or SkyEye is installed,
// else "absent" - the dev box shows no chip.
func gciProbe() (status string, pid int, name string, both bool) {
	rolesMu.Lock()
	all := make([]Role, len(roles))
	copy(all, roles)
	rolesMu.Unlock()
	pids := detectRunning(all)
	gciRegions := map[string]bool{}
	ours := 0
	for i, r := range all {
		if !strings.Contains(r.cmd, "--gci-only") {
			continue
		}
		gciRegions[r.Region] = true
		if pids[i] != 0 && ours == 0 {
			ours, name = pids[i], r.Name
		}
	}
	sStatus, sPID := skyeyeProbe()
	switch {
	case ours != 0 && sStatus == "online":
		return "online", ours, name + " and skyeye.exe", true
	case ours != 0:
		return "online", ours, name, false
	case sStatus == "online":
		return "online", sPID, "skyeye.exe", false
	}
	for i, r := range all {
		if pids[i] != 0 && gciRegions[r.Region] && !strings.Contains(r.cmd, "--gci-only") {
			return "offline", 0, "", false
		}
	}
	return sStatus, 0, "", false
}

// gciStationStatus reads the newest logs\gci-status-*.json our GCI writes
// every 10 s (cmd/atc/gci_coverage.go writeGCIStatus). A file older than a
// minute (GCI stopped) is ignored.
func gciStationStatus() (offStation bool, radars string) {
	files, _ := filepath.Glob(filepath.Join(rootDir, "logs", "gci-status-*.json"))
	var newest time.Time
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var st struct {
			Coverage  bool      `json:"coverage"`
			OnStation bool      `json:"onStation"`
			Radars    string    `json:"radars"`
			At        time.Time `json:"at"`
		}
		if json.Unmarshal(data, &st) != nil || time.Since(st.At) > time.Minute || !st.At.After(newest) {
			continue
		}
		newest = st.At
		offStation, radars = st.Coverage && !st.OnStation, st.Radars
	}
	return offStation, radars
}

// skyeyeProbe reports whether the SkyEye GCI server is running on this rig:
// "online" with its PID, "offline" when it's installed but not running, or
// "absent" when there's no SkyEye here to run — not every rig flies GCI, so
// that case shouldn't read as a fault.
func skyeyeProbe() (string, int) {
	image := *flagSkyEyeImage
	if out, err := exec.Command("tasklist", "/fi", "imagename eq "+image, "/fo", "csv", "/nh").Output(); err == nil {
		// No match prints an INFO line, which never parses as a row for image.
		if rec, err := csv.NewReader(strings.NewReader(string(out))).Read(); err == nil &&
			len(rec) > 1 && strings.EqualFold(rec[0], image) {
			var pid int
			fmt.Sscanf(rec[1], "%d", &pid)
			return "online", pid
		}
	}
	if skyeyeInstalled(image) {
		return "offline", 0
	}
	return "absent", 0
}

// skyeyeInstalled looks for the SkyEye binary in --skyeye-dir, defaulting to a
// Skyeye folder beside the SkyeyeATC root (C:\Skyeye next to C:\SkyeyeATC).
func skyeyeInstalled(image string) bool {
	dir := *flagSkyEyeDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(rootDir), "Skyeye")
	}
	for _, sub := range []string{"", "bin", "dist", "build"} {
		if fi, err := os.Stat(filepath.Join(dir, sub, image)); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
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
	if addr == "" { // not set up yet
		return false
	}
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
	// AtcShort is the commit atc.exe (what the roles run) was built from;
	// filled per request, since an update can swap it under this launcher.
	AtcShort string `json:"atcShort,omitempty"`
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
	v := cachedVersion
	if rev, _, err := exeRevision(filepath.Join(rootDir, "atc.exe")); err == nil {
		v.AtcShort = shortRev(rev)
	}
	writeJSON(w, v)
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

// recoveredSignatures name a fault signature in a message that reports the
// fault handled: ExternalAudio crashing mid-clip is resent from where it got
// to (b26e6ad), and only a resend that fails is logged as an error - which
// still alerts. Seen 2026-10-15: every recovered ATIS crash raised a warning.
var recoveredSignatures = []string{
	"resending the rest", "nothing left to resend",
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
	// Window titles miss roles launched without one; any atc.exe counts.
	if running > 0 || len(atcPIDs()) > 0 {
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

	alerts = append(alerts, elevenUsageAlerts()...)

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
			recovered := false
			for _, sig := range recoveredSignatures {
				if strings.Contains(l.Message, sig) {
					recovered = true
				}
			}
			// Only warn lines can match a signature: every transmission logs
			// "TX via OpenAI TTS + ExternalAudio" (now "Tower TX") at info, and each one raised
			// an alert (Day, 2026-10-15 09:49, Raider 302's first calls).
			if !fault && !recovered && l.Level == "warn" {
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
	Name  string `json:"name"`  // short id: /rig/<name>/ paths, saved picker choice
	Label string `json:"label"` // what the pages show; defaults to Name
	Host  string `json:"host"`
	Port  int    `json:"port"`
}

// parseFleet parses "name@host:port,name@host:port,..." into a rig list.
// A bare "host" or "host:port" (no name) uses the host as the name; a missing
// port defaults to 7000 (the launcher port). "name=Display Label@host:port"
// gives the rig a friendlier label while the short name stays the id that
// URLs and saved choices use — a label like "Night Training ATC/ATIS" has
// spaces and a slash, so it can't be the id.
func parseFleet(s string) []Rig {
	var rigs []Rig
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, addr := part, part
		if at := strings.LastIndexByte(part, '@'); at >= 0 {
			name, addr = part[:at], part[at+1:]
		}
		label := ""
		if eq := strings.IndexByte(name, '='); eq >= 0 && name != part {
			name, label = strings.TrimSpace(name[:eq]), strings.TrimSpace(name[eq+1:])
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
		if label == "" {
			label = name
		}
		rigs = append(rigs, Rig{Name: name, Label: label, Host: host, Port: port})
	}
	return rigs
}

// RigStatus is a point-in-time health snapshot of one rig for the fleet view.
type RigStatus struct {
	Name       string       `json:"name"`
	Label      string       `json:"label"`
	Host       string       `json:"host"`
	Port       int          `json:"port"`
	Self       bool         `json:"self"`
	HostUp     bool         `json:"hostUp"`
	LauncherUp bool         `json:"launcherUp"`
	Health     *Health      `json:"health,omitempty"`
	Version    *VersionInfo `json:"version,omitempty"`
	RolesTotal int          `json:"rolesTotal"`
	Running    []Role       `json:"running,omitempty"`
	Alerts     []Alert      `json:"alerts,omitempty"`
	Mission    *RigMission  `json:"mission,omitempty"`
	Air        *RigAir      `json:"air,omitempty"`
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
	rs := RigStatus{Name: rig.Name, Label: rig.Label, Host: rig.Host, Port: rig.Port, Self: isSelf(rig), At: time.Now()}
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
		rs.Mission = pollRigMission(base, rs.Running)
	}
	rs.Air = rigAir(rig)
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

var (
	localAddrsOnce sync.Once
	localAddrs     map[string]bool
)

// localAddrSet is every name this box answers to, used to spot which --fleet
// entry is the launcher itself.
func localAddrSet() map[string]bool {
	localAddrsOnce.Do(func() {
		localAddrs = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ipn, ok := a.(*net.IPNet); ok {
					localAddrs[ipn.IP.String()] = true
				}
			}
		}
		if h, err := os.Hostname(); err == nil {
			localAddrs[strings.ToLower(h)] = true
		}
	})
	return localAddrs
}

func listenPort() int {
	_, p, err := net.SplitHostPort(*flagListen)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(p)
	return n
}

// isSelf reports whether a fleet entry points back at this launcher. The UI
// uses it to address the local rig as "" — no proxy hop.
func isSelf(rig Rig) bool {
	return rig.Port == listenPort() && localAddrSet()[strings.ToLower(rig.Host)]
}

// handleRigProxy forwards /rig/<name>/<path> to another rig's launcher, so one
// exposed launcher can drive the whole fleet. This is what makes the dashboard's
// rig picker work from outside, where only this box is reachable.
//
// Note the trust it implies: the remote rigs run auth-off on the LAN and see
// this proxy as a trusted local caller, so an operator here is an operator
// everywhere in --fleet. Keep --admins tight (REMOTE_ACCESS.md).
func handleRigProxy(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/rig/")
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		http.Error(w, "bad rig path — want /rig/<name>/<path>", http.StatusBadRequest)
		return
	}
	rig, ok := rigByName(rest[:slash])
	if !ok {
		http.Error(w, "unknown rig", http.StatusNotFound)
		return
	}
	if isSelf(rig) {
		// A hop back to ourselves would arrive from loopback carrying no Access
		// token and be refused by withAuth. The UI addresses this rig as "".
		http.Error(w, "that rig is this launcher — request the path directly", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if code, err := controlAllowed(r); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
	}

	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(rig.Host, strconv.Itoa(rig.Port))}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1 // stream SSE (/tower/<port>/ws/log) immediately
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	r.URL.Path = "/" + rest[slash+1:]
	// Never hand this session's Access credentials to another host.
	r.Header.Del(accessJWTHeader)
	r.Header.Del("Cookie")
	proxy.ServeHTTP(w, r)
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
	refreshKeyEnv() // a key set with setx after this launcher started still reaches the role
	cmd := exec.Command("cmd", "/c", "start", "", filepath.Join(rootDir, r.Bat))
	cmd.Dir = rootDir
	return cmd.Start()
}

func stopRole(name string) error {
	if pids := roleWindowPIDs(listCmdWindows(), name); len(pids) > 0 {
		var firstErr error
		for _, pid := range pids { // every copy, not just the first
			if err := killTree(pid); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
	// No window to close — the role may still be running without one.
	if r := findRole(name); r != nil && r.cmd != "" {
		for _, p := range listATCProcs() {
			if roleMatchesProc(r.cmd, p) {
				return killTree(p.PID)
			}
		}
	}
	return fmt.Errorf("not running: %s", name)
}

// regionBats maps a theatre to its single-shot launcher bat (ATIS + towers +
// command, and Marshal/Deckboss for PG). These deliberately do NOT call
// start_launcher.bat — the launcher is already running when the dashboard
// triggers a region start. Keep keys in sync with regionForBat + ui.html.
var regionBats = map[string]string{
	"pg":       "start_region_pg.bat",
	"caucasus": "start_region_caucasus.bat",
	"germany":  "start_region_germany.bat",
	"iraq":     "start_region_iraq.bat",
	"syria":    "start_region_syria.bat",
	"kola":     "start_region_kola.bat",
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
	refreshKeyEnv() // a key set with setx after this launcher started still reaches the roles
	cmd := exec.Command("cmd", "/c", "start", "", full)
	cmd.Dir = rootDir
	return cmd.Start()
}

// stopRegion kills every running role window belonging to the region. Returns
// the number of roles killed so the dashboard can report "nothing was running".
func stopRegion(region string) (int, error) {
	rolesMu.Lock()
	rs := append([]Role(nil), roles...)
	rolesMu.Unlock()

	// Detect across every role, not just this region's, so a process is only
	// ever claimed by the role it actually belongs to.
	pids := detectRunning(rs)
	killed := 0
	var firstErr error
	for i := range rs {
		if rs[i].Region != region || pids[i] == 0 {
			continue
		}
		if err := killTree(pids[i]); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		killed++
	}
	// Then any second copy of a region role: detectRunning names one PID per
	// role, and the copies it didn't name were what came back after an Update.
	wins := listCmdWindows()
	for i := range rs {
		if rs[i].Region != region {
			continue
		}
		for _, pid := range roleWindowPIDs(wins, rs[i].Name) {
			if pid != pids[i] {
				_ = killTree(pid)
			}
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
// abused as an open proxy to arbitrary localhost services. Covers every
// theatre: PG towers 6001-6003 + Marshal 6004 + Deckboss 6005, Caucasus
// towers 6011-6015, Cold War Germany towers 6021-6028, Iraq towers
// 6031-6039, Syria towers 6041-6050 (6049/6050 = Bassel Al-Assad / Beirut), and
// Kola towers 6051-6054.
var proxyPorts = map[int]bool{
	6001: true, 6002: true, 6003: true, 6004: true, 6005: true,
	6011: true, 6012: true, 6013: true, 6014: true, 6015: true,
	6021: true, 6022: true, 6023: true, 6024: true,
	6025: true, 6026: true, 6027: true, 6028: true,
	6031: true, 6032: true, 6033: true, 6034: true, 6035: true,
	6036: true, 6037: true, 6038: true, 6039: true,
	6041: true, 6042: true, 6043: true, 6044: true,
	6045: true, 6046: true, 6047: true, 6048: true,
	6049: true, 6050: true,
	6051: true, 6052: true, 6053: true, 6054: true,
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
	// The towers have no auth of their own, so the proxy is where their control
	// surface gets gated. Reads (/status, /health, /ws/log) are open to any
	// authenticated viewer; anything that writes — POST /runway, /weather —
	// goes through the same operator check as the launcher's own actions.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if code, err := controlAllowed(r); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
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

	pids := detectRunning(out)
	for i := range out {
		if pids[i] != 0 {
			out[i].Status = "running"
			out[i].PID = pids[i]
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

// runningRoles names the roles running now, of those listed.
func runningRoles(names []string) []string {
	rolesMu.Lock()
	rs := append([]Role(nil), roles...)
	rolesMu.Unlock()
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []string
	for i, pid := range detectRunning(rs) {
		if pid != 0 && want[rs[i].Name] {
			out = append(out, rs[i].Name)
		}
	}
	return out
}

// regionStartRoles are the roles a region's bat starts: the region's roles
// whose own bat it calls (the on-demand RED card is in the region but not in
// its bat).
func regionStartRoles(region string) []string {
	body, err := os.ReadFile(filepath.Join(rootDir, regionBats[region]))
	if err != nil {
		return nil
	}
	text := strings.ToLower(string(body))
	rolesMu.Lock()
	defer rolesMu.Unlock()
	var out []string
	for _, r := range roles {
		if r.Region == region && r.Bat != "" && strings.Contains(text, strings.ToLower(r.Bat)) {
			out = append(out, r.Name)
		}
	}
	return out
}

// Starting a role that is already running is refused with a message the
// dashboard shows (operator 2026-10-23: Day had every tower running 2-3
// times). atc.exe refuses a second copy anyway (ec006bd); this says so up
// front instead of leaving a refused copy in the log.
func handleStart(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if len(runningRoles([]string{name})) > 0 {
		http.Error(w, name+" is already running", http.StatusConflict)
		return
	}
	if err := startRole(name); err != nil {
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
	region := r.URL.Query().Get("region")
	want := regionStartRoles(region)
	up := runningRoles(want)
	if len(want) > 0 && len(up) == len(want) {
		http.Error(w, fmt.Sprintf("all %d roles are already running", len(want)), http.StatusConflict)
		return
	}
	if err := startRegion(region); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// The bat starts every role; the ones already up refuse their second copy.
	writeJSON(w, map[string]any{"status": "started", "alreadyRunning": up})
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
	// Priority mirrors atc.exe's boot seed (cmd/atc/main.go): an explicit
	// --miz-path (fed SKYEYE_MIZ by start_launcher.bat) wins, so the widget
	// reflects the mission the roles actually loaded rather than whatever .miz
	// happens to be newest on disk. Ahead of both: the mission the DCS server
	// actually has loaded, as written by the server hook -- the roles follow it
	// live (cmd/atc/mizwatch.go), so the widget must too.
	watch := *flagMizWatch
	if watch == "" {
		watch = filepath.Join(rootDir, "current_mission.txt")
	}
	if path := currentMissionFromHook(watch); path != "" {
		if wx, err := miz.ReadMizWeather(path); err == nil {
			writeMizWeather(w, path, wx)
			return
		}
	}
	if path := *flagMizPath; path != "" {
		wx, err := miz.ReadMizWeather(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeMizWeather(w, path, wx)
		return
	}

	dir, tried := resolveMizDir()
	if dir == "" {
		http.Error(w, "no DCS Missions dir found; pass --miz-dir or set SKYEYE_MIZ. Tried: "+
			strings.Join(tried, " ; "), http.StatusNotFound)
		return
	}
	path, wx, err := newestParsableMiz(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeMizWeather(w, path, wx)
}

// currentMissionFromHook returns the .miz named in the hook's pointer file, or ""
// when there is none (hook not installed, empty, or the mission is not on disk).
func currentMissionFromHook(pointerFile string) string {
	if pointerFile == "" {
		return ""
	}
	b, err := os.ReadFile(pointerFile)
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(strings.TrimPrefix(string(b), "\uFEFF"))
	if p == "" || !strings.EqualFold(filepath.Ext(p), ".miz") {
		return ""
	}
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

func writeMizWeather(w http.ResponseWriter, path string, wx miz.Weather) {
	writeJSON(w, map[string]interface{}{
		"mizName": filepath.Base(path),
		"windDir": wx.WindDirTrue,
		"windKts": wx.WindKts,
		"ceilFt":  wx.CeilFt,
		"visNm":   wx.VisNm,
		"altInHg": wx.AltInHg,
		"tempC":   wx.TempC,
		"theatre": wx.Theatre,
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
