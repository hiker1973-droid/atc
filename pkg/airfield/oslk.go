package airfield

import "github.com/paulmach/orb"

// OSLK is Bassel Al-Assad — Syria (Eastern Med) theatre.
// Foothold alternate/divert field (operator ruling 2026-09-18). Tower/ATIS are
// the operator's picks, not the Eastern Med card; position, elevation, runways
// and ILS from DimOn Aerodrome Data 01 Feb 2026 and the DCS Syria beacons.lua.
// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
// DCS calls the field Latakia (airfield21). TWO PARALLEL RUNWAYS: 17R/35L
// 8,900 ft carries the ILS 109.10 (IBA, on 17R) and is listed first so it is
// the default; 17L/35R 7,900 ft. VOR/DME LTK 114.80 on the field, NDB 414.
// Position, elevation, runways and headings (173/353 magnetic) from DimOn
// Aerodrome Data 01 Feb 2026 -- 2026-09-19 correction of the single '17/35'
// first read off beacons.lua. Both pairs share one computed centre. Tower
// 250.600 = the DCS terrain's own UHF tower. ATIS 249.600 ASSIGNED.
var OSLK = &Airfield{
	ICAO:            "OSLK",
	Name:            "Bassel Al-Assad",
	DCSName:         "Bassel Al-Assad",             // VERIFY exact ME name on the Syria map
	Center:          orb.Point{35.95028, 35.40169}, // [lon, lat]
	ElevationFt:     94,
	MagVar:          5.5, // +5.5°E, DimOn 2025 value for the Syria map (operator 2026-09-19); wind, BRC, bearings
	PatternAltFt:    1500,
	TowerFreqMHz:    250.600,
	ApproachFreqMHz: 250.600,
	ATISFreqMHz:     249.600,
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel four", // COMMAND is COMM1 P4 on the Eastern Med card
	BreakDirections: map[string]string{
		"17R": "left", // TODO verify pattern side vs ramp in DCS
		"35L": "left",
		"17L": "left", // TODO verify pattern side vs ramp in DCS
		"35R": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "17R", MagneticHeading: 173.0, ThresholdLatLon: orb.Point{35.94976, 35.41387}},
			Reciprocal: Runway{Designator: "35L", MagneticHeading: 353.0, ThresholdLatLon: orb.Point{35.95081, 35.38952}},
		},
		{
			Primary:    Runway{Designator: "17L", MagneticHeading: 173.0, ThresholdLatLon: orb.Point{35.94976, 35.41387}},
			Reciprocal: Runway{Designator: "35R", MagneticHeading: 353.0, ThresholdLatLon: orb.Point{35.95081, 35.38952}},
		},
	},
}
