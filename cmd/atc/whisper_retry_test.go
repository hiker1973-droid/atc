package main

import (
	"context"
	"encoding/binary"
	"math"
	"testing"
)

// pcmWAV builds a 16 kHz mono 16-bit WAV, the shape convertToWAV writes.
func pcmWAV(samples []int16) []byte {
	data := make([]byte, 2*len(samples))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(data[2*i:], uint16(s))
	}
	b := []byte("RIFF\x00\x00\x00\x00WAVE")
	b = append(b, "fmt "...)
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 1)     // PCM
	b = binary.LittleEndian.AppendUint16(b, 1)     // mono
	b = binary.LittleEndian.AppendUint32(b, 16000) // rate
	b = binary.LittleEndian.AppendUint32(b, 32000) // byte rate
	b = binary.LittleEndian.AppendUint16(b, 2)     // block align
	b = binary.LittleEndian.AppendUint16(b, 16)    // bits
	b = append(b, "LIST"...)                       // ffmpeg adds one; it must be skipped
	b = binary.LittleEndian.AppendUint32(b, 4)
	b = append(b, "INFO"...)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(data)))
	b = append(b, data...)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	return b
}

func sine(amp float64, n int) []int16 {
	s := make([]int16, n)
	for i := range s {
		s[i] = int16(amp * math.Sin(2*math.Pi*440*float64(i)/16000))
	}
	return s
}

func TestMeasureWAV(t *testing.T) {
	// A full-scale sine is -3 dBFS RMS, 0 dBFS peak; a tenth of that, -23 / -20.
	for _, c := range []struct {
		amp       float64
		rms, peak float64
	}{
		{32767, -3.0, 0.0},
		{3276.7, -23.0, -20.0},
	} {
		lv := measureWAV(pcmWAV(sine(c.amp, 16000)))
		if !lv.ok || math.Abs(lv.rmsDB-c.rms) > 0.2 || math.Abs(lv.peakDB-c.peak) > 0.2 {
			t.Errorf("amp %v: got %+v, want rms %v peak %v", c.amp, lv, c.rms, c.peak)
		}
	}
	if lv := measureWAV(pcmWAV(make([]int16, 1600))); !lv.ok || lv.rmsDB != -120 {
		t.Errorf("silence: %+v", lv)
	}
	if lv := measureWAV([]byte("not a wav")); lv.ok {
		t.Errorf("garbage read as audio: %+v", lv)
	}
}

func TestUsableRetry(t *testing.T) {
	for text, want := range map[string]bool{
		"Bassel Tower, Venom 212, radio check.": true,
		"Thank you.":                            false,
		"you":                                   false,
		"Venom 212":                             false,
		"":                                      false,
	} {
		if got := usableRetry(text); got != want {
			t.Errorf("usableRetry(%q) = %v, want %v", text, got, want)
		}
	}
}

// Foothold 2026-10-21: Venom 212's calls to Bassel came back as the prompt.
func TestTranscribeWAVRetry(t *testing.T) {
	const prompt = "Bassel Al-Assad Tower, Raider, Venom, radio check, request taxi, holding short, runway, cleared for takeoff"
	echo := func(s string) bool { return isWhisperHallucination(s) || isPromptEcho(s, prompt) }
	speech := pcmWAV(sine(3000, 32000))
	silence := pcmWAV(make([]int16, 32000))
	defer func(f func(context.Context, string, []byte, string, string) (string, error)) { whisperCall = f }(whisperCall)

	for _, c := range []struct {
		name      string
		wav       []byte
		frames    int
		first, re string
		want      string
		wantRetry bool
	}{
		{"echo recovered", speech, 106, prompt, "Bassel Tower, Venom 212, radio check.", "Bassel Tower, Venom 212, radio check.", true},
		{"retry is filler", speech, 106, prompt, "Thank you.", prompt, true},
		{"retry echoes too", speech, 106, prompt, prompt, prompt, true},
		{"silent mic", silence, 106, prompt, "Bassel Tower, Venom 212, radio check.", prompt, false},
		{"blip", speech, 13, prompt, "Bassel Tower, Venom 212, radio check.", prompt, false},
		{"good first time", speech, 106, "Bassel Tower, Venom 212, request taxi.", "", "Bassel Tower, Venom 212, request taxi.", false},
	} {
		retried := false
		whisperCall = func(_ context.Context, _ string, _ []byte, _ string, p string) (string, error) {
			if p == "" {
				retried = true
				return c.re, nil
			}
			return c.first, nil
		}
		got, err := transcribeWAVRetry(context.Background(), "k", c.wav, c.frames, prompt, "Bassel Al-Assad Tower", echo)
		if err != nil || got != c.want || retried != c.wantRetry {
			t.Errorf("%s: got %q retried=%v err=%v, want %q retried=%v", c.name, got, retried, err, c.want, c.wantRetry)
		}
		if c.want == prompt && !echo(got) {
			t.Errorf("%s: a lost call must stay an echo so the caller drops it", c.name)
		}
	}
}
