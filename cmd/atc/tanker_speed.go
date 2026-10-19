package main

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/paulmach/orb"
)

// tankerSpeed works out the tanker's indicated airspeed from its Tacview
// track (operator 2026-10-19: "can you read texaco speed from tacview").
// Tacview sends no airspeed for AI jets, only position, so:
//
//  1. ground speed between fixes at least 20 s apart;
//  2. the two legs of the racetrack averaged, which cancels the wind
//     (Texaco 1-1 on Day 18 Oct: 304 kt north-east, 327 kt south-west =
//     316 kt true);
//  3. true to indicated for the altitude, standard atmosphere
//     (316 kt true at 13,000 ft = about 260 indicated).
//
// With only one leg seen yet, that leg's ground speed stands in for true.
type tankerSpeed struct {
	mu      sync.Mutex
	samples []tankerFix
}

type tankerFix struct {
	at    time.Time
	pos   orb.Point
	altFt float64
}

const (
	tankerSpeedWindow = time.Hour        // two full legs of a 50 nm racetrack
	tankerFixGap      = 20 * time.Second // shortest pair to time a ground speed over
	tankerMinGroups   = 3                // ground speeds needed on a leg
)

// Add records one fix; fixes older than the window drop off.
func (s *tankerSpeed) Add(at time.Time, pos orb.Point, altFt float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.samples); n > 0 && at.Sub(s.samples[n-1].at) < tankerFixGap {
		return
	}
	s.samples = append(s.samples, tankerFix{at, pos, altFt})
	cut := 0
	for cut < len(s.samples) && at.Sub(s.samples[cut].at) > tankerSpeedWindow {
		cut++
	}
	s.samples = s.samples[cut:]
}

// KIAS is the tanker's indicated airspeed rounded to 5 kt, false until
// there are enough fixes.
func (s *tankerSpeed) KIAS() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type leg struct{ gs, trk float64 }
	var legs []leg
	for i := 1; i < len(s.samples); i++ {
		a, b := s.samples[i-1], s.samples[i]
		dt := b.at.Sub(a.at).Hours()
		if dt <= 0 || b.at.Sub(a.at) > 3*tankerFixGap {
			continue // a gap in the track
		}
		trk, nm := bearingRangeFromRef(a.pos, b.pos[0], b.pos[1])
		legs = append(legs, leg{nm / dt, trk})
	}
	if len(legs) < tankerMinGroups {
		return 0, false
	}
	// The leg it is on now, and the reciprocal; turns fall in neither.
	ref := legs[len(legs)-1].trk
	var here, back []float64
	for _, l := range legs {
		switch d := math.Abs(math.Mod(l.trk-ref+540, 360) - 180); {
		case d < 30:
			here = append(here, l.gs)
		case d > 150:
			back = append(back, l.gs)
		}
	}
	if len(here) < tankerMinGroups {
		return 0, false
	}
	tas := median(here)
	if len(back) >= tankerMinGroups {
		tas = (tas + median(back)) / 2
	}
	alt := s.samples[len(s.samples)-1].altFt
	// ISA density ratio below the tropopause; indicated = true * sqrt(sigma).
	sigma := math.Pow(1-6.8756e-6*alt, 4.2559)
	kias := tas * math.Sqrt(sigma)
	return int(math.Round(kias/5) * 5), true
}

func median(v []float64) float64 {
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	if n := len(c); n%2 == 0 {
		return (c[n/2-1] + c[n/2]) / 2
	}
	return c[len(c)/2]
}
