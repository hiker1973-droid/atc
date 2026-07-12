package airfield

import "github.com/paulmach/orb"

// ORSH is Al Sahra Airfield (Tikrit / Camp Speicher) — Iraq theatre
// (vSFG-7 COMM1 recovery base). Tower UHF from the Iraq presets card + DCS
// radio table. Center/elevation from the field NDB (TKT, 1669 kHz). The map
// has no ILS/TACAN here, so the runway designator (15/33) is a best guess —
// VERIFY the exact runway on the Iraq map. Thresholds computed from center.
var ORSH = &Airfield{
	ICAO:            "ORSH",
	Name:            "Al Sahra",
	DCSName:         "Al-Sahra Airport", // VERIFY exact ME name on the Iraq map
	Center:          orb.Point{43.5393, 34.6662},
	ElevationFt:     420,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.150, // UHF tower (COMM1 P10 / DCS default)
	ApproachFreqMHz: 250.150,
	ATISFreqMHz:     230.200, // COMM1 P11 (Al-Sahra ATIS)
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four",
	BreakDirections: map[string]string{
		"15": "left", // TODO verify
		"33": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 15/33 — no ILS/TACAN on the map; designator is a best guess. NDB TKT.
			Primary:    Runway{Designator: "15", MagneticHeading: 150.0, ThresholdLatLon: orb.Point{43.5327, 34.6755}},
			Reciprocal: Runway{Designator: "33", MagneticHeading: 330.0, ThresholdLatLon: orb.Point{43.5459, 34.6569}},
		},
	},
}
