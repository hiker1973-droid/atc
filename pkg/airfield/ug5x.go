package airfield

import "github.com/paulmach/orb"

// UG5X is Kobuleti, Georgia — Caucasus (Black Sea) theatre.
// Data from the vSFG-7 Black Sea AOR kneeboard (March 2026). Threshold coords
// are computed from field center + runway heading + length; verify against DCS
// before relying on the --position-check hold-short gate.
var UG5X = &Airfield{
	ICAO:            "UG5X",
	Name:            "Kobuleti",
	DCSName:         "Kobuleti",
	Center:          orb.Point{41.87648, 41.93210}, // [lon, lat] — 67X (KBL) TACAN antenna
	ElevationFt:     59,
	MagVar:          6.5, // +6.5°E Caucasus — VERIFY vs DCS
	PatternAltFt:    1500,
	TowerFreqMHz:    262.000,
	ApproachFreqMHz: 262.000,
	ATISFreqMHz:     232.000,
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
			Primary:    Runway{Designator: "07", MagneticHeading: 63.0, ThresholdLatLon: orb.Point{41.8637, 41.9285}},
			Reciprocal: Runway{Designator: "25", MagneticHeading: 243.0, ThresholdLatLon: orb.Point{41.8892, 41.9357}},
		},
	},
}
