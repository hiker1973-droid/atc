package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
)

// callFrom is a pilot call heard from one SRS radio.
func callFrom(t *testing.T, c *ATCController, ctx context.Context, origin, text string) {
	t.Helper()
	req := ParseIntent(text, "Test Tower")
	if req == nil {
		t.Fatalf("ParseIntent(%q) = nil", text)
	}
	req.Origin = origin
	c.HandleRequest(ctx, req)
}

func recordReports(c *ATCController) *[]CommsEvent {
	evs := &[]CommsEvent{}
	c.SetReportFn(func(ev CommsEvent) { *evs = append(*evs, ev) })
	return evs
}

// isTakeoffClearance matches every ClearedForTakeoff wording ("cleared for
// takeoff", "clear for departure", "cleared for departure").
func isTakeoffClearance(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "cleared for") || strings.Contains(l, "clear for departure")
}

func hasKind(evs []CommsEvent, kind string) bool {
	for _, e := range evs {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// Senaki, 2026-10-15 18:33-18:40: Whisper heard Venom 201's taxi call as
// "Veta 201". The phantom headed the departure queue, Venom 201 was told to
// line up and wait and never released, and took off without a clearance.
// Same radio, so one aircraft: Venom 201 keeps the queue place and goes.
// ("Veta" itself is now repaired in normalizeCallsign, so a mishearing with
// no fix stands in for it here.)
func TestMisheardCallsignFromTheSameRadioIsOneAircraft(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, sent := proactiveController(t)
	evs := recordReports(c)
	s := c.airfieldState

	callFrom(t, c, ctx, "radioA", "Tower, Ventum 201, permission to taxi the active.")
	callFrom(t, c, ctx, "radioA", "Tower, Venom 201, holding short runway 27.")

	if s.Get("Ventum 201") != nil {
		t.Errorf("the misheard callsign is still tracked")
	}
	if n := s.DepartureQueueLen(); n != 1 {
		t.Fatalf("departure queue = %d, want the one jet", n)
	}
	if !hasKind(*evs, EventCallsignChanged) {
		t.Errorf("no comms report for the changed callsign: %+v", *evs)
	}
	s.Get("Venom 201").AutoReleaseAt = time.Now().Add(-time.Second) // past the LUAW gap
	c.checkConflicts(ctx)
	last := (*sent)[len(*sent)-1]
	if !strings.HasPrefix(last, "Venom 201") || !isTakeoffClearance(last) {
		t.Fatalf("Venom 201 was not released: %q", *sent)
	}
}

// A queue head that has only been cleared to taxi does not hold up a jet
// already waiting at the runway (different radios: two real aircraft).
func TestJetAtTheRunwayIsNotHeldBehindOneStillTaxiing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, sent := proactiveController(t)

	callFrom(t, c, ctx, "radioA", "Tower, Raider 11, request taxi.")
	callFrom(t, c, ctx, "radioB", "Tower, Venom 21, holding short runway 27.")
	c.airfieldState.Get("Venom 21").AutoReleaseAt = time.Now().Add(-time.Second)
	c.checkConflicts(ctx)
	last := (*sent)[len(*sent)-1]
	if !strings.HasPrefix(last, "Venom 21") || !isTakeoffClearance(last) {
		t.Fatalf("Venom 21 held behind a taxiing jet: %q", *sent)
	}
	if c.airfieldState.Get("Raider 11") == nil || c.airfieldState.DeparturePosition("Raider 11") != 1 {
		t.Errorf("Raider 11 should still be queued")
	}
}

// "Venom flight, request departure" (18:40:02) went unanswered.
func TestFlightNameWithoutNumber(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("unknown radio is asked for the full callsign", func(t *testing.T) {
		c, sent := proactiveController(t)
		evs := recordReports(c)
		callFrom(t, c, ctx, "radioZ", "Tower, Venom flight, request departure.")
		if len(*sent) != 1 || !strings.HasPrefix((*sent)[0], "Venom flight, Test Tower") || !strings.Contains((*sent)[0], "full callsign") {
			t.Fatalf("want a full-callsign request, got %q", *sent)
		}
		if !hasKind(*evs, EventFlightOnly) {
			t.Errorf("not reported: %+v", *evs)
		}
	})

	t.Run("radio that gave its callsign is that jet", func(t *testing.T) {
		c, sent := proactiveController(t)
		callFrom(t, c, ctx, "radioA", "Tower, Venom 201, request taxi.")
		callFrom(t, c, ctx, "radioA", "Tower, Venom flight, holding short runway 27.")
		last := (*sent)[len(*sent)-1]
		if !strings.HasPrefix(last, "Venom 201") {
			t.Fatalf("flight call not answered as Venom 201: %q", *sent)
		}
	})

	// Operator 2026-10-15: "Tower needs to tell them repeat or something stern."
	t.Run("no name at all is told to say again with full callsign", func(t *testing.T) {
		c, sent := proactiveController(t)
		evs := recordReports(c)
		callFrom(t, c, ctx, "radioZ", "Test traffic, clear active 09.")
		if len(*sent) != 1 || !strings.Contains((*sent)[0], "full callsign") ||
			!strings.HasPrefix((*sent)[0], "Station calling") && !strings.HasPrefix((*sent)[0], "Aircraft calling") {
			t.Fatalf("want one 'say again with full callsign' to the unknown station: %q", *sent)
		}
		if !hasKind(*evs, EventNoCallsign) {
			t.Errorf("not reported: %+v", *evs)
		}
		callFrom(t, c, ctx, "radioZ", "Test traffic, clear active 09.")
		if len(*sent) != 1 {
			t.Errorf("asked again within %v: %q", noCallsignReplyGap, *sent)
		}
	})

	t.Run("a bare number is addressed by that number", func(t *testing.T) {
		c, sent := proactiveController(t)
		callFrom(t, c, ctx, "radioY", "Tower, 201 airborne.")
		if len(*sent) != 1 || !strings.HasPrefix((*sent)[0], "Aircraft 201, Test Tower,") || !strings.Contains((*sent)[0], "full callsign") {
			t.Fatalf("Senaki 19:57 'Tower, 201 airborne.': %q", *sent)
		}
		if a := c.airfieldState.Get("201"); a != nil {
			t.Errorf("the call was acted on: tracked %+v", a)
		}
	})

	t.Run("a known radio is addressed by its callsign and not acted on", func(t *testing.T) {
		c, sent := proactiveController(t)
		callFrom(t, c, ctx, "radioX", "Tower, Venom 201, request taxi.")
		n := len(*sent)
		callFrom(t, c, ctx, "radioX", "Tower, 201 airborne.")
		if len(*sent) != n+1 || !strings.HasPrefix((*sent)[n], "Venom 201, Test Tower,") || !strings.Contains((*sent)[n], "full callsign") {
			t.Fatalf("want Venom 201 told to use the full callsign: %q", (*sent)[n:])
		}
	})
}

func TestDepartedWithoutClearance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("traffic on short final: reported and told", func(t *testing.T) {
		c, sent := proactiveController(t)
		evs := recordReports(c)
		s := c.airfieldState
		inbound := s.GetOrCreate("Raider 31")
		s.EnqueueLanding(inbound)
		lon, lat := offsetNm(2, 0)
		s.UpdatePosition("Raider 31", orb.Point{lon, lat}, 1000, 150)

		callFrom(t, c, ctx, "radioA", "Tower, Venom 21, holding short runway 27.")
		callFrom(t, c, ctx, "radioA", "Tower, Venom 21, airborne.")
		if !hasKind(*evs, EventDepartedNoClear) {
			t.Fatalf("not reported: %+v", *evs)
		}
		joined := strings.Join(*sent, " | ")
		if !strings.Contains(joined, "no takeoff clearance") {
			t.Errorf("not told with traffic on final: %q", *sent)
		}
	})

	t.Run("no traffic: reported only", func(t *testing.T) {
		c, sent := proactiveController(t)
		evs := recordReports(c)
		c.airfieldState.LastDepartureClearedAt = time.Now() // held for spacing
		callFrom(t, c, ctx, "radioA", "Tower, Venom 21, holding short runway 27.")
		callFrom(t, c, ctx, "radioA", "Tower, Venom 21, airborne.")
		if !hasKind(*evs, EventDepartedNoClear) {
			t.Fatalf("not reported: %+v", *evs)
		}
		if strings.Contains(strings.Join(*sent, " | "), "no takeoff clearance") {
			t.Errorf("spoken without traffic: %q", *sent)
		}
	})

	t.Run("cleared jet is not reported", func(t *testing.T) {
		c, _ := proactiveController(t)
		evs := recordReports(c)
		callFrom(t, c, ctx, "radioA", "Tower, Venom 21, holding short runway 27.")
		c.airfieldState.Get("Venom 21").AutoReleaseAt = time.Now().Add(-time.Second)
		c.checkConflicts(ctx) // cleared for takeoff
		callFrom(t, c, ctx, "radioA", "Tower, Venom 21, airborne.")
		if hasKind(*evs, EventDepartedNoClear) {
			t.Errorf("a cleared departure was reported: %+v", *evs)
		}
	})
}

