package controller

import (
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Carrier steaming due north at 26.5N/56.0E — a convenient reference point in
// the Gulf. offsetNm places a contact a given distance north/east of it.
const (
	carLat = 26.5
	carLon = 56.0
	carBRC = 0.0
)

func offsetNm(northNm, eastNm float64) (lon, lat float64) {
	lat = carLat + northNm/60.0
	lon = carLon + eastNm/60.0/0.894 // cos(26.5°) ≈ 0.894
	return lon, lat
}

func deckTestController(t *testing.T, contacts map[string]*TacviewContact) *ATCController {
	t.Helper()
	c := NewATCController("Deckboss", &airfield.Airfield{})
	c.allPositions = map[string]*TacviewContact{
		"CVN-72 ABE": {Lon: carLon, Lat: carLat, HeadingDeg: carBRC, UpdatedAt: time.Now()},
	}
	for cs, ct := range contacts {
		ct.UpdatedAt = time.Now()
		c.allPositions[cs] = ct
	}
	return c
}

// An aircraft spotted toward the bow must read as forward of the carrier point
// so Deckboss steers it to a bow cat.
func TestDeckSpotNm_ForwardIsPositive(t *testing.T) {
	lon, lat := offsetNm(0.05, 0)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: lon, Lat: lat, AltFt: 60},
	})
	foreAft, _, onDeck := c.DeckSpotNm("Raider 311")
	if !onDeck {
		t.Fatal("aircraft 0.05nm from the boat at 60ft should read as on deck")
	}
	if foreAft <= 0 {
		t.Errorf("aircraft toward the bow should be forward, got foreAftNm %.3f", foreAft)
	}
}

func TestDeckSpotNm_AftIsNegative(t *testing.T) {
	lon, lat := offsetNm(-0.05, 0)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: lon, Lat: lat, AltFt: 60},
	})
	foreAft, _, onDeck := c.DeckSpotNm("Raider 311")
	if !onDeck {
		t.Fatal("aircraft 0.05nm astern at 60ft should read as on deck")
	}
	if foreAft >= 0 {
		t.Errorf("aircraft toward the round-down should be aft, got foreAftNm %.3f", foreAft)
	}
}

// Fore/aft is measured along the BRC, not along true north — a carrier steaming
// east must classify an aircraft to its east as forward.
func TestDeckSpotNm_ForeAftFollowsBRC(t *testing.T) {
	lon, lat := offsetNm(0, 0.05)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: lon, Lat: lat, AltFt: 60},
	})
	c.allPositions["CVN-72 ABE"].HeadingDeg = 90
	foreAft, _, onDeck := c.DeckSpotNm("Raider 311")
	if !onDeck {
		t.Fatal("expected on-deck contact")
	}
	if foreAft <= 0 {
		t.Errorf("with BRC 090 an aircraft to the east is forward, got foreAftNm %.3f", foreAft)
	}
}

func TestDeckSpotNm_AirborneIsNotOnDeck(t *testing.T) {
	lon, lat := offsetNm(0.05, 0)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: lon, Lat: lat, AltFt: 1200},
	})
	if _, _, onDeck := c.DeckSpotNm("Raider 311"); onDeck {
		t.Error("aircraft at 1200ft over the boat must not read as on deck")
	}
}

func TestDeckSpotNm_DistantAircraftIsNotOnDeck(t *testing.T) {
	lon, lat := offsetNm(5, 0)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: lon, Lat: lat, AltFt: 60},
	})
	if _, _, onDeck := c.DeckSpotNm("Raider 311"); onDeck {
		t.Error("aircraft 5nm from the boat must not read as on deck")
	}
}

// The fail-open contract: an unknown callsign reports onDeck=false rather than
// panicking, so the caller falls back to first-free assignment.
func TestDeckSpotNm_UnknownCallsign(t *testing.T) {
	c := deckTestController(t, nil)
	if _, _, onDeck := c.DeckSpotNm("Ghost 01"); onDeck {
		t.Error("unknown callsign must not report on deck")
	}
}

func TestDeckSpotNm_NoCarrier(t *testing.T) {
	c := NewATCController("Deckboss", &airfield.Airfield{})
	c.allPositions = map[string]*TacviewContact{
		"Raider 311": {Lon: carLon, Lat: carLat, AltFt: 60, UpdatedAt: time.Now()},
	}
	if _, _, onDeck := c.DeckSpotNm("Raider 311"); onDeck {
		t.Error("without a carrier contact nothing can be placed on deck")
	}
}

// Stale telemetry must not be trusted to place an aircraft on a catapult.
func TestDeckSpotNm_StaleContact(t *testing.T) {
	lon, lat := offsetNm(0.05, 0)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: lon, Lat: lat, AltFt: 60},
	})
	c.allPositions["Raider 311"].UpdatedAt = time.Now().Add(-2 * time.Minute)
	if _, _, onDeck := c.DeckSpotNm("Raider 311"); onDeck {
		t.Error("contact stale by 2 minutes must not read as on deck")
	}
}

func TestGetDeckContacts_SortedBowFirstExcludingCarrier(t *testing.T) {
	aftLon, aftLat := offsetNm(-0.06, 0)
	fwdLon, fwdLat := offsetNm(0.06, 0)
	farLon, farLat := offsetNm(12, 0)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: aftLon, Lat: aftLat, AltFt: 60},
		"Raider 312": {Lon: fwdLon, Lat: fwdLat, AltFt: 60},
		"Raider 313": {Lon: farLon, Lat: farLat, AltFt: 60},
	})
	got := c.GetDeckContacts()
	if len(got) != 2 {
		t.Fatalf("expected the 2 on-deck aircraft (carrier and 12nm contact excluded), got %d: %+v", len(got), got)
	}
	if got[0].Callsign != "Raider 312" {
		t.Errorf("expected the bow-most aircraft first, got %q", got[0].Callsign)
	}
}
