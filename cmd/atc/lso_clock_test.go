package main

import (
	"testing"
	"time"
)

// The pass card shows the mission's clock: Tacview's ReferenceTime plus the frame time.
func TestLSOMissionClock(t *testing.T) {
	tv := newLSOTacview(&lsoFeed{})
	if !tv.missionTime().IsZero() {
		t.Fatal("no clock before the header")
	}
	tv.handleLine("0,ReferenceTime=2027-01-25T09:00:00Z")
	tv.handleLine("#754.5")
	want := time.Date(2027, 1, 25, 9, 12, 34, 500_000_000, time.UTC)
	if got := tv.missionTime(); !got.Equal(want) {
		t.Errorf("mission time = %v, want %v", got, want)
	}
}
