package airfield

import (
	"testing"
	"time"
)

// Verifies that RotationRunway returns the documented 4h-slot cycle at every
// hour offset, across all three deployed airfields. Catches regressions in
// the cycle order, the slot math, or any airfield definition that changes
// pair ordering.
func TestRotationRunway_Cycles(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		af       *Airfield
		expected []string
	}{
		{
			name:     "OMDM single-pair 09/27",
			af:       OMDM,
			expected: []string{"09", "27"},
		},
		{
			name:     "OMAM two-parallel 31L/13R/31R/13L",
			af:       OMAM,
			expected: []string{"31L", "13R", "31R", "13L"},
		},
		{
			name:     "OMAL single-pair 19/01",
			af:       OMAL,
			expected: []string{"19", "01"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Sample every 4h offset out to two full cycles + a wraparound to
			// confirm modulo behavior.
			for slot := 0; slot < len(tc.expected)*2+1; slot++ {
				offset := time.Duration(slot) * RotationSlotDuration
				now := anchor.Add(offset)
				got := tc.af.RotationRunway(now, anchor).Designator
				want := tc.expected[slot%len(tc.expected)]
				if got != want {
					t.Errorf("slot %d (offset %s): got %q, want %q",
						slot, offset, got, want)
				}
			}
		})
	}
}

// Verifies that times within a slot return the same runway — i.e. transitions
// are crisp at the 4h boundary, not gradual.
func TestRotationRunway_NoDriftWithinSlot(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, offsetMin := range []int{0, 1, 60, 119, 239} {
		now := anchor.Add(time.Duration(offsetMin) * time.Minute)
		got := OMDM.RotationRunway(now, anchor).Designator
		if got != "09" {
			t.Errorf("OMDM at +%dmin: got %q, want \"09\" (still slot 0)", offsetMin, got)
		}
	}
	// 240 min = 4h exactly → slot 1
	gotBoundary := OMDM.RotationRunway(anchor.Add(240*time.Minute), anchor).Designator
	if gotBoundary != "27" {
		t.Errorf("OMDM at +240min (slot 1): got %q, want \"27\"", gotBoundary)
	}
}

// Verifies negative offsets (now before anchor) clamp to slot 0 rather than
// panicking or wrapping into the cycle backwards.
func TestRotationRunway_PreAnchorClamps(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	before := anchor.Add(-12 * time.Hour)
	got := OMAM.RotationRunway(before, anchor).Designator
	if got != "31L" {
		t.Errorf("OMAM pre-anchor: got %q, want \"31L\" (slot 0)", got)
	}
}
