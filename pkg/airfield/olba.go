package airfield

import "github.com/paulmach/orb"

// OLBA is Beirut — Syria (Eastern Med) theatre.
// Foothold alternate/divert field (operator ruling 2026-09-18). Tower/ATIS are
// the operator's picks, not the Eastern Med card; position, elevation, runways
// and ILS from DimOn Aerodrome Data 01 Feb 2026 and the DCS Syria beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// Three runways (03/21, 16/34, 17/35); 16/34 (10,300 ft) and 03/21 modelled.
// ILS 16 IBB 110.10, ILS 17 BIL 109.50, ILS 03 IKK 110.70. VOR/DME KAD 112.60,
// NDB 351. Position (2026-09-19: moved ~700 m from the beacons.lua estimate),
// elevation and runways from DimOn Aerodrome Data 01 Feb 2026. ⚠ DCS terrain
// tower is UHF 253.200 -- the SHELL 2 tanker's frequency in Foothold -- so
// SkyEye uses 250.650 (operator ruling 2026-09-18). ATIS 249.700 ASSIGNED.
// Both pairs share one computed centre, so thresholds are rougher than usual;
// keep --position-check off.
var OLBA = &Airfield{
	ICAO:            "OLBA",
	Name:            "Beirut",
	DCSName:         "Beirut-Rafic Hariri",         // VERIFY exact ME name on the Syria map
	Center:          orb.Point{35.48757, 33.82740}, // [lon, lat]
	ElevationFt:     39,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
	PatternAltFt:    1500,
	TowerFreqMHz:    250.650,
	ApproachFreqMHz: 250.650,
	ATISFreqMHz:     249.700,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"16": "left", // TODO verify pattern side vs ramp in DCS
		"34": "left",
		"03": "left", // TODO verify pattern side vs ramp in DCS
		"21": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "16", MagneticHeading: 164.0, ThresholdLatLon: orb.Point{35.48433, 33.84124}},
			Reciprocal: Runway{Designator: "34", MagneticHeading: 344.0, ThresholdLatLon: orb.Point{35.49081, 33.81356}},
		},
		{
			Primary:    Runway{Designator: "03", MagneticHeading: 30.0, ThresholdLatLon: orb.Point{35.47784, 33.81585}},
			Reciprocal: Runway{Designator: "21", MagneticHeading: 210.0, ThresholdLatLon: orb.Point{35.49731, 33.83895}},
		},
	},
}
