package main

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
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
	// Always from the loaded table's own IP and target, so line 3 agrees with
	// lines 1-2 whichever mission version the table is for.
	_, d.DistNm = bearingRangeFromRef(t.IPPos, t.TgtPos[0], t.TgtPos[1])
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
		return "line 8, friendlies " + strings.TrimSuffix(strings.ToLower(d.Friend), " m") + " metres."
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
//
// miss is how many readbacks of this block were already wrong: from the second
// try the grid only has to match to the easting (Whisper garbles the northing's
// five digits: 64761 came back as "6764 61", "642" and "65" on Day 2026-11-12
// and Raider 302 was refused four times).
func briefReadbackOK(lower string, t taskingTarget, block, miss int) bool {
	digits := spokenDigitString(lower)
	d := nineData(t)
	switch block {
	case 1:
		// The heading is the line that matters. Whisper often loses the IP's
		// name ("IP, 3-1-2, 8.7 miles"; Raider 312 was refused twice on Day
		// 2026-11-12), so the name or the distance's last two digits backs it up,
		// and after one miss the heading alone is taken.
		if !strings.Contains(digits, fmt.Sprintf("%03d", t.RunIn)) {
			return false
		}
		if miss >= 1 || strings.Contains(lower, strings.ToLower(t.IP)) {
			return true
		}
		dist := strings.ReplaceAll(strconv.FormatFloat(d.DistNm, 'f', 1, 64), ".", "")
		return len(dist) >= 2 && strings.Contains(digits, dist[len(dist)-2:])
	case 2:
		// Pilots say "two zero niner" or "209er" for 2090 ft: the leading three
		// digits are enough.
		e := strconv.Itoa(t.ElevFt)
		if len(e) > 3 {
			e = e[:3]
		}
		if !strings.Contains(digits, e) {
			return false
		}
		g := gridDigits(d.Grid)
		return gridReadOK(digits, g) || (miss >= 1 && gridEastingOK(digits, g))
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
	lower = stripCallsign(lower, callsign)
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

	// Lines 4-6 are not checked (operator 2026-11-12, after Whisper garbled the
	// grid's northing and Raider 302 was refused four times): any attempt at the
	// readback, or a plain copy, is "readback correct" and lines 7-9 follow.
	if block == 2 && (spokenDigitString(lower) != "" || isTaskingAck(lower) || strings.Contains(lower, "read back")) {
		cur.stage++
		cur.briefMiss = 0
		return say("readback correct. %s", briefBlock(t, 3)), true
	}

	if briefReadbackOK(lower, t, block, cur.briefMiss) {
		cur.briefMiss = 0
		if block == 3 {
			cur.stage = jtacReady
			return say("confirmed. Report IP inbound."), true
		}
		cur.stage++
		return say("%s", briefBlock(t, block+1)), true
	}

	// A readback attempt (numbers, the IP) that was wrong; chatter is ignored.
	if spokenDigitString(lower) != "" || strings.Contains(lower, strings.ToLower(t.IP)) {
		cur.briefMiss++
		return say("negative, readback incorrect. %s", briefRetry(lower, t, block)), true
	}
	return "", true
}

// soleBriefing is the key of the one flight whose 9-line is being passed, when
// text is a repeat request addressed to Hitman; "" when none, or several.
func (b *taskingBoard) soleBriefing(text string) string {
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "hitman") && !strings.Contains(lower, "hammer") && !strings.Contains(lower, "hey man") {
		return ""
	}
	if !sayAgainRe.MatchString(lower) {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := ""
	for k, cur := range b.flights {
		if cur.stage >= jtacBriefed && cur.stage <= jtacBrief3 {
			if key != "" {
				return ""
			}
			key = k
		}
	}
	return key
}

// gridEastingOK: the first four digits of the grid's easting are in the readback.
func gridEastingOK(digits, g string) bool {
	return len(g) == 10 && strings.Contains(digits, g[:4])
}

// briefRetry is what Hitman says again after a wrong readback: block 2 gets only
// the line that was wrong (the long target line made the pilot start over),
// every other block the whole block.
func briefRetry(lower string, t taskingTarget, block int) string {
	if block != 2 {
		return briefBlock(t, block)
	}
	digits := spokenDigitString(lower)
	e := strconv.Itoa(t.ElevFt)
	if len(e) > 3 {
		e = e[:3]
	}
	if !strings.Contains(digits, e) {
		return briefLine(t, 4) + " Read back lines 4 to 6."
	}
	return briefLine(t, 6) + " Read back lines 4 to 6."
}

// gridReadOK: the full easting and northing, or the usual short readback (four
// digits each, Whisper often drops one): the eastings' first four and the
// northings' first or last four, in that order.
func gridReadOK(digits, g string) bool {
	if g == "" || strings.Contains(digits, g) {
		return true
	}
	if len(g) != 10 {
		return false
	}
	e, n := g[:5], g[5:]
	i := strings.Index(digits, e[:4])
	if i < 0 {
		return false
	}
	rest := digits[i+4:]
	return strings.Contains(rest, n[:4]) || strings.Contains(rest, n[1:]) || strings.Contains(rest, n)
}

// stripCallsign removes the pilot's own callsign from a readback, however
// Whisper spaced its digits ("Raider 3-0-2" for "raider 302").
func stripCallsign(lower, callsign string) string {
	cs := strings.ToLower(strings.TrimSpace(callsign))
	if cs == "" {
		return lower
	}
	word, num := splitCallsign(cs)
	if num == "" || word == "" {
		return strings.ReplaceAll(lower, cs, " ")
	}
	var sb strings.Builder
	sb.WriteString(regexp.QuoteMeta(word) + `[\s,.-]*`)
	for _, r := range num {
		sb.WriteString(string(r) + `[\s,.-]*`)
	}
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return strings.ReplaceAll(lower, cs, " ")
	}
	return re.ReplaceAllString(lower, " ")
}

