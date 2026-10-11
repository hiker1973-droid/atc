package airfield

import "github.com/paulmach/orb"

// ENDU is Bardufoss, Norway — Kola theatre (Operation WHITE NIGHT).
// Added 2026-10. Centre, elevation and runway ends are from the DCS
// Mission Editor (dcs-sms me airbase get / coords to-geo / coords magvar,
// 2026-10-09; MagVar was re-dated to the mission year on 2026-10-11, see KOLA_PLAN.md):
// the thresholds are real DCS runway edges, and the headings are the
// TRUE bearing between them minus the field's magvar — NOT the ME's grid course,
// which is 2° off true at this latitude.
// Tower 250.650 is the DCS map's own UHF tower; ATIS 248.650 is ASSIGNED BY US
// (tower minus two MHz) — the squadron Kola card carries no ATIS frequency.
var ENDU = &Airfield{
	ICAO:            "ENDU",
	Name:            "Bardufoss",
	DCSName:         "Bardufoss",
	Center:          orb.Point{18.5435, 69.0553}, // [lon, lat] — DCS airbase reference point
	ElevationFt:     234,
	MagVar:          8.0,  // DCS 7.95°E at the field on 12 Nov 2015, the WHITE NIGHT date — DCS takes magvar from the MISSION DATE (2026 gives about 3° more); per-field, it differs by 7° across this map
	PatternAltFt:    1500, // AGL, as the other fields
	TowerFreqMHz:    250.650,
	ApproachFreqMHz: 250.650,
	ATISFreqMHz:     248.650,
	DepartureDistNm: 7,
	DepartureAngels: 6, // TODO verify against the terrain around the field
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // Command is COMM1 CH1 on the Viper Kola ladder (the land fields are Viper bases); Hornet has it on CH4
	BreakDirections: map[string]string{
		"28": "left", // TODO verify pattern side against ramp location
		"10": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 6,398 ft between the DCS runway edges; true 289.0 / 109.0.
			Primary:    Runway{Designator: "28", MagneticHeading: 281.0, ThresholdLatLon: orb.Point{18.5667, 69.0525}},
			Reciprocal: Runway{Designator: "10", MagneticHeading: 101.0, ThresholdLatLon: orb.Point{18.5203, 69.0582}},
		},
	},
}
