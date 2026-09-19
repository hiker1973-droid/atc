package airfield

import "github.com/paulmach/orb"

// OLBA is Beirut–Rafic Hariri International — Syria (Eastern Med) theatre,
// DIVERT field.
//
// ⚠ NOT ON THE SQUADRON CARD. Pilots have NO preset for this field; the tower
// frequency has to be briefed or dialled manually. Added 2026-09-19 on operator
// request as an alternate/divert field.
//
// Tower 253.200 is the DCS terrain frequency, read from the Syria
// Mods/terrains/Syria/radio.lua entry "Beirut" (callsign "Hariri", UHF AM
// 253200000). As at Bassel Al-Assad, this shares the frequency with the DCS AI
// tower.
//
// ATIS 249.700 is ASSIGNED BY US — DCS has no ATIS and the card has no entry.
// Chosen clear of the existing Syria ATIS block and of Bassel Al-Assad 249.600.
//
// ⚠ NO ILS, NO TACAN, NO VOR. beacons.lua carries exactly one Beirut entry, an
// AIRPORT_HOMER_WITH_MARKER on 351 kHz, callsign BOD. The ATIS reports that NDB
// and nothing else — the same treatment H4 (OJHR) gets — so we never claim an
// approach aid the field does not have.
//
// ⚠ POSITION DATA IS DERIVED, NOT SURVEYED, AND WEAKER THAN OSLK's. beacons.lua
// gives only the NDB, so there is no localizer course to resolve the runway
// layout against — center, headings and thresholds come from the real-world
// OLBA plate (03/21 3395 m, 16/34 3180 m) and are approximate. Both pairs are
// declared so wind-driven selection has a sensible choice, but VERIFY AGAINST
// DCS before enabling --position-check.
var OLBA = &Airfield{
	ICAO:            "OLBA",
	Name:            "Beirut",
	DCSName:         "Beirut",                      // VERIFY exact ME name
	Center:          orb.Point{35.48840, 33.82090}, // [lon, lat]
	ElevationFt:     87,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    253.200,
	ApproachFreqMHz: 253.200,
	ATISFreqMHz:     249.700,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"03": "left", // TODO verify pattern side vs terminal in DCS
		"21": "left",
		"16": "left",
		"34": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "03", MagneticHeading: 34.0, ThresholdLatLon: orb.Point{35.48000, 33.81000}},
			Reciprocal: Runway{Designator: "21", MagneticHeading: 214.0, ThresholdLatLon: orb.Point{35.49800, 33.83600}},
		},
		{
			Primary:    Runway{Designator: "16", MagneticHeading: 164.0, ThresholdLatLon: orb.Point{35.49300, 33.83500}},
			Reciprocal: Runway{Designator: "34", MagneticHeading: 344.0, ThresholdLatLon: orb.Point{35.48500, 33.80800}},
		},
	},
}
