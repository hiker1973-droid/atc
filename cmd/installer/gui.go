package main

// The setup window (operator 2026-10-23: "make a pretty installer screen
// instead of prompt"). Readback-Setup.exe serves ui/setup.html on a private
// 127.0.0.1 port and shows it in a Microsoft Edge app window (no address bar
// or tabs; Edge is on every Windows 10 and 11 PC), or the default browser
// without Edge. The page drives the steps through the small API below; every
// call carries a random token, so no other page can. Closing the window ends
// setup (after a running install finishes).

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed ui
var uiFS embed.FS

type gui struct {
	mu        sync.Mutex
	mode      string // install | uninstall
	phase     string // welcome | working | done | removed | error
	err       string
	busy      string // a prerequisite being installed ("ffmpeg")
	busyErr   string
	lastPoll  time.Time
	quit      chan struct{}
	quitOnce  sync.Once
	token     string
	installed string // the version already in the folder, if any
	dir       string // the install folder, which the person can change
	dirErr    string // why the folder chosen will not do
}

func (g *gui) set(f func()) { g.mu.Lock(); f(); g.mu.Unlock() }

func (g *gui) stop() { g.quitOnce.Do(func() { close(g.quit) }) }

func (g *gui) working() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.phase == "working" || g.busy != ""
}

func runGUI(uninstallMode bool) error {
	g := &gui{mode: "install", phase: "welcome", quit: make(chan struct{}), installed: installedVersion(*flagDir), dir: *flagDir}
	if uninstallMode {
		g.mode = "uninstall"
	}
	b := make([]byte, 16)
	rand.Read(b)
	g.token = hex.EncodeToString(b)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: g.routes()}
	go srv.Serve(ln)
	defer srv.Close()
	url := fmt.Sprintf("http://127.0.0.1:%d/?t=%s", ln.Addr().(*net.TCPAddr).Port, g.token)

	say("Setup window: %s", url)
	var win *exec.Cmd
	var profile string
	if !*flagNoWindow {
		win, profile = openWindow(url)
	}
	if win == nil && !*flagNoWindow {
		openURL(url)
	}
	if win != nil {
		go win.Wait()
	}
	// The window is gone when the page stops asking for the state (it asks
	// every 0.7 s). Not when the Edge process we started exits: on its first
	// run with a new profile Edge can restart itself, handing the window to
	// a new process - setup used to stop there and leave the window on a
	// dead 127.0.0.1 page (operator 2026-10-23: "first time you start exe
	// it comes up with 127.xxxx error").
	opened := time.Now()
	watchdog := time.NewTicker(time.Second)
	defer watchdog.Stop()
wait:
	for {
		select {
		case <-g.quit:
			break wait
		case <-watchdog.C:
			g.mu.Lock()
			last := g.lastPoll
			g.mu.Unlock()
			switch {
			case !last.IsZero() && time.Since(last) > 6*time.Second:
				break wait // closed
			case last.IsZero() && time.Since(opened) > 3*time.Minute:
				break wait // never opened
			}
		}
	}
	for g.working() { // closed mid-install: let it finish
		time.Sleep(300 * time.Millisecond)
	}
	if profile != "" {
		closeWindow(profile)
	}
	if win != nil && win.ProcessState == nil {
		win.Process.Kill()
	}
	if profile != "" {
		for i := 0; i < 10 && os.RemoveAll(profile) != nil; i++ {
			time.Sleep(300 * time.Millisecond)
		}
	}
	return nil
}

// closeWindow ends every Edge process using setup's own profile (the window
// may belong to a process Edge started itself, not the one we did).
func closeWindow(profile string) {
	script := fmt.Sprintf(`Get-CimInstance Win32_Process -Filter "Name='msedge.exe'" | Where-Object { $_.CommandLine -like '*%s*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`,
		strings.ReplaceAll(filepath.Base(profile), "'", "''"))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	hideWindow(cmd)
	cmd.Run()
	time.Sleep(500 * time.Millisecond)
}

// openWindow starts Edge in app mode with a throwaway profile (its own
// window, never the person's browser), centred on the screen.
func openWindow(url string) (*exec.Cmd, string) {
	edge := ""
	for _, p := range []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(os.Getenv("ProgramFiles"), `Microsoft\Edge\Application\msedge.exe`),
	} {
		if exists(p) {
			edge = p
			break
		}
	}
	if edge == "" {
		return nil, ""
	}
	profile, err := os.MkdirTemp("", "readback-setup-")
	if err != nil {
		return nil, ""
	}
	const w, h = 780, 660
	args := []string{"--app=" + url, fmt.Sprintf("--window-size=%d,%d", w, h),
		"--user-data-dir=" + profile, "--no-first-run", "--no-default-browser-check", "--disable-sync"}
	if sw, sh := screenSize(); sw > w && sh > h {
		args = append(args, fmt.Sprintf("--window-position=%d,%d", (sw-w)/2, (sh-h)/2))
	}
	cmd := exec.Command(edge, args...)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profile)
		return nil, ""
	}
	return cmd, profile
}

