package controller

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
)

// Multi-ship flights (operator 2026-10-21): "Senaki traffic, flight of X
// FA-18s, Raider XX, XX and XX, holding short runway 27".
func TestParseFlight(t *testing.T) {
	cases := []struct {
		text     string
		callsign string
		typ      RequestType
		size     int
		wingmen  []string
		dist     int
	}{
		{"Test traffic, flight of three FA-18s, Raider 301, 302 and 303, holding short runway 27",
			"Raider 301", RequestHoldingShort, 3, []string{"Raider 302", "Raider 303"}, 0},
		{"Test Tower, Raider 301 flight of 2, ready for takeoff", "Raider 301", RequestTakeoffClear, 2, nil, 0},
		// "two" is the flight, not two miles (it was read as overhead).
		{"Test Tower, Raider 301, flight of two, inbound full stop", "Raider 301", RequestDistanceInitial, 2, nil, 0},
		{"Test Tower, Raider 301, two-ship, 10 miles east, inbound", "Raider 301", RequestDistanceInitial, 2, nil, 10},
		{"Test Tower, Venom 201, Venom 202, holding short", "Venom 201", RequestHoldingShort, 2, []string{"Venom 202"}, 0},
		{"Test Tower, Raider 301, 3-0-2 and 3-0-3, ready for takeoff", "Raider 301", RequestTakeoffClear, 3, []string{"Raider 302", "Raider 303"}, 0},
		// Single ships are untouched.
		{"Test Tower, Raider 302, holding short runway 27", "Raider 302", RequestHoldingShort, 0, nil, 0},
		{"Test Tower, Raider 301, 10 miles, inbound", "Raider 301", RequestDistanceInitial, 0, nil, 10},
		{"Test Tower, Raider 301, 3 mile initial", "Raider 301", RequestOverhead, 0, nil, 3},
	}
	for _, tc := range cases {
		req := ParseIntent(tc.text, "Test Tower")
		if req == nil {
			t.Errorf("%q: not addressed", tc.text)
			continue
		}
		if req.Callsign != tc.callsign || req.Type != tc.typ || req.FlightSize != tc.size ||
			!reflect.DeepEqual(req.Wingmen, tc.wingmen) || req.DistanceNm != tc.dist {
			t.Errorf("%q:\n got %q type %d size %d wingmen %v dist %d\nwant %q type %d size %d wingmen %v dist %d",
				tc.text, req.Callsign, req.Type, req.FlightSize, req.Wingmen, req.DistanceNm,
				tc.callsign, tc.typ, tc.size, tc.wingmen, tc.dist)
		}
	}
	if req := ParseIntent("Test traffic, flight of three FA-18s, Raider 301, 302 and 303, holding short", "Test Tower"); req.Airframe != "Hornet" {
		t.Errorf("FA-18s airframe %q, want Hornet", req.Airframe)
	}
}

// One takeoff clearance for the flight, wingmen answered as the flight, the
// next departure held longer, and one departure call at seven miles.
func TestFlightDeparture(t *testing.T) {
	c, sent, handle := departureController(t)
	last := func() string { return (*sent)[len(*sent)-1] }

	handle("Test traffic, flight of three FA-18s, Raider 301, 302 and 303, holding short runway 27")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, Test Tower, flight of three, ") {
		t.Errorf("hold short -> %q, want the flight addressed and 'flight of three'", got)
	}
	c.mu.Lock()
	for _, w := range []string{"Raider 302", "Raider 303"} {
		if c.airfieldState.Get(w) != nil {
			t.Errorf("%s has its own tower entry", w)
		}
	}
	c.mu.Unlock()

	handle("Test Tower, Raider 301 flight, ready for takeoff")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, Test Tower, ") || !(strings.Contains(got, "cleared for") || strings.Contains(got, "for departure")) {
		t.Errorf("takeoff -> %q", got)
	}
	if strings.Contains(last(), "flight of three") {
		t.Errorf("'flight of three' said twice: %q", last())
	}
	c.mu.Lock()
	ahead := time.Until(c.airfieldState.LastDepartureClearedAt)
	c.mu.Unlock()
	if ahead < 35*time.Second || ahead > 41*time.Second {
		t.Errorf("departure spacing pushed %v ahead, want 40s for two wingmen", ahead)
	}

	// A wingman's call is the flight's.
	handle("Test Tower, Raider 302, airborne")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, Test Tower, ") {
		t.Errorf("wingman airborne -> %q", got)
	}

	n := len(*sent)
	putContact(c, "Raider 301", airplane, 7.2, testFieldElevFt+5200)
	tick(c)
	if len(*sent) != n+1 || !strings.HasPrefix((*sent)[n], "Raider 301 flight, Test Departure") {
		t.Errorf("seven-mile call: %q", (*sent)[n:])
	}
}

