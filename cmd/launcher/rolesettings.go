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
	Bat      stationValues    `json:"bat"` // Bat.Name: what it calls itself, "Senaki Tower"
	Override rolecfg.Override `json:"override"`
}

// atisView is one field's ATIS: built in (its frequency in the Frequencies
// table), or added on this page.
type atisView struct {
	ICAO     string       `json:"icao"`
	Field    string       `json:"field"`
	Name     string       `json:"name"` // the default, "Batumi ATIS"
	Freq     float64      `json:"freq"` // as it will broadcast
	Added    bool         `json:"added,omitempty"`
	Override rolecfg.ATIS `json:"override"`
}

// netView is a tanker beyond the catalog's two: one of the bat's
// --tanker-net lines (Key = its callsign there) or one added here.
type netView struct {
	Key      string            `json:"key,omitempty"`
	Bat      rolecfg.TankerNet `json:"bat"`
	Override rolecfg.Override  `json:"override"`
	Added    bool              `json:"added,omitempty"`
}

type fieldRef struct {
	ICAO string `json:"icao"`
	Name string `json:"name"`
}

type theatreRolesView struct {
	Stations []stationView `json:"stations"`
	Towers   []towerView   `json:"towers"`
	// HasATIS / HasCommand: the theatre's bats run an ATIS / a Command, so
	// ATIS stations / tankers can be added.
	HasATIS    bool       `json:"hasATIS"`
	HasCommand bool       `json:"hasCommand"`
	ATIS       []atisView `json:"atis"`
	Nets       []netView  `json:"nets"`
	// NoATIS: the theatre's fields with no ATIS, which Add can give one.
	NoATIS []fieldRef `json:"noATIS"`
	// JTACs added beside Hitman 1-1 (CanJTAC: Hitman 1-1 is on, so they run),
	// and a second AWACS or Command (CanGCI / CanCommand: the theatre has its
	// own to copy). addedroles.go.
	JTACs      []rolecfg.JTACNet      `json:"jtacs"`
	Added      []rolecfg.AddedStation `json:"added"`
	CanJTAC    bool                   `json:"canJTAC"`
	CanGCI     bool                   `json:"canGCI"`
	CanCommand bool                   `json:"canCommand"`

	key string
}

func (v theatreRolesView) theatre() string { return v.key }

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
		if r.Region != theatre || r.cmd == "" || off[r.Name] || isAddedRole(r) { // added ones are listed from roles.json
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
	view := theatreRolesView{Stations: []stationView{}, Towers: []towerView{}, key: theatre}
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
		name := fl[rolecfg.TowerNameFlag]
		if name == "" || strings.Contains(name, "%") {
			name = af.Name + " Tower"
		}
		view.Towers = append(view.Towers, towerView{
			ICAO: icao, Name: af.Name, Role: r.Name,
			Bat: stationValues{
				Name:   name,
				Voice:  flagOr(fl, rolecfg.TowerVoiceFlag),
				Eleven: flagOr(fl, rolecfg.TowerElevenFlag),
				Speed:  atof(flagOr(fl, rolecfg.TowerSpeedFlag)),
			},
			Override: th.Towers[icao],
		})
	}

	// ATIS: one process speaks every field's broadcast.
	if _, ok := byOwner[rolecfg.OwnerATIS]; ok {
		view.HasATIS = true
		freqs, _ := airfield.ReadFreqOverrides(freqsPath())
		for _, af := range airfield.FieldsForMap(theatre) {
			o := th.ATIS[af.ICAO]
			v := atisView{ICAO: af.ICAO, Field: af.Name, Name: af.Name + " ATIS", Override: o}
			switch {
			case af.ATISFreqMHz > 0:
				v.Freq = af.ATISFreqMHz
				if f := freqs.Fields[af.ICAO].ATIS; airfield.ValidFreqMHz(f) {
					v.Freq = f
				}
			case o.Added:
				v.Freq, v.Added = o.Freq, true
			default:
				view.NoATIS = append(view.NoATIS, fieldRef{ICAO: af.ICAO, Name: af.Name})
				continue
			}
			view.ATIS = append(view.ATIS, v)
		}
	}

	// Tankers past the catalog's two: the Command bat's --tanker-net lines,
	// then the ones added here.
	if r, ok := byOwner[rolecfg.OwnerCommand]; ok {
		view.HasCommand = true
		for _, spec := range batRepeated(r, "tanker-net") {
			t, err := rolecfg.ParseTankerNet(spec)
			if err != nil {
				continue
			}
			view.Nets = append(view.Nets, netView{Key: t.Name, Bat: t, Override: th.Nets[t.Name]})
		}
		for _, t := range th.Tankers {
			view.Nets = append(view.Nets, netView{Bat: t, Added: true})
		}
		// Added JTACs run only beside Hitman 1-1 (startJTACs needs --jtac-freq).
		fl := batFlags(r)
		view.CanJTAC = nonzeroFreq(flagOr(fl, "jtac-freq")) && !th.Stations["hitman1"].Off
		view.CanCommand = true
	}
	_, view.CanGCI = byOwner[rolecfg.OwnerGCI]
	view.JTACs = append([]rolecfg.JTACNet{}, th.JTACs...)
	view.Added = append([]rolecfg.AddedStation{}, th.Added...)
	view.Nets = append([]netView{}, view.Nets...)
	view.ATIS = append([]atisView{}, view.ATIS...)
	view.NoATIS = append([]fieldRef{}, view.NoATIS...)
	return view
}

