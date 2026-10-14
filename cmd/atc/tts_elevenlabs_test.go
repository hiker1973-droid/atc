package main

import (
	"context"
	"testing"
)

// The towers' ElevenLabs cast: British woman Alice, Australian woman Lily, men
// George. Charlie is kept for Hitman and never used by a tower.
func TestElevenTowerVoice(t *testing.T) {
	cases := []struct {
		accent string
		male   bool
		want   string
	}{
		{accentBritish, false, elAlice}, {accentBritish, true, elGeorge},
		{accentAustralian, true, elGeorge}, {accentAustralian, false, elLily},
	}
	for _, c := range cases {
		if got := elevenTowerVoice(c.accent, c.male).ID; got != c.want {
			t.Errorf("elevenTowerVoice(%s, male=%v) = %q, want %q", c.accent, c.male, got, c.want)
		}
		if elevenTowerVoice(c.accent, c.male).ID == elCharlie {
			t.Errorf("a tower uses Charlie, Hitman's voice")
		}
	}
}

// Without a key, or with --elevenlabs=false, nothing goes to ElevenLabs.
func TestElevenLabsNeedsAKey(t *testing.T) {
	saveOn, saveKey := flagElevenLabs, flagElevenLabsKey
	defer func() { flagElevenLabs, flagElevenLabsKey = saveOn, saveKey }()
	t.Setenv("ELEVENLABS_API_KEY", "")
	flagElevenLabs, flagElevenLabsKey = true, ""
	v := voiceProfile{voice: "onyx", eleven: elHard(elDaniel, "Command", 1.05)}
	if _, ok := speakElevenLabs(context.Background(), "radio check", v); ok {
		t.Error("spoke ElevenLabs with no key")
	}
	flagElevenLabs, flagElevenLabsKey = false, "k"
	if elevenKey() != "" {
		t.Error("--elevenlabs=false still returned a key")
	}
}

// An OpenAI fallback clip is cached apart from the ElevenLabs one, so once
// ElevenLabs is back the cache does not keep serving the fallback.
func TestTTSKeySeparatesElevenLabs(t *testing.T) {
	el := voiceProfile{voice: "onyx", speed: 1.2, instructions: "x", eleven: elHard(elDaniel, "Command", 1.05)}
	oa := el
	oa.eleven = elevenVoice{}
	if ttsKey(el, "radio check") == ttsKey(oa, "radio check") {
		t.Error("ElevenLabs and OpenAI clips share a cache key")
	}
}
