package main

import (
	"context"
	"encoding/binary"
	"math"
	"strings"

	"github.com/rs/zerolog/log"
)

// Whisper prompt-echo retry and call loudness. Foothold 2026-10-21: all 15
// calls Venom 212 made to Bassel Al-Assad Tower came back from Whisper as the
// prompt itself, so the tower dropped every one and said nothing. An echo
// means Whisper found nothing in the audio it preferred over the prompt - a
// quiet, clipped or noisy call can still carry words the prompt drowned out.
//
// The call is now sent once more with no prompt, and its loudness goes in the
// log, so a silent or muted mic (nothing to retry) shows apart from a real
// call the prompt swallowed.

// silentRMSdB: a call quieter than this is treated as silence and not retried.
// Speech through SRS sits around -30 to -15 dBFS RMS.
const silentRMSdB = -50.0

// whisperCall is whisperTranscribeWithPrompt; tests swap it out.
var whisperCall = whisperTranscribeWithPrompt

type wavLevels struct {
	rmsDB, peakDB float64
	ok            bool
}

// measureWAV reads the loudness of a 16-bit PCM WAV, as convertToWAV writes
// it, in dB full scale. ok is false when the data is not 16-bit PCM.
func measureWAV(wav []byte) wavLevels {
	if len(wav) < 12 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return wavLevels{}
	}
	bits := 0
	for off := 12; off+8 <= len(wav); {
		id := string(wav[off : off+4])
		size := int(binary.LittleEndian.Uint32(wav[off+4 : off+8]))
		body := off + 8
		if size < 0 || body+size > len(wav) {
			size = len(wav) - body
		}
		switch id {
		case "fmt ":
			if size >= 16 {
				bits = int(binary.LittleEndian.Uint16(wav[body+14 : body+16]))
			}
		case "data":
			if bits != 16 || size < 2 {
				return wavLevels{}
			}
			var sum, peak float64
			n := size / 2
			for i := 0; i < n; i++ {
				v := float64(int16(binary.LittleEndian.Uint16(wav[body+2*i:])))
				sum += v * v
				if a := math.Abs(v); a > peak {
					peak = a
				}
			}
			return wavLevels{rmsDB: dbfs(math.Sqrt(sum / float64(n))), peakDB: dbfs(peak), ok: true}
		}
		off = body + size + size%2
	}
	return wavLevels{}
}

func dbfs(v float64) float64 {
	if v < 1 {
		return -120
	}
	return math.Round(200*math.Log10(v/32768)) / 10
}

// usableRetry: a no-prompt answer worth acting on. Whisper fills silence and
// noise with stock lines, and a word or two is not a radio call.
func usableRetry(text string) bool {
	lower := strings.Trim(strings.ToLower(strings.TrimSpace(text)), ".!? ")
	switch lower {
	case "", "you", "thank you", "thanks for watching", "thank you for watching", "bye", "okay":
		return false
	}
	return len(strings.Fields(lower)) >= 3
}

// transcribeWAVRetry sends a call to Whisper with prompt. When the answer is
// the prompt echoed back (echo says so), it logs the call's loudness and, for
// a call long enough to be speech and louder than silence, asks once more
// with no prompt. The echo text comes back unchanged when the call stays
// lost, so the caller's own hallucination filter still drops it.
func transcribeWAVRetry(ctx context.Context, apiKey string, wav []byte, frames int, prompt, role string, echo func(string) bool) (string, error) {
	text, err := whisperCall(ctx, apiKey, wav, "audio.wav", prompt)
	if err != nil || text == "" || !echo(text) {
		return text, err
	}
	lv := measureWAV(wav)
	ev := log.Warn().Str("role", role).Int("frames", frames).Bool("levelRead", lv.ok).
		Float64("rmsDb", lv.rmsDB).Float64("peakDb", lv.peakDB)
	switch {
	case frames < lostCallMinFrames:
		ev.Str("why", "under a second - a blip").Msg("Whisper prompt echo - not retried")
		return text, nil
	case lv.ok && lv.rmsDB < silentRMSdB:
		ev.Str("why", "call is silence - check the pilot's SRS mic").Msg("Whisper prompt echo - not retried")
		return text, nil
	}
	retry, rerr := whisperCall(ctx, apiKey, wav, "audio.wav", "")
	if rerr != nil || !usableRetry(retry) || echo(retry) {
		ev.AnErr("retryErr", rerr).Str("retry", retry).Msg("Whisper prompt echo - retry without prompt also lost")
		return text, nil
	}
	ev.Str("retry", retry).Msg("Whisper prompt echo - recovered by retry without prompt")
	return retry, nil
}

// transcribeFramesRetry is transcribeWAVRetry for raw SRS Opus frames.
func transcribeFramesRetry(ctx context.Context, apiKey string, frames [][]byte, prompt, role string, echo func(string) bool) (string, error) {
	wav, err := convertToWAV(flagFFmpeg, wrapOpusInOGG(frames))
	if err != nil {
		return "", err
	}
	return transcribeWAVRetry(ctx, apiKey, wav, len(frames), prompt, role, echo)
}
