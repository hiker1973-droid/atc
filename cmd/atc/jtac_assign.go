package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Assassin (operator 2026-11-14, Kola Training: "instead of Hitman ... they just assign targets. Keep it simple").
// --jtac-assign-only turns a JTAC net into an assigner: a flight checks in and is given a target, its grid, elevation and IP
// with run-in heading, and that is all. No 9-line readback, no cleared hot, no attack-heading checks, no BDA. The flight says
// target complete, destroyed, winchester or off station to be released, or asks for another.
//
// The board, the target set, the sector split and Command's handoff are the Hitman ones (jtac.go, command_tasking.go);
// only the net's answers differ.

var flagJTACAssignOnly bool

var (
	// assassinAddrRe: the net's own callsign, "Assassin 1-1" / "Assassin one two" / "Assassin". The digits mean nothing
	// (the net is set by frequency), so they are folded into the Hitman form jtacCallsign already parses.
	assassinAddrRe = regexp.MustCompile(`(?i)\bassassins?\b(?:[ ,-]+(?:one|two|won|to|1|2)){0,2}`)
	// assignAnotherRe: the flight wants a different target.
	assignAnotherRe = regexp.MustCompile(`\b(?:another|next|new|different|more)\b.{0,20}\b(?:target|tasking|task)\b|\b(?:target|tasking|task)\b.{0,12}\b(?:another|next|new|different)\b`)
	// assignAgainRe: repeat the assignment.
	assignAgainRe = regexp.MustCompile(`\b(?:say again|repeat|say the target|say target|confirm target|what(?:'s| is) my target|say my target)\b`)
	// assignDoneRe: the flight reports the target done.
	assignDoneRe = regexp.MustCompile(`\b(?:target|tasking|mission) complete\b|\bcomplete\b.{0,8}\btarget\b|\bwork(?:ed)? the target\b|\bbingo\b|\brtb\b|\breturning to base\b`)
)

// assassinCallsign is jtacCallsign for a net called Assassin.
func assassinCallsign(text string) string {
	return jtacCallsign(assassinAddrRe.ReplaceAllString(text, "hitman 1-1"))
}

func assignLine(t taskingTarget) string {
	d := nineData(t)
	s := fmt.Sprintf("target %d, %s.", t.Number, t.Name)
	if d.Desc != "" && d.Desc != t.Name {
		s += " " + d.Desc + "."
	}
	if d.Grid != "" {
		s += " Grid " + spokenGrid(d.Grid) + "."
	}
	if t.ElevFt > 0 {
		s += " Elevation " + spellOut(strconv.Itoa(t.ElevFt)) + " feet."
	}
	s += fmt.Sprintf(" IP %s, run-in heading %s.", t.IP, spokenBearing(float64(t.RunIn)))
	return s
}

// AssignResponse answers one call on an assign-only net. "" = not for this net.
func (b *taskingBoard) AssignResponse(text, callsign, name string) string {
	lower := strings.ToLower(text)
	key := strings.ToLower(strings.TrimSpace(callsign))
	if key == "" {
		return ""
	}
	say := func(format string, a ...interface{}) string {
		return fmt.Sprintf("%s, %s, ", callsign, name) + fmt.Sprintf(format, a...)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	cur := b.flights[key]

	if containsAny(lower, radioCheckPhrases...) {
		return say("loud and clear.")
	}
	if len(b.targets) == 0 {
		if isJTACCheckIn(lower) {
			return say("negative tasking at this time.")
		}
		return ""
	}
	assign := func(idx int) string {
		b.flights[key] = &taskingAssignment{target: idx, issued: now, stage: jtacBriefed}
		return say("%s Report target complete.", assignLine(b.targets[idx]))
	}

	switch {
	case jtacOffStationRe.MatchString(lower):
		delete(b.flights, key)
		return say("copy %s. You are released from the net.", jtacOffStationRe.FindString(lower))
	case jtacTargetDestroyedRe.MatchString(lower) || assignDoneRe.MatchString(lower):
		if cur == nil {
			return say("copy. Say again your callsign and request tasking.")
		}
		n := b.targets[cur.target].Number
		delete(b.flights, key)
		return say("copy target %d complete. Request tasking when ready.", n)
	case assignAnotherRe.MatchString(lower) && cur != nil:
		return assign(b.pick(key, now)) // pick() skips the target this flight just had
	case assignAgainRe.MatchString(lower) && cur != nil:
		return say("%s", assignLine(b.targets[cur.target]))
	case isJTACCheckIn(lower):
		if cur != nil && now.Sub(cur.issued) < taskingHoldTime {
			return say("%s", assignLine(b.targets[cur.target])) // checking in again: same target
		}
		return assign(b.pick(key, now))
	}
	return ""
}
