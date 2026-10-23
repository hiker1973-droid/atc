package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/license"
)

// Updates for SkyEye-Setup installs (operator 2026-10-23: "how do we have them
// update versions?", then "build the update check with GitHub releases").
// Each release is a GitHub release in a public repo holding only installers:
// tag v<version>, asset SkyEye-Setup.exe, notes in the body. An installed
// launcher asks GitHub once a day; when there is a newer one the dashboard and
// Settings offer it, and Update downloads it, checks its SHA-256 against
// GitHub's digest, and runs it with --update, which swaps the programs and
// starts the launcher again. The roles that were running come back too.
//
// A git checkout (the vSFG-7 rigs) never checks: it updates by git pull.

// productVersion is stamped by build_installer.bat (product_version.txt);
// empty on a source build.
var productVersion = ""

var flagReleases = flag.String("releases", "hiker1973-droid/readback-atc-releases",
	"Public GitHub repo whose releases carry Readback-Setup.exe, for installed copies' update check")

const setupAsset = "Readback-Setup.exe"

type appRelease struct {
	Version   string    `json:"version"`
	Notes     string    `json:"notes"`
	Page      string    `json:"page"`
	URL       string    `json:"-"`
	Size      int64     `json:"size"`
	Digest    string    `json:"-"` // sha256:<hex>, when GitHub gives one
	Published time.Time `json:"published"`
}

type appUpdateState struct {
	Current    string      `json:"current"`
	Source     bool        `json:"source"` // git checkout: updates by git pull
	Latest     *appRelease `json:"latest,omitempty"`
	Available  bool        `json:"available"`
	CheckedAt  time.Time   `json:"checkedAt,omitempty"`
	Error      string      `json:"error,omitempty"`
	Installing bool        `json:"installing"`
	Step       string      `json:"step,omitempty"`
}

var (
	appUpdMu sync.Mutex
	appUpd   appUpdateState
)

func updatesEnabled() bool { return productVersion != "" && !license.IsSource(rootDir) }

// semver reads "v1.2.3" or "1.2.3-beta" into three numbers.
func semver(s string) ([3]int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "-")
	parts := strings.Split(s, ".")
	var v [3]int
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// newerVersion reports whether a is a later version than b.
func newerVersion(a, b string) bool {
	va, ok1 := semver(a)
	vb, ok2 := semver(b)
	if !ok1 || !ok2 {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

var githubClient = &http.Client{Timeout: 20 * time.Second}

// githubAPI is the API root; tests point it elsewhere.
var githubAPI = "https://api.github.com"

// fetchLatestRelease reads the newest release; nil with no error when none
// has been published yet.
func fetchLatestRelease() (*appRelease, error) {
	req, _ := http.NewRequest(http.MethodGet, githubAPI+"/repos/"+*flagReleases+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Readback-ATC-launcher")
	resp, err := githubClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach GitHub")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	var r struct {
		Tag       string    `json:"tag_name"`
		Body      string    `json:"body"`
		Page      string    `json:"html_url"`
		Published time.Time `json:"published_at"`
		Assets    []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return nil, err
	}
	rel := &appRelease{Version: strings.TrimPrefix(r.Tag, "v"), Notes: r.Body, Page: r.Page, Published: r.Published}
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, setupAsset) {
			rel.URL, rel.Size, rel.Digest = a.URL, a.Size, a.Digest
		}
	}
	if rel.URL == "" {
		return nil, fmt.Errorf("release %s has no %s", r.Tag, setupAsset)
	}
	return rel, nil
}

// checkAppUpdate asks GitHub and records the answer.
func checkAppUpdate() appUpdateState {
	rel, err := fetchLatestRelease()
	appUpdMu.Lock()
	defer appUpdMu.Unlock()
	appUpd.Current, appUpd.Source, appUpd.CheckedAt = productVersion, !updatesEnabled(), time.Now()
	appUpd.Error = ""
	if err != nil {
		appUpd.Error = err.Error()
		return appUpd
	}
	appUpd.Latest = rel
	appUpd.Available = rel != nil && newerVersion(rel.Version, productVersion)
	return appUpd
}

// appUpdateLoop checks a minute after start, then daily.
func appUpdateLoop() {
	if !updatesEnabled() {
		return
	}
	time.Sleep(time.Minute)
	for {
		checkAppUpdate()
		time.Sleep(24 * time.Hour)
	}
}

func appUpdateSnapshot() appUpdateState {
	appUpdMu.Lock()
	defer appUpdMu.Unlock()
	s := appUpd
	s.Current, s.Source = productVersion, !updatesEnabled()
	return s
}

