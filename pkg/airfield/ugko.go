package airfield

import "github.com/paulmach/orb"

// UGKO is Kutaisi (Kopitnari), Georgia — Caucasus (Black Sea) theatre.
// Data from the vSFG-7 Black Sea AOR kneeboard (March 2026). Threshold coords
// are computed from field center + runway heading + length; verify against DCS
// before relying on the --position-check hold-short gate.
//
// 2026-09-19: headings 074/254 -> 067/247 magnetic and elevation 148 ft per
// DimOn Aerodrome Data 01 Feb 2026 (074 was the TRUE bearing; magvar ~+7).
var UGKO = &Airfield{
	ICAO:            "UGKO",
	Name:            "Kutaisi",
	DCSName:         "Kutaisi",
	Center:          orb.Point{42.49568, 42.17915}, // [lon, lat] — 44X (KTS) TACAN antenna
	ElevationFt:     148,
	MagVar:          6.5, // +6.5°E Caucasus — VERIFY vs DCS
	PatternAltFt:    1500,
	TowerFreqMHz:    263.000,
	ApproachFreqMHz: 263.000,
	ATISFreqMHz:     233.000,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"07": "left", // TODO verify pattern side against ramp location
		"25": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "07", MagneticHeading: 67.0, ThresholdLatLon: orb.Point{42.4812, 42.1774}},
			Reciprocal: Runway{Designator: "25", MagneticHeading: 247.0, ThresholdLatLon: orb.Point{42.5101, 42.1809}},
		},
	},
}
