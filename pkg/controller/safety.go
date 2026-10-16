package controller

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Near misses and comms discipline (operator 2026-10-15: "Should we have a
// tower or command report near miss or inproper comms?" -> "build 1, 2 and 3").
//
//  1. One radio is one aircraft: a call from an SRS client that earlier
//     called in under another callsign is the same jet (Whisper heard "Veta
//     201" for "Venom 201" at Senaki and the phantom held the real jet at the
//     runway). "Venom flight" with no number gets "say again with full
//     callsign" unless that radio already gave its full callsign.
//  2. Traffic alerts: two airborne aircraft near the field converging to a
//     close pass get "traffic alert, N o'clock, ..." (checkTrafficAlerts).
//  3. Every comms or safety event goes to the comms report (SetReportFn ->
//     comms_report\YYYY-MM-DD.jsonl, shown on the launcher dashboard). Only
//     the safety calls are spoken.

// CommsEvent is one line of the comms report.
type CommsEvent struct {
	Kind     string `json:"kind"`
	Callsign string `json:"callsign,omitempty"`
	Other    string `json:"other,omitempty"`
	Detail   string `json:"detail"`
	Text     string `json:"text,omitempty"` // what the pilot said, when it came from a call
}

// Comms report kinds.
const (
	EventNoCallsign      = "no_callsign"
	EventFlightOnly      = "flight_only"
	EventCallsignChanged = "callsign_changed"
	EventNotUnderstood   = "not_understood"
	EventDepartedNoClear = "departed_without_clearance"
	EventLandedNoClear   = "landed_without_clearance"
	EventNearMiss        = "near_miss"
	EventLostCall        = "lost_call"
)

const (
	originMemory            = 45 * time.Minute
	trafficAlertCooldown    = 3 * time.Minute
	trafficZoneNm           = 10.0
	trafficZoneAboveFieldFt = 8000.0
)

// Traffic-alert geometry. Conservative so formations, rejoins and the overhead
// break stay quiet: the pair must be converging fast (formation and in-trail
// have next to no closure), on headings at least 45 degrees apart (a rejoin
// or in-trail is nearly parallel), and a straight-line projection must put
// them inside 0.25 nm and 500 ft of each other within 30 seconds.
const (
	trafficMaxRangeNm   = 2.0
	trafficCPANm        = 0.25
	trafficCPAVertFt    = 500.0
	trafficNowVertFt    = 1000.0
	trafficLookahead    = 30 * time.Second
	trafficMinClosureKt = 80.0
	trafficMinHdgDiff   = 45.0
	trafficMinAGLFt     = 300.0
	trafficMinSpeedKt   = 60.0
)

type originSeen struct {
	callsign string
	at       time.Time
}

// SetReportFn receives every comms-report event (cmd/atc writes them to disk).
func (c *ATCController) SetReportFn(fn func(CommsEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reportFn = fn
}

// SetTrafficAlerts turns the tower's traffic alerts on or off (--traffic-alerts).
func (c *ATCController) SetTrafficAlerts(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.trafficAlerts = enabled
}

// ReportLostCall records a transmission SkyEye heard but could not use:
// Whisper handed back its own prompt word list instead of the call (Senaki
// 2026-10-15 18:34, three times on 10-14). Nothing is said on the radio.
func (c *ATCController) ReportLostCall(text, detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.report(CommsEvent{Kind: EventLostCall, Detail: detail, Text: text})
}

func (c *ATCController) report(ev CommsEvent) {
	log.Info().Str("kind", ev.Kind).Str("callsign", ev.Callsign).Str("other", ev.Other).
		Str("detail", ev.Detail).Str("text", ev.Text).Msg("comms report")
	if c.reportFn != nil {
		c.reportFn(ev)
	}
}

// flightOnlyRe finds "<name> flight" - a flight name with no number.
var flightOnlyRe = regexp.MustCompile(`(?i)\b([a-z][a-z'-]+)\s+flight\b`)

var notFlightNames = map[string]bool{"the": true, "my": true, "our": true, "your": true, "a": true,
	"of": true, "in": true, "this": true, "tower": true, "traffic": true, "per": true, "for": true}

// flightOnlyName is "Venom" for "Tower, Venom flight, request departure".
func flightOnlyName(raw string) string {
	for _, m := range flightOnlyRe.FindAllStringSubmatch(raw, -1) {
		w := strings.ToLower(m[1])
		if notFlightNames[w] {
			continue
		}
		return strings.ToUpper(w[:1]) + w[1:]
	}
	return ""
}

// callsignName is the word part of a callsign: "venom" for "Venom 2-0-1".
func callsignName(cs string) string {
	var words []string
	for _, w := range strings.Fields(strings.ToLower(cs)) {
		if strings.IndexAny(w, "0123456789") >= 0 {
			break
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

// resolveNoCallsign handles a call that parsed without a callsign. Returns
// true when the call is dealt with here. A flight name from a radio that
// already called in under that name is that jet; any other flight name gets
// "say again with full callsign"; no name at all stays unanswered. Caller
// holds mu.
func (c *ATCController) resolveNoCallsign(ctx context.Context, req *ATCRequest) bool {
	name := flightOnlyName(req.Raw)
	if name == "" {
		c.report(CommsEvent{Kind: EventNoCallsign, Detail: "call with no callsign - not answered", Text: req.Raw})
		return true
	}
	if seen, ok := c.originCallsigns[req.Origin]; ok && req.Origin != "" &&
		time.Since(seen.at) < originMemory && callsignName(seen.callsign) == strings.ToLower(name) {
		log.Info().Str("flight", name+" flight").Str("callsign", seen.callsign).
			Msg("flight call from a radio that gave its full callsign - answered as that jet")
		req.Callsign = seen.callsign
		return false
	}
	flight := name + " flight"
	c.report(CommsEvent{Kind: EventFlightOnly, Callsign: flight,
		Detail: "flight name with no number - asked for the full callsign", Text: req.Raw})
	c.transmit(ctx, c.composer.SayAgainFullCallsign(flight))
	return true
}

// reconcileOrigin makes one SRS radio one aircraft. When the radio that sent
// req called in before under another callsign, the two are the same jet: the
// tower keeps one entry (and its queue place) under the better of the two
// names - the one Tacview knows, else the one a player is flying under, else
// the newer. Caller holds mu.
func (c *ATCController) reconcileOrigin(req *ATCRequest) {
	if req.Origin == "" || req.Origin == "unknown" {
		return
	}
	if c.originCallsigns == nil {
		c.originCallsigns = make(map[string]originSeen)
	}
	defer func() { c.originCallsigns[req.Origin] = originSeen{req.Callsign, time.Now()} }()
	seen, ok := c.originCallsigns[req.Origin]
	if !ok || time.Since(seen.at) > originMemory || compactCallsign(seen.callsign) == compactCallsign(req.Callsign) {
		return
	}
	old, heard := seen.callsign, req.Callsign
	s := c.airfieldState
	if s.Get(old) == nil {
		return // the old one already left (handed off, landed, pruned)
	}
	keep, dropped := heard, old
	if c.preferCallsign(old, heard) {
		keep, dropped = old, heard
		req.Callsign = old
	} else {
		s.Rename(old, heard)
	}
	log.Info().Str("first", old).Str("then", heard).Str("kept", keep).
		Msg("same radio, two callsigns - one aircraft")
	c.report(CommsEvent{Kind: EventCallsignChanged, Callsign: keep, Other: dropped,
		Detail: fmt.Sprintf("same radio called in as %s, then %s - treated as one aircraft, %s", old, heard, keep),
		Text:   req.Raw})
}

// preferCallsign reports whether old is the better name than heard.
func (c *ATCController) preferCallsign(old, heard string) bool {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	oldTac, newTac := c.lookupContact(old) != nil, c.lookupContact(heard) != nil
	if oldTac != newTac {
		return oldTac
	}
	names := map[string]bool{}
	for p := range c.players {
		names[callsignName(p)] = true
	}
	oldKnown, newKnown := names[callsignName(old)], names[callsignName(heard)]
	if oldKnown != newKnown {
		return oldKnown
	}
	return false
}

// departingJet is a departure's state at the moment it calls airborne.
type departingJet struct {
	callsign, raw                 string
	queued, holdingShort, cleared bool
}

// departedWithoutClearance reports a jet that calls airborne while still
// waiting for its takeoff clearance, and returns the spoken call when there
// was traffic on short final. Caller holds mu.
func (c *ATCController) departedWithoutClearance(j *departingJet) string {
	if j.cleared || !j.queued {
		return ""
	}
	detail := "called airborne without a takeoff clearance"
	if j.holdingShort {
		detail += " (it had called holding short - check whether the tower should have released it)"
	} else {
		detail += " (no holding short or ready call)"
	}
	near := c.airfieldState.InboundsWithinNm(3)
	if len(near) > 0 {
		detail += fmt.Sprintf("; %s was %.1f nm on final", near[0].Aircraft.Callsign, near[0].DistanceNm)
	}
	c.report(CommsEvent{Kind: EventDepartedNoClear, Callsign: j.callsign, Detail: detail, Text: j.raw})
	if len(near) > 0 {
		return c.composer.DepartedWithoutClearance(j.callsign)
	}
	return ""
}

// ── Traffic alerts ────────────────────────────────────────────────────────────

type trafficTrack struct {
	key               string
	x, y              float64 // nm east / north of the field
	vx, vy            float64 // kt east / north
	altFt, vsFpm, hdg float64
	player            bool
}

// checkTrafficAlerts warns aircraft converging on each other near the field.
// Caller holds mu.
func (c *ATCController) checkTrafficAlerts(ctx context.Context) {
	if c.trafficAlertAt == nil {
		c.trafficAlertAt = make(map[string]time.Time)
	}
	s := c.airfieldState
	center := s.Airfield.Center
	elev := float64(s.Airfield.ElevationFt)
	cosLat := math.Cos(center[1] * math.Pi / 180)

	c.allPositionsMu.RLock()
	var tracks []trafficTrack
	for key, ct := range c.allPositions {
		if time.Since(ct.UpdatedAt) > contactFreshFor || (ct.ObjType != "" && !strings.HasPrefix(ct.ObjType, "Air")) {
			continue
		}
		agl := ct.AltFt - elev
		if agl < trafficMinAGLFt || agl > trafficZoneAboveFieldFt || ct.SpeedKts < trafficMinSpeedKt {
			continue
		}
		x, y := (ct.Lon-center[0])*60*cosLat, (ct.Lat-center[1])*60
		if math.Hypot(x, y) > trafficZoneNm {
			continue
		}
		h := ct.HeadingDeg * math.Pi / 180
		tracks = append(tracks, trafficTrack{key: key, x: x, y: y,
			vx: ct.SpeedKts * math.Sin(h), vy: ct.SpeedKts * math.Cos(h),
			altFt: ct.AltFt, vsFpm: ct.VertSpeedFpm, hdg: ct.HeadingDeg, player: c.players[key]})
	}
	c.allPositionsMu.RUnlock()
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].key < tracks[j].key })

	for i := 0; i < len(tracks); i++ {
		for j := i + 1; j < len(tracks); j++ {
			a, b := tracks[i], tracks[j]
			pair := a.key + "|" + b.key
			if t, ok := c.trafficAlertAt[pair]; ok && time.Since(t) < trafficAlertCooldown {
				continue
			}
			dCPA, tCPA, vCPA, ok := trafficConflict(a, b)
			if !ok {
				continue
			}
			nameA, nameB := c.addressFor(a), c.addressFor(b)
			if nameA == "" && nameB == "" && !a.player && !b.player {
				continue // AI against AI
			}
			c.trafficAlertAt[pair] = time.Now()
			dist := math.Hypot(b.x-a.x, b.y-a.y)
			labelA, labelB := firstNonEmpty(nameA, a.key), firstNonEmpty(nameB, b.key)
			log.Warn().Str("a", labelA).Str("b", labelB).Float64("distNm", dist).
				Float64("vertFt", b.altFt-a.altFt).Float64("cpaNm", dCPA).Dur("cpaIn", tCPA).
				Float64("cpaVertFt", vCPA).Msg("traffic alert - aircraft converging")
			c.report(CommsEvent{Kind: EventNearMiss, Callsign: labelA, Other: labelB,
				Detail: fmt.Sprintf("%.1f nm apart, %.0f ft vertical, converging to %.2f nm in %.0f s",
					dist, math.Abs(b.altFt-a.altFt), dCPA, tCPA.Seconds())})
			if nameA != "" {
				c.transmitTo(ctx, nameA, c.composer.TrafficAlert(nameA, clockOf(a, b), dist, b.altFt-a.altFt))
			}
			if nameB != "" {
				c.transmitTo(ctx, nameB, c.composer.TrafficAlert(nameB, clockOf(b, a), dist, a.altFt-b.altFt))
			}
		}
	}
	for k, t := range c.trafficAlertAt {
		if time.Since(t) > trafficAlertCooldown {
			delete(c.trafficAlertAt, k)
		}
	}
}

// trafficConflict projects the pair in straight lines and returns the closest
// point of approach when it is a real conflict.
func trafficConflict(a, b trafficTrack) (dCPA float64, tCPA time.Duration, vCPA float64, ok bool) {
	rx, ry := b.x-a.x, b.y-a.y
	if math.Hypot(rx, ry) > trafficMaxRangeNm || math.Abs(b.altFt-a.altFt) > trafficNowVertFt {
		return 0, 0, 0, false
	}
	vx, vy := b.vx-a.vx, b.vy-a.vy
	v2 := vx*vx + vy*vy
	if math.Sqrt(v2) < trafficMinClosureKt {
		return 0, 0, 0, false
	}
	if d := math.Abs(math.Mod(a.hdg-b.hdg+540, 360) - 180); d < trafficMinHdgDiff {
		return 0, 0, 0, false
	}
	th := -(rx*vx + ry*vy) / v2 // hours
	if th < 0 || th > trafficLookahead.Hours() {
		return 0, 0, 0, false
	}
	dCPA = math.Hypot(rx+vx*th, ry+vy*th)
	vCPA = (b.altFt + b.vsFpm*th*60) - (a.altFt + a.vsFpm*th*60)
	if dCPA > trafficCPANm || math.Abs(vCPA) > trafficCPAVertFt {
		return 0, 0, 0, false
	}
	return dCPA, time.Duration(th * float64(time.Hour)), vCPA, true
}

// clockOf is the clock position of b from a's nose.
func clockOf(a, b trafficTrack) int {
	brg := math.Atan2(b.x-a.x, b.y-a.y) * 180 / math.Pi
	rel := math.Mod(brg-a.hdg+720, 360)
	clock := int(math.Round(rel/30)) % 12
	if clock == 0 {
		clock = 12
	}
	return clock
}

// addressFor is the callsign to speak to a contact: the callsign it called
// the tower with, or a player's modex ("RAIDER 031" -> "Raider 031"). ""
// when it cannot be addressed (AI, or a player whose Tacview name is not a
// callsign, like "406-B.P-CVW5").
func (c *ATCController) addressFor(t trafficTrack) string {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	ct := c.allPositions[t.key]
	if ct == nil {
		return ""
	}
	for _, cs := range c.airfieldState.TrackedCallsigns() {
		if c.lookupContact(cs) == ct {
			return cs
		}
	}
	if t.player && playerCallsignRe.MatchString(t.key) {
		return titleCallsign(t.key)
	}
	return ""
}

// playerCallsignRe is a spoken callsign: a name and a number ("RAIDER 031").
var playerCallsignRe = regexp.MustCompile(`^[A-Za-z][A-Za-z'-]*(\s+[A-Za-z][A-Za-z'-]*)?\s*\d[\d\s-]{0,4}$`)

func titleCallsign(s string) string {
	f := strings.Fields(strings.ToLower(s))
	for i, w := range f {
		f[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(f, " ")
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
