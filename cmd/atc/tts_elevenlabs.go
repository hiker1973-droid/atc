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
// that. Keep ATIS the way it is." Command = round 11 voice 3 (Daniel, Gideon, then
// Spartan before it), Hitman = Scotty (Charlie at first), towers by
// the field's accent and gender (voice_accents.go). ATIS stays on OpenAI.
// Marshal (Adam) and Deckboss (Eric in the Caucasus, Brian elsewhere) joined
// the same day; Deckboss became Hank Harvey everywhere after round 9, and
// Marshal took Spartan on 2026-10-15 ("Lets use Spartan for Marshall"), when
// Deckboss became round 11 voice 5.
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
	elAdam    = "pNInz6obpgDQGcFmaJgB" // American man, dominant, firm (Marshal until 2026-10-15)
	elBrian   = "nPczCjzI2devNBz1zQrb" // American man, deep (Deckboss until round 9)
	elEric    = "cjVigY5qzO86Huf0OWal" // American man, smooth (Caucasus Deckboss until round 9)
)

// Library voices (shared by other ElevenLabs users; need a paid plan - the
// squadron is on Creator since 2026-10-14). An owner can withdraw a voice; the
// OpenAI fallback then answers until another is picked.
const (
	elScotty  = "7QoDxbTwBuOlgOCahTNI" // Australian man, 56, rough (Hitman)
	elLisa    = "bG3bxTbEPR7EZE4M0kcC" // Australian woman, assertive, calm (Senaki)
	elGround  = "X1WtgMmQUBwtiAPn4j1P" // Senaki Ground: round 13 senaki_6, operator 2026-10-16 "use this one"
	elGideon  = "q1h5HGdnfVxp4TXTJRNN" // English man, rugged, gravelly, Northern (Command until round 10)
	elSpartan = "S75jVZ0i3J6Xa5BbK8CE" // American man, calm, grounded baritone (Command, then Marshal, on 2026-10-15)
	elRolando = "6PBHdlpKVQ70t6tnDUYF" // American man, crisp, military (Marshal; operator 2026-10-15: "change marshall to this command_2_Rolando-crisp-military")
	// elCommand11 is voice 3 of round 11, the IDs the operator sent (pick 2026-10-15:
	// "command_3_XlDdozLmuTofIxK4BjPD"). Its name is not readable with the
	// rigs' key (no voices_read).
	elCommand11 = "XlDdozLmuTofIxK4BjPD" // "Misa" - Command until 2026-10-15
	// elDeckboss11 is voice 5 of the same round, picked for Deckboss the same
	// day ("command_5_ljX1ZrXuDIIRVcmiVSyR"); name likewise unreadable.
	elDeckboss11 = "ljX1ZrXuDIIRVcmiVSyR" // Deckboss
	// elDeparture11 is voice 4 of round 11, picked for the tower's "Departure"
	// 7-mile call (operator 2026-10-15: "use this for departure call
	// command_4_FF59babHL8N8gfTgtBMT").
	elDeparture11 = "FF59babHL8N8gfTgtBMT" // "Jodi" - Departure, and Command since 2026-10-15 (operator: "use this voice for command. good speed though"; back from Nora 2026-10-16: "go back to Jodi for Command")
	// Round 18 (2026-10-16, operator-sent library IDs): "Ok Nora for Command,
	// Lets do Hitman on 62.00 ... use command_5_9JbYHzFp74No6NSJoI0w".
	elNora     = "iV1OKYxbmgRtpzJ2q8kx" // "Nora Vale - Mission Control", mature woman, calm authority (Command c0fccab only, reverted same day)
	elHitman21 = "9JbYHzFp74No6NSJoI0w" // "George - Casual, Laid-Back", young American man (Hitman 2-1)
	// elBrad is round 10's "command_3_Brad-calm-commanding", picked 2026-10-16 for
	// Overlord Actual, who answers permission requests on Command's net.
	elBrad = "d7KZ4gJXiEWKJV0PMsGG" // "Brad", American man, calm, commanding (Overlord Actual)
	elHank = "KvaCHcJoMXfOzwtsULOZ" // American man, 38, gruff, rugged, calm (Deckboss until 2026-10-15)
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
// Speed 1.15 since 2026-10-15 (operator: "speed up command by 10%"; was 1.05,
// ElevenLabs takes 0.7-1.2).
func elCommand(id string) elevenVoice {
	return elevenVoice{ID: id, Name: "Command", Stability: 0.45, Style: 0.4, Speed: 1.15}
}

// The carrier roles as heard in round 6 (operator 2026-10-14: "Eric and Brian
// for deckboss and Adam for Marshall"; male is fine for both): Marshal calm and
// measured, Deckboss quicker and punchier.
func elMarshal(id string) elevenVoice {
	return elevenVoice{ID: id, Name: "Marshal", Stability: 0.6, Style: 0.2, Speed: 1.05, Model: elFlash}
}

// Deckboss alone is on the premium model at 1.05 since 2026-10-15 (operator:
// "deckboss speaks very fast ... slow down like 5% or change the updated model
// since we have overhead", picked the premium sample in round 17). Flash
// barely honoured the slower speed; the premium read came out ~7% longer.
// Deckboss is low traffic, so the doubled credits are tens a day.
func elDeckboss(id string) elevenVoice {
	return elevenVoice{ID: id, Name: "Deckboss", Stability: 0.35, Style: 0.5, Speed: 1.05, Model: "eleven_multilingual_v2"}
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
	mp3, err := io.ReadAll(resp.Body)
	if err == nil {
		recordElevenUsage(ev, text, resp.Header.Get("character-cost"))
	}
	return mp3, err
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
