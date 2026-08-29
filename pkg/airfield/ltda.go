package airfield

import "github.com/paulmach/orb"

// LTDA is Hatay — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// ILS 04 108.90/044 and 22 108.15/224. VOR/DME HTY 112.05, NDB HTY 336. ⚠ DCS
// terrain tower is 250.25; the card says 250.300. ATIS 249.300 is ASSIGNED --
// the card gives Hatay tower-only.
var LTDA = &Airfield{
	ICAO:            "LTDA",
	Name:            "Hatay",
	DCSName:         "Hatay",                       // VERIFY exact ME name on the Syria map
	Center:          orb.Point{36.28500, 36.36028}, // [lon, lat]
	ElevationFt:     253,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    250.300,
	ApproachFreqMHz: 250.300,
	ATISFreqMHz:     249.300,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"04": "left", // TODO verify pattern side vs ramp in DCS
		"22": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "04", MagneticHeading: 44.0, ThresholdLatLon: orb.Point{36.27279, 36.35173}},
			Reciprocal: Runway{Designator: "22", MagneticHeading: 224.0, ThresholdLatLon: orb.Point{36.29721, 36.36882}},
		},
	},
}
