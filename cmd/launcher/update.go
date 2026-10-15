package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// One-click rig update (operator 2026-10-15, "do 1, 2 and 4" - 4 being the
// update button). Every change used to need pull, build.bat and restarts by
// hand on each VM, in an order that is easy to get wrong: on 2026-10-15 the
// rigs kept a revoked ElevenLabs key because the roles were restarted by a
// launcher started before the new key was set, and the launchers ran a build
// behind the roles. POST /api/update does it in order, stopping at the first
// failure:
//
//  1. git pull --ff-only (never a merge)
//  2. build atc.exe and launcher.exe to *.new.exe - a failed build stops here
//     with nothing that is running touched
//  3. re-read the API keys from the registry (refreshKeyEnv), so the roles
//     restarted next get the current key, not the launcher's
//  4. stop each theatre that is running, swap atc.exe (the old one kept as
//     atc.prev.exe for rollback), start the theatres again
//  5. swap launcher.exe and restart the launcher last
//
// GET /api/update is the progress, for the fleet page.

type updateState struct {
	Running  bool      `json:"running"`
	Step     string    `json:"step"`
	Log      []string  `json:"log"`
	From     string    `json:"from,omitempty"`
	To       string    `json:"to,omitempty"`
	Regions  []string  `json:"regions,omitempty"`
	Started  time.Time `json:"started,omitempty"`
	Finished time.Time `json:"finished,omitempty"`
	Err      string    `json:"err,omitempty"`
	Done     bool      `json:"done"`
}

var (
	updMu sync.Mutex
	upd   updateState
)

func updLog(format string, a ...any) {
	line := time.Now().Format("15:04:05 ") + fmt.Sprintf(format, a...)
	updMu.Lock()
	upd.Log = append(upd.Log, line)
	upd.Step = fmt.Sprintf(format, a...)
	updMu.Unlock()
	fmt.Println("[update] " + line)
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		updMu.Lock()
		s := upd
		s.Log = append([]string(nil), upd.Log...)
		updMu.Unlock()
		writeJSON(w, s)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
		return
	}
	if code, err := controlAllowed(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	updMu.Lock()
	if upd.Running {
		updMu.Unlock()
		http.Error(w, "an update is already running", http.StatusConflict)
		return
	}
	upd = updateState{Running: true, Started: time.Now(), Step: "starting"}
	updMu.Unlock()
	force := r.URL.Query().Get("force") == "1"
	go runUpdate(force)
	writeJSON(w, map[string]string{"status": "started"})
}

func runUpdate(force bool) {
	err := doUpdate(force)
	if err != nil {
		updLog("FAILED: %v", err)
	}
	updMu.Lock()
	upd.Running, upd.Done, upd.Finished = false, true, time.Now()
	if err != nil {
		upd.Err = err.Error()
	}
	updMu.Unlock()
	saveUpdateState()
}

// The launcher restarts itself at the end of an update, so the outcome is
// written to update_last.json and read back by the new launcher.
func updateStatePath() string { return filepath.Join(rootDir, "update_last.json") }

func saveUpdateState() {
	updMu.Lock()
	b, _ := json.MarshalIndent(upd, "", "  ")
	updMu.Unlock()
	_ = os.WriteFile(updateStatePath(), b, 0o644)
}

// loadUpdateState shows the last update's outcome after a restart.
func loadUpdateState() {
	b, err := os.ReadFile(updateStatePath())
	if err != nil {
		return
	}
	var s updateState
	if json.Unmarshal(b, &s) == nil {
		s.Running = false
		updMu.Lock()
		upd = s
		updMu.Unlock()
	}
}

