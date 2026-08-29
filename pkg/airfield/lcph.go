package airfield

import "github.com/paulmach/orb"

// LCPH is Paphos — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// ILS on 29 (108.90/294). TACAN PHA ch79, VOR/DME PHA 117.90, NDB 328. ⚠ Tower
// 249.100 is a vSFG-7 MOVE: the card prints 252.100, which collides with
// TEXACO 1. The tanker keeps 252.100 (squadron-wide standard) and Paphos moved
// beside its own ATIS on 249.000. ⚠ DCS terrain tower is 251.80.
var LCPH = &Airfield{
	ICAO:            "LCPH",
	Name:            "Paphos",
	DCSName:         "Paphos",                      // VERIFY exact ME name on the Syria map
	Center:          orb.Point{32.48556, 34.71861}, // [lon, lat]
	ElevationFt:     40,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    249.100,
	ApproachFreqMHz: 249.100,
	ATISFreqMHz:     249.000,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"11": "left", // TODO verify pattern side vs ramp in DCS
		"29": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "11", MagneticHeading: 114.0, ThresholdLatLon: orb.Point{32.47265, 34.72449}},
			Reciprocal: Runway{Designator: "29", MagneticHeading: 294.0, ThresholdLatLon: orb.Point{32.49846, 34.71273}},
		},
	},
}
