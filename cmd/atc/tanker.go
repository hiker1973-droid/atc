package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"
)

// Tanker net (operator 2026-10-18: "lets do it for a test Texaco 1-1 for
// training"): a voice on the tanker's frequency, beside Command like the
// Hitman nets. DCS still does the refuelling itself - the pilot calls "Ready
// pre-contact" from the comms menu and the AI tanker answers "Cleared contact".
//
// The calls are the vSFG-7 AAQ "Aerial Refueling Procedure" comms table
// (operator 2026-10-19), plus the operator's "how much do you request?"
// (2026-10-16). Observation is always port, reform always starboard; the
// KC-130 has two baskets, one jet on each:
//
//	"Texaco 1, Knight 1-1, flight of 2, 15 mile initial for port observation with intent to refuel"
//	   -> "Knight 11, Texaco one one, cleared port observation, one three thousand, velocity two six zero. How much do you request?"
//	"Knight 1-1, request 5.0"                       -> "Knight 11, Texaco, you are cleared for five thousand pounds."
//	"Knight 1-1, 1 mile trail for port observation" -> "Knight 11, Texaco, continue port observation."
//	"Knight 1-1, established port observation"      -> "Knight 11, Texaco, copy. Port and starboard baskets open."
//	"Knight 1-3, moving to starboard basket"        -> "Knight 13, Texaco, cleared starboard basket."
//	"Knight 1-3, established starboard basket"      -> "Knight 13, Texaco, copy, call ready pre-contact."
//	"Knight 1-3, satisfied, moving to starboard reform" -> "Knight 13, Texaco, cleared starboard reform."
//	"Knight 1-3, established starboard reform"      -> "Knight 13, Texaco, copy."
//	"Texaco 1, Knight 1-1, checking out, resuming navigation, pushing 282"
//	   -> "Knight 11, Texaco, copy, cleared to depart. Fortuna super te, Viking."
//
// Texaco 1-1 in V1.31 Training (Day): KC-130, 252.100 AM (Hornet COMM2 CH8),
// TACAN 51Y TX1, angels 13, two wing drogues. Its altitude and speed come
// live from Tacview (tanker_speed.go: about 260 indicated); --tanker-kias
// only stands in until the track is long enough. The AAR procedure's table
// says 336 for a KC-130, but that is not what ours flies.
//
// Texaco 3-1 (operator 2026-10-19: "do Texaco 3-1 with a different voice"),
// --tanker2-freq: KC-135 MPRS, 252.300 AM (Hornet COMM2 CH10), TACAN 53Y TX3,
// angels 24 at Mach 0.65 (the builder's tankers.py), two wing pods - the same
// port / starboard flow. Its own net and its own voice (Brian, not Eric).
//
// More nets (operator 2026-10-22: "we need to add 2 Shells for 16's and Texaco
// 2-1 for 18's"), one --tanker-net each: Texaco 2-1 KC-135 MPRS 252.200 (Hornet
// COMM2 CH9, Hank), and the boom tankers Shell 1-1 253.100 / Shell 2-1 253.200
// (Viper COMM1 CH7 / CH8, Charlie / Daniel), angels 22 at Mach 0.68. A boom has
// one receiver at a time: the same port observation / starboard reform, but
// "Boom open" and "cleared to the boom" in place of the baskets.

var (
	flagTankerFreq        float64
	flagTankerName        string
	flagTankerSpoken      string
	flagTankerTacview     string
	flagTankerAngels      int
	flagTankerKIAS        int
	flagTankerVoice       string
	flagElevenTankerVoice string

	flagTanker2Freq        float64
	flagTanker2Name        string
	flagTanker2Spoken      string
	flagTanker2Tacview     string
	flagTanker2Angels      int
	flagTanker2KIAS        int
	flagTanker2Voice       string
	flagElevenTanker2Voice string

	flagTankerNets []string
)

// tankerNetVoices are the ElevenLabs voices a --tanker-net can name.
var tankerNetVoices = map[string]string{
	"eric": elEric, "brian": elBrian, "hank": elHank, "charlie": elCharlie,
	"daniel": elDaniel, "adam": elAdam, "george": elGeorge, "gideon": elGideon,
}

