package controller

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/state"
)

// Day 2026-10-19 21:28, Senaki: "Venom 203, declaring emergency, request
// landing on runway 09" got "you have runway two seven" (the rotation's pick,
// and 09 was into wind), "on final runway 09" got two seven again, and the
// answer to the tower's own "say souls on board and fuel state" - "one soul
// on board, fuel state zero" - got "say again your request".

var (
	// runwayNamedRe: "runway 09", "runway 9", "runway 31L".
	runwayNamedRe      = regexp.MustCompile(`\brunway\s+(\d{1,2})\s*([lrc]|left|right|cent(?:er|re))?\b`)
	soulsRe            = regexp.MustCompile(`\b(\d+|zero|one|two|three|four|a single)\s+(?:souls?|persons?|people)\b`)
	runwayDigitsJoinRe = regexp.MustCompile(`(\d)[\s-]+(\d)`) // "0 9" -> "09"
	fuelOutRe          = regexp.MustCompile(`\b(?:fuel(?: state)?\s+(?:zero|empty|none|nil|nothing)|no fuel|out of fuel|zero fuel|flamed? ?out)\b`)
)

// runwayDigitWords turns "runway zero niner" into "runway 0 9" for runwayNamedRe.
var runwayDigitWords = strings.NewReplacer("zero", "0", "niner", "9", "nine", "9", "one", "1", "two", "2",
	"three", "3", "tree", "3", "four", "4", "five", "5", "fife", "5", "six", "6", "seven", "7", "eight", "8")

// requestedRunway is the runway end a call names, as the field designates it
// ("09"), or "" for none or one the field does not have.
func requestedRunway(af *airfield.Airfield, lower string) string {
	if af == nil {
		return ""
	}
	i := strings.Index(lower, "runway")
	if i < 0 {
		return ""
	}
	tail := runwayDigitWords.Replace(lower[i:])
	tail = runwayDigitsJoinRe.ReplaceAllString(tail, "$1$2")
	m := runwayNamedRe.FindStringSubmatch(tail)
	if m == nil {
		return ""
	}
	num := strings.TrimLeft(m[1], "0")
	side := ""
	if m[2] != "" {
		side = strings.ToUpper(m[2][:1])
	}
	for _, pair := range af.RunwayPairs {
		for _, r := range []airfield.Runway{pair.Primary, pair.Reciprocal} {
			d := strings.ToUpper(r.Designator)
			dSide := strings.TrimLeft(d, "0123456789")
			if strings.TrimLeft(strings.TrimRight(d, "LRC"), "0") == num && (side == "" || side == dSide) {
				return r.Designator
			}
		}
	}
	return ""
}

// runwayFor is the runway an aircraft lands on: the one it asked for in an
// emergency, else the active.
func (c *ATCController) runwayFor(ac *state.AircraftState) string {
	if ac != nil && ac.Emergency && ac.EmergencyRunway != "" {
		return ac.EmergencyRunway
	}
	return c.airfieldState.ActiveRunway
}

// emergencyDetails answers "one soul on board, fuel state zero".
func (c *ATCController) emergencyDetails(req *ATCRequest, ac *state.AircraftState) string {
	lower := strings.ToLower(req.Raw)
	var parts []string
	if m := soulsRe.FindStringSubmatch(lower); m != nil {
		n := m[1]
		if n == "a single" {
			n = "one"
		}
		if n == "1" || n == "one" {
			parts = append(parts, n+" soul on board")
		} else {
			parts = append(parts, n+" souls on board")
		}
	}
	switch {
	case fuelOutRe.MatchString(lower):
		parts = append(parts, "fuel zero")
	case req.FuelState > 0:
		parts = append(parts, fmt.Sprintf("fuel state %s", spokenState(req.FuelState)))
	}
	copied := "copy"
	if len(parts) > 0 {
		copied = "copy " + strings.Join(parts, ", ")
	}
	tower := c.airfieldCallsign
	if ac == nil || !ac.Emergency {
		return fmt.Sprintf("%s, %s, %s.", req.Callsign, tower, copied)
	}
	return fmt.Sprintf("%s, %s, %s, crash crew standing by, runway %s, cleared to land.",
		req.Callsign, tower, copied, spokenRunway(c.runwayFor(ac)))
}

// spokenState: 5.2 -> "5 point 2".
func spokenState(v float64) string {
	s := strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0")
	return strings.Replace(s, ".", " point ", 1)
}

// spokenRunway: "09" -> "zero niner", "31L" -> "three one left".
func spokenRunway(d string) string {
	words := map[rune]string{'0': "zero", '1': "one", '2': "two", '3': "three", '4': "four", '5': "five",
		'6': "six", '7': "seven", '8': "eight", '9': "niner"}
	var out []string
	for _, r := range strings.ToUpper(d) {
		switch {
		case words[r] != "":
			out = append(out, words[r])
		case r == 'L':
			out = append(out, "left")
		case r == 'R':
			out = append(out, "right")
		case r == 'C':
			out = append(out, "center")
		}
	}
	return strings.Join(out, " ")
}
