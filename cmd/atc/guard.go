package main

import (
	"context"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"
)

// Guard Watch (operator 2026-11-15, "lets do Kola first"): a controller on the
// UHF emergency frequency, 243.0 AM. Nothing listened there before, so a pilot
// who called "Mayday, Guard" or went lost got silence.
//
// v1 answers three things and stays quiet otherwise (Guard is a shared,
// noisy channel):
//   - damage / mayday / pan-pan / "nearest field": the nearest friendly field
//     by magnetic bearing, range, elevation and tower frequency, from the
//     pilot's Tacview position (the same answer Command and Darkstar give,
//     emergency_divert.go), ending "declare your emergency";
//   - lost / disoriented / "request vectors": the same field, ending "say
//     intentions";
//   - a radio check addressed to Guard.
// Blue side only, one frequency. Warnings and intercepts are not built.

var (
	flagGuardOnly  bool
	flagGuardFreq  float64
	flagGuardName  string
	flagGuardVoice string
	flagGuardGrpc  string
	flagGuardWarn  bool
	flagGuardHail  bool
)

// registerGuardFlags is called from the flag block in main.go.
func registerGuardFlags(f *flag.FlagSet) {
	f.BoolVar(&flagGuardOnly, "guard-only", false, "Run as the Guard (243.0 AM) emergency watch only")
	f.Float64Var(&flagGuardFreq, "guard-freq", 243.0, "Guard frequency in MHz (AM)")
	f.StringVar(&flagGuardName, "guard-name", "Guard", "Guard's spoken name")
	f.StringVar(&flagGuardVoice, "guard-voice", "echo", "Guard OpenAI TTS voice (the fallback when ElevenLabs is off)")
	f.StringVar(&flagGuardGrpc, "guard-grpc-addr", "localhost:50051", "DCS-gRPC address Guard orders hailed AI aircraft through")
	f.BoolVar(&flagGuardWarn, "guard-warnings", true, "Guard calls SAM threats and unidentified aircraft unprompted")
	f.BoolVar(&flagGuardHail, "guard-hails", true, "Guard relays hails and orders to red AI aircraft")
}

const styleGuard = "Speak as a calm, steady military emergency-frequency controller on Guard: a man with a clear " +
	"British accent. Unhurried and reassuring, brisk and exact with numbers, no emphasis. " +
	"Run grouped numbers together as a single phrase rather than reading them one by one."

func guardVoice() voiceProfile {
	return voiceProfile{voice: flagGuardVoice, speed: flagTTSSpeed, instructions: styleGuard,
		eleven: elTower(elDaniel, "Guard")}
}

// guardLostRe: a pilot who does not know where he is.
var guardLostRe = regexp.MustCompile(`(?i)\b(?:i'?m|i am|we'?re|we are)\s+lost\b|\blost\s+(?:and|comms?|radio|nav|navigation|position)\b|\bdisoriented\b|\bunsure\s+of\s+(?:my\s+)?position\b|\brequest\s+(?:vectors?|bearing|steer|heading)\b|\bno\s+(?:nav|navigation|tacan)\b|\bnordo\b`)

// guardRadioCheckRe: "Guard, Raider 302, radio check".
var guardRadioCheckRe = regexp.MustCompile(`(?i)\b(?:radio\s*check|comm?\s*check|how\s+do\s+you\s+hear)\b`)

// guardReply is Guard's answer to one transcribed call ("" = stay silent).
// It returns the callsign it resolved for the log.
func guardReply(text, name string, store *tacviewPositions) (string, string) {
	lower := strings.ToLower(text)
	addressed := strings.Contains(lower, "guard") || strings.Contains(lower, "garde") // Whisper
	damaged := isDamagedCall(lower)
	lost := guardLostRe.MatchString(lower)
	if !addressed && !damaged && !lost {
		return "", ""
	}

	cs := guardCallsign(text, name, store)

	switch {
	case damaged || lost:
		if cs == "" {
			return "", "Aircraft calling Guard, say again your callsign and position."
		}
		resp := guardDivert(cs, name, store)
		if !damaged {
			resp = strings.Replace(resp, ", declare your emergency.", ", say intentions.", 1)
		}
		return cs, resp
	case addressed && guardRadioCheckRe.MatchString(lower) && cs != "":
		return cs, cs + ", " + name + ", loud and clear."
	}
	return cs, ""
}

