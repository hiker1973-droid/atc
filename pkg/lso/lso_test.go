package lso

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// flight flies a synthetic approach: the ship steams at 15 m/s on heading
// 100, and the jet closes down the landing axis at 70 m/s relative, its
// glideslope and lineup error (degrees) given as functions of metres astern.
type flight struct {
	gsErr, luErr func(astern float64) float64
	// after the landing point: "trap" stops, "bolter" carries on.
	finish string
	ball   bool
	// foulFrom, when set, fouls the deck once the jet is this close (m astern).
	foulFrom float64
	// redDeck: the jet ahead called "red deck" before this one started;
	// greenAt, when set, is where (m astern) he calls "green deck".
	redDeck bool
	greenAt float64
}

func world(p Params, s Ship, astern, right, height float64) (lat, lon, altM float64) {
	le, ln := p.landingPoint(s)
	ae, an := unit(s.HeadingDeg + p.AngledDeckDeg)
	be, bn := unit(s.HeadingDeg + p.AngledDeckDeg + 90)
	e := le - ae*astern + be*right
	n := ln - an*astern + bn*right
	return s.Lat + n/mPerDegLat, s.Lon + e/(111320.0*math.Cos(s.Lat*math.Pi/180)), height + p.DeckHeightM + p.HookBelowRefM
}

func (f flight) run(t *testing.T) (calls []Call, summary string) {
	t.Helper()
	p := NimitzParams()
	p.TouchdownCalM = 0 // corrects real Tacview tracks; these are exact
	tr := NewTracker(p)
	tr.Done = func(s string) { summary = s }
	t0 := time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)
	ship0 := Ship{Lat: 42.0, Lon: 41.0, HeadingDeg: 100, SpeedMps: 15}
	astern := 2300.0
	rel := 70.0
	dt := 0.21
	ballDone, greenDone := false, false
	if f.redDeck {
		tr.SetRedDeck(true, t0.Add(-20*time.Second))
	}
	for i := 0; i < 400; i++ {
		now := t0.Add(time.Duration(float64(i) * dt * float64(time.Second)))
		// The ship moves.
		se, sn := unit(ship0.HeadingDeg)
		d := ship0.SpeedMps * float64(i) * dt
		ship := ship0
		ship.Lat += sn * d / mPerDegLat
		ship.Lon += se * d / (111320.0 * math.Cos(ship0.Lat*math.Pi/180))

		var right, height float64
		if astern > 0 {
			gs := p.GlideslopeDeg + f.gsErr(astern)
			// A jet never sinks below the deck (real tracks sit at deckLevelM).
			height = math.Max(math.Tan(gs*math.Pi/180)*astern, deckLevelM)
			right = math.Tan(f.luErr(astern)*math.Pi/180) * astern
		} else {
			height = deckLevelM
		}
		lat, lon, alt := world(p, ship, astern, right, height)
		if f.ball && !ballDone && astern < 1400 {
			ballDone = true
			calls = append(calls, tr.BallCall("302", false, now))
		}
		if f.foulFrom > 0 && astern < f.foulFrom {
			tr.SetDeckFoul(true)
		}
		if f.greenAt > 0 && !greenDone && astern < f.greenAt {
			greenDone = true
			tr.SetRedDeck(false, now)
		}
		c := tr.Update("Raider 302", "FA-18C_hornet", ship, Sample{At: now, Lat: lat, Lon: lon, AltM: alt})
		if c != "" {
			calls = append(calls, c)
		}
		if summary != "" {
			return calls, summary
		}
		// Speed relative to the ship along the axis. A trap catches the 3
		// wire and runs out trapRunoutM past it.
		if wire3 := -(NimitzParams().WiresFromSternM[2] - NimitzParams().HookTouchFromSternM); astern <= wire3 && f.finish == "trap" {
			rel = math.Max(rel-70*70/(2*trapRunoutM)*dt, 0)
		}
		astern -= rel * dt
		if astern < -400 {
			break
		}
	}
	tr.Expire(t0.Add(time.Hour))
	return calls, summary
}

func zero(float64) float64 { return 0 }

func has(calls []Call, c Call) bool {
	for _, x := range calls {
		if x == c {
			return true
		}
	}
	return false
}

func TestOnGlideslopeTrap(t *testing.T) {
	calls, sum := flight{gsErr: zero, luErr: zero, finish: "trap", ball: true}.run(t)
	for _, c := range calls {
		if c != CallRogerBall {
			t.Errorf("unexpected call %q on a perfect pass (all %v)", c, calls)
		}
	}
	if !strings.Contains(sum, ": trap") || !strings.Contains(sum, "est. 3 wire") || !strings.Contains(sum, "grade _OK_: 3 wire") {
		t.Errorf("summary = %q, want a 3-wire trap", sum)
	}
}

func TestNoBallCallIsPrompted(t *testing.T) {
	calls, _ := flight{gsErr: zero, luErr: zero, finish: "trap"}.run(t)
	if !has(calls, CallCallTheBall) {
		t.Errorf("calls = %v, want %q", calls, CallCallTheBall)
	}
}

