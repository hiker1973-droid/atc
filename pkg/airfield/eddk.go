package airfield

import "github.com/paulmach/orb"

// EDDK is Köln/Bonn (Cologne) — Cold War Germany theatre (COMM1 recovery base).
// Name spoken as "Cologne" for English R/T; pilots may say "Köln" on the kneeboard
// preset — add a field-name alias if callsign matching misses it.
// See etar.go for data provenance and the magvar/threshold caveats.
var EDDK = &Airfield{
	ICAO:            "EDDK",
	Name:            "Cologne",
	DCSName:         "Koln/Bonn", // VERIFY exact ME name
	Center:          orb.Point{7.1427, 50.8659},
	ElevationFt:     242,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    119.70, // VHF tower; UHF tower 255.20
	ApproachFreqMHz: 119.70,
	ATISFreqMHz:     248.65, // UHF ATIS; simulcast VHF 128.32
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"14L": "left", // TODO verify
		"32R": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 14L/32R — TACAN 25X · ILS 110.90 (14L) / 111.10 (32R)
			Primary:    Runway{Designator: "14L", MagneticHeading: 137.0, ThresholdLatLon: orb.Point{7.1243, 50.8784}},
			Reciprocal: Runway{Designator: "32R", MagneticHeading: 317.0, ThresholdLatLon: orb.Point{7.1611, 50.8534}},
		},
	},
}
