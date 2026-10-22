package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// Fixed lines kept on disk (operator 2026-10-22: "yes go ahead and cache them
// to disk"). Paddles renders its 19 fixed calls at every start - about 70
// ElevenLabs credits a rig - and the memory cache died with each restart or
// Update: Paddles was 28% of the servers' credits in the first billing period.
//
// synthesizeSpeechKept keeps an ElevenLabs clip beside atc.exe in tts_cache\,
// named by a hash of everything that shapes the sound (OpenAI voice, speed and
// style, and the whole ElevenLabs voice: ID, model, stability, style, speed,
// tempo) plus the text. A voice, delivery or wording change renders afresh; an
// old clip is never played in a new voice. OpenAI fallback clips stay in
// memory only, so a recovered ElevenLabs is not shadowed by one.
//
// Only for lines said word for word again and again: a callsign-led reply
// would fill the disk.

// ttsDiskDir is tts_cache beside atc.exe; "" (tests) keeps nothing on disk.
var ttsDiskDir = func() string {
	exe, err := os.Executable()
	if err != nil || strings.Contains(strings.ToLower(filepath.Base(exe)), ".test") {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "tts_cache")
}()

func ttsDiskPath(v voiceProfile, text string) string {
	if ttsDiskDir == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%+v", ttsKey(v, text), v.eleven)))
	return filepath.Join(ttsDiskDir, hex.EncodeToString(sum[:16])+".mp3")
}

// synthesizeSpeechKept is synthesizeSpeech for a fixed line: memory, then
// disk, then ElevenLabs (saved to disk), then the OpenAI fallback.
func synthesizeSpeechKept(ctx context.Context, apiKey, text string, v voiceProfile) ([]byte, error) {
	if elevenCapped() || v.eleven.ID == "" {
		return synthesizeSpeech(ctx, apiKey, text, v)
	}
	if mp3, ok := globalTTSCache.get(v, text); ok {
		return mp3, nil
	}
	path := ttsDiskPath(v, text)
	if path != "" {
		if mp3, err := os.ReadFile(path); err == nil && len(mp3) > 0 {
			globalTTSCache.set(v, text, mp3)
			log.Debug().Str("text", text).Msg("TTS disk cache hit")
			return mp3, nil
		}
	}
	mp3, ok := speakElevenLabs(ctx, text, v)
	if !ok {
		return synthesizeSpeech(ctx, apiKey, text, v)
	}
	globalTTSCache.set(v, text, mp3)
	if path != "" {
		if err := writeFileAtomic(path, mp3); err != nil {
			log.Warn().Err(err).Str("text", text).Msg("TTS disk cache write failed")
		}
	}
	return mp3, nil
}

// writeFileAtomic writes through a temp file, so a crash never leaves half a
// clip to be played later.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
