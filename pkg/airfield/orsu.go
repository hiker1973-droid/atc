package airfield

import "github.com/paulmach/orb"

// ORSU is Sulaymaniyah International Airport — Iraq theatre (vSFG-7 COMM1
// field, tower only — no dedicated ATIS on the presets card). Tower UHF from
// the card + DCS radio table. Center/elevation from the field VOR/DME
// (SUL 117.0, ch 117). Runway 13/31 from the reciprocal ILS localizer courses
// (SUL 133 / NGA 313). Computed thresholds VERIFY vs DCS.
var ORSU = &Airfield{
	ICAO:            "ORSU",
	Name:            "Sulaymaniyah",
	DCSName:         "Sulaimaniyah International Airport", // VERIFY exact ME name
	Center:          orb.Point{45.3000, 35.5650},
	ElevationFt:     2494,
	MagVar:          4.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    250.500, // UHF tower (COMM1 P20 / DCS default)
	ApproachFreqMHz: 250.500,
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
			// 13/31 — VOR/DME 117.00 (SUL, ch 117) · ILS 111.70 (13, SUL) / 111.10 (31, NGA)
			Primary:    Runway{Designator: "13", MagneticHeading: 130.0, ThresholdLatLon: orb.Point{45.2865, 35.5742}},
			Reciprocal: Runway{Designator: "31", MagneticHeading: 310.0, ThresholdLatLon: orb.Point{45.3135, 35.5558}},
		},
	},
}
