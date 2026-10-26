package main

import (
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/lso"
	"github.com/vsfg7/atc/pkg/state"
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
		// Night 2026-10-25 18:05:09, an intent miss.
		{"Union Marshal Raider 319 requesting push time.", marshalPushTime},
		{"Marshal, 319, say push time", marshalPushTime},
		{"Marshal, 319, request approach time", marshalPushTime},
		// A readback with the time in it is still a readback.
		{"Marshal, 319, marshalling on 264, 21 DME, angels 6, push time 01", marshalReadback},
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

// Approach times are a whole minute, a minute clear of every other jet's.
func TestCase3EATSpacing(t *testing.T) {
	stack := state.NewMarshalStack()
	base := time.Date(2026, 10, 25, 18, 0, 20, 0, time.UTC)
	stack.Enqueue("Raider 1", 5)
	stack.SetEAT("Raider 1", base.Add(10*time.Minute).Truncate(time.Minute).Add(time.Minute)) // :11
	stack.Enqueue("Raider 2", 5)
	stack.SetEAT("Raider 2", time.Date(2026, 10, 25, 18, 12, 40, 0, time.UTC)) // :12, old seconds
	stack.Enqueue("Raider 3", 5)
	if got := case3EAT(stack, "Raider 3", base.Add(10*time.Minute)); got.Minute() != 13 || got.Second() != 0 {
		t.Errorf("Raider 3 got %s, want 18:13:00", got.Format("15:04:05"))
	}
	// Its own time does not block it.
	if got := case3EAT(stack, "Raider 1", base.Add(10*time.Minute)); got.Minute() != 11 {
		t.Errorf("Raider 1 got %s, want :11", got.Format("15:04"))
	}
	// Late: a new time at least two minutes out, clear of the others.
	now := time.Date(2026, 10, 25, 18, 9, 30, 0, time.UTC)
	stack.SetEAT("Raider 3", now.Add(-3*time.Minute))
	if got := case3HoldEAT(stack, "Raider 3", 3, now.Add(-3*time.Minute), now); got.Minute() != 13 {
		t.Errorf("late Raider 3 got %s, want :13", got.Format("15:04"))
	}
	// On time: kept.
	keep := time.Date(2026, 10, 25, 18, 20, 0, 0, time.UTC)
	if got := case3HoldEAT(stack, "Raider 3", 3, keep, now); !got.Equal(keep) {
		t.Errorf("on-time EAT changed to %s", got.Format("15:04"))
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
	if lsoCheckInRe.MatchString(lsoCheckInReply("Raider 319")) || lsoCheckInRe.MatchString(lsoAbeamReply("Raider 319")) {
		t.Error("Paddles' own reply reads as a check-in")
	}
}
