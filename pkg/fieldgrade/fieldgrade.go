// Package fieldgrade grades landings at an airfield the way Paddles grades
// traps: glide path, lineup and AoA through four segments of final, then the
// touchdown, from Tacview positions. Report only - nothing is said on the
// radio (operator 2026-10-18: "report only for test. Senaki is the only
// field").
//
// The numbers come from the vSFG-7 kneeboards (Airfield Arrivals, page 15):
// final turn at 450 ft AGL, rolled out at 350 ft AGL 1-1.5 nm from the
// runway, -3 to -3.5 degrees on the pitch ladder, 500-700 fpm on short final
// and never over 1000, AoA held to touchdown. DCS grades no field landings,
// so unlike Paddles there is no debrief to match: these are our standard.
package fieldgrade

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	nm = 1852.0
	// GlidepathDeg is the path graded against: 3 degrees to AimM past the
	// threshold. Senaki 2026-10-17 (Raider 303) flew 3.0-3.2 degrees to a
	// touchdown 315 m past the 09 threshold.
	GlidepathDeg = 3.0
	AimM         = 300.0
	// refAboveRunwayM: a Hornet's Tacview point sits ~2.3 m above the runway
	// on its wheels (Senaki, 15.4 m rolling, field 13.1 m). The runway level
	// is taken from the jet's own rollout when there is one.
	refAboveRunwayM = 2.3
	hornetOnSpeed   = 8.1
	keep            = 3 * time.Minute
)

// End is one landing direction: its threshold, the true course flown to land
// on it and the runway length ahead of it.
type End struct {
	Name      string
	Lat, Lon  float64
	CourseDeg float64
	LengthM   float64
}

// Field is the airfield graded.
type Field struct {
	Name  string
	ElevM float64
	Ends  []End
}

// Threshold is one runway end's designator and landing threshold.
type Threshold struct {
	Name     string
	Lat, Lon float64
}

// NewField builds both ends of each runway from its two thresholds - the
// course and length come from the line between them.
func NewField(name string, elevFt float64, pairs ...[2]Threshold) Field {
	f := Field{Name: name, ElevM: elevFt * 0.3048}
	for _, p := range pairs {
		a, b := p[0], p[1]
		e, n := enu(a.Lat, a.Lon, b.Lat, b.Lon)
		crs := math.Mod(math.Atan2(e, n)*180/math.Pi+360, 360)
		l := math.Hypot(e, n)
		f.Ends = append(f.Ends,
			End{Name: a.Name, Lat: a.Lat, Lon: a.Lon, CourseDeg: crs, LengthM: l},
			End{Name: b.Name, Lat: b.Lat, Lon: b.Lon, CourseDeg: math.Mod(crs+180, 360), LengthM: l})
	}
	return f
}

// enu is the east/north metres from (lat0, lon0) to (lat, lon).
func enu(lat0, lon0, lat, lon float64) (float64, float64) {
	return (lon - lon0) * 111320 * math.Cos(lat0*math.Pi/180), (lat - lat0) * 110540
}

// rel is a position relative to a runway end: metres before the threshold
// along the approach (negative = over the runway) and metres right of the
// extended centreline.
func (e End) rel(lat, lon float64) (before, right float64) {
	de, dn := enu(e.Lat, e.Lon, lat, lon)
	ce, cn := math.Sin(e.CourseDeg*math.Pi/180), math.Cos(e.CourseDeg*math.Pi/180)
	along := de*ce + dn*cn
	return -along, de*cn - dn*ce
}

// Sample is one Tacview position.
type Sample struct {
	At       time.Time
	Lat, Lon float64
	AltM     float64 // MSL
	PitchDeg float64
	HasPitch bool
}

type track struct {
	cs, model string
	buf       []Sample
	end       int       // runway end flown to, -1 none
	from      time.Time // when he turned onto base/final for it
	minH      float64
	onRwy     time.Time // first of the run of samples rolling on the runway
	landed    time.Time
	airborne  bool // left the runway again after landing: touch and go
	last      time.Time
}

// Tracker follows every jet near the field and grades each landing or
// go-around. Not safe for concurrent use.
type Tracker struct {
	F      Field
	Done   func(Result) // called once per graded pass
	tracks map[string]*track
}

func NewTracker(f Field) *Tracker {
	return &Tracker{F: f, tracks: make(map[string]*track)}
}

func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b+540, 360) - 180
	return math.Abs(d)
}

