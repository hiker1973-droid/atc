package state

import (
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

func TestTimeSinceLastDeparture_ZeroOnInit(t *testing.T) {
	s := NewAirfieldState(&airfield.Airfield{})
	if got := s.TimeSinceLastDeparture(); got < time.Hour {
		t.Errorf("with no prior departure, expected a large duration, got %s", got)
	}
}

func TestClearForTakeoff_UpdatesLastClearedAt(t *testing.T) {
	s := NewAirfieldState(&airfield.Airfield{})
	ac := s.GetOrCreate("Raider 032")
	s.EnqueueDeparture(ac)

	before := time.Now()
	cleared := s.ClearForTakeoff("Raider 032")
	after := time.Now()

	if cleared == nil {
		t.Fatal("ClearForTakeoff returned nil")
	}
	if s.LastDepartureClearedAt.Before(before) || s.LastDepartureClearedAt.After(after) {
		t.Errorf("LastDepartureClearedAt %s not within [%s, %s]", s.LastDepartureClearedAt, before, after)
	}
	if elapsed := s.TimeSinceLastDeparture(); elapsed > time.Second {
		t.Errorf("TimeSinceLastDeparture immediately after clear = %s, want < 1s", elapsed)
	}
}

func TestClearForTakeoff_NoUpdateOnMiss(t *testing.T) {
	s := NewAirfieldState(&airfield.Airfield{})
	got := s.ClearForTakeoff("Nonexistent 999")
	if got != nil {
		t.Errorf("ClearForTakeoff for absent callsign returned %v, want nil", got)
	}
	if !s.LastDepartureClearedAt.IsZero() {
		t.Errorf("LastDepartureClearedAt updated on miss: %s", s.LastDepartureClearedAt)
	}
}
