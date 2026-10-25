package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// Report a problem (operator 2026-10-25: "also need to add a bug report so
// people can send to us"; reports land in #bug-reports on the Discord, posted
// by the person). The page asks what happened; this bundles it with what we
// need to look into it - the version, licence state (never the key), the
// roles, the links' health, the settings (secrets only as set / not set), the
// config files and the end of every role log - into one zip to attach there.
// Every key this PC holds is scrubbed from everything in the zip, so nothing
// secret can leave in a log line that happened to print one.

// bugLogTail is how much of the end of each role log goes in.
const bugLogTail = 1 << 20

// Things that look like keys even where we don't know the value.
var bugSecretRE = regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{16,}|sk_[A-Za-z0-9]{16,}|SKYEYE1-[A-Za-z0-9._-]{8,})`)

func serveBugReportPage(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("bugreport.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleBugReport: POST (control) what / when / where -> the zip.
func handleBugReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	clip := func(s string, n int) string {
		s = strings.TrimSpace(s)
		if len(s) > n {
			s = s[:n]
		}
		return s
	}
	what, when, where := clip(r.FormValue("what"), 4000), clip(r.FormValue("when"), 200), clip(r.FormValue("where"), 200)
	now := time.Now()
	var buf bytes.Buffer
	if err := writeBugReport(&buf, what, when, where, now); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Readback-problem-%s.zip"`, now.Format("2006-01-02-1504")))
	w.Write(buf.Bytes())
}

// bugSecrets are the key values on this PC, longest first.
func bugSecrets() []string {
	var out []string
	for _, v := range settingVars {
		if s := strings.TrimSpace(userEnv(v.Name)); v.Secret && len(s) >= 6 {
			out = append(out, s)
		}
	}
	var lic struct {
		Key string `json:"key"`
	}
	if b, err := os.ReadFile(filepath.Join(rootDir, "licence.json")); err == nil && json.Unmarshal(b, &lic) == nil && len(lic.Key) >= 6 {
		out = append(out, lic.Key)
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func scrub(b []byte, secrets []string) []byte {
	for _, s := range secrets {
		b = bytes.ReplaceAll(b, []byte(s), []byte("[key removed]"))
	}
	return bugSecretRE.ReplaceAll(b, []byte("[key removed]"))
}

func writeBugReport(w io.Writer, what, when, where string, now time.Time) error {
	secrets := bugSecrets()
	z := zip.NewWriter(w)
	add := func(name string, b []byte) error {
		f, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: now})
		if err != nil {
			return err
		}
		_, err = f.Write(scrub(b, secrets))
		return err
	}
	addJSON := func(name string, v any) error {
		b, _ := json.MarshalIndent(v, "", "  ")
		return add(name, b)
	}

	// The summary first: what the person said and what this copy is.
	var s strings.Builder
	fmt.Fprintf(&s, "Readback ATC problem report, made %s\n\n", now.Format("Mon 2 Jan 2006 15:04 MST"))
	fmt.Fprintf(&s, "What happened:\n%s\n\nWhen: %s\nWhere (role / frequency): %s\n\n", orDash(what), orDash(when), orDash(where))
	v := cachedVersion
	fmt.Fprintf(&s, "Readback: %s (build %s %s)\n", orDash(productVersion), v.Short, v.Time)
	if rev, _, err := exeRevision(filepath.Join(rootDir, "atc.exe")); err == nil {
		fmt.Fprintf(&s, "atc.exe build: %s\n", shortRev(rev))
	}
	fmt.Fprintf(&s, "Windows: %s\n", windowsVersion())
	lic := licenceStatus()
	fmt.Fprintf(&s, "Licence: %s - %s\n", lic.State, lic.Message)
	if lic.Licence != nil {
		fmt.Fprintf(&s, "Licensed to: %s (expires %s)\n", lic.Licence.Squadron, orDash(lic.Licence.Expires))
	}
	fmt.Fprintf(&s, "Install folder: %s\n", rootDir)
	if err := add("report.txt", []byte(s.String())); err != nil {
		return err
	}

	// Settings: the values, except secrets, which say only whether they are set.
	var set strings.Builder
	for _, sv := range settingVars {
		val := userEnv(sv.Name)
		switch {
		case val == "":
			val = "(not set)"
		case sv.Secret:
			val = "(set)"
		}
		fmt.Fprintf(&set, "%s: %s\n", sv.Label, val)
	}
	if err := add("settings.txt", []byte(set.String())); err != nil {
		return err
	}

	rolesMu.Lock()
	rs := append([]Role(nil), roles...)
	rolesMu.Unlock()
	pids := detectRunning(rs)
	for i := range rs {
		rs[i].Status = "stopped"
		if pids[i] != 0 {
			rs[i].Status, rs[i].PID = "running", pids[i]
		}
	}
	healthMu.Lock()
	h := cachedHealth
	healthMu.Unlock()
	alertsMu.Lock()
	al := append([]Alert(nil), cachedAlerts...)
	alertsMu.Unlock()
	for name, val := range map[string]any{"roles.json": rs, "health.json": h, "alerts.json": al, "version.json": v} {
		if err := addJSON("status/"+name, val); err != nil {
			return err
		}
	}

	// The squadron's changes to the defaults (licence.json, the key, stays out).
	for _, name := range []string{"roles.json", "frequencies.json", "sides.json", "update_last.json"} {
		if b, err := os.ReadFile(filepath.Join(rootDir, name)); err == nil {
			if err := add("config/"+name, b); err != nil {
				return err
			}
		}
	}

	// The end of each log: the live file, topped up from the last rotated copy.
	logs, _ := filepath.Glob(filepath.Join(rootDir, "logs", "*.log"))
	for _, p := range logs {
		b := tailFile(p, bugLogTail)
		if len(b) < bugLogTail {
			b = append(tailFile(p+".1", bugLogTail-len(b)), b...)
		}
		if len(b) == 0 {
			continue
		}
		if err := add("logs/"+filepath.Base(p), b); err != nil {
			return err
		}
	}
	return z.Close()
}

// tailFile is the last n bytes of a file, from the start of a line.
func tailFile(path string, n int) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil
	}
	off := st.Size() - int64(n)
	if off < 0 {
		off = 0
	}
	b := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(b, off); err != nil && err != io.EOF {
		return nil
	}
	if off > 0 {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	return b
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func windowsVersion() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return runtime.GOOS
	}
	defer k.Close()
	name, _, _ := k.GetStringValue("ProductName")
	disp, _, _ := k.GetStringValue("DisplayVersion")
	build, _, _ := k.GetStringValue("CurrentBuild")
	if n, _ := strconv.Atoi(build); n >= 22000 { // Windows 11 still calls itself 10 here
		name = strings.Replace(name, "Windows 10", "Windows 11", 1)
	}
	return strings.TrimSpace(fmt.Sprintf("%s %s (build %s)", name, disp, build))
}
