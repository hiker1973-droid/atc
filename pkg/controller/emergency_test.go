package controller

import (
	"context"
	"strings"
	"testing"

	"github.com/paulmach/orb"
	"github.com/vsfg7/atc/pkg/airfield"
)

func emergencyController(t *testing.T) (*ATCController, *[]string, func(string)) {
	t.Helper()
	c := NewATCController("Senaki Tower", &airfield.Airfield{
		Name:        "Senaki",
		Center:      orb.Point{carLon, carLat},
		ElevationFt: testFieldElevFt,
		RunwayPairs: []airfield.RunwayPair{{
			Primary:    airfield.Runway{Designator: "09", MagneticHeading: 88},
			Reciprocal: airfield.Runway{Designator: "27", MagneticHeading: 268},
		}},
	})
	c.airfieldState.ActiveRunway = "27"
	sent := &[]string{}
	c.SetTransmitFn(func(_ context.Context, text string) { *sent = append(*sent, text) })
	handle := func(text string) {
		t.Helper()
		req := ParseIntent(text, "Senaki Tower")
		if req == nil {
			t.Fatalf("ParseIntent(%q) = nil", text)
		}
		c.HandleRequest(context.Background(), req)
	}
	return c, sent, handle
}

func last(sent *[]string) string {
	if len(*sent) == 0 {
		return ""
	}
	return (*sent)[len(*sent)-1]
}

// Day 2026-10-19 21:28, Venom 203 at Senaki with runway 27 active.
func TestEmergencyPilotsRunway(t *testing.T) {
	_, sent, handle := emergencyController(t)
	handle("Tower, Venom 203, declaring emergency, request landing on runway 09.")
	if got := last(sent); !strings.Contains(got, "runway zero niner") || strings.Contains(got, "two seven") {
		t.Errorf("emergency -> %q, want runway zero niner", got)
	}
	handle("Tower, Venom 203, one soul on board, fuel state zero.")
	got := last(sent)
	for _, want := range []string{"Venom 203, Senaki Tower, copy one soul on board, fuel zero", "crash crew", "runway zero niner, cleared to land"} {
		if !strings.Contains(got, want) {
			t.Errorf("souls and fuel -> %q, want %q", got, want)
		}
	}
	handle("Tower, Venom 203 on final runway 09.")
	if got := last(sent); !strings.Contains(got, "zero niner") || strings.Contains(got, "two seven") {
		t.Errorf("final -> %q, want runway zero niner", got)
	}
}

// No runway named: the active, as before; a non-emergency jet still gets the active.
func TestEmergencyNoRunwayNamed(t *testing.T) {
	_, sent, handle := emergencyController(t)
	handle("Tower, Venom 203, mayday, mayday, engine failure, 10 miles east.")
	if got := last(sent); !strings.Contains(got, "two seven") {
		t.Errorf("mayday -> %q, want the active", got)
	}
	handle("Tower, Raider 302, on final runway 09.")
	if got := last(sent); !strings.Contains(got, "two seven") {
		t.Errorf("routine final -> %q, want the active", got)
	}
}

func TestRequestedRunway(t *testing.T) {
	af := &airfield.Airfield{RunwayPairs: []airfield.RunwayPair{
		{Primary: airfield.Runway{Designator: "09"}, Reciprocal: airfield.Runway{Designator: "27"}},
		{Primary: airfield.Runway{Designator: "13L"}, Reciprocal: airfield.Runway{Designator: "31R"}},
	}}
	for in, want := range map[string]string{
		"request landing on runway 09":  "09",
		"runway 9":                      "09",
		"runway zero niner":             "09",
		"runway two seven":              "27",
		"runway 31 right":               "31R",
		"runway 13l":                    "13L",
		"runway 18":                     "",
		"declaring emergency, 10 miles": "",
	} {
		if got := requestedRunway(af, in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
