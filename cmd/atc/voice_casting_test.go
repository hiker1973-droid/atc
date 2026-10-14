package main

import (
	"strings"
	"testing"
)

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

// Operator 2026-10-14: ATIS English is British or Australian like every other
// ATC voice; the second-language pass keeps the station's native speaker.
func TestATISEnglishAccents(t *testing.T) {
	save := flagVoiceStyleATIS
	defer func() { flagVoiceStyleATIS = save }()
	flagVoiceStyleATIS = styleATIS // the flag default; flags are not registered under go test
	for _, m := range []string{"syria", "caucasus", "pg", "germany", "iraq", "kola"} {
		for _, st := range atisStationsForMap(m) {
			got := atisEnglishVoice(st).instructions
			if st.Style == styleATISBritish {
				if got != styleATISBritish {
					t.Errorf("%s lost its RAF British style: %q", st.Name, got)
				}
				continue
			}
			if !strings.Contains(got, accentBritish) && !strings.Contains(got, accentAustralian) {
				t.Errorf("%s (%s) English read has no British/Australian accent: %q", st.Name, st.ICAO, got)
			}
			if st.Style != "" && atisVoiceFor(st).instructions != st.Style {
				t.Errorf("%s: second-language voice lost the station style", st.Name)
			}
		}
	}
}

// The tower accent is fixed per field and follows the voice's gender; a start
// script's own style (Akrotiri raf-british) wins.
func TestTowerAccentCasting(t *testing.T) {
	save := flagVoiceStyleTower
	defer func() { flagVoiceStyleTower = save }()
	flagVoiceStyleTower = styleTower
	if a, b := towerStyleFor("UGSB", "nova"), towerStyleFor("UGSB", "nova"); a != b {
		t.Error("accent not stable for a field")
	}
	if got := towerStyleFor("UGSB", "onyx"); !strings.Contains(got, "a man with") {
		t.Errorf("onyx tower not cast male: %q", got)
	}
	if got := towerStyleFor("UGSB", "nova"); !strings.Contains(got, "a woman with") {
		t.Errorf("nova tower not cast female: %q", got)
	}
	flagVoiceStyleTower = styleTowerBritish
	if got := towerStyleFor("LCRA", "nova"); got != styleTowerBritish {
		t.Errorf("explicit style overridden: %q", got)
	}
	if !strings.Contains(styleCommand, "a man with a crisp southern British") || !strings.Contains(styleCommand, "Stern") {
		t.Errorf("Command is not a stern English man: %q", styleCommand)
	}
	if !strings.Contains(styleJTAC, "Australian man") {
		t.Errorf("Hitman is not Australian: %q", styleJTAC)
	}
}
