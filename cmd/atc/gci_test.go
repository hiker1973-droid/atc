package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"

	"github.com/martinlindhe/unit"

	"github.com/vsfg7/atc/pkg/gci/bearings"
	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/callsigns"
	"github.com/vsfg7/atc/pkg/gci/composer"
	"github.com/vsfg7/atc/pkg/gci/parser"
)

// What a pilot says on 264, as Whisper writes it, reaches the right handler.
func TestGCIParse(t *testing.T) {
	p := parser.New("Darkstar", nil, false)
	for text, want := range map[string]string{
		"Darkstar, Venom 2-1-2, radio check.":    "*brevity.RadioCheckRequest",
		"Darkstar, Raider 1-1, request picture.": "*brevity.PictureRequest",
		"Darkstar, Venom 212, bogey dope.":       "*brevity.BogeyDopeRequest",
		"Anyface, Raider 11, alpha check.":       "*brevity.AlphaCheckRequest",
	} {
		req := p.Parse(text)
		if got := typeName(req); got != want {
			t.Errorf("%q parsed as %s, want %s", text, got, want)
		}
	}
	if req := p.Parse("Senaki Tower, Raider 306, request taxi."); req != nil {
		t.Errorf("a tower call reached the GCI: %#v", req)
	}
}

func typeName(v any) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}

// Every call the controller can make has words (gciCompose covers SkyEye's composeCall).
func TestGCIComposeCovers(t *testing.T) {
	comp := composer.Composer{Callsign: "Darkstar"}
	g := &sampleGroup{contacts: 2, alt: 25000 * unit.Foot, track: brevity.West, aspect: brevity.Hot,
		decl: brevity.Hostile, platforms: []string{"Flanker"},
		braa: brevity.NewBRAA(bearings.NewMagneticBearing(45*unit.Degree), 30*unit.NauticalMile, []unit.Length{25000 * unit.Foot}, brevity.Hot)}
	for _, call := range []any{
		brevity.RadioCheckResponse{Callsign: "Venom 2 1 2", RadarContact: true},
		brevity.BogeyDopeResponse{Callsign: "Venom 2 1 2", Group: g},
		brevity.ThreatCall{Callsigns: []string{"Venom 2 1 2"}, Group: g},
		brevity.PictureResponse{Count: 1, Groups: []brevity.Group{g}},
	} {
		r, ok := gciCompose(comp, call)
		if !ok || r.Speech == "" {
			t.Errorf("%T: no speech", call)
		}
	}
	if _, ok := gciCompose(comp, struct{}{}); ok {
		t.Error("an unknown call was composed")
	}
}

// Operator 2026-10-21: no joke on "checking in".
func TestGCICheckInPlain(t *testing.T) {
	r, ok := gciCompose(composer.Composer{Callsign: "Darkstar"}, brevity.CheckInResponse{Callsign: "Raider 1 1"})
	if !ok || r.Speech != "RAIDER 1 1, DARKSTAR, radar contact." {
		t.Errorf("check in reply %q", r.Speech)
	}
}

// Operator 2026-10-21: "good copy" or "5 by 5" at random, not "Lima Charlie".
func TestGCIRadioCheckGoodCopy(t *testing.T) {
	comp := composer.Composer{Callsign: "Darkstar"}
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		r, _ := gciCompose(comp, brevity.RadioCheckResponse{Callsign: "Raider 3 0 2", RadarContact: true})
		seen[r.Speech] = true
	}
	if len(seen) != 2 || !seen["RAIDER 3 0 2, DARKSTAR, good copy."] || !seen["RAIDER 3 0 2, DARKSTAR, 5 by 5."] {
		t.Errorf("radio check replies: %v", seen)
	}
	r, _ := gciCompose(comp, brevity.RadioCheckResponse{Callsign: "Raider 3 0 2"})
	if !strings.HasSuffix(r.Speech, ", but I don't see you on scope.") || strings.Contains(r.Speech, "Lima") {
		t.Errorf("radio check off scope: %q", r.Speech)
	}
}

