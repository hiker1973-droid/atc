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

// A pilot the net has already understood is heard through our own
// transmission; an origin it has never heard (ExternalAudio, new every time)
// is not.
func TestKnownOrigins(t *testing.T) {
	k := newKnownOrigins()
	if k.known("pilot302") {
		t.Fatal("known before heard")
	}
	k.add("pilot302")
	if !k.known("pilot302") || k.known("externalaudio") {
		t.Errorf("known(pilot302) = %v, known(externalaudio) = %v", k.known("pilot302"), k.known("externalaudio"))
	}
	k.seen["pilot302"] = time.Now().Add(-knownOriginFor - time.Minute)
	if k.known("pilot302") {
		t.Error("still known after an hour")
	}
}
