package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vsfg7/atc/pkg/gci/callsigns"
)

// Shot and down calls on the AWACS and Command nets (operator 2026-10-21):
// "302 Fox 3, 08 for 56, 25,000, MiG-29" gets a copy from the AWACS, and
// "302 ejecting" - or another pilot's "302 is down" - gets "302 down,
// launching CSAR". Pilots make these with the flight number alone and
// without naming the station, so they are taken without an address.

// downCallRe: a callsign, then down / ejecting / bailing out. The callsign
// must sit right before the word, so "Raider 301, 302 is down" names 302.
var downCallRe = regexp.MustCompile(`(?i)(?:\b(raider|venom)s?[\s,-]*)?\b(\d(?:[\s-]?\d){2}|\d-\d)[\s,.]*(?:is\s+|has\s+)?(?:down|ejecting|ejected|eject(?:\s+eject)*|bailing\s+out|bailed\s+out|punching\s+out|punched\s+out)\b(.*)$`)

// downNotRe: "302 down to angels 5", "302 down low": a descent, not a loss.
var downNotRe = regexp.MustCompile(`(?i)^\s*(?:to|at|through|low|and\s+(?:left|right)|angels|\d)`)

// combatFoxRe: Whisper writes "Fox 3, 11 for 56" as "Fox 311" (Day
// 2026-10-21 16:43), so the digit need not end the word.
var combatFoxRe = regexp.MustCompile(`(?i)\bfox[\s-]*(one|two|three|[123])`)

// leadCallsignRe: the call opens with a callsign (after any station name).
var leadCallsignRe = regexp.MustCompile(`(?i)^\W*(?:(?:darkstar|overlord|magic|wizard|command|anyface|any face)\b[\s,.]*)?(?:(raider|venom)s?[\s,-]*)?(\d(?:[\s-]?\d){2}|\d-\d)\b`)

// downedCallsign is who a down call is about: "Raider 302" (from Tacview
// when the number is unique), else squadron + number, else the number.
// ok=false when the call is not a down call.
func downedCallsign(text string, byNumber func(string) string) (string, bool) {
	m := downCallRe.FindStringSubmatch(text)
	if m == nil || downNotRe.MatchString(m[3]) {
		return "", false
	}
	return resolveCallsign(m[1], m[2], byNumber), true
}

// leadCallsign is the callsign a shot call opens with, resolved as above.
func leadCallsign(text string, byNumber func(string) string) (string, bool) {
	m := leadCallsignRe.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return resolveCallsign(m[1], m[2], byNumber), true
}

func resolveCallsign(squadron, num string, byNumber func(string) string) string {
	num = strings.NewReplacer(" ", "", "-", "").Replace(num)
	if len(num) == 2 {
		num = num[:1] + "-" + num[1:]
	}
	if squadron == "" && byNumber != nil {
		if full := byNumber(num); full != "" {
			return full
		}
	}
	if squadron != "" {
		return strings.ToUpper(squadron[:1]) + strings.ToLower(squadron[1:]) + " " + num
	}
	return num
}

// foxNumber is "one" / "two" / "three" for a Fox call, "" if there is none.
func foxNumber(text string) string {
	if m := combatFoxRe.FindStringSubmatch(text); m != nil {
		return foxSpoken[strings.ToLower(m[1])]
	}
	return ""
}

// downReply is the CSAR call, said to everyone on the net.
func downReply(station, who string) string {
	return fmt.Sprintf("All players, %s, %s down, launching CSAR.", station, who)
}

// gciCombatCall answers a Fox, splash or down call on the AWACS net that
// does not name the AWACS ("302 Fox 3 ...", "302 splash", "302 ejecting").
// kind is "fox", "splash", "down", or "" when it is none of these.
func gciCombatCall(text, gci string, byNumber func(string) string) (kind, reply string) {
	if who, ok := downedCallsign(text, byNumber); ok {
		return "down", downReply(strings.ToUpper(gci), gciSpoken(who))
	}
	who, ok := leadCallsign(text, byNumber)
	if !ok {
		return "", ""
	}
	head := fmt.Sprintf("%s, %s, ", gciSpoken(who), strings.ToUpper(gci))
	if n := foxNumber(text); n != "" {
		return "fox", head + "copy Fox " + n + "."
	}
	if m := gciSplashRe.FindStringSubmatch(text); m != nil {
		if n := m[1]; n != "" {
			if w, ok := gciNumberWords[n]; ok {
				n = w
			}
			return "splash", head + "copy splash " + strings.ToLower(n) + "."
		}
		return "splash", head + "copy splash."
	}
	return "", ""
}

// gciSpoken writes a callsign the way the GCI says it ("RAIDER 3 0 2").
func gciSpoken(cs string) string {
	if p, ok := callsigns.ParsePilotCallsign(strings.ToLower(cs)); ok {
		return strings.ToUpper(p)
	}
	return strings.ToUpper(cs)
}
