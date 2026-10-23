package airfield

import "github.com/paulmach/orb"

// XRMF is Mozdok, Russia — Caucasus theatre. Red's field in the BFM Venom vs
// Raider mission (operator 2026-10-22: "Red will be using mozdok airfield",
// "Mozdok tower is 266"), blue in the Training mission (2026-10-23: "for the
// training miz I want to make Mozdok a blue airfield"). It runs with the
// Caucasus towers as blue; the dashboard's Side switch makes it red for a BFM
// night (cmd/atc/sides.go), which hands departures to Red Command 280 instead.
//
// Centre, elevation and runway edges are the live Mission Editor's
// (`dcs-sms me airbase get --name Mozdok`, kept in the BFM build's
// mozdok_stands.json), converted from DCS x/y to lat/lon here (the same
// conversion puts Senaki's centre within 40 m of its TACAN). The runway runs
// 090.8 true. MagVar is the map-wide 7.5: Mozdok sits between DimOn's 2025
// +7 and +8 lines. Tower 266.000 UHF is DCS's (Radio.lua); no ILS. The ATIS on
// 237.000 is SkyEye's own, moved from Tbilisi (operator 2026-10-23).
// The ME edges may not be the pavement ends (Tbilisi's were not).
var XRMF = &Airfield{
	ICAO:            "XRMF",
	Name:            "Mozdok",
	DCSName:         "Mozdok",
	Center:          orb.Point{44.6059, 43.7917}, // [lon, lat] — DCS airbase reference point
	ElevationFt:     507,
	MagVar:          7.5,
	PatternAltFt:    1500,
	TowerFreqMHz:    266.000,
	ApproachFreqMHz: 266.000,
	ATISFreqMHz:     237.000, // SkyEye's, taken over from Tbilisi (2026-10-23); DCS has none
	DepartureDistNm: 7,
	DepartureAngels: 3,
	// Blue: theatre-wide Command, as the other Caucasus fields. Set red, the
	// tower hands off to Red Command 280 instead (sides.go).
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"08": "left", // TODO verify pattern side against the ramp (south of the runway)
		"26": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "08", MagneticHeading: 83.3, ThresholdLatLon: orb.Point{44.5914, 43.7918}},
			Reciprocal: Runway{Designator: "26", MagneticHeading: 263.3, ThresholdLatLon: orb.Point{44.6204, 43.7916}},
		},
	},
}
