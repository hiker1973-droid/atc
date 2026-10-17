package main

import (
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/state"
)

func TestNextCharlie(t *testing.T) {
	now := time.Now()
	hold := func(cs string, angels int) *state.MarshalAircraft {
		return &state.MarshalAircraft{Callsign: cs, Angels: angels, Phase: "holding", UpdatedAt: now}
	}
	g := &charlieGate{}
	stack := []*state.MarshalAircraft{hold("Raider 303", 3), hold("Raider 302", 2),
		{Callsign: "Venom 201", Angels: 4, Phase: "inbound"}}
	if got := g.nextCharlie(stack, now); got != "Raider 302" {
		t.Fatalf("lowest holding jet -> %q, want Raider 302", got)
	}
	g.given(now)
	stack[1].Phase = "charlie"
	if got := g.nextCharlie(stack, now.Add(90*time.Second)); got != "" {
		t.Errorf("Raider 302 has Charlie and has not commenced -> %q, want nobody", got)
	}
	stack = stack[:1] // Raider 302 commenced and left the stack
	if got := g.nextCharlie(stack, now.Add(30*time.Second)); got != "" {
		t.Errorf("inside the spacing -> %q, want nobody", got)
	}
	if got := g.nextCharlie(stack, now.Add(61*time.Second)); got != "Raider 303" {
		t.Errorf("after the spacing -> %q, want Raider 303", got)
	}
	if got := (&charlieGate{}).nextCharlie([]*state.MarshalAircraft{{Callsign: "Venom 201", Phase: "inbound"}}, now); got != "" {
		t.Errorf("nobody holding -> %q", got)
	}
}
