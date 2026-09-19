package main

import (
	"strings"
	"testing"
)

// Every call below is a verbatim Whisper transcript from the Syria Marshal
// (foothold, 306.1) on 2026-09-14, Raider 331. The "was" note is what Marshal
// did with it before this fix.
func TestMarshalCallsFlown20260914(t *testing.T) {
	cases := []struct {
		text     string
		callsign string
		intent   marshalIntent
		was      string
	}{
		{"Marshall, Raider 331 checking in.", "Raider 331", marshalCheckIn, "silence — no check-in intent"},
		{"Marshal, Raider 331, clear mothers.", "Raider 331", marshalDepartureClear, "silence — needed 'clear of'"},
		{"Union Marshal, Raider 331, marking moms 350, for 43, angels 16, state 3.5.", "Raider 331", marshalMarkingMom, "ok"},
		{"Here Marshall Raider 331, request BRC.", "Raider 331", marshalBRCRequest, "dropped as self-echo"},
		{"Leader, Marshal, Raider 331, marking mom's 360 for 35, state 3.3, angels 17.", "Raider 331", marshalMarkingMom, "dropped as self-echo"},
		{"Unit Marshall, Raider 331, marking moms 360 for 31, angels 17, state 3.3.", "Raider 331", marshalMarkingMom, "callsign 'Unit Marshall', phantom 2nd stack slot"},
		{"Marshal Raider 331, see you at ten.", "Raider 331", marshalSeeYouAtTen, "ok"},
		{"Marshal Raider 331, establish angels 2.", "Raider 331", marshalEstablished, "silence — needed 'established angels'"},
		{"Unit Marshall, Raider 331 established, angels two.", "Raider 331", marshalEstablished, "silence — comma broke the match"},
		{"Marshal, Raider 331 established angels 2.", "Raider 331", marshalEstablished, "ok"},
		{"Union Marshall, Raider 331, establish angels 2.", "Raider 331", marshalEstablished, "silence"},
		{"Union Marshal, Raider 331, commencing.", "Raider 331", marshalCommencing, "ok"},
		{"Marshal, Raider 331, initial.", "Raider 331", marshalInitial, "ok"},
		{"Marshal Raider 331, checking in.", "Raider 331", marshalCheckIn, "silence"},
		{"Marshall Raider 331, 7 DME.", "Raider 331", marshalDME, "ok"},
		{"Union Marshall, Raider 331, clear Mother's push command.", "Raider 331", marshalDepartureClear, "only a push-command ack"},
		{"Unit Marshall, Raider 331, marking moms 040, angels 33 for 52.", "Raider 331", marshalMarkingMom, "callsign 'Unit Marshall', no radar contact"},
		{"Marshal, see you at ten. Raider 331, see you at ten.", "Raider 331", marshalSeeYouAtTen, "TX opened 'see you at ten. Raider 331'"},
		{"Marshal, Raider 331, angels 2, see you at 10.", "Raider 331", marshalSeeYouAtTen, "ok"},
		{"Marshall, Raider 331, state 2.5, angels 2.", "Raider 331", marshalState, "ok"},
		{"Marshal Raider 331 commencing on this go-round.", "Raider 331", marshalCommencing, "ok"},
		{"In here, Marshall, Raider 331 initial.", "Raider 331", marshalInitial, "dropped as self-echo"},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if _, ok := splitMarshalAddress(tc.text); !ok {
				t.Fatalf("not recognised as addressed to Marshal (was: %s)", tc.was)
			}
			if cs := marshalCallsignFromText(tc.text); cs != tc.callsign {
				t.Errorf("callsign = %q, want %q (was: %s)", cs, tc.callsign, tc.was)
			}
			lower := strings.ToLower(tc.text)
			if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != tc.intent {
				t.Errorf("intent = %d, want %d (was: %s)", got, tc.intent, tc.was)
			}
		})
	}
}

