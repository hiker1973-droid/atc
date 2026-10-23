package debrief

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Models the Reports page offers; the first is the default.
var Models = []struct{ ID, Label string }{
	{"claude-sonnet-5-5", "Claude Sonnet 5.5 (recommended)"},
	{"claude-opus-5-5", "Claude Opus 5.5 (most thorough, costs more)"},
	{"claude-haiku-5-5", "Claude Haiku 5.5 (fastest, cheapest)"},
}

// ValidModel reports whether id is one of Models.
func ValidModel(id string) bool {
	for _, m := range Models {
		if m.ID == id {
			return true
		}
	}
	return false
}

// API is the Anthropic API base (a var for tests).
var API = "https://api.anthropic.com"

var client = &http.Client{Timeout: 6 * time.Minute}

// Usage is what a debrief cost in tokens.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Write asks Claude for the debrief, with the customer's own key.
func Write(ctx context.Context, key, model, records string) (string, Usage, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 8000,
		"system":     System,
		"messages":   []map[string]any{{"role": "user", "content": records}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, API+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("could not reach Claude: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      Usage  `json:"usage"`
		Error      struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK {
		msg := out.Error.Message
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return "", Usage{}, fmt.Errorf("Claude rejected the key (%d): %s", resp.StatusCode, msg)
		default:
			return "", Usage{}, fmt.Errorf("Claude error %d: %s", resp.StatusCode, msg)
		}
	}
	var text strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	if out.StopReason == "max_tokens" {
		text.WriteString("\n\n*(The debrief was cut off at its length limit.)*")
	}
	return text.String(), out.Usage, nil
}

// CheckKey asks the API to list models: a cheap check that the key works.
func CheckKey(ctx context.Context, key string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, API+"/v1/models", nil)
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Claude")
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("Claude rejected the key (%d)", resp.StatusCode)
	default:
		return fmt.Errorf("Claude answered %d", resp.StatusCode)
	}
}
