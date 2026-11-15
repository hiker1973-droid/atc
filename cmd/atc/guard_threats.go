package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// Guard warnings (operator 2026-11-15, "do B"): Guard calls a pilot on its own
// when a red SAM battery's reach or an unidentified aircraft is close, from the
// Tacview picture (guard_contacts.go). Two kinds:
//
//	"Raider 302, Guard, SAM threat, SA-6, bearing 0 9 0, 12 miles."
//	"Raider 302, Guard, unidentified aircraft, bearing 0 4 5, 32 miles, angels 25."
//
// A warning repeats for the same threat only after a cooldown, a pilot hears at
// most one every guardPilotGap, and Guard stays quiet while a pilot is
// transmitting (guardLastHeard) or we are.

type samType struct {
	match     string  // lower-case substring of the Tacview Name
	label     string  // what Guard says
	rangeNm   float64 // reach used for the warning
	ceilingFt float64
}

// samTypes: the DCS surface-to-air systems a red force has on Kola. The reach
// is the usual planning figure, not DCS's exact envelope.
var samTypes = []samType{
	{"kub", "SA-6", 13, 45000}, {"2p25", "SA-6", 13, 45000}, {"1s91", "SA-6", 13, 45000},
	{"buk", "SA-11", 18, 50000}, {"9a310", "SA-11", 18, 50000}, {"9s470", "SA-11", 18, 50000},
	{"osa", "SA-8", 7, 16000}, {"9a33", "SA-8", 7, 16000},
	{"9a331", "SA-15", 8, 20000}, {"sa-15", "SA-15", 8, 20000},
	{"s-300", "SA-10", 45, 90000}, {"5p85", "SA-10", 45, 90000}, {"64h6", "SA-10", 45, 90000},
	{"s-75", "SA-2", 25, 60000}, {"sa-2", "SA-2", 25, 60000},
	{"tunguska", "SA-19", 4, 12000}, {"2s6", "SA-19", 4, 12000},
	{"strela", "SA-13", 3, 11000},
	{"igla", "SA-18", 3, 11000}, {"sa-18", "SA-18", 3, 11000},
}

func samFor(name string) (samType, bool) {
	n := strings.ToLower(name)
	for _, s := range samTypes {
		if strings.Contains(n, s.match) {
			return s, true
		}
	}
	return samType{}, false
}

// samSite is a battery: nearby red launchers and radars of one system.
type samSite struct {
	Label     string
	Pos       orb.Point
	RangeNm   float64
	CeilingFt float64
	N         int
}

const samClusterNm = 4.0

// samSites groups the red SAM contacts into batteries.
func samSites(contacts []tacviewContact) []samSite {
	var sites []samSite
	for _, c := range contacts {
		if c.IsAir() || !c.IsRed() {
			continue
		}
		s, ok := samFor(c.Name)
		if !ok {
			continue
		}
		placed := false
		for i := range sites {
			if sites[i].Label == s.label && haversineNm(sites[i].Pos, c.Pos) <= samClusterNm {
				n := float64(sites[i].N)
				sites[i].Pos = orb.Point{(sites[i].Pos[0]*n + c.Pos[0]) / (n + 1), (sites[i].Pos[1]*n + c.Pos[1]) / (n + 1)}
				sites[i].N++
				placed = true
				break
			}
		}
		if !placed {
			sites = append(sites, samSite{Label: s.label, Pos: c.Pos, RangeNm: s.rangeNm, CeilingFt: s.ceilingFt, N: 1})
		}
	}
	return sites
}

type guardWarning struct {
	Callsign string
	Text     string
}

const (
	guardSAMMargin    = 1.15              // warn a little outside the reach
	guardAirFarNm     = 40.0              // first call on an unidentified aircraft
	guardAirNearNm    = 15.0              // second call
	guardSAMCooldown  = 180 * time.Second // same battery, same pilot
	guardAirCooldown  = 300 * time.Second
	guardNearCooldown = 120 * time.Second
	guardPilotGap     = 20 * time.Second // between any two warnings to one pilot
	guardMinAltFt     = 1500             // a jet on the ramp is not warned
	guardScanEvery    = 5 * time.Second
	guardTXGap        = 8 * time.Second // between any two warnings on the frequency
	guardQuietAfter   = 4 * time.Second // after a pilot's call
)

// guardLastHeard is when a pilot last transmitted on Guard (unix nanos).
var guardLastHeard atomic.Int64

// guardWatch remembers what it has already warned about.
type guardWatch struct {
	mu   sync.Mutex
	last map[string]time.Time
	name string
}

func newGuardWatch(name string) *guardWatch {
	return &guardWatch{last: map[string]time.Time{}, name: name}
}

// due reports whether key may speak now, and marks it spoken if so.
func (w *guardWatch) due(key string, now time.Time, cooldown time.Duration) bool {
	if t, ok := w.last[key]; ok && now.Sub(t) < cooldown {
		return false
	}
	return true
}

