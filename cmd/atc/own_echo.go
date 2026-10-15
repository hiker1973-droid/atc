package main

import (
	"strings"
	"sync"
	"time"
	"unicode"
)

// Our own transmissions come back to us. Every TX goes out through a fresh
// ExternalAudio client with a new SRS GUID, and SRS relays it to every listener
// on the frequency, including the role that sent it. txCooldown covers short
// replies, but a long one outlasts it and is transcribed as if a pilot said it.
//
// DCS3 2026-10-15 14:25:34: Hitman's "copy off. Egress back to Cobra, angels
// 14. Report IP inbound for re-attack." came back 15 s later, matched the
// "off" trigger again, and Hitman answered itself every ~16 s for minutes.
// Whisper wrote the address as "Hitman 1-1", "Hitman 11", "Hitman 111" and
// "Hitland 11", so no guard on the address wording could hold.
//
// So compare against what we actually said: a call that repeats most of a
// line we transmitted on the same frequency moments ago is our own echo.

const (
	ownEchoWindow   = 45 * time.Second
	ownEchoMinShare = 0.6 // share of the heard words found in our TX
	ownEchoMinWords = 4   // shorter calls ("Raider 302, copy") are never echoes
	// ownEchoMinLength: an echo is about as long as what we said; a pilot's
	// readback of the brief repeats its words but is much shorter.
	ownEchoMinLength = 0.7
)

type ownTX struct {
	freq  float64
	words map[string]bool
	n     int // words in the line, repeats counted
	at    time.Time
}

var (
	ownTXMu  sync.Mutex
	ownTXLog []ownTX
)

func echoWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// noteOwnTX records a line about to be transmitted on freq.
func noteOwnTX(freq float64, text string) {
	all := echoWords(text)
	set := make(map[string]bool)
	for _, w := range all {
		set[w] = true
	}
	now := time.Now()
	ownTXMu.Lock()
	defer ownTXMu.Unlock()
	keep := ownTXLog[:0]
	for _, t := range ownTXLog {
		if now.Sub(t.at) < ownEchoWindow {
			keep = append(keep, t)
		}
	}
	ownTXLog = append(keep, ownTX{freq: freq, words: set, n: len(all), at: now})
}

// isOwnEcho reports whether heard on freq repeats a line we sent there within
// ownEchoWindow.
func isOwnEcho(freq float64, heard string) bool {
	words := echoWords(heard)
	if len(words) < ownEchoMinWords {
		return false
	}
	now := time.Now()
	ownTXMu.Lock()
	defer ownTXMu.Unlock()
	for _, t := range ownTXLog {
		if t.freq != freq || now.Sub(t.at) >= ownEchoWindow ||
			float64(len(words)) < ownEchoMinLength*float64(t.n) {
			continue
		}
		hit := 0
		for _, w := range words {
			if t.words[w] {
				hit++
			}
		}
		if float64(hit)/float64(len(words)) >= ownEchoMinShare {
			return true
		}
	}
	return false
}

// joinCallsignDigits folds a modex Whisper wrote digit by digit back into one
// number: "Raider 3-0-2" and "Raider 3 0 2" become "Raider 302". Hitman keys a
// flight's tasking by callsign, so on 2026-10-15 a readback heard as "Raider
// 3-0-2" was answered "no 9-line passed" five times while the brief sat under
// "Raider 302".
func joinCallsignDigits(cs string) string {
	f := strings.Fields(strings.ReplaceAll(cs, "-", " "))
	out := make([]string, 0, len(f))
	for _, w := range f {
		if n := len(out); n > 0 && isAllDigits(w) && isAllDigits(out[n-1]) {
			out[n-1] += w
			continue
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
