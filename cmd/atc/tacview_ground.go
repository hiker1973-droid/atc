package main

import (
	"strings"
	"time"

	"github.com/paulmach/orb"
)

// Ground objects and weapon impacts in the Command Tacview store, for the
// JTAC's BDA (jtac_bda.go).
//
// How DCS's real-time Tacview export reports a static or unit dying was not
// seen on a live stream when this was written, so every way it can is taken
// as a kill:
//   - the object is removed ("-id")
//   - a global "0,Event=Destroyed|id|" event
//   - the object stays but its Name turns into a wreck ("...wreck", "dead")
// A kill is kept until a new mission starts (ReferenceTime changes), so a
// reconnect that resends a wreck as a live object does not bring it back. If
// statics never appear in the stream at all, the BDA says it cannot assess
// the damage and reports impacts only.

// tacviewGround is a ground object as last seen.
type tacviewGround struct {
	ID    string
	Pos   orb.Point
	Name  string
	Alive bool
}

// tacviewImpact is where a weapon ended: its last position before Tacview
// dropped it. Shooter is the store key of the jet it came off, "" if unknown.
type tacviewImpact struct {
	Pos     orb.Point
	At      time.Time
	Shooter string
}

const impactMemory = 15 * time.Minute

// isWreckName reports whether a Name reads as a destroyed object.
func isWreckName(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "wreck") || strings.Contains(n, "dead") || strings.Contains(n, "destroyed")
}

// isGroundType reports whether an ACMI Type is a ground object: units
// ("Ground+Heavy+Armor+Vehicle") and statics ("Ground+Static+Building").
func isGroundType(typ string) bool {
	return strings.Contains(typ, "Ground") || strings.Contains(typ, "Building") || strings.Contains(typ, "Static")
}

// SetGround records a ground object's position. A kill already recorded stays.
func (t *tacviewPositions) SetGround(id string, pos orb.Point, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ground == nil {
		t.ground = make(map[string]*tacviewGround)
	}
	g := t.ground[id]
	if g == nil {
		g = &tacviewGround{ID: id, Alive: true}
		t.ground[id] = g
	}
	g.Pos, g.Name = pos, name
	if isWreckName(name) {
		g.Alive = false
	}
}

// GroundDead marks a ground object destroyed.
func (t *tacviewPositions) GroundDead(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if g := t.ground[id]; g != nil {
		g.Alive = false
	}
}

// GroundObjects returns a copy of every ground object seen this mission.
func (t *tacviewPositions) GroundObjects() []tacviewGround {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]tacviewGround, 0, len(t.ground))
	for _, g := range t.ground {
		out = append(out, *g)
	}
	return out
}

// AddImpact records where a weapon ended.
func (t *tacviewPositions) AddImpact(imp tacviewImpact) {
	t.mu.Lock()
	defer t.mu.Unlock()
	keep := t.impacts[:0]
	for _, i := range t.impacts {
		if imp.At.Sub(i.At) < impactMemory {
			keep = append(keep, i)
		}
	}
	t.impacts = append(keep, imp)
}

// Impacts returns the impacts at or after since.
func (t *tacviewPositions) Impacts(since time.Time) []tacviewImpact {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []tacviewImpact
	for _, i := range t.impacts {
		if !i.At.Before(since) {
			out = append(out, i)
		}
	}
	return out
}

// CallsignByNumber is the one contact whose callsign ends in the flight number
// a pilot gave alone ("302" -> "Raider 302"); "" when none or more than one.
func (t *tacviewPositions) CallsignByNumber(num string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	found := ""
	for k := range t.pos {
		if strings.HasSuffix(k, " "+num) {
			if found != "" && found != k {
				return ""
			}
			found = k
		}
	}
	return found
}

// NearestPilot is the store key of the contact nearest pos within radiusNm,
// "" if none. It names the jet a weapon came off when Tacview gives no Parent.
func (t *tacviewPositions) NearestPilot(pos orb.Point, radiusNm float64) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	best, bestNm := "", radiusNm
	for k, p := range t.pos {
		if d := haversineNm(pos, p); d <= bestNm {
			best, bestNm = k, d
		}
	}
	return best
}

// NewMission clears the ground picture and impacts when the stream's
// ReferenceTime changes - another mission, or this one restarted.
func (t *tacviewPositions) NewMission(ref string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ref == t.missionRef {
		return
	}
	if t.missionRef != "" {
		t.ground = make(map[string]*tacviewGround)
		t.impacts = nil
	}
	t.missionRef = ref
}
