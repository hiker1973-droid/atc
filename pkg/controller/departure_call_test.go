package controller

import (
	"context"
	"strings"
	"testing"
)

const airplane = "Air+FixedWing"

func departureController(t *testing.T) (*ATCController, *[]string, func(string)) {
	t.Helper()
	c, sent := proactiveController(t)
	c.SetDepartureCalls(true, "Safe skies, g'day")
	handle := func(text string) {
		t.Helper()
		req := ParseIntent(text, "Test Tower")
		if req == nil {
			t.Fatalf("ParseIntent(%q) = nil", text)
		}
		c.HandleRequest(context.Background(), req)
	}
	return c, sent, handle
}

func tick(c *ATCController) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkDepartures(context.Background())
}

// The tower calls the departure at seven miles; the pilot's later "seven DME"
// report gets "copy" (operator 2026-10-15).
func TestDepartureCallAtSevenMiles(t *testing.T) {
	c, sent, handle := departureController(t)
	handle("Test Tower, Raider 302, airborne")
	n := len(*sent)

	putContact(c, "Raider 302", airplane, 4, testFieldElevFt+3000)
	tick(c)
	if len(*sent) != n {
		t.Fatalf("called inside seven miles: %q", (*sent)[n:])
	}

	putContact(c, "Raider 302", airplane, 7.2, testFieldElevFt+5200)
	tick(c)
	if len(*sent) != n+1 {
		t.Fatalf("no departure call at 7.2 nm: %q", (*sent)[n:])
	}
	call := (*sent)[n]
	for _, want := range []string{"Raider 302", "Test Departure", "seven miles", "angels", "airspace", "g'day"} {
		if !strings.Contains(call, want) && !(want == "airspace" && strings.Contains(call, "zone")) {
			t.Errorf("departure call %q lacks %q", call, want)
		}
	}

	// Once only.
	putContact(c, "Raider 302", airplane, 9, testFieldElevFt+7000)
	tick(c)
	if len(*sent) != n+1 {
		t.Errorf("called twice: %q", (*sent)[n:])
	}

	// The pilot still reports seven DME: copy, not a second handoff.
	handle("Test Tower, Raider 302, seven DME")
	if got := (*sent)[len(*sent)-1]; got != "Raider 302, Test Departure, copy." {
		t.Errorf("late pilot report got %q, want copy", got)
	}
}

// A pilot who reports first gets the usual handoff, and the tower does not call again.
func TestDeparturePilotReportsFirst(t *testing.T) {
	c, sent, handle := departureController(t)
	handle("Test Tower, Raider 302, airborne")
	handle("Test Tower, Raider 302, seven DME")
	if got := (*sent)[len(*sent)-1]; strings.HasSuffix(got, "copy.") {
		t.Fatalf("first report got only %q, want the handoff", got)
	}
	n := len(*sent)
	putContact(c, "Raider 302", airplane, 7.5, testFieldElevFt+5000)
	tick(c)
	if len(*sent) != n {
		t.Errorf("tower called after the pilot's own report: %q", (*sent)[n:])
	}
}

// Cleared for takeoff and still on the ground, or not airborne: no call.
func TestDepartureCallNeedsAirborne(t *testing.T) {
	c, sent, handle := departureController(t)
	handle("Test Tower, Raider 302, airborne")
	n := len(*sent)
	putContact(c, "Raider 302", airplane, 7.5, testFieldElevFt+50)
	tick(c)
	if len(*sent) != n {
		t.Errorf("called a flight on the ground: %q", (*sent)[n:])
	}
}

func TestDepartureCallOff(t *testing.T) {
	c, sent, handle := departureController(t)
	c.SetDepartureCalls(false, "")
	handle("Test Tower, Raider 302, airborne")
	n := len(*sent)
	putContact(c, "Raider 302", airplane, 7.5, testFieldElevFt+5000)
	c.checkConflicts(context.Background()) // takes c.mu itself
	if len(*sent) != n {
		t.Errorf("flag off but tower called: %q", (*sent)[n:])
	}
}

// The common case: cleared for takeoff, never calls airborne. The real
// monitor tick makes the call.
func TestDepartureCallAfterTakeoffClearance(t *testing.T) {
	c, sent, handle := departureController(t)
	handle("Test Tower, Raider 302, request taxi")
	handle("Test Tower, Raider 302, holding short, ready for takeoff")
	// Line up and wait arms a 5 s auto-release; do the release here.
	c.airfieldState.ClearForTakeoff("Raider 302")
	if c.airfieldState.Get("Raider 302") == nil || len(c.airfieldState.ActiveDepartures()) != 1 {
		t.Fatalf("not cleared for takeoff: %q", *sent)
	}
	n := len(*sent)
	putContact(c, "Raider 302", airplane, 7.4, testFieldElevFt+4800)
	c.checkConflicts(context.Background())
	if len(*sent) <= n || !strings.Contains((*sent)[len(*sent)-1], "Test Departure") {
		t.Fatalf("no departure call after takeoff clearance: %q", (*sent)[n:])
	}
	t.Logf("sample: %s", (*sent)[len(*sent)-1])
}
