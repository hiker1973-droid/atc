package main

import (
	"testing"
	"time"
)

func TestTXGuardCoversPlaybackAndTail(t *testing.T) {
	const f = 251.775
	var cd int64
	if rxMuted(&cd, f) {
		t.Fatal("muted before any transmission")
	}
	end := beginTX(f)
	if !rxMuted(&cd, f) {
		t.Error("not muted while our transmission plays (the estimate cooldown is zero here)")
	}
	if rxMuted(&cd, 251.800) {
		t.Error("another frequency is muted")
	}
	end()
	end() // a second call must not count twice
	if !rxMuted(&cd, f) {
		t.Error("not muted in the tail after playback")
	}
	txGuardMu.Lock()
	txGuards[txGuardKey(f)].until = time.Now().Add(-time.Millisecond)
	txGuardMu.Unlock()
	if rxMuted(&cd, f) {
		t.Error("still muted after the tail")
	}
}

func TestTXGuardOverlappingTransmissions(t *testing.T) {
	const f = 133.000
	var cd int64
	a := beginTX(f)
	b := beginTX(f)
	a()
	txGuardMu.Lock()
	txGuards[txGuardKey(f)].until = time.Time{}
	txGuardMu.Unlock()
	if !rxMuted(&cd, f) {
		t.Error("one transmission still playing, yet not muted")
	}
	b()
}

func TestTXGuardHungPlaybackExpires(t *testing.T) {
	const f = 140.250
	var cd int64
	beginTX(f) // never ends
	txGuardMu.Lock()
	txGuards[txGuardKey(f)].started = time.Now().Add(-txGuardMaxPlay - time.Second)
	txGuardMu.Unlock()
	if rxMuted(&cd, f) {
		t.Error("a hung playback left the frequency deaf")
	}
}

func TestEstimateCooldownStillMutes(t *testing.T) {
	cd := time.Now().Add(time.Minute).UnixNano()
	if !rxMuted(&cd, 99.9) {
		t.Error("the estimate cooldown no longer mutes")
	}
}
