package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/paulmach/orb"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/composer"
)

// Command replies that need the Tacview picture: the pilot's bullseye and the
// nearest tanker. Kept apart from commandResponse, which is pure text.

// magVarFn returns magnetic variation in degrees (east positive) at a point.
// Bearings on the radio are magnetic; Tacview positions give true.
type magVarFn func(orb.Point) float64

// nearestFieldMagVar uses the closest airfield on the current map as the local
// variation. Good to a degree or two across a theatre, which is inside what a
// spoken three-digit bearing resolves anyway.
func nearestFieldMagVar(p orb.Point) float64 {
	best, bestNm := 0.0, math.MaxFloat64
	for _, fld := range airfield.FieldsForMap(flagMap) {
		if d := haversineNm(p, fld.Center); d < bestNm {
			best, bestNm = fld.MagVar, d
		}
	}
	return best
}

func toMagnetic(trueDeg, magVar float64) float64 {
	return math.Mod(trueDeg-magVar+720, 360)
}

func isBullseyeRequest(lower string) bool {
	return containsAny(lower, "say bullseye", "request bullseye", "bullseye check", "say my bullseye", "request my bullseye", "bullseye position")
}

// isRadarCheck: Marshal and the towers answer it; Command did not (DCS3
// 2026-10-15 14:51:49, open since 2026-10-14).
func isRadarCheck(lower string) bool {
	return containsAny(lower, "radar check", "request radar", "radar contact check")
}

func isTankerRequest(lower string) bool {
	return containsAny(lower, "request tanker", "say tanker", "where's the tanker", "where is the tanker", "wheres the tanker",
		"nearest tanker", "tanker location", "tanker bearing",
		"say texaco", "request texaco", "say arco", "request arco", "say shell", "request shell")
}

// commandReply is the full Command answer to a transcript: the Tacview-backed
// bullseye / tanker replies first, then the tasking exchange, then the
// text-only intents.
func commandReply(text, callsign, channelName string, store *tacviewPositions) string {
	if resp := wrongRadioMarshal(text, callsign, channelName); resp != "" {
		return resp
	}
	if resp := commandPositionResponse(text, callsign, channelName, store, nearestFieldMagVar); resp != "" {
		return resp
	}
	if resp := commandBrevityResponse(text, callsign, channelName, store, nearestFieldMagVar); resp != "" {
		return resp
	}
	var pic jtacPicture
	if store != nil {
		pic = tacviewJTAC{store: store}
	}
	if resp := commandRangeAreaBoard().Response(text, callsign, channelName, pic, nearestFieldMagVar); resp != "" {
		return resp
	}
	if fb := footholdTaskingBoard(); fb != nil {
		if resp := fb.Response(text, callsign, channelName, store); resp != "" {
			return resp
		}
	}
	if resp := commandTaskingBoard().Response(text, callsign, channelName); resp != "" {
		return resp
	}
	if resp := commandResponse(text, callsign, channelName); resp != "" {
		return resp
	}
	return commandShotResponse(text, callsign, channelName, store)
}

var (
	foxRe     = regexp.MustCompile(`\bfox[\s-]*(1|2|3|one|two|three)\b`)
	splashRe  = regexp.MustCompile(`\bsplash(ed)?\b`)
	bareNumRe = regexp.MustCompile(`^(\d{2,3})\b`)
	// The attack calls a pilot makes to Command when no JTAC is up (Day
	// 2026-10-19 19:44-19:45: "inbound artillery battery", "in hot", "pickle
	// times two" were misses; "proceeding to IP Granite" at 19:28).
	inHotRe      = regexp.MustCompile(`\b(in hot|rolling in|in from the \w+)\b`)
	releaseRe    = regexp.MustCompile(`\b(pickle|rifle|bombs away)\b`)
	attackMoveRe = regexp.MustCompile(`\b(inbound|proceeding|en ?route)\b`)
)

var foxSpoken = map[string]string{"1": "one", "2": "two", "3": "three", "one": "one", "two": "two", "three": "three"}

