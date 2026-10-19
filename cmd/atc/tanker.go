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
//	   -> "Knight 11, Texaco one one, cleared port observation, one three thousand, velocity three three six. How much do you request?"
//	"Knight 1-1, request 5.0"                       -> "Knight 11, Texaco copies five thousand pounds."
//	"Knight 1-1, 1 mile trail for port observation" -> "Knight 11, Texaco, continue port observation."
//	"Knight 1-1, established port observation"      -> "Knight 11, Texaco, copy. Port and starboard baskets open."
//	"Knight 1-3, moving to starboard basket"        -> "Knight 13, Texaco, cleared starboard basket."
//	"Knight 1-3, established starboard basket"      -> "Knight 13, Texaco, copy, call ready pre-contact."
//	"Knight 1-3, satisfied, moving to starboard reform" -> "Knight 13, Texaco, cleared starboard reform."
//	"Knight 1-3, established starboard reform"      -> "Knight 13, Texaco, copy."
//	"Texaco 1, Knight 1-1, checking out, resuming navigation, pushing 282"
//	   -> "Knight 11, Texaco, copy, cleared to depart. Good hunting."
//
// Texaco 1-1 in V1.31 Training (Day): KC-130, 252.100 AM (Hornet COMM2 CH8),
// TACAN 51Y TX1, angels 13, two wing drogues. Its speed is the procedure's
// table: a KC-130 flies 336 at every altitude.

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
	f.IntVar(&flagTankerKIAS, "tanker-kias", 336, "The refuelling speed the tanker reports (vSFG-7 AAR table: KC-130 336 at every altitude)")
	f.StringVar(&flagTankerVoice, "tanker-voice", "echo", "OpenAI TTS voice for the tanker (the ElevenLabs fallback)")
	f.StringVar(&flagElevenTankerVoice, "elevenlabs-tanker-voice", elEric, "ElevenLabs voice ID for the tanker (default Eric)")
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
	kias   int

	mu      sync.Mutex
	flights map[string]*tankerFlight
	now     func() time.Time
}

// tankerFlightAge: a flight not heard from in this long starts again.
// tankerBasketAge: a basket held this long without a call is free again.
const (
	tankerFlightAge = 30 * time.Minute
	tankerBasketAge = 10 * time.Minute
)

func newTankerNet(spoken string, angels func() int, kias int) *tankerNet {
	short := spoken
	if i := strings.IndexByte(spoken, ' '); i > 0 {
		short = spoken[:i]
	}
	return &tankerNet{Spoken: spoken, Short: short, angels: angels, kias: kias,
		flights: make(map[string]*tankerFlight), now: time.Now}
}

var (
	tankerCheckInRe     = regexp.MustCompile(`\b(initial|intent(ion)? to (refuel|tank|get gas)|request(ing)? (fuel|gas|tanking|to refuel|refuel|rejoin)|for (fuel|gas)|need (fuel|gas)|checking in|check in|inbound)\b`)
	tankerTrailRe       = regexp.MustCompile(`\b(mile trail|in trail|trail|one mile|1 mile|astern)\b`)
	tankerEstablishedRe = regexp.MustCompile(`\b(established|in position|stabilized|in place)\b`)
	tankerObservationRe = regexp.MustCompile(`\bobservation\b`)
	tankerBasketRe      = regexp.MustCompile(`\b(?:(port|starboard|left|right)\s+)?(basket|drogue|hose)\b`)
	tankerPreContactRe  = regexp.MustCompile(`\bpre-?\s?contact\b`)
	tankerReformRe      = regexp.MustCompile(`\b(reform|satisfied|complete|completed|disconnect(ed|ing)?|done)\b`)
	tankerCheckOutRe    = regexp.MustCompile(`\b(checking out|check out|resuming (navigation|mission|nav)|pushing|departing|leaving|clear(ing)? (of|the) tanker)\b`)
	tankerFullRe        = regexp.MustCompile(`\b(full|top(ped)? off|fill (it|her|us) up|max)\b`)
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

var otherBasket = map[string]string{"port": "starboard", "starboard": "port"}

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
	checkIn := tankerCheckInRe.MatchString(lower)
	established := tankerEstablishedRe.MatchString(lower)
	preContact := tankerPreContactRe.MatchString(lower)
	basket := tankerBasketRe.FindStringSubmatch(lower)
	reform := strings.Contains(lower, "reform")

	switch {
	// "checking out, resuming navigation, pushing 282"
	case tankerCheckOutRe.MatchString(lower) && !checkIn:
		f.stage, f.basket = 0, ""
		return fmt.Sprintf("%s, %s, copy, cleared to depart. Good hunting.", cs, n.Short)

	// "satisfied, moving to starboard reform" / "established starboard reform"
	case reform || tankerReformRe.MatchString(lower) && f.stage > 0:
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
			return fmt.Sprintf("%s, %s copies, full give.", cs, n.Short)
		}
		return fmt.Sprintf("%s, %s copies %s.", cs, n.Short, spokenThousands(amount))

	// "15 mile initial for port observation with intent to refuel"
	case checkIn || f.stage == 0:
		f.stage, f.basket = tankerCheckedIn, ""
		lead := fmt.Sprintf("%s, %s, cleared port observation, %s thousand, velocity %s.", cs, n.Spoken,
			spokenDigits(n.angels()), spokenDigits(n.kias))
		switch { // "intent to refuel, request 5.0" in one call
		case amount < 0:
			f.stage = tankerGiveSet
			return lead + " Copies full give."
		case amount > 0:
			f.stage = tankerGiveSet
			return lead + " Copies " + spokenThousands(amount) + "."
		}
		return lead + " How much do you request?"
	}
	return ""
}

// tankerCallsign is the pilot's callsign in a call to the tanker, without
// the address: "Texaco 1-1, Raider 302, one Hornet, 15 miles" -> "Raider 302".
func tankerCallsign(text string) string {
	cs := extractCallsignSkippingAddress(text, "texaco one-one", "texaco one one", "texaco 1-1", "texaco 1 1", "texaco 11",
		"texaco one", "texaco 1", "texaco", "tanker")
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
	prompt := strings.Replace(flagTankerName, "-", " ", 1) + ", Knight, Raider, flight of two, 15 mile initial for port observation, " +
		"intent to refuel, request, thousand pounds, 1 mile trail, established port observation, moving to starboard basket, " +
		"ready pre-contact, satisfied, moving to starboard reform, checking out, resuming navigation, pushing 282"
	go voiceNetLoop(ctx, flagSRSAddr, flagTankerFreq, flagTankerName, apiKey, flagEAMPassword, vp, flagExternalAudio,
		prompt, nil, func(text string) (string, string) {
			cs := tankerCallsign(text)
			return cs, tn.Response(text, cs)
		})
	log.Info().Float64("freq", flagTankerFreq).Str("name", flagTankerName).Str("tacview", flagTankerTacview).
		Bool("store", store != nil).Msg("Tanker net starting")
}