// hdgOf is the track over the ground between two samples.
func hdgOf(a, b Sample) float64 {
	e, n := enu(a.Lat, a.Lon, b.Lat, b.Lon)
	return math.Mod(math.Atan2(e, n)*180/math.Pi+360, 360)
}

func (t *Tracker) height(s Sample) float64 { return s.AltM - t.F.ElevM - refAboveRunwayM }

// Update feeds one position of a player jet.
func (t *Tracker) Update(callsign, model string, s Sample) {
	tr := t.tracks[callsign]
	if tr == nil {
		tr = &track{cs: callsign, model: model, end: -1}
		t.tracks[callsign] = tr
	}
	tr.model, tr.last = model, s.At
	tr.buf = append(tr.buf, s)
	for len(tr.buf) > 2 && s.At.Sub(tr.buf[0].At) > keep {
		tr.buf = tr.buf[1:]
	}
	h := t.height(s)
	if tr.end < 0 {
		// On approach: inside 2 nm of a threshold, below 1150 ft, and pointed
		// at the runway or turning base/final onto it (not downwind). Flying:
		// a jet rolling out or taxiing is not starting an approach.
		if h > 350 || h < 15 || len(tr.buf) < 2 {
			return
		}
		hdg := hdgOf(tr.buf[len(tr.buf)-2], s)
		for i, e := range t.F.Ends {
			before, right := e.rel(s.Lat, s.Lon)
			if before > 0 && before < 2*nm && math.Abs(right) < 1.2*nm && angleDiff(hdg, e.CourseDeg) < 120 {
				tr.end, tr.minH, tr.from = i, h, s.At
				return
			}
		}
		return
	}
	e := t.F.Ends[tr.end]
	before, right := e.rel(s.Lat, s.Lon)
	tr.minH = math.Min(tr.minH, h)
	onRwy := before < 60 && before > -e.LengthM-150 && math.Abs(right) < 40 && h < 2.0
	switch {
	case onRwy && tr.landed.IsZero():
		if tr.onRwy.IsZero() {
			tr.onRwy = s.At
		}
		if s.At.Sub(tr.onRwy) >= 2*time.Second {
			tr.landed = tr.onRwy
		}
	case !onRwy && tr.landed.IsZero():
		tr.onRwy = time.Time{}
	}
	switch {
	case !tr.landed.IsZero():
		n := len(tr.buf)
		slow := n > 1 && groundSpeed(tr.buf[n-2], s) < 20 // ~40 kt
		if !onRwy && h > 15 {
			tr.airborne = true
			t.finish(tr)
		} else if slow || s.At.Sub(tr.landed) > 60*time.Second || before < -e.LengthM-150 {
			t.finish(tr)
		}
	case h > tr.minH+120 || before < -e.LengthM || before > 4*nm || math.Abs(right) > 2*nm:
		// Climbing away, past the far end, or gone: a go-around if he got
		// down to 300 ft, otherwise he was only passing through.
		t.finish(tr)
	}
}

func groundSpeed(a, b Sample) float64 {
	dt := b.At.Sub(a.At).Seconds()
	if dt <= 0 {
		return 0
	}
	e, n := enu(a.Lat, a.Lon, b.Lat, b.Lon)
	return math.Hypot(e, n) / dt
}

// Expire finishes passes whose jet has not been seen for 20 s (despawned,
// disconnected, shut down on the runway).
func (t *Tracker) Expire(now time.Time) {
	for cs, tr := range t.tracks {
		if now.Sub(tr.last) > 20*time.Second {
			if tr.end >= 0 {
				t.finish(tr)
			}
			delete(t.tracks, cs)
		}
	}
}

func (t *Tracker) finish(tr *track) {
	defer func() {
		tr.end, tr.landed, tr.onRwy, tr.airborne = -1, time.Time{}, time.Time{}, false
		tr.buf = tr.buf[len(tr.buf)-1:]
	}()
	if tr.landed.IsZero() && tr.minH > 90 {
		return // never below 300 ft: not an approach
	}
	// From just before he turned in: the buffer also holds the break, which
	// passes over the threshold at 1500 ft on runway heading.
	k := 0
	for k < len(tr.buf)-1 && tr.buf[k].At.Before(tr.from.Add(-10*time.Second)) {
		k++
	}
	r := t.Analyze(tr.cs, tr.model, tr.end, tr.buf[k:], tr.landed, tr.airborne)
	if t.Done != nil {
		t.Done(r)
	}
}

