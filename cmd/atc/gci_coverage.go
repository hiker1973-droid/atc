package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/coalitions"
	"github.com/vsfg7/atc/pkg/gci/sim"
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
// Terrain masking is phase 2.

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

type gciCoverage struct {
	store     *tacviewPositions
	gci       string               // our callsign, matched in the AWACS unit name
	coalition coalitions.Coalition // our side
	fades     chan<- sim.Faded     // SkyEye's fade input
	now       func() time.Time

	mu     sync.Mutex
	seen   map[uint64]bool // hostiles on our scope
	radar  *gciRadar       // last platform found, for logging changes
	onStn  bool            // announced state
	since  time.Time       // when the current raw state began
	rawOn  bool
	booted bool
}

func newGCICoverage(store *tacviewPositions, gci string, side coalitions.Coalition, fades chan<- sim.Faded) *gciCoverage {
	return &gciCoverage{store: store, gci: gci, coalition: side, fades: fades, now: time.Now, seen: map[uint64]bool{}}
}

// tacviewSide: the ACMI Coalition string for our side.
func tacviewSide(c coalitions.Coalition) string {
	if c == coalitions.Red {
		return "enemies"
	}
	return "allies"
}

// platform finds the GCI's AWACS: airborne, recently updated, our side.
func (c *gciCoverage) platform() (gciRadar, bool) {
	c.store.mu.RLock()
	defer c.store.mu.RUnlock()
	side := tacviewSide(c.coalition)
	var named, any []gciRadar
	for key, info := range c.store.info {
		rng, ok := gciRadarRangeNm[info.Name]
		if !ok || !strings.EqualFold(info.Coalition, side) {
			continue
		}
		if c.now().Sub(info.UpdatedAt) > gciAWACSStale || info.AltFt < gciAWACSMinAltFt {
			continue
		}
		r := gciRadar{Key: key, Type: info.Name, Pos: c.store.pos[key], AltFt: info.AltFt, RangeNm: rng}
		if strings.Contains(strings.ToLower(key), strings.ToLower(c.gci)) {
			named = append(named, r)
		} else {
			any = append(any, r)
		}
	}
	for _, list := range [][]gciRadar{named, any} {
		if len(list) > 0 {
			sort.Slice(list, func(i, j int) bool { return list[i].Key < list[j].Key })
			return list[0], true
		}
	}
	return gciRadar{}, false
}

// sees: inside the radar horizon and the type's range.
func (r gciRadar) sees(p orb.Point, altFt float64) bool {
	d := haversineNm(r.Pos, p)
	return d <= r.RangeNm && d <= radarHorizonNm(r.AltFt, altFt)
}

// filter is the radar.CoverageFilter hook.
func (c *gciCoverage) filter(u sim.Updated) bool {
	if u.Labels.Coalition == c.coalition {
		return true // friendlies: IFF / datalink
	}
	r, on := c.platform()
	visible := on && r.sees(u.Frame.Point, u.Frame.Altitude.Feet())
	id := u.Labels.ID
	c.mu.Lock()
	defer c.mu.Unlock()
	if visible {
		c.seen[id] = true
		return true
	}
	if c.seen[id] {
		delete(c.seen, id)
		// Out of coverage while the AWACS is up: fade it, as a real scope would.
		// Off station: dropped quietly (no FADED call per contact).
		if on && c.fades != nil {
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

// onStation reports whether the GCI has an AWACS up. changed is true once
// when a state has held gciStationDebounce (the first state after start is
// taken silently).
func (c *gciCoverage) onStation() (on, changed bool, r gciRadar) {
	r, raw := c.platform()
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if raw && (c.radar == nil || c.radar.Key != r.Key) {
		log.Info().Str("awacs", r.Key).Str("type", r.Type).Float64("rangeNm", r.RangeNm).Msg("GCI radar platform")
		rr := r
		c.radar = &rr
	}
	if !c.booted {
		c.booted, c.onStn, c.rawOn, c.since = true, raw, raw, now
		log.Info().Bool("onStation", raw).Msg("GCI coverage: initial state")
		return raw, false, r
	}
	if raw != c.rawOn {
		c.rawOn, c.since = raw, now
	}
	if c.rawOn != c.onStn && now.Sub(c.since) >= gciStationDebounce {
		c.onStn = c.rawOn
		if !c.onStn {
			c.radar = nil
			c.seen = map[uint64]bool{}
		}
		return c.onStn, true, r
	}
	return c.onStn, false, r
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
