package main

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// spokenGrid: "37T GJ 31827 70457" -> "Golf Juliet, three one eight two seven, seven zero four five seven".
// The zone designator is not spoken (operator 2026-11-13): every range is in
// 37T; the kneeboard card keeps it.
func spokenGrid(grid string) string {
	parts := strings.Fields(grid)
	if len(parts) >= 3 && len(parts[0]) == 3 && parts[0][0] >= '0' && parts[0][0] <= '9' {
		parts = parts[1:]
	}
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

var (
	sayAgainRe      = regexp.MustCompile(`\b(?:say (?:that |it |all )?again|repeat|say the (?:lines|nine|9)|come again)\b`)
	againLocationRe = regexp.MustCompile(`\b(?:location|grid|coordinates)\b`)
	againLineRe     = regexp.MustCompile(`\bline[s]?\s*(\d|one|two|three|four|five|six|seven|eight|nine)\b`)
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
		// "Say again location" / "say again grid" is line 6.
		if againLocationRe.MatchString(lower) {
			return say("%s", briefLine(t, 6)), true
		}
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

	// Only line 6, the location, is checked (operator 2026-11-13). Any attempt at
	// a readback of the other blocks, or a plain copy, is "readback correct".
	digits := spokenDigitString(lower)
	attempt := digits != "" || isTaskingAck(lower) || strings.Contains(lower, "read back") || strings.Contains(lower, strings.ToLower(t.IP))
	if d := nineData(t); !attempt && block == 3 && d.Friend != "" {
		attempt = strings.Contains(squash(lower), squash(strings.Fields(d.Friend)[0]))
	}
	if !attempt {
		return "", true // chatter
	}

	switch block {
	case 1:
		cur.stage++
		cur.briefMiss = 0
		return say("readback correct. %s", briefBlock(t, 2)), true
	case 2:
		g := gridDigits(nineData(t).Grid)
		if g == "" || gridReadOK(digits, g) || (cur.briefMiss >= 1 && gridEastingOK(digits, g)) {
			cur.stage++
			cur.briefMiss = 0
			return say("readback correct. %s", briefBlock(t, 3)), true
		}
		cur.briefMiss++
		if cur.briefMiss >= 2 {
			// Whisper garbles the northing (Raider 302 was refused four times on
			// Day 2026-11-12): say the location once more and move on.
			cur.stage++
			cur.briefMiss = 0
			return say("negative. %s Continue. %s", briefLine(t, 6), briefBlock(t, 3)), true
		}
		return say("negative, readback incorrect. %s Read back location.", briefLine(t, 6)), true
	}
	// Line 8: a friendly distance that is not the briefed one is read back once
	// (Raider 312, 2026-11-13: "1,800 meter friendly" against 1400 was confirmed).
	if want := friendMetres(nineData(t).Friend); want != "" && cur.briefMiss < 1 {
		if got := friendMetresHeard(lower); got != "" && got != want {
			cur.briefMiss++
			return say("negative, line 8, friendlies %s metres. Read back.", want), true
		}
	}
	cur.stage = jtacReady
	cur.briefMiss = 0
	return say("confirmed. Report IP inbound."), true
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

// soleFlight is the key of the one flight active on this net, for a call
// addressed to Hitman that carried no usable callsign ("Hitman 21, pickle times
// two", Day 2026-11-13). The call must be one the exchange acts on; "" when
// none or several flights are active.
func (b *taskingBoard) soleFlight(text string) string {
	if k := b.soleBriefing(text); k != "" {
		return k
	}
	lower := strings.ToLower(jtacAddressFixRe.ReplaceAllString(text, "hitman"))
	if k := b.soleLoadAsked(lower); k != "" {
		return k
	}
	if !containsAny(lower, "hitman", "hammer", "hey man") {
		return ""
	}
	if !(jtacReleaseCallRe.MatchString(lower) || jtacAbortRe.MatchString(lower) || jtacOffRe.MatchString(lower) ||
		jtacInRe.MatchString(lower) || jtacOffStationRe.MatchString(lower) || isJTACInbound(lower) || isBDARequest(lower)) {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	key := ""
	for k, cur := range b.flights {
		last := cur.issued
		if cur.clearedAt.After(last) {
			last = cur.clearedAt
		}
		if cur.stage < jtacBriefed || now.Sub(last) > 20*time.Minute {
			continue
		}
		if key != "" {
			return ""
		}
		key = k
	}
	return key
}

// gridEastingOK: the first four digits of the grid's easting are in the readback.
func gridEastingOK(digits, g string) bool {
	return len(g) == 10 && strings.Contains(digits, g[:4])
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

// lineSixRe is the location line, through its closing full stop (a grid has none).
var lineSixRe = regexp.MustCompile(`(?i)\bline 6,[^.]*\.`)

// jtacLineSixSlow: line 6 is spoken 12% slower again than the other 9-line lines.
const jtacLineSixSlow = 0.88

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
	}{{pre, vp}}
	// Line 6, the location, is read a further 12% slower than the rest of the
	// 9-line (operator 2026-11-13).
	nl := slowedVoice(vp, vp.lineSlow)
	if loc := lineSixRe.FindStringIndex(slow); loc != nil {
		for _, seg := range []struct {
			t string
			v voiceProfile
		}{{slow[:loc[0]], nl}, {slow[loc[0]:loc[1]], slowedVoice(nl, jtacLineSixSlow)}, {slow[loc[1]:], nl}} {
			parts = append(parts, struct {
				text string
				v    voiceProfile
			}{strings.TrimSpace(seg.t), seg.v})
		}
	} else {
		parts = append(parts, struct {
			text string
			v    voiceProfile
		}{slow, nl})
	}
	parts = append(parts, struct {
		text string
		v    voiceProfile
	}{post, vp})
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

var friendNumRe = regexp.MustCompile(`(\d[\d,]{2,5})\s*(?:m|met)`)

// friendMetres is the briefed line 8 distance, digits only ("West 1300 m" -> "1300").
func friendMetres(friend string) string {
	for _, f := range strings.Fields(friend) {
		if len(f) >= 3 && strings.Trim(f, "0123456789") == "" {
			return f
		}
	}
	return ""
}

// friendMetresHeard is the distance a readback gave for the friendlies, in
// metres ("1,800 meter friendly" -> "1800"); "" when none was heard.
func friendMetresHeard(lower string) string {
	if m := friendNumRe.FindStringSubmatch(lower); m != nil {
		return strings.ReplaceAll(m[1], ",", "")
	}
	return ""
}

// soleLoadAsked is the one flight Hitman asked for ordnance and playtime, when
// lower is that answer with no callsign ("8 times GBU-12s, playtime 30 minutes",
// Raider 312, 2026-11-13); "" when none or several.
func (b *taskingBoard) soleLoadAsked(lower string) string {
	if !b.askLoad || !hasLoadWords(lower) {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := ""
	for k, cur := range b.flights {
		if cur.loadAsked && !cur.loadDone && cur.stage == jtacAssigned {
			if key != "" {
				return ""
			}
			key = k
		}
	}
	return key
}
