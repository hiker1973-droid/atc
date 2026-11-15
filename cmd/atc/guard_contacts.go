package main

import (
	"strings"
	"time"

	"github.com/paulmach/orb"
)

// Every aircraft and ground unit Tacview reports, by object id, for Guard's
// warnings and intercepts (guard_threats.go, guard_hail.go). The callsign-keyed
// maps beside it cannot hold them: every AI Su-27 shares the key "Su-27", and
// ground units carry no side.

// tacviewContact is one object as last seen.
type tacviewContact struct {
	ID     string
	Pos    orb.Point
	AltFt  float64
	Name   string // ACMI Name=, the model: "Su-27", "Kub 2P25 ln"
	Type   string // ACMI Type=, "Air+FixedWing", "Ground+Vehicle"...
	Color  string // "Blue" / "Red": the side
	Pilot  string // "Raider 032 |Jedi" for a player, the unit name for AI
	Group  string // the DCS group name
	HdgDeg float64
	Seen   time.Time
}

// IsAir is an aircraft (fixed wing or helicopter).
// The Type is a "+"-joined list ("Air+FixedWing", "Ground+AntiAircraft+Vehicle"):
// match the whole token, not the letters.
func (c tacviewContact) IsAir() bool {
	for _, tok := range strings.Split(c.Type, "+") {
		if tok == "Air" {
			return true
		}
	}
	return false
}

// IsPlayer is a human-flown jet: Tacview writes "Raider 032 |Jedi".
func (c tacviewContact) IsPlayer() bool { return c.IsAir() && strings.Contains(c.Pilot, "|") }

// Callsign is the callsign half of a player's Pilot field.
func (c tacviewContact) Callsign() string {
	cs, _, _ := strings.Cut(c.Pilot, "|")
	return strings.TrimSpace(cs)
}

func (c tacviewContact) IsRed() bool  { return strings.EqualFold(c.Color, "Red") }
func (c tacviewContact) IsBlue() bool { return strings.EqualFold(c.Color, "Blue") }

func newContact(id string, o *miniObject, pt orb.Point) *tacviewContact {
	return &tacviewContact{ID: id, Pos: pt, AltFt: o.altFt, Name: o.name, Type: o.typ, Color: o.color,
		Pilot: o.pilot, Group: o.group, HdgDeg: o.hdg, Seen: time.Now()}
}

// SetContact records or refreshes an object. A wreck is dropped.
func (t *tacviewPositions) SetContact(c *tacviewContact) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.contacts == nil {
		t.contacts = make(map[string]*tacviewContact)
	}
	if isWreckName(c.Name) {
		delete(t.contacts, c.ID)
		return
	}
	if old := t.contacts[c.ID]; old != nil {
		// Tacview sends only what changed; keep the descriptive fields.
		if c.Name == "" {
			c.Name = old.Name
		}
		if c.Type == "" {
			c.Type = old.Type
		}
		if c.Color == "" {
			c.Color = old.Color
		}
		if c.Pilot == "" {
			c.Pilot = old.Pilot
		}
		if c.Group == "" {
			c.Group = old.Group
		}
	}
	t.contacts[c.ID] = c
}

// RemoveContact drops an object Tacview stopped reporting.
func (t *tacviewPositions) RemoveContact(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.contacts, id)
}

// Contacts is a copy of every contact seen within maxAge (0 = all).
func (t *tacviewPositions) Contacts(maxAge time.Duration) []tacviewContact {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]tacviewContact, 0, len(t.contacts))
	now := time.Now()
	for _, c := range t.contacts {
		if maxAge > 0 && now.Sub(c.Seen) > maxAge {
			continue
		}
		out = append(out, *c)
	}
	return out
}
