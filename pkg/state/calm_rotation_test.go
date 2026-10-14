package state

import (
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

func calmField() *airfield.Airfield {
	return &airfield.Airfield{ICAO: "UGSB", RunwayPairs: []airfield.RunwayPair{{
		Primary:    airfield.Runway{Designator: "13", MagneticHeading: 126},
		Reciprocal: airfield.Runway{Designator: "31", MagneticHeading: 306},
	}}}
}

// Light wind: the runway comes from the wall-clock slot, not the wind; at or
// above the cutoff the wind decides again.
func TestCalmRotationFollowsWindAboveCutoff(t *testing.T) {
	af := calmField()
	s := NewAirfieldState(af)
	s.DisableRunwayRotation()
	s.SetCalmRotation(5)

	want := af.CalmRunway(time.Now()).Designator
	s.UpdateWeather(306, 3, 29.92, 10) // light wind from 306 (MagVar 0)
	if s.ActiveRunway != want {
		t.Fatalf("light wind: runway %s, want calm-slot %s", s.ActiveRunway, want)
	}
	s.UpdateWeather(126, 12, 29.92, 10) // real wind from 126
	if s.ActiveRunway != "13" {
		t.Fatalf("12 kts from 126: runway %s, want 13", s.ActiveRunway)
	}
	s.UpdateWeather(306, 5, 29.92, 10) // exactly the cutoff counts as wind
	if s.ActiveRunway != "31" {
		t.Fatalf("5 kts from 306: runway %s, want 31", s.ActiveRunway)
	}
}

// 0 keeps the old always-by-wind behaviour (calm -> first primary).
func TestCalmRotationOff(t *testing.T) {
	s := NewAirfieldState(calmField())
	s.DisableRunwayRotation()
	s.UpdateWeather(306, 4, 29.92, 10)
	if s.ActiveRunway != "31" {
		t.Fatalf("4 kts from 306, calm rotation off: runway %s, want 31", s.ActiveRunway)
	}
}

// A dashboard pick holds until the slot rolls; the tick does not undo it.
func TestCalmRotationKeepsManualPickWithinSlot(t *testing.T) {
	af := calmField()
	s := NewAirfieldState(af)
	s.DisableRunwayRotation()
	s.SetCalmRotation(5)
	other := "13"
	if s.ActiveRunway == "13" {
		other = "31"
	}
	s.SetActiveRunway(other)
	if _, _, changed := s.RotateRunwayIfDue(); changed || s.ActiveRunway != other {
		t.Fatalf("tick changed a manual pick within the slot: %s", s.ActiveRunway)
	}
}

// Both ends come up, and the same clock hour does not land on the same end
// every day (the reason for hashing instead of cycling).
func TestCalmRunwayVaries(t *testing.T) {
	seen := map[int]bool{}
	sameHour := map[int]bool{}
	for slot := int64(0); slot < 60; slot++ {
		seen[airfield.CalmSlotIndex("UGSB", slot, 2)] = true
	}
	for day := int64(0); day < 10; day++ {
		sameHour[airfield.CalmSlotIndex("UGSB", day*6+4, 2)] = true
	}
	if len(seen) != 2 || len(sameHour) != 2 {
		t.Fatalf("calm rotation not varied: ends %v, same-hour %v", seen, sameHour)
	}
}
