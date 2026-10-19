package lso

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Case I pattern before the groove (operator 2026-10-18, off the vSFG-7
// Case I Recovery kneeboard, p24): initial 3 nm astern on BRC at 800 ft, a
// level break, downwind at 600 ft 1-1.5 nm abeam, the 180 on-speed at 600
// ft, the 90 at 450-500 ft, the 45 at 370-400 ft, then 15-18 s in the
// groove. Paddles keeps every jet's last few minutes in the ship's frame and
// reads the pattern back when the pass ends. Report only: nothing is said.

const (
	patternKeep = 5 * time.Minute
	ftPerM      = 3.28084
)

// patSample is a jet relative to the ship: metres astern of the ship's
// stern along its heading (negative = ahead of it), metres to port of its
// centreline, feet above the water, and its track relative to BRC (0 =
// upwind on BRC, 180 = downwind; the Case I turns are to port).
type patSample struct {
	at       time.Time
	aft      float64
	port     float64
	ft       float64
	trackRel float64
}

// Pattern is what the jet flew before the groove. A field is valid only
// with its Has flag.
type Pattern struct {
	InitialFt, BreakFt   float64
	AbeamNm, AbeamFt     float64
	NinetyFt, FortyFive  float64
	GrooveS              float64
	HasInitial, HasBreak bool
	HasAbeam, HasNinety  bool
	HasFortyFive         bool
	HasGroove            bool
}

// Kneeboard numbers and how far off is worth a note.
const (
	initialFt, breakFt = 800.0, 800.0
	abeamFt            = 600.0
	ninetyLo, ninetyHi = 450.0, 500.0
	fortyLo, fortyHi   = 370.0, 400.0
	abeamLoNm          = 1.0
	abeamHiNm          = 1.5
	grooveLoS          = 15.0
	grooveHiS          = 18.0
	altSlackFt         = 100.0 // initial, break and abeam
	turnSlackFt        = 50.0  // the 90 and the 45
)

// record keeps a sample in the jet's pattern history.
func (t *Tracker) record(callsign string, s Ship, smp Sample, trackDeg float64, haveTrack bool) {
	if !haveTrack {
		return
	}
	if t.hist == nil {
		t.hist = make(map[string][]patSample)
	}
	e, n := local(s, smp.Lat, smp.Lon)
	se, sn := unit(s.HeadingDeg)
	// Stern: SternM along the centreline from the reference point.
	along := e*se + n*sn - t.P.SternM
	starboard := e*sn - n*se
	h := append(t.hist[callsign], patSample{
		at:       smp.At,
		aft:      -along,
		port:     -starboard,
		ft:       smp.AltM * ftPerM,
		trackRel: math.Mod(trackDeg-s.HeadingDeg+720, 360),
	})
	for len(h) > 1 && smp.At.Sub(h[0].at) > patternKeep {
		h = h[1:]
	}
	t.hist[callsign] = h
}

// relOff is how far a track is from a relative heading, in degrees.
func relOff(trackRel, want float64) float64 { return angleDiff(trackRel, want) }

// pattern reads the Case I pattern out of the jet's history up to end (the
// ramp, or the last sample of a pass that never got there).
func (t *Tracker) pattern(callsign string, end time.Time) Pattern {
	var pt Pattern
	h := t.hist[callsign]
	k := len(h)
	for k > 0 && h[k-1].at.After(end) {
		k--
	}
	h = h[:k]
	if len(h) < 10 {
		return pt
	}
	axis := math.Mod(t.P.AngledDeckDeg+360, 360) // the landing axis, relative to BRC
	// Groove: back from the ramp while the track stays within 10 degrees of
	// the landing axis - wings level out of the 45. (The history ends at the
	// ramp; the jet's own point is already a few metres past the stern.)
	i := len(h) - 1
	for i > 0 && relOff(h[i-1].trackRel, axis) <= 10 {
		i--
	}
	if i < len(h)-1 && i > 0 {
		pt.GrooveS, pt.HasGroove = end.Sub(h[i].at).Seconds(), true
	}
	// The 45 and the 90: turn still to go to the landing axis (the angled
	// deck, 9 degrees left of BRC), the last time before the groove (mean over
	// the samples within 8 degrees of it).
	at := func(want float64, from int) (float64, int, bool) {
		sum, n, first := 0.0, 0, -1
		for j := from; j >= 0; j-- {
			if relOff(h[j].trackRel, want) <= 8 {
				sum += h[j].ft
				n++
				first = j
			} else if n > 0 {
				break
			}
		}
		if n == 0 {
			return 0, -1, false
		}
		return sum / float64(n), first, true
	}
	var j45, j90 int
	pt.FortyFive, j45, pt.HasFortyFive = at(math.Mod(axis+45, 360), i)
	from := i
	if pt.HasFortyFive {
		from = j45
	}
	pt.NinetyFt, j90, pt.HasNinety = at(math.Mod(axis+90, 360), from)
	if pt.HasNinety {
		from = j90
	}
	// Abeam: downwind (within 20 degrees of reciprocal), crossing the stern.
	for j := from; j > 0; j-- {
		a, b := h[j-1], h[j]
		if relOff(b.trackRel, 180) > 20 || relOff(a.trackRel, 180) > 20 {
			continue
		}
		if a.aft <= 0 && b.aft > 0 { // downwind, the stern passes from ahead to behind him
			f := -a.aft / (b.aft - a.aft)
			pt.AbeamNm = (a.port + f*(b.port-a.port)) / mPerNm
			pt.AbeamFt = a.ft + f*(b.ft-a.ft)
			pt.HasAbeam = true
			from = j - 1
			break
		}
	}
	if !pt.HasAbeam {
		// No downwind seen: not a Case I pattern (a straight-in, or he joined
		// late). Any "90" or "45" was some other turn.
		pt.HasNinety, pt.HasFortyFive = false, false
		return pt
	}
	// Break: the last time upwind on BRC before the downwind, then the turn
	// off it; the break height is where he is 30 degrees through the turn.
	brk := -1
	for j := from; j > 0; j-- {
		if relOff(h[j].trackRel, 0) <= 10 {
			brk = j
			break
		}
	}
	if brk < 0 {
		return pt
	}
	brkFt, hasBrk := 0.0, false
	for j := brk; j < from; j++ {
		if relOff(h[j].trackRel, 0) >= 30 {
			brkFt, hasBrk = h[j].ft, true
			break
		}
	}
	// Initial: on BRC 2-4 nm astern, before the break.
	sum, n := 0.0, 0
	for j := brk; j >= 0; j-- {
		if relOff(h[j].trackRel, 0) > 15 {
			break
		}
		if h[j].aft > 2*mPerNm && h[j].aft < 4*mPerNm {
			sum += h[j].ft
			n++
		}
	}
	if n > 0 {
		pt.InitialFt, pt.HasInitial = sum/float64(n), true
		// No initial, no break: after a bolter or wave-off he turns downwind
		// off a 600 ft climb, which is not a break at 800.
		pt.BreakFt, pt.HasBreak = brkFt, hasBrk
	}
	return pt
}

// Faults are the kneeboard misses, in plain words for the report.
func (p Pattern) Faults() []string {
	var out []string
	alt := func(name string, v, lo, hi, slack float64) {
		switch {
		case v > hi+slack:
			out = append(out, fmt.Sprintf("high %s (%.0f ft)", name, v))
		case v < lo-slack:
			out = append(out, fmt.Sprintf("low %s (%.0f ft)", name, v))
		}
	}
	if p.HasInitial {
		alt("initial", p.InitialFt, initialFt, initialFt, altSlackFt)
	}
	if p.HasBreak {
		alt("break", p.BreakFt, breakFt, breakFt, altSlackFt)
	}
	if p.HasAbeam {
		switch {
		case p.AbeamNm < abeamLoNm:
			out = append(out, fmt.Sprintf("close abeam (%.1f nm)", p.AbeamNm))
		case p.AbeamNm > abeamHiNm:
			out = append(out, fmt.Sprintf("wide abeam (%.1f nm)", p.AbeamNm))
		}
		alt("abeam", p.AbeamFt, abeamFt, abeamFt, altSlackFt)
	}
	if p.HasNinety {
		alt("at the 90", p.NinetyFt, ninetyLo, ninetyHi, turnSlackFt)
	}
	if p.HasFortyFive {
		alt("at the 45", p.FortyFive, fortyLo, fortyHi, turnSlackFt)
	}
	if p.HasGroove && p.HasAbeam {
		switch {
		case p.GrooveS > grooveHiS:
			out = append(out, fmt.Sprintf("long in the groove (%.0f s)", p.GrooveS))
		case p.GrooveS < grooveLoS:
			out = append(out, fmt.Sprintf("short groove (%.0f s)", p.GrooveS))
		}
	}
	return out
}

// String is the pass line's pattern part: "initial 820 ft, break 790 ft,
// abeam 1.2 nm 610 ft, 90 480 ft, 45 390 ft, groove 16.2 s".
func (p Pattern) String() string {
	var parts []string
	if p.HasInitial {
		parts = append(parts, fmt.Sprintf("initial %.0f ft", p.InitialFt))
	}
	if p.HasBreak {
		parts = append(parts, fmt.Sprintf("break %.0f ft", p.BreakFt))
	}
	if p.HasAbeam {
		parts = append(parts, fmt.Sprintf("abeam %.1f nm %.0f ft", p.AbeamNm, p.AbeamFt))
	}
	if p.HasNinety {
		parts = append(parts, fmt.Sprintf("90 %.0f ft", p.NinetyFt))
	}
	if p.HasFortyFive {
		parts = append(parts, fmt.Sprintf("45 %.0f ft", p.FortyFive))
	}
	if p.HasGroove {
		parts = append(parts, fmt.Sprintf("groove %.1f s", p.GrooveS))
	}
	return strings.Join(parts, ", ")
}
