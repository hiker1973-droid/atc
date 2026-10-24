package main

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/state"
)

// Garbled calls from a jet Marshal is already working (operator 2026-10-24).
//
// Day 2026-10-23 17:10:28 and 2026-10-24 08:01:51: Marshal asked "say state"
// and the answer came back "Junior Marshall, Raider 312, state 5.9" / "Senior
// Marshal, reader 302, 5.7" - not addressed to Marshal, so ignored. Day
// 2026-10-23 17:17:03: "In marshal radio 3.2, three mile initial" was answered
// "Radio 3, Union Marshal, paddles has you": the commencing jet had already
// left the stack, so the garbled callsign went out as heard.
//
// Marshal now remembers who it talked to. A stack call whose callsign is no
// jet Marshal knows takes the one it does: the jet with the same modex, else
// the only jet Marshal has worked lately. And for a minute after "say state",
// a call that carries only a fuel state is that jet's answer, however the
// address came out.

const (
	marshalRecentWindow = 20 * time.Minute
	marshalStateWindow  = time.Minute
)

var marshalRecent = struct {
	mu     sync.Mutex
	worked map[string]time.Time // callsign -> last call Marshal answered
	asked  map[string]time.Time // callsign -> when Marshal said "say state"
}{worked: map[string]time.Time{}, asked: map[string]time.Time{}}

func noteMarshalWorked(cs string, now time.Time) {
	if cs == "" {
		return
	}
	marshalRecent.mu.Lock()
	marshalRecent.worked[cs] = now
	marshalRecent.mu.Unlock()
}

func noteMarshalAskedState(cs string, now time.Time) {
	marshalRecent.mu.Lock()
	marshalRecent.asked[cs] = now
	marshalRecent.mu.Unlock()
}

// marshalWorked: the jets Marshal answered within the window, plus the stack.
func marshalWorked(stack *state.MarshalStack, now time.Time) []string {
	seen := map[string]bool{}
	var out []string
	add := func(cs string) {
		if cs != "" && !seen[cs] {
			seen[cs] = true
			out = append(out, cs)
		}
	}
	marshalRecent.mu.Lock()
	for cs, t := range marshalRecent.worked {
		if now.Sub(t) <= marshalRecentWindow {
			add(cs)
		} else {
			delete(marshalRecent.worked, cs)
		}
	}
	marshalRecent.mu.Unlock()
	for _, ac := range stack.GetAll() {
		add(ac.Callsign)
	}
	return out
}

// marshalFixesCallsign: the calls only a jet already working Marshal makes.
// A check-in, marking mom's or radar check can be a new jet, so keeps its own.
func marshalFixesCallsign(in marshalIntent) bool {
	switch in {
	case marshalEstablished, marshalCommencing, marshalSeeYouAtTen, marshalInitial,
		marshalState, marshalPlatform, marshalPushPaddles:
		return true
	}
	return false
}

// fixMarshalCallsign maps a garbled callsign on a stack call to the jet Marshal
// is working: the one with the same modex, else the only one.
func fixMarshalCallsign(callsign string, in marshalIntent, stack *state.MarshalStack, now time.Time) string {
	if !marshalFixesCallsign(in) {
		return callsign
	}
	worked := marshalWorked(stack, now)
	for _, cs := range worked {
		if strings.EqualFold(cs, callsign) {
			return callsign
		}
	}
	_, num := splitCallsign(callsign)
	pick := ""
	if len(num) >= 3 {
		for _, cs := range worked {
			if _, n := splitCallsign(cs); n == num {
				if pick != "" {
					return callsign // two jets with that modex: don't guess
				}
				pick = cs
			}
		}
	}
	if pick == "" && len(worked) == 1 {
		pick = worked[0]
	}
	if pick == "" {
		return callsign
	}
	log.Info().Str("heard", callsign).Str("callsign", pick).Msg("Marshal: garbled callsign matched to the jet Marshal is working")
	return pick
}

// looseStateRe is a fuel state anywhere in a call: "5.7", "state 5 point 7".
var looseStateRe = regexp.MustCompile(`\b(\d{1,2})(?:\.| point )(\d)\b`)

// pendingStateAnswer: a call that skipped (or garbled) the address but carries
// a fuel state, within a minute of Marshal asking exactly one jet "say state".
// Never our own echo: every Marshal TX names "Union Marshal" and its acks say
// "copy" or "say state".
func pendingStateAnswer(text string, now time.Time) (string, float64) {
	lower := strings.ToLower(text)
	if containsAny(lower, "union", "onion", "copy", "roger", "say state", "charlie") {
		return "", 0
	}
	m := looseStateRe.FindStringSubmatch(lower)
	if m == nil {
		return "", 0
	}
	v, err := strconv.ParseFloat(m[1]+"."+m[2], 64)
	if err != nil || v <= 0 || v >= 30 {
		return "", 0
	}
	marshalRecent.mu.Lock()
	defer marshalRecent.mu.Unlock()
	cs := ""
	for c, t := range marshalRecent.asked {
		if now.Sub(t) > marshalStateWindow {
			delete(marshalRecent.asked, c)
			continue
		}
		if cs != "" {
			return "", 0 // two jets asked: can't tell whose answer
		}
		cs = c
	}
	if cs != "" {
		delete(marshalRecent.asked, cs)
	}
	return cs, v
}
