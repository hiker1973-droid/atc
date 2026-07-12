package airfield

import "github.com/paulmach/orb"

// ORBD is Balad Air Base (Al Bakr) — Iraq theatre (vSFG-7 COMM1 recovery base).
// Tower UHF from the Iraq presets card + DCS radio table. Center/elevation from
// the field VORTAC (BLD 114.6, ch 93). Runway 15/33 from the ILS localizer
// courses (IBLD 146 / IANC 326). Computed thresholds VERIFY vs DCS.
var ORBD = &Airfield{
	ICAO:            "ORBD",
	Name:            "Balad",
	DCSName:         "Balad Airbase", // VERIFY exact ME name on the Iraq map
	Center:          orb.Point{44.3729, 33.9314},
	ElevationFt:     161,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.550, // UHF tower (COMM1 P15 / DCS default)
	ApproachFreqMHz: 250.550,
	ATISFreqMHz:     230.700, // COMM1 P16 (Balad ATIS)
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
			// 15/33 — VORTAC 114.60 (BLD, ch 93) · ILS 109.90 (15, IBLD) / 109.95 (33, IANC)
			Primary:    Runway{Designator: "15", MagneticHeading: 150.0, ThresholdLatLon: orb.Point{44.3642, 33.9438}},
			Reciprocal: Runway{Designator: "33", MagneticHeading: 330.0, ThresholdLatLon: orb.Point{44.3816, 33.9190}},
		},
	},
}
