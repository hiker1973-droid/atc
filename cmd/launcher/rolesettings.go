package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/rolecfg"
)

// Settings page, Roles section (operator 2026-10-23: "Should we allow to set a
// voice type?" then "yes go ahead"): frequency, name, voices, speed and on/off
// for every station a theatre's bats start. What the bats say is the default;
// changes go to roles.json next to atc.exe (pkg/rolecfg), which atc.exe
// applies at startup.

var rolesCfgMu sync.Mutex

func rolesCfgPath() string { return filepath.Join(rootDir, "roles.json") }

func writeRolesCfg(f rolecfg.File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := rolesCfgPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, rolesCfgPath())
}

// stationValues are a station's settings: as its bat line has them, or with
// the Settings changes on top.
type stationValues struct {
	Freq   float64 `json:"freq"`
	Name   string  `json:"name"`
	Voice  string  `json:"voice"`
	Eleven string  `json:"eleven"`
	Speed  float64 `json:"speed"`
}

type stationView struct {
	rolecfg.Station
	Role     string           `json:"role"` // the launcher role (window title) that speaks it
	Bat      stationValues    `json:"bat"`
	Override rolecfg.Override `json:"override"`
}

type towerView struct {
	ICAO     string           `json:"icao"`
	Name     string           `json:"name"`
	Role     string           `json:"role"`
	Bat      stationValues    `json:"bat"`
	Override rolecfg.Override `json:"override"`
}

type theatreRolesView struct {
	Stations []stationView `json:"stations"`
	Towers   []towerView   `json:"towers"`
}

// batFlags reads a role's atc.exe flags as its bat line gives them, without
// the leading "--".
func batFlags(r Role) map[string]string {
	toks := splitCmdLine(r.cmd)
	if len(toks) > 0 && strings.HasSuffix(strings.ToLower(toks[0]), "atc.exe") {
		toks = toks[1:]
	}
	raw, _ := flagSet(toks)
	out := map[string]string{}
	for k, v := range raw {
		out[strings.TrimPrefix(k, "--")] = v
	}
	return out
}

// reCommented spots a bat line that is commented out (":: start ..." or
// "rem start ..."); the role scan still lists those.
var reCommented = regexp.MustCompile(`(?im)^\s*(::|rem\s).*start\s+"([^"]+)"`)

func commentedRoles() map[string]bool {
	out := map[string]bool{}
	entries, _ := os.ReadDir(rootDir)
	for _, e := range entries {
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".bat") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(rootDir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range reCommented.FindAllStringSubmatch(string(b), -1) {
			out[m[2]] = true
		}
	}
	return out
}

