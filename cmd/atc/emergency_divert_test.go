package main

import (
	"strings"
	"testing"

	"github.com/paulmach/orb"
	"github.com/vsfg7/atc/pkg/airfield"
)

// Operator 2026-10-21: a hit / damaged jet is told the nearest field, its
// bearing and range, on the AWACS or Command net.
func TestDamagedCallWords(t *testing.T) {
	for in, want := range map[string]bool{
		"darkstar, raider 302, i'm hit.":                        true,
		"command, raider 302, battle damage, rtb.":              true,
		"302, took a hit, left engine fire.":                    true,
		"mayday mayday, raider 302.":                            true,
		"command, raider 302, request nearest field.":           true,
		"raider 302, lost my right engine.":                     true,
		"hitman, raider 302, request bda.":                      false,
		"command, raider 302, good hits on the target.":         false,
		"command, raider 302, target is hit.":                   false,
		"command, raider 302, target damaged.":                  false,
		"darkstar, raider 302, request picture.":                false,
		"hitman, raider 302, request battle damage assessment.": false,
	} {
		if got := isDamagedCall(in); got != want {
			t.Errorf("%q: damaged %v, want %v", in, got, want)
		}
	}
}

func TestDivertReply(t *testing.T) {
	east := &airfield.Airfield{Name: "Eastfield", Center: orb.Point{43.0, 42.0}, MagVar: 7, ElevationFt: 150, TowerFreqMHz: 262}
	west := &airfield.Airfield{Name: "Westfield", Center: orb.Point{41.0, 42.0}, MagVar: 7, ElevationFt: 59, TowerFreqMHz: 260}
	fields := []*airfield.Airfield{east, west}
	// A jet 18 nm west of Eastfield, much further from Westfield.
	got := divertReply("RAIDER 3 0 2", "DARKSTAR", orb.Point{42.6, 42.0}, true, fields)
	for _, want := range []string{"RAIDER 3 0 2, DARKSTAR, copy.", "Nearest field Eastfield", "bearing zero-eight-three", "18 miles",
		"elevation 150", "Eastfield tower two six two point zero"} {
		if !strings.Contains(got, want) {
			t.Errorf("divert reply %q lacks %q", got, want)
		}
	}
	if got := divertReply("Raider 302", "vSFG-7-Command", orb.Point{}, false, fields); !strings.Contains(got, "say position") {
		t.Errorf("no position: %q", got)
	}

	// Through Command, with the jet on Tacview.
	store := newTacviewPositions()
	store.Set("Raider 302", orb.Point{42.6, 42.0})
	in := "Command, Raider 302, I'm hit, request nearest field."
	cs := joinCallsignDigits(extractCallsignSkippingAddress(in, commandAddresses...))
	if got := commandReply(in, cs, "vSFG-7-Command", store); !strings.HasPrefix(got, "Raider 302, vSFG-7-Command, copy. Nearest field ") {
		t.Errorf("Command %q: %q", in, got)
	}
}
