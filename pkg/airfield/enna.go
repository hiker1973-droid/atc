package airfield

import "github.com/paulmach/orb"

// ENNA is Banak, Norway — Kola theatre (Operation WHITE NIGHT).
// Added 2026-10. Centre, elevation, magvar and runway ends are from the DCS
// Mission Editor (dcs-sms me airbase get / coords to-geo / coords magvar,
// 2026-10-09): the thresholds are real DCS runway edges, and the headings are the
// TRUE bearing between them minus the field's magvar — NOT the ME's grid course,
// which is 4° off true at this latitude.
// Tower 250.550 is the DCS map's own UHF tower; ATIS 248.550 is ASSIGNED BY US
// (tower minus two MHz) — the squadron Kola card carries no ATIS frequency.
var ENNA = &Airfield{
	ICAO:            "ENNA",
	Name:            "Banak",
	DCSName:         "Banak",
	Center:          orb.Point{24.9735, 70.0688}, // [lon, lat] — DCS airbase reference point
	ElevationFt:     17,
	MagVar:          14.8, // DCS 14.80°E at the field — magvar runs 8°E to 15°E across this map, so it is per-field
	PatternAltFt:    1500, // AGL, as the other fields
	TowerFreqMHz:    250.550,
	ApproachFreqMHz: 250.550,
	ATISFreqMHz:     248.550,
	DepartureDistNm: 7,
	DepartureAngels: 4, // TODO verify against the terrain around the field
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // Command is COMM1 CH1 on the Viper Kola ladder (the land fields are Viper bases); Hornet has it on CH4
	BreakDirections: map[string]string{
		"34": "left", // TODO verify pattern side against ramp location
		"16": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 8,051 ft between the DCS runway edges; true 355.5 / 175.5.
			Primary:    Runway{Designator: "34", MagneticHeading: 341.0, ThresholdLatLon: orb.Point{24.9761, 70.0578}},
			Reciprocal: Runway{Designator: "16", MagneticHeading: 161.0, ThresholdLatLon: orb.Point{24.9710, 70.0798}},
		},
	},
}
