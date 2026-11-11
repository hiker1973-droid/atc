package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The JTAC's 9-line in three blocks (operator 2026-11-11): lines 1-3, 4-6,
// 7-9, each read back by the pilot before the next. A wrong readback gets
// "negative, readback incorrect" and the block again; the pilot can ask for a
// block (or one line) again. After the third readback: "confirmed. Report IP
// inbound." and the exchange carries on as before. The lines are the ones on
// the pilot's kneeboard card (jtac_nine_lines.go).

// nineLineData is lines 3 and 5-8 of a target's kneeboard 9-line.
type nineLineData struct {
	Desc   string // line 5
	Grid   string // line 6, "37T GJ 31827 70457"
	Mark   string // line 7
	Friend string // line 8, "South-West 1400 m"
	DistNm float64
}

func nineKey(t taskingTarget) string {
	return fmt.Sprintf("%d/%d/%s", t.Number, t.Phase, strings.ToLower(t.Name))
}

// nineData is the target's lines, with the older tables' fields filling what
// the generated data does not have.
func nineData(t taskingTarget) nineLineData {
	d := jtacNineLines[nineKey(t)]
	if d.Desc == "" {
		d.Desc = t.Spoken
	}
	if d.Desc == "" {
		d.Desc = t.Name
	}
	if d.DistNm == 0 {
		_, d.DistNm = bearingRangeFromRef(t.IPPos, t.TgtPos[0], t.TgtPos[1])
	}
	return d
}

var spokenDigitWords = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "niner"}

var natoWords = map[byte]string{}

func init() {
	for i, w := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India", "Juliet", "Kilo", "Lima",
		"Mike", "November", "Oscar", "Papa", "Quebec", "Romeo", "Sierra", "Tango", "Uniform", "Victor", "Whiskey", "X-ray", "Yankee", "Zulu"} {
		natoWords[byte('A'+i)] = w
	}
}

// spellOut says each digit, and each letter by its NATO word.
func spellOut(s string) string {
	var out []string
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			out = append(out, spokenDigitWords[c-'0'])
		case c >= 'A' && c <= 'Z':
			out = append(out, natoWords[c])
		case c >= 'a' && c <= 'z':
			out = append(out, natoWords[c-32])
		}
	}
	return strings.Join(out, " ")
}

// spokenGrid: "37T GJ 31827 70457" -> "three seven Tango, Golf Juliet, three one eight two seven, seven zero four five seven".
func spokenGrid(grid string) string {
	parts := strings.Fields(grid)
	for i, p := range parts {
		parts[i] = spellOut(p)
	}
	return strings.Join(parts, ", ")
}

// spokenDist: 20.0 -> "two zero point zero".
func spokenDist(nm float64) string {
	s := strconv.FormatFloat(nm, 'f', 1, 64)
	i := strings.IndexByte(s, '.')
	return spellOut(s[:i]) + " point " + spellOut(s[i+1:])
}

// briefLine is one line of the 9-line as Hitman says it (n is 1..9).
func briefLine(t taskingTarget, n int) string {
	d := nineData(t)
	switch n {
	case 1:
		return "line 1, IP " + t.IP + "."
	case 2:
		return "line 2, heading " + spokenBearing(float64(t.RunIn)) + "."
	case 3:
		return "line 3, distance " + spokenDist(d.DistNm) + " miles."
	case 4:
		return "line 4, elevation " + spellOut(strconv.Itoa(t.ElevFt)) + " feet MSL."
	case 5:
		return "line 5, target, " + d.Desc + "."
	case 6:
		if d.Grid == "" {
			return "line 6, location on your kneeboard."
		}
		return "line 6, location " + spokenGrid(d.Grid) + "."
	case 7:
		if d.Mark == "" {
			return "line 7, no mark."
		}
		return "line 7, mark, " + strings.TrimRight(d.Mark, ".") + "."
	case 8:
		if d.Friend == "" {
			return "line 8, friendlies on your kneeboard."
		}
		return "line 8, friendlies " + strings.ToLower(d.Friend) + "."
	case 9:
		return "line 9, egress, back to " + t.IP + ", " + t.Egress + "."
	}
	return ""
}

// briefBlock is the three lines of a block: block 1 = lines 1-3, 2 = 4-6, 3 = 7-9.
func briefBlock(t taskingTarget, block int) string {
	var parts []string
	for n := block*3 - 2; n <= block*3; n++ {
		parts = append(parts, briefLine(t, n))
	}
	return strings.Join(parts, " ") + " Read back."
}

