package main

import (
	"strings"
	"testing"
)

// The levels are the ones measured off gpt-4o-mini-tts on 2026-10-10, when
// Command (sage) was reported far quieter than the towers.
func TestLevelGainLiftsQuietVoicesToTheTowerLevel(t *testing.T) {
	cases := []struct {
		name   string
		meanDB float64
		want   float64
	}{
		{"sage as Command", -34.5, 14.5},
		{"sage as Marshal", -36.3, 16.3},
		{"onyx", -24.8, 4.8},
		{"alloy tower is left alone", -20.5, 0.5},
		{"nova tower is left alone", -19.7, -0.3},
		{"near-silent clip is capped", -70, 24},
		{"hot clip is only trimmed", -5, -6},
	}
	for _, c := range cases {
		if got := levelGainDB(c.meanDB); got < c.want-0.05 || got > c.want+0.05 {
			t.Errorf("%s: levelGainDB(%.1f) = %.2f, want %.2f", c.name, c.meanDB, got, c.want)
		}
	}
}

func TestParseMeanVolume(t *testing.T) {
	out := "[Parsed_volumedetect_0 @ 000001] n_samples: 123456\n" +
		"[Parsed_volumedetect_0 @ 000001] mean_volume: -34.5 dB\n" +
		"[Parsed_volumedetect_0 @ 000001] max_volume: -17.9 dB\n"
	if got, ok := parseMeanVolumeDB(out); !ok || got != -34.5 {
		t.Errorf("parseMeanVolumeDB = %v, %v; want -34.5, true", got, ok)
	}
	if _, ok := parseMeanVolumeDB("Invalid data found when processing input"); ok {
		t.Error("output without a mean_volume line must not parse")
	}
}

// The radio graph must level first and raise the output after the mix: gain
// ahead of amix would lift the voice out of its tier's hiss, and gain after
// the limiter would clip.
func TestRadioFilterOrdersLevelGainAndLimiter(t *testing.T) {
	f := radioFilter("heavy", 14.5, 14)
	for _, want := range []string{
		"[0:a]volume=14.5dB,highpass=f=500,lowpass=f=2700,",
		"amplitude=0.128:",
		"amix=inputs=2:duration=first:dropout_transition=0,volume=14.0dB,alimiter=",
		"[click_up][body][click_dn]concat=n=3:v=0:a=1[out]",
	} {
		if !strings.Contains(f, want) {
			t.Errorf("radio filter is missing %q:\n%s", want, f)
		}
	}
}

func TestTxIntensityByCallsign(t *testing.T) {
	flagRadioIntensity, flagTowerRadioIntensity, flagATISRadioIntensity = "medium", "heavy", "light"
	for callsign, want := range map[string]string{
		"UGKS-ATC":       "heavy",
		"Senaki ATIS":    "light",
		"vSFG-7-Command": "medium",
		"OMDM-MSH":       "medium",
	} {
		if got := txIntensity(callsign); got != want {
			t.Errorf("txIntensity(%q) = %q, want %q", callsign, got, want)
		}
	}
}