// downloadRelease fetches the installer into updates\ and checks it.
func downloadRelease(rel *appRelease) (string, error) {
	dir := filepath.Join(rootDir, "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "Readback-Setup-"+rel.Version+".exe")
	req, _ := http.NewRequest(http.MethodGet, rel.URL, nil)
	req.Header.Set("User-Agent", "Readback-ATC-launcher")
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(path + ".part")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 500<<20))
	f.Close()
	if err != nil {
		os.Remove(path + ".part")
		return "", fmt.Errorf("download failed: %v", err)
	}
	if rel.Size > 0 && n != rel.Size {
		os.Remove(path + ".part")
		return "", fmt.Errorf("download incomplete (%d of %d bytes)", n, rel.Size)
	}
	if want, ok := strings.CutPrefix(rel.Digest, "sha256:"); ok && !strings.EqualFold(want, hex.EncodeToString(h.Sum(nil))) {
		os.Remove(path + ".part")
		return "", fmt.Errorf("the download does not match GitHub's checksum - not installed")
	}
	if err := os.Rename(path+".part", path); err != nil {
		return "", err
	}
	return path, nil
}

// restartFile lists the roles to start again once the update is in.
func restartFile() string { return filepath.Join(rootDir, "restart_after_update.json") }

// installAppUpdate downloads the latest release and hands over to it.
func installAppUpdate() {
	set := func(step string) { appUpdMu.Lock(); appUpd.Step = step; appUpdMu.Unlock() }
	fail := func(err error) {
		appUpdMu.Lock()
		appUpd.Installing, appUpd.Step, appUpd.Error = false, "", err.Error()
		appUpdMu.Unlock()
	}
	rel := appUpdateSnapshot().Latest
	if rel == nil {
		fail(fmt.Errorf("no release to install - check again"))
		return
	}
	set("downloading " + rel.Version)
	path, err := downloadRelease(rel)
	if err != nil {
		fail(err)
		return
	}
	// The roles running now come back after the update (restoreRolesAfterUpdate).
	rolesMu.Lock()
	var names []string
	for _, r := range roles {
		names = append(names, r.Name)
	}
	rolesMu.Unlock()
	if running := runningRoles(names); len(running) > 0 {
		b, _ := json.Marshal(running)
		os.WriteFile(restartFile(), b, 0o644)
	}
	set("installing " + rel.Version + " - the launcher restarts")
	cmd := exec.Command("cmd", "/c", "start", "Readback ATC update", "/min", path, "--dir", rootDir, "--update")
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		os.Remove(restartFile())
		fail(fmt.Errorf("could not start the installer: %v", err))
	}
}

// restoreRolesAfterUpdate starts the roles that were running when an update
// was installed (a fresh file only: an old one is a failed update's).
func restoreRolesAfterUpdate() {
	st, err := os.Stat(restartFile())
	if err != nil {
		return
	}
	b, _ := os.ReadFile(restartFile())
	os.Remove(restartFile())
	if time.Since(st.ModTime()) > 15*time.Minute {
		return
	}
	var names []string
	if json.Unmarshal(b, &names) != nil {
		return
	}
	started := map[string]bool{}
	for _, n := range names {
		if r := findRole(n); r != nil && !started[r.Bat] {
			started[r.Bat] = true
			_ = startRole(n)
			time.Sleep(time.Second)
		}
	}
}

// handleAppUpdate: GET the update state (?check=1 asks GitHub now); POST
// (control) installs the latest release.
func handleAppUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		if r.URL.Query().Get("check") == "1" && updatesEnabled() {
			s := appUpdateSnapshot()
			if time.Since(s.CheckedAt) > 30*time.Second { // one GitHub call per half minute at most
				checkAppUpdate()
			}
		}
		writeJSON(w, appUpdateSnapshot())
		return
	}
	if code, err := controlAllowed(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	if !updatesEnabled() {
		http.Error(w, "this copy updates with the dashboard's Update button (git)", http.StatusConflict)
		return
	}
	appUpdMu.Lock()
	if appUpd.Installing {
		appUpdMu.Unlock()
		http.Error(w, "an update is already being installed", http.StatusConflict)
		return
	}
	if !appUpd.Available {
		appUpdMu.Unlock()
		http.Error(w, "no newer version to install", http.StatusConflict)
		return
	}
	appUpd.Installing, appUpd.Error = true, ""
	appUpdMu.Unlock()
	go installAppUpdate()
	writeJSON(w, map[string]any{"ok": true})
}
