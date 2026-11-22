package main

import (
	"context"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// brcSlowTempo: the spoken BRC digits are played 20% slower (operator
// 2026-11-22, "slow it down when they say BRC by 20%"). ElevenLabs' speed
// setting is not linear, so the BRC is rendered as its own clip and stretched
// on the audio (ffmpeg atempo, pitch kept), as Hitman's line 6 is.
const brcSlowTempo = 0.80

// brcCallRe is "BRC zero... nine... six" as brcSpeech writes it.
var brcCallRe = regexp.MustCompile(`(?i)\bBRC (?:zero|one|two|three|four|five|six|seven|eight|nine)(?:\.{3} ?(?:zero|one|two|three|four|five|six|seven|eight|nine))+`)

// splitBRC cuts text around its first spoken BRC; ok is false when it has none.
func splitBRC(text string) (pre, brc, post string, ok bool) {
	loc := brcCallRe.FindStringIndex(text)
	if loc == nil {
		return text, "", "", false
	}
	return strings.TrimSpace(text[:loc[0]]), text[loc[0]:loc[1]], strings.TrimSpace(text[loc[1]:]), true
}

// speakWithSlowBRC speaks text with vp, the BRC digits 20% slower. Any failure
// of the split path falls back to the whole line at the normal speed.
func speakWithSlowBRC(ctx context.Context, apiKey, text string, vp voiceProfile) ([]byte, error) {
	pre, brc, post, ok := splitBRC(text)
	if !ok {
		return synthesizeSpeech(ctx, apiKey, text, vp)
	}
	var out []byte
	for i, seg := range []string{pre, brc, post} {
		if seg == "" {
			continue
		}
		clip, err := synthesizeSpeech(ctx, apiKey, seg, vp)
		if err != nil {
			log.Warn().Err(err).Msg("BRC split TTS failed - speaking whole line at normal speed")
			return synthesizeSpeech(ctx, apiKey, text, vp)
		}
		if i == 1 {
			if slow, err := tempoMP3(clip, brcSlowTempo); err != nil {
				log.Warn().Err(err).Msg("BRC stretch failed - speaking it at the voice speed")
			} else {
				clip = slow
			}
		}
		if out == nil {
			out = clip
			continue
		}
		joined, err := concatMP3WithSilence(out, clip, flagFFmpeg, 0.08)
		if err != nil {
			log.Warn().Err(err).Msg("BRC join failed - speaking whole line at normal speed")
			return synthesizeSpeech(ctx, apiKey, text, vp)
		}
		out = joined
	}
	return out, nil
}