var nineLineStartRe = regexp.MustCompile(`(?i)\bline [1-9],`)

// splitNineLine cuts a reply into what comes before the 9-line, the 9-line
// lines themselves (through the block's "Read back."), and what follows.
// slow == "" when the reply has no 9-line in it.
func splitNineLine(resp string) (pre, slow, post string) {
	loc := nineLineStartRe.FindStringIndex(resp)
	if loc == nil {
		return resp, "", ""
	}
	end := len(resp)
	if i := strings.LastIndex(resp, "Read back."); i >= loc[0] {
		end = i + len("Read back.")
	}
	return strings.TrimSpace(resp[:loc[0]]), resp[loc[0]:end], strings.TrimSpace(resp[end:])
}

// slowedVoice is v with its speech speed scaled by m (ElevenLabs floor 0.7).
func slowedVoice(v voiceProfile, m float64) voiceProfile {
	v.lineSlow = 0
	v.speed *= m
	if v.eleven.ID != "" {
		v.eleven.Speed *= m
		if v.eleven.Speed < 0.7 {
			v.eleven.Speed = 0.7
		}
	}
	return v
}

// speakWithSlowNineLine speaks resp with vp; when vp.lineSlow is set the 9-line
// portion is spoken slower and the pieces joined. Any failure of the split
// path falls back to the whole reply at the normal speed.
func speakWithSlowNineLine(ctx context.Context, apiKey, resp string, vp voiceProfile) ([]byte, error) {
	if vp.lineSlow <= 0 {
		return synthesizeSpeech(ctx, apiKey, resp, vp)
	}
	pre, slow, post := splitNineLine(resp)
	if slow == "" {
		return synthesizeSpeech(ctx, apiKey, resp, vp)
	}
	parts := []struct {
		text string
		v    voiceProfile
	}{{pre, vp}, {slow, slowedVoice(vp, vp.lineSlow)}, {post, vp}}
	var out []byte
	for _, p := range parts {
		if p.text == "" {
			continue
		}
		clip, err := synthesizeSpeech(ctx, apiKey, p.text, p.v)
		if err != nil {
			log.Warn().Err(err).Msg("9-line split TTS failed - speaking whole reply at normal speed")
			return synthesizeSpeech(ctx, apiKey, resp, vp)
		}
		if out == nil {
			out = clip
			continue
		}
		joined, err := concatMP3WithSilence(out, clip, flagFFmpeg, 0.08)
		if err != nil {
			log.Warn().Err(err).Msg("9-line join failed - speaking whole reply at normal speed")
			return synthesizeSpeech(ctx, apiKey, resp, vp)
		}
		out = joined
	}
	return out, nil
}
