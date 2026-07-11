package airfield

import "github.com/paulmach/orb"

// ETAD is Spangdahlem AB — Cold War Germany theatre (COMM1 recovery base).
// See etar.go for data provenance and the magvar/threshold caveats.
var ETAD = &Airfield{
	ICAO:            "ETAD",
	Name:            "Spangdahlem",
	DCSName:         "Spangdahlem", // VERIFY exact ME name
	Center:          orb.Point{6.6925, 49.9727},
	ElevationFt:     984,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    122.20, // VHF tower; UHF tower 251.80
	ApproachFreqMHz: 122.20,
	ATISFreqMHz:     248.05, // UHF ATIS; simulcast VHF 128.02
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"05": "left", // TODO verify
		"23": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 05/23 — TACAN 32X · ILS 109.15 (05) / 108.10 (23)
			Primary:    Runway{Designator: "05", MagneticHeading: 56.0, ThresholdLatLon: orb.Point{6.6747, 49.9650}},
			Reciprocal: Runway{Designator: "23", MagneticHeading: 236.0, ThresholdLatLon: orb.Point{6.7103, 49.9804}},
		},
	},
}
