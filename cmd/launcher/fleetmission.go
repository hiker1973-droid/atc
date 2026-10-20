// Fleet monitor: what each rig's DCS server is flying.
//
// The /fleet cards show launcher health and roles; this adds the mission the
// rig's DCS server has loaded (from that rig's /api/miz-weather, which follows
// the server hook's current_mission.txt), the DCS link (this launcher's own
// Tacview feed for the rig, see airborne.go) and the traffic in it. It also
// flags a rig whose running roles are for a different theatre than the
// mission: Caucasus ATC talking to a Syria mission is the live mistake this
// page should make obvious.
package main

import (
	"sort"
	"strings"
	"time"
)

// RigMission is the mission a rig's DCS server has loaded.
type RigMission struct {
	Name    string `json:"name,omitempty"`
	Theatre string `json:"theatre,omitempty"` // DCS name; empty from launchers before 2026-10-20
	Region  string `json:"region,omitempty"`  // launcher region key the theatre maps to
	// RegionGuessed is true when Region came from the file name because the
	// rig's launcher does not report the theatre yet.
	RegionGuessed bool `json:"regionGuessed,omitempty"`
	// Mismatch names the regions of the running roles when none of them is
	// the mission's region. Empty = fine or unknown.
	Mismatch []string `json:"mismatch,omitempty"`
	Err      string   `json:"err,omitempty"`
}

// RigAir summarises the rig's Tacview air picture.
type RigAir struct {
	Connected bool         `json:"connected"`
	Err       string       `json:"err,omitempty"`
	AgeSec    float64      `json:"ageSec"`
	Airborne  int          `json:"airborne"`
	OnGround  int          `json:"onGround"`
	Blue      int          `json:"blue"`
	Red       int          `json:"red"`
	Contacts  []AirSummary `json:"contacts,omitempty"`
}

// AirSummary is one airborne contact, trimmed for the card.
type AirSummary struct {
	Callsign string `json:"callsign"`
	Aircraft string `json:"aircraft"`
	Color    string `json:"color"`
	AltFt    int    `json:"altFt"`
}

// theatreRegions maps the DCS theatre name to the launcher's region keys.
var theatreRegions = map[string]string{
	"caucasus":    "caucasus",
	"persiangulf": "pg",
	"syria":       "syria",
	"kola":        "kola",
	"iraq":        "iraq",
	"germanycw":   "germany",
}

// nameRegionHints guesses the region from a mission file name, for rigs whose
// launcher does not report the theatre. Checked in order; first hit wins.
var nameRegionHints = []struct{ token, region string }{
	{"caucasus", "caucasus"}, {"_ca ", "caucasus"}, {"_ca_", "caucasus"},
	{"syria", "syria"}, {"_sy_", "syria"},
	{"kola", "kola"}, {"white night", "kola"},
	{"iraq", "iraq"},
	{"germany", "germany"},
	{"persian", "pg"}, {"_pg_", "pg"},
}

func regionFromName(name string) string {
	n := strings.ToLower(name)
	for _, h := range nameRegionHints {
		if strings.Contains(n, h.token) {
			return h.region
		}
	}
	return ""
}

func pollRigMission(base string, running []Role) *RigMission {
	var wx struct {
		MizName string `json:"mizName"`
		Theatre string `json:"theatre"`
	}
	if err := fleetGetJSON(base+"/api/miz-weather", &wx); err != nil {
		return &RigMission{Err: err.Error()}
	}
	return missionStatus(wx.MizName, wx.Theatre, running)
}

func missionStatus(name, theatre string, running []Role) *RigMission {
	m := &RigMission{Name: name, Theatre: theatre}
	if theatre != "" {
		m.Region = theatreRegions[strings.ToLower(theatre)]
	} else if r := regionFromName(name); r != "" {
		m.Region, m.RegionGuessed = r, true
	}
	if m.Region == "" {
		return m
	}
	seen := map[string]bool{}
	for _, r := range running {
		if r.Region != "" {
			seen[r.Region] = true
		}
	}
	if len(seen) == 0 || seen[m.Region] {
		return m
	}
	for r := range seen {
		m.Mismatch = append(m.Mismatch, r)
	}
	sort.Strings(m.Mismatch)
	return m
}

// rigAir reads this launcher's feed for the rig ("" is this box).
func rigAir(rig Rig) *RigAir {
	key := rig.Name
	if isSelf(rig) {
		key = ""
	}
	airMu.Lock()
	s, ok := airSources[key]
	airMu.Unlock()
	if !ok {
		return nil
	}
	return summariseAir(s.get())
}

func summariseAir(p AirPicture) *RigAir {
	a := &RigAir{Connected: p.Connected, Err: p.Err, OnGround: p.OnGround}
	if !p.At.IsZero() {
		a.AgeSec = time.Since(p.At).Seconds()
	}
	for _, c := range p.Contacts {
		if !c.Airborne {
			continue
		}
		a.Airborne++
		switch strings.ToLower(c.Color) {
		case "blue":
			a.Blue++
		case "red":
			a.Red++
		}
		a.Contacts = append(a.Contacts, AirSummary{Callsign: c.Callsign, Aircraft: c.Aircraft, Color: c.Color, AltFt: c.AltFt})
	}
	sort.Slice(a.Contacts, func(i, j int) bool {
		if a.Contacts[i].Color != a.Contacts[j].Color {
			return a.Contacts[i].Color < a.Contacts[j].Color
		}
		return a.Contacts[i].Callsign < a.Contacts[j].Callsign
	})
	return a
}
