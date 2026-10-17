// Package lso is SkyEye's Paddles: it watches jets fly the groove to the
// carrier from Tacview positions and decides what the LSO says. It is pure
// (no sockets, no audio) so the geometry and the calls can be tested with
// synthetic tracks; cmd/atc/lso.go feeds it Tacview and speaks its calls.
//
// Operator 2026-10-17: own role on 128.300 (Hornet COMM2 CH4), OpenAI voice,
// live calls from the first build. Tacview gives position, pitch and heading
// at ~4.7 Hz and no AoA, so on-speed is estimated from pitch minus the flight
// path angle and only called when it is well off.
package lso

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Deck geometry for a Nimitz-class carrier in DCS, measured from the ship's
// Tacview reference point along its centreline (MOOSE AIRBOSS's Nimitz values).
// LandingLateralM and the touchdown point are first guesses: every trap is
// logged with where it touched down so they can be calibrated.
type Params struct {
	DeckHeightM     float64 // deck above the ship reference point
	SternM          float64 // stern, metres along the centreline (negative = aft)
	WiresFromSternM [4]float64
	AngledDeckDeg   float64 // landing-area axis relative to the ship's heading
	LandingLateralM float64 // landing point offset from the centreline, starboard positive
	GlideslopeDeg   float64
	HookBelowRefM   float64 // jet's Tacview reference point above its hook; the glideslope is flown by the hook
}

// NimitzParams are the defaults for CVN-72 Abe.
func NimitzParams() Params {
	return Params{
		DeckHeightM:     20.1,
		SternM:          -164,
		WiresFromSternM: [4]float64{55, 67, 79, 92},
		AngledDeckDeg:   -9.14,
		LandingLateralM: 0,
		GlideslopeDeg:   3.5,
		HookBelowRefM:   2.5,
	}
}

// landingFromRefM is the target touchdown point (the 3 wire), metres along the
// ship's centreline from its reference point.
func (p Params) landingFromRefM() float64 { return p.SternM + p.WiresFromSternM[2] }

// Ship is the carrier at one instant.
type Ship struct {
	Lat, Lon   float64
	HeadingDeg float64 // true, direction of travel
	SpeedMps   float64
}

// Sample is one Tacview position of a jet.
type Sample struct {
	At       time.Time
	Lat, Lon float64
	AltM     float64 // MSL
	PitchDeg float64
	HasPitch bool
}

// groovePos is a jet in the landing area's frame.
type groovePos struct {
	astern float64 // metres behind the landing point along the landing axis (negative = past it)
	right  float64 // metres right of the landing axis
	height float64 // metres above the deck
}

const (
	mPerDegLat = 110540.0
	mPerNm     = 1852.0
	mpsPerKt   = 0.514444
)

// local returns (east, north) metres of (lat, lon) from the ship.
func local(s Ship, lat, lon float64) (float64, float64) {
	return (lon - s.Lon) * 111320.0 * math.Cos(s.Lat*math.Pi/180), (lat - s.Lat) * mPerDegLat
}

func unit(deg float64) (float64, float64) {
	r := deg * math.Pi / 180
	return math.Sin(r), math.Cos(r) // east, north
}

// toGroove puts a sample in the landing frame.
func (p Params) toGroove(s Ship, smp Sample) groovePos {
	e, n := local(s, smp.Lat, smp.Lon)
	// Landing point: along the ship's centreline from the reference point,
	// then sideways.
	se, sn := unit(s.HeadingDeg)
	re, rn := unit(s.HeadingDeg + 90)
	le := se*p.landingFromRefM() + re*p.LandingLateralM
	ln := sn*p.landingFromRefM() + rn*p.LandingLateralM
	de, dn := e-le, n-ln
	ae, an := unit(s.HeadingDeg + p.AngledDeckDeg)
	be, bn := unit(s.HeadingDeg + p.AngledDeckDeg + 90)
	return groovePos{
		astern: -(de*ae + dn*an),
		right:  de*be + dn*bn,
		height: smp.AltM - p.DeckHeightM - p.HookBelowRefM,
	}
}

