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
// Hitman nets, for the check-in and join. DCS still does the refuelling
// itself - the pilot calls "Ready pre-contact" from the comms menu. The flow
// is the operator's own (2026-10-16):
//
//	"Texaco 1-1, Raider 302, one Hornet, 15 miles, intent to refuel"
//	   -> "Raider 302, Texaco one one, angels one three, two five zero knots, how much do you request?"
//	"Raider 302, request 5.0"            -> "Raider 302, Texaco copies five thousand pounds."
//	"Raider 302, one mile trail"         -> "Raider 302, Texaco, form on my left."
//	"Raider 302, established"            -> "Raider 302, Texaco, copy, you are cleared port side, pre-contact."
//	"Raider 302, refuel complete"        -> "Raider 302, Texaco, cleared to depart. Good hunting."
//
// Texaco 1-1 in V1.31 Training (Day): KC-130, 252.100 AM (Hornet COMM2 CH8),
// TACAN 51Y TX1, angels 13 at ~250 KIAS, two wing drogues.

var (
	flagTankerFreq        float64
	flagTankerName        string
	flagTankerSpoken      string
	flagTankerTacview     string
	flagTankerAngels      int
	flagTankerKIAS        int
	flagTankerVoice       string
	flagElevenTankerVoice string
)

func registerTankerFlags(f *flag.FlagSet) {
	f.Float64Var(&flagTankerFreq, "tanker-freq", 0,
		"With --command-only: run a tanker check-in net on this frequency in MHz, e.g. 252.1 (Texaco 1-1). 0=disabled")
	f.StringVar(&flagTankerName, "tanker-name", "Texaco-1-1", "Tanker SRS client name (no spaces)")
	f.StringVar(&flagTankerSpoken, "tanker-spoken", "Texaco one one", "Tanker callsign as spoken on the radio")
	f.StringVar(&flagTankerTacview, "tanker-tacview", "Texaco 1-1", "The tanker's Tacview callsign, for its live altitude")
	f.IntVar(&flagTankerAngels, "tanker-angels", 13, "The tanker's altitude in thousands of feet when Tacview has no fix")
	f.IntVar(&flagTankerKIAS, "tanker-kias", 250, "The tanker's refuelling speed, knots indicated (Tacview gives no airspeed)")
	f.StringVar(&flagTankerVoice, "tanker-voice", "echo", "OpenAI TTS voice for the tanker (the ElevenLabs fallback)")
	f.StringVar(&flagElevenTankerVoice, "elevenlabs-tanker-voice", elEric, "ElevenLabs voice ID for the tanker (default Eric)")
}

func tankerVoice() voiceProfile {
	return voiceProfile{voice: flagTankerVoice, speed: flagTTSSpeed, eleven: elTower(flagElevenTankerVoice, "Texaco")}
}

// Where a flight is in the check-in.
type tankerStage int

const (
	tankerCheckedIn tankerStage = iota + 1 // asked how much
	tankerGiveSet                          // amount copied
	tankerJoining                          // told which side to form on
	tankerCleared                          // cleared pre-contact
)

type tankerFlight struct {
	stage tankerStage
	side  string // "left" / "right"
	at    time.Time
}

// tankerNet is one tanker's side of the net: the flights it is working and
// which drogue each has.
type tankerNet struct {
	Spoken string // "Texaco one one"
	Short  string // "Texaco", after the first call
	angels func() int
	kias   int

	mu      sync.Mutex
	flights map[string]*tankerFlight
	now     func() time.Time
}

// tankerFlightAge: a flight not heard from in this long starts again.
const tankerFlightAge = 30 * time.Minute

func newTankerNet(spoken string, angels func() int, kias int) *tankerNet {
	short := spoken
	if i := strings.IndexByte(spoken, ' '); i > 0 {
		short = spoken[:i]
	}
	return &tankerNet{Spoken: spoken, Short: short, angels: angels, kias: kias,
		flights: make(map[string]*tankerFlight), now: time.Now}
}

