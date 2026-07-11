package airfield

import "github.com/paulmach/orb"

// EDDL is Düsseldorf — Cold War Germany theatre (COMM1 recovery base).
// Name spoken as "Dusseldorf"; add a field-name alias if pilots say "Düsseldorf"
// and callsign matching misses it. See etar.go for provenance/caveats.
var EDDL = &Airfield{
	ICAO:            "EDDL",
	Name:            "Dusseldorf",
	DCSName:         "Dusseldorf", // VERIFY exact ME name
	Center:          orb.Point{6.7668, 51.2895},
	ElevationFt:     121,
	MagVar:          0.0,
	PatternAltFt:    1500,
	TowerFreqMHz:    118.30, // VHF tower; UHF tower 255.25
	ApproachFreqMHz: 118.30,
	ATISFreqMHz:     248.70, // UHF ATIS; simulcast VHF 128.35
	DepartureDistNm: 7,
	DepartureAngels: 3,
	HandoffCallsign: "command",
	HandoffFreqMHz:  282.000,
	HandoffPreset:   "channel one",
	BreakDirections: map[string]string{
		"05": "left", // TODO verify
		"23": "left",
	},
	RunwayPairs: []RunwayPair{
		{
			// 05/23 — VOR 115.15 · ILS 111.50 (05) / 109.90 (23)
			Primary:    Runway{Designator: "05", MagneticHeading: 65.0, ThresholdLatLon: orb.Point{6.7473, 51.2838}},
			Reciprocal: Runway{Designator: "23", MagneticHeading: 245.0, ThresholdLatLon: orb.Point{6.7863, 51.2952}},
		},
	},
}
