package lso

import (
	"math"
	"strings"
	"testing"
	"time"
)

// casePattern flies a Case I pattern around a ship steaming north at the
// origin (ship frame: north = along BRC, east = starboard) and feeds it to
// the tracker's history. abeamNm sets the downwind; ninetyFt the height
// through the final turn's middle. It returns the time at the ramp.
func casePattern(t *Tracker, cs string, abeamNm, ninetyFt float64) time.Time {
	s := Ship{Lat: 42, Lon: 41, HeadingDeg: 0}
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	const v, dt = 70.0, 0.25
	n, e := -4*mPerNm, 0.0
	hdg, ft := 0.0, 800.0
	emit := func() {
		lat := s.Lat + n/mPerDegLat
		lon := s.Lon + e/(111320*math.Cos(s.Lat*math.Pi/180))
		t.record(cs, s, Sample{At: at, Lat: lat, Lon: lon, AltM: ft / ftPerM}, hdg, true)
		at = at.Add(time.Duration(dt * float64(time.Second)))
	}
	step := func() {
		de, dn := unit(hdg)
		e += de * v * dt
		n += dn * v * dt
		emit()
	}
	// Initial up the wake to 1/2 nm past the bow, level at 800.
	for n < 0.5*mPerNm {
		step()
	}
	// Break: a level-ish 180 to port, its diameter the abeam distance.
	r := abeamNm * mPerNm / 2
	turn := func(deg, toFt float64) {
		rate := v / r * 180 / math.Pi // deg/s
		steps := int(deg / rate / dt)
		from := ft
		for i := 0; i < steps; i++ {
			hdg = math.Mod(hdg-rate*dt+360, 360)
			ft = from + (toFt-from)*float64(i+1)/float64(steps)
			step()
		}
	}
	turn(180, 600)
	// Downwind to 1/2 nm past the stern.
	for n > -0.5*mPerNm {
		step()
	}
	// The final turn, 189 degrees to the landing axis, through ninetyFt.
	turn(99, ninetyFt)
	turn(90, 330)
	// 16 s in the groove, then the ramp.
	for i := 0; i < int(16/dt); i++ {
		ft -= 2.4
		step()
	}
	return at.Add(-time.Duration(dt * float64(time.Second)))
}

func TestPatternByTheBook(t *testing.T) {
	tr := NewTracker(NimitzParams())
	end := casePattern(tr, "Raider 302", 1.2, 475)
	p := tr.pattern("Raider 302", end)
	if !p.HasInitial || math.Abs(p.InitialFt-800) > 5 {
		t.Errorf("initial %+v", p)
	}
	if !p.HasBreak || p.BreakFt < 700 || p.BreakFt > 800 {
		t.Errorf("break %.0f", p.BreakFt)
	}
	if !p.HasAbeam || math.Abs(p.AbeamNm-1.2) > 0.1 || math.Abs(p.AbeamFt-600) > 5 {
		t.Errorf("abeam %.2f nm %.0f ft", p.AbeamNm, p.AbeamFt)
	}
	if !p.HasNinety || math.Abs(p.NinetyFt-475) > 40 {
		t.Errorf("90 %.0f", p.NinetyFt)
	}
	// The groove starts within 10 degrees of the axis: the end of the turn
	// (~1.5 s here) counts, as real tracks wander 2-8 degrees in the groove.
	if !p.HasGroove || p.GrooveS < 15.5 || p.GrooveS > 18.5 {
		t.Errorf("groove %.1f s", p.GrooveS)
	}
	if f := p.Faults(); len(f) != 0 {
		t.Errorf("by the book, faults %v (%s)", f, p)
	}
}

func TestPatternWideAndHigh(t *testing.T) {
	tr := NewTracker(NimitzParams())
	end := casePattern(tr, "Raider 302", 1.9, 650)
	f := strings.Join(tr.pattern("Raider 302", end).Faults(), "; ")
	for _, want := range []string{"wide abeam", "high at the 90"} {
		if !strings.Contains(f, want) {
			t.Errorf("faults %q lack %q", f, want)
		}
	}
}

// A straight-in (Case III, or a long final) has no downwind: no pattern but
// the groove.
func TestPatternStraightIn(t *testing.T) {
	tr := NewTracker(NimitzParams())
	s := Ship{Lat: 42, Lon: 41, HeadingDeg: 0}
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	axis := NimitzParams().AngledDeckDeg + 360
	de, dn := unit(axis)
	for i := 0; i < 200; i++ {
		d := 6*mPerNm - float64(i)*70*0.25
		lat := s.Lat - dn*d/mPerDegLat
		lon := s.Lon - de*d/(111320*math.Cos(s.Lat*math.Pi/180))
		tr.record("Raider 302", s, Sample{At: at, Lat: lat, Lon: lon, AltM: 300}, axis, true)
		at = at.Add(250 * time.Millisecond)
	}
	p := tr.pattern("Raider 302", at)
	if p.HasAbeam || p.HasNinety || p.HasBreak || p.HasInitial || len(p.Faults()) != 0 {
		t.Errorf("straight-in: %+v", p)
	}
}
