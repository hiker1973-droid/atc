package main

import "testing"

// Operator 2026-09-19: a start script names a preset, never a raw sentence, so
// the preset must expand; anything else passes through untouched.
func TestResolveStylePresets(t *testing.T) {
	saveT, saveM := flagVoiceStyleTower, flagVoiceStyleMarshal
	defer func() { flagVoiceStyleTower, flagVoiceStyleMarshal = saveT, saveM }()

	flagVoiceStyleTower, flagVoiceStyleMarshal = "raf-british", "Speak plainly."
	resolveStylePresets()
	if flagVoiceStyleTower != styleTowerBritish {
		t.Errorf("raf-british did not expand: %q", flagVoiceStyleTower)
	}
	if flagVoiceStyleMarshal != "Speak plainly." {
		t.Errorf("literal instructions were changed: %q", flagVoiceStyleMarshal)
	}
}

// Every Syria ATIS reads English in its local accent (Akrotiri: British female).
func TestSyriaATISAccents(t *testing.T) {
	for _, st := range atisStationsForMap("syria") {
		if st.Style == "" {
			t.Errorf("%s (%s) has no accent style", st.Name, st.ICAO)
		}
		if got := atisVoiceFor(st).instructions; got != st.Style {
			t.Errorf("%s: voice instructions %q, want the station style", st.Name, got)
		}
	}
}
