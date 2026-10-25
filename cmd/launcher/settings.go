package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/debrief"
	"github.com/vsfg7/atc/pkg/license"
	"github.com/vsfg7/atc/pkg/rolecfg"
)

// Settings page (operator 2026-10-23: "backend gui for product. where to
// enter keys for open ai and eleven labs. setting your frequncys"). It edits
// what a rig was set up with by hand before:
//
//   - API keys and the SRS/Tacview connection: the user environment variables
//     the bats and atc.exe already read (setx). The launcher re-reads them
//     before starting a role (refreshKeyEnv), so nothing else changes. A key
//     never leaves the rig: GET shows only whether it is set and its last 4.
//   - Field frequencies: frequencies.json next to atc.exe, applied by atc.exe
//     at startup (pkg/airfield/overrides.go). Only changed values are stored.
//
// Every change takes effect when the roles restart.

// settingVars are the variables the page may write, in display order.
var settingVars = []struct {
	Name, Label string
	Secret      bool
}{
	{"OPENAI_API_KEY", "OpenAI API key", true},
	{"ELEVENLABS_API_KEY", "ElevenLabs API key", true},
	{"SKYEYE_SRS", "SRS server (host:port)", false},
	{"SRS_EAM", "SRS external-audio password", true},
	{"SKYEYE_TACVIEW", "Tacview real-time (host:port)", false},
	{"ANTHROPIC_API_KEY", "Claude API key", true}, // Reports page debriefs (reports.go); roles never read it
}

func settingVar(name string) (label string, secret, ok bool) {
	for _, v := range settingVars {
		if v.Name == name {
			return v.Label, v.Secret, true
		}
	}
	return "", false, false
}

const userEnvKey = `Environment`

// userEnv reads a user environment variable as setx stored it, falling back
// to this process (a machine-wide value, or one passed in by the shell).
// It is a var so a test can stand in for a fresh install.
var userEnv = func(name string) string {
	if k, err := registry.OpenKey(registry.CURRENT_USER, userEnvKey, registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if v, _, err := k.GetStringValue(name); err == nil && v != "" {
			return v
		}
	}
	return os.Getenv(name)
}

// setUserEnv stores (or, empty, deletes) a user environment variable the way
// setx does, and updates this process so the next role started gets it.
func setUserEnv(name, value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, userEnvKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if value == "" {
		if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return os.Unsetenv(name)
	}
	if err := k.SetStringValue(name, value); err != nil {
		return err
	}
	return os.Setenv(name, value)
}

// maskSecret shows that a secret is set without showing it.
func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "••••"
	}
	return "••••" + v[len(v)-4:]
}

// checkSetting validates a value before it is stored.
func checkSetting(name, value string) error {
	if value == "" {
		if name == "ELEVENLABS_API_KEY" || name == "ANTHROPIC_API_KEY" {
			return nil // optional: every voice falls back to OpenAI; reports need it only to write debriefs
		}
		return fmt.Errorf("%s cannot be empty", name)
	}
	if strings.ContainsAny(value, " \t\r\n\"") {
		return fmt.Errorf("%s has spaces or quotes in it", name)
	}
	switch name {
	case "SKYEYE_SRS", "SKYEYE_TACVIEW":
		host, port, err := net.SplitHostPort(value)
		if err != nil || host == "" || port == "" {
			return fmt.Errorf("want host:port, e.g. 192.168.1.220:5008")
		}
	case "OPENAI_API_KEY", "ELEVENLABS_API_KEY", "ANTHROPIC_API_KEY":
		if len(value) < 20 {
			return fmt.Errorf("that is too short to be an API key")
		}
	}
	return nil
}

// ttsEngine is the voice engine roles use: SKYEYE_TTS, else OpenAI on a
// source install (the vSFG-7 rigs) and Windows on a SkyEye-Setup install.
// Mirrors cmd/atc ttsEngine.
func ttsEngine() string {
	switch strings.ToLower(strings.TrimSpace(userEnv("SKYEYE_TTS"))) {
	case "windows":
		return "windows"
	case "openai":
		return "openai"
	}
	if license.IsSource(rootDir) {
		return "openai"
	}
	return "windows"
}

