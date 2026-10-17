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
	// HookTouchFromSternM is where the lens puts the hook: 230 ft from the
	// stern for the 3 wire, crossing the ramp at 14.1 ft (DCS Supercarrier
	// guide, LSO station). 3.5 deg over 70 m is that 14.1 ft.
	HookTouchFromSternM float64
}

// NimitzParams are the defaults for CVN-72 Abe.
func NimitzParams() Params {
	return Params{
		DeckHeightM:     20.1,
		SternM:          -164,
		WiresFromSternM: [4]float64{55, 67, 79, 92},
		AngledDeckDeg:   -9.14,
		LandingLateralM: -4, // 5 passes in the 2026-10-17 Day recording touched 0-8 m port
		GlideslopeDeg:   3.5,
		HookBelowRefM:   2.5,

		HookTouchFromSternM: 70.1,
	}
}

// landingFromRefM is the target hook touchdown point (for the 3 wire), metres
// along the ship's centreline from its reference point.
func (p Params) landingFromRefM() float64 { return p.SternM + p.HookTouchFromSternM }

// InLandingArea reports whether an aircraft is on the landing area (stern to
// past the 4 wire's run-out, on the angled deck): a foul deck for the next jet.
func (p Params) InLandingArea(s Ship, smp Sample) bool {
	g := p.toGroove(s, smp)
	fwd := -g.astern
	return fwd > -p.HookTouchFromSternM && fwd < 190 && math.Abs(g.right) < 18 && g.height < 8
}

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

// The lines follow the DCS Supercarrier LSO (Supercarrier Operations Guide,
// Case I "Inside 3/4 mile"), checked 2026-10-17.
const (
	CallRogerBall    Call = "Roger ball."
	CallClara        Call = "Paddles contact, keep it coming."
	CallCallTheBall  Call = "Call the ball."
	CallHigh         Call = "You're high."
	CallHighBring    Call = "You're high, bring it down." // high for more than 3 s
	CallLittleHigh   Call = "A little high."
	CallLowPower     Call = "You're low, power."   // far out
	CallPower        Call = "Power!"               // in close; stern (operator 2026-10-17, take B)
	CallPowerUrgent  Call = "POWER!"               // still low 2 s after a power call
	CallPowerSink    Call = "Power, power, POWER!" // far below and sinking
	CallEasy         Call = "Easy with it."        // over-correcting after a power call
	CallRight        Call = "Right for lineup."
	CallLeft         Call = "Come left."
	CallLinedUpLeft  Call = "You're lined up left." // far left, farther out
	CallLinedUpRight Call = "You're lined up right."
	CallFast         Call = "You're fast."
	CallSlow         Call = "You're slow."
	CallWaveOff      Call = "Wave off, wave off, wave off."
	CallWaveOffFoul  Call = "Wave off, wave off, wave off, foul deck."
	CallBolter       Call = "Bolter, bolter, bolter."
)

// AllCalls lists every fixed line, so the role can render them at start-up
// and speak them without waiting for TTS.
var AllCalls = []Call{CallRogerBall, CallClara, CallCallTheBall, CallHigh, CallHighBring, CallLittleHigh,
	CallLowPower, CallPower, CallPowerUrgent, CallPowerSink, CallEasy, CallRight, CallLeft, CallLinedUpLeft,
	CallLinedUpRight, CallFast, CallSlow, CallWaveOff, CallWaveOffFoul, CallBolter}

// IsWaveOff reports a wave-off call (either form).
func IsWaveOff(c Call) bool { return c == CallWaveOff || c == CallWaveOffFoul }

// Thresholds. Distances are metres astern of the landing point.
const (
	grooveStartM    = 2400.0 // ~1.3 nm: a pass starts inside this
	grooveMinM      = 120.0  // in the gate at least this far out
	grooveMaxHeight = 450.0  // and below ~1,500 ft
	grooveMaxLineup = 12.0   // and within this many degrees of centreline
	grooveMaxHdgOff = 35.0   // and pointing roughly down the landing axis
	grooveMaxGS     = 2.5    // and within this many degrees of the glideslope
	closeInM        = 300.0  // inside this, lineup is judged in metres
	waveOffRightM   = 10.0   // off the landing axis this far in close
	callsFromM      = 1500.0 // deviation calls from ~0.8 nm (earlier after a ball call)
	callsUntilM     = 70.0   // nothing but wave off over the ramp
	callTheBallM    = 1100.0
	gapMin          = 2 * time.Second
	sameCallGap     = 8 * time.Second // "a little high" every 4.5 s was nagging
	lostAfter       = 6 * time.Second
	onDeckM         = 1.0 // hook height above the deck
	trapRelKts      = 20.0
	bolterRelKts    = 60.0
	bolterPastM     = 90.0 // past the 4 wire, beyond a trap's run-out
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
	foulDeck        bool
	highSince       time.Time

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
	// foul is set by the feed when another aircraft is on the landing area.
	foul bool
	// Done receives each finished pass's summary line.
	Done func(summary string)
}

