package airfield

import "github.com/paulmach/orb"

// LTAG is Incirlik — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// ILS 05 109.30/055 and 23 111.70/235. TACAN DAN ch21. Tower matches the DCS
// terrain exactly (360.10) -- the only one of the eight that does.
var LTAG = &Airfield{
	ICAO:            "LTAG",
	Name:            "Incirlik",
	DCSName:         "Incirlik",                    // VERIFY exact ME name on the Syria map
	Center:          orb.Point{35.42583, 37.00194}, // [lon, lat]
	ElevationFt:     156,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    360.100,
	ApproachFreqMHz: 360.100,
	ATISFreqMHz:     360.200,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"05": "left", // TODO verify pattern side vs ramp in DCS
		"23": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "05", MagneticHeading: 55.0, ThresholdLatLon: orb.Point{35.41099, 36.99510}},
			Reciprocal: Runway{Designator: "23", MagneticHeading: 235.0, ThresholdLatLon: orb.Point{35.44068, 37.00879}},
		},
	},
}
