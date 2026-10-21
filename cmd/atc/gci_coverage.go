package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/coalitions"
	"github.com/vsfg7/atc/pkg/gci/sim"
	"github.com/vsfg7/atc/pkg/terrain"
)

// AWACS line of sight, phase 1 (operator 2026-10-21: "Could we do awacs to
// have a certain picture but line of sight based on the aircraft type (e-2
// compared to e-3)"). SkyEye reads every aircraft out of Tacview - a god's-eye
// view. Here the GCI's picture is what the mission's own AI AWACS could see:
//
//   - the radar is the GCI's AWACS aircraft in the mission: the one whose
//     unit name carries the GCI's callsign ("Darkstar 1"), else any AWACS of
//     our coalition (Foothold's "AWACS_BLUE-1");
//   - a hostile is seen if it is inside the radar horizon - both altitudes,
//     4/3-earth: 1.23 x (sqrt(h_awacs ft) + sqrt(h_target ft)) nm - and inside
//     the type's range (gciRadarRangeNm, published open-source figures);
//   - friendlies are always seen (IFF / datalink): check-in, alpha check and
//     quiet-in-recoveries keep working;
//   - no AWACS airborne (not spawned yet, shot down, landed) = off station: no
//     picture, no threat calls, requests answered "off station".
//
// A hostile that drops out of coverage is reported to SkyEye as faded, so it
// leaves the picture and Darkstar calls it faded - what a real AWACS does.
//
// Phase 2 (operator 2026-10-21 "start phase 2"): terrain masking. With a height
// grid for the map (pkg/terrain: Caucasus, Syria, Kola) the line from the AWACS to the
// hostile must also clear the ground - a jet low in a valley behind the main
// ridge drops off the scope until it climbs or an AWACS gets an angle on it.

// gciRadarRangeNm: detection range against a fighter-size target, by ACMI type.
//   - E-3A (AN/APY-2): USAF fact sheet "more than 250 miles" (217 nm) against
//     low-flying targets, farther against higher ones - 217 used for all.
//   - E-2C/D (AN/APS-145): open sources, fighter-size "well beyond 200 nm".
//   - A-50 / KJ-2000 (red, should a red GCI ever run): open-source ~200 nm.
var gciRadarRangeNm = map[string]float64{
	"E-3A": 217, "E-2C": 200, "E-2D": 200, "A-50": 200, "KJ-2000": 200,
}

const (
	gciAWACSStale       = 60 * time.Second // no Tacview update this long = gone
	gciAWACSMinAltFt    = 1000             // on deck / taxiing is not on station
	gciStationDebounce  = 30 * time.Second // a state must hold this long before it is announced
	gciRadarHorizonCoef = 1.23
)

// radarHorizonNm is the 4/3-earth radar horizon between two altitudes (ft MSL).
func radarHorizonNm(aFt, bFt float64) float64 {
	return gciRadarHorizonCoef * (math.Sqrt(math.Max(aFt, 0)) + math.Sqrt(math.Max(bFt, 0)))
}

type gciRadar struct {
	Key     string // tracker key, e.g. "Darkstar 1-1"
	Type    string // ACMI type, e.g. "E-3A"
	Pos     orb.Point
	AltFt   float64
	RangeNm float64
}

// Short is the radar as the dashboard shows it: "E-3 Darkstar 1-1".
func (r gciRadar) Short() string {
	t := strings.TrimSuffix(strings.TrimSuffix(r.Type, "A"), "C")
	return t + " " + r.Key
}

type gciCoverage struct {
	store     *tacviewPositions
	gci       string               // our callsign (logging only since the picture is fused)
	coalition coalitions.Coalition // our side
	fades     chan<- sim.Faded     // SkyEye's fade input
	ground    *terrain.Grid        // terrain masking; nil = radar horizon only
	now       func() time.Time

	mu      sync.Mutex
	seen    map[uint64]bool // hostiles on our scope
	setSig  string          // the radars last logged, to log changes once
	onStn   bool            // announced state
	since   time.Time       // when the current raw state began
	rawOn   bool
	booted  bool
	current []gciRadar // radars at the last onStation check (dashboard)
}

func newGCICoverage(store *tacviewPositions, gci string, side coalitions.Coalition, fades chan<- sim.Faded) *gciCoverage {
	return &gciCoverage{store: store, gci: gci, coalition: side, fades: fades, now: time.Now, seen: map[uint64]bool{}}
}

// tacviewSide: the ACMI Color for our side. Not Coalition=: our servers'
// recorder labels blue "Enemies" (Darkstar 1-1 ... Color=Blue,Coalition=Enemies),
// which kept the GCI off station on Day and Night all of 2026-10-21.
func tacviewSide(c coalitions.Coalition) string {
	if c == coalitions.Red {
		return "Red"
	}
	return "Blue"
}

