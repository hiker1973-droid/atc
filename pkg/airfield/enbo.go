package airfield

import "github.com/paulmach/orb"

// ENBO is Bodo, Norway — Kola theatre (Operation WHITE NIGHT).
// Added 2026-10. Centre, elevation, magvar and runway ends are from the DCS
// Mission Editor (dcs-sms me airbase get / coords to-geo / coords magvar,
// 2026-10-09): the thresholds are real DCS runway edges, and the headings are the
// TRUE bearing between them minus the field's magvar — NOT the ME's grid course,
// which is 6° off true at this latitude.
// Tower 251.150 is the DCS map's own UHF tower; ATIS 249.150 is ASSIGNED BY US
// (tower minus two MHz) — the squadron Kola card carries no ATIS frequency.
var ENBO = &Airfield{
	ICAO:            "ENBO",
	Name:            "Bodo",
	DCSName:         "Bodo",
	Center:          orb.Point{14.3654, 67.2692}, // [lon, lat] — DCS airbase reference point
	ElevationFt:     33,
	MagVar:          8.0,  // DCS 8.03°E at the field — magvar runs 8°E to 15°E across this map, so it is per-field
	PatternAltFt:    1500, // AGL, as the other fields
	TowerFreqMHz:    251.150,
	ApproachFreqMHz: 251.150,
	ATISFreqMHz:     249.150,
	DepartureDistNm: 7,
	DepartureAngels: 5, // TODO verify against the terrain around the field
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // Command is COMM1 CH1 on the Viper Kola ladder (the land fields are Viper bases); Hornet has it on CH4
	BreakDirections: map[string]string{
		"07": "left", // TODO verify pattern side against ramp location
		"25": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 8,583 ft between the DCS runway edges; true 77.7 / 257.7.
			Primary:    Runway{Designator: "07", MagneticHeading: 70.0, ThresholdLatLon: orb.Point{14.3356, 67.2667}},
			Reciprocal: Runway{Designator: "25", MagneticHeading: 250.0, ThresholdLatLon: orb.Point{14.3951, 67.2717}},
		},
	},
}