// Seg is one segment's mean errors: glide path and lineup in degrees
// (positive = high / right), sink in fpm, estimated AoA.
type Seg struct {
	Name         string
	GP, LU       float64
	SinkFpm      float64
	AoA          float64
	HasGP, HasLU bool
	HasAoA       bool
	TurningPct   float64 // share of the segment still turning onto final
	samples, nLU int
	nAoA, nTurn  int
	sinkN        int
}

// Segments of final, in metres before the threshold - Paddles' X, IM, IC
// and AR, stretched to a runway: 1.5 nm (where the kneeboard's rollout ends)
// to 1 nm, 1 to 1/2, 1/2 to 1/4, and the last 1/4 nm to the threshold.
var Segments = []struct {
	Name     string
	From, To float64
}{
	{"X", 1.5 * nm, 1.0 * nm},
	{"IM", 1.0 * nm, 0.5 * nm},
	{"IC", 0.5 * nm, 0.25 * nm},
	{"AR", 0.25 * nm, 0},
}

// Result is one graded pass.
type Result struct {
	Callsign, Model string
	Runway          string
	Outcome         string // "landed", "touch and go", "go-around"
	Grade           string // "OK: (LO)IM", "WO", ...
	Segs            []Seg
	// Touchdown, metres past the threshold and right of the centreline,
	// and the sink rate into it.
	TDPastM, TDRightM, TDSinkFpm float64
	HasTD                        bool
	// Rollout: where he was wings-level on final, nm out and ft AGL.
	RolloutNm, RolloutFt float64
	HasRollout           bool
	// Final turn: height while 90-45 degrees off the runway heading.
	TurnFt    float64
	HasTurn   bool
	LowestFt  float64
	Samples   int
	Start, At time.Time
}

