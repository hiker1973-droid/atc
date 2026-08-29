package airfield

import "github.com/paulmach/orb"

// OJHR is H4 — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// The squadron card calls this H3; there is NO H3 on the Syria map -- H3 is an
// Iraqi field, off-map east. This is OJHR 'H-4 AB' (user ruling 2026-08-29). ⚠
// NO ILS, TACAN, VOR or NDB anywhere on the field: beacons.lua has nothing
// here, so its ATIS reports no approach aids. ⚠ DCS terrain tower is 250.10;
// the card says 252.250.
var OJHR = &Airfield{
	ICAO:            "OJHR",
	Name:            "H4",
	DCSName:         "H4",                          // VERIFY exact ME name on the Syria map
	Center:          orb.Point{38.20611, 32.53667}, // [lon, lat]
	ElevationFt:     2257,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    252.250,
	ApproachFreqMHz: 252.250,
	ATISFreqMHz:     240.850,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"10": "left", // TODO verify pattern side vs ramp in DCS
		"28": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "10", MagneticHeading: 106.0, ThresholdLatLon: orb.Point{38.19417, 32.54053}},
			Reciprocal: Runway{Designator: "28", MagneticHeading: 286.0, ThresholdLatLon: orb.Point{38.21805, 32.53280}},
		},
	},
}
