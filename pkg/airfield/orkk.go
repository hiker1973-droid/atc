package airfield

import "github.com/paulmach/orb"

// ORKK is Kirkuk International Airport — Iraq theatre (vSFG-7 COMM1 field,
// tower only — no dedicated ATIS on the presets card). Tower UHF from the card
// + DCS radio table. Center/elevation from the field TACAN (KRK, ch 86) and
// VOR/DME (KIR 111.2, ch 49). Runway 13/31 from the ILS localizer course
// (ISAD 134). Designator (13 vs 14) and computed thresholds VERIFY vs DCS.
var ORKK = &Airfield{
	ICAO:            "ORKK",
	Name:            "Kirkuk",
	DCSName:         "Kirkuk International Airport", // VERIFY exact ME name
	Center:          orb.Point{44.3476, 35.4713},
	ElevationFt:     1061,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.050, // UHF tower (COMM1 P19 / DCS default)
	ApproachFreqMHz: 250.050,
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
			// 13/31 — TACAN 86X (KRK) · VOR/DME 111.20 (KIR, ch 49) · ILS 109.10 (13, ISAD)
			Primary:    Runway{Designator: "13", MagneticHeading: 130.0, ThresholdLatLon: orb.Point{44.3341, 35.4805}},
			Reciprocal: Runway{Designator: "31", MagneticHeading: 310.0, ThresholdLatLon: orb.Point{44.3611, 35.4621}},
		},
	},
}
