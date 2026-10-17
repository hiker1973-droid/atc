// Package airfield defines static airfield data for ATC operations.
// Runway headings are magnetic. Coordinates are WGS-84 [lon, lat].
package airfield

import (
	"hash/fnv"
	"strconv"
	"time"

	"github.com/paulmach/orb"
)

// RotationSlotDuration is the wall-clock length of one runway-rotation slot.
// Declared as a var (not const) so debug builds can shorten it via the
// --runway-slot flag for live verification without waiting 4h between rolls.
var RotationSlotDuration = 4 * time.Hour

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
	// ParkingFirst..ParkingLast are the numbered parking spots the tower
	// hands out in turn to landing traffic. Zero = the plain "taxi to parking".
	ParkingFirst, ParkingLast int
	// ParkingVia is the taxi route to parking by landing runway, as spoken
	// ("Bravo, then November"); a runway not listed gets no route.
	ParkingVia map[string]string
	// TaxiVia is the taxi route out to the active runway, by runway, as
	// spoken ("November to Alpha"); a runway not listed gets no route.
	TaxiVia map[string]string
}

// ParkingSpot returns the n-th spot (0-based, wrapping); ok is false when the
// field has none.
func (a *Airfield) ParkingSpot(n int) (spot int, ok bool) {
	total := a.ParkingLast - a.ParkingFirst + 1
	if a.ParkingFirst <= 0 || total <= 0 {
		return 0, false
	}
	return a.ParkingFirst + n%total, true
}

// ThresholdFor returns the landing threshold of the runway with the given
// designator. The boolean is false if no runway matches — callers should
// fall open when this happens (treat the position check as a no-op).
func (a *Airfield) ThresholdFor(designator string) (orb.Point, bool) {
	for _, pair := range a.RunwayPairs {
		if pair.Primary.Designator == designator {
			return pair.Primary.ThresholdLatLon, true
		}
		if pair.Reciprocal.Designator == designator {
			return pair.Reciprocal.ThresholdLatLon, true
		}
	}
	return orb.Point{}, false
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

// RotationRunway returns the active runway for a time-based rotation cycle
// anchored at startTime. Cycle order visits every runway end (Primary then
// Reciprocal of each pair, in pair-declaration order); at OMAM with two
// parallels this gives 31L → 13R → 31R → 13L over 16h.
//
// Unlike ActiveRunway, this ignores wind — pilots will sometimes land
// downwind. Intentional, for training variety.
func (a *Airfield) RotationRunway(now, startTime time.Time) Runway {
	elapsed := now.Sub(startTime)
	if elapsed < 0 {
		elapsed = 0
	}
	slot := int(elapsed / RotationSlotDuration)

	cycle := make([]Runway, 0, len(a.RunwayPairs)*2)
	for _, pair := range a.RunwayPairs {
		cycle = append(cycle, pair.Primary, pair.Reciprocal)
	}
	if len(cycle) == 0 {
		return Runway{}
	}
	return cycle[slot%len(cycle)]
}

// CalmRunway is the runway for light-wind days (--runway-calm-kts): when the
// wind is too light to matter, the field still changes runways, one per
// RotationSlotDuration of wall-clock time. The slot is counted from the Unix
// epoch, not process start, so a restart keeps the runway and the tower and
// ATIS agree. The runway end for a slot is a hash of the field and the slot
// number: a plain cycle would put the same runway at the same time of day
// every day (six 4h slots, two ends), so an evening session would never see
// the other end. Operator 2026-10-14 ("do 4").
func (a *Airfield) CalmRunway(now time.Time) Runway {
	cycle := make([]Runway, 0, len(a.RunwayPairs)*2)
	for _, pair := range a.RunwayPairs {
		cycle = append(cycle, pair.Primary, pair.Reciprocal)
	}
	if len(cycle) == 0 {
		return Runway{}
	}
	return cycle[CalmSlotIndex(a.ICAO, CalmSlot(now), len(cycle))]
}

// CalmSlot is the wall-clock rotation slot now falls in.
func CalmSlot(now time.Time) int64 {
	return now.Unix() / int64(RotationSlotDuration/time.Second)
}

// CalmSlotIndex picks one of n runway ends for a field's slot.
func CalmSlotIndex(icao string, slot int64, n int) int {
	h := fnv.New32a()
	h.Write([]byte(icao + "|" + strconv.FormatInt(slot, 10)))
	return int(h.Sum32() % uint32(n))
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
