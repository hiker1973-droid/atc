package state

import (
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

func TestDeparturePosition(t *testing.T) {
	s := NewAirfieldState(&airfield.Airfield{})
	a := s.GetOrCreate("Raider 032")
	b := s.GetOrCreate("Raider 39")
	c := s.GetOrCreate("Venom 211")
	s.EnqueueDeparture(a)
	s.EnqueueDeparture(b)
	s.EnqueueDeparture(c)

	if got := s.DeparturePosition("Raider 032"); got != 1 {
		t.Errorf("Raider 032 position = %d, want 1", got)
	}
	if got := s.DeparturePosition("Raider 39"); got != 2 {
		t.Errorf("Raider 39 position = %d, want 2", got)
	}
	if got := s.DeparturePosition("Venom 211"); got != 3 {
		t.Errorf("Venom 211 position = %d, want 3", got)
	}
	if got := s.DeparturePosition("Nonexistent"); got != 0 {
		t.Errorf("absent callsign position = %d, want 0", got)
	}
}

func TestDepartureAheadOf(t *testing.T) {
	s := NewAirfieldState(&airfield.Airfield{})
	a := s.GetOrCreate("Raider 032")
	b := s.GetOrCreate("Raider 39")
	c := s.GetOrCreate("Venom 211")
	s.EnqueueDeparture(a)
	s.EnqueueDeparture(b)
	s.EnqueueDeparture(c)

	if got := s.DepartureAheadOf("Raider 032"); got != "" {
		t.Errorf("Raider 032 ahead = %q, want \"\" (first in queue)", got)
	}
	if got := s.DepartureAheadOf("Raider 39"); got != "Raider 032" {
		t.Errorf("Raider 39 ahead = %q, want Raider 032", got)
	}
	if got := s.DepartureAheadOf("Venom 211"); got != "Raider 39" {
		t.Errorf("Venom 211 ahead = %q, want Raider 39", got)
	}
	if got := s.DepartureAheadOf("Nonexistent"); got != "" {
		t.Errorf("absent callsign ahead = %q, want \"\"", got)
	}
}