func flagOr(flags map[string]string, name string) string {
	if name == "" {
		return ""
	}
	if v, ok := flags[name]; ok && !strings.Contains(v, "%") {
		return v
	}
	return rolecfg.Defaults[name]
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// theatreRoles builds a theatre's Roles view from its bats and roles.json.
func theatreRoles(theatre string, cfg rolecfg.File) theatreRolesView {
	rolesMu.Lock()
	rs := append([]Role(nil), roles...)
	rolesMu.Unlock()
	off := commentedRoles()

	byOwner := map[string]Role{}
	var towers []Role
	for _, r := range rs {
		if r.Region != theatre || r.cmd == "" || off[r.Name] {
			continue
		}
		owner := rolecfg.OwnerOf(batFlags(r))
		if owner == rolecfg.OwnerTower {
			towers = append(towers, r)
			continue
		}
		if _, dup := byOwner[owner]; !dup {
			byOwner[owner] = r
		}
	}
	th := cfg.Theatres[theatre]
	view := theatreRolesView{Stations: []stationView{}, Towers: []towerView{}}
	for _, s := range rolecfg.Stations {
		r, ok := byOwner[s.Owner]
		if !ok {
			continue
		}
		fl := batFlags(r)
		v := stationViewFor(s, r.Name, fl, th.Stations[s.Key])
		// A station inside another role that its bat leaves off (Hitman in
		// Syria) is not running in this theatre.
		if !s.Main && s.Freq != "" && v.Bat.Freq == 0 && !v.Override.Off {
			continue
		}
		view.Stations = append(view.Stations, v)
	}
	live := map[string]bool{}
	for _, af := range airfield.FieldsForMap(theatre) {
		live[af.ICAO] = true
	}
	seen := map[string]bool{}
	for _, r := range towers {
		fl := batFlags(r)
		icao := strings.ToUpper(fl["airfield"])
		af := airfield.ByICAO(icao)
		if af == nil || !live[icao] || seen[icao] {
			continue
		}
		seen[icao] = true
		view.Towers = append(view.Towers, towerView{
			ICAO: icao, Name: af.Name, Role: r.Name,
			Bat: stationValues{
				Voice:  flagOr(fl, rolecfg.TowerVoiceFlag),
				Eleven: flagOr(fl, rolecfg.TowerElevenFlag),
				Speed:  atof(flagOr(fl, rolecfg.TowerSpeedFlag)),
			},
			Override: th.Towers[icao],
		})
	}
	return view
}

func stationViewFor(s rolecfg.Station, role string, fl map[string]string, ov rolecfg.Override) stationView {
	return stationView{
		Station: s, Role: role, Override: ov,
		Bat: stationValues{
			Freq:   atof(flagOr(fl, s.Freq)),
			Name:   flagOr(fl, s.Name),
			Voice:  flagOr(fl, s.Voice),
			Eleven: flagOr(fl, s.Eleven),
			Speed:  atof(flagOr(fl, s.Speed)),
		},
	}
}

// trimOverride drops the parts of an override that equal the bat's, so
// roles.json holds only real changes.
func trimOverride(o rolecfg.Override, bat stationValues) rolecfg.Override {
	eq := func(a, b float64) bool { return fmt.Sprintf("%.3f", a) == fmt.Sprintf("%.3f", b) }
	if eq(o.Freq, bat.Freq) {
		o.Freq = 0
	}
	if o.Name == bat.Name {
		o.Name = ""
	}
	if o.Voice == bat.Voice {
		o.Voice = ""
	}
	if o.Eleven == bat.Eleven {
		o.Eleven = ""
	}
	if eq(o.Speed, bat.Speed) {
		o.Speed = 0
	}
	return o
}

// setRoleOverride stores a station's (key) or a tower's (icao) changes.
func setRoleOverride(theatre, key, icao string, o rolecfg.Override) error {
	rolesCfgMu.Lock()
	defer rolesCfgMu.Unlock()
	cfg, err := rolecfg.Read(rolesCfgPath())
	if err != nil {
		return fmt.Errorf("roles.json: %w", err)
	}
	view := theatreRoles(theatre, cfg)
	th := cfg.Theatres[theatre]
	if th.Stations == nil {
		th.Stations = map[string]rolecfg.Override{}
	}
	if th.Towers == nil {
		th.Towers = map[string]rolecfg.Override{}
	}
	switch {
	case key != "":
		var sv *stationView
		for i := range view.Stations {
			if view.Stations[i].Key == key {
				sv = &view.Stations[i]
			}
		}
		if sv == nil {
			return fmt.Errorf("%s does not run in %s", key, theatre)
		}
		if err := rolecfg.Validate(&sv.Station, o); err != nil {
			return err
		}
		if o = trimOverride(o, sv.Bat); o == (rolecfg.Override{}) {
			delete(th.Stations, key)
		} else {
			th.Stations[key] = o
		}
	case icao != "":
		var tv *towerView
		for i := range view.Towers {
			if view.Towers[i].ICAO == icao {
				tv = &view.Towers[i]
			}
		}
		if tv == nil {
			return fmt.Errorf("no %s tower in %s", icao, theatre)
		}
		if o.Freq != 0 || o.Name != "" || o.Off {
			return fmt.Errorf("a tower's frequency is in the Frequencies table; towers cannot be switched off here")
		}
		if err := rolecfg.Validate(nil, o); err != nil {
			return err
		}
		if o = trimOverride(o, tv.Bat); o == (rolecfg.Override{}) {
			delete(th.Towers, icao)
		} else {
			th.Towers[icao] = o
		}
	default:
		return fmt.Errorf("want a station key or a tower icao")
	}
	if len(th.Stations) == 0 && len(th.Towers) == 0 {
		delete(cfg.Theatres, theatre)
	} else {
		cfg.Theatres[theatre] = th
	}
	return writeRolesCfg(cfg)
}

// restartTheatre restarts the theatre's running roles so they read the new
// settings: stop them all, then start each bat once (a bat that starts
// several roles brings them all back; one already up refuses its second copy).
func restartTheatre(theatre string) []string {
	rolesMu.Lock()
	var names []string
	for _, r := range roles {
		if r.Region == theatre {
			names = append(names, r.Name)
		}
	}
	rolesMu.Unlock()
	running := runningRoles(names)
	if len(running) == 0 {
		return nil
	}
	go func() {
		for _, n := range running {
			_ = stopRole(n)
		}
		time.Sleep(2 * time.Second)
		started := map[string]bool{}
		for _, n := range running {
			if r := findRole(n); r != nil && !started[r.Bat] {
				started[r.Bat] = true
				_ = startRole(n)
			}
		}
	}()
	return running
}

// --- voices ---

type elevenVoiceView struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Labels  map[string]string `json:"labels,omitempty"`
	Preview string            `json:"preview,omitempty"`
}