// Deviations of a groove position in degrees: glideslope (+ high) and lineup
// (+ right of centreline).
func (p Params) deviations(g groovePos) (gs, lineup float64) {
	x := math.Max(g.astern, 1)
	return math.Atan2(g.height, x)*180/math.Pi - p.GlideslopeDeg, math.Atan2(g.right, x) * 180 / math.Pi
}

// Call is something Paddles says.
type Call string

const (
	CallRogerBall   Call = "Roger ball."
	CallClara       Call = "Paddles contact, keep it coming."
	CallCallTheBall Call = "Call the ball."
	CallHigh        Call = "You're high."
	CallLittleHigh  Call = "A little high."
	CallLow         Call = "You're low."
	CallPower       Call = "Power!" // stern (operator 2026-10-17)
	CallRight       Call = "Right for lineup."
	CallLeft        Call = "Come left."
	CallFast        Call = "You're fast."
	CallSlow        Call = "You're slow."
	CallWaveOff     Call = "Wave off, wave off."
	CallBolter      Call = "Bolter, bolter, bolter."
)

// AllCalls lists every fixed line, so the role can render them at start-up
// and speak them without waiting for TTS.
var AllCalls = []Call{CallRogerBall, CallClara, CallCallTheBall, CallHigh, CallLittleHigh, CallLow, CallPower,
	CallRight, CallLeft, CallFast, CallSlow, CallWaveOff, CallBolter}

// Thresholds. Distances are metres astern of the landing point.
const (
	grooveStartM    = 2400.0 // ~1.3 nm: a pass starts inside this
	grooveMinM      = 120.0  // in the gate at least this far out
	grooveMaxHeight = 450.0  // and below ~1,500 ft
	grooveMaxLineup = 12.0   // and within this many degrees of centreline
	grooveMaxHdgOff = 35.0   // and pointing roughly down the landing axis
	callsFromM      = 1500.0 // deviation calls from ~0.8 nm (earlier after a ball call)
	callsUntilM     = 70.0   // nothing but wave off over the ramp
	callTheBallM    = 1100.0
	gapMin          = 2 * time.Second
	sameCallGap     = 4500 * time.Millisecond
	lostAfter       = 6 * time.Second
	onDeckM         = 1.0 // hook height above the deck
	trapRelKts      = 20.0
	bolterRelKts    = 60.0
	hornetOnSpeed   = 8.1
)

// Pass is one jet's approach, from entering the groove to trap, bolter or
// leaving.
type Pass struct {
	Callsign string
	Model    string
	Started  time.Time

	samples   []passSample
	ballAt    time.Time
	clara     bool
	askedBall bool
	lastCall  time.Time
	lastSaid  map[Call]time.Time
	calls     []string
	waveOff   bool

	waveOffRepeated bool

	touchdown   *passSample
	bolterSaid  bool
	outcome     string // "trap", "bolter", "wave off", "no touch", "left"
	minAstern   float64
	trapStopFwd float64
}

type passSample struct {
	at          time.Time
	g           groovePos
	gs, lineup  float64
	aoa         float64
	hasAoA      bool
	relSpeedKts float64
}

// Tracker holds the passes in progress.
type Tracker struct {
	P      Params
	passes map[string]*Pass
	prev   map[string]Sample
	// Done receives each finished pass's summary line.
	Done func(summary string)
}

// NewTracker returns a Tracker with p.
func NewTracker(p Params) *Tracker {
	return &Tracker{P: p, passes: make(map[string]*Pass), prev: make(map[string]Sample)}
}

// InGroove reports the callsigns with a pass in progress.
func (t *Tracker) InGroove() []string {
	var out []string
	for cs := range t.passes {
		out = append(out, cs)
	}
	sort.Strings(out)
	return out
}

