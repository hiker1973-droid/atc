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

// Raider 302 on the Caucasus Marshal (DCS3, 306.3), 2026-10-14. All three got
// "intent miss" and silence; the first two answered Marshal's own "report your
// altitude and range".
func TestMarshalCallsFlown20261014(t *testing.T) {
	cases := []struct {
		text   string
		intent marshalIntent
		dist   int
	}{
		{"Union Marshal Raider 302, Angels 9, Range 25.", marshalDME, 25},
		{"Union Marshal Raider 302, angels 8.5, range 23.", marshalDME, 23},
		{"Union Marshal Raider 302 entering stack position 3, angels 2.", marshalEstablished, 0},
		{"Union Marshall Raider 302 entering the stack, angels 2.", marshalEstablished, 0},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if cs := marshalCallsignFromText(tc.text); cs != "Raider 302" {
				t.Errorf("callsign = %q, want Raider 302", cs)
			}
			lower := strings.ToLower(tc.text)
			if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != tc.intent {
				t.Errorf("intent = %d, want %d", got, tc.intent)
			}
			if tc.dist > 0 {
				if d := extractDMEDistance(lower); d != tc.dist {
					t.Errorf("distance = %d, want %d", d, tc.dist)
				}
			}
		})
	}
	// Our own established ack says "entering stack" — it must stay an echo.
	if _, ok := splitMarshalAddress("Raider 302, Union Marshal, entering stack at angels 2, position 3, hold for Charlie."); ok {
		t.Error("Marshal's own entering-stack ack read as a pilot call")
	}
}

// Raider 308 on Deckboss 128.6, 2026-10-14: Whisper split the callsign with a
// comma and Deckboss answered "Raider, Deckboss, five by five"; "Dark boss"
// and "Black boss" weren't known addresses at all.
func TestDeckbossCallsignFlown20261014(t *testing.T) {
	cases := []struct{ text, want string }{
		{"Deck boss, Raider, 308, comm check.", "Raider 308"},
		{"Tech boss, Raider, 308, Comcheck.", "Raider 308"},
		{"Dark boss, Raider, 308, com check.", "Raider 308"},
		{"Black Boss Raider 308, Kamchatka.", "Raider 308"},
		{"Duck boss, Raider 308, green jet.", "Raider 308"},
	}
	for _, tc := range cases {
		if got := extractCallsignSkippingAddress(tc.text, deckbossAddrPrefixes...); got != tc.want {
			t.Errorf("callsign(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// Raider 302 off and back to the boat, DCS3 2026-10-15 14:43-15:00.
func TestMarshalCallsFlown20261015(t *testing.T) {
	cases := []struct {
		text   string
		cs     string
		intent marshalIntent
	}{
		// Whisper's "marker" for "marking": got only "state 7.6, copy".
		{"Union Marshal Raider 302, marker mom's 300 for 27, angels 9.7, state 7.6.", "Raider 302", marshalMarkingMom},
		// "You need Marshall" for "Union Marshal": dropped as not addressed.
		{"You need Marshall, Raider 302, marking moms, 305, angels not for 25, angels 9.3, state 7.9.", "Raider 302", marshalMarkingMom},
		// A readback of "report see me at ten", at 29 nm: answered "copy visual
		// at ten miles". Now no reply.
		{"Unit Marshall, Raider 302, BRC 96, Marshalling at Angels 2, we'll report see you at 10.", "Raider 302", marshalNone},
		// The real call still lands.
		{"Union Marshal Raider 302, see you at 10.", "Raider 302", marshalSeeYouAtTen},
		// A jet leaving: answered "contact 7 DME, continue".
		{"Union Marshal Raider 302 is seven miles out, pushing command.", "Raider 302", marshalPushCommand},
		// A bare distance is still a DME report.
		{"Union Marshal Raider 302, altitude Angels 4.2, range 16.", "Raider 302", marshalDME},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if cs := marshalCallsignFromText(tc.text); cs != tc.cs {
				t.Errorf("callsign = %q, want %q", cs, tc.cs)
			}
			lower := strings.ToLower(tc.text)
			if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != tc.intent {
				t.Errorf("intent = %d, want %d", got, tc.intent)
			}
		})
	}
}

// Raiders 302 and 312 back to the boat, DCS3 2026-10-17 10:40-11:05. Signal
// Charlie never came: both stack calls were read as fuel-state reports.
func TestMarshalCallsFlown20261017(t *testing.T) {
	cases := []struct {
		text   string
		intent marshalIntent
	}{
		// Answered "state 6.4, copy".
		{"Union Marshal, Raider 312 is 221 for 59, Angels 28, state 6.4.", marshalMarkingMom},
		// Readbacks answered "BRC 096" again.
		{"Union Marshal, Raider 312, understands BRC 096, altimeter 3000, Marshal, angels 2, I'll call you at 10.", marshalNone},
		{"Union Marshal Raider 302 BRC 096 marshalling at angels 3, will report CU at 10.", marshalNone},
		// Answered "state, copy": never holding, so no Charlie.
		{"Union Marshal Raider 302 entering stack position 3, fuel state 4.9.", marshalEstablished},
		{"Union Marshal, Raider 312, established angels 2, entry point 1, state 5.3.", marshalEstablished},
		// Intent miss.
		{"Union Marshal, Raider 312, request Charlie.", marshalEstablished},
		// Answered "contact 3 DME, continue" instead of paddles.
		{"Union Marshal Raider 3-0-2, three mile initial.", marshalInitial},
		{"Union Marshall, Raider 312, 3-mile initial, 4.7.", marshalInitial},
		// Still a BRC question and still a state report.
		{"Union Marshall Raider 302 current BRC.", marshalBRCRequest},
		{"Union Marshal, Raider 302, request BRC.", marshalBRCRequest},
		{"Union Marshal Raider 302 entering stack position 3, fuel state 4.9, 15 miles.", marshalEstablished},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			lower := strings.ToLower(tc.text)
			if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != tc.intent {
				t.Errorf("intent = %d, want %d", got, tc.intent)
			}
		})
	}
}

