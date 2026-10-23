package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/license"
)

// Windows voices (operator 2026-10-23: "make windows the deafult (unless they
// put in key)", then "Windows, OpenAI is upgrade"). On a SkyEye-Setup install
// the free Windows voices speak unless the Settings page's "Use OpenAI
// voices" is on; an ElevenLabs key still wins wherever a role has an
// ElevenLabs voice. The vSFG-7 rigs (git checkouts) keep OpenAI.
//
// SRS's ExternalAudio speaks text itself with the Windows voices (--text), so
// a Windows "clip" is not audio: it is the line and how to say it, marked so
// the transmit step hands it to ExternalAudio as text. It gets no radio
// effect, and ATIS reads English only (the Windows English voices cannot read
// the local language).

// ttsEngine is "windows" or "openai": SKYEYE_TTS (the Settings switch), else
// OpenAI on a source install and Windows on any other.
func ttsEngine() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SKYEYE_TTS"))) {
	case "windows":
		return "windows"
	case "openai":
		return "openai"
	}
	if license.IsSource(installDir()) {
		return "openai"
	}
	return "windows"
}

const winClipMagic = "SKYEYE-WINTTS1\n"

// winClip is a line for ExternalAudio to speak with a Windows voice.
type winClip struct {
	Text   string `json:"text"`
	Gender string `json:"gender"` // male / female, from the role's OpenAI voice
	Speed  int    `json:"speed"`  // ExternalAudio's -10..10, 1 = normal
}

func makeWinClip(text string, v voiceProfile) []byte {
	gender := "female"
	if voiceIsMale(v.voice) {
		gender = "male"
	}
	speed := v.speed
	if speed <= 0 {
		speed = speedDeckboss
	}
	// The roles' 1.2 is a touch brisk; ExternalAudio's scale is coarse.
	s := 1 + int((speed-1)*10+0.5)
	if s > 10 {
		s = 10
	}
	if s < -10 {
		s = -10
	}
	b, _ := json.Marshal(winClip{Text: text, Gender: gender, Speed: s})
	return append([]byte(winClipMagic), b...)
}

func readWinClip(b []byte) (winClip, bool) {
	if !bytes.HasPrefix(b, []byte(winClipMagic)) {
		return winClip{}, false
	}
	var c winClip
	if err := json.Unmarshal(b[len(winClipMagic):], &c); err != nil || c.Text == "" {
		return winClip{}, false
	}
	return c, true
}

// isWinClip: a Windows-voice line rather than audio.
func isWinClip(b []byte) bool { return bytes.HasPrefix(b, []byte(winClipMagic)) }

// winClipArgs are ExternalAudio's arguments for speaking a clip, in place of
// "--file <mp3>". US English: every Windows 10/11 has David and Zira.
func winClipArgs(c winClip) []string {
	return []string{"--text", c.Text, "-g", c.Gender, "-l", "en-US", "-s", fmt.Sprintf("%d", c.Speed)}
}

// transmitWinClip has ExternalAudio speak a Windows-voice line on a frequency.
func transmitWinClip(ctx context.Context, c winClip, freqMHz float64, callsign, srsPort, exePath string) {
	args := append(winClipArgs(c),
		"--freqs", fmt.Sprintf("%.3f", freqMHz),
		"--modulations", srsModulationName(freqMHz),
		"-c", srsCoalitionArg(),
		"-n", strings.ReplaceAll(callsign, " ", "-"),
		"-p", srsPort,
		"-v", "1",
	)
	if out, err := runExternalAudio(ctx, exePath, args); err != nil {
		log.Error().Err(err).Str("output", lastLine(string(out))).Str("callsign", callsign).Msg("ExternalAudio Windows-voice TX error")
	}
}
