package main

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

// case3EAT is the first whole minute at or after earliest that no other jet in
// the stack holds: Case III approach times are marshalCase3IntervalMin apart
// (operator 2026-10-25: "push time spacing is 1 min for other aircraft").
func case3EAT(stack *state.MarshalStack, callsign string, earliest time.Time) time.Time {
	eat := earliest.Truncate(time.Minute)
	if eat.Before(earliest) {
		eat = eat.Add(time.Minute)
	}
	taken := map[int64]bool{}
	for _, ac := range stack.GetAll() {
		if ac.Callsign != callsign && !ac.EAT.IsZero() {
			taken[ac.EAT.Truncate(time.Minute).Unix()] = true
		}
	}
	for taken[eat.Unix()] {
		eat = eat.Add(time.Duration(marshalCase3IntervalMin) * time.Minute)
	}
	return eat
}

// case3HoldEAT is a holding jet's approach time: the one it has, or a new one
// if it never got one or it is too close to give. Night 2026-10-25 14:29:
// Raider 319 reported established six minutes after his :23 and was told "your
// push time is two three" again.
func case3HoldEAT(stack *state.MarshalStack, callsign string, position int, eat, now time.Time) time.Time {
	switch {
	case eat.IsZero():
		eat = case3EAT(stack, callsign, now.Add(time.Duration(marshalCase3LeadTimeMin+(position-1)*marshalCase3IntervalMin)*time.Minute))
	case eat.Before(now.Add(marshalCase3MinNotice)):
		old := eat
		eat = case3EAT(stack, callsign, now.Add(marshalCase3MinNotice))
		log.Info().Str("callsign", callsign).Time("was", old).Time("eat", eat).Msg("Marshal: Case 3 approach time had passed - new one")
	default:
		return eat
	}
	stack.SetEAT(callsign, eat)
	return eat
}

// Case III approach, after commencing (operator 2026-10-25, against the
// squadron's DimOn Case III kneeboard, cards 26-27: "do all 4"):
//
//   - Marshal hands the jet to Paddles at 3 miles. In Case I the pilot calls
//     "three mile initial" and Marshal answers "contact paddles"; in Case III
//     nobody calls Marshal after "platform", so a commenced jet was never
//     handed off (Night 2026-10-25, Raider 319). The watcher below makes the
//     call from the Tacview picture as the jet reaches 3 nm.
//   - The card's approach calls come callsign-first, without "Marshal":
//     "319, checking in, 15 miles", "319, platform", "319, needles up and
//     left", "319, negative needles", "319, coupled", "319, command control".
//     From a jet that commenced in the last caseThreeApproachFor they are
//     taken as addressed to Marshal (unaddressedApproachCallsign).

const (
	caseThreeApproachFor = 20 * time.Minute // commencing to the deck, with room for a wave-off
	caseThreePaddlesNm   = 3                // the handoff range
	caseThreeTick        = 3 * time.Second
)

type case3Jet struct {
	say   string // how Marshal names it on the radio
	since time.Time
}

// case3Approach: the Case III jets that commenced and are not yet with Paddles.
var case3Approach = struct {
	sync.Mutex
	jets map[string]case3Jet
}{jets: map[string]case3Jet{}}

// trackCase3Approach starts watching a jet that commenced in Case III.
func trackCase3Approach(callsign, say string, now time.Time) {
	case3Approach.Lock()
	case3Approach.jets[callsign] = case3Jet{say: say, since: now}
	case3Approach.Unlock()
}

// case3ApproachJet: the callsign of a commenced Case III jet the text's
// callsign names ("319" or "Raider 319"), or "".
func case3ApproachJet(spoken string, now time.Time) string {
	digits := digitsIn(spoken)
	case3Approach.Lock()
	defer case3Approach.Unlock()
	for cs, j := range case3Approach.jets {
		if now.Sub(j.since) > caseThreeApproachFor {
			delete(case3Approach.jets, cs)
			continue
		}
		if strings.EqualFold(cs, spoken) || (len(digits) >= 2 && strings.HasSuffix(digitsIn(cs), digits)) {
			return cs
		}
	}
	return ""
}

func digitsIn(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// approachCallRe are the card's Case III approach calls.
var approachCallRe = regexp.MustCompile(`(?i)\b(?:checking in|check in|platform|needles|coupled|command control|ball)\b|\b\d+\s*(?:miles?|dme)\b`)

// approachLeadRe: a call that opens with a callsign or a bare modex.
var approachLeadRe = regexp.MustCompile(`(?i)^\W*((?:[a-z]{3,}\s*)?\d(?:[\s-]?\d){1,3})\b`)

// unaddressedApproachCallsign accepts a callsign-first approach call from a
// commenced Case III jet. Our own TX reads "<callsign>, Union Marshal, ...",
// so a call naming Marshal (or our ack words) is left to the echo guard.
func unaddressedApproachCallsign(text string, now time.Time) string {
	lower := strings.ToLower(text)
	if containsAny(lower, "marshal", "union", "onion", "paddles has you", "contact paddles", "roger", "copy") ||
		!approachCallRe.MatchString(lower) {
		return ""
	}
	m := approachLeadRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return case3ApproachJet(strings.TrimSpace(m[1]), now)
}

// needlesRe is a needles report: "needles up and left", "on and on",
// "negative needles".
var needlesRe = regexp.MustCompile(`(?i)\bneedles?\b`)

// watchCase3Paddles hands each commenced Case III jet to Paddles as it
// reaches caseThreePaddlesNm on the Tacview picture.
func watchCase3Paddles(ctx context.Context, comp *composer.ATCComposer, transmit func(string), atcCtrl *controller.ATCController) {
	tk := time.NewTicker(caseThreeTick)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}
		now := time.Now()
		var due []case3Jet
		case3Approach.Lock()
		for cs, j := range case3Approach.jets {
			if now.Sub(j.since) > caseThreeApproachFor {
				delete(case3Approach.jets, cs)
				continue
			}
			_, dist, _, found := atcCtrl.LookupCallerRelativeToCarrier(cs)
			if found && dist <= caseThreePaddlesNm {
				log.Info().Str("callsign", cs).Int("distNm", dist).Msg("Marshal: Case 3 at three miles - handing to Paddles")
				due = append(due, j)
				delete(case3Approach.jets, cs)
			}
		}
		case3Approach.Unlock()
		for _, j := range due {
			transmit(comp.MarshalCase3Paddles(j.say))
		}
	}
}
