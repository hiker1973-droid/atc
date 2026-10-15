package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A successful ElevenLabs call writes one usage line with the credits from
// the "character-cost" header; a failed one writes nothing.
func TestElevenUsageRecorded(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("character-cost", "21")
		w.Write([]byte("mp3"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	defer func(d string, c *http.Client) { elevenUsageDir, elevenClient = d, c }(elevenUsageDir, elevenClient)
	elevenUsageDir = dir
	elevenClient = &http.Client{Transport: rewriteTo(srv.URL)}

	text := "Raider 302, Senaki Tower, cleared for takeoff."
	if _, err := synthesizeElevenLabs(context.Background(), "k", text, elCommand(elCommand11)); err != nil {
		t.Fatal(err)
	}
	synthesizeElevenLabs(context.Background(), "k", text, elCommand(elCommand11)) // 429: not recorded

	b, err := os.ReadFile(filepath.Join(dir, time.Now().UTC().Format("2006-01-02")+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 usage line, got %q", lines)
	}
	var u elevenUsage
	json.Unmarshal([]byte(lines[0]), &u)
	if u.Voice != "Command" || u.Credits != 21 || u.Chars != len(text) {
		t.Errorf("usage line %+v", u)
	}
}

type rewriteTo string

func (r rewriteTo) RoundTrip(req *http.Request) (*http.Response, error) {
	u := string(r) + req.URL.Path + "?" + req.URL.RawQuery
	nr, _ := http.NewRequestWithContext(req.Context(), req.Method, u, req.Body)
	nr.Header = req.Header
	return http.DefaultTransport.RoundTrip(nr)
}
