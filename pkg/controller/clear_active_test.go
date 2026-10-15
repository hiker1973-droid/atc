package controller

import (
	"testing"
	"time"
)

// "Clear the active 09" after takeoff is the airborne call (operator
// 2026-10-15); after landing it is still the runway-vacated call.
func TestClearTheActiveFromADepartureIsAirborne(t *testing.T) {
	_, sent, handle := newRecordingController(t)
	handle("Senaki Tower, Raider 302, request taxi")
	handle("Senaki Tower, Raider 302, clear the active 09")
	if got := (*sent)[len(*sent)-1]; !isRelease(got) {
		t.Errorf("departure clear the active got %q, want the departure release", got)
	}
}

func TestClearTheActiveFromAnArrivalIsVacated(t *testing.T) {
	_, sent, handle := newRecordingController(t)
	handle("Senaki Tower, Raider 11, 10 miles inbound")
	handle("Senaki Tower, Raider 11, clear of the active")
	if got := (*sent)[len(*sent)-1]; !isVacated(got) {
		t.Errorf("arrival clear of the active got %q, want runway vacated", got)
	}
}

// No departure on file (callsign heard differently at taxi): Tacview decides.
func TestClearTheActiveUnknownFlightUsesTacview(t *testing.T) {
	c, sent, handle := newRecordingController(t)
	c.allPositions["Raider 302"] = &TacviewContact{Callsign: "Raider 302", AltFt: 1500, SpeedKts: 250, UpdatedAt: time.Now()}
	handle("Senaki Tower, Raider 302, clear the active")
	if got := (*sent)[len(*sent)-1]; !isRelease(got) {
		t.Errorf("airborne unknown flight got %q, want the departure release", got)
	}

	// Rolling out on the ground after landing: vacated.
	c.allPositions["Venom 21"] = &TacviewContact{Callsign: "Venom 21", AltFt: 40, SpeedKts: 20, UpdatedAt: time.Now()}
	handle("Senaki Tower, Venom 21, clear the active")
	if got := (*sent)[len(*sent)-1]; !isVacated(got) {
		t.Errorf("landed unknown flight got %q, want runway vacated", got)
	}
}

// The composer varies its wording; these check which reply it was.
func isRelease(s string) bool { return containsAny(s, "angels") && !containsAny(s, "parking") }
func isVacated(s string) bool { return containsAny(s, "parking") && !containsAny(s, "angels") }
