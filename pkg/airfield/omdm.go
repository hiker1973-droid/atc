package airfield

import "github.com/paulmach/orb"

// OMDM is Al Minhad Air Base, UAE (Al Minhad AFB in DCS).
// Data sourced from [USAF] AFD-3351 airport diagram, July 2020.
var OMDM = &Airfield{
	ICAO:            "OMDM",
	Name:            "Al Minhad",
	DCSName:         "Al Minhad AFB",
	Center:          orb.Point{55.36582, 25.02684}, // [lon, lat] DimOn Aerodrome Data 01 Feb 2026 (was 55.3692/25.0333 off the diagram grid, ~740 m north)
	ElevationFt:     190,
	MagVar:          2.5, // +2.5°E, DimOn 2016-2025 value for the PG map (operator 2026-09-19); wind, BRC, bearings
	PatternAltFt:    1500,
	TowerFreqMHz:    250.100,
	ApproachFreqMHz: 250.100, // No separate approach freq on diagram — use tower
	ATISFreqMHz:     248.3,
	DepartureDistNm:  7,
	DepartureAngels:   3,
	HandoffCallsign:  "command",
	HandoffFreqMHz:   282.000,
	HandoffPreset:    "channel four",
	BreakDirections: map[string]string{
		// Ramp on south side. Heading west on 27 → south is on the left.
		// Heading east on 09 → south is on the right.
		"27": "left",
		"09": "right",
	},
	RunwayPairs: []RunwayPair{
		{
			// Single runway 09/27 — 11,865 ft long
			// Heading 088°/268° magnetic in DCS (DimOn Aerodrome Data 01 Feb
			// 2026; the real-world diagram says 090/270). Thresholds recomputed
			// 2026-09-19 from the corrected centre, 12,600 ft and 090.5 true.
			// ILS 09: 110.70 MHz | ILS 27: 110.75 MHz
			// Primary set to 09 — calm-wind default. Wind-based picker still
			// flips to 27 when easterly winds exceed 3 kts (see
			// airfield.ActiveRunway).
			Primary: Runway{
				Designator:      "09",
				MagneticHeading: 88.0,
				ThresholdLatLon: orb.Point{55.34678, 25.02699}, // West threshold (computed)
			},
			Reciprocal: Runway{
				Designator:      "27",
				MagneticHeading: 268.0,
				ThresholdLatLon: orb.Point{55.38485, 25.02669}, // East threshold (computed)
			},
		},
	},
}
