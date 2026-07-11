package airfield

import "github.com/paulmach/orb"

// EDDH is Hamburg — Cold War Germany theatre (COMM1 recovery base).
// See etar.go for data provenance and the magvar/threshold caveats.
var EDDH = &Airfield{
	ICAO:            "EDDH",
	Name:            "Hamburg",
	DCSName:         "Hamburg", // VERIFY exact ME name
	Center:          orb.Point{9.9882, 53.6304},
	ElevationFt:     73,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    126.85, // VHF tower; UHF tower 252.35
	ApproachFreqMHz: 126.85,
	ATISFreqMHz:     248.80, // UHF ATIS; simulcast VHF 128.40
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
			// 05/23 — VOR 115.80 · ILS 110.50 (05) / 111.50 (23)
			Primary:    Runway{Designator: "05", MagneticHeading: 59.0, ThresholdLatLon: orb.Point{9.9671, 53.6229}},
			Reciprocal: Runway{Designator: "23", MagneticHeading: 239.0, ThresholdLatLon: orb.Point{10.0093, 53.6379}},
		},
	},
}