// --- frequencies.json ---

var freqsMu sync.Mutex

func freqsPath() string { return filepath.Join(rootDir, "frequencies.json") }

func writeFreqOverrides(o airfield.FreqOverrides) error {
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	tmp := freqsPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, freqsPath())
}

// setFieldFreqs stores one field's frequencies. A value equal to the built-in
// one, or zero, is dropped, so the file holds only real changes.
func setFieldFreqs(icao string, tower, atis, handoff float64) error {
	af := airfield.ByICAO(icao)
	if af == nil {
		return fmt.Errorf("unknown field %s", icao)
	}
	for _, f := range []float64{tower, atis, handoff} {
		if f != 0 && !airfield.ValidFreqMHz(f) {
			return fmt.Errorf("%.3f MHz is outside 30-400", f)
		}
	}
	if atis != 0 && af.ATISFreqMHz == 0 {
		return fmt.Errorf("%s has no ATIS station", af.Name)
	}
	same := func(a, b float64) bool { return a == 0 || fmt.Sprintf("%.3f", a) == fmt.Sprintf("%.3f", b) }
	ov := airfield.FreqOverride{}
	if !same(tower, af.TowerFreqMHz) {
		ov.Tower = tower
	}
	if !same(atis, af.ATISFreqMHz) {
		ov.ATIS = atis
	}
	if !same(handoff, af.HandoffFreqMHz) {
		ov.Handoff = handoff
	}
	freqsMu.Lock()
	defer freqsMu.Unlock()
	o, err := airfield.ReadFreqOverrides(freqsPath())
	if err != nil {
		return fmt.Errorf("frequencies.json: %w", err)
	}
	if ov == (airfield.FreqOverride{}) {
		delete(o.Fields, af.ICAO)
	} else {
		o.Fields[af.ICAO] = ov
	}
	return writeFreqOverrides(o)
}

// --- GET /api/settings ---

type settingView struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Secret bool   `json:"secret"`
	Set    bool   `json:"set"`
	Value  string `json:"value"` // the masked hint for a secret
}

type fieldFreqView struct {
	ICAO        string                `json:"icao"`
	Name        string                `json:"name"`
	Tower       float64               `json:"tower"`
	ATIS        float64               `json:"atis"`
	Handoff     float64               `json:"handoff"`
	HandoffName string                `json:"handoffName"`
	Override    airfield.FreqOverride `json:"override"`
	// HandoffFollows: the handoff is Command's frequency from the Roles
	// section, not the field's own.
	HandoffFollows bool `json:"handoffFollows,omitempty"`
	// ATISNote: the field's ATIS is set under Roles (added there, or off).
	ATISNote string `json:"atisNote,omitempty"`
}

type theatreView struct {
	Key    string          `json:"key"`
	Label  string          `json:"label"`
	Fields []fieldFreqView `json:"fields"`
}

var settingsTheatres = []struct{ Key, Label string }{
	{"caucasus", "Caucasus"}, {"syria", "Syria"}, {"pg", "Persian Gulf"},
	{"germany", "Germany"}, {"iraq", "Iraq"}, {"kola", "Kola"},
}

