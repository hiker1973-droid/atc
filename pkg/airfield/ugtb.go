package airfield

import "github.com/paulmach/orb"

// UGTB is Tbilisi-Lochini, Georgia — Caucasus (Black Sea) theatre.
// Added 2026-10 as the RED field for the vSFG-7 BFM mission (Venom vs Raider).
// Centre, elevation, magvar and runway ends are from the DCS Mission Editor
// (dcs-sms me airbase get / coords to-geo / coords magvar, 2026-10-05); tower is
// DCS Radio.lua airfield29_0; ATIS 237.000 is the squadron Hornet preset card
// (COMM1 CH19 "TBILISI ATIS"). No TACAN at the field.
var UGTB = &Airfield{
	ICAO:            "UGTB",
	Name:            "Tbilisi",
	DCSName:         "Tbilisi-Lochini",
	Center:          orb.Point{44.9563, 41.6670}, // [lon, lat] — DCS airbase reference point
	ElevationFt:     1574,
	MagVar:          7.0,  // DCS 6.98°E at the field (the four coastal fields carry the map-wide 7.5)
	PatternAltFt:    1500, // AGL, as the other fields
	TowerFreqMHz:    267.000,
	ApproachFreqMHz: 267.000,
	ATISFreqMHz:     0, // its SkyEye ATIS and 237.000 went to Mozdok (operator 2026-10-23)
	DepartureDistNm: 7,
	DepartureAngels: 5, // field is at 1,574 ft — angels 3 would be 1,400 ft AGL
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000, // theatre-wide Command; the BFM mission's RED net is 280.000 — see SKYEYE_TBILISI_UPDATE
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"13R": "left", // TODO verify pattern side against ramp location
		"31L": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// Thresholds are real DCS runway edges, not centre ± half length.
			Primary:    Runway{Designator: "13R", MagneticHeading: 121.0, ThresholdLatLon: orb.Point{44.9466, 41.6745}},
			Reciprocal: Runway{Designator: "31L", MagneticHeading: 301.0, ThresholdLatLon: orb.Point{44.9660, 41.6595}},
		},
	},
}
