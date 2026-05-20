package airfield

import "github.com/paulmach/orb"

// OMDM is Al Minhad Air Base, UAE (Al Minhad AFB in DCS).
// Data sourced from [USAF] AFD-3351 airport diagram, July 2020.
var OMDM = &Airfield{
	ICAO:            "OMDM",
	Name:            "Al Minhad",
	DCSName:         "Al Minhad AFB",
	Center:          orb.Point{55.3692, 25.0333}, // [lon, lat] derived from diagram grid
	ElevationFt:     190,
	MagVar:          2.25, // +2.25°E per diagram
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
			// Heading 090°/270° magnetic (diagram confirmed)
			// ILS 09: 110.70 MHz | ILS 27: 110.75 MHz
			// Primary set to 09 — calm-wind default. Wind-based picker still
			// flips to 27 when easterly winds exceed 3 kts (see
			// airfield.ActiveRunway).
			Primary: Runway{
				Designator:      "09",
				MagneticHeading: 90.0,
				ThresholdLatLon: orb.Point{55.3550, 25.0333}, // West threshold (point A)
			},
			Reciprocal: Runway{
				Designator:      "27",
				MagneticHeading: 270.0,
				ThresholdLatLon: orb.Point{55.3833, 25.0333}, // East threshold (point G)
			},
		},
	},
}
