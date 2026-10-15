package main

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Tower shift change: a field with an alternate ElevenLabs voice
// (--elevenlabs-tower-alt-voice) swaps controllers every --tower-shift, the
// way a real tower hands over between shifts. Operator 2026-10-15, for Senaki
// (Lisa and round 13 voice 2): "I would like to rotate based on time", picked
// "shift change" every 2 hours.
//
// The shift is a wall-clock slot counted from the Unix epoch, so every rig
// has the same controller at the same time and a restart keeps it. The
// handover itself waits until the tower has been quiet for towerShiftQuiet:
// a pilot working the pattern at the top of the hour keeps the controller
// they started with (the old 4-hour gender swap flipped mid-pattern, and
// turned Senaki male - 62bd959). The Departure voice is not part of it.

const towerShiftQuiet = 5 * time.Minute

type towerShift struct {
	slot  time.Duration
	quiet time.Duration

	mu     sync.Mutex
	set    bool
	alt    bool // the alternate controller is on
	lastTX time.Time
}

// onAlt reports whether this transmission, at now, is spoken by the alternate
// controller, and records it as the tower's latest transmission.
func (s *towerShift) onAlt(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	want := s.slot > 0 && (now.Unix()/int64(s.slot/time.Second))%2 == 1
	switch {
	case !s.set:
		s.alt, s.set = want, true
	case want != s.alt && now.Sub(s.lastTX) >= s.quiet:
		s.alt = want
		log.Info().Bool("alternate", want).Msg("tower shift change - the other controller takes the frequency")
	}
	s.lastTX = now
	return s.alt
}

var towerShiftState = &towerShift{quiet: towerShiftQuiet}

// towerShiftProfile is the tower's voice for this transmission: the field's
// own, or its alternate when that controller is on shift. Same delivery and
// OpenAI fallback either way.
func towerShiftProfile(towerVoice string, now time.Time) voiceProfile {
	p := towerProfile(towerVoice)
	if flagElevenTowerAltVoice == "" {
		return p
	}
	towerShiftState.slot = flagTowerShift
	if towerShiftState.onAlt(now) {
		p.eleven = elTower(flagElevenTowerAltVoice, "Tower alternate")
	}
	return p
}