// guardCallsign is the caller's callsign in a transcript ("" = none heard).
func guardCallsign(text, name string, store *tacviewPositions) string {
	cs := joinCallsignDigits(extractCallsignSkippingAddress(text, "guard", strings.ToLower(name)))
	var byNumber func(string) string
	if store != nil {
		byNumber = store.CallsignByNumber
		cs = undoubleCallsign(cs, func(c string) bool { _, ok := store.Get(c); return ok })
	}
	cs = commandCallsign(cs, store)
	if cs == "" {
		if who, ok := leadCallsign(text, byNumber); ok {
			cs = who
		}
	}
	return cs
}

// guardDivert is the nearest-field answer for a callsign.
func guardDivert(cs, name string, store *tacviewPositions) string {
	if store == nil {
		return divertReply(cs, name, orb.Point{}, false, divertFields())
	}
	pos, ok := store.Get(cs)
	return divertReply(cs, name, pos, ok, divertFields())
}

// guardSpeak transmits text on Guard in voice v as SRS client srsName.
func guardSpeak(ctx context.Context, apiKey string, freqMHz float64, text, srsName string, v voiceProfile) {
	mp3, err := synthesizeSpeech(ctx, apiKey, text, v)
	if err != nil {
		log.Error().Err(err).Str("text", text).Msg("Guard TTS failed")
		return
	}
	srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)
	noteOwnTX(freqMHz, text)
	transmitExternalAudioFile(ctx, mp3, freqMHz, srsName, srsHost, srsPort, flagExternalAudio)
}

// runGuard is the --guard-only role: one voice net on Guard, plus the warnings
// (guard_threats.go) and the hails (guard_hail.go).
func runGuard(ctx context.Context, apiKey string) error {
	if flagGuardFreq <= 0 {
		log.Error().Msg("Guard: --guard-freq must be set")
		return nil
	}
	var store *tacviewPositions
	if flagTacviewAddr != "" {
		store = newTacviewPositions()
		go runMiniTacview(ctx, flagTacviewAddr, store)
	} else {
		log.Warn().Msg("Guard: no --tacview-addr - callers will be asked for a position")
	}
	log.Info().Float64("freq", flagGuardFreq).Str("name", flagGuardName).Str("map", flagMap).
		Bool("warnings", flagGuardWarn).Bool("hails", flagGuardHail).Msg("Guard watch starting")

	if store != nil && flagGuardWarn {
		go runGuardWatch(ctx, store, newGuardWatch(flagGuardName), flagGuardFreq, func(text string) {
			guardSpeak(ctx, apiKey, flagGuardFreq, text, flagGuardName, guardVoice())
		})
	}
	var hail *guardHail
	if store != nil && flagGuardHail {
		ai := &grpcAI{addr: flagGuardGrpc}
		go ai.probe(ctx)
		hail = newGuardHail(store, ai)
	}

	voiceNetLoop(ctx, flagSRSAddr, flagGuardFreq, flagGuardName, apiKey, flagEAMPassword, guardVoice(), flagExternalAudio,
		"Guard, Raider, Venom, Viper, mayday, pan pan, lost, request vectors, nearest field, radio check, hit, damaged, engine fire, "+
			"unidentified aircraft, identify, bearing, turn left, turn right, heading, leave the area, follow me, land at Bodo",
		nil,
		func(text string) (string, string) {
			guardLastHeard.Store(time.Now().UnixNano())
			cs, reply := guardReply(text, flagGuardName, store)
			if reply != "" {
				log.Info().Str("text", text).Str("callsign", cs).Str("reply", reply).Msg("Guard reply")
				return cs, reply
			}
			if hail != nil {
				if cs == "" {
					cs = guardCallsign(text, flagGuardName, store)
				}
				if r := hail.handle(text, cs, time.Now()); r.handled() {
					log.Info().Str("text", text).Str("callsign", cs).Str("guard", r.Guard).Str("aircraft", r.AI).Msg("Guard hail")
					go runGuardResult(ctx, apiKey, r)
					return cs, netHandled
				}
			}
			log.Info().Str("text", text).Msg("Guard: not for us - silent")
			return "", netHandled
		})
	return nil
}

// runGuardResult speaks a hail result: Guard's line, or the aircraft's reply
// after its order has been carried out.
func runGuardResult(ctx context.Context, apiKey string, r guardResult) {
	if r.Guard != "" {
		guardSpeak(ctx, apiKey, flagGuardFreq, r.Guard, flagGuardName, guardVoice())
	}
	if r.AI == "" {
		return
	}
	text := r.AI
	if r.Do != nil {
		if err := r.Do(ctx); err != nil {
			log.Warn().Err(err).Msg("Guard: order to AI aircraft failed")
			text = r.Fail
		}
	}
	// A beat, as a pilot would take before answering.
	time.Sleep(1500 * time.Millisecond)
	guardSpeak(ctx, apiKey, flagGuardFreq, text, "Russian Air Force", aiVoice())
}
