package controller

import (
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Senaki hands out parking areas F, G, A, B, C, D via November in turn
// (operator 2026-10-16), on the clear-of-runway call and on "request taxi to
// parking"; a flight asking twice keeps its area.
func TestParkingAssignedOnLanding(t *testing.T) {
	c, sent, handle := departureController(t)
	c.mu.Lock()
	c.airfieldState.Airfield.ParkingAreas = []string{"Foxtrot", "Golf", "Alpha"}
	c.airfieldState.Airfield.ParkingVia = "November"
	c.mu.Unlock()
	last := func() string { return (*sent)[len(*sent)-1] }

	handle("Test Tower, Raider 302, clear of the active")
	if got := last(); !strings.Contains(got, "parking area Foxtrot") || !strings.Contains(got, "November") {
		t.Errorf("first clear call -> %q", got)
	}
	handle("Test Tower, Raider 303, runway vacated")
	if got := last(); !strings.Contains(got, "parking area Golf") {
		t.Errorf("second clear call -> %q", got)
	}
	handle("Test Tower, Raider 302, request taxi to parking")
	if got := last(); !strings.HasPrefix(got, "Raider 302, Test Tower, ") || !strings.Contains(got, "parking area Foxtrot") {
		t.Errorf("Raider 302 asking again -> %q, want Foxtrot again", got)
	}
	handle("Test Tower, Venom 201, request taxi to parking")
	if got := last(); !strings.Contains(got, "parking area Alpha") || strings.Contains(got, "hold short") {
		t.Errorf("taxi to parking -> %q", got)
	}
	handle("Test Tower, Venom 202, request taxi to parking")
	if got := last(); !strings.Contains(got, "parking area Foxtrot") {
		t.Errorf("fourth flight -> %q, want the list to wrap to Foxtrot", got)
	}
	// An outbound taxi request is unchanged.
	handle("Test Tower, Venom 203, request taxi")
	if got := last(); !strings.Contains(got, "hold short") {
		t.Errorf("request taxi -> %q", got)
	}
}

func TestParkingNoAreas(t *testing.T) {
	_, sent, handle := departureController(t)
	handle("Test Tower, Raider 302, request taxi to parking")
	if got := (*sent)[len(*sent)-1]; !strings.Contains(got, "taxi to parking") || strings.Contains(got, "area") {
		t.Errorf("no areas -> %q", got)
	}
}

func TestSenakiParkingAreas(t *testing.T) {
	want := "Foxtrot Golf Alpha Bravo Charlie Delta"
	if got := strings.Join(airfield.UGKS.ParkingAreas, " "); got != want || airfield.UGKS.ParkingVia != "November" {
		t.Errorf("Senaki parking = %q via %q", got, airfield.UGKS.ParkingVia)
	}
}
