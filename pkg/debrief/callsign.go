package debrief

import (
	"regexp"
	"strings"
	"unicode"
)

// Callsigns on the radio: "Raider 302", "Raider 3-0-2", "Venom 1-1". Speech
// recognition can mishear the word ("reader 302") but rarely the number, so
// a three-digit modex is the key on its own: every call from 302 is one
// aircraft. A shorter number keeps its word ("venom11").

var callsignRE = regexp.MustCompile(`(?i)\b([a-z][a-z']{2,})[ ,-]*(\d(?:[ -]?\d){0,3})\b`)

// notCallsign is the words a number follows that are not a callsign:
// stations, instructions and the fields.
var notCallsign = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`tower ground texaco shell arco command darkstar overlord magic focus wizard hitman paddles
		approach departure traffic atis information marshal marshall union mother mom mom's moms radio channel button preset
		angels angel state runway rwy base final initial stack position case bearing range radial brc heading altitude speed
		knots kts mile miles dme for commencing pushing push contact frequency freq point decimal signal charlie wire with and
		the see you are plus minus squawk level climb descend maintain ceiling visibility fuel pounds lbs thousand request
		requesting requested mark bingo joker platform degrees left right turn holding hold short taxi gate spot parking
		alpha alfa bravo delta echo foxtrot golf hotel india juliet kilo lima papa sierra tango victor whiskey yankee zulu
		hornet viper tomcat harrier warthog flight two three four inbound outbound out over number cleared clear wind
		mozdok senaki kutaisi batumi kobuleti tbilisi vaziani gudauta sukhumi sochi anapa krymsk maykop nalchik beslan
		incirlik ramat akrotiri paphos bassel damascus beirut stennis lincoln roosevelt washington truman vinson forrestal tarawa
		server time minutes minute seconds hours wires trap bolter cable requests each fox splash bandit bandits bogey
		bogeys spike mud defending winchester nails buddy group groups bullseye bulls`) {
		notCallsign[w] = true
	}
}

// CallsignIn finds the first callsign in a call: "Mozdok Tower, Raider 302,
// request taxi." -> "Raider 302".
func CallsignIn(text string) string {
	for _, m := range callsignRE.FindAllStringSubmatch(text, -1) {
		if notCallsign[strings.ToLower(m[1])] {
			continue
		}
		return formatCallsign(m[1], m[2])
	}
	return ""
}

func formatCallsign(word, num string) string {
	r := []rune(strings.ToLower(word))
	r[0] = unicode.ToUpper(r[0])
	d := digitsOf(num)
	if len(d) == 2 && strings.ContainsAny(num, " -") { // "1-1"
		d = d[:1] + "-" + d[1:]
	}
	return string(r) + " " + d
}

// CallsignKey is what two spellings of one callsign share: "#302" for a
// modex, "venom11" otherwise; "" when s holds no callsign.
func CallsignKey(s string) string {
	c := CallsignIn(s)
	if c == "" {
		if m := bareModexRE.FindStringSubmatch(s); m != nil { // "302, Fox 3": the modex alone
			return "#" + m[1]
		}
		return ""
	}
	m := callsignRE.FindStringSubmatch(c)
	d := digitsOf(m[2])
	if len(d) >= 3 {
		return "#" + d
	}
	return strings.ToLower(m[1]) + d
}

var bareModexRE = regexp.MustCompile(`^\W{0,3}(\d{3})\b`)

func hasLetter(s string) bool { return strings.IndexFunc(s, unicode.IsLetter) >= 0 }

func digitsOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// numberWords turns spoken digits into figures, the way a readback is
// compared with its clearance: "two six" -> "2 6", "niner" -> "9".
var numberWords = strings.NewReplacer()

func init() {
	var pairs []string
	for w, d := range map[string]string{"zero": "0", "one": "1", "two": "2", "three": "3", "tree": "3", "four": "4", "fower": "4",
		"five": "5", "fife": "5", "six": "6", "seven": "7", "eight": "8", "nine": "9", "niner": "9"} {
		pairs = append(pairs, w, d)
	}
	numberWords = strings.NewReplacer(pairs...)
}

var wordRE = regexp.MustCompile(`(?i)\b(zero|one|two|three|tree|four|fower|five|fife|six|seven|eight|niner|nine)\b`)

func spokenDigits(s string) string {
	return wordRE.ReplaceAllStringFunc(s, func(w string) string { return numberWords.Replace(strings.ToLower(w)) })
}
