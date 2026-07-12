package airfield

import "github.com/paulmach/orb"

// ORBR is Al Salam Airbase — Iraq theatre (vSFG-7 COMM1 recovery base).
// Tower UHF from the Iraq presets card + DCS radio table. The map has no
// beacon here, so center was derived by converting the field's DCS map
// coordinates (airdromeId 14, x≈3195 z≈24619) against the Baghdad TACAN
// anchor — approximate. ICAO (ORBR) and runway (09/27) are BEST GUESSES —
// VERIFY center, ICAO, and runway on the Iraq map before --position-check.
var ORBR = &Airfield{
	ICAO:            "ORBR",
	Name:            "Al Salam",
	DCSName:         "Al-Salam Airbase", // VERIFY exact ME name on the Iraq map
	Center:          orb.Point{44.4809, 33.2859},
	ElevationFt:     130,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.250, // UHF tower (COMM1 P12 / DCS default)
	ApproachFreqMHz: 250.250,
	ATISFreqMHz:     230.300, // COMM1 P13 (Al-Salam ATIS)
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four",
	BreakDirections: map[string]string{
		"09": "left", // TODO verify
		"27": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 09/27 — no beacon on the map; designator is a best guess.
			Primary:    Runway{Designator: "09", MagneticHeading: 90.0, ThresholdLatLon: orb.Point{44.4681, 33.2860}},
			Reciprocal: Runway{Designator: "27", MagneticHeading: 270.0, ThresholdLatLon: orb.Point{44.4939, 33.2860}},
		},
	},
}
