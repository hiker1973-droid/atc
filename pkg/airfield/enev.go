package airfield

import "github.com/paulmach/orb"

// ENEV is Evenes, Norway -- Kola theatre (Kola Training, with Bodo and Andoya). Added 2026-11-14.
// Surveyed from the Mission Editor (dcs-sms me airbase get / coords magvar, 2026-11-14):
//   - Centre = the DCS airbase reference point (16.6781 E 68.4912 N), elevation 26.0 m = 85 ft.
//   - Thresholds = the real DCS runway edges (17 at x 61470.7 / z -239382, 35 at x 58946.8 / z -239452),
//     converted with kola_proj; 2,525 m between them. TRUE bearing 177.6, magvar 6.5 E (ME 6.38 now; the
//     other Kola fields run about 0.1 above the ME's current value for the 2015 mission epoch), so the
//     magnetic headings are 171 / 351 and the designators the ME's own, 17 / 35.
//
// Tower 250.800 is the DCS map's own UHF tower; ATIS 248.800 is ASSIGNED BY US (tower minus two MHz).
// Navaids (beacons.lua): ILS 108.90 runway 17 (the localizer sits at the 35 end), VOR/DME EVD 117.35,
// TACAN 67X "Evenes". The ME lists the airbase as RED in the training mission; it is switched to blue
// there so the tower has a field to serve.
var ENEV = &Airfield{
	ICAO:            "ENEV",
	Name:            "Evenes",
	DCSName:         "Evenes",
	Center:          orb.Point{16.6781, 68.4912}, // [lon, lat] -- DCS airbase reference point
	ElevationFt:     85,
	MagVar:          6.5,
	PatternAltFt:    1500, // AGL, as the other fields
	TowerFreqMHz:    250.800,
	ApproachFreqMHz: 250.800,
	ATISFreqMHz:     248.800,
	DepartureDistNm: 7,
	DepartureAngels: 5, // TODO verify against the terrain around the field
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // Command is COMM1 CH1 on the Viper Kola ladder; Hornet has it on CH4
	BreakDirections: map[string]string{
		"17": "left", // TODO verify pattern side against ramp location
		"35": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 2,525 m between the DCS runway edges; true 177.6 / 357.6.
			Primary:    Runway{Designator: "17", MagneticHeading: 171.0, ThresholdLatLon: orb.Point{16.676829, 68.502496}},
			Reciprocal: Runway{Designator: "35", MagneticHeading: 351.0, ThresholdLatLon: orb.Point{16.679448, 68.479880}},
		},
	},
}
