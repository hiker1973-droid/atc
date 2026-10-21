package main

import (
	"fmt"
	"testing"

	"github.com/martinlindhe/unit"

	"github.com/vsfg7/atc/pkg/gci/bearings"
	"github.com/vsfg7/atc/pkg/gci/brevity"
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

// Operator 2026-10-21: "good copy", not "Lima Charlie".
func TestGCIRadioCheckGoodCopy(t *testing.T) {
	comp := composer.Composer{Callsign: "Darkstar"}
	r, _ := gciCompose(comp, brevity.RadioCheckResponse{Callsign: "Raider 3 0 2", RadarContact: true})
	if r.Speech != "RAIDER 3 0 2, DARKSTAR, good copy." {
		t.Errorf("radio check: %q", r.Speech)
	}
	r, _ = gciCompose(comp, brevity.RadioCheckResponse{Callsign: "Raider 3 0 2"})
	if r.Speech != "RAIDER 3 0 2, DARKSTAR, good copy, but I don't see you on scope." {
		t.Errorf("radio check off scope: %q", r.Speech)
	}
}