func TestLandedWithoutClearanceIsReported(t *testing.T) {
	ctx := context.Background()
	c, _ := proactiveController(t)
	evs := recordReports(c)
	callFrom(t, c, ctx, "radioA", "Tower, Raider 31, overhead.")
	callFrom(t, c, ctx, "radioA", "Tower, Raider 31, clear of the active.")
	if !hasKind(*evs, EventLandedNoClear) {
		t.Fatalf("not reported: %+v", *evs)
	}
}

// putTraffic puts an airborne contact northNm/eastNm of the field.
func putTraffic(c *ATCController, key string, northNm, eastNm, altFt, hdg, kts float64) {
	lon, lat := offsetNm(northNm, eastNm)
	c.allPositions[key] = &TacviewContact{Callsign: key, ObjType: "Air+FixedWing", Lon: lon, Lat: lat,
		AltFt: altFt, HeadingDeg: hdg, SpeedKts: kts, UpdatedAt: time.Now()}
	c.MarkPlayer(key)
}

func TestTrafficAlerts(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		place func(c *ATCController)
		alert bool
	}{
		{"head-on, same altitude", func(c *ATCController) {
			putTraffic(c, "RAIDER 031", 3, 0, 3000, 90, 300)
			putTraffic(c, "VENOM 201", 3, 1.2, 3000, 270, 300)
		}, true},
		{"crossing at right angles", func(c *ATCController) {
			putTraffic(c, "RAIDER 031", 3, -0.8, 3000, 90, 300)
			putTraffic(c, "VENOM 201", 2.2, 0, 3200, 0, 300)
		}, true},
		{"head-on, 2,000 ft apart", func(c *ATCController) {
			putTraffic(c, "RAIDER 031", 3, 0, 3000, 90, 300)
			putTraffic(c, "VENOM 201", 3, 1.2, 5000, 270, 300)
		}, false},
		{"formation", func(c *ATCController) {
			putTraffic(c, "VENOM 201", 3, 0, 3000, 90, 300)
			putTraffic(c, "VENOM 203", 3.02, -0.02, 3000, 90, 300)
		}, false},
		{"rejoin from behind", func(c *ATCController) {
			putTraffic(c, "VENOM 201", 3, 0, 3000, 90, 250)
			putTraffic(c, "VENOM 203", 3.1, -0.5, 3000, 80, 400)
		}, false},
		{"diverging", func(c *ATCController) {
			putTraffic(c, "RAIDER 031", 3, 0, 3000, 270, 300)
			putTraffic(c, "VENOM 201", 3, 0.5, 3000, 90, 300)
		}, false},
		{"downwind against initial, a mile apart", func(c *ATCController) {
			putTraffic(c, "RAIDER 031", 0, 1.0, 1500, 270, 300)
			putTraffic(c, "VENOM 201", 0, -1.0, 1500, 90, 350)
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, sent := proactiveController(t)
			evs := recordReports(c)
			c.SetTrafficAlerts(true)
			tc.place(c)
			c.checkConflicts(ctx)
			got := hasKind(*evs, EventNearMiss)
			if got != tc.alert {
				t.Fatalf("alert = %v, want %v (sent %q)", got, tc.alert, *sent)
			}
			if !tc.alert {
				return
			}
			if len(*sent) != 2 || !strings.Contains((*sent)[0], "traffic alert") {
				t.Fatalf("want a traffic alert to each jet, got %q", *sent)
			}
			c.checkConflicts(ctx)
			if len(*sent) != 2 {
				t.Errorf("alert repeated inside the cooldown: %q", *sent)
			}
		})
	}
}

