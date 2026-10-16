package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// The ElevenLabs credit cap: past --elevenlabs-stop-at credits used this
// billing period (the account's own count, every rig and every audition), every
// role speaks with OpenAI until the period resets. Operator 2026-10-16: "lets
// flip to open AI once we hit 127k. We should see that coming though" - the
// launcher's dashboard alerts give the warning (cmd/launcher/elevenusage.go).
//
// Cached ElevenLabs clips are skipped too while capped, so a controller keeps
// one voice instead of switching between cached and fresh lines.

// elevenCapEvery: how often each role reads the account. 27 roles across the
// fleet at one small GET each - nothing against the API's limits.
const elevenCapEvery = 10 * time.Minute

var elevenCap struct {
	mu          sync.Mutex
	capped      bool
	used, limit int
}

// elevenCapped reports whether the credit cap has switched ElevenLabs off.
func elevenCapped() bool {
	elevenCap.mu.Lock()
	defer elevenCap.mu.Unlock()
	return elevenCap.capped
}

// elevenSubscriptionURL is the account read; tests point it at a fake.
var elevenSubscriptionURL = "https://api.elevenlabs.io/v1/user/subscription"

// fetchElevenUsage reads this period's credits used and the plan's limit.
func fetchElevenUsage(ctx context.Context, key string) (used, limit int, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", elevenSubscriptionURL, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("xi-api-key", key)
	resp, err := elevenClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("subscription: HTTP %d", resp.StatusCode)
	}
	var sub struct {
		CharacterCount int `json:"character_count"`
		CharacterLimit int `json:"character_limit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return 0, 0, err
	}
	return sub.CharacterCount, sub.CharacterLimit, nil
}

// checkElevenCap reads the account once and switches the cap on or off. A
// failed read keeps the last state.
func checkElevenCap(ctx context.Context, key string, stopAt int) {
	used, limit, err := fetchElevenUsage(ctx, key)
	if err != nil {
		log.Debug().Err(err).Msg("ElevenLabs credit check failed - keeping the last state")
		return
	}
	elevenCap.mu.Lock()
	was := elevenCap.capped
	elevenCap.capped, elevenCap.used, elevenCap.limit = used >= stopAt, used, limit
	now := elevenCap.capped
	elevenCap.mu.Unlock()
	switch {
	case now && !was:
		log.Warn().Int("used", used).Int("stopAt", stopAt).Int("limit", limit).
			Msg("ElevenLabs credit cap reached - every role speaks with OpenAI until the billing period resets")
	case !now && was:
		log.Info().Int("used", used).Int("stopAt", stopAt).Msg("ElevenLabs credits below the cap again (new period) - ElevenLabs voices back")
	}
}

// runElevenCapWatch checks the cap at start (after a jitter so a rig's roles
// do not all ask at once) and every elevenCapEvery after.
func runElevenCapWatch(ctx context.Context) {
	if flagElevenStopAt <= 0 {
		return
	}
	key := elevenKey()
	if key == "" {
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Duration(rand.Intn(30)) * time.Second):
	}
	tk := time.NewTicker(elevenCapEvery)
	defer tk.Stop()
	for {
		checkElevenCap(ctx, key, flagElevenStopAt)
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}
	}
}