var numberWords = map[string]string{"zero": "0", "oh": "0", "one": "1", "two": "2", "three": "3", "tree": "3", "four": "4", "five": "5", "fife": "5",
	"six": "6", "seven": "7", "eight": "8", "nine": "9", "niner": "9"}

var nonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

// spokenDigitString turns "two three eight five" or "2,385" into "2385",
// ignoring every other word.
func spokenDigitString(lower string) string {
	var sb strings.Builder
	for _, tok := range nonAlnumRe.Split(lower, -1) {
		if d, ok := numberWords[tok]; ok {
			sb.WriteString(d)
			continue
		}
		if tok != "" && strings.Trim(tok, "0123456789") == "" {
			sb.WriteString(tok)
		}
	}
	return sb.String()
}

func squash(s string) string { return nonAlnumRe.ReplaceAllString(strings.ToLower(s), "") }

// gridDigits is the easting and northing of a grid, "3182770457".
func gridDigits(grid string) string {
	f := strings.Fields(grid)
	if len(f) < 2 {
		return ""
	}
	return f[len(f)-2] + f[len(f)-1]
}

// briefReadbackOK judges the pilot's readback of block 1-3.
func briefReadbackOK(lower string, t taskingTarget, block int) bool {
	digits := spokenDigitString(lower)
	d := nineData(t)
	switch block {
	case 1:
		return strings.Contains(lower, strings.ToLower(t.IP)) && strings.Contains(digits, fmt.Sprintf("%03d", t.RunIn))
	case 2:
		if !strings.Contains(digits, strconv.Itoa(t.ElevFt)) {
			return false
		}
		g := gridDigits(d.Grid)
		return g == "" || strings.Contains(digits, g)
	default:
		// Lines 7-9 carry no numbers to check: the friendly direction, the
		// egress altitude or a plain copy is taken.
		sq := squash(lower)
		if d.Friend != "" && strings.Contains(sq, squash(strings.Fields(d.Friend)[0])) {
			return true
		}
		alt := strings.TrimSpace(strings.TrimPrefix(t.Egress, "angels"))
		return isTaskingAck(lower) || (alt != "" && strings.Contains(digits, alt) && strings.Contains(lower, "angels"))
	}
}

var (
	sayAgainRe  = regexp.MustCompile(`\b(?:say (?:that |it |all )?again|repeat|say the (?:lines|nine|9)|come again)\b`)
	againLineRe = regexp.MustCompile(`\bline[s]?\s*(\d|one|two|three|four|five|six|seven|eight|nine)\b`)
)

// briefPassThrough: calls the briefing leaves to the rest of the exchange.
func briefPassThrough(lower string) bool {
	return isBDARequest(lower) || jtacAbortRe.MatchString(lower) || jtacReleaseCallRe.MatchString(lower) || isJTACInbound(lower) ||
		jtacOffRe.MatchString(lower) || jtacInRe.MatchString(lower) || strings.Contains(lower, "how copy")
}

// briefStep handles a call while the 9-line is being passed (cur.stage
// jtacBriefed..jtacBrief3). ok false = not a briefing call.
func (b *taskingBoard) briefStep(cur *taskingAssignment, lower, callsign string, t taskingTarget, say func(string, ...interface{}) string) (string, bool) {
	if briefPassThrough(lower) {
		return "", false
	}
	// Our own brief heard back leads with the callsign and names Hitman mid-string.
	if i := strings.Index(lower, "hitman"); i > 0 {
		return "", true
	}
	// The callsign's own digits are not part of a readback.
	lower = strings.ReplaceAll(lower, strings.ToLower(strings.TrimSpace(callsign)), " ")
	block := int(cur.stage-jtacBriefed) + 1

	if sayAgainRe.MatchString(lower) {
		if m := againLineRe.FindStringSubmatch(lower); m != nil {
			n := 0
			if v, err := strconv.Atoi(m[1]); err == nil {
				n = v
			} else if d, ok := numberWords[m[1]]; ok {
				n, _ = strconv.Atoi(d)
			}
			if n >= 1 && n <= 9 {
				return say("%s", briefLine(t, n)), true
			}
		}
		return say("%s", briefBlock(t, block)), true
	}

	if briefReadbackOK(lower, t, block) {
		if block == 3 {
			cur.stage = jtacReady
			return say("confirmed. Report IP inbound."), true
		}
		cur.stage++
		return say("%s", briefBlock(t, block+1)), true
	}

	// A readback attempt (numbers, the IP) that was wrong; chatter is ignored.
	if spokenDigitString(lower) != "" || strings.Contains(lower, strings.ToLower(t.IP)) {
		return say("negative, readback incorrect. %s", briefBlock(t, block)), true
	}
	return "", true
}
