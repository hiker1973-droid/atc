package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestElevenCap(t *testing.T) {
	used := 120000
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != "k" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprintf(w, `{"character_count": %d, "character_limit": 127855}`, used)
	}))
	defer srv.Close()
	saveURL := elevenSubscriptionURL
	elevenSubscriptionURL = srv.URL
	defer func() {
		elevenSubscriptionURL = saveURL
		elevenCap.mu.Lock()
		elevenCap.capped = false
		elevenCap.mu.Unlock()
	}()

	ctx := context.Background()
	checkElevenCap(ctx, "k", 127000)
	if elevenCapped() {
		t.Fatal("capped at 120,000")
	}
	used = 127000
	checkElevenCap(ctx, "k", 127000)
	if !elevenCapped() {
		t.Fatal("not capped at 127,000")
	}
	// A failed read keeps the cap on.
	checkElevenCap(ctx, "bad", 127000)
	if !elevenCapped() {
		t.Fatal("a failed read lifted the cap")
	}
	// The new period: back to ElevenLabs.
	used = 300
	checkElevenCap(ctx, "k", 127000)
	if elevenCapped() {
		t.Fatal("still capped after the reset")
	}
}

// Capped, synthesis never reaches ElevenLabs - not even its cached clips.
func TestElevenCapSkipsElevenCache(t *testing.T) {
	v := voiceProfile{voice: "onyx", eleven: elevenVoice{ID: "x", Name: "Command"}}
	globalTTSCache.set(v, "cap test line", []byte("ELEVEN"))
	open := v
	open.eleven = elevenVoice{}
	globalTTSCache.set(open, "cap test line", []byte("OPENAI"))
	elevenCap.mu.Lock()
	elevenCap.capped = true
	elevenCap.mu.Unlock()
	defer func() { elevenCap.mu.Lock(); elevenCap.capped = false; elevenCap.mu.Unlock() }()
	got, err := synthesizeSpeech(context.Background(), "", "cap test line", v)
	if err != nil || string(got) != "OPENAI" {
		t.Fatalf("capped synthesis = %q, %v; want the OpenAI clip", got, err)
	}
}
