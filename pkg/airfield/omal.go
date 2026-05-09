package airfield

import "github.com/paulmach/orb"

// OMAL is Al Ain International Airport, UAE.
// Data sourced from [USAF] AFD-3351 airport diagram, July 2020.
// Single runway 01/19 — heading 010°/190° magnetic.
var OMAL = &Airfield{
	ICAO:            "OMAL",
	Name:            "Al Ain",
	DCSName:         "Al Ain Intl",
	Center:          orb.Point{55.6083, 24.2617}, // [lon, lat] derived from diagram grid
	ElevationFt:     814,
	MagVar:          2.25, // +2.25°E per diagram
	PatternAltFt:    2000, // Higher pattern alt due to field elevation
	TowerFreqMHz:    250.700,
	ApproachFreqMHz: 250.700,
	ATISFreqMHz:     248.850,
	DepartureDistNm:  7,
	DepartureAngels:   5,
	HandoffCallsign:  "command",
	HandoffFreqMHz:   282.000,
	HandoffPreset:    "channel four",
	BreakDirections: map[string]string{
		// Terminal on west side. Heading south on 19 → west is on the right.
		// Heading north on 01 → west is on the left.
		"19": "right",
		"01": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// Single runway 01/19 — 11,267 x 197 ft
			// VOR: 112.60 MHz ALA 119
			Primary: Runway{
				Designator:      "19",
				MagneticHeading: 190.0,
				ThresholdLatLon: orb.Point{55.6083, 24.2833}, // North threshold (RWY 19 landing north→south)
			},
			Reciprocal: Runway{
				Designator:      "01",
				MagneticHeading: 10.0,
				ThresholdLatLon: orb.Point{55.6083, 24.2367}, // South threshold
			},
		},
	},
}

// AllAirfields lists all supported PG ATC airfields.
var AllAirfields = []*Airfield{OMDM, OMAM, OMAL}