func registerTankerFlags(f *flag.FlagSet) {
	f.Float64Var(&flagTankerFreq, "tanker-freq", 0,
		"With --command-only: run a tanker check-in net on this frequency in MHz, e.g. 252.1 (Texaco 1-1). 0=disabled")
	f.StringVar(&flagTankerName, "tanker-name", "Texaco-1-1", "Tanker SRS client name (no spaces)")
	f.StringVar(&flagTankerSpoken, "tanker-spoken", "Texaco one one", "Tanker callsign as spoken on the radio")
	f.StringVar(&flagTankerTacview, "tanker-tacview", "Texaco 1-1", "The tanker's Tacview callsign, for its live altitude")
	f.IntVar(&flagTankerAngels, "tanker-angels", 13, "The tanker's altitude in thousands of feet when Tacview has no fix")
	f.IntVar(&flagTankerKIAS, "tanker-kias", 260, "The tanker's indicated speed until Tacview has its track (Texaco 1-1 measured 18 Oct)")
	f.StringVar(&flagTankerVoice, "tanker-voice", "echo", "OpenAI TTS voice for the tanker (the ElevenLabs fallback)")
	f.StringVar(&flagElevenTankerVoice, "elevenlabs-tanker-voice", elEric, "ElevenLabs voice ID for the tanker (default Eric)")

	f.Float64Var(&flagTanker2Freq, "tanker2-freq", 0,
		"With --command-only: run a second tanker net on this frequency in MHz, e.g. 252.3 (Texaco 3-1). 0=disabled")
	f.StringVar(&flagTanker2Name, "tanker2-name", "Texaco-3-1", "Second tanker SRS client name (no spaces)")
	f.StringVar(&flagTanker2Spoken, "tanker2-spoken", "Texaco three one", "Second tanker callsign as spoken on the radio")
	f.StringVar(&flagTanker2Tacview, "tanker2-tacview", "Texaco 3-1", "The second tanker's Tacview callsign, for its live altitude")
	f.IntVar(&flagTanker2Angels, "tanker2-angels", 24, "The second tanker's altitude in thousands of feet when Tacview has no fix")
	f.IntVar(&flagTanker2KIAS, "tanker2-kias", 265, "The second tanker's indicated speed until Tacview has its track (Mach 0.65 at angels 24)")
	f.StringVar(&flagTanker2Voice, "tanker2-voice", "verse", "OpenAI TTS voice for the second tanker (the ElevenLabs fallback)")
	f.StringVar(&flagElevenTanker2Voice, "elevenlabs-tanker2-voice", elBrian, "ElevenLabs voice ID for the second tanker (default Brian)")

	f.StringArrayVar(&flagTankerNets, "tanker-net", nil,
		"With --command-only: one more tanker net, FREQ/Callsign-N-N/angels/KIAS/basket|boom/voice, "+
			"e.g. 253.1/Shell-1-1/22/288/boom/charlie (repeatable)")
}

// parseTankerNet reads one --tanker-net: "253.1/Shell-1-1/22/288/boom/charlie".
func parseTankerNet(spec string) (tankerConfig, error) {
	p := strings.Split(spec, "/")
	if len(p) != 6 {
		return tankerConfig{}, fmt.Errorf("want FREQ/Callsign-N-N/angels/KIAS/basket|boom/voice, got %q", spec)
	}
	freq, err1 := strconv.ParseFloat(p[0], 64)
	angels, err2 := strconv.Atoi(p[2])
	kias, err3 := strconv.Atoi(p[3])
	if err1 != nil || err2 != nil || err3 != nil || freq <= 0 {
		return tankerConfig{}, fmt.Errorf("bad number in %q", spec)
	}
	if p[4] != "basket" && p[4] != "boom" {
		return tankerConfig{}, fmt.Errorf("%q: want basket or boom", p[4])
	}
	id, ok := tankerNetVoices[strings.ToLower(p[5])]
	if !ok {
		id = p[5] // a raw ElevenLabs voice ID
	}
	// "Shell-1-1" -> Tacview "Shell 1-1", spoken "Shell one one".
	tacview := strings.Replace(p[1], "-", " ", 1)
	words := strings.Fields(strings.ReplaceAll(p[1], "-", " "))
	for i, w := range words[1:] {
		if n, err := strconv.Atoi(w); err == nil {
			words[i+1] = spokenDigits(n)
		}
	}
	return tankerConfig{FreqMHz: freq, Name: p[1], Spoken: strings.Join(words, " "), Tacview: tacview,
		Angels: angels, KIAS: kias, Boom: p[4] == "boom",
		Voice: voiceProfile{voice: "ash", speed: flagTTSSpeed, eleven: elTower(id, tacview)}}, nil
}

