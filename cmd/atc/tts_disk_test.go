package main

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// A clip on disk is played without asking ElevenLabs (the test has no key),
// and any change to the voice is a different file.
func TestTTSDiskCache(t *testing.T) {
	old := ttsDiskDir
	ttsDiskDir = t.TempDir()
	defer func() { ttsDiskDir = old }()

	v := voiceProfile{voice: "ash", speed: 1.0, eleven: elTower(elMatilda, "Paddles")}
	text := "Disk cache test, call the ball."
	clip := []byte("ID3 fake mp3")
	if err := writeFileAtomic(ttsDiskPath(v, text), clip); err != nil {
		t.Fatal(err)
	}
	got, err := synthesizeSpeechKept(context.Background(), "", text, v)
	if err != nil || !bytes.Equal(got, clip) {
		t.Fatalf("got %q, %v; want the clip on disk", got, err)
	}

	w := v
	w.eleven.Stability += 0.1
	if ttsDiskPath(w, text) == ttsDiskPath(v, text) {
		t.Error("a stability change kept the same file")
	}
	if ttsDiskPath(v, text+" ") == ttsDiskPath(v, text) {
		t.Error("a wording change kept the same file")
	}
	if _, err := os.Stat(ttsDiskPath(v, text) + ".tmp"); err == nil {
		t.Error("temp file left behind")
	}
}
