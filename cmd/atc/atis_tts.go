package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ATIS OpenAI calls are bounded, retried and length-checked (2026-10-14).
//
// Observed on vSFG7-Training3 the same day: gpt-4o-mini-tts intermittently
// held an ATIS request open for ~5 minutes and then returned 200 with a
// truncated body — Kobuleti's English came back 0.4s long, Senaki's 1s, a
// Russian pass 1.6s. The calls used http.DefaultClient with the process
// context, so nothing timed them out: the station went silent for the whole
// stall, then cached and rebroadcast the clipped report every cycle until the
// weather changed. It hit both the 373f2d1 and 5e97223 builds, so it is an
// API-side fault, not the accent instructions. The same requests normally
// finish in 4-15s.
const (
	atisTTSAttempts      = 3
	atisTTSTimeout       = 45 * time.Second
	atisTranslateTimeout = 30 * time.Second
	atisRetryPause       = 2 * time.Second

	// atisMaxWordsPerSec is the fastest a complete read can plausibly run.
	// Real reads at speed 1.20 come in around 2.5-3 words/s (Batumi: ~55
	// words in ~19s); the truncated ones were 40+ words/s. 6 leaves wide
	// headroom for a faster voice before a good read is ever rejected.
	atisMaxWordsPerSec = 6.0
)

// atisAudioTooShort reports whether sec of audio is too short to hold a full
// read of text.
func atisAudioTooShort(text string, sec float64) bool {
	words := len(strings.Fields(text))
	if words == 0 {
		return false
	}
	return sec < float64(words)/atisMaxWordsPerSec
}

var ffmpegTimeRe = regexp.MustCompile(`time=(\d+):(\d+):(\d+(?:\.\d+)?)`)

// mp3DurationSec decodes mp3 with ffmpeg and returns its length in seconds.
// It decodes rather than reading the header because a truncated stream still
// carries a plausible header.
func mp3DurationSec(ffmpegPath string, mp3 []byte) (float64, error) {
	tmp, err := os.CreateTemp("", "atc-atis-len-*.mp3")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(mp3); err != nil {
		tmp.Close()
		return 0, err
	}
	tmp.Close()

	// Stats stay on: the final "time=" progress line is the decoded length.
	out, err := exec.Command(ffmpegPath, "-hide_banner",
		"-i", tmp.Name(), "-f", "null", "-").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: %v: %s", err, string(out))
	}
	return parseFFmpegTime(string(out))
}

// parseFFmpegTime reads the last "time=HH:MM:SS.ss" ffmpeg printed.
func parseFFmpegTime(out string) (float64, error) {
	m := ffmpegTimeRe.FindAllStringSubmatch(out, -1)
	if len(m) == 0 {
		return 0, fmt.Errorf("no time= in ffmpeg output")
	}
	last := m[len(m)-1]
	h, _ := strconv.Atoi(last[1])
	mi, _ := strconv.Atoi(last[2])
	s, _ := strconv.ParseFloat(last[3], 64)
	return float64(h*3600+mi*60) + s, nil
}

// atisSynthesize is synthesizeSpeechAPI for ATIS: each attempt is bounded by
// atisTTSTimeout, and a reply too short for its text counts as a failure and
// is retried. If the length cannot be measured the audio is accepted — ffmpeg
// trouble must not take ATIS off the air.
func atisSynthesize(ctx context.Context, apiKey, text string, v voiceProfile, station, part string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= atisTTSAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(atisRetryPause):
			}
		}
		start := time.Now()
		actx, cancel := context.WithTimeout(ctx, atisTTSTimeout)
		mp3, err := synthesizeSpeechAPI(actx, apiKey, text, v)
		cancel()
		if err != nil {
			lastErr = err
			log.Warn().Err(err).Str("station", station).Str("part", part).Int("attempt", attempt).
				Dur("took", time.Since(start)).Msg("ATIS TTS attempt failed")
			continue
		}
		if isWinClip(mp3) { // Windows voice: text, not audio to measure
			return mp3, nil
		}
		sec, derr := mp3DurationSec(flagFFmpeg, mp3)
		if derr != nil {
			log.Warn().Err(derr).Str("station", station).Str("part", part).
				Msg("ATIS audio length unmeasurable — accepting it unchecked")
			return mp3, nil
		}
		if atisAudioTooShort(text, sec) {
			lastErr = fmt.Errorf("audio %.1fs is too short for %d words", sec, len(strings.Fields(text)))
			log.Warn().Str("station", station).Str("part", part).Int("attempt", attempt).
				Float64("sec", sec).Int("words", len(strings.Fields(text))).Dur("took", time.Since(start)).
				Msg("ATIS TTS returned truncated audio — retrying")
			continue
		}
		log.Info().Str("station", station).Str("part", part).Int("attempt", attempt).
			Float64("sec", sec).Dur("took", time.Since(start)).Msg("ATIS TTS ok")
		return mp3, nil
	}
	return nil, fmt.Errorf("%d attempts: %w", atisTTSAttempts, lastErr)
}

// atisTranslate is translateATIS bounded by atisTranslateTimeout per attempt.
func atisTranslate(ctx context.Context, apiKey, text, language, station string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= atisTTSAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(atisRetryPause):
			}
		}
		actx, cancel := context.WithTimeout(ctx, atisTranslateTimeout)
		out, err := translateATIS(actx, apiKey, text, language)
		cancel()
		if err == nil && out != "" {
			return out, nil
		}
		if err == nil {
			err = fmt.Errorf("empty translation")
		}
		lastErr = err
		log.Warn().Err(err).Str("station", station).Str("lang", language).Int("attempt", attempt).
			Msg("ATIS translate attempt failed")
	}
	return "", fmt.Errorf("%d attempts: %w", atisTTSAttempts, lastErr)
}
