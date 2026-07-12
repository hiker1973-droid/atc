package airfield

import "github.com/paulmach/orb"

// ORBI is Baghdad International Airport — Iraq theatre (vSFG-7 COMM1 field,
// tower only — no dedicated ATIS on the presets card). Tower UHF from the
// card + DCS radio table. Center/elevation from the field VOR/DME (BGD 112.9,
// ch 76) and TACAN (BAP, ch 64). Two parallel runways 15L/33R and 15R/33L
// from the four ILS localizer courses (~150/330). L/R assignment and computed
// thresholds VERIFY vs DCS before enabling --position-check.
var ORBI = &Airfield{
	ICAO:            "ORBI",
	Name:            "Baghdad",
	DCSName:         "Baghdad International Airport", // VERIFY exact ME name
	Center:          orb.Point{44.2340, 33.2800},
	ElevationFt:     114,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.300, // UHF tower (COMM1 P14 / DCS default)
	ApproachFreqMHz: 250.300,
	ATISFreqMHz:     0, // no dedicated ATIS on the Iraq presets card
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four",
	BreakDirections: map[string]string{
		"15L": "left", // TODO verify pattern sides
		"33R": "left",
		"15R": "left",
		"33L": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 15R/33L — TACAN 64X (BAP) · VOR/DME 112.90 (BGD) · ILS 110.10 (15R, IYCA) / 110.70 (33L, IYDB)
			Primary:    Runway{Designator: "15R", MagneticHeading: 150.0, ThresholdLatLon: orb.Point{44.2249, 33.2932}},
			Reciprocal: Runway{Designator: "33L", MagneticHeading: 330.0, ThresholdLatLon: orb.Point{44.2431, 33.2668}},
		},
		{
			// 15L/33R — ILS 110.30 (15L, INHA) / 110.90 (33R, ITAK)
			Primary:    Runway{Designator: "15L", MagneticHeading: 150.0, ThresholdLatLon: orb.Point{44.2269, 33.2922}},
			Reciprocal: Runway{Designator: "33R", MagneticHeading: 330.0, ThresholdLatLon: orb.Point{44.2451, 33.2658}},
		},
	},
}
