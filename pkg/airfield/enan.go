package airfield

import "github.com/paulmach/orb"

// ENAN is Andoya, Norway -- Kola theatre, FOOTHOLD KOLA (blue's home field with Bodo and CVN-72).
// Added 2026-11-14. The Mission Editor was not available, so this field is DERIVED, not surveyed:
//   * Centre = the DCS airbase reference point (ME dump, airbases.json: 16.1354 E 69.2976 N, elevation 6.69 m).
//     At Bodo that point is exactly the runway midpoint, so the same is assumed here.
//   * Runway axis = the runway edge in the terrain's own taxi network
//     (Mods\terrains\Kola\AirfieldsTaxiways\ENAN Andoya.rn5): grid bearing 154.33, which is TRUE 149.78
//     at this longitude (grid convergence -4.55 deg). Designators 14/32 are the ME's own (airfield_freqs.json).
//   * Length 2,500 m is the real runway's, ASSUMED -- the thresholds are centre +/- 1,250 m along the axis
//     ("computed, not surveyed", as the Caucasus, Iraq and Syria fields). Good enough for phraseology and
//     the initial call; verify against the ME before anyone enables the --position-check hold-short gate.
//   * Magvar 6.3 E = DCS's model for a 2015 mission date, the same epoch as the other Kola fields
//     (dcs_magvar.py in the White Night _BUILD folder). The Foothold Kola mission is dated 2015 on purpose.
//   * Heading = true bearing between the thresholds minus magvar: 149.78 - 6.33 = 143.45 -> 143 / 323.
// Tower 250.600 is the DCS map's own UHF tower; ATIS 248.600 is ASSIGNED BY US (tower minus two MHz).
// Navaids (beacons.lua): VOR/DME AND 112.20, TACAN OYA 28 (mode X assumed). There is NO ILS at Andoya.
var ENAN = &Airfield{
	ICAO:            "ENAN",
	Name:            "Andoya",
	DCSName:         "Andoya",
	Center:          orb.Point{16.1354, 69.2976}, // [lon, lat] -- DCS airbase reference point
	ElevationFt:     22,
	MagVar:          6.3, // DCS model for a 2015 mission date (the Kola epoch); per-field, it differs by 7 deg across this map
	PatternAltFt:    1500, // AGL, as the other fields
	TowerFreqMHz:    250.600,
	ApproachFreqMHz: 250.600,
	ATISFreqMHz:     248.600,
	DepartureDistNm: 7,
	DepartureAngels: 5, // TODO verify against the terrain around the field
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one", // Command is COMM1 CH1 on the Viper Kola ladder; Hornet has it on CH4
	BreakDirections: map[string]string{
		"14": "left", // TODO verify pattern side against ramp location
		"32": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// ~2,500 m, true 149.8 / 329.8 (derived -- see the header).
			Primary:    Runway{Designator: "14", MagneticHeading: 143.0, ThresholdLatLon: orb.Point{16.11945, 69.30726}},
			Reciprocal: Runway{Designator: "32", MagneticHeading: 323.0, ThresholdLatLon: orb.Point{16.15133, 69.28790}},
		},
	},
}