func (w *guardWatch) mark(key string, now time.Time) {
	w.last[key] = now
	if len(w.last) > 2000 {
		for k, t := range w.last {
			if now.Sub(t) > time.Hour {
				delete(w.last, k)
			}
		}
	}
}

// scan returns the warnings due now: at most one per pilot.
func (w *guardWatch) scan(now time.Time, contacts []tacviewContact) []guardWarning {
	w.mu.Lock()
	defer w.mu.Unlock()
	sites := samSites(contacts)
	var reds []tacviewContact
	for _, c := range contacts {
		if c.IsAir() && c.IsRed() && c.AltFt >= guardMinAltFt && now.Sub(c.Seen) < 30*time.Second { // parked jets and ground-hugging patrols are not calls
			reds = append(reds, c)
		}
	}
	var out []guardWarning
	for _, p := range contacts {
		if !p.IsPlayer() || !p.IsBlue() || p.AltFt < guardMinAltFt || now.Sub(p.Seen) > 30*time.Second {
			continue
		}
		cs := p.Callsign()
		if cs == "" || !w.due("any|"+cs, now, guardPilotGap) {
			continue
		}
		magvar := nearestFieldMagVar(p.Pos)
		say := func(key, text string) {
			w.mark(key, now)
			w.mark("any|"+cs, now)
			out = append(out, guardWarning{Callsign: cs, Text: text})
		}
		// SAMs first: the nearest battery that reaches the jet.
		var best *samSite
		bestNm := math.MaxFloat64
		for i := range sites {
			s := &sites[i]
			d := haversineNm(p.Pos, s.Pos)
			if d <= s.RangeNm*guardSAMMargin && p.AltFt <= s.CeilingFt && d < bestNm {
				best, bestNm = s, d
			}
		}
		if best != nil {
			key := fmt.Sprintf("sam|%s|%s|%.1f|%.1f", cs, best.Label, best.Pos[0], best.Pos[1])
			if w.due(key, now, guardSAMCooldown) {
				say(key, fmt.Sprintf("%s, %s, SAM threat, %s, bearing %s, %d miles.", cs, w.name, best.Label,
					spokenBearing(toMagnetic(initialBearing(p.Pos, best.Pos), magvar)), int(bestNm+0.5)))
				continue
			}
		}
		// Then the nearest unidentified aircraft.
		var near *tacviewContact
		nearNm := math.MaxFloat64
		for i := range reds {
			if d := haversineNm(p.Pos, reds[i].Pos); d < nearNm {
				near, nearNm = &reds[i], d
			}
		}
		if near == nil || nearNm > guardAirFarNm {
			continue
		}
		tier, cool := "far", guardAirCooldown
		if nearNm <= guardAirNearNm {
			tier, cool = "near", guardNearCooldown
		}
		key := fmt.Sprintf("air|%s|%s|%s", cs, near.ID, tier)
		if !w.due(key, now, cool) {
			continue
		}
		text := fmt.Sprintf("%s, %s, unidentified aircraft, bearing %s, %d miles, angels %d.", cs, w.name,
			spokenBearing(toMagnetic(initialBearing(p.Pos, near.Pos), magvar)), int(nearNm+0.5), int(near.AltFt/1000+0.5))
		if tier == "far" && nearNm <= guardAirNearNm {
			w.mark(fmt.Sprintf("air|%s|%s|far", cs, near.ID), now)
		}
		if tier == "near" {
			w.mark(fmt.Sprintf("air|%s|%s|far", cs, near.ID), now) // the far call is moot
		}
		say(key, text)
	}
	return out
}

// runGuardWatch scans the Tacview picture and speaks the warnings through tx.
func runGuardWatch(ctx context.Context, store *tacviewPositions, w *guardWatch, freqMHz float64, tx func(text string)) {
	tk := time.NewTicker(guardScanEvery)
	defer tk.Stop()
	var lastTX time.Time
	log.Info().Msg("Guard: warnings armed - SAM threats and unidentified aircraft")
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}
		now := time.Now()
		if now.Sub(lastTX) < guardTXGap || ownTXOnAir(freqMHz) ||
			now.UnixNano()-guardLastHeard.Load() < int64(guardQuietAfter) {
			continue
		}
		ws := w.scan(now, store.Contacts(2*time.Minute))
		if len(ws) == 0 {
			continue
		}
		// One call this tick; the rest are still due and go on the next.
		first := ws[0]
		for _, extra := range ws[1:] {
			w.mu.Lock()
			delete(w.last, "any|"+extra.Callsign)
			for k := range w.last {
				if strings.HasPrefix(k, "sam|"+extra.Callsign+"|") || strings.HasPrefix(k, "air|"+extra.Callsign+"|") {
					if w.last[k].Equal(now) {
						delete(w.last, k)
					}
				}
			}
			w.mu.Unlock()
		}
		lastTX = now
		log.Info().Str("callsign", first.Callsign).Str("text", first.Text).Msg("Guard warning")
		tx(first.Text)
	}
}
