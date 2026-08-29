package airfield

import "github.com/paulmach/orb"

// LCRA is Akrotiri — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// ⚠ RUNWAY RESOLVED FROM THE CHART, NOT THE LOCALIZER. beacons.lua gives the
// IAK localizer 111 true, which reads as RWY 11; the surveyed chart and the
// real RAF Akrotiri are both 10/28, with the ILS on 28 (109.70/291). The chart
// wins. TACAN AK ch107, DME 116.00. ⚠ DCS terrain tower is 251.70; the card
// says 252.000. ATIS 249.500 is ASSIGNED -- the card gives Akrotiri
// tower-only.
var LCRA = &Airfield{
	ICAO:            "LCRA",
	Name:            "Akrotiri",
	DCSName:         "Akrotiri",                    // VERIFY exact ME name on the Syria map
	Center:          orb.Point{32.98806, 34.59056}, // [lon, lat]
	ElevationFt:     76,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    252.000,
	ApproachFreqMHz: 252.000,
	ATISFreqMHz:     249.500,
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
			Primary:    Runway{Designator: "10", MagneticHeading: 106.0, ThresholdLatLon: orb.Point{32.97408, 34.59497}},
			Reciprocal: Runway{Designator: "28", MagneticHeading: 286.0, ThresholdLatLon: orb.Point{33.00203, 34.58614}},
		},
	},
}
