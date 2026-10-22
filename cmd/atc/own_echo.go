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
//
// Only the message is compared, not the "Raider 302, Red-Command," address in
// front of it: a pilot's call names the same callsign and station as our reply
// to it. Day 2026-10-22 17:19: Red Command answered "Raider 302, Red-Command,
// loud and clear." and then dropped four of Raider 302's repeated "Red Command,
// Raider 302, comm check." as its own echo (4 of 6 words shared).

const (
	ownEchoWindow   = 45 * time.Second
	ownEchoMinShare = 0.6 // share of the heard words found in our TX
	ownEchoMinWords = 4   // shorter messages ("Raider 302, copy") are never echoes
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

// echoDigits are the spoken digits our TTS lines use.
var echoDigits = map[string]string{
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4",
	"five": "5", "six": "6", "seven": "7", "eight": "8", "nine": "9", "niner": "9",
}

// echoWords splits a line into lowercased words, with every number written as
// one word however it was said: we speak "three six zero point one zero zero"
// and Whisper writes "360.100". Without this a long frequency counted eight
// words in our line and two in the echo (Foothold 2026-10-18 11:47 and 12:50,
// Command's "contact Incirlik Tower on 360.100" handoff logged as a pilot's
// intent miss).
func echoWords(s string) []string {
	type tok struct {
		w       string
		sep     string // the characters between this word and the last
		spelled bool
	}
	var toks []tok
	sep, cur := "", ""
	flush := func() {
		if cur != "" {
			toks = append(toks, tok{w: cur, sep: sep})
			sep, cur = "", ""
		}
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur += string(r)
			continue
		}
		flush()
		sep += string(r)
	}
	flush()
	out := make([]string, 0, len(toks))
	num, spelled := false, false // the last word in out is a number; it was spoken
	for i, t := range toks {
		w := t.w
		if d, ok := echoDigits[w]; ok {
			w, t.spelled = d, true
		}
		joins := strings.TrimSpace(t.sep) == "-" || strings.TrimSpace(t.sep) == "." || (spelled && t.spelled)
		switch {
		case isAllDigits(w) && num && joins:
			out[len(out)-1] += w
			spelled = t.spelled
		case isAllDigits(w):
			out = append(out, w)
			num, spelled = true, t.spelled
		case w == "point" && num && spelled && i+1 < len(toks) && echoDigits[toks[i+1].w] != "":
			// "point" inside a spoken number: dropped, as Whisper's "." is.
		default:
			out = append(out, w)
			num, spelled = false, false
		}
	}
	return out
}

// echoAddressWords is the most words one part of a call's address has
// ("Raider 302", "vSFG-7-Command", "BSFG7 command").
const echoAddressWords = 3

// echoMessage is echoWords of a call without its address: the first two
// comma- or full-stop-separated parts, each of at most echoAddressWords words
// ("Raider 302, Hitland 11. Copy off ..." -> "copy off ..."). A call that does
// not start like that is kept whole.
func echoMessage(s string) []string {
	rest := s
	for part := 0; part < 2; part++ {
		cut := -1
		for i := 0; i < len(rest); i++ {
			if c := rest[i]; (c == ',' || c == '.') && (i+1 == len(rest) || rest[i+1] == ' ') {
				cut = i
				break
			}
		}
		if cut < 0 {
			break
		}
		if n := len(echoWords(rest[:cut])); n == 0 || n > echoAddressWords {
			break
		}
		rest = rest[cut+1:]
	}
	return echoWords(rest)
}

// noteOwnTX records a line about to be transmitted on freq.
func noteOwnTX(freq float64, text string) {
	all := echoMessage(text)
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
	words := echoMessage(heard)
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

// undoubleCallsign repairs a modex Whisper heard with a digit doubled, using
// the Tacview picture: "Raider 3022" with no such contact but a "Raider 302"
// on scope becomes "Raider 302". Marshal tracked a phantom "Raider 3022" on
// DCS3 2026-10-15 14:54:27 (also 311 -> 3311 on 2026-07-02). Unchanged when
// the callsign is known, when no single repair is known, or when two are.
func undoubleCallsign(cs string, known func(string) bool) string {
	i := strings.LastIndexByte(cs, ' ')
	if i < 0 || known == nil || !isAllDigits(cs[i+1:]) || len(cs)-i-1 < 3 || known(cs) {
		return cs
	}
	word, num := cs[:i], cs[i+1:]
	found := ""
	for j := 1; j < len(num); j++ {
		if num[j] != num[j-1] {
			continue
		}
		c := word + " " + num[:j] + num[j+1:]
		if c == found || !known(c) {
			continue
		}
		if found != "" {
			return cs
		}
		found = c
	}
	if found == "" {
		return cs
	}
	return found
}
