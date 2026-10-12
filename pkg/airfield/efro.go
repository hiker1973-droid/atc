package airfield

import "github.com/paulmach/orb"

// EFRO is Rovaniemi, Finland — Kola theatre (Operation WHITE NIGHT).
// Added 2026-10. Centre, elevation and runway ends are from the DCS
// Mission Editor (dcs-sms me airbase get / coords to-geo / coords magvar,
// 2026-10-09; MagVar was re-dated to the mission year on 2026-10-11, see KOLA_PLAN.md):
// the thresholds are real DCS runway edges, and the headings are the
// TRUE bearing between them minus the field's magvar — NOT the ME's grid course,
// which is 5° off true at this latitude.
// Tower 250.950 is the DCS map's own UHF tower; ATIS 248.950 is ASSIGNED BY US
// (tower minus two MHz) — the squadron Kola card carries no ATIS frequency.
var EFRO = &Airfield{
	ICAO:            "EFRO",
	Name:            "Rovaniemi",
	DCSName:         "Rovaniemi",
	Center:          orb.Point{25.8305, 66.5648}, // [lon, lat] — DCS airbase reference point
	ElevationFt:     636,
	MagVar:          10.8, // DCS 10.82°E at the field on 12 Nov 2015, the WHITE NIGHT date — DCS takes magvar from the MISSION DATE (2026 gives about 3° more); per-field, it differs by 7° across this map
	PatternAltFt:    1500, // AGL, as the other fields
	TowerFreqMHz:    250.950,
	ApproachFreqMHz: 250.950,
	ATISFreqMHz:     248.950,
	DepartureDistNm: 7,
	DepartureAngels: 3, // TODO verify against the terrain around the field
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // Command is COMM1 CH1 on the Viper Kola ladder (the land fields are Viper bases); Hornet has it on CH4
	BreakDirections: map[string]string{
		"21": "left", // TODO verify pattern side against ramp location
		"03": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 9,040 ft between the DCS runway edges; true 217.4 / 37.4.
			Primary:    Runway{Designator: "21", MagneticHeading: 207.0, ThresholdLatLon: orb.Point{25.8494, 66.5747}},
			Reciprocal: Runway{Designator: "03", MagneticHeading: 27.0, ThresholdLatLon: orb.Point{25.8116, 66.5550}},
		},
	},
}