// tankerConfig is one tanker net: Texaco 1-1 (--tanker-freq) and, when set,
// Texaco 3-1 (--tanker2-freq).
type tankerConfig struct {
	FreqMHz float64
	Name    string // SRS client name, "Texaco-1-1"
	Spoken  string // "Texaco one one"
	Tacview string // "Texaco 1-1"
	Angels  int
	KIAS    int
	Boom    bool // a KC-135 boom (the Shells), not two baskets
	Voice   voiceProfile
}

func tankerConfigs() []tankerConfig {
	var cfgs []tankerConfig
	if flagTankerFreq > 0 {
		cfgs = append(cfgs, tankerConfig{FreqMHz: flagTankerFreq, Name: flagTankerName, Spoken: flagTankerSpoken,
			Tacview: flagTankerTacview, Angels: flagTankerAngels, KIAS: flagTankerKIAS, Voice: tankerVoice()})
	}
	if flagTanker2Freq > 0 {
		cfgs = append(cfgs, tankerConfig{FreqMHz: flagTanker2Freq, Name: flagTanker2Name, Spoken: flagTanker2Spoken,
			Tacview: flagTanker2Tacview, Angels: flagTanker2Angels, KIAS: flagTanker2KIAS,
			Voice: voiceProfile{voice: flagTanker2Voice, speed: flagTTSSpeed, eleven: elTower(flagElevenTanker2Voice, "Texaco 3-1")}})
	}
	for _, spec := range flagTankerNets {
		cfg, err := parseTankerNet(spec)
		if err != nil {
			log.Error().Err(err).Msg("--tanker-net ignored")
			continue
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs
}

func tankerVoice() voiceProfile {
	return voiceProfile{voice: flagTankerVoice, speed: flagTTSSpeed, eleven: elTower(flagElevenTankerVoice, "Texaco")}
}

// Where a flight is on the tanker.
type tankerStage int

const (
	tankerCheckedIn   tankerStage = iota + 1 // cleared port observation, asked how much
	tankerGiveSet                            // amount copied
	tankerObservation                        // in trail / established port observation
	tankerBasket                             // cleared to a basket
	tankerReform                             // satisfied, starboard reform
)

type tankerFlight struct {
	stage  tankerStage
	basket string // "port" / "starboard" while the jet holds one
	at     time.Time
}

// tankerNet is one tanker's side of the net: the flights it is working and
// which basket each holds.
type tankerNet struct {
	Spoken string // "Texaco one one"
	Short  string // "Texaco", after the first call
	angels func() int
	kias   func() int
	boom   bool // one receiver on the boom, not port and starboard baskets

	mu      sync.Mutex
	flights map[string]*tankerFlight
	now     func() time.Time
	pick    func([3]string) string // which of a reply's versions; tests pin the first
}

// tankerFlightAge: a flight not heard from in this long starts again.
// tankerBasketAge: a basket held this long without a call is free again.
const (
	tankerFlightAge = 30 * time.Minute
	tankerBasketAge = 10 * time.Minute
)

func newTankerNet(spoken string, angels, kias func() int) *tankerNet {
	short := spoken
	if i := strings.IndexByte(spoken, ' '); i > 0 {
		short = spoken[:i]
	}
	return &tankerNet{Spoken: spoken, Short: short, angels: angels, kias: kias,
		flights: make(map[string]*tankerFlight), now: time.Now, pick: commandPick}
}

var (
	tankerCheckInRe     = regexp.MustCompile(`\b(initial|intent(ion)? to (refuel|tank|get gas)|request(ing)? (fuel|gas|tanking|to refuel|refuel|rejoin)|for (fuel|gas)|need (fuel|gas)|checking in|check in|inbound)\b`)
	tankerTrailRe       = regexp.MustCompile(`\b(mile trail|in trail|trail|one mile|1 mile|astern)\b`)
	tankerEstablishedRe = regexp.MustCompile(`\b(established|establish(ing)?|in position|stabilized|in place)\b`)
	tankerObservationRe = regexp.MustCompile(`\bobservation\b`)
	tankerBasketRe      = regexp.MustCompile(`\b(?:(port|starboard|left|right)\s+)?(basket|drogue|hose|boom)\b`)
	tankerMovingRe      = regexp.MustCompile(`\b(moving|move|request(ing)?|cleared?)\b`)
	tankerPreContactRe  = regexp.MustCompile(`\bpre-?\s?contact\b`)
	tankerReformRe      = regexp.MustCompile(`\b(reform|satisfied|complete|completed|disconnect(ed|ing)?|done)\b`)
	tankerCheckOutRe    = regexp.MustCompile(`\b(checking out|check out|resuming (navigation|mission|nav)|pushing|departing|leaving|clear(ing)? (of|the) tanker|moving on|on (to )?(the )?mission|proceeding (to )?(the )?mission|back to (the )?mission)\b`)
	tankerFullRe        = regexp.MustCompile(`\b(full|top(ped)? off|fill (it|her|us) up|max)\b`)
	tankerRadioCheckRe  = regexp.MustCompile(`\b((radio|comms?|com|comp)[\s-]?check|comcheck|how (do you )?copy|five by( five)?)\b`)
	// "request 5.0", "five point oh", "5000 pounds", "5 thousand".
	tankerAmountRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(thousand|k\b|pounds|lbs?)?`)
)

// tankerPointRe joins "5 . 0" / "5 .0" (from "five point oh") into "5.0".
var tankerPointRe = regexp.MustCompile(`(\d) ?\. ?(\d)`)

var tankerWordNums = map[string]string{
	"one": "1", "two": "2", "three": "3", "four": "4", "five": "5", "six": "6", "seven": "7",
	"eight": "8", "nine": "9", "ten": "10", "zero": "0", "oh": "0", "point": ".",
}

// tankerAmount is the give a pilot asked for, in pounds; 0 = none heard,
// -1 = full. "5.0" / "5 point 0" / "5 thousand" / "5,000 pounds" are 5000.
func tankerAmount(lower string) int {
	if tankerFullRe.MatchString(lower) {
		return -1
	}
	s := strings.ReplaceAll(lower, ",", "")
	words := strings.Fields(s)
	for i, w := range words {
		if d, ok := tankerWordNums[w]; ok {
			words[i] = d
		}
	}
	s = strings.Join(words, " ")
	s = tankerPointRe.ReplaceAllString(s, "$1.$2")
	// Skip the callsign's digits ("Raider 302") and "1 mile": take the number
	// after "request"/"give"/"want", or one with a unit, or a decimal.
	best := 0
	for _, m := range tankerAmountRe.FindAllStringSubmatchIndex(s, -1) {
		num := s[m[2]:m[3]]
		unit := ""
		if m[4] >= 0 {
			unit = s[m[4]:m[5]]
		}
		before := s[:m[0]]
		asked := strings.HasSuffix(strings.TrimSpace(before), "request") || strings.HasSuffix(strings.TrimSpace(before), "give") ||
			strings.HasSuffix(strings.TrimSpace(before), "want") || strings.HasSuffix(strings.TrimSpace(before), "need") ||
			strings.HasSuffix(strings.TrimSpace(before), "requesting")
		if !asked && unit == "" && !strings.Contains(num, ".") {
			continue
		}
		v, err := strconv.ParseFloat(num, 64)
		if err != nil || v <= 0 {
			continue
		}
		switch {
		case unit == "thousand" || unit == "k":
			v *= 1000
		case v < 30: // a state, in thousands
			v *= 1000
		}
		best = int(v + 0.5)
	}
	return best
}

// spokenThousands is a give as the tanker says it: "five thousand pounds",
// "two thousand five hundred pounds".
func spokenThousands(lbs int) string {
	if lbs < 0 {
		return "full"
	}
	lbs = (lbs + 50) / 100 * 100
	th, hu := lbs/1000, lbs%1000/100
	var parts []string
	if th > 0 {
		parts = append(parts, spokenInt(th)+" thousand")
	}
	if hu > 0 {
		parts = append(parts, spokenInt(hu)+" hundred")
	}
	return strings.Join(parts, " ") + " pounds"
}

var tankerOnes = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}

func spokenInt(n int) string {
	if n < 10 {
		return tankerOnes[n]
	}
	return map[int]string{10: "ten", 11: "eleven", 12: "twelve", 13: "thirteen", 14: "fourteen", 15: "fifteen",
		16: "sixteen", 17: "seventeen", 18: "eighteen", 19: "nineteen", 20: "twenty"}[n]
}

// spokenDigits reads a number digit by digit: 13 -> "one three", 250 -> "two five zero".
func spokenDigits(n int) string {
	s := strconv.Itoa(n)
	out := make([]string, len(s))
	for i, c := range s {
		out[i] = tankerOnes[c-'0']
	}
	return strings.Join(out, " ")
}

// basketHolder is the other flight holding a basket, "" if it is free.
func (n *tankerNet) basketHolder(side, cs string) string {
	for other, f := range n.flights {
		if other != cs && f.basket == side && n.now().Sub(f.at) < tankerBasketAge {
			return other
		}
	}
	return ""
}

// openBaskets: "Port and starboard baskets open." / "Starboard basket open." /
// "" when both are taken.
func (n *tankerNet) openBaskets(cs string) string {
	if n.boom {
		if n.basketHolder("boom", cs) == "" {
			return "Boom open."
		}
		return ""
	}
	port, stbd := n.basketHolder("port", cs) == "", n.basketHolder("starboard", cs) == ""
	switch {
	case port && stbd:
		return "Port and starboard baskets open."
	case port:
		return "Port basket open."
	case stbd:
		return "Starboard basket open."
	}
	return ""
}

// basketSide is the side a call names: "port" / "starboard", "" for none.
func basketSide(word string) string {
	switch word {
	case "port", "left":
		return "port"
	case "starboard", "right":
		return "starboard"
	}
	return ""
}

// onlyFlight is the one flight working the tanker, "" for none or several.
func (n *tankerNet) onlyFlight(now time.Time) string {
	only := ""
	for cs, f := range n.flights {
		if now.Sub(f.at) > tankerFlightAge {
			continue
		}
		if only != "" {
			return ""
		}
		only = cs
	}
	return only
}

var otherBasket = map[string]string{"port": "starboard", "starboard": "port"}

// Response is the tanker's reply to one call ("" = nothing to say).
func (n *tankerNet) Response(text, cs string) string {
	lower := strings.ToLower(text)
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.now()
	if cs == "" {
		// "Ready pre-contact." with no callsign (Day 2026-10-19 19:38): the
		// one jet on the tanker said it. A call that names the tanker and no
		// callsign still gets "say again your callsign".
		if strings.Contains(lower, strings.ToLower(n.Short)) || strings.Contains(lower, "tanker") {
			return fmt.Sprintf("Station calling %s, say again your callsign.", n.Spoken)
		}
		if cs = n.onlyFlight(now); cs == "" {
			return ""
		}
	}
	f := n.flights[cs]
	if f != nil && now.Sub(f.at) > tankerFlightAge {
		f = nil
	}
	if f == nil {
		f = &tankerFlight{}
	}
	defer func() {
		if f.stage == 0 {
			delete(n.flights, cs)
			return
		}
		f.at = now
		n.flights[cs] = f
	}()
	amount := tankerAmount(lower)
	checkIn := tankerCheckInRe.MatchString(lower)
	established := tankerEstablishedRe.MatchString(lower)
	preContact := tankerPreContactRe.MatchString(lower)
	basket := tankerBasketRe.FindStringSubmatch(lower)
	reform := strings.Contains(lower, "reform")

	if n.boom {
		if r, ok := n.boomResponse(lower, cs, f, basket != nil, preContact, established, reform); ok {
			return r
		}
	}

	switch {
	// "Texaco 1-1, Raider 302, comm check": loud and clear, and the flight
	// stays where it is. Day 2026-10-19 16:47 a ramp comm check got "cleared
	// port observation" (any first call was a check-in) and at 17:09, mid-flow,
	// two comm checks got nothing.
	case tankerRadioCheckRe.MatchString(lower) && !checkIn && amount == 0:
		return fmt.Sprintf("%s, %s, loud and clear.", cs, n.Spoken)

	// "checking out, resuming navigation, pushing 282"
	case tankerCheckOutRe.MatchString(lower) && !checkIn:
		f.stage, f.basket = 0, ""
		// Three send-offs (operator 2026-10-19: one of them "stay classy
		// Raider" - the callsign's name, so Venom hears "stay classy, Venom" -
		// one "fara í víking", Old Norse for "go a-viking", and #1 "Fortuna super
		// te, Viking", Latin "fortune be upon you", in place of "Good hunting").
		name := cs
		if i := strings.IndexByte(cs, ' '); i > 0 {
			name = cs[:i]
		}
		return n.pick([3]string{
			fmt.Sprintf("%s, %s, copy, cleared to depart. Fortuna super te, Viking.", cs, n.Short),
			fmt.Sprintf("%s, %s, cleared to depart. Stay classy, %s.", cs, n.Short, name),
			fmt.Sprintf("%s, %s, copy, cleared to depart. Fara í víking.", cs, n.Short),
		})

	// "satisfied, moving to starboard reform" / "established starboard reform"
	// "establish starboard side" after the reform clearance is the reform
	// established (Day 2026-10-19 19:40 was a miss).
	case reform || tankerReformRe.MatchString(lower) && f.stage > 0 ||
		established && strings.Contains(lower, "starboard") && f.stage >= tankerBasket && basket == nil:
		f.basket = ""
		if established {
			f.stage = tankerReform
			return fmt.Sprintf("%s, %s, copy.", cs, n.Short)
		}
		f.stage = tankerReform
		return fmt.Sprintf("%s, %s, cleared starboard reform.", cs, n.Short)

	// "moving to starboard basket" / "established starboard basket" / "ready pre-contact"
	case basket != nil || preContact:
		side := ""
		if basket != nil {
			side = basketSide(basket[1])
		}
		if side == "" {
			side = f.basket
		}
		if established || preContact {
			if side == "" {
				side = "port"
			}
			f.stage, f.basket = tankerBasket, side
			return fmt.Sprintf("%s, %s, copy, call ready pre-contact.", cs, n.Short)
		}
		if side == "" { // "moving to the basket": the open one
			side = "port"
			if n.basketHolder("port", cs) != "" {
				side = "starboard"
			}
		}
		if n.basketHolder(side, cs) != "" {
			if other := otherBasket[side]; n.basketHolder(other, cs) == "" {
				return fmt.Sprintf("%s, %s, standby, %s basket occupied. %s basket open.", cs, n.Short, side,
					strings.ToUpper(other[:1])+other[1:])
			}
			return fmt.Sprintf("%s, %s, standby, both baskets occupied. Hold port observation.", cs, n.Short)
		}
		f.stage, f.basket = tankerBasket, side
		return fmt.Sprintf("%s, %s, cleared %s basket.", cs, n.Short, side)

	// "1 mile trail for port observation"
	case tankerTrailRe.MatchString(lower) && !checkIn && !established && f.stage > 0:
		f.stage = tankerObservation
		return fmt.Sprintf("%s, %s, continue port observation.", cs, n.Short)

	// "established port observation"
	case established && (tankerObservationRe.MatchString(lower) || f.stage > 0 && f.stage < tankerBasket):
		f.stage = tankerObservation
		if open := n.openBaskets(cs); open != "" {
			return fmt.Sprintf("%s, %s, copy. %s", cs, n.Short, open)
		}
		return fmt.Sprintf("%s, %s, copy, both baskets occupied. Hold port observation.", cs, n.Short)

	// "request 5.0"
	case amount != 0 && f.stage > 0 && !checkIn:
		if f.stage < tankerGiveSet {
			f.stage = tankerGiveSet
		}
		if amount < 0 {
			return fmt.Sprintf("%s, %s, you are cleared for full give.", cs, n.Short)
		}
		return fmt.Sprintf("%s, %s, you are cleared for %s.", cs, n.Short, spokenThousands(amount))

	// "15 mile initial for port observation with intent to refuel"
	case checkIn || f.stage == 0:
		f.stage, f.basket = tankerCheckedIn, ""
		lead := fmt.Sprintf("%s, %s, cleared port observation, %s thousand, velocity %s.", cs, n.Spoken,
			spokenDigits(n.angels()), spokenDigits(n.kias()))
		switch { // "intent to refuel, request 5.0" in one call
		case amount < 0:
			f.stage = tankerGiveSet
			return lead + " You are cleared for full give."
		case amount > 0:
			f.stage = tankerGiveSet
			return lead + " You are cleared for " + spokenThousands(amount) + "."
		}
		return lead + " How much do you request?"
	}
	return ""
}

// boomResponse is a boom tanker's reply where it differs from the baskets'
// (false = answer as a basket tanker): one receiver at a time.
//
//	"Shell 1-1, Venom 1-1, moving to the boom"     -> "Venom 11, Shell, cleared to the boom."
//	"Venom 1-1, established pre-contact"           -> "Venom 11, Shell, copy, call ready pre-contact."
//	"Venom 1-2, established port observation" (boom taken) -> "... copy, boom occupied. Hold port observation."
func (n *tankerNet) boomResponse(lower, cs string, f *tankerFlight, boom, preContact, established, reform bool) (string, bool) {
	switch {
	case reform || tankerReformRe.MatchString(lower) && f.stage > 0:
		return "", false
	case boom || preContact:
		if established || preContact && !tankerMovingRe.MatchString(lower) {
			f.stage, f.basket = tankerBasket, "boom"
			return fmt.Sprintf("%s, %s, copy, call ready pre-contact.", cs, n.Short), true
		}
		if n.basketHolder("boom", cs) != "" {
			return fmt.Sprintf("%s, %s, standby, boom occupied. Hold port observation.", cs, n.Short), true
		}
		f.stage, f.basket = tankerBasket, "boom"
		return fmt.Sprintf("%s, %s, cleared to the boom.", cs, n.Short), true
	case established && (tankerObservationRe.MatchString(lower) || f.stage > 0 && f.stage < tankerBasket) &&
		n.basketHolder("boom", cs) != "":
		f.stage = tankerObservation
		return fmt.Sprintf("%s, %s, copy, boom occupied. Hold port observation.", cs, n.Short), true
	}
	return "", false
}

// tankerCallsign is the pilot's callsign in a call to the tanker, without
// the address: "Texaco 1-1, Raider 302, one Hornet, 15 miles" -> "Raider 302".
func tankerCallsign(text string) string {
	// "Texaco 3-1, one flight of F-18, Raider 302, ..." took "one" as the
	// callsign (Day 2026-10-19 19:35): drop the flight description first.
	text = tankerFlightDescRe.ReplaceAllString(text, "$1")
	cs := extractCallsignSkippingAddress(text, "texaco one-one", "texaco one one", "texaco 1-1", "texaco 1 1", "texaco 11",
		"texaco three-one", "texaco three one", "texaco 3-1", "texaco 3 1", "texaco 31",
		"texaco two-one", "texaco two one", "texaco 2-1", "texaco 2 1", "texaco 21",
		"shell one-one", "shell one one", "shell 1-1", "shell 1 1", "shell 11",
		"shell two-one", "shell two one", "shell 2-1", "shell 2 1", "shell 21",
		"texaco one", "texaco 1", "texaco three", "texaco 3", "texaco two", "texaco 2",
		"shell one", "shell 1", "shell two", "shell 2", "texaco", "shell", "tanker")
	cs = strings.TrimSpace(cs)
	if jtacBareNumberRe.MatchString(cs) {
		return ""
	}
	lower := strings.ToLower(cs)
	cut := len(cs)
	for _, v := range []string{",", " one ", " flight", " request", " intent", " checking", " established", " mile",
		" complete", " in ", " on ", " with ", " for ", " moving", " satisfied", " port", " starboard",
		" initial", " ready", " pushing", " 15 "} {
		if i := strings.Index(lower, v); i > 0 && i < cut {
			cut = i
		}
	}
	cs = joinCallsignDigits(strings.TrimSpace(cs[:cut]))
	if !strings.ContainsAny(cs, "0123456789") {
		return "" // "Ready pre-contact." is not a callsign
	}
	return cs
}

// tankerFlightDescRe: ", one flight of F-18," / ", flight of 2," / ", two Hornets,"
// ahead of the callsign.
var tankerFlightDescRe = regexp.MustCompile(`(?i)(,\s*)(?:(?:one|two|three|four|a|\d)\s+(?:flights?\s+of|ships?\s+of|hornets?\b|f-?1[68]s?\b|vipers?\b|jets?\b)|flights?\s+of)[^,]*,\s*`)

// startTankers runs every configured tanker net.
func startTankers(ctx context.Context, apiKey string, store *tacviewPositions) {
	for _, cfg := range tankerConfigs() {
		startTanker(ctx, apiKey, store, cfg)
	}
}

// startTanker runs one tanker net beside Command. store may be nil (no
// Tacview): the tanker then gives its briefed angels and speed.
func startTanker(ctx context.Context, apiKey string, store *tacviewPositions, cfg tankerConfig) {
	angels := func() int {
		if store != nil {
			if info := store.Info(cfg.Tacview); info.AltFt > 1000 && time.Since(info.UpdatedAt) < 2*time.Minute {
				return int(info.AltFt/1000 + 0.5)
			}
		}
		return cfg.Angels
	}
	var speed tankerSpeed
	kias := func() int {
		if v, ok := speed.KIAS(); ok {
			return v
		}
		return cfg.KIAS
	}
	if store != nil {
		go func() {
			tick := time.NewTicker(10 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
				}
				info := store.Info(cfg.Tacview)
				if pos, ok := store.Get(cfg.Tacview); ok && time.Since(info.UpdatedAt) < 30*time.Second {
					speed.Add(info.UpdatedAt, pos, info.AltFt)
				}
			}
		}()
	}
	tn := newTankerNet(cfg.Spoken, angels, kias)
	tn.boom = cfg.Boom
	position := "moving to starboard basket"
	if cfg.Boom {
		position = "Venom, moving to the boom, established pre-contact"
	}
	prompt := strings.Replace(cfg.Name, "-", " ", 1) + ", Knight, Raider, flight of two, 15 mile initial for port observation, " +
		"intent to refuel, request, thousand pounds, 1 mile trail, established port observation, " + position + ", " +
		"ready pre-contact, satisfied, moving to starboard reform, checking out, resuming navigation, pushing 282"
	go voiceNetLoop(ctx, flagSRSAddr, cfg.FreqMHz, cfg.Name, apiKey, flagEAMPassword, cfg.Voice, flagExternalAudio,
		prompt, nil, func(text string) (string, string) {
			cs := tankerCallsign(text)
			return cs, tn.Response(text, cs)
		})
	log.Info().Float64("freq", cfg.FreqMHz).Str("name", cfg.Name).Str("tacview", cfg.Tacview).
		Bool("store", store != nil).Msg("Tanker net starting")
}
