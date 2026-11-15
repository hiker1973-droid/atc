package main

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Intercepts on Guard (operator 2026-11-15, "do C"): a pilot hails an
// unidentified aircraft, hears it answer as its air force, and can order it to
// turn, leave, follow or land. The aircraft is the nearest red aircraft in
// Tacview (to a bearing, if the pilot gave one); its reply is spoken in the
// country's accent (voice_accents.go) and the order is carried out through
// DCS-gRPC (guard_ai.go).
//
//	"Guard, Raider 302, unidentified aircraft bearing 0 4 5, identify."
//	  -> "Aircraft calling, this is Russian Air Force Flanker, on patrol. Go ahead."
//	"Raider 302, turn right heading 2 7 0."  -> "Wilco, turning right heading 2 7 0."
//	"... leave the area." / "... follow me." / "... land at Bodo."

const (
	hailRangeNm    = 80.0
	hailContextFor = 10 * time.Minute
	hailMinAltFt   = 800.0 // a jet parked on its ramp cannot answer
	orderRunNm     = 60.0  // how far a "turn" or "leave" route reaches
)

var (
	guardHailRe   = regexp.MustCompile(`(?i)\b(?:unidentified|unknown)\s+(?:aircraft|traffic|flight|bogey|contact)\b|\bhail(?:ing)?\b|\bidentify\b|\bstate\s+(?:your\s+)?(?:identity|intentions|business)\b|\bsay\s+(?:your\s+)?identity\b`)
	hailBearingRe = regexp.MustCompile(`(?i)\bbearing\s+(\d{3})\b`)
	orderFollowRe = regexp.MustCompile(`(?i)\bfollow\s+(?:me|my\s+lead)\b|\bform\s+(?:up\s+)?on\s+me\b`)
	orderLandRe   = regexp.MustCompile(`(?i)\b(?:land|divert|proceed|escort)\b.{0,25}?\b(?:at|to|into)\b\s+(.+)`)
	orderLeaveRe  = regexp.MustCompile(`(?i)\b(?:leave|depart|exit|vacate|get\s+out\s+of)\b|\bturn\s+(?:around|back)\b`)
	orderTurnRe   = regexp.MustCompile(`(?i)\b(?:(left|right)\s+)?(?:turn\s+(?:(left|right)\s+)?(?:to\s+|onto\s+)?)?(?:a\s+)?heading\s+(?:of\s+)?(\d{3})\b`)
	spacedDigits  = regexp.MustCompile(`(\d)[\s-]+(\d)\b`)
)

// collapseDigits turns "2 7 0" into "270" so a heading reads as one number.
func collapseDigits(s string) string {
	for i := 0; i < 6; i++ {
		n := spacedDigits.ReplaceAllString(s, "$1$2")
		if n == s {
			break
		}
		s = n
	}
	return s
}

// aiIdentity is what a red aircraft calls itself: NATO reporting name and a
// phrase for what it is doing.
func aiIdentity(model string) (nato, doing string) {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "su-27"), strings.Contains(m, "su-30"), strings.Contains(m, "su-33"):
		return "Flanker", "on patrol"
	case strings.Contains(m, "mig-29"):
		return "Fulcrum", "on patrol"
	case strings.Contains(m, "mig-31"):
		return "Foxhound", "on patrol"
	case strings.Contains(m, "su-34"):
		return "Fullback", "on a training flight"
	case strings.Contains(m, "tu-22"):
		return "Backfire", "on a transit flight"
	case strings.Contains(m, "tu-95"):
		return "Bear", "on a transit flight"
	case strings.Contains(m, "a-50"):
		return "Mainstay", "on patrol"
	case strings.Contains(m, "mi-24"):
		return "Hind", "on patrol"
	case strings.Contains(m, "mi-28"):
		return "Havoc", "on patrol"
	}
	return "aircraft", "on patrol"
}

// aiVoice is the red aircraft's radio voice: a Russian man.
func aiVoice() voiceProfile {
	v := countryVoices[accentGender{accentRussian, true}]
	return voiceProfile{voice: "onyx", speed: flagTTSSpeed, eleven: elTower(v.id, v.name),
		instructions: "Speak as a military pilot on the radio: " + accentPhrase(accentRussian, true) +
			". Clipped and flat, a little slow."}
}

