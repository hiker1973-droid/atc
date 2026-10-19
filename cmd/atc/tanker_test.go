package main

import (
	"strings"
	"testing"
	"time"
)

func testTanker() *tankerNet {
	tn := newTankerNet("Texaco one one", func() int { return 13 }, 250)
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	tn.now = func() time.Time { return at }
	return tn
}

func TestTankerCallsign(t *testing.T) {
	for in, want := range map[string]string{
		"Texaco 1-1, Raider 302, one Hornet, 15 miles, intent to refuel.": "Raider 302",
		"Texaco 11 Raider 302 request 5.0":                                "Raider 302",
		"Texaco one one, Raider 3-1-2, one mile trail.":                   "Raider 312",
		"Raider 302, established.":                                        "Raider 302",
		"Texaco, Raider 302 refuel complete.":                             "Raider 302",
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
	} {
		if got := tankerAmount(in); got != want {
			t.Errorf("%q: %d, want %d", in, got, want)
		}
	}
}

// The operator's flow, call by call.
func TestTankerFlow(t *testing.T) {
	tn := testTanker()
	steps := []struct{ call, want string }{
		{"Texaco 1-1, Raider 302, one Hornet, 15 miles, intent to refuel.",
			"Raider 302, Texaco one one, angels one three, two five zero knots, how much do you request?"},
		{"Texaco, Raider 302, request 5.0.", "Raider 302, Texaco copies five thousand pounds."},
		{"Raider 302, one mile trail.", "Raider 302, Texaco, form on my left."},
		{"Raider 302, established.", "Raider 302, Texaco, copy, you are cleared port side, pre-contact."},
		{"Texaco, Raider 302, refuel complete.", "Raider 302, Texaco, copy, cleared to depart. Good hunting."},
	}
	for _, s := range steps {
		if got := tn.Response(s.call, tankerCallsign(s.call)); got != s.want {
			t.Errorf("%q:\n got %q\nwant %q", s.call, got, s.want)
		}
	}
	if len(tn.flights) != 0 {
		t.Errorf("flight not cleared after departing: %v", tn.flights)
	}
}

// Two flights on the tanker get a drogue each.
func TestTankerSides(t *testing.T) {
	tn := testTanker()
	for _, cs := range []string{"Raider 302", "Raider 312"} {
		tn.Response("Texaco 1-1, "+cs+", intent to refuel, request 4.0", cs)
	}
	if got := tn.Response("Raider 302, one mile trail", "Raider 302"); !strings.Contains(got, "my left") {
		t.Errorf("first: %q", got)
	}
	if got := tn.Response("Raider 312, one mile trail", "Raider 312"); !strings.Contains(got, "my right") {
		t.Errorf("second: %q", got)
	}
	if got := tn.Response("Raider 312, established", "Raider 312"); !strings.Contains(got, "starboard side") {
		t.Errorf("second cleared: %q", got)
	}
}

// Check-in with the give in one call, and an unknown station.
func TestTankerShortcuts(t *testing.T) {
	tn := testTanker()
	got := tn.Response("Texaco 1-1, Raider 302, intent to refuel, request 6.0", "Raider 302")
	if !strings.HasSuffix(got, "copies six thousand pounds.") {
		t.Errorf("combined: %q", got)
	}
	if got := tn.Response("Texaco 1-1, say again", ""); !strings.Contains(got, "say again your callsign") {
		t.Errorf("no callsign: %q", got)
	}
}