// Analyze grades one pass over its samples. landed is when he was first on
// the runway (zero for a go-around); airborne: he took off again.
func (t *Tracker) Analyze(cs, model string, endIdx int, buf []Sample, landed time.Time, airborne bool) Result {
	e := t.F.Ends[endIdx]
	r := Result{Callsign: cs, Model: model, Runway: e.Name, Samples: len(buf)}
	if len(buf) > 0 {
		r.Start, r.At = buf[0].At, buf[len(buf)-1].At
	}
	switch {
	case landed.IsZero():
		r.Outcome = "go-around"
	case airborne:
		r.Outcome = "touch and go"
	default:
		r.Outcome = "landed"
	}
	// Runway level: the median of his own rolling heights, else the field
	// elevation plus a Hornet's height on its wheels.
	ground := t.F.ElevM + refAboveRunwayM
	var roll []float64
	for _, s := range buf {
		before, right := e.rel(s.Lat, s.Lon)
		if !landed.IsZero() && !s.At.Before(landed) && before < 0 && math.Abs(right) < 40 && s.AltM < t.F.ElevM+refAboveRunwayM+2 {
			roll = append(roll, s.AltM)
		}
	}
	if len(roll) >= 3 {
		sort.Float64s(roll)
		ground = roll[len(roll)/2]
	}
	type pt struct {
		before, right, h, hdg, gs, vs, aoa float64
		hasAoA                             bool
		s                                  Sample
	}
	pts := make([]pt, len(buf))
	for i, s := range buf {
		before, right := e.rel(s.Lat, s.Lon)
		p := pt{before: before, right: right, h: s.AltM - ground, s: s}
		a, b := buf[max(0, i-3)], buf[min(len(buf)-1, i+3)]
		if dt := b.At.Sub(a.At).Seconds(); dt > 0 {
			p.gs = groundSpeed(a, b)
			p.vs = (b.AltM - a.AltM) / dt
			p.hdg = hdgOf(a, b)
			if s.HasPitch && p.gs > 30 {
				p.aoa, p.hasAoA = s.PitchDeg-math.Atan2(p.vs, p.gs)*180/math.Pi, true
			}
		}
		pts[i] = p
	}
	// Touchdown: the first time down to the runway level, interpolated.
	td := -1
	if !landed.IsZero() {
		for i, p := range pts {
			if !p.s.At.Before(landed.Add(-5*time.Second)) && p.before < 60 && p.h < 0.4 {
				td = i
				break
			}
		}
	}
	if td > 0 {
		a, b := pts[td-1], pts[td]
		f := 1.0
		if a.h > b.h {
			f = (a.h - 0.4) / (a.h - b.h)
			f = math.Max(0, math.Min(1, f))
		}
		r.TDPastM = -(a.before + f*(b.before-a.before))
		r.TDRightM = a.right + f*(b.right-a.right)
		// Sink into it: over the last 2 s airborne.
		k := td - 1
		for k > 0 && pts[td-1].s.At.Sub(pts[k].s.At) < 2*time.Second {
			k--
		}
		if dt := pts[td].s.At.Sub(pts[k].s.At).Seconds(); dt > 0 {
			r.TDSinkFpm = -(pts[td].s.AltM - pts[k].s.AltM) / dt * 196.85
		}
		r.HasTD = true
	}
	last := len(pts)
	if td >= 0 {
		last = td
	}
	r.LowestFt = math.Inf(1)
	// Rollout: the start of the run of wings-level, on-course samples that
	// ends at touchdown (or the threshold for a go-around).
	ro := -1
	for i := last - 1; i >= 0; i-- {
		p := pts[i]
		if p.before < 0 {
			continue
		}
		if angleDiff(p.hdg, e.CourseDeg) > 8 || math.Abs(p.right) > 0.25*nm {
			break
		}
		ro = i
	}
	if ro >= 0 && pts[ro].before > 0 {
		r.RolloutNm, r.RolloutFt, r.HasRollout = pts[ro].before/nm, pts[ro].h*3.2808, true
	}
	var turnSum float64
	var turnN int
	for i := 0; i < last; i++ {
		p := pts[i]
		if p.h*3.2808 < r.LowestFt {
			r.LowestFt = p.h * 3.2808
		}
		if d := angleDiff(p.hdg, e.CourseDeg); d >= 45 && d <= 90 && p.before > 0 && p.before < 2.5*nm {
			turnSum += p.h * 3.2808
			turnN++
		}
	}
	if turnN > 0 {
		r.TurnFt, r.HasTurn = turnSum/float64(turnN), true
	}
	for _, sg := range Segments {
		m := Seg{Name: sg.Name}
		for i := 0; i < last; i++ {
			p := pts[i]
			if p.before > sg.From || p.before <= sg.To || p.h < 0.4 ||
				math.Abs(p.right) > 1000 || angleDiff(p.hdg, e.CourseDeg) > 100 {
				continue // not on final yet (downwind abeam the threshold)
			}
			d := p.before + AimM
			m.GP += math.Atan2(p.h, d)*180/math.Pi - GlidepathDeg
			m.samples++
			m.SinkFpm += -p.vs * 196.85
			m.sinkN++
			if angleDiff(p.hdg, e.CourseDeg) > 20 {
				m.nTurn++
			}
			// Lineup from the rollout on: before it he is still turning in,
			// and a late rollout is LRO once, not LRO and LUL in two places.
			if ro >= 0 && i >= ro {
				// Lineup as an angle from the aim point, but not finer than
				// a 600 m lever: over the threshold a jet 6 m off the
				// centreline is not "lined up right".
				m.LU += math.Atan2(p.right, math.Max(d, 600)) * 180 / math.Pi
				m.nLU++
			}
			if p.hasAoA {
				m.AoA += p.aoa
				m.nAoA++
			}
		}
		if m.samples > 0 {
			m.GP /= float64(m.samples)
			m.SinkFpm /= float64(m.sinkN)
			m.HasGP = true
			m.TurningPct = 100 * float64(m.nTurn) / float64(m.samples)
		}
		if m.nLU > 0 {
			m.LU /= float64(m.nLU)
			m.HasLU = true
		}
		if m.nAoA > 0 {
			m.AoA /= float64(m.nAoA)
			m.HasAoA = true
		}
		r.Segs = append(r.Segs, m)
	}
	r.Grade = grade(r)
	return r
}

