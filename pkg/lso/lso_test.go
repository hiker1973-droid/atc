package lso

import (
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
	ballDone := false
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
			height = math.Tan(gs*math.Pi/180) * astern
			right = math.Tan(f.luErr(astern)*math.Pi/180) * astern
		} else {
			height = 0
		}
		lat, lon, alt := world(p, ship, astern, right, height)
		if f.ball && !ballDone && astern < 1400 {
			ballDone = true
			calls = append(calls, tr.BallCall("302", false, now))
		}
		if f.foulFrom > 0 && astern < f.foulFrom {
			tr.SetDeckFoul(true)
		}
		c := tr.Update("Raider 302", "FA-18C_hornet", ship, Sample{At: now, Lat: lat, Lon: lon, AltM: alt})
		if c != "" {
			calls = append(calls, c)
		}
		if summary != "" {
			return calls, summary
		}
		// Speed relative to the ship along the axis.
		if astern <= 0 && f.finish == "trap" {
			rel = math.Max(rel-25*dt*3, 0)
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
	if !has(calls, CallWaveOffFoul) {
		t.Errorf("calls = %v, want %q", calls, CallWaveOffFoul)
	}
	if !strings.Contains(sum, "WO (foul deck)") {
		t.Errorf("summary = %q, want WO (foul deck)", sum)
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
