package main

import (
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/state"
)

// Flight calls that name no stack jet (Day 2026-10-20, Training 1). Raider 302
// checked in by modex and was given angels 3; from then on his lead called as
// the flight, "Raider 2 flight, entering stack position 4, angels 3", and once
// with no callsign at all, "Marshal established, Angels 2, state 6.4". Marshal
// treated "Raider 2" as a new jet ("entering stack at angels six, position
// one"), dropped the callsign-less call, so neither jet was ever holding, the
// Charlie watcher had nobody to signal, and "Raider 2 flight commencing" left
// Raider 302 in the stack.
//
// resolveStackCallsign maps such a call onto the stack jet it means:
//   - a callsign seen before as an alias keeps its stack jet;
//   - "angels N" in the call names the one stack jet assigned angels N;
//   - a short flight number names the one stack jet of the same callsign
//     word whose modex ends in it ("Raider 2" -> "Raider 302").
//
// Only a callsign the stack doesn't hold and that is not a full modex is
// resolved: "Raider 304, marking moms ..., angels 2" is a new jet even with
// Raider 303 at angels 2. Only stack calls are resolved (see
// marshalResolvesAlias); a check-in or DME report keeps its own callsign.
var marshalAliases = struct {
	mu sync.Mutex
	m  map[string]string
}{m: map[string]string{}}

var angelsInCallRe = regexp.MustCompile(`\bangels?,?\s+(\d{1,2})\b`)

// marshalResolvesAlias: the intents a holding jet's flight call can carry.
func marshalResolvesAlias(in marshalIntent) bool {
	switch in {
	case marshalEstablished, marshalCommencing, marshalSeeYouAtTen, marshalInitial,
		marshalState, marshalPlatform, marshalPushPaddles, marshalReadback, marshalNone:
		return true
	}
	return false
}

func resolveStackCallsign(callsign, lower string, stack *state.MarshalStack) string {
	if callsign != "" {
		if _, ok := stack.GetAircraft(callsign); ok {
			return callsign
		}
		marshalAliases.mu.Lock()
		target := marshalAliases.m[callsign]
		marshalAliases.mu.Unlock()
		if target != "" {
			if _, ok := stack.GetAircraft(target); ok {
				return target
			}
		}
	}
	word, num := splitCallsign(callsign)
	if callsign != "" && len(num) >= 3 {
		return callsign // a full modex: a jet of its own
	}
	all := stack.GetAll()
	var match *state.MarshalAircraft
	if m := angelsInCallRe.FindStringSubmatch(lower); m != nil {
		n, _ := strconv.Atoi(m[1])
		for _, ac := range all {
			if ac.Angels != n {
				continue
			}
			if w, _ := splitCallsign(ac.Callsign); word != "" && !strings.EqualFold(w, word) {
				continue
			}
			if match != nil {
				return callsign // two jets at that angels: don't guess
			}
			match = ac
		}
	}
	if match == nil && num != "" {
		for _, ac := range all {
			w, n := splitCallsign(ac.Callsign)
			if !strings.EqualFold(w, word) || len(n) <= len(num) || !strings.HasSuffix(n, num) {
				continue
			}
			if match != nil {
				return callsign
			}
			match = ac
		}
	}
	if match == nil {
		return callsign
	}
	if callsign != "" {
		marshalAliases.mu.Lock()
		marshalAliases.m[callsign] = match.Callsign
		marshalAliases.mu.Unlock()
	}
	log.Info().Str("heard", callsign).Str("callsign", match.Callsign).Msg("Marshal: flight call matched to its stack jet")
	return match.Callsign
}

// splitCallsign: "Raider 302" -> "Raider", "302". No number -> num "".
func splitCallsign(cs string) (word, num string) {
	f := strings.Fields(cs)
	if len(f) == 0 {
		return "", ""
	}
	if n := len(f); n >= 2 && isAllDigits(f[n-1]) {
		return f[0], f[n-1]
	}
	return f[0], ""
}