// grade puts the pass in Paddles' notation: "(LO)IM" is a little low in the
// middle, "LOIC" a lot low in close; three big errors is "---", one or two
// "(OK)", none "OK", and nothing at all with a touchdown in the zone "_OK_".
// Field-only codes: LRO late rollout, SINK fast descent, SHT/LNG touchdown
// short/long, HARD a firm one, TD lineup at touchdown.
func grade(r Result) string {
	var errs []string
	majors := 0
	add := func(code, where string, big bool) {
		if big {
			errs = append(errs, code+where)
			majors++
			return
		}
		errs = append(errs, "("+code+")"+where)
	}
	if r.HasRollout {
		// Kneeboard: rolled out 1-1.5 nm. Inside 1/2 nm is a big one.
		switch {
		case r.RolloutNm < 0.5:
			add("LRO", "", true)
		case r.RolloutNm < 0.9:
			add("LRO", "", false)
		}
	} else if r.Outcome != "go-around" {
		add("LRO", "", true) // never wings level on final
	}
	hornet := strings.Contains(strings.ToLower(r.Model), "fa-18")
	for _, m := range r.Segs {
		if !m.HasGP {
			continue
		}
		switch {
		case m.GP > 1.0:
			add("H", m.Name, true)
		case m.GP > 0.5:
			add("H", m.Name, false)
		case m.GP < -0.8:
			add("LO", m.Name, true)
		case m.GP < -0.4:
			add("LO", m.Name, false)
		}
		if m.HasLU {
			big := m.Name != "X" // still rolling out at X
			switch {
			case m.LU < -2.0:
				add("LUL", m.Name, big)
			case m.LU < -1.0:
				add("LUL", m.Name, false)
			case m.LU > 2.0:
				add("LUR", m.Name, big)
			case m.LU > 1.0:
				add("LUR", m.Name, false)
			}
		}
		if m.Name == "IC" || m.Name == "AR" {
			// Kneeboard: 500-700 fpm on short final, never over 1000.
			switch {
			case m.SinkFpm > 1000:
				add("SINK", m.Name, true)
			case m.SinkFpm > 850:
				add("SINK", m.Name, false)
			}
		}
		if hornet && m.HasAoA && m.Name != "X" {
			switch {
			case m.AoA < hornetOnSpeed-1.5:
				add("F", m.Name, true)
			case m.AoA < hornetOnSpeed-0.75:
				add("F", m.Name, false)
			case m.AoA > hornetOnSpeed+1.5:
				add("SLO", m.Name, true)
			case m.AoA > hornetOnSpeed+0.75:
				add("SLO", m.Name, false)
			}
		}
	}
	with := func(g string) string {
		if len(errs) == 0 {
			return g
		}
		return g + ": " + strings.Join(errs, " ")
	}
	if !r.HasTD {
		if r.Outcome == "go-around" {
			return with("WO")
		}
		return with("NG") // landed but no touchdown seen (gap in the data)
	}
	short := r.TDPastM < 0
	switch {
	case short:
		add("SHT", "TD", true)
	case r.TDPastM < 100:
		add("SHT", "TD", false)
	case r.TDPastM > 900:
		add("LNG", "TD", true)
	case r.TDPastM > 600:
		add("LNG", "TD", false)
	}
	switch {
	case r.TDRightM < -15:
		add("LUL", "TD", true)
	case r.TDRightM < -8:
		add("LUL", "TD", false)
	case r.TDRightM > 15:
		add("LUR", "TD", true)
	case r.TDRightM > 8:
		add("LUR", "TD", false)
	}
	switch {
	case r.TDSinkFpm > 1200:
		add("HARD", "", true)
	case r.TDSinkFpm > 900:
		add("HARD", "", false)
	}
	g := "OK"
	switch {
	case short:
		g = "C" // landed before the threshold: unsafe
	case majors >= 3:
		g = "---"
	case majors > 0:
		g = "(OK)"
	case len(errs) == 0 && r.TDPastM >= 150 && r.TDPastM <= 450:
		g = "_OK_"
	}
	return with(g)
}

// Summary is the one-line log record the daily report reads.
func (r Result) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s): rwy %s %s, grade %s", r.Callsign, r.Model, r.Runway, r.Outcome, r.Grade)
	if r.HasTD {
		fmt.Fprintf(&b, ", touched %.0f m past the threshold, %+.0f m right, %.0f fpm", r.TDPastM, r.TDRightM, r.TDSinkFpm)
	}
	if r.HasRollout {
		fmt.Fprintf(&b, ", rolled out %.1f nm %.0f ft", r.RolloutNm, r.RolloutFt)
	}
	if r.HasTurn {
		fmt.Fprintf(&b, ", final turn %.0f ft", r.TurnFt)
	}
	if r.Outcome == "go-around" && !math.IsInf(r.LowestFt, 1) {
		fmt.Fprintf(&b, ", lowest %.0f ft", r.LowestFt)
	}
	fmt.Fprintf(&b, "; %d samples", r.Samples)
	for _, m := range r.Segs {
		if !m.HasGP {
			continue
		}
		fmt.Fprintf(&b, "; %s GP %+.1f°", m.Name, m.GP)
		if m.HasLU {
			fmt.Fprintf(&b, " LU %+.1f°", m.LU)
		}
		fmt.Fprintf(&b, " sink %.0f", m.SinkFpm)
		if m.HasAoA {
			fmt.Fprintf(&b, " AoA %.1f", m.AoA)
		}
		if m.TurningPct >= 50 {
			b.WriteString(" (turning)")
		}
	}
	return b.String()
}
