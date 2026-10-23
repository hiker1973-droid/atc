package main

import (
	"context"
	"strings"
	"testing"
)

// With the Windows engine the OpenAI step hands back the line, marked, and
// ExternalAudio gets it as text in the role's gender at about its speed.
func TestWindowsVoiceClip(t *testing.T) {
	t.Setenv("SKYEYE_TTS", "windows")
	b, err := synthesizeSpeechAPI(context.Background(), "", "Batumi Tower, runway one three.", voiceProfile{voice: "onyx", speed: 1.2})
	if err != nil || !isWinClip(b) {
		t.Fatalf("not a Windows clip: %q %v", b, err)
	}
	c, ok := readWinClip(b)
	if !ok || c.Text != "Batumi Tower, runway one three." || c.Gender != "male" || c.Speed != 3 {
		t.Fatalf("%+v", c)
	}
	args := strings.Join(winClipArgs(c), " ")
	if !strings.Contains(args, "--text Batumi Tower") || !strings.Contains(args, "-g male") || strings.Contains(args, "--file") {
		t.Fatalf("args %q", args)
	}
	if _, ok := readWinClip([]byte("ID3 real mp3 bytes")); ok {
		t.Fatal("mp3 read as a Windows clip")
	}
}

func TestTTSEngineSetting(t *testing.T) {
	t.Setenv("SKYEYE_TTS", "openai")
	if ttsEngine() != "openai" {
		t.Fatal("Settings switch to OpenAI ignored")
	}
	t.Setenv("SKYEYE_TTS", "Windows")
	if ttsEngine() != "windows" {
		t.Fatal("Settings switch to Windows ignored")
	}
}
