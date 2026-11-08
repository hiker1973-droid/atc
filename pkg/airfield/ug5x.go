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
	MagVar:          7.5, // +7.5°E, DimOn 2020s value for the Caucasus map (operator 2026-09-19); wind, BRC, bearings
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
			// 2026-11-07: thresholds rotated about the runway midpoint by the Caucasus map's grid convergence
			// ((lon - 33E) * sin(lat), 5.9 deg here). The points had been laid off the DCS yaw; DCS's grid is that
			// far off true north, which at Senaki (refit from 1648 Tacview ground-roll fixes) put the old points
			// 60-80 m off the runway. The same method reproduces the Senaki refit's bearing (100.7). NOT yet checked
			// against a Tacview recording at this field.
			Primary:    Runway{Designator: "07", MagneticHeading: 63.0, ThresholdLatLon: orb.Point{41.86327, 41.92951}},
			Reciprocal: Runway{Designator: "25", MagneticHeading: 243.0, ThresholdLatLon: orb.Point{41.88963, 41.93469}},
		},
	},
}
