package main

import (
	"testing"
	"time"
)

// 2-hour shifts from the epoch: 10:00-12:00 UTC is slot 5 (alternate),
// 12:00-14:00 slot 6 (the field's own voice).
func TestTowerShiftWaitsForQuiet(t *testing.T) {
	s := &towerShift{slot: 2 * time.Hour, quiet: 5 * time.Minute}
	at := func(hhmm string) time.Time {
		tm, _ := time.Parse("2006-01-02 15:04", "2026-10-15 "+hhmm)
		return tm
	}
	if !s.onAlt(at("11:50")) {
		t.Fatal("11:50 is the alternate's shift")
	}
	// A pilot still in the pattern across 12:00: the alternate keeps working them.
	for _, tm := range []string{"11:58", "12:01", "12:04", "12:08"} {
		if !s.onAlt(at(tm)) {
			t.Fatalf("%s: handover mid-conversation", tm)
		}
	}
	// Quiet from 12:08 to 12:20: the next call is the new shift.
	if s.onAlt(at("12:20")) {
		t.Error("12:20 after 12 min quiet: want the field's own controller")
	}
	if s.onAlt(at("12:21")) {
		t.Error("12:21: shift should hold")
	}
}

// A restart mid-shift picks up the controller on shift - same on every rig.
func TestTowerShiftStartsOnTheClock(t *testing.T) {
	for _, c := range []struct {
		hhmm string
		alt  bool
	}{{"10:30", true}, {"12:30", false}, {"15:59", true}} {
		s := &towerShift{slot: 2 * time.Hour, quiet: 5 * time.Minute}
		tm, _ := time.Parse("2006-01-02 15:04", "2026-10-15 "+c.hhmm)
		if got := s.onAlt(tm); got != c.alt {
			t.Errorf("fresh start at %s: alternate=%v, want %v", c.hhmm, got, c.alt)
		}
	}
}

// No alternate voice (every field but Senaki): always the field's own.
func TestTowerShiftOffWithoutAlternate(t *testing.T) {
	defer func(a, f, v string) { flagElevenTowerAltVoice, flagAirfield, flagTTSVoice = a, f, v }(flagElevenTowerAltVoice, flagAirfield, flagTTSVoice)
	flagAirfield, flagTTSVoice, flagVoiceStyleTower = "UGKS", "alloy", styleTower
	flagElevenTowerAltVoice = ""
	tm, _ := time.Parse("2006-01-02 15:04", "2026-10-15 10:30")
	if p := towerShiftProfile("alloy", tm); p.eleven.ID != elLisa {
		t.Errorf("no alternate set: voice %s, want Lisa", p.eleven.ID)
	}
	flagElevenTowerAltVoice = "irzJNDCo2SzXrIpo6Yd6"
	flagTowerShift = 2 * time.Hour
	towerShiftState = &towerShift{quiet: towerShiftQuiet}
	if p := towerShiftProfile("alloy", tm); p.eleven.ID != "irzJNDCo2SzXrIpo6Yd6" {
		t.Errorf("10:30 with an alternate: voice %s, want the alternate", p.eleven.ID)
	}
}
