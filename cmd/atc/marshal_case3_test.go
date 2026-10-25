package main

import (
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/lso"
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

// Night 2026-10-25, Raider 319's Case III: two calls that went unanswered.
func TestWilburCase3Calls(t *testing.T) {
	// "Union Marshal" heard as "You're in Marshall".
	if _, ok := splitMarshalAddress("You're in Marshall, Raider 319 platform."); !ok {
		t.Error(`"You're in Marshall" not taken as the Marshal address`)
	}
	// Our own TX still is not.
	if _, ok := splitMarshalAddress("Raider 319, Union Marshal, copy platform, final bearing zero eight four."); ok {
		t.Error("Marshal's own TX taken as addressed to it")
	}
	// "three-mile initial" on Paddles: a check-in, told to call the ball.
	f := newLSOFeed(lso.NimitzParams(), func(lso.Call) {})
	if cs, resp := f.heard("Raider 319, three-mile initial."); cs != "Raider 319" || resp != "Paddles contact, Raider 319, call the ball at three-quarters of a mile." {
		t.Errorf("three-mile initial: %q %q", cs, resp)
	}
	// The ball call is still the ball call, and Paddles' own words are not a check-in.
	if _, resp := f.heard("319, Hornet ball, 4.4."); resp != netHandled {
		t.Errorf("ball call: %q", resp)
	}
	if cs, _ := f.heard("Paddles contact, Raider 319, call the ball at three-quarters of a mile."); cs != "" && lsoCheckInRe.MatchString("call the ball at three-quarters of a mile") {
		t.Error("Paddles' own reply reads as a check-in")
	}
}
