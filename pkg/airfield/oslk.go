package airfield

import "github.com/paulmach/orb"

// OSLK is Bassel Al-Assad (Latakia) — Syria (Eastern Med) theatre, DIVERT field.
//
// ⚠ NOT ON THE SQUADRON CARD. The vSFG-7 "Hornet Radio Presets — Eastern Med"
// card gives eight recovery bases and this is not one of them, so pilots have
// NO preset for it — the tower frequency has to be briefed or dialled manually.
// Added 2026-09-19 on operator request as an alternate/divert field.
//
// Tower 250.600 is the DCS terrain frequency, read from the Syria
// Mods/terrains/Syria/radio.lua entry "Latakia" (UHF AM 250600000). That is the
// same source the card itself follows — Akrotiri's card frequency 252.000
// matches radio.lua exactly — so we transmit on the field's own UHF rather than
// inventing one. NOTE this means we share the frequency with the DCS AI tower,
// the same deviation Deckboss runs on 128.600.
//
// ATIS 249.600 is ASSIGNED BY US — DCS has no ATIS and the card has no entry.
// Chosen clear of the existing Syria ATIS block (240.850 / 249.000 / 249.300 /
// 249.400 / 249.500 / 255.550 / 256.150 / 360.200).
//
// Navaids from Mods/terrains/Syria/beacons.lua ("LATAKIA"): ILS 109.10 IBA with
// localizer course 178.9 true, VOR/DME LTK 114.80, NDB LTK 414. The RSBN ch7 /
// PRMG ch9 pair is Russian-only and deliberately omitted from the ATIS.
//
// ⚠ POSITION DATA IS DERIVED, NOT SURVEYED. Center and thresholds are taken
// from the beacons.lua positionGeo values (ILS glideslope 35.4112N near the
// RWY 17 threshold, localizer 35.3875N beyond the RWY 35 rollout) and the
// real-world 17/35 layout. Elevation 92 ft follows the DCS beacon altitudes
// (~28 m), NOT the real-world 157 ft. Verify against DCS before enabling
// --position-check.
var OSLK = &Airfield{
	ICAO:            "OSLK",
	Name:            "Bassel Al-Assad",
	DCSName:         "Bassel Al-Assad",             // VERIFY exact ME name — radio.lua calls it "Latakia"
	Center:          orb.Point{35.94890, 35.40010}, // [lon, lat]
	ElevationFt:     92,
	MagVar:          5.0, // ~+5.0°E over the Levant; documentation only
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
		"17": "left", // TODO verify pattern side vs ramp in DCS
		"35": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			Primary:    Runway{Designator: "17", MagneticHeading: 174.0, ThresholdLatLon: orb.Point{35.94850, 35.41120}},
			Reciprocal: Runway{Designator: "35", MagneticHeading: 354.0, ThresholdLatLon: orb.Point{35.94920, 35.38900}},
		},
	},
}
