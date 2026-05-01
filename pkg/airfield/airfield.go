// Package airfield defines static airfield data for ATC operations.
// Runway headings are magnetic. Coordinates are WGS-84 [lon, lat].
package airfield

import "github.com/paulmach/orb"

// Runway defines a single runway end.
type Runway struct {
	// Designator is the runway number, e.g. "27" or "31L"
	Designator string
	// MagneticHeading is the runway heading in degrees magnetic (0–360).
	MagneticHeading float64
	// ThresholdLatLon is the landing threshold position [lon, lat].
	ThresholdLatLon orb.Point
}

// RunwayPair is a reciprocal pair, e.g. 09/27 or 13L/31R.
type RunwayPair struct {
	Primary    Runway
	Reciprocal Runway
}

// Airfield defines a DCS airfield for ATC operations.
type Airfield struct {
	// ICAO is the 4-letter ICAO identifier.
	ICAO string
	// Name is the human-readable name used in ATC phraseology.
	Name string
	// DCSName is the name exactly as it appears in DCS Mission Editor.
	DCSName string
	// Center is the approximate airfield center [lon, lat].
	Center orb.Point
	// ElevationFt is field elevation in feet MSL.
	ElevationFt int
	// MagVar is magnetic variation in degrees (positive = East).
	MagVar float64
	// PatternAltFt is traffic pattern altitude in feet AGL.
	PatternAltFt int
	// TowerFreqMHz is the tower control frequency.
	TowerFreqMHz float64
	// ApproachFreqMHz is the approach/departure control frequency.
	ApproachFreqMHz float64
	// ATISFreqMHz is the ATIS frequency (0 if none configured).
	ATISFreqMHz float64
	// DepartureDistNm is how far (nm) to proceed on runway heading before free to navigate.
	DepartureDistNm int
	// DepartureAngels is the altitude (thousands ft) to maintain until free to navigate.
	DepartureAngels int
	// HandoffCallsign is what ATC calls the next controller, e.g. "command".
	HandoffCallsign string
	// HandoffFreqMHz is the frequency of the next controller in MHz.
	HandoffFreqMHz float64
	// HandoffPreset is the preset description spoken on radio, e.g. "channel four".
	HandoffPreset string
	// RunwayPairs lists all runway pairs at the airfield.
	RunwayPairs []RunwayPair
	// BreakDirections maps a runway designator (e.g. "27") to the overhead-break
	// direction ("left" or "right") used at the 3-mile initial call. Determined
	// by the ramp/terminal location relative to that runway's approach heading.
	// Missing key → composer falls back to "left".
	BreakDirections map[string]string
}

// ActiveRunway returns the runway end most into-wind given a magnetic wind-from
// direction and speed. Returns the primary runway of the first pair when calm.
func (a *Airfield) ActiveRunway(windFromMag float64, windKts float64) Runway {
	if windKts < 3 {
		return a.RunwayPairs[0].Primary
	}
	best := a.RunwayPairs[0].Primary
	bestDiff := headingDiff(best.MagneticHeading, windFromMag)
	for _, pair := range a.RunwayPairs {
		for _, rwy := range []Runway{pair.Primary, pair.Reciprocal} {
			if d := headingDiff(rwy.MagneticHeading, windFromMag); d < bestDiff {
				bestDiff = d
				best = rwy
			}
		}
	}
	return best
}

// headingDiff returns the absolute angular difference between two headings (0–180).
func headingDiff(a, b float64) float64 {
	d := a - b
	for d < 0 {
		d += 360
	}
	for d >= 360 {
		d -= 360
	}
	if d > 180 {
		d = 360 - d
	}
	return d
}
