package controller

import (
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// 2026-11-06: Bacon (Raider 031) flew the jet Tacview called "Raider 306 | Turbo".
func TestPlayerAliasStalePilotName(t *testing.T) {
	c := NewATCController("Test", &airfield.Airfield{})
	c.UpdateAnyPosition("Raider 306", "FA-18C_hornet", "Air+FixedWing", 36.5, 44.0, 3000, 250, 90, 0)
	c.UpdateAnyPosition("Raider 302", "FA-18C_hornet", "Air+FixedWing", 36.8, 44.0, 3000, 250, 90, 0)
	if c.lookupContact("Raider 031") != nil {
		t.Fatal("031 found before any alias")
	}
	// DCS: 031 is in the jet near 36.5, 44.0 (a few hundred metres off, the poll is a few seconds old); 302 is where Tacview has him.
	c.RefreshPlayerAliases([]PlayerFix{{"Raider 031", 44.003, 36.502}, {"Raider 302", 44.0, 36.8}})
	ct := c.lookupContact("Raider 031")
	if ct == nil || ct.Lon != 36.5 {
		t.Fatalf("031 not mapped to the stale-named jet: %+v", ct)
	}
	if c.lookupContact("Raider 306") != nil {
		t.Error("306 (not flying) still finds Bacon's jet")
	}
	if c.lookupContact("Raider 302") == nil || c.lookupContact("Raider 302").Lon != 36.8 {
		t.Error("302 lost his own jet")
	}
	// Turbo comes back under his own name: no alias, his jet is his.
	c.RefreshPlayerAliases([]PlayerFix{{"Raider 306", 44.0, 36.5}, {"Raider 302", 44.0, 36.8}})
	if c.lookupContact("Raider 306") == nil || c.lookupContact("Raider 031") != nil {
		t.Error("alias not cleared when the pilot is the one Tacview names")
	}
}
