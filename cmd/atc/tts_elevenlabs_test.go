package main

import (
	"context"
	"testing"
)

// The towers' ElevenLabs cast: British woman Alice, Australian woman Lisa, men
// George. Hitman's voice is never used by a tower.
func TestElevenTowerVoice(t *testing.T) {
	cases := []struct {
		accent string
		male   bool
		want   string
	}{
		{accentBritish, false, elAlice}, {accentBritish, true, elGeorge},
		{accentAustralian, true, elGeorge}, {accentAustralian, false, elLisa},
	}
	for _, c := range cases {
		if got := elevenTowerVoice(c.accent, c.male).ID; got != c.want {
			t.Errorf("elevenTowerVoice(%s, male=%v) = %q, want %q", c.accent, c.male, got, c.want)
		}
		if id := elevenTowerVoice(c.accent, c.male).ID; id == elScotty || id == elCharlie {
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

// Marshal and Deckboss speak ElevenLabs too (Adam; Brian, or Eric in the
// Caucasus via its start script); ATIS does not.
func TestCarrierRolesUseElevenLabs(t *testing.T) {
	saveM, saveD := flagElevenMarshalVoice, flagElevenDeckbossVoice
	defer func() { flagElevenMarshalVoice, flagElevenDeckbossVoice = saveM, saveD }()
	flagElevenMarshalVoice, flagElevenDeckbossVoice = elAdam, elBrian
	if got := marshalVoice("echo").eleven.ID; got != elAdam {
		t.Errorf("Marshal ElevenLabs voice = %q, want Adam", got)
	}
	if got := deckbossVoice("verse").eleven.ID; got != elBrian {
		t.Errorf("Deckboss ElevenLabs voice = %q, want Brian", got)
	}
	if got := atisVoice("nova").eleven.ID; got != "" {
		t.Errorf("ATIS got an ElevenLabs voice %q, it stays on OpenAI", got)
	}
	if !voiceIsMale("echo") || !voiceIsMale("verse") {
		t.Error("the carrier fallbacks must be male voices")
	}
}

// Marshal reads its long replies on the cheaper Flash model; the others use
// the default model unless set.
func TestMarshalOnFlash(t *testing.T) {
	if got := marshalVoice("echo").eleven.model(); got != elFlash {
		t.Errorf("Marshal model = %q, want %q", got, elFlash)
	}
	save := flagElevenLabsModel
	defer func() { flagElevenLabsModel = save }()
	flagElevenLabsModel = "eleven_multilingual_v2"
	if got := deckbossVoice("verse").eleven.model(); got != "eleven_multilingual_v2" {
		t.Errorf("Deckboss model = %q, want the default", got)
	}
	el := voiceProfile{voice: "echo", eleven: elMarshal(elAdam)}
	other := el
	other.eleven.Model = "eleven_multilingual_v2"
	if ttsKey(el, "x") == ttsKey(other, "x") {
		t.Error("two models share a cache key")
	}
}
