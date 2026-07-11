package airfield

import "github.com/paulmach/orb"

// ETAR is Ramstein AB — Cold War Germany theatre (COMM1 recovery base).
// Data from the vSFG-7 Cold War Germany kneeboard suite (July 2026). MagVar ≈ 0°
// for the 1999–2005 era the card is valid for, so magnetic ≈ true. Center is the
// real-world ARP; thresholds are computed from center + heading + runway length
// (VERIFY vs DCS before enabling --position-check).
var ETAR = &Airfield{
	ICAO:            "ETAR",
	Name:            "Ramstein",
	DCSName:         "Ramstein", // VERIFY exact ME name on Cold War Germany map
	Center:          orb.Point{7.6003, 49.4369},
	ElevationFt:     805,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    133.20, // VHF tower (COMM1 preset); UHF tower is 251.95
	ApproachFreqMHz: 133.20,
	ATISFreqMHz:     248.00, // UHF ATIS (COMM1); also simulcast VHF 128.00
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // COMMAND is COMM1 P1 on the Germany presets
	BreakDirections: map[string]string{
		"09": "left", // TODO verify pattern side vs ramp location in DCS
		"27": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 09/27 — TACAN 81X · ILS 111.50 (09) / 110.75 (27)
			Primary:    Runway{Designator: "09", MagneticHeading: 90.0, ThresholdLatLon: orb.Point{7.5808, 49.4369}},
			Reciprocal: Runway{Designator: "27", MagneticHeading: 270.0, ThresholdLatLon: orb.Point{7.6198, 49.4369}},
		},
	},
}
