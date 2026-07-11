package airfield

import "github.com/paulmach/orb"

// EDDF is Frankfurt/Main — Cold War Germany theatre (COMM1 recovery base).
// See etar.go for data provenance and the magvar/threshold caveats.
var EDDF = &Airfield{
	ICAO:            "EDDF",
	Name:            "Frankfurt",
	DCSName:         "Frankfurt/Main", // VERIFY exact ME name
	Center:          orb.Point{8.5706, 50.0333},
	ElevationFt:     328,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    127.30, // VHF tower; UHF tower 251.85
	ApproachFreqMHz: 127.30,
	ATISFreqMHz:     248.60, // UHF ATIS; simulcast VHF 128.30
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"07L": "left", // TODO verify
		"25R": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 07L/25R — TACAN 89X · ILS 110.70 (07L) / 110.10 (25R)
			Primary:    Runway{Designator: "07L", MagneticHeading: 79.0, ThresholdLatLon: orb.Point{8.5431, 50.0299}},
			Reciprocal: Runway{Designator: "25R", MagneticHeading: 259.0, ThresholdLatLon: orb.Point{8.5981, 50.0367}},
		},
	},
}
