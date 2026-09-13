package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
	"github.com/vsfg7/atc/pkg/airfield"
)

// The test field sits on the deck_spot_test.go reference point so offsetNm
// places contacts relative to it.
const testFieldElevFt = 200

func proactiveController(t *testing.T) (*ATCController, *[]string) {
	t.Helper()
	c := NewATCController("Test Tower", &airfield.Airfield{
		Name:        "Test",
		Center:      orb.Point{carLon, carLat},
		ElevationFt: testFieldElevFt,
	})
	sent := &[]string{}
	c.SetTransmitFn(func(_ context.Context, text string) { *sent = append(*sent, text) })
	return c, sent
}

// putContact places a contact northNm due north of the field, fresh.
func putContact(c *ATCController, cs, objType string, northNm, altFt float64) *TacviewContact {
	lon, lat := offsetNm(northNm, 0)
	ct := &TacviewContact{Callsign: cs, ObjType: objType, Lon: lon, Lat: lat, AltFt: altFt, UpdatedAt: time.Now()}
	c.allPositions[cs] = ct
	return ct
}

func TestUnknownTrafficCall(t *testing.T) {
	const air = "Air+FixedWing"
	cases := []struct {
		name         string
		objType      string
		player       bool
		dist1, dist2 float64
		altFt        float64
		phase        string
		talked       bool
		want         bool
	}{
		{"human closing inside radius", air, true, 7.5, 6.5, 2200, "", false, true},
		{"AI flight is never called", air, false, 7.5, 6.5, 2200, "", false, false},
		{"carrier is never called", "Sea+Watercraft+AircraftCarrier", true, 7.5, 6.5, 2200, "", false, false},
		{"opening range", air, true, 6.5, 7.5, 2200, "", false, false},
		{"outside radius", air, true, 12, 11, 2200, "", false, false},
		{"overflying high", air, true, 7.5, 6.5, 9000, "", false, false},
		{"on the ground", air, true, 1.2, 1.0, testFieldElevFt + 10, "", false, false},
		{"departing", air, true, 7.5, 6.5, 2200, "departing", false, false},
		{"talked to tower recently", air, true, 7.5, 6.5, 2200, "", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, sent := proactiveController(t)
			ctx := context.Background()
			if tc.player {
				c.MarkPlayer("Raider 11")
			}
			if tc.talked {
				c.airfieldState.GetOrCreate("Raider 11").LastContact = time.Now()
			}
			putContact(c, "Raider 11", tc.objType, tc.dist1, tc.altFt)
			c.checkUnknownTraffic(ctx)
			if len(*sent) != 0 {
				t.Fatalf("first sighting must never call (no range trend): %q", *sent)
			}
			putContact(c, "Raider 11", tc.objType, tc.dist2, tc.altFt).DetectedPhase = tc.phase
			c.checkUnknownTraffic(ctx)
			got := len(*sent) == 1
			if got != tc.want {
				t.Fatalf("called = %v, want %v (sent %q)", got, tc.want, *sent)
			}
			if got && (!strings.Contains((*sent)[0], "Raider 11") || !strings.Contains((*sent)[0], "say intentions")) {
				t.Errorf("unexpected call text: %q", (*sent)[0])
			}
		})
	}
}

func TestUnknownTrafficCooldownAndOnePerTick(t *testing.T) {
	c, sent := proactiveController(t)
	ctx := context.Background()
	c.MarkPlayer("Raider 11")
	c.MarkPlayer("Venom 21")
	putContact(c, "Raider 11", "Air+FixedWing", 7.5, 2200)
	putContact(c, "Venom 21", "Air+FixedWing", 7.8, 2200)
	c.checkUnknownTraffic(ctx)

	putContact(c, "Raider 11", "Air+FixedWing", 6.5, 2200)
	putContact(c, "Venom 21", "Air+FixedWing", 7.0, 2200)
	c.checkUnknownTraffic(ctx)
	if len(*sent) != 1 || !strings.HasPrefix((*sent)[0], "Raider 11") {
		t.Fatalf("tick 2 should call only the closest aircraft: %q", *sent)
	}

	putContact(c, "Raider 11", "Air+FixedWing", 5.5, 2200)
	putContact(c, "Venom 21", "Air+FixedWing", 6.0, 2200)
	c.checkUnknownTraffic(ctx)
	if len(*sent) != 2 || !strings.HasPrefix((*sent)[1], "Venom 21") {
		t.Fatalf("tick 3 should call the second aircraft, not repeat the first: %q", *sent)
	}

	putContact(c, "Raider 11", "Air+FixedWing", 4.5, 2200)
	putContact(c, "Venom 21", "Air+FixedWing", 5.0, 2200)
	c.checkUnknownTraffic(ctx)
	if len(*sent) != 2 {
		t.Fatalf("both aircraft are inside the cooldown, no new call expected: %q", *sent)
	}

	// Stale data is never acted on, even after the cooldown.
	c.unknownWarnedAt = map[string]time.Time{}
	putContact(c, "Raider 11", "Air+FixedWing", 3.5, 2200).UpdatedAt = time.Now().Add(-time.Minute)
	delete(c.allPositions, "Venom 21")
	c.checkUnknownTraffic(ctx)
	if len(*sent) != 2 {
		t.Fatalf("stale contact was called: %q", *sent)
	}
}