func TestHighStart(t *testing.T) {
	calls, _ := flight{gsErr: func(a float64) float64 {
		if a > 900 {
			return 1.6
		}
		return 0
	}, luErr: zero, finish: "trap", ball: true}.run(t)
	if !has(calls, CallHigh) {
		t.Errorf("calls = %v, want %q", calls, CallHigh)
	}
}

func TestLowInCloseIsWavedOff(t *testing.T) {
	calls, sum := flight{gsErr: func(a float64) float64 {
		switch {
		case a > 1000:
			return 0
		case a > 500:
			return -0.8
		}
		return -1.6
	}, luErr: zero, finish: "trap", ball: true}.run(t)
	if !has(calls, CallPower) || !has(calls, CallWaveOff) {
		t.Errorf("calls = %v, want power then wave off", calls)
	}
	if !strings.Contains(sum, "wave off") && !strings.Contains(sum, "trap") {
		t.Logf("summary %q", sum)
	}
}

func TestLinedUpLeft(t *testing.T) {
	calls, _ := flight{gsErr: zero, luErr: func(a float64) float64 {
		if a > 600 {
			return -2.5
		}
		return 0
	}, finish: "trap", ball: true}.run(t)
	if !has(calls, CallRight) {
		t.Errorf("calls = %v, want %q", calls, CallRight)
	}
}

func TestBolter(t *testing.T) {
	calls, sum := flight{gsErr: zero, luErr: zero, finish: "bolter", ball: true}.run(t)
	if !has(calls, CallBolter) {
		t.Errorf("calls = %v, want %q", calls, CallBolter)
	}
	if !strings.Contains(sum, ": bolter") {
		t.Errorf("summary = %q, want bolter", sum)
	}
}

// A jet that isn't on the landing axis (abeam, downwind) never starts a pass.
func TestDownwindIsNotAGroove(t *testing.T) {
	p := NimitzParams()
	tr := NewTracker(p)
	ship := Ship{Lat: 42, Lon: 41, HeadingDeg: 100, SpeedMps: 15}
	t0 := time.Now()
	for i := 0; i < 50; i++ {
		// Abeam to port at 1.2 km, flying the reciprocal.
		lat, lon, alt := world(p, ship, -float64(i)*14, -1200, 180)
		if c := tr.Update("Raider 302", "FA-18C_hornet", ship, Sample{At: t0.Add(time.Duration(i) * 200 * time.Millisecond), Lat: lat, Lon: lon, AltM: alt}); c != "" {
			t.Fatalf("call %q downwind", c)
		}
	}
	if len(tr.InGroove()) != 0 {
		t.Errorf("in groove: %v", tr.InGroove())
	}
}

// Running in for the break, level at 800 ft straight up the landing axis, is
// not a pass (the 2026-10-17 Day recording had one before every recovery,
// each "waved off").
func TestOverheadIsNotAGroove(t *testing.T) {
	p := NimitzParams()
	tr := NewTracker(p)
	ship := Ship{Lat: 42, Lon: 41, HeadingDeg: 100, SpeedMps: 15}
	t0 := time.Now()
	for i := 0; i < 160; i++ {
		lat, lon, alt := world(p, ship, 2300-float64(i)*20, 0, 245)
		if c := tr.Update("Raider 302", "FA-18C_hornet", ship, Sample{At: t0.Add(time.Duration(i) * 200 * time.Millisecond), Lat: lat, Lon: lon, AltM: alt}); c != "" {
			t.Fatalf("call %q on the run-in", c)
		}
	}
	if len(tr.InGroove()) != 0 {
		t.Errorf("in groove: %v", tr.InGroove())
	}
}

// Low and flat over the ramp, then down on the 3 wire: not a 1 wire (Raider
// 302, Day 2026-10-18 14:56: DCS "C ... LOAR ... WIRE# 3", Paddles said 1 wire
// because the hook was within a metre of deck height at the ramp).
func TestLowFlatOverRampStillThreeWire(t *testing.T) {
	_, sum := flight{gsErr: func(a float64) float64 {
		// About a metre above deck height from 90 m out to the aim point.
		if a < 90 {
			return math.Atan2(0.9+0.01*a, a)*180/math.Pi - NimitzParams().GlideslopeDeg
		}
		return 0
	}, luErr: zero, finish: "trap", ball: true}.run(t)
	var fwd float64
	if _, err := fmt.Sscanf(sum[strings.Index(sum, "hook touched ")+len("hook touched "):], "%f", &fwd); err != nil ||
		!strings.Contains(sum, ": trap") || fwd < -15 {
		t.Errorf("summary = %q, want a trap touching down at the aim point, not over the ramp", sum)
	}
}

// The wire is where the jet stopped, less the run-out: seven traps with DCS's
// wire (2026-10-17/18) stopped 90 - 121 m past the aim point.
func TestWireFromStop(t *testing.T) {
	tr := NewTracker(NimitzParams())
	for _, c := range []struct {
		stop float64
		want int
	}{{90.3, 1}, {90.0, 1}, {99.4, 2}, {111.1, 3}, {115.1, 3}, {116.8, 3}, {121.0, 4}} {
		if got := tr.WireFromStop(c.stop); got != c.want {
			t.Errorf("WireFromStop(%.1f) = %d, want %d", c.stop, got, c.want)
		}
	}
}