type hailTarget struct {
	Target tacviewContact
	Pilot  tacviewContact
	At     time.Time
}

type guardHail struct {
	mu    sync.Mutex
	store *tacviewPositions
	ai    aiOrders
	by    map[string]hailTarget // pilot callsign -> who they hailed
}

func newGuardHail(store *tacviewPositions, ai aiOrders) *guardHail {
	return &guardHail{store: store, ai: ai, by: map[string]hailTarget{}}
}

// guardResult is what to say. Guard speaks Guard's line, the aircraft its own,
// and Do (if set) runs before the aircraft's line, which turns into Fail's
// text if it errors.
type guardResult struct {
	Guard string
	AI    string
	Do    func(ctx context.Context) error
	Fail  string
}

func (r guardResult) handled() bool { return r.Guard != "" || r.AI != "" }

// playerContact finds the contact flown by callsign cs, via its Tacview
// position (the callsign formats differ between a spoken call and the Pilot
// field).
func playerContact(store *tacviewPositions, cs string) (tacviewContact, bool) {
	if store == nil {
		return tacviewContact{}, false
	}
	pos, ok := store.Get(cs)
	if !ok {
		return tacviewContact{}, false
	}
	best, bestNm, found := tacviewContact{}, 1.0, false
	for _, c := range store.Contacts(2 * time.Minute) {
		if c.IsPlayer() {
			if d := haversineNm(pos, c.Pos); d < bestNm {
				best, bestNm, found = c, d, true
			}
		}
	}
	return best, found
}

// pickTarget is the nearest red aircraft to the pilot within hailRangeNm, inside
// 25 degrees of bearingTrue when one was given (bearingTrue < 0 = none).
func pickTarget(contacts []tacviewContact, from orb.Point, bearingMag, magvar float64, now time.Time) (tacviewContact, bool) {
	best, bestNm, found := tacviewContact{}, hailRangeNm, false
	for _, c := range contacts {
		if !c.IsAir() || !c.IsRed() || c.AltFt < hailMinAltFt || now.Sub(c.Seen) > time.Minute {
			continue
		}
		d := haversineNm(from, c.Pos)
		if d > bestNm {
			continue
		}
		if bearingMag >= 0 {
			diff := math.Abs(math.Mod(toMagnetic(initialBearing(from, c.Pos), magvar)-bearingMag+540, 360) - 180)
			if diff > 25 {
				continue
			}
		}
		best, bestNm, found = c, d, true
	}
	return best, found
}

// fieldInText is the map's field named in text, or nil.
func fieldInText(lower string) *airfield.Airfield {
	for _, f := range airfield.FieldsForMap(flagMap) {
		if strings.Contains(lower, strings.ToLower(f.Name)) || (f.DCSName != "" && strings.Contains(lower, strings.ToLower(f.DCSName))) {
			return f
		}
	}
	return nil
}

func guardDigits(n int) string {
	out := make([]string, 0, 3)
	for _, r := range fmt.Sprintf("%03d", n) {
		out = append(out, string(r))
	}
	return strings.Join(out, " ")
}

// handle is one pilot call on Guard: a hail, or an order to the aircraft hailed.
func (h *guardHail) handle(text, cs string, now time.Time) guardResult {
	if cs == "" || h.store == nil {
		return guardResult{}
	}
	text = collapseDigits(text)
	lower := strings.ToLower(text)
	if isDamagedCall(lower) {
		return guardResult{}
	}
	// An order first: it needs a hail, and a bare "heading 270" is no hail.
	if kind := orderKind(lower); kind != "" {
		return h.order(kind, text, lower, cs, now)
	}
	if !guardHailRe.MatchString(lower) {
		return guardResult{}
	}
	me, ok := playerContact(h.store, cs)
	if !ok {
		return guardResult{Guard: fmt.Sprintf("%s, Guard, no radar contact on you, say position.", cs)}
	}
	magvar := nearestFieldMagVar(me.Pos)
	bearing := -1.0
	if m := hailBearingRe.FindStringSubmatch(lower); m != nil {
		bearing, _ = strconv.ParseFloat(m[1], 64)
	}
	tgt, ok := pickTarget(h.store.Contacts(time.Minute), me.Pos, bearing, magvar, now)
	if !ok {
		return guardResult{Guard: fmt.Sprintf("%s, Guard, no radar contact on that aircraft.", cs)}
	}
	h.mu.Lock()
	h.by[strings.ToLower(cs)] = hailTarget{Target: tgt, Pilot: me, At: now}
	h.mu.Unlock()
	nato, doing := aiIdentity(tgt.Name)
	log.Info().Str("callsign", cs).Str("target", tgt.Name).Str("group", tgt.Group).Msg("Guard: hail")
	return guardResult{AI: fmt.Sprintf("Aircraft calling, this is Russian Air Force %s, %s. Go ahead.", nato, doing)}
}

