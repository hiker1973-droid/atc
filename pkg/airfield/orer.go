package airfield

import "github.com/paulmach/orb"

// ORER is Erbil International Airport — Iraq theatre (vSFG-7 COMM1 field,
// tower only — no dedicated ATIS on the presets card). Tower UHF from the card
// + DCS radio table. Center/elevation from the field VOR/DME (RER 116.3,
// ch 110). Runway 18/36 from the ILS localizer courses (IREA 359 / IREB 176);
// the map also carries a 33 approach (IRER) not modelled here. Thresholds
// computed VERIFY vs DCS.
var ORER = &Airfield{
	ICAO:            "ORER",
	Name:            "Erbil",
	DCSName:         "Erbil International Airport", // VERIFY exact ME name
	Center:          orb.Point{43.9663, 36.2431},
	ElevationFt:     1341,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.350, // UHF tower (COMM1 P18 / DCS default)
	ApproachFreqMHz: 250.350,
	ATISFreqMHz:     0, // no dedicated ATIS on the Iraq presets card
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four",
	BreakDirections: map[string]string{
		"18": "left", // TODO verify
		"36": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 18/36 — VOR/DME 116.30 (RER, ch 110) · ILS 109.70 (18, IREB) / 109.20 (36, IREA)
			Primary:    Runway{Designator: "18", MagneticHeading: 180.0, ThresholdLatLon: orb.Point{43.9663, 36.2602}},
			Reciprocal: Runway{Designator: "36", MagneticHeading: 360.0, ThresholdLatLon: orb.Point{43.9663, 36.2260}},
		},
	},
}
