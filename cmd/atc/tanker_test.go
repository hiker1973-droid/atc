package main

import (
	"strings"
	"testing"
	"time"
)

func testTanker() *tankerNet {
	tn := newTankerNet("Texaco one one", func() int { return 13 }, func() int { return 336 })
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	tn.now = func() time.Time { return at }
	tn.pick = func(o [3]string) string { return o[0] }
	return tn
}

// The three check-out send-offs (operator 2026-10-19, one "stay classy Raider").
func TestTankerCheckOutVariations(t *testing.T) {
	want := []string{
		"Raider 302, Texaco, copy, cleared to depart. Good hunting.",
		"Raider 302, Texaco, cleared to depart. Stay classy, Raider.",
		"Raider 302, Texaco, copy, cleared to depart. Go get 'em.",
	}
	for i, w := range want {
		tn := testTanker()
		tn.pick = func(o [3]string) string { return o[i] }
		call := "Texaco 3-1, Raider 302, checking out, pushing 282."
		if got := tn.Response(call, tankerCallsign(call)); got != w {
			t.Errorf("version %d: got %q, want %q", i, got, w)
		}
	}
}

func TestTankerCallsign(t *testing.T) {
	for in, want := range map[string]string{
		"Texaco 1-1, Raider 302, one Hornet, 15 miles, intent to refuel.":                               "Raider 302",
		"Texaco 11 Raider 302 request 5.0":                                                              "Raider 302",
		"Texaco one one, Raider 3-1-2, one mile trail.":                                                 "Raider 312",
		"Raider 302, established.":                                                                      "Raider 302",
		"Texaco, Raider 302 refuel complete.":                                                           "Raider 302",
		"Texaco 1, Knight 1-1, flight of 2, 15 mile initial for port observation with intent to refuel": "Knight 11",
		"Knight 1-3, moving to starboard basket":                                                        "Knight 13",
		"Knight 1-3 satisfied, moving to starboard reform":                                              "Knight 13",
		"Texaco 1, Knight 1-1, checking out, resuming navigation, pushing 282":                          "Knight 11",
		"Texaco 3-1, Raider 302, 15 mile initial, intent to refuel.":                                    "Raider 302",
		"Texaco three one, Raider 3-0-2, comm check.":                                                   "Raider 302",
		"Texaco 31 Raider 302 request 5.0":                                                              "Raider 302",
	} {
		if got := tankerCallsign(in); got != want {
			t.Errorf("%q: callsign %q, want %q", in, got, want)
		}
	}
}

func TestTankerAmount(t *testing.T) {
	for in, want := range map[string]int{
		"raider 302, request 5.0":                                5000,
		"raider 302 request five point oh":                       5000,
		"raider 302, 5000 pounds":                                5000,
		"raider 302 request 5 thousand":                          5000,
		"raider 302, request 2.5":                                2500,
		"raider 302, request full":                               -1,
		"raider 302, top off":                                    -1,
		"raider 302, one mile trail":                             0,
		"texaco 1-1, raider 302, 15 miles":                       0,
		"raider 302 one hornet 15 miles requesting 4,000 pounds": 4000,
		"texaco 1, knight 1-1, flight of 2, 15 mile initial for port observation with intent to refuel": 0,
	} {
		if got := tankerAmount(in); got != want {
			t.Errorf("%q: %d, want %d", in, got, want)
		}
	}
}

func runTanker(t *testing.T, tn *tankerNet, steps []struct{ call, want string }) {
	t.Helper()
	for _, s := range steps {
		if got := tn.Response(s.call, tankerCallsign(s.call)); got != s.want {
			t.Errorf("%q:\n got %q\nwant %q", s.call, got, s.want)
		}
	}
}

// The vSFG-7 AAR procedure's comms table, a flight of two, plus the
// operator's "how much do you request?".
func TestTankerProcedureFlow(t *testing.T) {
	tn := testTanker()
	runTanker(t, tn, []struct{ call, want string }{
		{"Texaco 1, Knight 1-1, flight of 2, 15 mile initial for port observation with intent to refuel",
			"Knight 11, Texaco one one, cleared port observation, one three thousand, velocity three three six. How much do you request?"},
		{"Knight 1-1, request 5.0", "Knight 11, Texaco copies five thousand pounds."},
		{"Texaco 1, Knight 1-1, flight of 2, 1 mile trail for port observation", "Knight 11, Texaco, continue port observation."},
		{"Texaco 1, Knight 1-1, established port observation", "Knight 11, Texaco, copy. Port and starboard baskets open."},
		{"Knight 1-1, moving to port basket", "Knight 11, Texaco, cleared port basket."},
		{"Knight 1-3, moving to starboard basket", "Knight 13, Texaco, cleared starboard basket."},
		{"Knight 1-3, established starboard basket", "Knight 13, Texaco, copy, call ready pre-contact."},
		{"Knight 1-3, satisfied, moving to starboard reform", "Knight 13, Texaco, cleared starboard reform."},
		{"Knight 1-3, established starboard reform", "Knight 13, Texaco, copy."},
		{"Knight 1-1, satisfied, moving to starboard reform", "Knight 11, Texaco, cleared starboard reform."},
		{"Knight 1-1, established starboard reform", "Knight 11, Texaco, copy."},
		{"Texaco 1, Knight 1-1, checking out, resuming navigation, pushing 282",
			"Knight 11, Texaco, copy, cleared to depart. Good hunting."},
	})
	if f := tn.flights["Knight 11"]; f != nil {
		t.Errorf("lead still on the tanker after checking out: %+v", f)
	}
}