var (
	tankerCheckInRe  = regexp.MustCompile(`\b(intent(ion)? to (refuel|tank|get gas)|request(ing)? (fuel|gas|tanking|to refuel|refuel)|for (fuel|gas)|need (fuel|gas)|checking in|check in|inbound)\b`)
	tankerTrailRe    = regexp.MustCompile(`\b(mile trail|in trail|trail|one mile|1 mile|astern)\b`)
	tankerInPlaceRe  = regexp.MustCompile(`\b(established|in position|on (your|the) (left|right|port|starboard)|left wing|right wing|port wing|starboard wing|observation|ready pre-?contact|pre-?contact)\b`)
	tankerCompleteRe = regexp.MustCompile(`\b(complete|completed|disconnect(ed|ing)?|refuel(l)?ing complete|fuel complete|done|thanks|thank you|departing|clear(ing)? (of|the) tanker|leaving)\b`)
	tankerFullRe     = regexp.MustCompile(`\b(full|top(ped)? off|fill (it|her|us) up|max)\b`)
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

// sideFor gives a joining flight the drogue no other flight on the tanker
// holds: left unless left is taken.
func (n *tankerNet) sideFor(cs string) string {
	for other, f := range n.flights {
		if other != cs && f.stage >= tankerJoining && f.side == "left" && n.now().Sub(f.at) < 10*time.Minute {
			return "right"
		}
	}
	return "left"
}

// Response is the tanker's reply to one call ("" = nothing to say).
func (n *tankerNet) Response(text, cs string) string {
	lower := strings.ToLower(text)
	if cs == "" {
		if strings.Contains(lower, "texaco") || strings.Contains(lower, "tanker") {
			return fmt.Sprintf("Station calling %s, say again your callsign.", n.Spoken)
		}
		return ""
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.now()
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
	side := func() string {
		if f.side == "left" {
			return "port"
		}
		return "starboard"
	}
	switch {
	case tankerCompleteRe.MatchString(lower) && f.stage >= tankerJoining:
		f.stage = 0
		return fmt.Sprintf("%s, %s, copy, cleared to depart. Good hunting.", cs, n.Short)
	case tankerInPlaceRe.MatchString(lower) && f.stage >= tankerGiveSet:
		if f.side == "" {
			f.side = n.sideFor(cs)
		}
		f.stage = tankerCleared
		return fmt.Sprintf("%s, %s, copy, you are cleared %s side, pre-contact.", cs, n.Short, side())
	case tankerTrailRe.MatchString(lower) && f.stage >= tankerGiveSet:
		f.side = n.sideFor(cs)
		f.stage = tankerJoining
		return fmt.Sprintf("%s, %s, form on my %s.", cs, n.Short, f.side)
	case amount != 0 && f.stage >= tankerCheckedIn:
		f.stage = tankerGiveSet
		if amount < 0 {
			return fmt.Sprintf("%s, %s copies, full give.", cs, n.Short)
		}
		return fmt.Sprintf("%s, %s copies %s.", cs, n.Short, spokenThousands(amount))
	case tankerCheckInRe.MatchString(lower) || f.stage == 0:
		f.stage, f.side = tankerCheckedIn, ""
		if amount != 0 { // "intent to refuel, request 5.0" in one call
			f.stage = tankerGiveSet
			return fmt.Sprintf("%s, %s, angels %s, %s knots, copies %s.", cs, n.Spoken,
				spokenDigits(n.angels()), spokenDigits(n.kias), spokenThousands(amount))
		}
		return fmt.Sprintf("%s, %s, angels %s, %s knots, how much do you request?", cs, n.Spoken,
			spokenDigits(n.angels()), spokenDigits(n.kias))
	}
	return ""
}

// tankerCallsign is the pilot's callsign in a call to the tanker, without
// the address: "Texaco 1-1, Raider 302, one Hornet, 15 miles" -> "Raider 302".
func tankerCallsign(text string) string {
	cs := extractCallsignSkippingAddress(text, "texaco one-one", "texaco one one", "texaco 1-1", "texaco 1 1", "texaco 11",
		"texaco", "tanker")
	cs = strings.TrimSpace(cs)
	if jtacBareNumberRe.MatchString(cs) {
		return ""
	}
	lower := strings.ToLower(cs)
	cut := len(cs)
	for _, v := range []string{",", " one ", " flight", " request", " intent", " checking", " established", " mile",
		" complete", " in ", " on ", " with ", " for "} {
		if i := strings.Index(lower, v); i > 0 && i < cut {
			cut = i
		}
	}
	return joinCallsignDigits(strings.TrimSpace(cs[:cut]))
}

// startTanker runs the tanker net beside Command. store may be nil (no
// Tacview): the tanker then gives its briefed angels.
func startTanker(ctx context.Context, apiKey string, store *tacviewPositions) {
	angels := func() int {
		if store != nil {
			if info := store.Info(flagTankerTacview); info.AltFt > 1000 && time.Since(info.UpdatedAt) < 2*time.Minute {
				return int(info.AltFt/1000 + 0.5)
			}
		}
		return flagTankerAngels
	}
	tn := newTankerNet(flagTankerSpoken, angels, flagTankerKIAS)
	vp := tankerVoice()
	prompt := strings.Replace(flagTankerName, "-", " ", 1) + ", Raider, intent to refuel, request, thousand pounds, " +
		"one mile trail, established, left wing, right wing, pre-contact, refuel complete"
	go voiceNetLoop(ctx, flagSRSAddr, flagTankerFreq, flagTankerName, apiKey, flagEAMPassword, vp, flagExternalAudio,
		prompt, nil, func(text string) (string, string) {
			cs := tankerCallsign(text)
			return cs, tn.Response(text, cs)
		})
	log.Info().Float64("freq", flagTankerFreq).Str("name", flagTankerName).Str("tacview", flagTankerTacview).
		Bool("store", store != nil).Msg("Tanker net starting")
}