// commandShotResponse copies a shot or a kill (Day 2026-10-19 16:59-17:00:
// "302 Fox 3, 26 for 73", "302, splash MiG-29" were intent misses). Only
// reached when nothing else answered. Pilots call these with the flight
// number alone, so "302" becomes the Tacview callsign that ends in it. Also
// the attack calls: in hot, pickle/rifle, inbound/proceeding.
func commandShotResponse(text, callsign, channelName string, store *tacviewPositions) string {
	lower := strings.ToLower(text)
	splash := splashRe.MatchString(lower)
	fox := foxRe.FindStringSubmatch(lower)
	release := releaseRe.FindStringSubmatch(lower)
	inHot := inHotRe.MatchString(lower)
	move := attackMoveRe.MatchString(lower)
	if !splash && fox == nil && release == nil && !inHot && !move {
		return ""
	}
	// "302 Fox 3, 26 for 73" extracts as "302 Fox 3".
	if m := bareNumRe.FindStringSubmatch(callsign); m != nil {
		callsign = m[1]
		if store != nil {
			if full := store.CallsignByNumber(m[1]); full != "" {
				callsign = full
			}
		}
	}
	if callsign == "" {
		return ""
	}
	switch {
	case release != nil:
		word := release[1]
		if word == "bombs away" {
			word = "pickle"
		}
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy %s.", callsign, channelName, word),
			fmt.Sprintf("%s, %s, copy %s, call off safe.", callsign, channelName, word),
			fmt.Sprintf("%s, %s, roger %s.", callsign, channelName, word),
		})
	case inHot:
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, cleared hot.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy in hot, cleared hot.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger, cleared hot.", callsign, channelName),
		})
	case !splash && fox == nil: // inbound / proceeding
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy, continue.", callsign, channelName),
		})
	}
	if splash {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy splash, good kill.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy splash.", callsign, channelName),
			fmt.Sprintf("%s, %s, splash copied, good shooting.", callsign, channelName),
		})
	}
	n := foxSpoken[fox[1]]
	return commandPick([3]string{
		fmt.Sprintf("%s, %s, copy Fox %s.", callsign, channelName, n),
		fmt.Sprintf("%s, %s, copy your Fox %s.", callsign, channelName, n),
		fmt.Sprintf("%s, %s, roger Fox %s.", callsign, channelName, n),
	})
}

var (
	winchesterRe = regexp.MustCompile(`\bwinchester\b`)
	bingoRe      = regexp.MustCompile(`\bbingo\b`)
	jokerRe      = regexp.MustCompile(`\bjoker\b`)
)

// commandBrevityResponse answers the calls that close out a range run
// (operator 2026-10-16: after "copy off Alpha, say intentions" the pilot
// says "RTB" or "Winchester, RTB" - copy it, in brevity):
//
//	Winchester  out of ordnance: done, RTB, handoff at 30 nm
//	Bingo       fuel to get home: RTB, the nearest tanker if Tacview has one
//	Joker       fuel to keep working a little: copy, call Bingo
//
// Any of these, or RTB / fence out, frees the range area the flight held.
// A plain RTB with no "off" in it is left to commandResponse.
func commandBrevityResponse(text, callsign, channelName string, store *tacviewPositions, magVar magVarFn) string {
	lower := strings.ToLower(text)
	win, bingo, joker := winchesterRe.MatchString(lower), bingoRe.MatchString(lower), jokerRe.MatchString(lower)
	recovery := isRecoveryCall(lower)
	if !win && !bingo && !joker && !recovery {
		return ""
	}
	if extractFuelStateMarshal(lower) > 0 && !win {
		return "" // "bingo, state 1.8": the fuel state reply has it
	}
	board := commandRangeAreaBoard()
	if joker && !win && !bingo && !recovery {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy Joker.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy Joker, call Bingo.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger Joker.", callsign, channelName),
		})
	}
	// "off Alpha, Winchester, RTB" answers the off call too.
	off := ""
	if word := board.Leave(callsign); word != "" && (rangeAreaOffRe.MatchString(lower) || rangeAreaOffAnyRe.MatchString(lower)) {
		off = fmt.Sprintf("copy off %s, ", word)
	} else if rangeAreaOffRe.MatchString(lower) || rangeAreaOffAnyRe.MatchString(lower) {
		off = "copy off target, "
	}
	handoff := "Expect recovery handoff at three zero miles."
	switch {
	case win:
		lead := "copy Winchester, RTB."
		if off != "" {
			lead = off + "Winchester, RTB."
		}
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, %s Good work. %s", callsign, channelName, lead, handoff),
			fmt.Sprintf("%s, %s, %s Cleared off the range. %s", callsign, channelName, lead, handoff),
			fmt.Sprintf("%s, %s, %s Good shooting. %s", callsign, channelName, lead, handoff),
		})
	case bingo:
		lead := "copy Bingo, RTB."
		if off != "" {
			lead = off + "Bingo, RTB."
		}
		tanker := ""
		if store != nil {
			if pos, ok := store.Get(callsign); ok {
				if tk, ok := store.NearestTanker(pos, store.Info(callsign).Coalition); ok {
					brg, nm := bearingRangeFromRef(pos, tk.pos[0], tk.pos[1])
					tanker = fmt.Sprintf(" Nearest tanker %s bears %s for %d, angels %d.", tk.callsign,
						spokenBearing(toMagnetic(brg, magVar(pos))), int(nm+0.5), int(tk.info.AltFt/1000+0.5))
				}
			}
		}
		return fmt.Sprintf("%s, %s, %s%s %s", callsign, channelName, lead, tanker, handoff)
	case off != "":
		// "off target, RTB"
		return fmt.Sprintf("%s, %s, %sRTB. %s", callsign, channelName, off, handoff)
	}
	return "" // plain RTB / fence out: commandResponse, area already freed
}

