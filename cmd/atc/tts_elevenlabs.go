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
// that. Keep ATIS the way it is." Command = Spartan (Daniel, then Gideon, at first), Hitman = Scotty (Charlie at first), towers by
// the field's accent and gender (voice_accents.go). ATIS stays on OpenAI.
// Marshal (Adam) and Deckboss (Eric in the Caucasus, Brian elsewhere) joined
// the same day; Deckboss became Hank Harvey everywhere after round 9.
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
	Model     string // "" = --elevenlabs-model
}

// Stock ElevenLabs voices (present on every account, free tier included).
const (
	elDaniel  = "onwK4e9ZLuTAKqWW03F9" // British man, "steady broadcaster"
	elGeorge  = "JBFqnCBsd6RMkjVDRZzb" // British man, older, warmer
	elAlice   = "Xb7hH8MSUJpSbSDYk0k2" // British woman, clear
	elLily    = "pFZP5JQG7iQjIQuC4Bku" // British woman, confident
	elCharlie = "IKne3meq5aSn9XLyUdCD" // Australian man
	elAdam    = "pNInz6obpgDQGcFmaJgB" // American man, dominant, firm (Marshal)
	elBrian   = "nPczCjzI2devNBz1zQrb" // American man, deep (Deckboss until round 9)
	elEric    = "cjVigY5qzO86Huf0OWal" // American man, smooth (Caucasus Deckboss until round 9)
)

// Library voices (shared by other ElevenLabs users; need a paid plan - the
// squadron is on Creator since 2026-10-14). An owner can withdraw a voice; the
// OpenAI fallback then answers until another is picked.
const (
	elScotty  = "7QoDxbTwBuOlgOCahTNI" // Australian man, 56, rough (Hitman)
	elLisa    = "bG3bxTbEPR7EZE4M0kcC" // Australian woman, assertive, calm (Senaki)
	elGideon  = "q1h5HGdnfVxp4TXTJRNN" // English man, rugged, gravelly, Northern (Command until round 10)
	elSpartan = "S75jVZ0i3J6Xa5BbK8CE" // American man, calm, grounded baritone (Command)
	elHank    = "KvaCHcJoMXfOzwtsULOZ" // American man, 38, gruff, rugged, calm (Deckboss)
)

// elFlash is ElevenLabs' cheaper model, about half the credits a character.
// Operator 2026-10-14: "Marshal flash is fine" (Marshal has the longest replies).
const elFlash = "eleven_flash_v2_5"

// The two deliveries the operator picked: the towers as heard in round 1, and
// the harder read used for Command and Hitman in rounds 2-3.
func elTower(id, name string) elevenVoice {
	return elevenVoice{ID: id, Name: name, Stability: 0.6, Style: 0.2, Speed: 1.1}
}
func elHard(id, name string, speed float64) elevenVoice {
	return elevenVoice{ID: id, Name: name, Stability: 0.25, Style: 0.75, Speed: speed}
}

// elCommand is Command's read since round 10 (operator 2026-10-14: "The command
// voice is not good" / "need american"): steadier than elHard, which made
// Gideon's gravel muddy on the radio.
func elCommand(id string) elevenVoice {
	return elevenVoice{ID: id, Name: "Command", Stability: 0.45, Style: 0.4, Speed: 1.05}
}

// The carrier roles as heard in round 6 (operator 2026-10-14: "Eric and Brian
// for deckboss and Adam for Marshall"; male is fine for both): Marshal calm and
// measured, Deckboss quicker and punchier.
func elMarshal(id string) elevenVoice {
	return elevenVoice{ID: id, Name: "Marshal", Stability: 0.6, Style: 0.2, Speed: 1.05, Model: elFlash}
}
func elDeckboss(id string) elevenVoice {
	return elevenVoice{ID: id, Name: "Deckboss", Stability: 0.35, Style: 0.5, Speed: 1.1}
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

// backOff reports whether the failure is the account's rather than this
// line's: no credits (402), rate limit (429), or a bad key - 401, and the 400s
// ElevenLabs gives a malformed key, e.g. "api_key_id_used_as_api_key" when the
// key's 64-hex ID was set instead of the sk_ key (seen on DCS3 2026-10-14).
// Without it a bad key made every transmission try ElevenLabs first. Other
// 400s (one voice withdrawn, a bad setting) stay per-call.
func (e *elevenStatusError) backOff() bool {
	switch e.code {
	case 401, 402, 429:
		return true
	case 400, 403:
		return strings.Contains(strings.ToLower(e.body), "api_key")
	}
	return false
}

func (e *elevenStatusError) Error() string {
	return fmt.Sprintf("ElevenLabs TTS error %d: %s", e.code, e.body)
}

func (ev elevenVoice) model() string {
	if ev.Model != "" {
		return ev.Model
	}
	return flagElevenLabsModel
}

// synthesizeElevenLabs returns MP3 for text in voice ev.
func synthesizeElevenLabs(ctx context.Context, key, text string, ev elevenVoice) ([]byte, error) {
	body := map[string]interface{}{
		"text":     text,
		"model_id": ev.model(),
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
		if se, ok := err.(*elevenStatusError); ok && se.backOff() {
			elevenBackOff(se.Error())
		}
		log.Warn().Err(err).Str("voice", v.eleven.Name).Msg("ElevenLabs TTS failed - falling back to OpenAI")
		return nil, false
	}
	log.Debug().Str("voice", v.eleven.Name).Int64("ms", time.Since(start).Milliseconds()).Msg("ElevenLabs TTS")
	return mp3, true
}