func orderKind(lower string) string {
	switch {
	case orderFollowRe.MatchString(lower):
		return "follow"
	case orderLandRe.MatchString(lower):
		return "land"
	case orderLeaveRe.MatchString(lower):
		return "leave"
	case orderTurnRe.MatchString(lower):
		return "turn"
	}
	return ""
}

func (h *guardHail) order(kind, text, lower, cs string, now time.Time) guardResult {
	h.mu.Lock()
	ht, ok := h.by[strings.ToLower(cs)]
	h.mu.Unlock()
	if !ok || now.Sub(ht.At) > hailContextFor {
		return guardResult{Guard: fmt.Sprintf("%s, Guard, you have not hailed an aircraft. Say bearing and identify.", cs)}
	}
	// Where the aircraft is now; gone means shot down or out of range.
	var cur *tacviewContact
	for _, c := range h.store.Contacts(time.Minute) {
		if c.ID == ht.Target.ID {
			c := c
			cur = &c
			break
		}
	}
	if cur == nil {
		return guardResult{Guard: fmt.Sprintf("%s, Guard, lost radar contact on the aircraft.", cs)}
	}
	if cur.Group == "" {
		return guardResult{AI: "Unable, say again."}
	}
	magvar := nearestFieldMagVar(cur.Pos)
	fail := "Unable to comply."
	switch kind {
	case "follow":
		if ht.Pilot.Group == "" {
			return guardResult{AI: "Unable, say again."}
		}
		return guardResult{AI: "Wilco, following you.", Fail: fail,
			Do: func(ctx context.Context) error { return h.ai.Follow(ctx, cur.Group, ht.Pilot.Group) }}
	case "land":
		m := orderLandRe.FindStringSubmatch(lower)
		var f *airfield.Airfield
		if m != nil {
			f = fieldInText(m[1])
		}
		if f == nil {
			return guardResult{AI: "Say again, which field?"}
		}
		return guardResult{AI: fmt.Sprintf("Wilco, landing at %s.", f.Name), Fail: fail,
			Do: func(ctx context.Context) error { return h.ai.Land(ctx, cur.Group, f.DCSName) }}
	case "leave":
		// Away from the pilot.
		hdg := initialBearing(ht.Pilot.Pos, cur.Pos)
		return guardResult{AI: "Wilco, leaving the area.", Fail: fail,
			Do: func(ctx context.Context) error { return h.ai.Turn(ctx, cur.Group, hdg, orderRunNm) }}
	}
	m := orderTurnRe.FindStringSubmatch(text)
	if m == nil {
		return guardResult{AI: "Say again."}
	}
	mag, _ := strconv.Atoi(m[3])
	if mag < 0 || mag > 360 {
		return guardResult{AI: "Say again the heading."}
	}
	dir := m[1]
	if dir == "" {
		dir = m[2]
	}
	say := fmt.Sprintf("Wilco, heading %s.", guardDigits(mag))
	if dir != "" {
		say = fmt.Sprintf("Wilco, turning %s heading %s.", strings.ToLower(dir), guardDigits(mag))
	}
	hdgTrue := math.Mod(float64(mag)+magvar+360, 360)
	return guardResult{AI: say, Fail: fail,
		Do: func(ctx context.Context) error { return h.ai.Turn(ctx, cur.Group, hdgTrue, orderRunNm) }}
}