// batRepeated is every value a repeatable flag has on a role's bat line
// (batFlags keeps only the last).
func batRepeated(r Role, name string) []string {
	var out []string
	toks := splitCmdLine(r.cmd)
	for i := 0; i < len(toks); i++ {
		switch t := toks[i]; {
		case t == "--"+name && i+1 < len(toks):
			out = append(out, toks[i+1])
			i++
		case strings.HasPrefix(t, "--"+name+"="):
			out = append(out, strings.TrimPrefix(t, "--"+name+"="))
		}
	}
	return out
}

// editTheatre changes one theatre's part of roles.json under the lock.
func editTheatre(theatre string, edit func(view theatreRolesView, th *rolecfg.Theatre) error) error {
	rolesCfgMu.Lock()
	defer rolesCfgMu.Unlock()
	cfg, err := rolecfg.Read(rolesCfgPath())
	if err != nil {
		return fmt.Errorf("roles.json: %w", err)
	}
	th := cfg.Theatres[theatre]
	if err := edit(theatreRoles(theatre, cfg), &th); err != nil {
		return err
	}
	if th.Empty() {
		delete(cfg.Theatres, theatre)
	} else {
		cfg.Theatres[theatre] = th
	}
	return writeRolesCfg(cfg)
}

// setATIS stores a field's ATIS changes: a new name or voice, off the air, or
// (a field with none) a new ATIS on its own frequency.
func setATIS(theatre, icao string, a rolecfg.ATIS) error {
	a.Name, a.Voice = strings.TrimSpace(a.Name), strings.TrimSpace(a.Voice)
	return editTheatre(theatre, func(view theatreRolesView, th *rolecfg.Theatre) error {
		if !view.HasATIS {
			return fmt.Errorf("no ATIS runs in %s", theatre)
		}
		builtIn, known := false, false
		for _, v := range view.ATIS {
			if v.ICAO == icao {
				known, builtIn = true, !v.Added
				if a.Name == v.Name {
					a.Name = ""
				}
			}
		}
		for _, f := range view.NoATIS {
			if f.ICAO == icao {
				known = true
				if a.Name == f.Name+" ATIS" {
					a.Name = ""
				}
			}
		}
		if !known {
			return fmt.Errorf("%s is not a field in %s", icao, theatre)
		}
		if builtIn {
			a.Added, a.Freq = false, 0
		} else if !a.Added { // removing an added ATIS forgets it
			delete(th.ATIS, icao)
			return nil
		}
		if err := rolecfg.ValidateATIS(a); err != nil {
			return err
		}
		if a.Added && !a.Off {
			if other := freqInUse(view, a.Freq, "atis:"+icao); other != "" {
				return fmt.Errorf("%.3f is already %s's frequency", a.Freq, other)
			}
		}
		if th.ATIS == nil {
			th.ATIS = map[string]rolecfg.ATIS{}
		}
		if a == (rolecfg.ATIS{}) {
			delete(th.ATIS, icao)
		} else {
			th.ATIS[icao] = a
		}
		return nil
	})
}

// setNet stores changes to one of the bat's --tanker-net tankers.
func setNet(theatre, key string, o rolecfg.Override) error {
	o.Name, o.Eleven = strings.TrimSpace(o.Name), strings.TrimSpace(o.Eleven)
	return editTheatre(theatre, func(view theatreRolesView, th *rolecfg.Theatre) error {
		var nv *netView
		for i := range view.Nets {
			if !view.Nets[i].Added && view.Nets[i].Key == key {
				nv = &view.Nets[i]
			}
		}
		if nv == nil {
			return fmt.Errorf("no tanker %s in %s's start script", key, theatre)
		}
		t := nv.Bat
		if o.Freq > 0 {
			t.Freq = o.Freq
		}
		if o.Name != "" {
			t.Name = o.Name
		}
		if o.Eleven != "" {
			t.Eleven = o.Eleven
		}
		if err := t.Validate(); err != nil {
			return err
		}
		if !o.Off {
			if other := freqInUse(view, t.Freq, "net:"+key); other != "" {
				return fmt.Errorf("%.3f is already %s's frequency", t.Freq, other)
			}
		}
		if fmt.Sprintf("%.3f", o.Freq) == fmt.Sprintf("%.3f", nv.Bat.Freq) {
			o.Freq = 0
		}
		if o.Name == nv.Bat.Name {
			o.Name = ""
		}
		if o.Eleven == nv.Bat.Eleven {
			o.Eleven = ""
		}
		o.Voice, o.Speed = "", 0
		if th.Nets == nil {
			th.Nets = map[string]rolecfg.Override{}
		}
		if o == (rolecfg.Override{}) {
			delete(th.Nets, key)
		} else {
			th.Nets[key] = o
		}
		return nil
	})
}

