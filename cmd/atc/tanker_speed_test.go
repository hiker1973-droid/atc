package main

import (
	"math"
	"testing"
	"time"

	"github.com/paulmach/orb"
)

// fly moves p along a true track at a ground speed for d, a fix every 10 s.
func fly(s *tankerSpeed, at *time.Time, p *orb.Point, trk, gs float64, d time.Duration) {
	for t := time.Duration(0); t < d; t += 10 * time.Second {
		nm := gs * 10 / 3600
		p[1] += nm / 60 * math.Cos(trk*math.Pi/180)
		p[0] += nm / 60 * math.Sin(trk*math.Pi/180) / math.Cos(p[1]*math.Pi/180)
		*at = at.Add(10 * time.Second)
		s.Add(*at, *p, 13008)
	}
}

// Texaco 1-1 on Day 18 Oct (Tacview): 304 kt north-east, 327 kt south-west
// at 13,000 ft = 316 kt true, about 260 indicated.
func TestTankerSpeedBothLegs(t *testing.T) {
	var s tankerSpeed
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	p := orb.Point{42.0, 42.5}
	if _, ok := s.KIAS(); ok {
		t.Fatal("speed with no track")
	}
	fly(&s, &at, &p, 55, 304, 10*time.Minute)
	fly(&s, &at, &p, 145, 316, time.Minute) // the turn, roughly
	fly(&s, &at, &p, 235, 327, 10*time.Minute)
	if got, ok := s.KIAS(); !ok || got != 260 {
		t.Errorf("KIAS %d %v, want 260", got, ok)
	}
}

// One leg only: its ground speed stands in for true airspeed.
func TestTankerSpeedOneLeg(t *testing.T) {
	var s tankerSpeed
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	p := orb.Point{42.0, 42.5}
	fly(&s, &at, &p, 55, 304, 2*time.Minute)
	if got, ok := s.KIAS(); !ok || got != 250 {
		t.Errorf("KIAS %d %v, want 250 (304 kt ground speed at 13,000 ft)", got, ok)
	}
}