func settingsSnapshot() map[string]any {
	vars := []settingView{}
	for _, v := range settingVars {
		val := userEnv(v.Name)
		sv := settingView{Name: v.Name, Label: v.Label, Secret: v.Secret, Set: val != ""}
		if v.Secret {
			sv.Value = maskSecret(val)
		} else {
			sv.Value = val
		}
		vars = append(vars, sv)
	}
	freqsMu.Lock()
	o, ferr := airfield.ReadFreqOverrides(freqsPath())
	freqsMu.Unlock()
	rolesCfgMu.Lock()
	rc, rerr := rolecfg.Read(rolesCfgPath())
	rolesCfgMu.Unlock()
	theatres := []theatreView{}
	roleViews := map[string]theatreRolesView{}
	for _, t := range settingsTheatres {
		tv := theatreView{Key: t.Key, Label: t.Label}
		cmdOv, _ := rc.Station(t.Key, "command")
		for _, af := range airfield.FieldsForMap(t.Key) {
			fv := fieldFreqView{
				ICAO: af.ICAO, Name: af.Name,
				Tower: af.TowerFreqMHz, ATIS: af.ATISFreqMHz, Handoff: af.HandoffFreqMHz,
				HandoffName: af.HandoffCallsign, Override: o.Fields[af.ICAO],
			}
			// Mirrors cmd/atc commandHandoffFollows.
			if cmdOv.Freq > 0 && !cmdOv.Off && strings.EqualFold(af.HandoffCallsign, "command") && fv.Override.Handoff == 0 {
				fv.Handoff, fv.HandoffFollows = cmdOv.Freq, true
			}
			switch a := rc.Theatres[t.Key].ATIS[af.ICAO]; {
			case a.Off:
				fv.ATISNote = "off (Roles)"
			case a.Added && af.ATISFreqMHz == 0:
				fv.ATISNote = fmt.Sprintf("%.3f (Roles)", a.Freq)
			}
			tv.Fields = append(tv.Fields, fv)
		}
		theatres = append(theatres, tv)
		roleViews[t.Key] = theatreRoles(t.Key, rc)
	}
	out := map[string]any{"vars": vars, "theatres": theatres, "roles": roleViews, "tts": ttsEngine()}
	if ferr != nil {
		out["freqError"] = ferr.Error()
	}
	if rerr != nil {
		out["rolesError"] = rerr.Error()
	}
	return out
}

// --- tests: does the key / address work? ---

var settingsTestClient = &http.Client{Timeout: 10 * time.Second}

// testSetting checks a value (or, empty, the stored one) against the service
// it is for. A key goes only to its own provider.
func testSetting(name, value string) (string, error) {
	if value == "" {
		value = userEnv(name)
	}
	if value == "" {
		return "", fmt.Errorf("not set")
	}
	switch name {
	case "OPENAI_API_KEY":
		req, _ := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+value)
		return providerCheck(req, "OpenAI")
	case "ELEVENLABS_API_KEY":
		req, _ := http.NewRequest(http.MethodGet, elevenAPI+"/v1/user/subscription", nil)
		req.Header.Set("xi-api-key", value)
		return providerCheck(req, "ElevenLabs")
	case "ANTHROPIC_API_KEY":
		if err := debrief.CheckKey(context.Background(), value); err != nil {
			return "", err
		}
		return "key accepted", nil
	case "SKYEYE_SRS", "SKYEYE_TACVIEW":
		c, err := net.DialTimeout("tcp", value, 3*time.Second)
		if err != nil {
			return "", fmt.Errorf("no answer at %s", value)
		}
		c.Close()
		return "reachable", nil
	}
	return "", fmt.Errorf("%s has no test", name)
}

func providerCheck(req *http.Request, who string) (string, error) {
	resp, err := settingsTestClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach %s", who)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode == http.StatusUnauthorized && strings.Contains(string(body), "missing_permissions"):
		// A speech-only ElevenLabs key: fine for the roles, but it cannot
		// read the account, so the usage panel stays empty.
		return "key accepted (speech only - it cannot read the account's credits)", nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", fmt.Errorf("%s rejected the key (%d)", who, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("%s answered %d", who, resp.StatusCode)
	}
	if who == "ElevenLabs" {
		var s struct {
			Tier  string `json:"tier"`
			Used  int    `json:"character_count"`
			Limit int    `json:"character_limit"`
		}
		if json.Unmarshal(body, &s) == nil && s.Limit > 0 {
			return fmt.Sprintf("key works - %s plan, %d of %d credits used", s.Tier, s.Used, s.Limit), nil
		}
	}
	return "key works", nil
}

// --- handler ---

