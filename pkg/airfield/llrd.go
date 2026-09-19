package airfield

import "github.com/paulmach/orb"

// LLRD is Ramat David — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua
// and (2026-09-19) DimOn Aerodrome Data 01 Feb 2026.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// Three runway pairs on the field (09/27, 11/29, 15/33); only 15/33 has an ILS
// so that is the pair modelled here. ⚠ 2026-09-19 per DimOn Aerodrome Data 01
// Feb 2026: the ILS 111.10 serves RUNWAY 33, not 15; centre moved ~420 m to
// the aerodrome reference point, elevation 105 -> 146 ft, headings 146/326 ->
// 141/321 magnetic, TACAN 84X, VOR 113.70, NDB 368. ⚠ DCS terrain tower is
// 251.05; the card says 251.300 and the card wins.
var LLRD = &Airfield{
	ICAO:            "LLRD",
	Name:            "Ramat David",
	DCSName:         "Ramat David",                 // VERIFY exact ME name on the Syria map
	Center:          orb.Point{35.18390, 32.66638}, // [lon, lat]
	ElevationFt:     146,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    251.300,
	ApproachFreqMHz: 251.300,
	ATISFreqMHz:     256.150,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"15": "left", // TODO verify pattern side vs ramp in DCS
		"33": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "15", MagneticHeading: 141.0, ThresholdLatLon: orb.Point{35.17690, 32.67512}},
			Reciprocal: Runway{Designator: "33", MagneticHeading: 321.0, ThresholdLatLon: orb.Point{35.19090, 32.65764}},
		},
	},
}