func gitOut(args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", rootDir}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func doUpdate(force bool) error {
	from, _ := gitOut("rev-parse", "--short", "HEAD")
	updMu.Lock()
	upd.From = from
	updMu.Unlock()

	updLog("git pull --ff-only (at %s)", from)
	if out, err := gitOut("pull", "--ff-only"); err != nil {
		return fmt.Errorf("git pull: %v: %s", err, out)
	}
	to, _ := gitOut("rev-parse", "--short", "HEAD")
	updMu.Lock()
	upd.To = to
	updMu.Unlock()
	if to == from && cachedVersion.Short == to && !force {
		updLog("already up to date at %s - nothing restarted", to)
		return nil
	}
	updLog("now at %s - building", to)

	for _, b := range []struct{ out, pkg string }{{"atc.new.exe", "./cmd/atc"}, {"launcher.new.exe", "./cmd/launcher"}} {
		cmd := exec.Command("go", "build", "-o", b.out, b.pkg)
		cmd.Dir = rootDir
		if out, err := cmd.CombinedOutput(); err != nil {
			os.Remove(filepath.Join(rootDir, b.out))
			return fmt.Errorf("build %s failed, nothing restarted: %v: %s", b.pkg, err, strings.TrimSpace(string(out)))
		}
		updLog("built %s", b.out)
	}

	n := refreshKeyEnv()
	updLog("re-read %d key/setting variable(s) from the registry for the roles", n)

	regions := runningRegions()
	updMu.Lock()
	upd.Regions = regions
	updMu.Unlock()
	for _, reg := range regions {
		k, err := stopRegion(reg)
		updLog("stopped %s (%d role(s))%s", reg, k, errSuffix(err))
	}
	if len(regions) > 0 {
		time.Sleep(3 * time.Second) // let the killed atc.exe images unlock
	}
	if err := swapExe("atc.exe", "atc.new.exe", "atc.prev.exe"); err != nil {
		// Put the theatres back on the old build rather than leave them down.
		for _, reg := range regions {
			_ = startRegion(reg)
		}
		return fmt.Errorf("swap atc.exe (theatres restarted on the old build): %v", err)
	}
	updLog("atc.exe updated (previous kept as atc.prev.exe)")
	for _, reg := range regions {
		if err := startRegion(reg); err != nil {
			return fmt.Errorf("start %s: %v", reg, err)
		}
		updLog("started %s", reg)
	}
	if len(regions) == 0 {
		updLog("no theatre was running - none started")
	}

	if err := swapExe("launcher.exe", "launcher.new.exe", "launcher.prev.exe"); err != nil {
		return fmt.Errorf("swap launcher.exe (roles are updated, launcher is not): %v", err)
	}
	updLog("launcher.exe updated - restarting the launcher")
	updMu.Lock()
	upd.Running, upd.Done, upd.Finished = false, true, time.Now()
	updMu.Unlock()
	saveUpdateState()
	return restartLauncher()
}

func errSuffix(err error) string {
	if err != nil {
		return " - " + err.Error()
	}
	return ""
}

// runningRegions is every theatre with a role running, in a stable order.
func runningRegions() []string {
	rolesMu.Lock()
	rs := append([]Role(nil), roles...)
	rolesMu.Unlock()
	pids := detectRunning(rs)
	set := map[string]bool{}
	for i, r := range rs {
		if pids[i] != 0 && r.Region != "" {
			if _, ok := regionBats[r.Region]; ok {
				set[r.Region] = true
			}
		}
	}
	var out []string
	for r := range set {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// swapExe moves cur to prev (a running image can be renamed on Windows) and
// next to cur. On failure cur is put back.
func swapExe(cur, next, prev string) error {
	c, n, p := filepath.Join(rootDir, cur), filepath.Join(rootDir, next), filepath.Join(rootDir, prev)
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		// The old rollback copy is still running (or locked): keep it, use a
		// dated name for this one.
		p = filepath.Join(rootDir, strings.TrimSuffix(prev, ".exe")+time.Now().Format("-20060102-150405")+".exe")
	}
	if err := os.Rename(c, p); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(n, c); err != nil {
		_ = os.Rename(p, c)
		return err
	}
	return nil
}

// restartLauncher starts the new launcher.exe with this one's arguments in its
// own window, then exits so it can take the port (main retries the listen).
func restartLauncher() error {
	args := append([]string{"/c", "start", "vSFG-7 Launcher", filepath.Join(rootDir, "launcher.exe")}, os.Args[1:]...)
	cmd := exec.Command("cmd", args...)
	cmd.Dir = rootDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start new launcher: %v", err)
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// keyEnvPrefixes are the variables re-read from the registry before the
// launcher starts roles: the API keys and SkyEye settings an operator changes
// with setx. A process only sees setx values set before it started, and the
// roles inherit the launcher's environment - the 2026-10-15 revoked-key 401s.
var keyEnvPrefixes = []string{"ELEVENLABS_", "OPENAI_", "SKYEYE_"}

// refreshKeyEnv copies those variables from the registry (machine, then user,
// which wins - the same precedence Windows uses) into this process. Returns
// how many were set.
func refreshKeyEnv() int {
	vals := map[string]string{}
	read := func(root registry.Key, path string) {
		k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
		if err != nil {
			return
		}
		defer k.Close()
		names, _ := k.ReadValueNames(0)
		for _, name := range names {
			up := strings.ToUpper(name)
			for _, p := range keyEnvPrefixes {
				if strings.HasPrefix(up, p) {
					if v, _, err := k.GetStringValue(name); err == nil && v != "" {
						vals[name] = v
					}
				}
			}
		}
	}
	read(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`)
	read(registry.CURRENT_USER, `Environment`)
	for k, v := range vals {
		os.Setenv(k, v)
	}
	return len(vals)
}
