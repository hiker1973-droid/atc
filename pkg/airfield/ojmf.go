package airfield

import "github.com/paulmach/orb"

// OJMF is Mafraq — Syria (Eastern Med) theatre.
// Tower/ATIS from the ratified vSFG-7 "Hornet Radio Presets — Eastern Med" card;
// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams
// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// Called KING HUSSEIN AC on the squadron card; OJMF Mafraq AB on the chart.
// VORTAC ABC 115.90 ch106, ILS 111.70 on 13. ⚠ DCS terrain tower is 250.40;
// the card says 250.450.
var OJMF = &Airfield{
	ICAO:            "OJMF",
	Name:            "Mafraq",
	DCSName:         "King Hussein Air College",    // VERIFY exact ME name on the Syria map
	Center:          orb.Point{36.25917, 32.35639}, // [lon, lat]
	ElevationFt:     2204,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    250.450,
	ApproachFreqMHz: 250.450,
	ATISFreqMHz:     255.550,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"13": "left", // TODO verify pattern side vs ramp in DCS
		"31": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "13", MagneticHeading: 132.0, ThresholdLatLon: orb.Point{36.24829, 32.36624}},
			Reciprocal: Runway{Designator: "31", MagneticHeading: 312.0, ThresholdLatLon: orb.Point{36.27005, 32.34653}},
		},
	},
}