// The operator's 16 Oct wording still works.
func TestTankerOperatorFlow(t *testing.T) {
	tn := testTanker()
	runTanker(t, tn, []struct{ call, want string }{
		{"Texaco 1-1, Raider 302, one Hornet, 15 miles, intent to refuel.",
			"Raider 302, Texaco one one, cleared port observation, one three thousand, velocity three three six. How much do you request?"},
		{"Texaco, Raider 302, request 5.0.", "Raider 302, Texaco copies five thousand pounds."},
		{"Raider 302, one mile trail.", "Raider 302, Texaco, continue port observation."},
		{"Raider 302, established.", "Raider 302, Texaco, copy. Port and starboard baskets open."},
		{"Raider 302, ready pre-contact.", "Raider 302, Texaco, copy, call ready pre-contact."},
		{"Texaco, Raider 302, refuel complete.", "Raider 302, Texaco, cleared starboard reform."},
		{"Texaco, Raider 302, departing.", "Raider 302, Texaco, copy, cleared to depart. Good hunting."},
	})
}

// One jet per basket: a taken basket gets a standby and the open one.
func TestTankerBaskets(t *testing.T) {
	tn := testTanker()
	for _, cs := range []string{"Raider 302", "Raider 312", "Raider 303"} {
		tn.Response("Texaco 1-1, "+cs+", intent to refuel, request 4.0", cs)
	}
	if got := tn.Response("Raider 302, moving to port basket", "Raider 302"); !strings.HasSuffix(got, "cleared port basket.") {
		t.Errorf("first: %q", got)
	}
	if got := tn.Response("Raider 312, moving to port basket", "Raider 312"); !strings.Contains(got, "standby, port basket occupied. Starboard basket open.") {
		t.Errorf("taken: %q", got)
	}
	if got := tn.Response("Raider 312, moving to the basket", "Raider 312"); !strings.HasSuffix(got, "cleared starboard basket.") {
		t.Errorf("open one: %q", got)
	}
	if got := tn.Response("Raider 303, established port observation", "Raider 303"); !strings.Contains(got, "both baskets occupied") {
		t.Errorf("both taken: %q", got)
	}
	tn.Response("Raider 302, satisfied, moving to starboard reform", "Raider 302")
	if got := tn.Response("Raider 303, moving to port basket", "Raider 303"); !strings.HasSuffix(got, "cleared port basket.") {
		t.Errorf("freed: %q", got)
	}
}

// Check-in with the give in one call, and an unknown station.
func TestTankerShortcuts(t *testing.T) {
	tn := testTanker()
	got := tn.Response("Texaco 1-1, Raider 302, intent to refuel, request 6.0", "Raider 302")
	if !strings.HasSuffix(got, "velocity three three six. Copies six thousand pounds.") {
		t.Errorf("combined: %q", got)
	}
	if got := tn.Response("Texaco 1-1, say again", ""); !strings.Contains(got, "say again your callsign") {
		t.Errorf("no callsign: %q", got)
	}
}

// Day 2026-10-19: a ramp comm check got "cleared port observation", and two
// mid-flow comm checks got nothing.
func TestTankerRadioCheck(t *testing.T) {
	tn := testTanker()
	runTanker(t, tn, []struct{ call, want string }{
		{"Texaco 1-1, Raider 3-0-2, com check.", "Raider 302, Texaco one one, loud and clear."},
		{"Texaco 1-1, Raider 302, 15 mile initial, intent to refuel",
			"Raider 302, Texaco one one, cleared port observation, one three thousand, velocity three three six. How much do you request?"},
		{"Texaco 1-1, Raider 302, request 5,000 pounds.", "Raider 302, Texaco copies five thousand pounds."},
		{"Texaco 11, Raider 302, comm check.", "Raider 302, Texaco one one, loud and clear."},
		{"Raider 302, 1 mile trail for port observation", "Raider 302, Texaco, continue port observation."},
	})
}
