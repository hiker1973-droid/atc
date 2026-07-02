package airfield

import "github.com/paulmach/orb"

// UGKS is Senaki-Kolkhi, Georgia — Caucasus (Black Sea) theatre.
// Data from the vSFG-7 Black Sea AOR kneeboard (March 2026). Threshold coords
// are computed from field center + runway heading + length; verify against DCS
// before relying on the --position-check hold-short gate.
var UGKS = &Airfield{
	ICAO:            "UGKS",
	Name:            "Senaki",
	DCSName:         "Senaki-Kolkhi",
	Center:          orb.Point{42.04760, 42.24108}, // [lon, lat] — 31X (TSK) TACAN antenna
	ElevationFt:     43,
	MagVar:          6.5, // +6.5°E Caucasus — VERIFY vs DCS
	PatternAltFt:    1500,
	TowerFreqMHz:    261.000,
	ApproachFreqMHz: 261.000,
	ATISFreqMHz:     234.000,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"09": "left", // TODO verify pattern side against ramp location
		"27": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "09", MagneticHeading: 88.0, ThresholdLatLon: orb.Point{42.0342, 42.2419}},
			Reciprocal: Runway{Designator: "27", MagneticHeading: 268.0, ThresholdLatLon: orb.Point{42.0610, 42.2403}},
		},
	},
}
