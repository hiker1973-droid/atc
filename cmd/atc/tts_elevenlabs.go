package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ElevenLabs voices for the live controllers, operator 2026-10-14 after an A/B
// over SRS-processed clips: "I like the eleven labs all the best. lets roll with
// that. Keep ATIS the way it is." Command = Daniel, Hitman = Charlie, towers by
// the field's accent and gender (voice_accents.go). ATIS, Marshal and Deckboss
// stay on OpenAI.
//
// ElevenLabs is used only when ELEVENLABS_API_KEY (or --elevenlabs-key) is set
// on the rig. Any failure falls back to the role's OpenAI voice inside the same
// call, so a pilot hears an answer either way; a quota or auth failure turns
// ElevenLabs off for elevenBackoff so every call doesn't wait on it first.

// elevenVoice is one ElevenLabs voice and its delivery settings. ID "" = none.
type elevenVoice struct {
	ID        string
	Name      string // for logs
	Stability float64
	Style     float64
	Speed     float64
}

// Stock ElevenLabs voices (present on every account, free tier included).
const (
	elDaniel  = "onwK4e9ZLuTAKqWW03F9" // British man, "steady broadcaster"
	elGeorge  = "JBFqnCBsd6RMkjVDRZzb" // British man, older, warmer
	elAlice   = "Xb7hH8MSUJpSbSDYk0k2" // British woman, clear
	elLily    = "pFZP5JQG7iQjIQuC4Bku" // British woman, confident
	elCharlie = "IKne3meq5aSn9XLyUdCD" // Australian man
)

// The two deliveries the operator picked: the towers as heard in round 1, and
// the harder read used for Command and Hitman in rounds 2-3.
func elTower(id, name string) elevenVoice { return elevenVoice{id, name, 0.6, 0.2, 1.1} }
func elHard(id, name string, speed float64) elevenVoice {
	return elevenVoice{id, name, 0.25, 0.75, speed}
}

const (
	elevenBudget  = 6 * time.Second  // leaves room for the OpenAI fallback inside a 10 s tower ceiling
	elevenBackoff = 10 * time.Minute // after a quota / auth failure
)

var (
	elevenClient   = &http.Client{Timeout: 30 * time.Second}
	elevenMu       sync.Mutex
	elevenOffUntil time.Time
)

func elevenKey() string {
	if !flagElevenLabs {
		return ""
	}
	if flagElevenLabsKey != "" {
		return flagElevenLabsKey
	}
	return strings.TrimSpace(os.Getenv("ELEVENLABS_API_KEY"))
}

func elevenAvailable() bool {
	elevenMu.Lock()
	defer elevenMu.Unlock()
	return time.Now().After(elevenOffUntil)
}

func elevenBackOff(reason string) {
	elevenMu.Lock()
	elevenOffUntil = time.Now().Add(elevenBackoff)
	elevenMu.Unlock()
	log.Warn().Str("reason", reason).Dur("for", elevenBackoff).Msg("ElevenLabs off - OpenAI voices until it is retried")
}

// elevenStatusError is a non-200 from ElevenLabs.
type elevenStatusError struct {
	code int
	body string
}

func (e *elevenStatusError) Error() string {
	return fmt.Sprintf("ElevenLabs TTS error %d: %s", e.code, e.body)
}

// synthesizeElevenLabs returns MP3 for text in voice ev.
func synthesizeElevenLabs(ctx context.Context, key, text string, ev elevenVoice) ([]byte, error) {
	body := map[string]interface{}{
		"text":     text,
		"model_id": flagElevenLabsModel,
		"voice_settings": map[string]interface{}{
			"stability": ev.Stability, "similarity_boost": 0.85, "style": ev.Style, "speed": ev.Speed,
		},
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.elevenlabs.io/v1/text-to-speech/"+ev.ID+"?output_format=mp3_44100_128", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")
	resp, err := elevenClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		return nil, &elevenStatusError{resp.StatusCode, string(b)}
	}
	return io.ReadAll(resp.Body)
}

// speakElevenLabs tries the profile's ElevenLabs voice. ok=false means use
// OpenAI: no voice for this role, no key, backed off, or the call failed.
func speakElevenLabs(ctx context.Context, text string, v voiceProfile) ([]byte, bool) {
	key := elevenKey()
	if v.eleven.ID == "" || key == "" || !elevenAvailable() {
		return nil, false
	}
	budget := elevenBudget
	if dl, ok := ctx.Deadline(); ok {
		if rem := time.Until(dl) / 2; rem < budget {
			budget = rem
		}
	}
	elCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	start := time.Now()
	mp3, err := synthesizeElevenLabs(elCtx, key, text, v.eleven)
	if err != nil {
		if se, ok := err.(*elevenStatusError); ok && (se.code == 401 || se.code == 402 || se.code == 429) {
			elevenBackOff(se.Error())
		}
		log.Warn().Err(err).Str("voice", v.eleven.Name).Msg("ElevenLabs TTS failed - falling back to OpenAI")
		return nil, false
	}
	log.Debug().Str("voice", v.eleven.Name).Int64("ms", time.Since(start).Milliseconds()).Msg("ElevenLabs TTS")
	return mp3, true
}
