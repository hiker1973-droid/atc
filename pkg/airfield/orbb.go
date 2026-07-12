package airfield

import "github.com/paulmach/orb"

// ORBB is Bashur Airfield (Harir) — Iraq theatre (vSFG-7 COMM1 field, tower
// only — no dedicated ATIS on the presets card). Tower UHF from the card + DCS
// radio table. Center/elevation from the field TACAN (ORB, ch 47). No ILS on
// the map, so the runway designator (13/31) is a best guess — VERIFY. ICAO
// (ORBB) is a best guess (map TACAN ident is "ORB").
var ORBB = &Airfield{
	ICAO:            "ORBB",
	Name:            "Bashur",
	DCSName:         "Bashur Airport", // VERIFY exact ME name on the Iraq map
	Center:          orb.Point{44.3447, 36.5314},
	ElevationFt:     2053,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.400, // UHF tower (COMM1 P17 / DCS default)
	ApproachFreqMHz: 250.400,
	ATISFreqMHz:     0, // no dedicated ATIS on the Iraq presets card
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four",
	BreakDirections: map[string]string{
		"13": "left", // TODO verify
		"31": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 13/31 — TACAN 47X (ORB); no ILS on the map. Designator is a best guess.
			Primary:    Runway{Designator: "13", MagneticHeading: 130.0, ThresholdLatLon: orb.Point{44.3344, 36.5383}},
			Reciprocal: Runway{Designator: "31", MagneticHeading: 310.0, ThresholdLatLon: orb.Point{44.3550, 36.5245}},
		},
	},
}