// One landing clearance for the flight, and a spot for every jet.
func TestFlightArrivalParking(t *testing.T) {
	c, sent, handle := departureController(t)
	c.mu.Lock()
	c.airfieldState.Airfield.ParkingFirst, c.airfieldState.Airfield.ParkingLast = 1, 20
	c.mu.Unlock()
	last := func() string { return (*sent)[len(*sent)-1] }

	handle("Test Tower, Raider 301, flight of two, Raider 302, 10 miles, inbound full stop")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, Test Tower, flight of two, ") {
		t.Errorf("inbound -> %q", got)
	}
	handle("Test Tower, Raider 302, base, gear down")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, ") {
		t.Errorf("wingman base -> %q", got)
	}
	handle("Test Tower, Raider 301 flight, clear of the active")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, ") ||
		!strings.Contains(got, "Raider 301 spot 1") || !strings.Contains(got, "Raider 302 spot 2") {
		t.Errorf("clear of runway -> %q", got)
	}

	// Wingmen not named: the spots in a row.
	handle("Test Tower, Venom 201, flight of three, 8 miles, inbound")
	handle("Test Tower, Venom 201, clear of the runway")
	if got := last(); !strings.Contains(got, "spots 3, 4 and 5") {
		t.Errorf("unnamed wingmen -> %q", got)
	}
}

// A flight's wingmen are not called as unknown traffic.
func TestFlightWingmanNotUnknown(t *testing.T) {
	c, _, handle := departureController(t)
	handle("Test Tower, Raider 301, flight of three, Raider 302, 10 miles, inbound")
	putContact(c, "Raider 301", airplane, 5, testFieldElevFt+2000)
	w3 := putContact(c, "RAIDER 303", airplane, 5.5, testFieldElevFt+2000)
	far := putContact(c, "Venom 210", airplane, 9, testFieldElevFt+2000)
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	if !c.isFlightWingman("Raider 302", orb.Point{0, 0}) {
		t.Error("named wingman Raider 302 not recognised")
	}
	if !c.isFlightWingman("RAIDER 303", orb.Point{w3.Lon, w3.Lat}) {
		t.Error("unnamed third jet half a mile from the lead not recognised")
	}
	if c.isFlightWingman("Venom 210", orb.Point{far.Lon, far.Lat}) {
		t.Error("a stranger 4 nm away counted as a wingman")
	}
}

// Operator 2026-10-21: "also arrivals, flight of 2 full stop". The lead's
// "flight of two, full stop" clears the flight to land; the wingman, never
// named, calls final and is answered as the flight because he is flying with
// the lead. A jet of the same squadron miles away is not taken in.
func TestFlightOfTwoFullStop(t *testing.T) {
	c, sent, handle := departureController(t)
	last := func() string { return (*sent)[len(*sent)-1] }
	putContact(c, "Raider 301", airplane, 2, testFieldElevFt+800)
	putContact(c, "Raider 302", airplane, 2.3, testFieldElevFt+800)
	putContact(c, "Raider 310", airplane, 9, testFieldElevFt+3000)

	handle("Test Tower, Raider 301, flight of 2, full stop")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, ") || !strings.Contains(got, "flight of two") ||
		!strings.Contains(got, "cleared to land") {
		t.Errorf("flight of 2 full stop -> %q", got)
	}
	handle("Test Tower, Raider 302, final, gear down, full stop")
	if got := last(); !strings.HasPrefix(got, "Raider 301 flight, ") {
		t.Errorf("unnamed wingman on final -> %q, want the flight", got)
	}
	handle("Test Tower, Raider 310, 9 miles, inbound full stop")
	if got := last(); !strings.HasPrefix(got, "Raider 310, ") {
		t.Errorf("another Raider 9 nm out -> %q, want his own reply", got)
	}
	if f := c.flightOf("Raider 301"); f == nil || len(f.wingmen) != 1 || f.wingmen[0] != "Raider 302" {
		t.Errorf("flight %+v, want Raider 302 as the wingman", f)
	}
}