// Operator 2026-10-21: "just pay attention to the Raider/Venom XXX part" -
// names straight from DCSServerBot, and what Whisper writes.
func TestGCISquadronCallsigns(t *testing.T) {
	for in, want := range map[string]string{
		"vSFG-7 | Raider 302 | Jedi": "raider 3 0 2",
		"Raider 302| Jedi":           "raider 3 0 2",
		"Venom 212 | Dabs":           "venom 2 1 2",
		"[CLAN] Bob | Venom 212":     "venom 2 1 2",
		"RAIDER 031 | BACON":         "raider 0 3 1",
		"Raider 1-1 | Boss | 333":    "raider 1 1",
		"Raiders 306":                "raider 3 0 6",
		"Knight 11 | Someone":        "knight 1 1", // not a Raider/Venom name: SkyEye as before
	} {
		if got, ok := callsigns.ParsePilotCallsign(in); !ok || got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	p := parser.New("Darkstar", nil, false)
	if req, ok := p.Parse("Darkstar, Raiders 306, radio check.").(*brevity.RadioCheckRequest); !ok || req.Callsign != "raider 3 0 6" {
		t.Errorf("spoken call parsed as %#v", p.Parse("Darkstar, Raiders 306, radio check."))
	}
}

// Operator 2026-10-21: "fast movers" = bogey dope on jets, "slow movers" on helicopters.
func TestGCIMovers(t *testing.T) {
	p := parser.New("Darkstar", nil, false)
	for text, want := range map[string]brevity.ContactCategory{
		"Darkstar, Raider 302, any fast movers?":  brevity.FixedWing,
		"Darkstar, Venom 212, slow movers.":       brevity.RotaryWing,
		"Darkstar, Raider 302, fast-mover check.": brevity.FixedWing,
		"Darkstar, Raider 302, bogey dope":        brevity.Aircraft,
	} {
		req, ok := p.Parse(gciAliases(text)).(*brevity.BogeyDopeRequest)
		if !ok || req.Filter != want {
			t.Errorf("%q -> %#v, want bogey dope filter %v", text, p.Parse(gciAliases(text)), want)
		}
	}
}

// Operator 2026-10-21: "checking out" gets a reply, not "say again".
func TestGCICheckOut(t *testing.T) {
	for text, want := range map[string]string{
		"Darkstar, Raider 302, checking out.": "RAIDER 3 0 2, DARKSTAR, copy, good day.",
		"Anyface, Venom 212, check out, RTB.": "VENOM 2 1 2, DARKSTAR, copy, good day.",
		"Darkstar, Raider 302, picture.":      "",
		"Command, Raider 302, checking out.":  "", // not addressed to the GCI
	} {
		if got := gciCheckOut(text, "Darkstar"); got != want {
			t.Errorf("%q -> %q, want %q", text, got, want)
		}
	}
}

// Command points a fenced-in flight at the GCI.
func TestCommandGCIHint(t *testing.T) {
	defer func(n string, f float64) { flagCommandGCIName, flagCommandGCIFreq = n, f }(flagCommandGCIName, flagCommandGCIFreq)
	flagCommandGCIName, flagCommandGCIFreq = "", 0
	if r := commandResponse("Command, Raider 302, fence in.", "Raider 302", "vSFG-7-Command"); strings.Contains(r, "picture") {
		t.Errorf("hint without the flag: %q", r)
	}
	flagCommandGCIName, flagCommandGCIFreq = "Darkstar", 264.0
	r := commandResponse("Command, Raider 302, fence in.", "Raider 302", "vSFG-7-Command")
	if !strings.HasSuffix(r, " Darkstar is up on two six four point zero for picture.") {
		t.Errorf("fence in reply %q", r)
	}
}

// Quiet during recoveries: near the boat low, in a field's pattern, or not.
func TestGCIRecovery(t *testing.T) {
	store := newTacviewPositions()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	put := func(key string, lon, lat, alt float64) {
		store.Set(key, orb.Point{lon, lat})
		store.SetInfo(key, tacviewContactInfo{AltFt: alt, UpdatedAt: now})
	}
	put("CVN_72", 37.5, 42.8, 0)
	put("Raider 302", 37.55, 42.85, 2500)  // ~3.5 nm from the boat, angels 2.5: in the stack
	put("Raider 303", 37.55, 42.85, 22000) // same place, angels 22: transiting overhead
	put("Venom 212", 42.06, 42.25, 1500)   // Senaki pattern
	put("Venom 201", 42.06, 43.40, 1500)   // 70 nm north of Senaki, low: on the range
	r := newGCIRecovery(store, gciFields("caucasus"))
	r.now = func() time.Time { return now }
	for name, want := range map[string]string{
		"Raider 302 | Jedi":    "the carrier",
		"Raider 303 | Thrash":  "",
		"Venom 212 | Dabs":     "Senaki",
		"Venom 201 | Screwy":   "",
		"Unknown 999 | Nobody": "",
	} {
		if got := r.where(name); got != want {
			t.Errorf("%s: recovering at %q, want %q", name, got, want)
		}
	}
	if !r.anyRecovering([]string{"Raider 303 | Thrash", "Venom 212 | Dabs"}) || r.anyRecovering([]string{"Raider 303 | Thrash"}) {
		t.Error("anyRecovering")
	}
	// A track not updated for a minute (slot left, Tacview lost it) is not recovering.
	r.now = func() time.Time { return now.Add(time.Minute) }
	if r.where("Raider 302 | Jedi") != "" {
		t.Error("stale track still recovering")
	}
}
