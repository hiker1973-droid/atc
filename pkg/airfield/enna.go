package airfield

import "github.com/paulmach/orb"

// ENNA is Banak, Norway — Kola theatre (Operation WHITE NIGHT).
// Added 2026-10. Centre, elevation and runway ends are from the DCS
// Mission Editor (dcs-sms me airbase get / coords to-geo / coords magvar,
// 2026-10-09; MagVar was re-dated to the mission year on 2026-10-11, see KOLA_PLAN.md):
// the thresholds are real DCS runway edges, and the headings are the
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
	MagVar:          11.6, // DCS 11.62°E at the field on 12 Nov 2015, the WHITE NIGHT date — DCS takes magvar from the MISSION DATE (2026 gives about 3° more); per-field, it differs by 7° across this map
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
			Primary:    Runway{Designator: "34", MagneticHeading: 344.0, ThresholdLatLon: orb.Point{24.9761, 70.0578}},
			Reciprocal: Runway{Designator: "16", MagneticHeading: 164.0, ThresholdLatLon: orb.Point{24.9710, 70.0798}},
		},
	},
}