// Marshal's own transmissions come back through SRS and get transcribed. They
// open with the callsign, so the address guard must keep rejecting them — the
// same night's echoes, verbatim.
func TestMarshalEchoesStayRejected(t *testing.T) {
	echoes := []string{
		"Raider 331, Union Marshall, radar contact, angels 16, range 43 miles, bearing 006 from mother. Mother ceiling unrestricted, visibility 10 plus. Case 1 recovery, BRC 249, altimeter 2992. Marshall at angels 2, report see me at 10.",
		"Radar 331, Union Marshal, copy switch to BSFG7 command. Good day.",
		"Raider 331, Union Marshal, state 1.9, priority recovery.",
		"Raider, Union Marshal, five by five, go ahead.",
		"Raider 331.",
		"Raider 331, Union Marshal, copy established. Angels 2, position 1 in the stack.",
		"Raider 331, Union Marshal, contact paddles. Good luck.",
	}
	for _, e := range echoes {
		if _, ok := splitMarshalAddress(e); ok {
			t.Errorf("echo accepted as a pilot call: %q", e)
		}
		if cs := marshalCallsignFromText(e); cs != "" {
			t.Errorf("echo produced callsign %q: %q", cs, e)
		}
	}
	// Opens with a garbled address but carries no callsign — must not be
	// answered, or the TX would open ", Union Marshal" and loop.
	if cs := marshalCallsignFromText("You to Marshall, signal Charlie."); cs != "" {
		t.Errorf("callsign-less call produced callsign %q", cs)
	}
}

// Garbled addresses from the Syria Marshal on 2026-09-17, Raider 331 — all
// four were dropped as "not addressed to Marshal".
func TestMarshalCallsFlown20260917(t *testing.T) {
	cases := []struct {
		text   string
		intent marshalIntent
	}{
		{"You there, Marshall, Raider 331, marking moms, 044 for 50 miles, angels 31, state 5.0.", marshalMarkingMom},
		{"You did Marshal, Raider 331, Marking mom 044 for 50 miles, Angels 31, State 4.9.", marshalMarkingMom},
		{"Inner Marshall Raider 331, see you at 10, state 3.9, angels 2.", marshalSeeYouAtTen},
		{"Ma- either Marshal Raider 331, initial.", marshalInitial},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if cs := marshalCallsignFromText(tc.text); cs != "Raider 331" {
				t.Fatalf("callsign = %q, want Raider 331", cs)
			}
			lower := strings.ToLower(tc.text)
			if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != tc.intent {
				t.Errorf("intent = %d, want %d", got, tc.intent)
			}
		})
	}
}

// The operator asked that pilots can ask Marshal for BRC however they phrase it.
func TestMarshalBRCRequestPhrasings(t *testing.T) {
	calls := []string{
		"marshal, raider 331, request brc",
		"marshal, raider 331, say brc",
		"marshal, raider 331, brc",
		"marshal, raider 331, brc?",
		"marshal, raider 331, b r c",
		"marshal, raider 331, vrc",
		"marshal, raider 331, confirm brc",
		"marshal, raider 331, say mother's heading",
		"union marshal, raider 331, what's brc",
	}
	for _, c := range calls {
		if got := classifyMarshalCall(c, extractFuelStateMarshal(c)); got != marshalBRCRequest {
			t.Errorf("%q → intent %d, want BRC request", c, got)
		}
	}
	// A marking-mom's call that mentions BRC is still a check-in to the stack.
	c := "marshal, raider 331, marking moms 350 for 43, angels 16, state 3.5, say brc"
	if got := classifyMarshalCall(c, extractFuelStateMarshal(c)); got != marshalBRCRequest && got != marshalMarkingMom {
		t.Errorf("%q → intent %d", c, got)
	}
}

// Whisper returned the carrier vocabulary prompt verbatim on the Syria Marshal
// freq (20:01:25) and on Training 1's Deckboss (05:30:51); Marshal answered it.
func TestCarrierPromptEchoIsHallucination(t *testing.T) {
	echo := "Marshal, Raider, Venom, marking mom, angels, state, established, commencing, pushing, checking in, see you at ten, signal Charlie, BRC, altimeter, radio check, five by five."
	if !isWhisperHallucination(echo) {
		t.Errorf("carrier prompt echo not filtered: %q", echo)
	}
	real := []string{
		"Union Marshal, Raider 331, marking moms 350, for 43, angels 16, state 3.5.",
		"Marshal, Raider 331, establish angels 2, state 1.9.",
		"Marshal, Raider 331, checking in, marking moms 350 for 43, angels 16, state 3.5, request BRC.",
		"Union Marshall, Raider 331, clear Mother's push command.",
	}
	for _, r := range real {
		if isWhisperHallucination(r) {
			t.Errorf("real call filtered as hallucination: %q", r)
		}
	}
}