func TestTrafficAlertWording(t *testing.T) {
	ctx := context.Background()
	c, sent := proactiveController(t)
	c.SetTrafficAlerts(true)
	putTraffic(c, "RAIDER 031", 3, 0, 3000, 90, 300)
	putTraffic(c, "VENOM 201", 3, 1.2, 3000, 270, 300)
	c.checkConflicts(ctx)
	want := map[string]bool{
		"Raider 031, Test Tower, traffic alert, twelve o'clock, one mile, same altitude.": true,
		"Venom 201, Test Tower, traffic alert, twelve o'clock, one mile, same altitude.":  true,
	}
	for _, s := range *sent {
		if !want[s] {
			t.Errorf("unexpected alert %q", s)
		}
	}
}

// A player whose Tacview name is not a callsign is reported but not called.
func TestTrafficAlertOnlyCallsAddressableJets(t *testing.T) {
	ctx := context.Background()
	c, sent := proactiveController(t)
	evs := recordReports(c)
	c.SetTrafficAlerts(true)
	putTraffic(c, "RAIDER 031", 3, 0, 3000, 90, 300)
	putTraffic(c, "406-B.P-CVW5", 3, 1.2, 3000, 270, 300)
	c.checkConflicts(ctx)
	if !hasKind(*evs, EventNearMiss) || len(*sent) != 1 || !strings.HasPrefix((*sent)[0], "Raider 031") {
		t.Fatalf("want one call to Raider 031 and a report, got %q / %+v", *sent, *evs)
	}
}

// A call Whisper handed back as its own prompt goes to the comms report.
func TestLostCallIsReported(t *testing.T) {
	c, sent := proactiveController(t)
	evs := recordReports(c)
	c.ReportLostCall("Senaki Tower, Raider, Venom, radio check, request taxi", "lost")
	if !hasKind(*evs, EventLostCall) || (*evs)[0].Text == "" {
		t.Errorf("lost call not reported: %+v", *evs)
	}
	if len(*sent) != 0 {
		t.Errorf("a lost call must not be answered: %q", *sent)
	}
}
