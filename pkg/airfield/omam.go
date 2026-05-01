package airfield

import "github.com/paulmach/orb"

// OMAM is Al Dhafra Air Base, UAE (Al Dhafra AFB in DCS).
// Data sourced from [USAF] AFD-3351 airport diagram, July 2020.
// Parallel runways 13L/31R (upper) and 13R/31L (lower).
// Heading 128°/308° magnetic (diagram confirmed).
var OMAM = &Airfield{
	ICAO:            "OMAM",
	Name:            "Al Dhafra",
	DCSName:         "Al Dhafra AFB",
	Center:          orb.Point{54.5483, 24.2467}, // [lon, lat] derived from diagram grid
	ElevationFt:     52,
	MagVar:          2.25, // +2.25°E per diagram
	PatternAltFt:    1500,
	TowerFreqMHz:    251.100,
	ApproachFreqMHz: 251.100,
	ATISFreqMHz:     248.2,
	DepartureDistNm:  5,
	DepartureAngels:   3,
	HandoffCallsign:  "command",
	HandoffFreqMHz:   282.000,
	HandoffPreset:    "channel four",
	BreakDirections: map[string]string{
		// Ramp on south/east side. Both 31 ends have ramp on the left side
		// of approach; both 13 ends have it on the right.
		"31L": "left",
		"31R": "left",
		"13L": "right",
		"13R": "right",
	},
	RunwayPairs: []RunwayPair{
		{
			// Upper parallel: 13L / 31R — 11,403 x 150 ft
			// ILS 31R: 109.1 MHz Chan 28 | ILS 13L: 108.7 MHz Chan 24
			Primary: Runway{
				Designator:      "31R",
				MagneticHeading: 308.0,
				ThresholdLatLon: orb.Point{54.5633, 24.2583}, // NE threshold
			},
			Reciprocal: Runway{
				Designator:      "13L",
				MagneticHeading: 128.0,
				ThresholdLatLon: orb.Point{54.5333, 24.2667}, // SW threshold
			},
		},
		{
			// Lower parallel: 13R / 31L — 11,403 x 150 ft
			// ILS 31L: 111.10 MHz Chan 48 | ILS 13R: 114.90 MHz Chan 24
			Primary: Runway{
				Designator:      "31L",
				MagneticHeading: 308.0,
				ThresholdLatLon: orb.Point{54.5633, 24.2250}, // SE threshold
			},
			Reciprocal: Runway{
				Designator:      "13R",
				MagneticHeading: 128.0,
				ThresholdLatLon: orb.Point{54.5333, 24.2417}, // NW threshold
			},
		},
	},
}
