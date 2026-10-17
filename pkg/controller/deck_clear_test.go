package controller

import "testing"

// The carrier, escorts and jets parked on deck leave the deck clear; a jet
// flying in close (groove, bolter) fouls it (Day 2026-10-16: Charlie never came).
func TestIsDeckClear(t *testing.T) {
	parkedLon, parkedLat := offsetNm(0.05, 0)
	escortLon, escortLat := offsetNm(0.5, 0.3)
	c := deckTestController(t, map[string]*TacviewContact{
		"Raider 311": {Lon: parkedLon, Lat: parkedLat, AltFt: 60, SpeedKts: 25, ObjType: "Air+FixedWing"},
		"Escort 1":   {Lon: escortLon, Lat: escortLat, AltFt: 0, SpeedKts: 25, ObjType: "Sea+Watercraft"},
	})
	if !c.IsDeckClear() {
		t.Fatal("carrier, escort and a parked jet should leave the deck clear")
	}
	grooveLon, grooveLat := offsetNm(-0.6, 0)
	c = deckTestController(t, map[string]*TacviewContact{
		"Raider 302": {Lon: grooveLon, Lat: grooveLat, AltFt: 300, SpeedKts: 135, ObjType: "Air+FixedWing"},
	})
	if c.IsDeckClear() {
		t.Error("a jet in the groove should foul the deck")
	}
}
