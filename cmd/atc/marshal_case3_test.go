package main

import (
	"strings"
	"testing"
	"time"
)

// The squadron's DimOn Case III cards (26-27): the pilot's readback of the
// marshal, and the approach calls after commencing.
func TestMarshalCase3Calls(t *testing.T) {
	cases := []struct {
		text   string
		intent marshalIntent
	}{
		{"Marshal, 319, marshal on the 263, 21 DME, angels 6, expected approach time 23", marshalReadback},
		{"Marshal, Raider 319, 263 radial, 21 DME, angels 6, commence at 23", marshalReadback},
		{"Marshal, 319, established angels 6, state 9.8", marshalEstablished},
		{"Marshal, 319, commencing, altimeter 30.16, state 9.5", marshalCommencing},
		{"Marshal, 319, checking in, 15 miles", marshalDME},
		{"Marshal, 319, platform", marshalPlatform},
		{"Marshal, 319, needles up and left", marshalNeedles},
		{"Marshal, 319, negative needles", marshalNeedles},
		{"Marshal, 319, coupled", marshalACLSReport},
		{"Marshal, 319, command control", marshalACLSReport},
	}
	for _, tc := range cases {
		lower := strings.ToLower(tc.text)
		if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != tc.intent {
			t.Errorf("%q: intent %d, want %d", tc.text, got, tc.intent)
		}
	}
}

// After commencing, the card's calls come callsign-first; from a jet Marshal
// is bringing in they are its, from anyone else (or our echo) they are not.
func TestCase3ApproachCallsWithoutAddress(t *testing.T) {
	now := time.Now()
	case3Approach.Lock()
	case3Approach.jets = map[string]case3Jet{}
	case3Approach.Unlock()
	trackCase3Approach("Raider 319", "Raider 319", now)

	for text, want := range map[string]string{
		"319, checking in, 15 miles": "Raider 319",
		"Raider 319, platform":       "Raider 319",
		"319, needles up and on":     "Raider 319",
		"319, negative needles":      "Raider 319",
		"312, platform":              "", // not commenced with us
		"Raider 319, Union Marshal, three miles, contact paddles, call the ball.": "", // our echo
		"319, request taxi": "", // not an approach call
	} {
		if got := unaddressedApproachCallsign(text, now); got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
	// A commenced jet stops counting once it is long gone.
	if got := unaddressedApproachCallsign("319, platform", now.Add(caseThreeApproachFor+time.Minute)); got != "" {
		t.Errorf("stale approach jet still matched: %q", got)
	}
}
