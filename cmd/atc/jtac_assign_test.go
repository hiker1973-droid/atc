package main

import (
	"strings"
	"testing"
)

func TestAssassinAssignsOnly(t *testing.T) {
	old := flagJTACAssignOnly
	flagJTACAssignOnly = true
	defer func() { flagJTACAssignOnly = old }()
	if cfgs := jtacConfigs(); cfgs[0].Spoken != "Assassin one one" || cfgs[0].Name != "Assassin-1-1" {
		t.Fatalf("net 1 = %+v", cfgs[0])
	}
	b, _ := newTestTaskingBoard(kolaTaskingTargets)
	const as = "Assassin one one"
	cs := assassinCallsign("Assassin 1-1, Raider 11, checking in")
	if cs != "Raider 11" {
		t.Fatalf("callsign = %q", cs)
	}
	got := b.AssignResponse("Assassin 1-1, Raider 11, checking in", cs, as)
	if !strings.HasPrefix(got, "Raider 11, Assassin one one, target ") || !strings.Contains(got, "Grid ") || !strings.Contains(got, "run-in heading") || !strings.Contains(got, "Report target complete") {
		t.Fatalf("assignment = %q", got)
	}
	if again := b.AssignResponse("Raider 11, checking in", cs, as); !strings.Contains(again, got[strings.Index(got, "target "):strings.Index(got, ".")]) {
		t.Errorf("second check-in changed target: %q vs %q", again, got)
	}
	other := b.AssignResponse("Assassin, Raider 11, request another target", cs, as)
	if !strings.Contains(other, "Raider 11, Assassin one one, target ") || other == got {
		t.Errorf("another = %q", other)
	}
	if done := b.AssignResponse("Raider 11, target complete", cs, as); !strings.Contains(done, "complete") {
		t.Errorf("complete = %q", done)
	}
	if b.AssignResponse("Raider 11, winchester", cs, as) == "" {
		t.Error("no answer to winchester")
	}
	if got := b.AssignResponse("Raider 11, radio check", cs, as); !strings.Contains(got, "loud and clear") {
		t.Errorf("radio check = %q", got)
	}
	// Nothing the 9-line flow says: no cleared hot, no readback demands.
	for _, bad := range []string{"cleared hot", "9-line", "attack heading", "BDA"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(bad)) {
			t.Errorf("assign-only answer mentions %q: %q", bad, got)
		}
	}
}
