package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
)

// A departure held for traffic or spacing must be cleared by checkConflicts
// once that passes. Before, only the no-traffic "line up and wait" path set
// HoldingShort, so a held jet sat at the hold until the pilot called again.

func TestHeldForTrafficIsReleased(t *testing.T) {
	for _, call := range []string{"holding short runway 27", "ready for takeoff"} {
		t.Run(call, func(t *testing.T) {
			ctx := context.Background()
			c, sent := proactiveController(t)
			s := c.airfieldState

			inbound := s.GetOrCreate("Venom 21")
			s.EnqueueLanding(inbound)
			lon, lat := offsetNm(10, 0)
			s.UpdatePosition("Venom 21", orb.Point{lon, lat}, 3000, 250)

			c.HandleRequest(ctx, ParseIntent("Test Tower, Raider 11, "+call, "Test Tower"))
			if len(*sent) != 1 || !strings.Contains((*sent)[0], "hold") {
				t.Fatalf("want a hold for traffic, got %q", *sent)
			}

			c.checkConflicts(ctx)
			if len(*sent) != 1 {
				t.Fatalf("released with traffic still inside the hold-short radius: %q", *sent)
			}

			s.Remove("Venom 21") // runway vacated
			c.checkConflicts(ctx)
			if len(*sent) != 2 || !strings.HasPrefix((*sent)[1], "Raider 11") || !strings.Contains((*sent)[1], "airborne") {
				t.Fatalf("want one takeoff clearance once traffic clears, got %q", *sent)
			}

			c.checkConflicts(ctx)
			if len(*sent) != 2 {
				t.Fatalf("takeoff clearance repeated: %q", *sent)
			}
		})
	}
}

func TestHeldForSpacingIsReleased(t *testing.T) {
	ctx := context.Background()
	c, sent := proactiveController(t)
	s := c.airfieldState
	s.LastDepartureClearedAt = time.Now()

	c.HandleRequest(ctx, ParseIntent("Test Tower, Raider 11, holding short runway 27", "Test Tower"))
	if len(*sent) != 1 || !strings.Contains((*sent)[0], "spacing") {
		t.Fatalf("want a departure-spacing hold, got %q", *sent)
	}

	c.checkConflicts(ctx)
	if len(*sent) != 1 {
		t.Fatalf("released inside the spacing window: %q", *sent)
	}

	s.LastDepartureClearedAt = time.Now().Add(-2 * DepartureSpacingSec)
	c.checkConflicts(ctx)
	if len(*sent) != 2 || !strings.Contains((*sent)[1], "airborne") {
		t.Fatalf("want a takeoff clearance once spacing expires, got %q", *sent)
	}
}