func (g *gui) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.URL.Query().Get("t") != g.token {
			http.NotFound(w, r)
			return
		}
		page, _ := uiFS.ReadFile("ui/setup.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(page)
	})
	img := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			data, _ := uiFS.ReadFile("ui/" + name)
			w.Header().Set("Content-Type", "image/png")
			w.Write(data)
		}
	}
	mux.HandleFunc("/banner.png", img("banner.png"))
	mux.HandleFunc("/icon.png", img("icon.png"))
	mux.HandleFunc("/api/", g.api)
	return mux
}

func (g *gui) api(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Token") != g.token {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Startup bool   `json:"startup"`
		Launch  bool   `json:"launch"`
		Key     string `json:"key"`
		Dir     string `json:"dir"`
	}
	if r.Method == http.MethodPost {
		json.NewDecoder(r.Body).Decode(&body)
	}
	g.mu.Lock()
	dir := g.dir
	g.mu.Unlock()
	setDir := func(d string) {
		g.set(func() { g.dir, g.installed, g.dirErr = d, installedVersion(d), "" })
		dir = d
	}
	switch r.URL.Path {
	case "/api/state":
		g.set(func() { g.lastPoll = time.Now() })
	case "/api/browse": // Windows' folder picker; a parent folder gets "Readback ATC" added
		if g.working() || g.mode != "install" {
			break
		}
		start := dir
		for start != "" && !exists(start) {
			start = filepath.Dir(start)
		}
		if picked := pickFolder(start); picked != "" {
			setDir(chosenDir(picked))
		}
	case "/api/dir": // typed in the folder box
		if !g.working() && g.mode == "install" && strings.TrimSpace(body.Dir) != "" {
			setDir(filepath.Clean(strings.TrimSpace(body.Dir)))
		}
	case "/api/install":
		if g.working() {
			break
		}
		if d := strings.TrimSpace(body.Dir); d != "" {
			setDir(filepath.Clean(d))
		}
		if err := checkDir(dir); err != nil {
			g.set(func() { g.dirErr = err.Error() })
			break
		}
		g.set(func() { g.phase, g.err = "working", "" })
		go func() {
			err := install(dir, installOptions{Shortcuts: !*flagNoLinks, Startup: body.Startup})
			g.set(func() {
				if err != nil {
					g.phase, g.err = "error", err.Error()
				} else {
					g.phase = "done"
				}
			})
		}()
	case "/api/uninstall":
		if g.working() {
			break
		}
		g.set(func() { g.phase = "working" })
		go func() {
			err := uninstall(dir)
			g.set(func() {
				if err != nil {
					g.phase, g.err = "error", err.Error()
				} else {
					g.phase = "removed"
				}
			})
		}()
	case "/api/ffmpeg":
		if g.working() {
			break
		}
		g.set(func() { g.busy, g.busyErr = "ffmpeg", "" })
		go func() {
			err := installFFmpeg(dir)
			g.set(func() {
				g.busy = ""
				if err != nil {
					g.busyErr = err.Error()
				}
			})
		}()
	case "/api/open":
		if u, ok := prereqURL[body.Key]; ok {
			openURL(u)
		} else if body.Key == "folder" {
			exec.Command("explorer", dir).Start()
		}
	case "/api/finish":
		if body.Launch && !*flagNoStart {
			if err := startLauncher(dir); err == nil {
				openURL(setupURL())
			}
		}
		g.stop()
	case "/api/quit":
		g.stop()
	default:
		http.NotFound(w, r)
		return
	}
	g.mu.Lock()
	state := map[string]any{
		"mode": g.mode, "phase": g.phase, "err": g.err, "busy": g.busy, "busyErr": g.busyErr,
		"version": productVersion, "dir": g.dir, "dirErr": g.dirErr, "installed": g.installed,
		"startup": exists(startupLink()), "log": logLines(),
	}
	g.mu.Unlock()
	if state["phase"] == "done" {
		state["prereqs"] = prerequisites(dir)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(state)
}
