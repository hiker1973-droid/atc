package controller

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Senaki hands out parking spots 1-58 in turn, 1-30 via Bravo and 31-58 via
// Charlie, both then November (operator 2026-10-16), on the clear-of-runway
// call and on "request taxi to parking"; a flight asking twice keeps its spot.
func TestParkingAssignedOnLanding(t *testing.T) {
	c, sent, handle := departureController(t)
	c.mu.Lock()
	c.airfieldState.Airfield.ParkingRoutes = []airfield.ParkingRoute{
		{First: 1, Last: 2, Via: "Bravo, November"},
		{First: 31, Last: 31, Via: "Charlie, November"},
	}
	c.mu.Unlock()
	last := func() string { return (*sent)[len(*sent)-1] }

	handle("Test Tower, Raider 302, clear of the active")
	if got := last(); !hasSpot(got, 1) || !strings.Contains(got, "Bravo, November") {
		t.Errorf("first clear call -> %q", got)
	}
	handle("Test Tower, Raider 303, runway vacated")
	if got := last(); !hasSpot(got, 2) {
		t.Errorf("second clear call -> %q", got)
	}
	handle("Test Tower, Raider 302, request taxi to parking")
	if got := last(); !strings.HasPrefix(got, "Raider 302, Test Tower, ") || !hasSpot(got, 1) {
		t.Errorf("Raider 302 asking again -> %q, want spot 1 again", got)
	}
	handle("Test Tower, Venom 201, request taxi to parking")
	if got := last(); !hasSpot(got, 31) || !strings.Contains(got, "Charlie, November") || strings.Contains(got, "hold short") {
		t.Errorf("taxi to parking -> %q", got)
	}
	handle("Test Tower, Venom 202, request taxi to parking")
	if got := last(); !hasSpot(got, 1) {
		t.Errorf("fourth flight -> %q, want the spots to wrap to 1", got)
	}
	// An outbound taxi request is unchanged.
	handle("Test Tower, Venom 203, request taxi")
	if got := last(); !strings.Contains(got, "hold short") {
		t.Errorf("request taxi -> %q", got)
	}
}

func TestParkingNoSpots(t *testing.T) {
	_, sent, handle := departureController(t)
	handle("Test Tower, Raider 302, request taxi to parking")
	if got := (*sent)[len(*sent)-1]; !strings.Contains(got, "taxi to parking") || strings.Contains(got, "spot") {
		t.Errorf("no spots -> %q", got)
	}
}

func TestSenakiParkingSpots(t *testing.T) {
	af := airfield.UGKS
	for _, c := range []struct {
		n, spot int
		via     string
	}{{0, 1, "Bravo, November"}, {29, 30, "Bravo, November"}, {30, 31, "Charlie, November"}, {57, 58, "Charlie, November"}, {58, 1, "Bravo, November"}} {
		spot, via, ok := af.ParkingSpot(c.n)
		if !ok || spot != c.spot || via != c.via {
			t.Errorf("ParkingSpot(%d) = %d %q, want %d %q", c.n, spot, via, c.spot, c.via)
		}
	}
}

func hasSpot(got string, n int) bool {
	return regexp.MustCompile(fmt.Sprintf(`spot %d\b`, n)).MatchString(got)
}