// commandPositionResponse answers "say bullseye" and "request tanker" from the
// Tacview store. Returns "" for any other call. When the picture can't answer
// it says so rather than going silent, because the pilot asked a question.
func commandPositionResponse(text, callsign, channelName string, store *tacviewPositions, magVar magVarFn) string {
	lower := strings.ToLower(text)
	wantBull, wantTanker, wantRadar := isBullseyeRequest(lower), isTankerRequest(lower), isRadarCheck(lower)
	if !wantBull && !wantTanker && !wantRadar {
		return ""
	}
	if store == nil {
		return fmt.Sprintf("%s, %s, unable, no radar picture.", callsign, channelName)
	}
	pos, ok := store.Get(callsign)
	if !ok {
		return fmt.Sprintf("%s, %s, unable, no radar contact, say position.", callsign, channelName)
	}
	coalition := store.Info(callsign).Coalition

	if wantTanker {
		tk, ok := store.NearestTanker(pos, coalition)
		if !ok {
			return fmt.Sprintf("%s, %s, negative tanker on scope.", callsign, channelName)
		}
		brg, nm := bearingRangeFromRef(pos, tk.pos[0], tk.pos[1])
		reply := fmt.Sprintf("%s, %s, %s bears %s for %d, angels %d.",
			callsign, channelName, tk.callsign,
			spokenBearing(toMagnetic(brg, magVar(pos))), int(nm+0.5), int(tk.info.AltFt/1000+0.5))
		// The tanker with a SkyEye net (tanker.go) takes his check-in.
		for _, cfg := range tankerConfigs() {
			if strings.EqualFold(tk.callsign, cfg.Tacview) {
				reply += fmt.Sprintf(" Contact %s on %s.", cfg.Spoken, composer.SpellFrequency(cfg.FreqMHz))
				break
			}
		}
		return reply
	}

	bull, ok := store.Bullseye(coalition)
	if wantRadar {
		angels := int(store.Info(callsign).AltFt/1000 + 0.5)
		if !ok {
			return fmt.Sprintf("%s, %s, radar contact, angels %d.", callsign, channelName, angels)
		}
		brg, nm := bearingRangeFromRef(bull, pos[0], pos[1])
		return fmt.Sprintf("%s, %s, radar contact, bullseye %s for %d, angels %d.",
			callsign, channelName, spokenBearing(toMagnetic(brg, magVar(bull))), int(nm+0.5), angels)
	}
	if !ok {
		return fmt.Sprintf("%s, %s, unable, no bullseye on scope.", callsign, channelName)
	}
	brg, nm := bearingRangeFromRef(bull, pos[0], pos[1])
	return fmt.Sprintf("%s, %s, bullseye %s for %d.",
		callsign, channelName, spokenBearing(toMagnetic(brg, magVar(bull))), int(nm+0.5))
}

// marshalAddresses open a call meant for Marshal (Whisper's spellings too).
var marshalAddresses = []string{"union marshall", "union marshal", "unit marshall", "marshall", "marshal"}

// wrongRadioMarshal answers a call that opens with Marshal's address on
// Command's frequency with where Marshal is (operator ruling 2026-10-15, after
// Raider 302 called "Union Marshall" on 282 at 14:56). "" when the call is
// not addressed to Marshal, or Command has no Marshal frequency.
func wrongRadioMarshal(text, callsign, channelName string) string {
	if flagHandoffMarshalFreq <= 0 {
		return ""
	}
	lower := strings.TrimLeft(strings.ToLower(text), " ,.")
	for _, a := range marshalAddresses {
		if strings.HasPrefix(lower, a) && (len(lower) == len(a) || strings.ContainsRune(" ,.", rune(lower[len(a)]))) {
			who := callsign
			if who == "" {
				who = "Station calling"
			}
			return fmt.Sprintf("%s, %s, you are on Command. You need to tune to %s, %s.", who, channelName,
				flagHandoffMarshalName, formatFreqMHz(flagHandoffMarshalFreq))
		}
	}
	return ""
}
