package airfield

import "github.com/paulmach/orb"

// UGSB is Batumi, Georgia — Caucasus (Black Sea) theatre.
// Data from the vSFG-7 Black Sea AOR kneeboard (March 2026). Threshold coords
// are computed from field center + runway heading + length; verify against DCS
// before relying on the --position-check hold-short gate.
//
// NOTE: two kneeboard discrepancies resolved here — the AOR card lists tower
// 261.000 (a duplicate of Senaki) and ILS "09/108.90" (copied from Senaki);
// the presets card gives the real tower 260.000, and Batumi's ILS is on 13.
var UGSB = &Airfield{
	ICAO:            "UGSB",
	Name:            "Batumi",
	DCSName:         "Batumi",
	Center:          orb.Point{41.60927, 41.60327}, // [lon, lat] — 16X (BTM) TACAN antenna
	ElevationFt:     33, // DimOn Aerodrome Data 01 Feb 2026 (was 32)
	MagVar:          7.5, // +7.5°E, DimOn 2020s value for the Caucasus map (operator 2026-09-19); wind, BRC, bearings
	PatternAltFt:    1500,
	TowerFreqMHz:    260.000, // presets card (AOR card's 261 is a typo)
	ApproachFreqMHz: 260.000,
	ATISFreqMHz:     230.000,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // Command is COMM1 P1 on Black Sea (was ch4 on PG)
	BreakDirections: map[string]string{
		"13": "left", // TODO verify pattern side against ramp location
		"31": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 2026-11-07: thresholds rotated about the runway midpoint by the Caucasus map's grid convergence
			// ((lon - 33E) * sin(lat), 5.7 deg here). The points had been laid off the DCS yaw; DCS's grid is that
			// far off true north, which at Senaki (refit from 1648 Tacview ground-roll fixes) put the old points
			// 60-80 m off the runway. The same method reproduces the Senaki refit's bearing (100.7). NOT yet checked
			// against a Tacview recording at this field.
			Primary:    Runway{Designator: "13", MagneticHeading: 119.0, ThresholdLatLon: orb.Point{41.59986, 41.60943}},
			Reciprocal: Runway{Designator: "31", MagneticHeading: 299.0, ThresholdLatLon: orb.Point{41.61864, 41.59717}},
		},
	},
}