func angleDiff(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func bearingDeg(dE, dN float64) float64 {
	return math.Mod(math.Atan2(dE, dN)*180/math.Pi+360, 360)
}

// Update feeds one jet sample against the ship at that moment and returns what
// Paddles should say now ("" for nothing).
func (t *Tracker) Update(callsign, model string, s Ship, smp Sample) Call {
	prev, havePrev := t.prev[callsign]
	t.prev[callsign] = smp
	g := t.P.toGroove(s, smp)
	gs, lineup := t.P.deviations(g)

	ps := passSample{at: smp.At, g: g, gs: gs, lineup: lineup}
	var trackDeg float64
	haveTrack := false
	if havePrev {
		dt := smp.At.Sub(prev.At).Seconds()
		if dt > 0.05 && dt < 3 {
			e1, n1 := local(s, prev.Lat, prev.Lon)
			e2, n2 := local(s, smp.Lat, smp.Lon)
			ve, vn := (e2-e1)/dt, (n2-n1)/dt
			se, sn := unit(s.HeadingDeg)
			re, rn := ve-se*s.SpeedMps, vn-sn*s.SpeedMps
			ps.relSpeedKts = math.Hypot(re, rn) / mpsPerKt
			gsMps := math.Hypot(ve, vn)
			if gsMps > 20 {
				trackDeg, haveTrack = bearingDeg(ve, vn), true
			}
			if smp.HasPitch && gsMps > 20 {
				gamma := math.Atan2((smp.AltM-prev.AltM)/dt, gsMps) * 180 / math.Pi
				ps.aoa, ps.hasAoA = smp.PitchDeg-gamma, true
			}
		}
	}

	pass := t.passes[callsign]
	if pass == nil {
		axis := math.Mod(s.HeadingDeg+t.P.AngledDeckDeg+360, 360)
		if !haveTrack || g.astern > grooveStartM || g.astern < grooveMinM || g.height > grooveMaxHeight ||
			g.height < 0 || math.Abs(lineup) > grooveMaxLineup || angleDiff(trackDeg, axis) > grooveMaxHdgOff {
			return ""
		}
		pass = &Pass{Callsign: callsign, Model: model, Started: smp.At, lastSaid: make(map[Call]time.Time), minAstern: g.astern}
		t.passes[callsign] = pass
	}
	pass.samples = append(pass.samples, ps)
	if g.astern < pass.minAstern {
		pass.minAstern = g.astern
	}
	call := t.decide(pass, ps)
	if pass.outcome != "" {
		t.finish(pass)
	}
	return call
}

// Expire ends passes that stopped getting samples (jet left Tacview, crashed,
// or the feed dropped).
func (t *Tracker) Expire(now time.Time) {
	for _, p := range t.passes {
		if len(p.samples) > 0 && now.Sub(p.samples[len(p.samples)-1].at) > lostAfter {
			if p.outcome == "" {
				p.outcome = "lost"
			}
			t.finish(p)
		}
	}
}

// BallCall records a ball call heard on the net. modex is the digits the pilot
// gave ("302"); clara is "Clara" (no ball in sight). It always returns a reply,
// even with nobody in the groove: the jet may be off Tacview.
func (t *Tracker) BallCall(modex string, clara bool, now time.Time) Call {
	var match *Pass
	for cs, p := range t.passes {
		if modex != "" && strings.Contains(strings.ReplaceAll(cs, " ", ""), modex) {
			match = p
			break
		}
	}
	if match == nil && len(t.passes) == 1 {
		for _, p := range t.passes {
			match = p
		}
	}
	if match != nil {
		match.ballAt, match.clara = now, clara
		match.lastCall = now
		what := "roger ball"
		if clara {
			what = "clara"
		}
		match.calls = append(match.calls, fmt.Sprintf("%s %.1fs", what, now.Sub(match.Started).Seconds()))
	}
	if clara {
		return CallClara
	}
	return CallRogerBall
}

// smoothed is the median glideslope and lineup of the last three samples, so
// one jittery position doesn't trigger a call.
func smoothed(ss []passSample) (gs, lineup float64) {
	n := len(ss)
	if n > 3 {
		ss = ss[n-3:]
	}
	g := make([]float64, len(ss))
	l := make([]float64, len(ss))
	for i, s := range ss {
		g[i], l[i] = s.gs, s.lineup
	}
	sort.Float64s(g)
	sort.Float64s(l)
	return g[len(g)/2], l[len(l)/2]
}

func (t *Tracker) say(p *Pass, c Call, now time.Time) Call {
	if c != CallWaveOff && c != CallBolter {
		if now.Sub(p.lastCall) < gapMin || now.Sub(p.lastSaid[c]) < sameCallGap {
			return ""
		}
	}
	p.lastCall, p.lastSaid[c] = now, now
	p.calls = append(p.calls, fmt.Sprintf("%s %.1fs", strings.TrimSuffix(strings.ToLower(string(c)), "."), now.Sub(p.Started).Seconds()))
	return c
}

func (t *Tracker) decide(p *Pass, s passSample) Call {
	g := s.g
	// On deck: the hook within a metre of it. The touch point is extrapolated
	// down the jet's own descent from this sample (4.7 Hz is ~15 m a sample).
	if p.touchdown == nil && g.height < onDeckM && g.astern < 60 {
		td := s
		if n := len(p.samples); n >= 2 && g.height > 0 {
			pr := p.samples[n-2].g
			if dx, dh := pr.astern-g.astern, pr.height-g.height; dx > 1 && dh > 0.05 {
				td.g.astern -= g.height * dx / dh
			}
		}
		p.touchdown = &td
	}
	if p.touchdown != nil {
		fwd := -g.astern
		switch {
		case s.relSpeedKts > 0 && s.relSpeedKts < trapRelKts && g.height < onDeckM:
			p.outcome, p.trapStopFwd = "trap", fwd
		case !p.bolterSaid && fwd > t.wireFwd(3)+40 && s.relSpeedKts > bolterRelKts:
			p.bolterSaid = true
			if !p.waveOff {
				p.outcome = "bolter"
				return t.say(p, CallBolter, s.at)
			}
		case fwd > 350:
			p.outcome = "bolter"
		}
		return ""
	}
	if g.astern < -30 && g.height > 15 {
		if p.waveOff {
			p.outcome = "wave off"
		} else {
			p.outcome = "no touch"
		}
		return ""
	}
	if g.astern > grooveStartM+600 || g.height > grooveMaxHeight+150 || math.Abs(s.lineup) > grooveMaxLineup+8 {
		if p.waveOff {
			p.outcome = "wave off"
		} else {
			p.outcome = "left"
		}
		return ""
	}
	if p.waveOff {
		// Repeat once if he is still coming in close.
		if !p.waveOffRepeated && g.astern > 0 && g.astern < 500 && s.at.Sub(p.lastSaid[CallWaveOff]) > 3*time.Second {
			p.waveOffRepeated = true
			return t.say(p, CallWaveOff, s.at)
		}
		return ""
	}
	if len(p.samples) < 3 {
		return ""
	}
	gs, lineup := smoothed(p.samples)

	// Wave off: dangerously low or badly lined up in close.
	if (g.astern < 600 && (gs < -1.3 || math.Abs(lineup) > 5)) || (g.astern < 300 && gs < -0.9) {
		p.waveOff = true
		return t.say(p, CallWaveOff, s.at)
	}
	if g.astern < callsUntilM {
		return ""
	}
	start := callsFromM
	if !p.ballAt.IsZero() {
		start = grooveStartM
	}
	if !p.askedBall && p.ballAt.IsZero() && g.astern < callTheBallM {
		p.askedBall = true
		if c := t.say(p, CallCallTheBall, s.at); c != "" {
			return c
		}
	}
	if g.astern > start {
		return ""
	}
	switch {
	case gs < -0.6 && g.astern < 1100:
		return t.say(p, CallPower, s.at)
	case gs < -0.6:
		return t.say(p, CallLow, s.at)
	case lineup < -1.5:
		return t.say(p, CallRight, s.at)
	case lineup > 1.5:
		return t.say(p, CallLeft, s.at)
	case gs > 1.2:
		return t.say(p, CallHigh, s.at)
	case gs > 0.7:
		return t.say(p, CallLittleHigh, s.at)
	}
	if s.hasAoA && strings.Contains(strings.ToLower(p.Model), "fa-18") && g.astern < 1300 {
		switch {
		case s.aoa < hornetOnSpeed-2.0:
			return t.say(p, CallFast, s.at)
		case s.aoa > hornetOnSpeed+2.0:
			return t.say(p, CallSlow, s.at)
		}
	}
	return ""
}

// wireFwd is wire n (1-4) in metres forward of the landing point.
func (t *Tracker) wireFwd(n int) float64 {
	return t.P.WiresFromSternM[n-1] - t.P.WiresFromSternM[2]
}

// EstimateWire guesses the wire from where the jet first touched: the nearest
// wire forward of the touchdown point (the hook skips along the deck to it).
func (t *Tracker) EstimateWire(touchFwd float64) int {
	for n := 1; n <= 4; n++ {
		if touchFwd <= t.wireFwd(n)+4 {
			return n
		}
	}
	return 0
}

// segment means, the way an LSO grades a pass: X (start), IM, IC, AR.
var segments = []struct {
	name     string
	from, to float64
}{{"X", 2400, 1390}, {"IM", 1390, 740}, {"IC", 740, 370}, {"AR", 370, 0}}

func (t *Tracker) finish(p *Pass) {
	delete(t.passes, p.Callsign)
	delete(t.prev, p.Callsign)
	if t.Done == nil {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s): %s", p.Callsign, p.Model, p.outcome)
	if p.touchdown != nil {
		fwd := -p.touchdown.g.astern
		fmt.Fprintf(&b, ", touched %+.0f m from the 3 wire, %+.0f m right", fwd, p.touchdown.g.right)
		if w := t.EstimateWire(fwd); w > 0 && p.outcome == "trap" {
			fmt.Fprintf(&b, ", est. %d wire", w)
		}
		if p.outcome == "trap" {
			fmt.Fprintf(&b, ", stopped %+.0f m", p.trapStopFwd)
		}
	}
	n := len(p.samples)
	if n > 1 {
		dur := p.samples[n-1].at.Sub(p.samples[0].at).Seconds()
		maxGap := 0.0
		for i := 1; i < n; i++ {
			if d := p.samples[i].at.Sub(p.samples[i-1].at).Seconds(); d > maxGap {
				maxGap = d
			}
		}
		fmt.Fprintf(&b, "; %d samples in %.0f s (max gap %.1f s)", n, dur, maxGap)
	}
	for _, seg := range segments {
		var gs, lu, aoa float64
		var k, ka int
		for _, s := range p.samples {
			if s.g.astern <= seg.from && s.g.astern > seg.to {
				gs += s.gs
				lu += s.lineup
				k++
				if s.hasAoA {
					aoa += s.aoa
					ka++
				}
			}
		}
		if k > 0 {
			fmt.Fprintf(&b, "; %s GS %+.1f° LU %+.1f°", seg.name, gs/float64(k), lu/float64(k))
			if ka > 0 {
				fmt.Fprintf(&b, " AoA %.1f", aoa/float64(ka))
			}
		}
	}
	if len(p.calls) > 0 {
		fmt.Fprintf(&b, "; said: %s", strings.Join(p.calls, ", "))
	}
	t.Done(b.String())
}
