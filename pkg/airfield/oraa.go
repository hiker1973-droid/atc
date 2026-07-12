package airfield

import "github.com/paulmach/orb"

// ORAA is Al Asad Air Base — Iraq theatre (vSFG-7 COMM1 recovery base).
// Data from the DCS Iraq terrain (beacons.lua / radio.lua) cross-checked with
// the vSFG-7 Iraq Hornet presets card (July 2026). Tower is UHF per the card
// and the DCS airfield radio table. Center/elevation from the field TACAN
// (RAA, ch 80) beacon; runway from the ILS localizer courses (090/270).
// Thresholds are computed from center + heading (VERIFY vs DCS before enabling
// --position-check). MagVar is documentation-only (unused in the wind logic).
var ORAA = &Airfield{
	ICAO:            "ORAA",
	Name:            "Al Asad",
	DCSName:         "Al-Asad Airbase", // VERIFY exact ME name on the Iraq map
	Center:          orb.Point{42.4436, 33.7874},
	ElevationFt:     594,
	MagVar:          4.5, // ~+4.5°E (modern Iraq); documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    363.700, // UHF tower (COMM1 P8 / DCS default)
	ApproachFreqMHz: 363.700,
	ATISFreqMHz:     230.100, // COMM1 P9 (Al-Asad ATIS)
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Iraq presets
	BreakDirections: map[string]string{
		"09": "left", // TODO verify pattern side vs ramp in DCS
		"27": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 09/27 — TACAN 80X (RAA) · ILS 108.90 (09, ISAD) / 108.80 (27, IRAA)
			Primary:    Runway{Designator: "09", MagneticHeading: 90.0, ThresholdLatLon: orb.Point{42.4263, 33.7874}},
			Reciprocal: Runway{Designator: "27", MagneticHeading: 270.0, ThresholdLatLon: orb.Point{42.4609, 33.7874}},
		},
	},
}
