package main

import (
	"context"
	"regexp"
	"strings"

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
)

// registerGuardFlags is called from the flag block in main.go.
func registerGuardFlags(f *flag.FlagSet) {
	f.BoolVar(&flagGuardOnly, "guard-only", false, "Run as the Guard (243.0 AM) emergency watch only")
	f.Float64Var(&flagGuardFreq, "guard-freq", 243.0, "Guard frequency in MHz (AM)")
	f.StringVar(&flagGuardName, "guard-name", "Guard", "Guard's spoken name")
	f.StringVar(&flagGuardVoice, "guard-voice", "echo", "Guard OpenAI TTS voice (the fallback when ElevenLabs is off)")
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

// guardDivert is the nearest-field answer for a callsign.
func guardDivert(cs, name string, store *tacviewPositions) string {
	if store == nil {
		return divertReply(cs, name, orb.Point{}, false, divertFields())
	}
	pos, ok := store.Get(cs)
	return divertReply(cs, name, pos, ok, divertFields())
}

// runGuard is the --guard-only role: one voice net on Guard.
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
	log.Info().Float64("freq", flagGuardFreq).Str("name", flagGuardName).Str("map", flagMap).Msg("Guard watch starting")
	voiceNetLoop(ctx, flagSRSAddr, flagGuardFreq, flagGuardName, apiKey, flagEAMPassword, guardVoice(), flagExternalAudio,
		"Guard, Raider, Venom, Viper, mayday, pan pan, lost, request vectors, nearest field, radio check, hit, damaged, engine fire",
		nil,
		func(text string) (string, string) {
			cs, reply := guardReply(text, flagGuardName, store)
			if reply == "" {
				log.Info().Str("text", text).Msg("Guard: not for us - silent")
				return "", netHandled
			}
			log.Info().Str("text", text).Str("callsign", cs).Str("reply", reply).Msg("Guard reply")
			return cs, reply
		})
	return nil
}
