package main

import (
	"context"
	"math"
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
//
// Against the Master Arms Case III page (operator 2026-10-26, "do 1, 3, 5"):
//
//   - "dirty up" at 8 miles, from the same picture.
//   - Bolter and wave-off: after the Paddles handoff the jet is still watched.
//     Seen climbing away ahead of the ship (or calling "bolter" / "wave off"),
//     it gets the Case III bolter pattern: straight ahead, climb to 1200, turn
//     downwind at 4 miles, report abeam. Back astern inside 3 miles it is
//     handed to Paddles again. Sitting on the deck, it is done.

const (
	caseThreeApproachFor = 20 * time.Minute // commencing to the deck, with room for a wave-off
	caseThreePaddlesNm   = 3                // the handoff range
	caseThreeDirtyNm     = 8                // "dirty up"
	caseThreeTick        = 3 * time.Second
	caseThreeTrapTicks   = 4 // ticks on the deck at 0 nm, angels 0: trapped
)

// case3Phase is where a commenced Case III jet is.
type case3Phase int

const (
	case3Inbound     case3Phase = iota // commenced, not yet at 3 miles
	case3WithPaddles                   // handed to Paddles
	case3Pattern                       // bolter or wave-off, coming round again
)

type case3Jet struct {
	say       string // how Marshal names it on the radio
	since     time.Time
	phase     case3Phase
	dirtySaid bool
	astern    int // true bearing from the ship at the handoff, -1 unknown
	closest   int // nearest range since the handoff
	onDeck    int // ticks in a row at 0 nm, angels 0
	patternAt time.Time
}

// case3Say is what the watcher has to say about a jet.
type case3Say int

const (
	case3SayNothing case3Say = iota
	case3SayDirty
	case3SayPaddles
	case3SayBolter
	case3Trapped
)

// step moves a jet on from one Tacview fix (true bearing from the ship).
func (j *case3Jet) step(angels, dist, brg int, now time.Time) case3Say {
	switch j.phase {
	case case3Inbound:
		if dist <= caseThreePaddlesNm {
			j.phase, j.astern, j.closest, j.onDeck = case3WithPaddles, brg, dist, 0
			return case3SayPaddles
		}
		if !j.dirtySaid && dist <= caseThreeDirtyNm {
			j.dirtySaid = true
			return case3SayDirty
		}
	case case3Pattern:
		// Back on final: inside 3 miles and astern, not abeam on the 2-mile
		// downwind.
		if dist <= caseThreePaddlesNm && (j.astern < 0 || angleDiff(float64(brg), float64(j.astern)) <= 30) {
			if j.astern < 0 {
				j.astern = brg
			}
			j.phase, j.closest, j.onDeck = case3WithPaddles, dist, 0
			return case3SayPaddles
		}
	case case3WithPaddles:
		if dist < j.closest {
			j.closest = dist
		}
		if dist == 0 && angels == 0 {
			if j.onDeck++; j.onDeck >= caseThreeTrapTicks {
				return case3Trapped
			}
		} else {
			j.onDeck = 0
		}
		// Over the ship and away ahead of it.
		if j.closest <= 1 && dist >= 2 && angleDiff(float64(brg), float64((j.astern+180)%360)) <= 60 {
			j.phase, j.since, j.patternAt = case3Pattern, now, now
			return case3SayBolter
		}
	}
	return case3SayNothing
}

// case3Approach: the Case III jets that commenced and are not yet on deck.
var case3Approach = struct {
	sync.Mutex
	jets map[string]*case3Jet
}{jets: map[string]*case3Jet{}}

// trackCase3Approach starts watching a jet that commenced in Case III.
func trackCase3Approach(callsign, say string, now time.Time) {
	case3Approach.Lock()
	case3Approach.jets[callsign] = &case3Jet{say: say, since: now, astern: -1}
	case3Approach.Unlock()
}

// case3CalledPattern puts a jet that called "bolter", "wave off" or "abeam"
// into the bolter pattern, watching it if it was not; astern is the true
// bearing of final from the ship (-1 unknown). told: the bolter pattern was
// given in the last minute, so a bolter call needs only an ack.
func case3CalledPattern(callsign, say string, astern int, now time.Time) (told bool) {
	case3Approach.Lock()
	defer case3Approach.Unlock()
	j := case3Approach.jets[callsign]
	if j == nil {
		j = &case3Jet{say: say, astern: astern}
		case3Approach.jets[callsign] = j
	}
	if j.astern < 0 {
		j.astern = astern
	}
	told = j.phase == case3Pattern && now.Sub(j.patternAt) < time.Minute
	if j.phase != case3Pattern {
		j.patternAt = now
	}
	j.phase, j.since = case3Pattern, now
	return told
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
var approachCallRe = regexp.MustCompile(`(?i)\b(?:checking in|check in|platform|needles|coupled|command control|ball|abeam|bolter|wave[\s-]?off|waved off)\b|\b\d+\s*(?:miles?|dme)\b`)

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

// bolterCallRe is a jet reporting its own bolter or wave-off.
var bolterCallRe = regexp.MustCompile(`\b(?:bolter|wave[\s-]?off|waved off|waving off)\b`)

// case3AsternTrue is the true bearing of final from the ship, from its BRC
// (true): final bearing is BRC less the angled-deck offset, and final lies
// the other way. -1 when the BRC is unknown.
func case3AsternTrue(brcTrue float64) int {
	if brcTrue < 0 {
		return -1
	}
	return ((int(math.Round(brcTrue))-marshalFinalBearingOffset+180)%360 + 360) % 360
}

// watchCase3Paddles calls "dirty up" at 8 miles, hands each commenced Case III
// jet to Paddles as it reaches caseThreePaddlesNm on the Tacview picture, and
// gives the bolter pattern to one seen going round again.
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
		var calls []string
		case3Approach.Lock()
		for cs, j := range case3Approach.jets {
			if now.Sub(j.since) > caseThreeApproachFor {
				delete(case3Approach.jets, cs)
				continue
			}
			angels, dist, brg, found := atcCtrl.LookupCallerRelativeToCarrier(cs)
			if !found {
				continue
			}
			switch j.step(angels, dist, brg, now) {
			case case3SayDirty:
				log.Info().Str("callsign", cs).Int("distNm", dist).Msg("Marshal: Case 3 at eight miles - dirty up")
				calls = append(calls, comp.MarshalCase3DirtyUp(j.say))
			case case3SayPaddles:
				log.Info().Str("callsign", cs).Int("distNm", dist).Int("bearingTrue", brg).Msg("Marshal: Case 3 at three miles - handing to Paddles")
				calls = append(calls, comp.MarshalCase3Paddles(j.say))
			case case3SayBolter:
				log.Info().Str("callsign", cs).Int("distNm", dist).Int("bearingTrue", brg).Msg("Marshal: Case 3 bolter or wave-off - bolter pattern")
				calls = append(calls, comp.MarshalCase3Bolter(j.say, ""))
			case case3Trapped:
				log.Info().Str("callsign", cs).Msg("Marshal: Case 3 jet on deck")
				delete(case3Approach.jets, cs)
			}
		}
		case3Approach.Unlock()
		for _, c := range calls {
			transmit(c)
		}
	}
}