var (
	voicesMu    sync.Mutex
	voicesCache []elevenVoiceView
	voicesAt    time.Time
)

// elevenVoices lists the voices in the rig's ElevenLabs account (its own and
// the library voices added to it), with ElevenLabs' preview clips: playing
// those costs no credits.
func elevenVoices() ([]elevenVoiceView, error) {
	voicesMu.Lock()
	defer voicesMu.Unlock()
	if voicesCache != nil && time.Since(voicesAt) < 10*time.Minute {
		return voicesCache, nil
	}
	key := userEnv("ELEVENLABS_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("no ElevenLabs key")
	}
	req, _ := http.NewRequest(http.MethodGet, elevenAPI+"/v1/voices", nil)
	req.Header.Set("xi-api-key", key)
	resp, err := settingsTestClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach ElevenLabs")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ElevenLabs answered %d (the key may not be allowed to list voices)", resp.StatusCode)
	}
	var body struct {
		Voices []struct {
			ID      string            `json:"voice_id"`
			Name    string            `json:"name"`
			Labels  map[string]string `json:"labels"`
			Preview string            `json:"preview_url"`
		} `json:"voices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]elevenVoiceView, 0, len(body.Voices))
	for _, v := range body.Voices {
		out = append(out, elevenVoiceView{ID: v.ID, Name: v.Name, Labels: v.Labels, Preview: v.Preview})
	}
	voicesCache, voicesAt = out, time.Now()
	return out, nil
}

// handleSettingsVoices: the voice lists for the pickers.
func handleSettingsVoices(w http.ResponseWriter, _ *http.Request) {
	out := map[string]any{"openai": rolecfg.OpenAIVoices}
	if vs, err := elevenVoices(); err != nil {
		out["elevenError"] = err.Error()
	} else {
		out["eleven"] = vs
	}
	writeJSON(w, out)
}

// sampleLine is what a sample says: short, so an ElevenLabs sample costs a
// few dozen credits once (it is kept on disk after).
const sampleLine = "Viper one one, runway two seven, wind two six zero at eight, cleared to land."

var reVoiceID = regexp.MustCompile(`^[A-Za-z0-9]{1,40}$`)

// voiceSample returns an mp3 of sampleLine in a voice, made once and kept in
// tts_cache/samples.
func voiceSample(provider, voice string) ([]byte, error) {
	if !reVoiceID.MatchString(voice) || (provider == "openai" && !rolecfg.ValidOpenAIVoice(voice)) {
		return nil, fmt.Errorf("bad voice %q", voice)
	}
	dir := filepath.Join(rootDir, "tts_cache", "samples")
	path := filepath.Join(dir, provider+"-"+voice+".mp3")
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return b, nil
	}
	var req *http.Request
	switch provider {
	case "openai":
		body, _ := json.Marshal(map[string]any{"model": "gpt-4o-mini-tts", "input": sampleLine, "voice": voice,
			"speed": 1.15, "response_format": "mp3"})
		req, _ = http.NewRequest(http.MethodPost, "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+userEnv("OPENAI_API_KEY"))
	case "eleven":
		body, _ := json.Marshal(map[string]any{"text": sampleLine, "model_id": "eleven_flash_v2_5"})
		req, _ = http.NewRequest(http.MethodPost, elevenAPI+"/v1/text-to-speech/"+voice+"?output_format=mp3_44100_64", bytes.NewReader(body))
		req.Header.Set("xi-api-key", userEnv("ELEVENLABS_API_KEY"))
	default:
		return nil, fmt.Errorf("provider must be openai or eleven")
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach the voice service")
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the voice service answered %d", resp.StatusCode)
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		_ = os.WriteFile(path, b, 0o644)
	}
	return b, nil
}
