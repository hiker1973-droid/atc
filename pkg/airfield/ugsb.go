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
	ElevationFt:     32,
	MagVar:          6.5, // +6.5°E Caucasus — VERIFY vs DCS
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
			Primary:    Runway{Designator: "13", MagneticHeading: 119.0, ThresholdLatLon: orb.Point{41.5991, 41.6087}},
			Reciprocal: Runway{Designator: "31", MagneticHeading: 299.0, ThresholdLatLon: orb.Point{41.6194, 41.5979}},
		},
	},
}