func TestProactiveCallsAreOffByDefault(t *testing.T) {
	c, sent := proactiveController(t)
	ctx := context.Background()
	c.MarkPlayer("Raider 11")
	putContact(c, "Raider 11", "Air+FixedWing", 7.5, 2200)
	c.checkConflicts(ctx)
	putContact(c, "Raider 11", "Air+FixedWing", 6.5, 2200)
	c.checkConflicts(ctx)
	if len(*sent) != 0 {
		t.Fatalf("flag off must stay silent: %q", *sent)
	}

	// Enabling starts the range trend from scratch, so the first enabled tick
	// is a first sighting and only the second one calls.
	c.SetUnknownTrafficCalls(true)
	putContact(c, "Raider 11", "Air+FixedWing", 5.5, 2200)
	c.checkConflicts(ctx)
	putContact(c, "Raider 11", "Air+FixedWing", 4.5, 2200)
	c.checkConflicts(ctx)
	if len(*sent) != 1 {
		t.Fatalf("flag on should call once: %q", *sent)
	}
}

func TestRunwayVacateChase(t *testing.T) {
	const tower = "Test Tower"
	lon, lat := offsetNm(0.3, 0)
	onField := func(c *ATCController, speedKts, altFt float64) {
		c.UpdateAnyPosition("Raider 11", "", "Air+FixedWing", lon, lat, altFt, speedKts, 0, 0)
	}
	agedClock := func(c *ATCController) {
		c.airfieldState.Get("Raider 11").GroundSlowSince = time.Now().Add(-2 * RunwayChaseDelay)
	}

	t.Run("stopped after landing is chased once", func(t *testing.T) {
		c, sent := proactiveController(t)
		ctx := context.Background()
		c.HandleRequest(ctx, ParseIntent("Test Tower, Raider 11, on final", tower))
		n := len(*sent)
		onField(c, 20, testFieldElevFt+5)
		c.checkRunwayVacate(ctx)
		if len(*sent) != n {
			t.Fatalf("first slow sighting only starts the clock: %q", *sent)
		}
		agedClock(c)
		onField(c, 20, testFieldElevFt+5)
		c.checkRunwayVacate(ctx)
		if len(*sent) != n+1 || !strings.Contains((*sent)[n], "runway") {
			t.Fatalf("want one runway chase, got %q", (*sent)[n:])
		}
		c.checkRunwayVacate(ctx)
		if len(*sent) != n+1 {
			t.Fatalf("chase must not repeat: %q", (*sent)[n:])
		}
	})

	t.Run("rollout speed never starts the clock", func(t *testing.T) {
		c, sent := proactiveController(t)
		ctx := context.Background()
		c.HandleRequest(ctx, ParseIntent("Test Tower, Raider 11, on final, touch and go", tower))
		n := len(*sent)
		onField(c, 90, testFieldElevFt+5)
		c.checkRunwayVacate(ctx)
		c.checkRunwayVacate(ctx)
		if ac := c.airfieldState.Get("Raider 11"); !ac.GroundSlowSince.IsZero() || len(*sent) != n {
			t.Fatalf("fast rollout started the chase: slowSince=%v sent=%q", ac.GroundSlowSince, (*sent)[n:])
		}
	})

	t.Run("no landing clearance, no chase", func(t *testing.T) {
		c, sent := proactiveController(t)
		ctx := context.Background()
		c.HandleRequest(ctx, ParseIntent("Test Tower, Raider 11, radio check", tower))
		n := len(*sent)
		onField(c, 10, testFieldElevFt+5)
		c.checkRunwayVacate(ctx)
		c.airfieldState.Get("Raider 11").GroundSlowSince = time.Now().Add(-2 * RunwayChaseDelay)
		c.checkRunwayVacate(ctx)
		if len(*sent) != n {
			t.Fatalf("aircraft never cleared to land was chased: %q", (*sent)[n:])
		}
	})

	t.Run("runway vacated ends it", func(t *testing.T) {
		c, sent := proactiveController(t)
		ctx := context.Background()
		c.HandleRequest(ctx, ParseIntent("Test Tower, Raider 11, on final", tower))
		onField(c, 20, testFieldElevFt+5)
		c.checkRunwayVacate(ctx)
		c.HandleRequest(ctx, ParseIntent("Test Tower, Raider 11, runway vacated", tower))
		n := len(*sent)
		onField(c, 20, testFieldElevFt+5)
		c.checkRunwayVacate(ctx)
		c.checkRunwayVacate(ctx)
		if len(*sent) != n {
			t.Fatalf("vacated aircraft was chased: %q", (*sent)[n:])
		}
	})
}
