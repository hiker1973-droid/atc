package airfield

import "github.com/paulmach/orb"

// EDFH is Hahn (Frankfurt-Hahn) — Cold War Germany theatre (COMM1 recovery base).
// See etar.go for data provenance and the magvar/threshold caveats.
var EDFH = &Airfield{
	ICAO:            "EDFH",
	Name:            "Hahn",
	DCSName:         "Hahn", // VERIFY exact ME name
	Center:          orb.Point{7.2639, 49.9487},
	ElevationFt:     1621,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    119.50, // VHF tower; UHF tower 251.40
	ApproachFreqMHz: 119.50,
	ATISFreqMHz:     248.15, // UHF ATIS; simulcast VHF 128.07
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"03": "left", // TODO verify
		"21": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 03/21 — TACAN 24X · ILS 109.30 (03) / 111.30 (21)
			Primary:    Runway{Designator: "03", MagneticHeading: 44.0, ThresholdLatLon: orb.Point{7.2492, 49.9389}},
			Reciprocal: Runway{Designator: "21", MagneticHeading: 224.0, ThresholdLatLon: orb.Point{7.2786, 49.9585}},
		},
	},
}
