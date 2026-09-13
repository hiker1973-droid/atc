package controller

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// Proactive tower calls driven by the Tacview picture rather than a pilot
// transmission. Both are off by default (--unknown-traffic-calls,
// --runway-vacate-chase) so they can be trialled on the dev rig before a live
// server hears them.

const (
	// UnknownTrafficRadiusNm — an aircraft that hasn't talked to the tower is
	// called once it closes inside this range of the field.
	UnknownTrafficRadiusNm = 8.0
	// UnknownTrafficMaxAGLFt — above this it is overflying, not entering.
	UnknownTrafficMaxAGLFt = 5000.0
	// UnknownTrafficCooldown — at most one call per aircraft in this window.
	UnknownTrafficCooldown = 10 * time.Minute
	// UnknownTrafficQuietFor — an aircraft that transmitted to the tower within
	// this window is known traffic and is never called.
	UnknownTrafficQuietFor = 10 * time.Minute

	// RunwayChaseMaxKts — below this on the ground is taxi speed. A landing
	// rollout or a touch and go is still faster, so neither starts the clock.
	RunwayChaseMaxKts = 40.0
	// RunwayChaseDelay — time at taxi speed on the field before tower asks.
	RunwayChaseDelay = 90 * time.Second
	// RunwayChaseRadiusNm — only aircraft on the airfield itself.
	RunwayChaseRadiusNm = 2.0

	// onGroundAGLFt — Tacview altitude above field elevation read as on the
	// ground. Same threshold detectAircraftPhase uses for "taxiing".
	onGroundAGLFt = 100.0
	// contactFreshFor — Tacview data older than this is never acted on.
	contactFreshFor = 15 * time.Second
)

// SetUnknownTrafficCalls enables the "say intentions" call to aircraft that
// enter the zone without talking to the tower. Wired to --unknown-traffic-calls.
func (c *ATCController) SetUnknownTrafficCalls(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unknownTrafficCalls = enabled
}

// SetRunwayVacateChase enables "report clear of the runway" to landed aircraft
// that slow to taxi speed without calling vacated. Wired to --runway-vacate-chase.
func (c *ATCController) SetRunwayVacateChase(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runwayVacateChase = enabled
}

// MarkPlayer records that callsign is flown by a human. DCS gives humans a
// "<modex> | <player>" Pilot field and AI units a bare one. Only humans get the
// unknown-traffic call: an AI flight can't answer and would be called again
// every cooldown for as long as it orbits nearby.
func (c *ATCController) MarkPlayer(callsign string) {
	if callsign == "" {
		return
	}
	c.allPositionsMu.Lock()
	defer c.allPositionsMu.Unlock()
	if c.players == nil {
		c.players = make(map[string]bool)
	}
	c.players[callsign] = true
}

// checkUnknownTraffic asks a human-flown aircraft that is closing on the field
// inside UnknownTrafficRadiusNm, below UnknownTrafficMaxAGLFt, and hasn't
// talked to the tower, to say intentions. Conservative on purpose: first
// sighting never calls (no range trend yet), departing traffic is skipped, and
// at most one aircraft is called per monitor tick so a busy frequency stays
// usable. Caller holds c.mu.
func (c *ATCController) checkUnknownTraffic(ctx context.Context) {
	if c.unknownPrevDistNm == nil {
		c.unknownPrevDistNm = make(map[string]float64)
	}
	if c.unknownWarnedAt == nil {
		c.unknownWarnedAt = make(map[string]time.Time)
	}
	s := c.airfieldState
	center := s.Airfield.Center
	elevFt := float64(s.Airfield.ElevationFt)
	now := time.Now()

	type candidate struct {
		callsign     string
		distNm       float64
		fromFieldDeg float64
	}
	var candidates []candidate

	c.allPositionsMu.RLock()
	for cs, ct := range c.allPositions {
		if now.Sub(ct.UpdatedAt) > contactFreshFor || !c.players[cs] {
			continue
		}
		// Aircraft only. Carriers and escorts are fed through the same map,
		// and an empty type is unknown, so neither is called.
		if !strings.HasPrefix(ct.ObjType, "Air") {
			continue
		}
		pt := orb.Point{ct.Lon, ct.Lat}
		dist := haversineNm(pt, center)
		prev, seen := c.unknownPrevDistNm[cs]
		c.unknownPrevDistNm[cs] = dist

		aglFt := ct.AltFt - elevFt
		if dist > UnknownTrafficRadiusNm || aglFt <= onGroundAGLFt || aglFt > UnknownTrafficMaxAGLFt {
			continue
		}
		if !seen || dist >= prev || ct.DetectedPhase == "departing" {
			continue
		}
		candidates = append(candidates, candidate{cs, dist, bearingDegFromTo(center, pt)})
	}
	c.allPositionsMu.RUnlock()

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].distNm < candidates[j].distNm })
	for _, cand := range candidates {
		if ac := s.Get(cand.callsign); ac != nil && now.Sub(ac.LastContact) < UnknownTrafficQuietFor {
			continue
		}
		if last, ok := c.unknownWarnedAt[cand.callsign]; ok && now.Sub(last) < UnknownTrafficCooldown {
			continue
		}
		c.unknownWarnedAt[cand.callsign] = now
		distNm := int(math.Round(cand.distNm))
		log.Info().
			Str("callsign", cand.callsign).
			Int("distNm", distNm).
			Msg("unknown traffic in zone — asking intentions")
		c.transmitTo(ctx, cand.callsign, c.composer.UnknownTrafficSayIntentions(cand.callsign, distNm, cand.fromFieldDeg))
		return
	}
}

// checkRunwayVacate asks a tracked aircraft that was cleared to land, and has
// then sat on the field at taxi speed for RunwayChaseDelay without calling
// runway vacated, to report clear. Once per landing clearance; a "runway
// vacated" call removes the aircraft from tracking and ends the chase.
// Caller holds c.mu.
func (c *ATCController) checkRunwayVacate(ctx context.Context) {
	s := c.airfieldState
	now := time.Now()
	for _, near := range s.AllAircraftWithinNm(RunwayChaseRadiusNm) {
		// AllAircraftWithinNm returns snapshots; the chase bookkeeping has to
		// land on the tracked record or it would repeat every tick.
		ac := s.Get(near.Aircraft.Callsign)
		if ac == nil || ac.LandingClearedAt.IsZero() || ac.RunwayChased {
			continue
		}
		c.allPositionsMu.RLock()
		ct := c.lookupContact(ac.Callsign)
		slowOnField := false
		if ct != nil && now.Sub(ct.UpdatedAt) <= contactFreshFor {
			aglFt := ct.AltFt - float64(s.Airfield.ElevationFt)
			dist := haversineNm(orb.Point{ct.Lon, ct.Lat}, s.Airfield.Center)
			slowOnField = aglFt < onGroundAGLFt && ct.SpeedKts < RunwayChaseMaxKts && dist <= RunwayChaseRadiusNm
		}
		c.allPositionsMu.RUnlock()

		if !slowOnField {
			ac.GroundSlowSince = time.Time{}
			continue
		}
		if ac.GroundSlowSince.IsZero() {
			ac.GroundSlowSince = now
			continue
		}
		if now.Sub(ac.GroundSlowSince) < RunwayChaseDelay {
			continue
		}
		ac.RunwayChased = true
		log.Info().Str("callsign", ac.Callsign).Msg("landed without runway-vacated call — asking to report clear")
		c.transmitTo(ctx, ac.Callsign, c.composer.ReportClearOfRunway(ac.Callsign))
	}
}