// NewTracker returns a Tracker with p.
func NewTracker(p Params) *Tracker {
	return &Tracker{P: p, passes: make(map[string]*Pass), prev: make(map[string]Sample)}
}

// SetDeckFoul tells the tracker whether the landing area is foul.
func (t *Tracker) SetDeckFoul(foul bool) { t.foul = foul }

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
	haveTrack, descending := false, false
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
				descending = smp.AltM-prev.AltM < -math.Tan(0.5*math.Pi/180)*gsMps*dt
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
		// A groove descends near the glideslope; the overhead break and
		// upwind fly-bys (level, several degrees high) are not passes.
		if !haveTrack || !descending || math.Abs(gs) > grooveMaxGS || g.astern > grooveStartM || g.astern < grooveMinM || g.height > grooveMaxHeight ||
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
	if !IsWaveOff(c) && c != CallBolter {
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
		// The 2026-10-17 Day recording: a trap still rolls at touchdown speed
		// 30 m past the 4 wire and stops ~100 m past the aim point; a bolter
		// is still at 120+ kt there.
		case !p.bolterSaid && fwd > t.wireFwd(4)+bolterPastM && s.relSpeedKts > bolterRelKts:
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
	if g.astern > grooveStartM+600 || g.height > grooveMaxHeight+150 ||
		(g.astern > closeInM && math.Abs(s.lineup) > grooveMaxLineup+8) {
		if p.waveOff {
			p.outcome = "wave off"
		} else {
			p.outcome = "left"
		}
		return ""
	}
	if p.waveOff {
		// Repeat once if he is still coming in close.
		c := CallWaveOff
		if p.foulDeck {
			c = CallWaveOffFoul
		}
		if !p.waveOffRepeated && g.astern > 0 && g.astern < 500 && s.at.Sub(p.lastSaid[c]) > 3*time.Second {
			p.waveOffRepeated = true
			return t.say(p, c, s.at)
		}
		return ""
	}
	if len(p.samples) < 3 {
		return ""
	}
	gs, lineup := smoothed(p.samples)
	rate := gsRate(p.samples)

	// Foul deck: another aircraft on the landing area as he comes in close.
	if t.foul && g.astern < 550 {
		p.waveOff, p.foulDeck = true, true
		return t.say(p, CallWaveOffFoul, s.at)
	}
	// Wave off: dangerously low or badly lined up in close.
	// Lineup in degrees grows as range closes, so in close it is metres.
	badLineup := math.Abs(lineup) > 5
	if g.astern < closeInM {
		badLineup = math.Abs(g.right) > waveOffRightM
	}
	if (g.astern < 600 && (gs < -1.3 || badLineup)) || (g.astern < closeInM && gs < -0.9) {
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
	if gs > 0.7 {
		if p.highSince.IsZero() {
			p.highSince = s.at
		}
	} else {
		p.highSince = time.Time{}
	}
	if g.astern > start {
		return ""
	}
	lastPower := latest(p.lastSaid, CallPower, CallPowerUrgent, CallPowerSink, CallLowPower)
	// Easy with it: coming up through the glideslope fast after a power call.
	if !lastPower.IsZero() && s.at.Sub(lastPower) < 4*time.Second && rate > 0.6 && gs > -0.3 {
		return t.say(p, CallEasy, s.at)
	}
	switch {
	case gs < -0.6 && g.astern < 1100:
		switch {
		case gs < -1.0 && rate < -0.3:
			return t.say(p, CallPowerSink, s.at)
		case !lastPower.IsZero() && s.at.Sub(lastPower) < 3500*time.Millisecond:
			return t.say(p, CallPowerUrgent, s.at)
		}
		return t.say(p, CallPower, s.at)
	case gs < -0.6:
		return t.say(p, CallLowPower, s.at)
	case lineup < -3.0 && g.astern > 700:
		return t.say(p, CallLinedUpLeft, s.at)
	case lineup > 3.0 && g.astern > 700:
		return t.say(p, CallLinedUpRight, s.at)
	case lineup < -1.5:
		return t.say(p, CallRight, s.at)
	case lineup > 1.5:
		return t.say(p, CallLeft, s.at)
	case gs > 1.2:
		// Still high 3 s after being told: bring it down.
		if !p.lastSaid[CallHigh].IsZero() && s.at.Sub(p.highSince) > 3*time.Second {
			return t.say(p, CallHighBring, s.at)
		}
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

// gsRate is how fast the jet is moving through the glideslope, degrees per
// second over about the last second (+ = going up).
func gsRate(ss []passSample) float64 {
	n := len(ss)
	if n < 2 {
		return 0
	}
	last := ss[n-1]
	for i := n - 2; i >= 0; i-- {
		if dt := last.at.Sub(ss[i].at).Seconds(); dt >= 0.9 {
			return (last.gs - ss[i].gs) / dt
		}
	}
	return 0
}

func latest(m map[Call]time.Time, cs ...Call) time.Time {
	var t time.Time
	for _, c := range cs {
		if m[c].After(t) {
			t = m[c]
		}
	}
	return t
}

// wireFwd is wire n (1-4) in metres forward of the hook touchdown point.
func (t *Tracker) wireFwd(n int) float64 {
	return t.P.WiresFromSternM[n-1] - t.P.HookTouchFromSternM
}

// EstimateWire guesses the wire from where the hook first touched: the first
// wire at or forward of the touchdown point (the hook skips along to it).
func (t *Tracker) EstimateWire(touchFwd float64) int {
	for n := 1; n <= 4; n++ {
		if t.wireFwd(n) >= touchFwd-2 {
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
	if g := t.Grade(p); g != "" {
		fmt.Fprintf(&b, ", grade %s", g)
	}
	if p.touchdown != nil {
		fwd := -p.touchdown.g.astern
		fmt.Fprintf(&b, ", hook touched %+.0f m from the aim point, %+.0f m right", fwd, p.touchdown.g.right)
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

// segMeans is one segment's mean glideslope, lineup and AoA error.
type segMeans struct {
	name          string
	gs, lu, aoa   float64
	hasGS, hasAoA bool
}

func (t *Tracker) segMeans(p *Pass) []segMeans {
	var out []segMeans
	for _, seg := range segments {
		m := segMeans{name: seg.name}
		var n, na int
		for _, s := range p.samples {
			if s.g.astern <= seg.from && s.g.astern > seg.to {
				m.gs += s.gs
				m.lu += s.lineup
				n++
				if s.hasAoA {
					m.aoa += s.aoa
					na++
				}
			}
		}
		if n > 0 {
			m.gs /= float64(n)
			m.lu /= float64(n)
			m.hasGS = true
		}
		if na > 0 {
			m.aoa /= float64(na)
			m.hasAoA = true
		}
		out = append(out, m)
	}
	return out
}

// Grade is the pass in the DCS LSO's notation (Supercarrier guide, Landing):
// grade, errors with where they were (X, IM, IC, AR; "(H)" = a little high,
// "[BC]" = no ball call) and the wire. "" for a pass that never got to a
// trap, bolter or wave-off.
func (t *Tracker) Grade(p *Pass) string {
	var errs []string
	major, majorInClose := false, false
	hornet := strings.Contains(strings.ToLower(p.Model), "fa-18")
	for _, m := range t.segMeans(p) {
		if !m.hasGS {
			continue
		}
		add := func(code string, big bool) {
			if !big {
				errs = append(errs, "("+code+")"+m.name)
				return
			}
			errs = append(errs, code+m.name)
			major = true
			if m.name == "IC" || m.name == "AR" {
				majorInClose = true
			}
		}
		switch {
		case m.gs > 1.2:
			add("H", true)
		case m.gs > 0.6:
			add("H", false)
		case m.gs < -0.6:
			add("LO", true)
		case m.gs < -0.3:
			add("LO", false)
		}
		switch {
		case m.lu < -1.5:
			add("LUL", true)
		case m.lu < -0.75:
			add("LUL", false)
		case m.lu > 1.5:
			add("LUR", true)
		case m.lu > 0.75:
			add("LUR", false)
		}
		if hornet && m.hasAoA {
			switch {
			case m.aoa < hornetOnSpeed-1.5:
				add("F", true)
			case m.aoa < hornetOnSpeed-0.75:
				add("F", false)
			case m.aoa > hornetOnSpeed+1.5:
				add("SLO", true)
			case m.aoa > hornetOnSpeed+0.75:
				add("SLO", false)
			}
		}
	}
	if p.ballAt.IsZero() {
		errs = append([]string{"[BC]"}, errs...)
	}
	with := func(g, extra string) string {
		parts := errs
		if extra != "" {
			parts = append(append([]string{}, errs...), extra)
		}
		if len(parts) == 0 {
			return g
		}
		return g + ": " + strings.Join(parts, " ")
	}
	// A waved-off pass is graded WO even if he landed anyway.
	switch outcome := p.outcome; {
	case p.waveOff || outcome == "wave off":
		if p.foulDeck {
			return "WO (foul deck)"
		}
		return with("WO", "")
	case outcome == "bolter":
		return with("B", "")
	case outcome == "trap":
	default:
		return ""
	}
	wire := 0
	if p.touchdown != nil {
		wire = t.EstimateWire(-p.touchdown.g.astern)
	}
	g := "OK"
	switch {
	case majorInClose || wire == 1:
		g = "---"
	case major:
		g = "(OK)"
	case len(errs) == 0 && wire == 3:
		g = "_OK_"
	}
	w := ""
	if wire > 0 {
		w = fmt.Sprintf("%d wire", wire)
	}
	return with(g, w)
}
