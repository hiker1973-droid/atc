package controller

import (
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// logCarrierChoice's observable state is lastCarrierChoice — verify it tracks
// transitions correctly so the log-on-change behavior holds. Actual log-level
// switching is not asserted (would require capturing zerolog output) but is
// driven directly by the same predicate this test exercises.
func TestCarrierChoiceTransition(t *testing.T) {
	c := NewATCController("Test", &airfield.Airfield{})

	c.logCarrierChoice("CVN-72 ABE", []string{"CVN-72 ABE"}, "carrier match")
	if c.lastCarrierChoice != "CVN-72 ABE" {
		t.Errorf("after first match lastCarrierChoice = %q, want %q", c.lastCarrierChoice, "CVN-72 ABE")
	}

	c.logCarrierChoice("CVN-72 ABE", []string{"CVN-72 ABE"}, "carrier match")
	if c.lastCarrierChoice != "CVN-72 ABE" {
		t.Errorf("after repeat match lastCarrierChoice = %q, want %q", c.lastCarrierChoice, "CVN-72 ABE")
	}

	c.logCarrierChoice("Carrier strike group-5", []string{"Carrier strike group-5 [group]"}, "carrier match (group-label fallback)")
	if c.lastCarrierChoice != "Carrier strike group-5" {
		t.Errorf("after transition lastCarrierChoice = %q, want %q", c.lastCarrierChoice, "Carrier strike group-5")
	}
}

// feedTraining1 loads the controller with the strike-group contacts exactly as
// the live Training 1 Tacview feed exports them (captured 2026-08-06). Every
// one of these keys contains "carrier", and only one of them is the boat
// Marshal is talking about.
func feedTraining1(c *ATCController) {
	c.UpdateAnyPosition("Carrier strike group", "CVN_72", "Sea+Watercraft+AircraftCarrier", 54.1, 25.3, 60, 25, 358.3, 0)
	c.UpdateAnyPosition("Carrier strike group-1", "LHA_Tarawa", "Sea+Watercraft+AircraftCarrier", 54.3, 25.1, 60, 20, 179.1, 0)
	c.UpdateAnyPosition("Carrier strike group-2", "S-3B Tanker", "Air+FixedWing", 54.2, 25.4, 20000, 250, 176.3, 0)
	c.UpdateAnyPosition("Carrier strike group-10", "SH-60B", "Air+Rotorcraft", 54.1, 25.3, 500, 90, 0.9, 0)
}

// The bug this guards: Marshal called a BRC 180° out because the "does the key
// contain carrier" match picked LHA_Tarawa (steaming 179) instead of CVN_72
// (steaming 358). Both are flat-tops, both keys say "carrier" — only the hull
// name separates them.
func TestFindCarrierPrefersCVNOverAmphib(t *testing.T) {
	c := NewATCController("Test", &airfield.Airfield{})
	feedTraining1(c)

	cs, contact, found := c.findCarrierContact()
	if !found {
		t.Fatal("findCarrierContact found nothing in a feed containing CVN_72")
	}
	if contact.UnitName != "CVN_72" {
		t.Errorf("matched %q (%s), want the CVN_72 hull", contact.UnitName, cs)
	}
	if brc := c.GetCarrierBRC(); brc != 358.3 {
		t.Errorf("BRC = %.1f, want 358.3 (179.1 means the Tarawa was matched)", brc)
	}
}

// Go randomizes map iteration order, so a match that picks "first hit wins"
// can return a different hull on every call — observed live as Marshal
// alternating between two strike groups. Repeat enough times that an unstable
// match is near-certain to show.
func TestFindCarrierIsStableAcrossCalls(t *testing.T) {
	c := NewATCController("Test", &airfield.Airfield{})
	feedTraining1(c)

	first, _, _ := c.findCarrierContact()
	for i := 0; i < 200; i++ {
		cs, _, found := c.findCarrierContact()
		if !found || cs != first {
			t.Fatalf("call %d matched %q, first call matched %q — match is order-dependent", i, cs, first)
		}
	}
}

// Aircraft parented to a strike group carry "carrier" in their group label too.
// With no boat on scope the answer must be "mother off scope" (-1), never a
// tanker's heading.
func TestFindCarrierIgnoresStrikeGroupAircraft(t *testing.T) {
	c := NewATCController("Test", &airfield.Airfield{})
	c.UpdateAnyPosition("Carrier strike group-2", "S-3B Tanker", "Air+FixedWing", 54.2, 25.4, 20000, 250, 176.3, 0)
	c.UpdateAnyPosition("Carrier strike group-10", "SH-60B", "Air+Rotorcraft", 54.1, 25.3, 500, 90, 0.9, 0)

	if cs, _, found := c.findCarrierContact(); found {
		t.Errorf("matched %q, want no match — those contacts are aircraft", cs)
	}
	if brc := c.GetCarrierBRC(); brc != -1 {
		t.Errorf("BRC = %.1f, want -1 (mother off scope)", brc)
	}
}

// Missions that name the CVN in the key and export no Type= must still match —
// the older Training naming convention.
func TestFindCarrierMatchesNamedCVNWithoutType(t *testing.T) {
	c := NewATCController("Test", &airfield.Airfield{})
	c.UpdateAnyPosition("CVN-72 ABE", "", "", 54.1, 25.3, 60, 25, 12.0, 0)

	if brc := c.GetCarrierBRC(); brc != 12.0 {
		t.Errorf("BRC = %.1f, want 12.0", brc)
	}
}