// Far below the glide path at the ramp is a cut, even on a trap (302, Day
// 2026-10-18 14:56: DCS "C ... LOAR"); just below it is LOAR.
func TestRampCut(t *testing.T) {
	low := func(m float64) func(float64) float64 {
		return func(a float64) float64 {
			if a < 300 && a > 40 {
				gs := NimitzParams().GlideslopeDeg
				return math.Atan2(math.Tan(gs*math.Pi/180)*a+m, a)*180/math.Pi - gs
			}
			return 0
		}
	}
	_, sum := flight{gsErr: low(-2.5), luErr: zero, finish: "trap", ball: true}.run(t)
	if !strings.Contains(sum, "grade C") || !strings.Contains(sum, "LOAR") {
		t.Errorf("2.5 m low at the ramp: summary = %q, want grade C with LOAR", sum)
	}
	_, sum = flight{gsErr: low(-0.8), luErr: zero, finish: "trap", ball: true}.run(t)
	if strings.Contains(sum, "grade C") || !strings.Contains(sum, "LOAR") {
		t.Errorf("0.8 m low at the ramp: summary = %q, want LOAR, not a cut", sum)
	}
}

// Settling a metre low over the ramp is a landing, not a wave-off (three good
// passes in the 2026-10-17 recordings were waved off 35-50 m from the aim point).
func TestLowOverRampIsNotWavedOff(t *testing.T) {
	calls, sum := flight{gsErr: func(a float64) float64 {
		if a < 60 {
			return -1.2
		}
		return 0
	}, luErr: zero, finish: "trap", ball: true}.run(t)
	if has(calls, CallWaveOff) || !strings.Contains(sum, ": trap") {
		t.Errorf("calls = %v, summary %q: want a trap, no wave off", calls, sum)
	}
}

func TestFoulDeckWaveOff(t *testing.T) {
	calls, sum := flight{gsErr: zero, luErr: zero, finish: "trap", ball: true, foulFrom: 900}.run(t)
	want := Call("302, foul deck, wave off, wave off, proceed to bolter pattern.")
	if !has(calls, want) || !IsWaveOff(want) {
		t.Errorf("calls = %v, want %q", calls, want)
	}
	if !strings.Contains(sum, "WO (foul deck)") {
		t.Errorf("summary = %q, want WO (foul deck)", sum)
	}
}

// "Red deck" from the jet that just landed, no "green deck" by a quarter
// mile: the next jet is waved off (operator 2026-10-18). Green deck in time,
// or a red deck long forgotten, and he lands.
func TestRedDeckWaveOff(t *testing.T) {
	want := FoulDeckCall("Raider 302")
	calls, sum := flight{gsErr: zero, luErr: zero, finish: "trap", ball: true, redDeck: true}.run(t)
	if !has(calls, want) || !strings.Contains(sum, "WO (foul deck, no green deck)") {
		t.Errorf("no green deck: calls = %v, summary %q; want %q", calls, sum, want)
	}
	calls, sum = flight{gsErr: zero, luErr: zero, finish: "trap", ball: true, redDeck: true, greenAt: 700}.run(t)
	if has(calls, want) || !strings.Contains(sum, ": trap") {
		t.Errorf("green deck at 700 m: calls = %v, summary %q; want a trap", calls, sum)
	}
	if c := FoulDeckCall("Raider Formation-1"); c != "Raider Formation-1"+foulDeckTail {
		t.Errorf("FoulDeckCall(no modex) = %q", c)
	}
	tr := NewTracker(NimitzParams())
	now := time.Now()
	tr.SetRedDeck(true, now.Add(-redDeckFor))
	if tr.RedDeck(now) {
		t.Error("red deck still set after redDeckFor")
	}
}

// High all the way down: "You're high", then "bring it down" while it lasts.
func TestStaysHigh(t *testing.T) {
	calls, sum := flight{gsErr: func(a float64) float64 {
		if a > 300 {
			return 1.5
		}
		return 0
	}, luErr: zero, finish: "trap", ball: true}.run(t)
	if !has(calls, CallHigh) || !has(calls, CallHighBring) {
		t.Errorf("calls = %v, want high then bring it down", calls)
	}
	if !strings.Contains(sum, "HX") || !strings.Contains(sum, "HIM") {
		t.Errorf("summary = %q, want H in X and IM", sum)
	}
}

// Low and settling in close: power, then the urgent power.
func TestPowerEscalates(t *testing.T) {
	calls, _ := flight{gsErr: func(a float64) float64 {
		if a > 1050 || a < 450 {
			return 0
		}
		return -0.8
	}, luErr: zero, finish: "trap", ball: true}.run(t)
	if !has(calls, CallPower) || !has(calls, CallPowerUrgent) {
		t.Errorf("calls = %v, want power then POWER", calls)
	}
}