// platforms is every AWACS of our side that is airborne and recently
// updated. The picture is FUSED over all of them (operator 2026-10-21:
// "combine all the AWACS"), as real AWACS share one picture over datalink:
// an E-3 and two E-2s in the Caucasus missions all feed Darkstar.
func (c *gciCoverage) platforms() []gciRadar {
	c.store.mu.RLock()
	defer c.store.mu.RUnlock()
	side := tacviewSide(c.coalition)
	var out []gciRadar
	for key, info := range c.store.info {
		rng, ok := gciRadarRangeNm[info.Name]
		if !ok || !strings.EqualFold(info.Color, side) {
			continue
		}
		if c.now().Sub(info.UpdatedAt) > gciAWACSStale || info.AltFt < gciAWACSMinAltFt {
			continue
		}
		out = append(out, gciRadar{Key: key, Type: info.Name, Pos: c.store.pos[key], AltFt: info.AltFt, RangeNm: rng})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// sees: inside the radar horizon and the type's range, and (with a grid) the
// line of sight clears the terrain.
func (r gciRadar) sees(p orb.Point, altFt float64, ground *terrain.Grid) bool {
	d := haversineNm(r.Pos, p)
	if d > r.RangeNm || d > radarHorizonNm(r.AltFt, altFt) {
		return false
	}
	return ground == nil || ground.Visible(r.Pos.Lat(), r.Pos.Lon(), r.AltFt, p.Lat(), p.Lon(), altFt)
}

// filter is the radar.CoverageFilter hook: seen by any of our AWACS.
func (c *gciCoverage) filter(u sim.Updated) bool {
	if u.Labels.Coalition == c.coalition {
		return true // friendlies: IFF / datalink
	}
	radars := c.platforms()
	visible := false
	for _, r := range radars {
		if r.sees(u.Frame.Point, u.Frame.Altitude.Feet(), c.ground) {
			visible = true
			break
		}
	}
	id := u.Labels.ID
	c.mu.Lock()
	defer c.mu.Unlock()
	if visible {
		c.seen[id] = true
		return true
	}
	if c.seen[id] {
		delete(c.seen, id)
		// Out of every AWACS' coverage while one is up: fade it, as a real
		// scope would. Off station: dropped quietly (no FADED call per contact).
		if len(radars) > 0 && c.fades != nil {
			go func() {
				select {
				case c.fades <- sim.Faded{ID: id}:
				case <-time.After(5 * time.Second):
				}
			}()
		}
	}
	return false
}

// gciRadarsText: "E-3 Darkstar 1-1 + E-2 Wizard 1-1".
func gciRadarsText(radars []gciRadar) string {
	parts := make([]string, len(radars))
	for i, r := range radars {
		parts[i] = r.Short()
	}
	return strings.Join(parts, " + ")
}

// onStation reports whether any AWACS of ours is up. changed is true once
// when a state has held gciStationDebounce (the first state after start is
// taken silently). radars are the ones feeding the picture now.
func (c *gciCoverage) onStation() (on, changed bool, radars []gciRadar) {
	radars = c.platforms()
	raw := len(radars) > 0
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	c.current = radars
	if sig := gciRadarsText(radars); sig != c.setSig {
		c.setSig = sig
		if raw {
			log.Info().Str("radars", sig).Int("count", len(radars)).Msg("GCI radar picture fed by")
		}
	}
	if !c.booted {
		c.booted, c.onStn, c.rawOn, c.since = true, raw, raw, now
		log.Info().Bool("onStation", raw).Msg("GCI coverage: initial state")
		return raw, false, radars
	}
	if raw != c.rawOn {
		c.rawOn, c.since = raw, now
	}
	if c.rawOn != c.onStn && now.Sub(c.since) >= gciStationDebounce {
		c.onStn = c.rawOn
		if !c.onStn {
			c.seen = map[uint64]bool{}
		}
		return c.onStn, true, radars
	}
	return c.onStn, false, radars
}

// gciStatus is what the GCI tells the launcher's dashboard chip
// (logs\gci-status-<callsign>.json, read by launcher gciProbe).
type gciStatus struct {
	Callsign  string    `json:"callsign"`
	Coverage  bool      `json:"coverage"`  // line of sight on (--gci-awacs-coverage)
	OnStation bool      `json:"onStation"` // an AWACS is up (always true with coverage off)
	Radars    string    `json:"radars"`    // "E-3 Darkstar 1-1 + E-2 Wizard 1-1"
	At        time.Time `json:"at"`
}

func writeGCIStatus(st gciStatus) {
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	path := filepath.Join(`C:\SkyeyeATC\logs`, "gci-status-"+strings.ToLower(st.Callsign)+".json")
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// gciNeedsPicture: a request that needs the radar picture. With no AWACS up
// it is answered "off station, no picture"; radio check, check in / out,
// alpha check, fence and splash are answered as usual (the GCI still hears
// you and knows where friendlies are).
func gciNeedsPicture(req any) bool {
	switch req.(type) {
	case *brevity.PictureRequest, *brevity.BogeyDopeRequest, *brevity.DeclareRequest,
		*brevity.SnaplockRequest, *brevity.SpikedRequest, *brevity.StrobeRequest,
		*brevity.TripwireRequest, *brevity.ShoppingRequest, *brevity.VectorRequest:
		return true
	}
	return false
}

// gciOffStation is the answer to such a request with no AWACS up.
func gciOffStation(callsign, gci string) string {
	return fmt.Sprintf("%s, %s, off station, no picture.", strings.ToUpper(callsign), strings.ToUpper(gci))
}
