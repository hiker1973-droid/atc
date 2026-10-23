package main

import (
	"fmt"
	"strings"
)

// Only one copy of each role may run (operator 2026-10-23: "we need a check
// that only one roll of each can run"). Day 06:39 that morning had every
// Caucasus tower running two or three times: start_towers_caucasus.bat starts
// all five towers, so starting it while some were up doubled the rest, and
// Stop/Update then killed one copy each. Two copies of a tower answer every
// call twice and fight over the dashboard port.
//
// The check lives in atc.exe, not the launcher, so it holds however the role
// was started: region bat, tower bat, Update, or by hand. The second copy logs
// why and exits; the first keeps the frequency.

// instanceKey names a role: two processes with the same key are the same role
// on the same SRS server. Blue Command and Red Command differ by coalition,
// Batumi Tower and Deckboss (both --airfield UGSB) by mode, Caucasus and Syria
// Command by --map. A tower's key leaves its side out (""): a field has one
// tower, blue or red, never both on its frequency.
func instanceKey(mode, icao, mapName, coalition, srsAddr string) string {
	k := fmt.Sprintf("vsfg7-atc|%s|%s|%s|%s|%s", mode, icao, mapName, coalition, srsAddr)
	// A kernel object name may not contain a backslash.
	return strings.ToLower(strings.ReplaceAll(k, `\`, "/"))
}
