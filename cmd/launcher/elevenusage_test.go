package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBillingPeriod(t *testing.T) {
	at := func(s string) time.Time { tm, _ := time.Parse("2006-01-02 15:04", s); return tm }
	cases := []struct {
		now        string
		day        int
		start, end string
	}{
		{"2026-10-15 12:00", 14, "2026-10-14", "2026-11-14"},
		{"2026-10-13 23:59", 14, "2026-09-14", "2026-10-14"},
		{"2026-10-14 00:00", 14, "2026-10-14", "2026-11-14"},
		{"2026-02-15 00:00", 31, "2026-01-31", "2026-02-28"},
		{"2026-03-01 00:00", 31, "2026-02-28", "2026-03-31"},
	}
	for _, c := range cases {
		s, e := billingPeriod(at(c.now), c.day)
		if s.Format("2006-01-02") != c.start || e.Format("2006-01-02") != c.end {
			t.Errorf("billingPeriod(%s, %d) = %s..%s, want %s..%s", c.now, c.day,
				s.Format("2006-01-02"), e.Format("2006-01-02"), c.start, c.end)
		}
	}
}

func TestReadAndSumUsage(t *testing.T) {
	dir := t.TempDir()
	write := func(day, body string) { os.WriteFile(filepath.Join(dir, day+".jsonl"), []byte(body), 0o644) }
	write("2026-10-13", `{"t":"2026-10-13T20:00:00Z","voice":"Command","credits":500,"chars":500}`+"\n")
	write("2026-10-15", strings.Join([]string{
		`{"t":"2026-10-15T01:00:00Z","voice":"Command","credits":100,"chars":100}`,
		`{"t":"2026-10-15T02:00:00Z","voice":"Lisa","credits":40,"chars":40}`,
		`not json`,
		`{"t":"2026-10-15T03:00:00Z","voice":"Marshal","credits":30,"chars":60}`,
	}, "\n")+"\n")
	write("notes.txt", "ignored")
	start, end := billingPeriod(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC), 14)
	u := readUsage(dir, start, end)
	if u.Credits != 170 || u.Calls != 3 || u.ByVoice["Command"] != 100 || u.ByVoice["Lisa"] != 40 {
		t.Fatalf("readUsage = %+v; want 170 credits in 3 calls (the 13th is last period)", u)
	}
	if got := topVoices(u.ByVoice); got[0] != "Command" {
		t.Errorf("topVoices = %v", got)
	}

	now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC)
	fu := sumUsage([]RigUsage{u, {Credits: 80000, ByVoice: map[string]int{"Hitman": 80000}, ByDay: map[string]int{"2026-10-16": 2000}}}, start, end, 100000, now)
	if fu.Credits != 80170 || fu.Level != "warn" || fu.ByVoice["Hitman"] != 80000 || fu.Today != 2000 {
		t.Errorf("sumUsage = %+v; want 80,170 credits, level warn", fu)
	}
	fu = sumUsage([]RigUsage{{Credits: 96000}}, start, end, 100000, now)
	if fu.Level != "error" {
		t.Errorf("96%%: level %s, want error", fu.Level)
	}
	if thousands(1234567) != "1,234,567" || thousands(12) != "12" {
		t.Error("thousands formatting")
	}
}

func TestShortVoiceName(t *testing.T) {
	for in, want := range map[string]string{
		"Lisa - Assertive Australian Woman - Calm":                "Lisa",
		"Jerry B. - Military Commander | Gruff, Gritty Authority": "Jerry B.",
		"Kateryna | Native Ukrainian":                             "Kateryna",
		"Naomy- Calm and Friendly- Australian":                    "Naomy",
		"Michael":                                                 "Michael",
		"":                                                        "?",
	} {
		if got := shortVoiceName(in); got != want {
			t.Errorf("shortVoiceName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The account's figures replace the local total, plan and period; SkyEye's
// own count stays alongside.
func TestApplyAccount(t *testing.T) {
	now := time.Date(2026, 10, 16, 16, 31, 0, 0, time.UTC)
	fu := sumUsage([]RigUsage{{Credits: 900, ByVoice: map[string]int{"Command": 900}}}, now, now, 100000, now)
	a := &elevenAccount{Tier: "creator", Used: 110000, Limit: 127855,
		Reset: time.Date(2026, 11, 14, 16, 31, 0, 0, time.UTC), Start: time.Date(2026, 10, 14, 16, 31, 0, 0, time.UTC),
		ByVoice: map[string]int{"Lisa": 60000, "Misa": 50000}, ByDay: map[string]int{"2026-10-16": 4000, "2026-10-15": 106000}}
	fu.SkyEyeCredits = fu.Credits
	applyAccount(&fu, a, now)
	if fu.Source != "account" || fu.Credits != 110000 || fu.Quota != 127855 || fu.SkyEyeCredits != 900 ||
		fu.Today != 4000 || fu.Level != "warn" || fu.ByVoice["Lisa"] != 60000 {
		t.Errorf("applyAccount = %+v", fu)
	}
	// Pace: 110,000 spent + SkyEye's 900 a day (counted from Oct 14 16:31,
	// two days) for the 29 days left - not 110,000 extrapolated.
	if fu.Projected < 110000 || fu.Projected > 125000 {
		t.Errorf("projected %d, want spent + the servers' rate", fu.Projected)
	}
	if d := sortedDays(fu.ByDay); d[0] != "2026-10-15" {
		t.Errorf("days %v", d)
	}
}
