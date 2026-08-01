package state

import "testing"

func TestAssignCatPreferred_BowPrefersLowCats(t *testing.T) {
	ds := NewDeckbossState()
	if got := ds.AssignCatPreferred("Raider 311", true); got != 1 {
		t.Errorf("aircraft forward on deck should get a bow cat, got cat %d", got)
	}
	if got := ds.AssignCatPreferred("Raider 312", true); got != 2 {
		t.Errorf("second bow aircraft should get cat 2, got cat %d", got)
	}
}

func TestAssignCatPreferred_WaistPrefersHighCats(t *testing.T) {
	ds := NewDeckbossState()
	if got := ds.AssignCatPreferred("Raider 311", false); got != 3 {
		t.Errorf("aircraft aft on deck should get a waist cat, got cat %d", got)
	}
	if got := ds.AssignCatPreferred("Raider 312", false); got != 4 {
		t.Errorf("second waist aircraft should get cat 4, got cat %d", got)
	}
}

// A full bow should not push an aircraft into the conga while waist cats sit
// idle — the preference is a preference, not a restriction.
func TestAssignCatPreferred_FallsThroughToOtherPair(t *testing.T) {
	ds := NewDeckbossState()
	ds.AssignCatPreferred("Raider 311", true)
	ds.AssignCatPreferred("Raider 312", true)
	got := ds.AssignCatPreferred("Raider 313", true)
	if got != 3 && got != 4 {
		t.Errorf("bow full, expected fall-through to a waist cat, got cat %d", got)
	}
}

func TestAssignCatPreferred_ZeroWhenDeckFull(t *testing.T) {
	ds := NewDeckbossState()
	for _, cs := range []string{"a", "b", "c", "d"} {
		ds.AssignCatPreferred(cs, true)
	}
	if got := ds.AssignCatPreferred("Raider 999", true); got != 0 {
		t.Errorf("all four cats taken, expected 0 so caller queues the conga, got %d", got)
	}
}

// Repeated §1 check-ins are common when a pilot doesn't hear the first reply.
// The caller must get their existing cat back, never a second slot.
func TestAssignCatPreferred_IdempotentPerCallsign(t *testing.T) {
	ds := NewDeckbossState()
	first := ds.AssignCatPreferred("Raider 311", true)
	second := ds.AssignCatPreferred("Raider 311", true)
	if first != second {
		t.Fatalf("repeat check-in returned a different cat: %d then %d", first, second)
	}
	held := 0
	for _, cat := range ds.Cats {
		if cat.Callsign == "Raider 311" {
			held++
		}
	}
	if held != 1 {
		t.Errorf("callsign should hold exactly one cat, holds %d", held)
	}
}

// Preference must not leak across the fall-through: an aft aircraft assigned a
// bow cat because the waist was full still owns that bow cat on re-check-in.
func TestAssignCatPreferred_FallThroughIsStable(t *testing.T) {
	ds := NewDeckbossState()
	ds.AssignCatPreferred("Raider 311", false)
	ds.AssignCatPreferred("Raider 312", false)
	got := ds.AssignCatPreferred("Raider 313", false)
	if again := ds.AssignCatPreferred("Raider 313", false); again != got {
		t.Errorf("fell through to cat %d then returned %d on repeat", got, again)
	}
}
