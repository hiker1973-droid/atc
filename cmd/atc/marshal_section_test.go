package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/state"
)

func TestMarshalWingmen(t *testing.T) {
	cases := []struct {
		text, lead string
		want       []string
	}{
		// The operator's phrase (2026-10-20).
		{"Union Marshal, Raider 302 holding hands with 032, marking moms 090 for 30, angels 20, state 7.0.", "Raider 302", []string{"Raider 032"}},
		// Flown Day 2026-10-20.
		{"Marshal Raider flight, Raider 303 holding hands with Raider 319, inbound 330 for 44, angels 20, state 7.8.", "Raider 303", []string{"Raider 319"}},
		{"Union Marshal, Raider 302 and 0-3-1, marking moms, 42 for 030, angels 22, state 8.2.", "Raider 302", []string{"Raider 031"}},
		// A four-ship.
		{"Union Marshal, Raider 302 with 032 and 311 and 319, marking moms 090 for 30, angels 20, state 7.0.", "Raider 302", []string{"Raider 032", "Raider 311", "Raider 319"}},
		// Alone: nothing after the lead is a wingman.
		{"Union Marshal, Raider 302, marking moms 090 for 30, angels 20, state 7.0.", "Raider 302", nil},
		{"Union Marshal, Raider 302 marking moms 090 for 30 and angels 20.", "Raider 302", nil},
	}
	for _, c := range cases {
		if got := marshalWingmen(c.text, c.lead); !reflect.DeepEqual(got, c.want) {
			t.Errorf("marshalWingmen(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// A section holds as one: one slot, addressed "<lead> flight", and a wingman's
// call by modex reaches the lead's entry.
func TestMarshalSection(t *testing.T) {
	marshalAliases.m = map[string]string{}
	stack := state.NewMarshalStack()
	stack.Enqueue("Raider 032", 6.9) // the wingman had checked in alone first
	stack.SetAngels("Raider 032", 2)
	stack.Enqueue("Raider 302", 7.0)
	joinSection(stack, "Raider 302", []string{"Raider 032"})
	stack.SetAngels("Raider 302", 2)

	if n := stack.Count(); n != 1 {
		t.Fatalf("stack has %d entries, want the one section", n)
	}
	if got := stackTXCallsign(stack, "Raider 302"); got != "Raider 302 flight" {
		t.Errorf("addressed as %q", got)
	}
	for _, c := range []struct{ heard, text string }{
		{"Raider 032", "union marshal, raider 032, established angels 2"},
		{"", "marshal, 032, see you at 10"},
		{"Raider 302", "union marshal, raider 302 flight commencing"},
	} {
		if got := resolveStackCallsign(c.heard, c.text, stack); got != "Raider 302" {
			t.Errorf("resolve(%q, %q) = %q, want Raider 302", c.heard, c.text, got)
		}
	}
	ac, _ := stack.GetAircraft("Raider 302")
	if strings.Join(ac.Wingmen, ",") != "Raider 032" {
		t.Errorf("wingmen = %v", ac.Wingmen)
	}
	stack.Remove("Raider 302") // commencing
	if stack.Count() != 0 {
		t.Error("section still in the stack after commencing")
	}
}