// setTanker adds a tanker (prev ""), changes the one called prev, or removes it.
func setTanker(theatre, prev string, t rolecfg.TankerNet, remove bool) error {
	t.Name, t.Eleven = strings.TrimSpace(t.Name), strings.TrimSpace(t.Eleven)
	return editTheatre(theatre, func(view theatreRolesView, th *rolecfg.Theatre) error {
		if !view.HasCommand {
			return fmt.Errorf("no Command runs in %s to speak for a tanker", theatre)
		}
		at := -1
		for i, x := range th.Tankers {
			if prev != "" && x.Name == prev {
				at = i
			}
		}
		if prev != "" && at < 0 {
			return fmt.Errorf("no added tanker %s", prev)
		}
		if remove {
			if at < 0 {
				return fmt.Errorf("which tanker?")
			}
			th.Tankers = append(th.Tankers[:at:at], th.Tankers[at+1:]...)
			return nil
		}
		if err := t.Validate(); err != nil {
			return err
		}
		self := "tanker:" + prev
		for _, s := range view.Stations {
			if strings.EqualFold(strings.ReplaceAll(s.Bat.Name, " ", "-"), t.Name) || strings.EqualFold(s.Override.Name, t.Name) {
				return fmt.Errorf("%s is already a station's name", t.Name)
			}
		}
		for _, n := range view.Nets {
			name := n.Bat.Name
			if n.Override.Name != "" {
				name = n.Override.Name
			}
			if strings.EqualFold(name, t.Name) && !(n.Added && n.Bat.Name == prev) {
				return fmt.Errorf("there is already a tanker called %s", t.Name)
			}
		}
		if other := freqInUse(view, t.Freq, self); other != "" {
			return fmt.Errorf("%.3f is already %s's frequency", t.Freq, other)
		}
		if at >= 0 {
			th.Tankers[at] = t
		} else {
			th.Tankers = append(th.Tankers, t)
		}
		return nil
	})
}

// freqInUse names the station already on freq in this theatre ("" for none),
// not counting self ("atis:UGTB", "net:Shell-1-1", "tanker:Arco-1-1").
func freqInUse(view theatreRolesView, freq float64, self string) string {
	same := func(f float64) bool { return f > 0 && fmt.Sprintf("%.3f", f) == fmt.Sprintf("%.3f", freq) }
	for _, s := range view.Stations {
		f := s.Bat.Freq
		if s.Override.Freq > 0 {
			f = s.Override.Freq
		}
		if !s.Override.Off && same(f) {
			return s.Label
		}
	}
	for _, n := range view.Nets {
		f, key := n.Bat.Freq, "net:"+n.Key
		if n.Added {
			key = "tanker:" + n.Bat.Name
		} else if n.Override.Freq > 0 {
			f = n.Override.Freq
		}
		if key != self && !n.Override.Off && same(f) {
			return n.Bat.Name
		}
	}
	for _, a := range view.ATIS {
		if "atis:"+a.ICAO != self && !a.Override.Off && same(a.Freq) {
			return a.Field + " ATIS"
		}
	}
	for _, j := range view.JTACs {
		if "jtac:"+j.Name != self && same(j.Freq) {
			return j.Name
		}
	}
	for _, a := range view.Added {
		if "added:"+a.ID != self && same(a.Freq) {
			return a.Name
		}
	}
	for _, af := range airfield.FieldsForMap(view.theatre()) {
		if same(af.TowerFreqMHz) {
			return af.Name + " Tower"
		}
	}
	return ""
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
		if o.Freq != 0 {
			return fmt.Errorf("a tower's frequency is in the Frequencies table")
		}
		if err := rolecfg.Validate(&rolecfg.TowerStation, o); err != nil {
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
	if th.Empty() {
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
		// Stations added in Settings since the theatre started come up too.
		if !started[addedBat(theatre)] {
			startAddedBat(theatre)
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
