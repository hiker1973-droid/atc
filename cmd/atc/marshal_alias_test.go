package main

import (
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/state"
)

// The Raider 1 / Raider 2 recovery, Day 2026-10-20 18:53-19:17 (Training 1).
func TestMarshalFlightCallsFlown20261020(t *testing.T) {
	marshalAliases.m = map[string]string{}
	stack := state.NewMarshalStack()
	stack.Enqueue("Raider 303", 7.8)
	stack.SetAngels("Raider 303", 2)
	stack.Enqueue("Raider 302", 8.2)
	stack.SetAngels("Raider 302", 3)

	steps := []struct {
		text, heard, want string
		intent            marshalIntent
	}{
		{"Marshal, Raider 303 overhead, angels 2.", "Raider 303", "Raider 303", marshalEstablished},
		{"Marshal established, Angels 2, state 6.4.8.", "", "Raider 303", marshalEstablished},
		{"Union Marshal, Raider 2 flight, see you at 10.", "Raider 2", "Raider 302", marshalSeeYouAtTen},
		{"Union Marshall Raider 2 flight, entering stack position 4, Angels 3.", "Raider 2", "Raider 302", marshalEstablished},
		{"Union Marshal, Raider 2 flight commencing heading 330.", "Raider 2", "Raider 302", marshalCommencing},
	}
	for _, s := range steps {
		got := marshalCallsignFromText(s.text)
		if got != s.heard {
			t.Errorf("callsign(%q) = %q, want %q", s.text, got, s.heard)
		}
		lower := toLowerForTest(s.text)
		in := classifyMarshalCall(lower, extractFuelStateMarshal(lower))
		if in != s.intent {
			t.Errorf("intent(%q) = %v, want %v", s.text, in, s.intent)
		}
		if cs := resolveStackCallsign(got, lower, stack); cs != s.want {
			t.Errorf("resolve(%q) = %q, want %q", s.text, cs, s.want)
		}
	}
	// A new jet's check-in keeps its own modex even at a taken angels.
	if cs := resolveStackCallsign("Raider 304", "marshal, raider 304, marking moms 090 for 30, angels 2", stack); cs != "Raider 304" {
		t.Errorf("new jet resolved to %q", cs)
	}
	// Two jets at the same angels: no guess.
	stack.Enqueue("Raider 311", 5)
	stack.SetAngels("Raider 311", 2)
	if cs := resolveStackCallsign("", "marshal established, angels 2", stack); cs != "" {
		t.Errorf("ambiguous angels resolved to %q", cs)
	}
}

func toLowerForTest(s string) string { return strings.ToLower(s) }
