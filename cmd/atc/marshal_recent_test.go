package main

import (
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/state"
)

func resetMarshalRecent() {
	marshalRecent.mu.Lock()
	marshalRecent.worked = map[string]time.Time{}
	marshalRecent.asked = map[string]time.Time{}
	marshalRecent.mu.Unlock()
}

// Day 2026-10-23 17:10:28 and 2026-10-24 08:01:51: the answer to "say state".
func TestMarshalStateAnswerGarbled(t *testing.T) {
	for _, s := range []struct{ text, cs string }{
		{"Junior Marshall, Raider 312, state 5.9.", "Raider 312"},
		{"Senior Marshal, reader 302, 5.7.", "Raider 302"},
	} {
		if got := marshalCallsignFromText(s.text); got != s.cs {
			t.Errorf("callsign(%q) = %q, want %q", s.text, got, s.cs)
		}
		lower := toLowerForTest(s.text)
		fuel := extractFuelStateMarshal(lower)
		if fuel == 0 {
			fuel = bareMarshalState(lower)
		}
		if in := classifyMarshalCall(lower, fuel); in != marshalState {
			t.Errorf("intent(%q) = %v, want state", s.text, in)
		}
	}

	// Any other garble, within a minute of "say state" to one jet.
	resetMarshalRecent()
	now := time.Now()
	noteMarshalAskedState("Raider 302", now)
	if cs, fuel := pendingStateAnswer("Minor Marsha, Raider 302, 5.7.", now.Add(10*time.Second)); cs != "Raider 302" || fuel != 5.7 {
		t.Errorf("pending answer = %q %v", cs, fuel)
	}
	// Used once, and never for our own ack's echo or after the minute.
	if cs, _ := pendingStateAnswer("5.7", now.Add(12*time.Second)); cs != "" {
		t.Errorf("answer taken twice: %q", cs)
	}
	noteMarshalAskedState("Raider 302", now)
	if cs, _ := pendingStateAnswer("Raider 302, Union Marshal, copy state 5.7.", now.Add(time.Second)); cs != "" {
		t.Errorf("own echo accepted as %q", cs)
	}
	if cs, _ := pendingStateAnswer("state 5.7", now.Add(2*time.Minute)); cs != "" {
		t.Errorf("late answer accepted as %q", cs)
	}
	// Two jets asked: whose answer is it?
	noteMarshalAskedState("Raider 302", now)
	noteMarshalAskedState("Raider 312", now)
	if cs, _ := pendingStateAnswer("state 5.7", now.Add(time.Second)); cs != "" {
		t.Errorf("ambiguous answer accepted as %q", cs)
	}
}

// Day 2026-10-23 17:17:03: "In marshal radio 3.2, three mile initial" was
// answered "Radio 3, ... paddles has you" - the jet had left the stack at
// commencing.
func TestMarshalGarbledCallsignWorkedJet(t *testing.T) {
	resetMarshalRecent()
	stack := state.NewMarshalStack()
	now := time.Now()
	noteMarshalWorked("Raider 312", now.Add(-5*time.Minute))
	text := "In marshal radio 3.2, three mile initial."
	heard := marshalCallsignFromText(text)
	in := classifyMarshalCall(toLowerForTest(text), 0)
	if in != marshalInitial {
		t.Fatalf("intent = %v, want initial", in)
	}
	if cs := fixMarshalCallsign(heard, in, stack, now); cs != "Raider 312" {
		t.Errorf("fix(%q) = %q, want Raider 312", heard, cs)
	}
	// A known jet keeps its callsign; a check-in is never re-mapped.
	if cs := fixMarshalCallsign("Raider 312", marshalCommencing, stack, now); cs != "Raider 312" {
		t.Errorf("known jet = %q", cs)
	}
	if cs := fixMarshalCallsign("Raider 305", marshalMarkingMom, stack, now); cs != "Raider 305" {
		t.Errorf("check-in re-mapped to %q", cs)
	}
	// Two jets: the modex decides, else no guess.
	noteMarshalWorked("Raider 302", now)
	if cs := fixMarshalCallsign("Reader 302", marshalInitial, stack, now); cs != "Raider 302" {
		t.Errorf("modex match = %q", cs)
	}
	if cs := fixMarshalCallsign("Radio 3", marshalInitial, stack, now); cs != "Radio 3" {
		t.Errorf("two jets guessed %q", cs)
	}
	// Long gone: not a jet Marshal is working.
	resetMarshalRecent()
	noteMarshalWorked("Raider 312", now.Add(-time.Hour))
	if cs := fixMarshalCallsign("Radio 3", marshalInitial, stack, now); cs != "Radio 3" {
		t.Errorf("stale jet used: %q", cs)
	}
}
