package main

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Receive guard sized from real playback. Each role's txCooldown is set from
// estimateTTSDuration when a reply is spoken, so a transmission that plays
// longer than the estimate is heard back once the cooldown runs out, and the
// proactive transmissions (Command's recovery handoff, Hitman's BDA and
// aborts) set no cooldown at all - own_echo.go was the only thing catching
// those. transmitExternalAudioFile returns when ExternalAudio has finished
// playing (eaMs tracks text length at ~14 chars/s on DCS3 2026-10-15), so it
// marks its frequency busy for the whole playback and for txGuardTail after,
// and every receiver on that frequency in this process ignores what it hears
// meanwhile. The estimate stays as the floor.

// txGuardTail covers SRS relaying the last of our packets back after
// ExternalAudio exits.
const txGuardTail = 2 * time.Second

// txGuardMaxPlay bounds a playback that never reports finishing, so a hung
// ExternalAudio can't leave a frequency deaf.
const txGuardMaxPlay = 90 * time.Second

type txGuardEntry struct {
	playing int       // transmissions on the air now
	until   time.Time // end of the tail after the last one finished
	started time.Time // start of the oldest one still playing
}

var (
	txGuardMu sync.Mutex
	txGuards  = map[int64]*txGuardEntry{}
)

func txGuardKey(freqMHz float64) int64 { return int64(math.Round(freqMHz * 1000)) } // kHz

// beginTX marks freqMHz as transmitting; call the returned func when playback ends.
func beginTX(freqMHz float64) (end func()) {
	k := txGuardKey(freqMHz)
	txGuardMu.Lock()
	g := txGuards[k]
	if g == nil {
		g = &txGuardEntry{}
		txGuards[k] = g
	}
	if g.playing == 0 {
		g.started = time.Now()
	}
	g.playing++
	txGuardMu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			txGuardMu.Lock()
			g.playing--
			if t := time.Now().Add(txGuardTail); t.After(g.until) {
				g.until = t
			}
			txGuardMu.Unlock()
		})
	}
}

// ownTXOnAir reports whether we are transmitting on freqMHz, or did within txGuardTail.
func ownTXOnAir(freqMHz float64) bool {
	txGuardMu.Lock()
	defer txGuardMu.Unlock()
	g := txGuards[txGuardKey(freqMHz)]
	if g == nil {
		return false
	}
	now := time.Now()
	if g.playing > 0 && now.Sub(g.started) < txGuardMaxPlay {
		return true
	}
	return now.Before(g.until)
}

// rxMuted is the receive check: the estimate-sized cooldown, or our own
// transmission still playing on this frequency.
func rxMuted(cooldown *int64, freqMHz float64) bool {
	if until := atomic.LoadInt64(cooldown); until > 0 && time.Now().UnixNano() < until {
		return true
	}
	return ownTXOnAir(freqMHz)
}

// knownOrigins remembers the SRS origins a net has understood (pilots), for an
// hour since last heard, so their calls get through while we transmit.
type knownOrigins struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

const knownOriginFor = time.Hour

func newKnownOrigins() *knownOrigins { return &knownOrigins{seen: map[string]time.Time{}} }

func (k *knownOrigins) add(origin string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.seen[origin] = time.Now()
	for o, t := range k.seen {
		if time.Since(t) > knownOriginFor {
			delete(k.seen, o)
		}
	}
}

func (k *knownOrigins) known(origin string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	t, ok := k.seen[origin]
	return ok && time.Since(t) <= knownOriginFor
}
