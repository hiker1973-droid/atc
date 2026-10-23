package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Setup wizard (operator 2026-10-23: "start on the setup wizard"): the first
// thing a new install shows. Five steps - keys, server, theatre, test call,
// done - over the Settings page's own API, plus a test call that has atc.exe
// speak one line on SRS (cmd/atc/say.go). setup.json next to atc.exe records
// the chosen theatre and that setup was finished; a rig that already has its
// OpenAI key and SRS address never needs it, so the dashboard only offers it
// when one of those is missing.

type setupState struct {
	Done        bool      `json:"done"`
	Theatre     string    `json:"theatre,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

var setupMu sync.Mutex

func setupPath() string { return filepath.Join(rootDir, "setup.json") }

func readSetup() setupState {
	var s setupState
	if b, err := os.ReadFile(setupPath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func writeSetup(s setupState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := setupPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, setupPath())
}

// externalAudioPath is where DCS-SRS installs the sender every role uses
// (atc.exe's --external-audio default).
const externalAudioPath = `C:\Program Files\Eagle Dynamics\SRS\ExternalAudio\DCS-SR-ExternalAudio.exe`

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// setupTheatres are the theatres this rig has start scripts for.
func setupTheatres() []map[string]any {
	off := commentedRoles()
	rolesMu.Lock()
	have := map[string]int{}
	for _, r := range roles {
		if !off[r.Name] {
			have[r.Region]++
		}
	}
	rolesMu.Unlock()
	var out []map[string]any
	for _, t := range settingsTheatres {
		if have[t.Key] == 0 {
			continue
		}
		var freq float64
		if fs := airfield.FieldsForMap(t.Key); len(fs) > 0 {
			freq = fs[0].TowerFreqMHz
		}
		out = append(out, map[string]any{"key": t.Key, "label": t.Label, "roles": have[t.Key], "testFreq": freq})
	}
	return out
}

func setupSnapshot() map[string]any {
	setupMu.Lock()
	s := readSetup()
	setupMu.Unlock()
	checks := map[string]bool{
		"openai":        userEnv("OPENAI_API_KEY") != "",
		"elevenlabs":    userEnv("ELEVENLABS_API_KEY") != "",
		"srs":           userEnv("SKYEYE_SRS") != "",
		"eam":           userEnv("SRS_EAM") != "",
		"tacview":       userEnv("SKYEYE_TACVIEW") != "",
		"externalAudio": fileExists(externalAudioPath),
		"atcExe":        fileExists(filepath.Join(rootDir, "atc.exe")),
	}
	return map[string]any{
		"done":       s.Done,
		"theatre":    s.Theatre,
		"needsSetup": !s.Done && (!checks["openai"] || !checks["srs"]),
		"checks":     checks,
		"theatres":   setupTheatres(),
	}
}

// testCall has atc.exe speak one line on freq through SRS, in Lisa when the
// rig has an ElevenLabs key, else OpenAI nova.
func testCall(freq float64, text string) (string, error) {
	if freq < 30 || freq > 400 {
		return "", fmt.Errorf("pick a frequency between 30 and 400 MHz")
	}
	srs := userEnv("SKYEYE_SRS")
	if srs == "" {
		return "", fmt.Errorf("set the SRS server first")
	}
	if text == "" {
		text = "Radio check. This is Sky Eye, set up and transmitting on this frequency. How do you read?"
	}
	refreshKeyEnv()
	args := []string{"--say", text, "--freq", fmt.Sprintf("%.3f", freq), "--srs-addr", srs, "--tts-voice", "nova"}
	if eam := userEnv("SRS_EAM"); eam != "" {
		args = append(args, "--eam-password", eam)
	}
	if userEnv("ELEVENLABS_API_KEY") != "" {
		args = append(args, "--elevenlabs-tower-voice", "bG3bxTbEPR7EZE4M0kcC") // Lisa
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(rootDir, "atc.exe"), args...)
	cmd.Dir = rootDir
	out, err := cmd.CombinedOutput()
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(l)
		if f := strings.Fields(l); len(f) >= 2 && f[0] == "SAY-OK" {
			return fmt.Sprintf("Sent on %.3f (%s). Did you hear it in SRS?", freq, strings.Join(f[1:], " ")), nil
		}
	}
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "Error:") {
			return "", fmt.Errorf("%s", strings.TrimSpace(strings.TrimPrefix(l, "Error:")))
		}
	}
	if ctx.Err() != nil {
		return "", fmt.Errorf("no answer from SRS within 100 s")
	}
	if err != nil {
		return "", fmt.Errorf("atc.exe failed: %v", err)
	}
	return "", fmt.Errorf("atc.exe did not say whether it sent")
}

// handleSetup: GET the wizard's state (viewer); POST (control) with JSON
// {"op": "theatre"|"testcall"|"done", ...}.
func handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		snap := setupSnapshot()
		snap["canEdit"] = who(r).Admin
		writeJSON(w, snap)
		return
	}
	if code, err := controlAllowed(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	var req struct {
		Op      string  `json:"op"`
		Theatre string  `json:"theatre"`
		Freq    float64 `json:"freq"`
		Text    string  `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	switch req.Op {
	case "theatre", "done":
		setupMu.Lock()
		s := readSetup()
		if req.Op == "theatre" {
			ok := false
			for _, t := range settingsTheatres {
				ok = ok || t.Key == req.Theatre
			}
			if !ok {
				setupMu.Unlock()
				http.Error(w, "unknown theatre "+req.Theatre, http.StatusBadRequest)
				return
			}
			s.Theatre = req.Theatre
		} else {
			s.Done, s.CompletedAt = true, time.Now().UTC()
		}
		err := writeSetup(s)
		setupMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case "testcall":
		text := strings.TrimSpace(req.Text)
		if len(text) > 200 || strings.ContainsAny(text, "\"%") {
			http.Error(w, "keep the test line short, without quotes or %", http.StatusBadRequest)
			return
		}
		msg, err := testCall(req.Freq, text)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "msg": msg})
	default:
		http.Error(w, "op must be theatre, testcall or done", http.StatusBadRequest)
	}
}

func serveSetupUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("setup.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
