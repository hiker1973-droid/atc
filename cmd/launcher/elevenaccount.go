package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// The ElevenLabs account's own figures, when the launcher's key may read them
// (operator 2026-10-15: "i want you to be able to read into eleven labs and see
// our credit burn" - a restricted key with User: Read and Workspace analytics:
// Read). They replace the local count's total, plan size, reset date and
// by-voice / by-day breakdown, and include everything on the account (voice
// auditions too), not only SkyEye's calls. Without the key or the permissions
// the dashboard keeps the local count.

const elevenAPI = "https://api.elevenlabs.io"

// elevenAccount is what the account endpoints return, trimmed.
type elevenAccount struct {
	Tier    string         `json:"tier"`
	Used    int            `json:"used"`
	Limit   int            `json:"limit"`
	Reset   time.Time      `json:"reset"`
	Start   time.Time      `json:"start"`
	ByVoice map[string]int `json:"byVoice,omitempty"`
	ByDay   map[string]int `json:"byDay,omitempty"`
	Err     string         `json:"err,omitempty"` // breakdown failed; totals may still be good
}

var elevenAccountClient = &http.Client{Timeout: 8 * time.Second}

func elevenAccountKey() string { return strings.TrimSpace(os.Getenv("ELEVENLABS_API_KEY")) }

func elevenGet(path string, v any) error {
	req, _ := http.NewRequest("GET", elevenAPI+path, nil)
	req.Header.Set("xi-api-key", elevenAccountKey())
	resp, err := elevenAccountClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// fetchElevenAccount reads the subscription (used, limit, next reset) and the
// period's usage by voice per day. nil when there is no key or the
// subscription cannot be read.
func fetchElevenAccount(now time.Time) (*elevenAccount, error) {
	if elevenAccountKey() == "" {
		return nil, fmt.Errorf("no ELEVENLABS_API_KEY in the launcher's environment")
	}
	var sub struct {
		Tier      string `json:"tier"`
		Used      int    `json:"character_count"`
		Limit     int    `json:"character_limit"`
		ResetUnix int64  `json:"next_character_count_reset_unix"`
	}
	if err := elevenGet("/v1/user/subscription", &sub); err != nil {
		return nil, err
	}
	a := &elevenAccount{Tier: sub.Tier, Used: sub.Used, Limit: sub.Limit,
		Reset: time.Unix(sub.ResetUnix, 0).UTC()}
	a.Start = a.Reset.AddDate(0, -1, 0)
	if a.Start.After(now) {
		a.Start = now.AddDate(0, -1, 0)
	}
	var stats struct {
		Time  []int64              `json:"time"`
		Usage map[string][]float64 `json:"usage"`
	}
	q := fmt.Sprintf("/v1/usage/character-stats?start_unix=%d&end_unix=%d&breakdown_type=voice&aggregation_interval=day",
		a.Start.UnixMilli(), now.UnixMilli())
	if err := elevenGet(q, &stats); err != nil {
		a.Err = "usage breakdown: " + err.Error()
		return a, nil
	}
	a.ByVoice, a.ByDay = map[string]int{}, map[string]int{}
	for voice, series := range stats.Usage {
		name := shortVoiceName(voice)
		for i, n := range series {
			if n == 0 || i >= len(stats.Time) {
				continue
			}
			a.ByVoice[name] += int(n)
			a.ByDay[time.UnixMilli(stats.Time[i]).UTC().Format("2006-01-02")] += int(n)
		}
	}
	return a, nil
}

// shortVoiceName trims a library voice's tagline: "Lisa - Assertive Australian
// Woman - Calm" -> "Lisa". Keeps the role names SkyEye logs (Command, Hitman).
func shortVoiceName(s string) string {
	for _, sep := range []string{" - ", " – ", " — ", " | ", "- "} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "?"
	}
	return s
}

// applyAccount puts the account's figures on a fleet total: its used / plan /
// period are the truth; SkyEye's own per-rig count stays alongside.
func applyAccount(fu *FleetUsage, a *elevenAccount, now time.Time) {
	fu.Source = "account"
	fu.Account = a
	fu.SkyEyeCredits = fu.Credits
	fu.SkyEyeByVoice = fu.ByVoice
	fu.Credits, fu.Quota = a.Used, a.Limit
	fu.PeriodStart, fu.PeriodEnd = a.Start, a.Reset
	if a.ByVoice != nil {
		fu.ByVoice, fu.ByDay = a.ByVoice, a.ByDay
	}
	fu.Today = fu.ByDay[now.UTC().Format("2006-01-02")]
	fu.Percent, fu.Projected, fu.Level = 0, fu.Credits, "ok"
	if a.Limit > 0 {
		fu.Percent = 100 * float64(a.Used) / float64(a.Limit)
	}
	if days := now.Sub(a.Start).Hours() / 24; days >= 1 {
		fu.Projected = int(float64(a.Used) / days * a.Reset.Sub(a.Start).Hours() / 24)
	}
	switch {
	case fu.Percent >= 95:
		fu.Level = "error"
	case fu.Percent >= 80:
		fu.Level = "warn"
	}
}

// sortedDays lists a by-day map's keys in order (tests).
func sortedDays(m map[string]int) []string {
	d := make([]string, 0, len(m))
	for k := range m {
		d = append(d, k)
	}
	sort.Strings(d)
	return d
}
