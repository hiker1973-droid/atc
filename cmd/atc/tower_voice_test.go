package main

import "testing"

// The field keeps the voice its start script gives it all day: Senaki's alloy
// (ElevenLabs Lisa) must not flip to the male slot on a 4-hour bucket.
func TestTowerVoiceIsTheScriptVoice(t *testing.T) {
	defer func(v, m string, h int) { flagTTSVoice, flagTTSVoiceMale, flagVoiceRotateHrs = v, m, h }(flagTTSVoice, flagTTSVoiceMale, flagVoiceRotateHrs)
	flagTTSVoice, flagTTSVoiceMale = "alloy", "onyx"
	flagVoiceRotateHrs = 0
	if got := currentTowerVoice(); got != "alloy" {
		t.Fatalf("tower voice %q, want alloy", got)
	}
}
