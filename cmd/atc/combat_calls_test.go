package main

import (
	"strings"
	"testing"

	"github.com/paulmach/orb"
)

// Operator 2026-10-21: the AWACS copies a Fox call, and "302 ejecting" (or
// another pilot's "302 is down") gets "302 down, launching CSAR". Inputs
// are Day 2026-10-21 16:41-16:43 transcripts where there is one.
func TestGCICombatCalls(t *testing.T) {
	store := newTacviewPositions()
	store.Set("Raider 302", orb.Point{41.6, 42.0})
	for _, tc := range []struct{ in, kind, want string }{
		{"302, Fox 3, eight for sixty-eight, twenty-four thousand, MiG-29.", "fox", "RAIDER 3 0 2, DARKSTAR, copy Fox three."},
		{"302 Fox 311 for 56, 21,000.", "fox", "RAIDER 3 0 2, DARKSTAR, copy Fox three."},
		{"Darkstar, Raider 302, fox two.", "fox", "RAIDER 3 0 2, DARKSTAR, copy Fox two."},
		{"302 splash.", "splash", "RAIDER 3 0 2, DARKSTAR, copy splash."},
		// Foothold 2026-10-21 17:41:42: no callsign at all.
		{"Splash one.", "splash", "DARKSTAR, copy splash one."},
		{"302 ejecting.", "down", "RAIDER 3 0 2 down, launching CSAR."},
		{"302, eject, eject, eject!", "down", "RAIDER 3 0 2 down, launching CSAR."},
		{"Darkstar, Raider 301, 302 is down.", "down", "RAIDER 3 0 2 down, launching CSAR."},
		{"Venom 212 is down, Venom 212 is down.", "down", "VENOM 2 1 2 down, launching CSAR."},
		// Not shot or down calls.
		{"302 defending.", "", ""},
		{"302 down to angels 5.", "", ""},
		{"302, gear down.", "", ""},
		{"Darkstar, Raider 302, request picture.", "", ""},
	} {
		kind, got := gciCombatCall(tc.in, "Darkstar", store.CallsignByNumber)
		if kind != tc.kind || got != tc.want {
			t.Errorf("%q:\n got %q %q\nwant %q %q", tc.in, kind, got, tc.kind, tc.want)
		}
	}
}

func TestCommandCombatCalls(t *testing.T) {
	store := newTacviewPositions()
	store.Set("Raider 302", orb.Point{41.6, 42.0})
	for in, want := range map[string]string{
		"302 ejecting.":                     "Raider 302 down, launching CSAR.",
		"Command, Raider 301, 302 is down.": "Raider 302 down, launching CSAR.",
	} {
		cs := joinCallsignDigits(extractCallsignSkippingAddress(in, commandAddresses...))
		if got := commandReply(in, cs, "vSFG-7-Command", store); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	// Day 2026-10-21 16:43:07: "Fox 3, 11 for 56" heard as "Fox 311" was a miss.
	in := "302 Fox 311 for 56, 21,000."
	cs := joinCallsignDigits(extractCallsignSkippingAddress(in, commandAddresses...))
	if got := commandReply(in, cs, "vSFG-7-Command", store); got == "" || !strings.Contains(got, "Fox three") {
		t.Errorf("%q: %q, want a Fox three copy", in, got)
	}
}

// Foothold 2026-10-21 17:39, 17:47: Jedi called Overlord "Darkstar".
func TestGCIOwnName(t *testing.T) {
	if got := gciOwnName("Darkstar Raider 302 requests bogey dope.", "Overlord"); got != "Overlord Raider 302 requests bogey dope." {
		t.Errorf("got %q", got)
	}
	if got := gciOwnName("Overlord, Raider 302, picture.", "Overlord"); got != "Overlord, Raider 302, picture." {
		t.Errorf("own name changed: %q", got)
	}
}
