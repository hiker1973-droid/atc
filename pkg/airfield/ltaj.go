package airfield

import "github.com/paulmach/orb"

// LTAJ is Gaziantep — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// Chart gives RWY 10/28 with the ILS on 28 (109.10/286). The beacons.lua
// localizer direction of 106 is the 10 end. VOR/DME GAZ 116.70, NDB 432. ⚠ DCS
// terrain tower is 250.05; the card says 250.100. ATIS 249.400 is ASSIGNED --
// the card gives Gaziantep tower-only.
var LTAJ = &Airfield{
	ICAO:            "LTAJ",
	Name:            "Gaziantep",
	DCSName:         "Gaziantep",                   // VERIFY exact ME name on the Syria map
	Center:          orb.Point{37.47889, 36.94778}, // [lon, lat]
	ElevationFt:     2305,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    250.100,
	ApproachFreqMHz: 250.100,
	ATISFreqMHz:     249.400,
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
			Primary:    Runway{Designator: "10", MagneticHeading: 106.0, ThresholdLatLon: orb.Point{37.46315, 36.95261}},
			Reciprocal: Runway{Designator: "28", MagneticHeading: 286.0, ThresholdLatLon: orb.Point{37.49463, 36.94295}},
		},
	},
}
