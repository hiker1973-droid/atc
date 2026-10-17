package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Senaki hands out parking spots 1-58 in turn (operator 2026-10-16) on the
// clear-of-runway call and on "request taxi to parking"; a flight asking twice
// keeps its spot. The tower says them until Ground is wired.
func TestParkingAssignedOnLanding(t *testing.T) {
	c, sent, handle := departureController(t)
	c.mu.Lock()
	c.airfieldState.Airfield.ParkingFirst, c.airfieldState.Airfield.ParkingLast = 1, 3
	c.mu.Unlock()
	last := func() string { return (*sent)[len(*sent)-1] }

	handle("Test Tower, Raider 302, clear of the active")
	if got := last(); !strings.HasPrefix(got, "Raider 302, Test Tower, ") || !hasSpot(got, 1) {
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
	if got := last(); !hasSpot(got, 3) || strings.Contains(got, "hold short") {
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

// With Ground wired the parking reply is spoken by "<field> Ground" in its own
// voice; everything else stays with the tower.
func TestParkingSpokenByGround(t *testing.T) {
	c, sent, handle := departureController(t)
	var ground []string
	c.mu.Lock()
	c.airfieldState.Airfield.ParkingFirst, c.airfieldState.Airfield.ParkingLast = 1, 58
	c.mu.Unlock()
	c.SetGroundTransmitFn("Test Ground", func(_ context.Context, text string) { ground = append(ground, text) })

	n := len(*sent)
	handle("Test Tower, Raider 302, clear of the active")
	if len(ground) != 1 || !strings.HasPrefix(ground[0], "Raider 302, Test Ground, ") || !hasSpot(ground[0], 1) {
		t.Errorf("Ground said %q", ground)
	}
	if len(*sent) != n {
		t.Errorf("the tower also spoke: %q", (*sent)[n:])
	}
	handle("Test Tower, Venom 203, request taxi")
	if len(ground) != 1 || len(*sent) == n {
		t.Errorf("outbound taxi went to Ground: ground %q", ground)
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
	for _, c := range []struct{ n, spot int }{{0, 1}, {57, 58}, {58, 1}} {
		if spot, ok := af.ParkingSpot(c.n); !ok || spot != c.spot {
			t.Errorf("ParkingSpot(%d) = %d, want %d", c.n, spot, c.spot)
		}
	}
}

func hasSpot(got string, n int) bool {
	return regexp.MustCompile(fmt.Sprintf(`spot %d\b`, n)).MatchString(got)
}
