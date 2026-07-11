package airfield

import "github.com/paulmach/orb"

// EDDV is Hannover — Cold War Germany theatre (COMM1 recovery base).
// See etar.go for data provenance and the magvar/threshold caveats.
var EDDV = &Airfield{
	ICAO:            "EDDV",
	Name:            "Hannover",
	DCSName:         "Hannover", // VERIFY exact ME name
	Center:          orb.Point{9.6851, 52.4611},
	ElevationFt:     168,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    120.20, // VHF tower; UHF tower 252.45
	ApproachFreqMHz: 120.20,
	ATISFreqMHz:     248.85, // UHF ATIS; simulcast VHF 128.42
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"09": "left", // TODO verify
		"27": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 09/27 — no TACAN · ILS 109.50 (09) / 108.70 (27)
			Primary:    Runway{Designator: "09", MagneticHeading: 101.0, ThresholdLatLon: orb.Point{9.6576, 52.4644}},
			Reciprocal: Runway{Designator: "27", MagneticHeading: 281.0, ThresholdLatLon: orb.Point{9.7126, 52.4578}},
		},
	},
}
