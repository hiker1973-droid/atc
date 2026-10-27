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
		// The bolter pattern (Master Arms Case III page).
		{"Marshal, 319, bolter", marshalBolter},
		{"Marshal, 319, waved off, 1 mile", marshalBolter},
		{"Marshal, 319, wave off", marshalBolter},
		{"Marshal, 319, abeam", marshalAbeam},
		{"Marshal, 319, abeam, state 4.2", marshalAbeam},
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
	case3Approach.jets = map[string]*case3Jet{}
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
		"319, bolter":       "Raider 319",
		"319, abeam":        "Raider 319",
		"Raider 319, Union Marshal, roger abeam, four miles past abeam turn final, final bearing zero eight four.": "",
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

// Approach times are a whole minute, ten minutes clear of every other jet's.
func TestCase3EATSpacing(t *testing.T) {
	stack := state.NewMarshalStack()
	base := time.Date(2026, 10, 25, 18, 0, 20, 0, time.UTC)
	stack.Enqueue("Raider 1", 5)
	stack.SetEAT("Raider 1", base.Add(10*time.Minute).Truncate(time.Minute).Add(time.Minute)) // :11
	stack.Enqueue("Raider 2", 5)
	stack.SetEAT("Raider 2", time.Date(2026, 10, 25, 18, 21, 40, 0, time.UTC)) // :21, old seconds
	stack.Enqueue("Raider 3", 5)
	if got := case3EAT(stack, "Raider 3", base.Add(10*time.Minute)); got.Minute() != 31 || got.Second() != 0 {
		t.Errorf("Raider 3 got %s, want 18:31:00", got.Format("15:04:05"))
	}
	// Its own time does not block it.
	if got := case3EAT(stack, "Raider 1", base.Add(10*time.Minute)); got.Minute() != 11 {
		t.Errorf("Raider 1 got %s, want :11", got.Format("15:04"))
	}
	// Late: a new time at least two minutes out, ten clear of the others.
	now := time.Date(2026, 10, 25, 18, 9, 30, 0, time.UTC)
	stack.SetEAT("Raider 3", now.Add(-3*time.Minute))
	if got := case3HoldEAT(stack, "Raider 3", 3, now.Add(-3*time.Minute), now); got.Minute() != 31 {
		t.Errorf("late Raider 3 got %s, want :31", got.Format("15:04"))
	}
	// Ten minutes past the last jet is clear.
	if got := case3EAT(stack, "Raider 3", time.Date(2026, 10, 25, 18, 40, 0, 0, time.UTC)); got.Minute() != 40 {
		t.Errorf("clear slot got %s, want :40", got.Format("15:04"))
	}
	// On time: kept.
	keep := time.Date(2026, 10, 25, 18, 50, 0, 0, time.UTC)
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

// One jet round the Case III approach on the Tacview picture: dirty up at 8,
// Paddles at 3, a bolter climbing away ahead, Paddles again astern at 3, then
// the trap. Final lies on 264 true from the ship.
func TestCase3ApproachPattern(t *testing.T) {
	now := time.Now()
	j := &case3Jet{say: "Raider 319", since: now, astern: -1}
	steps := []struct {
		angels, dist, brg int
		want              case3Say
	}{
		{5, 18, 264, case3SayNothing},
		{2, 8, 264, case3SayDirty},
		{1, 6, 264, case3SayNothing},
		{1, 3, 264, case3SayPaddles},
		{0, 1, 264, case3SayNothing},
		{0, 0, 264, case3SayNothing}, // touched down, went round
		{1, 1, 84, case3SayNothing},
		{1, 2, 84, case3SayBolter},
		{1, 4, 84, case3SayNothing},
		{1, 2, 174, case3SayNothing}, // abeam on the downwind, inside 3 miles
		{1, 4, 250, case3SayNothing},
		{1, 3, 262, case3SayPaddles},
		{0, 1, 264, case3SayNothing},
		{0, 0, 264, case3SayNothing},
		{0, 0, 264, case3SayNothing},
		{0, 0, 264, case3SayNothing},
		{0, 0, 264, case3Trapped},
	}
	for i, s := range steps {
		if got := j.step(s.angels, s.dist, s.brg, now); got != s.want {
			t.Fatalf("step %d (%d nm, %03d): %d, want %d", i, s.dist, s.brg, got, s.want)
		}
	}
	if got := case3AsternTrue(93); got != 264 {
		t.Errorf("astern for BRC 093: %d, want 264", got)
	}
}

// A jet that calls its own bolter is told the pattern once; the radar seeing
// it a moment later says nothing more, and a second call gets an ack.
func TestCase3BolterCalled(t *testing.T) {
	now := time.Now()
	case3Approach.Lock()
	case3Approach.jets = map[string]*case3Jet{}
	case3Approach.Unlock()
	trackCase3Approach("Raider 319", "Raider 319", now)
	if case3CalledPattern("Raider 319", "Raider 319", 264, now) {
		t.Error("first bolter call already told")
	}
	if !case3CalledPattern("Raider 319", "Raider 319", 264, now.Add(10*time.Second)) {
		t.Error("second bolter call not seen as told")
	}
	case3Approach.Lock()
	j := case3Approach.jets["Raider 319"]
	case3Approach.Unlock()
	if j.phase != case3Pattern || j.astern != 264 {
		t.Errorf("phase %d astern %d", j.phase, j.astern)
	}
	if got := j.step(1, 2, 84, now); got != case3SayNothing {
		t.Errorf("radar repeated the bolter pattern: %d", got)
	}
}