// Day 2026-10-17 afternoon: calls dropped as not addressed, and a readback
// answered as the real "see you at ten".
func TestMarshalCallsFlown20261017Afternoon(t *testing.T) {
	if cs := marshalCallsignFromText("You and your marshal, Raider 312, angels 2000, 8.8 DME."); cs != "Raider 312" {
		t.Errorf("garbled address: callsign = %q, want Raider 312", cs)
	}
	if cs := unaddressedStackCallsign("Raider 312, established, angels 2, entry point 3, state 6.5."); cs != "Raider 312" {
		t.Errorf("unaddressed established: callsign = %q, want Raider 312", cs)
	}
	for _, s := range []string{
		// Our own acks, with and without the address surviving Whisper.
		"Raider 312, Union Marshal, copy established, angels two, position one in the stack.",
		"Raider 312, entering stack at angels two, position one, hold for Charlie.",
		"Raider 312, onion martial, roger, in the stack angels two at position one, stand by for Charlie.",
		// Not a stack call, or no callsign up front.
		"312, Hornet ball, 5.5.",
		"Raider 312, three mile initial.",
		"Well, established angels 2, Raider 312.",
	} {
		if cs := unaddressedStackCallsign(s); cs != "" {
			t.Errorf("accepted %q as a stack call from %q", s, cs)
		}
	}
	lower := strings.ToLower("Union Marshall Raider 302, BRC 08096, Angels stack at Angels 3, will call, see you at 10.")
	if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != marshalNone {
		t.Errorf("readback intent = %d, want none", got)
	}
}

// A DME report carrying a state has the state read back with the DME (Day
// 2026-10-17 13:41:12, 10:50:54).
func TestMarshalAltRangeStateAnswer(t *testing.T) {
	cases := []struct {
		text  string
		dist  int
		state float64
	}{
		{"Union Marshal, Raider 302, Angels 3, range 17.6, state 7.7.", 17, 7.7},
		{"Union Marshal, Raider 312, angels 2, 9 miles, state 5.5.", 9, 5.5},
		{"Union Marshal, Raider 319, angels 2, 10 miles range.", 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			lower := strings.ToLower(tc.text)
			fs := extractFuelStateMarshal(lower)
			if got := classifyMarshalCall(lower, fs); got != marshalDME {
				t.Errorf("intent = %d, want DME", got)
			}
			if d := extractDMEDistance(lower); d != tc.dist {
				t.Errorf("dist = %d, want %d", d, tc.dist)
			}
			if fs != tc.state {
				t.Errorf("state = %v, want %v", fs, tc.state)
			}
		})
	}
}

// "Tac boss, Raider 302 airborne" was answered "Tac boss, Deckboss, copy
// airborne" (DCS3 2026-10-15 14:49:43).
func TestDeckbossCallsignFlown20261015(t *testing.T) {
	if got := extractCallsignSkippingAddress("Tac boss, Raider 302 airborne.", deckbossAddrPrefixes...); got != "Raider 302" {
		t.Errorf("callsign = %q, want Raider 302", got)
	}
}