// handleSettings: GET the page's data (any viewer; no secret is returned).
// POST (control) with JSON {"op": "set"|"test"|"freq", ...}.
func handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		snap := settingsSnapshot()
		snap["canEdit"] = who(r).Admin // a hint for the page; POST is checked below
		writeJSON(w, snap)
		return
	}
	if code, err := controlAllowed(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	var req struct {
		Op      string  `json:"op"`
		Name    string  `json:"name"`
		Value   string  `json:"value"`
		ICAO    string  `json:"icao"`
		Tower   float64 `json:"tower"`
		ATIS    float64 `json:"atis"`
		Handoff float64 `json:"handoff"`
		// op role / restart / sample
		Theatre  string           `json:"theatre"`
		Key      string           `json:"key"`
		Override rolecfg.Override `json:"override"`
		Provider string           `json:"provider"`
		Voice    string           `json:"voice"`
		// op atis / net / tanker (rolesettings.go)
		ATISSet rolecfg.ATIS      `json:"atisSet"`
		Tanker  rolecfg.TankerNet `json:"tanker"`
		Prev    string            `json:"prev"`
		Remove  bool              `json:"remove"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	req.Value = strings.TrimSpace(req.Value)
	switch req.Op {
	case "set":
		label, _, ok := settingVar(req.Name)
		if !ok {
			http.Error(w, "not a setting: "+req.Name, http.StatusBadRequest)
			return
		}
		if err := checkSetting(req.Name, req.Value); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := setUserEnv(req.Name, req.Value); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "msg": label + " saved. Restart the roles to use it."})
	case "test":
		if _, _, ok := settingVar(req.Name); !ok {
			http.Error(w, "not a setting: "+req.Name, http.StatusBadRequest)
			return
		}
		msg, err := testSetting(req.Name, req.Value)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "msg": msg})
	case "freq":
		icao := strings.ToUpper(strings.TrimSpace(req.ICAO))
		if err := setFieldFreqs(icao, req.Tower, req.ATIS, req.Handoff); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "msg": icao + " saved. Restart its tower, ATIS and Command to use it."})
	case "role":
		o := req.Override
		o.Name, o.Voice, o.Eleven = strings.TrimSpace(o.Name), strings.TrimSpace(o.Voice), strings.TrimSpace(o.Eleven)
		icao := strings.ToUpper(strings.TrimSpace(req.ICAO))
		if err := setRoleOverride(req.Theatre, req.Key, icao, o); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "msg": "Saved. Restart the theatre's roles to use it."})
	case "atis", "net", "tanker":
		var err error
		switch req.Op {
		case "atis":
			err = setATIS(req.Theatre, strings.ToUpper(strings.TrimSpace(req.ICAO)), req.ATISSet)
		case "net":
			err = setNet(req.Theatre, req.Key, req.Override)
		default:
			err = setTanker(req.Theatre, req.Prev, req.Tanker, req.Remove)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "msg": "Saved. Restart the theatre's roles to use it."})
	case "tts":
		// The voice engine (cmd/atc/tts_windows.go): free Windows voices, or
		// OpenAI's. ElevenLabs speaks wherever its key and a role's voice are set.
		if req.Value != "windows" && req.Value != "openai" {
			http.Error(w, "want windows or openai", http.StatusBadRequest)
			return
		}
		if err := setUserEnv("SKYEYE_TTS", req.Value); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		msg := "OpenAI voices on. Restart the roles to hear them."
		if req.Value == "windows" {
			msg = "Windows voices on (free). Restart the roles to hear them."
		}
		writeJSON(w, map[string]any{"ok": true, "msg": msg})
	case "restart":
		if licenceBlocks(w) {
			return
		}
		running := restartTheatre(req.Theatre)
		msg := "Nothing running in that theatre - the changes apply when it starts."
		if len(running) > 0 {
			msg = fmt.Sprintf("Restarting %d roles: %s. Allow a minute for the towers.", len(running), strings.Join(running, ", "))
		}
		writeJSON(w, map[string]any{"ok": true, "msg": msg})
	case "sample":
		b, err := voiceSample(req.Provider, req.Voice)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(b)
	default:
		http.Error(w, "op must be set, test, freq, role, atis, net, tanker, restart or sample", http.StatusBadRequest)
	}
}

func serveSettingsUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("settings.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
