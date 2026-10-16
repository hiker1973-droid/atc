package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ElevenLabs credit use on the dashboard (operator 2026-10-15). Every atc.exe
// appends one line per ElevenLabs call to <root>\elevenlabs_usage\
// YYYY-MM-DD.jsonl (cmd/atc/eleven_usage.go). All rigs share one ElevenLabs
// account, so the number that matters is the fleet's: each launcher totals its
// own rig (/api/elevenlabs-usage) and sums the fleet (/api/elevenlabs-fleet)
// against the plan's monthly credits, warning at 80 % and 95 % - when the
// credits run out every role silently drops to OpenAI.

var (
	// Must match atc.exe's --elevenlabs-stop-at (same default): the dashboard
	// warns ahead of the switch the roles make (cmd/atc/eleven_cap.go).
	flagElevenStopAt = flag.Int("elevenlabs-stop-at", 127000, "ElevenLabs credits at which the SkyEye roles switch to OpenAI (atc.exe --elevenlabs-stop-at); the dashboard warns as it nears")
	flagElevenMonthly  = flag.Int("elevenlabs-monthly-credits", 127500, "ElevenLabs plan credits per billing month, used only when the account cannot be read (squadron plan: 127,500 - operator 2026-10-15); the dashboard warns at 80 % and 95 %")
	flagElevenResetDay = flag.Int("elevenlabs-reset-day", 14, "Day of the month the ElevenLabs credits reset (the subscription date; Creator from 2026-10-14)")
)

type usageLine struct {
	Time     time.Time `json:"t"`
	Voice    string    `json:"voice"`
	Airfield string    `json:"airfield"`
	Model    string    `json:"model"`
	Chars    int       `json:"chars"`
	Credits  int       `json:"credits"`
}

// RigUsage is one rig's ElevenLabs use in a billing period.
type RigUsage struct {
	Rig     string         `json:"rig,omitempty"`
	Label   string         `json:"label,omitempty"`
	Credits int            `json:"credits"`
	Chars   int            `json:"chars"`
	Calls   int            `json:"calls"`
	ByVoice map[string]int `json:"byVoice"` // credits
	ByDay   map[string]int `json:"byDay"`   // credits, YYYY-MM-DD (UTC)
	First   time.Time      `json:"first,omitempty"`
	Last    time.Time      `json:"last,omitempty"`
	Err     string         `json:"err,omitempty"`
}

// FleetUsage is the whole account's use in the period.
type FleetUsage struct {
	PeriodStart time.Time      `json:"periodStart"`
	PeriodEnd   time.Time      `json:"periodEnd"`
	Quota       int            `json:"quota"`
	Credits     int            `json:"credits"`
	Percent     float64        `json:"percent"`
	Projected   int            `json:"projected"` // at this period's daily rate, by PeriodEnd
	Today       int            `json:"today"`
	Level       string         `json:"level"` // ok | warn | error
	ByVoice     map[string]int `json:"byVoice"`
	ByDay       map[string]int `json:"byDay"`
	Rigs        []RigUsage     `json:"rigs"`
	First       time.Time      `json:"first,omitempty"`
	At          time.Time      `json:"at"`
	// Source is "account" when the totals above are the ElevenLabs account's
	// own (elevenaccount.go), "skyeye" when they are SkyEye's local count.
	Source     string         `json:"source"`
	Account    *elevenAccount `json:"account,omitempty"`
	AccountErr string         `json:"accountErr,omitempty"`
	// SkyEye's own transmissions (the servers' burn, auditions excluded) -
	// the panel's headline (operator 2026-10-15: "We just need to look at
	// server messages for burn"). Filled for either source.
	SkyEyeCredits   int            `json:"skyeyeCredits"`
	SkyEyeToday     int            `json:"skyeyeToday"`
	SkyEyeProjected int            `json:"skyeyeProjected"`
	SkyEyeByVoice   map[string]int `json:"skyeyeByVoice,omitempty"`
	SkyEyeByDay     map[string]int `json:"skyeyeByDay,omitempty"`
}

// billingPeriod is the credit month containing now: from resetDay (clamped to
// the month's length) to the same day next month, UTC.
func billingPeriod(now time.Time, resetDay int) (time.Time, time.Time) {
	now = now.UTC()
	day := func(y int, m time.Month) time.Time {
		last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
		d := resetDay
		if d < 1 {
			d = 1
		}
		if d > last {
			d = last
		}
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	start := day(now.Year(), now.Month())
	if start.After(now) {
		start = day(now.Year(), now.Month()-1)
	}
	return start, day(start.Year(), start.Month()+1)
}

// readUsage totals the usage files under dir for [start, end).
func readUsage(dir string, start, end time.Time) RigUsage {
	u := RigUsage{ByVoice: map[string]int{}, ByDay: map[string]int{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			u.Err = err.Error()
		}
		return u
	}
	for _, e := range entries {
		day, err := time.Parse("2006-01-02.jsonl", e.Name())
		if err != nil || day.Before(start.AddDate(0, 0, -1)) || !day.Before(end) {
			continue
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var l usageLine
			if json.Unmarshal(sc.Bytes(), &l) != nil || l.Time.Before(start) || !l.Time.Before(end) {
				continue
			}
			u.Credits += l.Credits
			u.Chars += l.Chars
			u.Calls++
			u.ByVoice[l.Voice] += l.Credits
			u.ByDay[l.Time.UTC().Format("2006-01-02")] += l.Credits
			if u.First.IsZero() || l.Time.Before(u.First) {
				u.First = l.Time
			}
			if l.Time.After(u.Last) {
				u.Last = l.Time
			}
		}
		f.Close()
	}
	return u
}

func usageDir() string { return filepath.Join(rootDir, "elevenlabs_usage") }

// handleElevenUsage is this rig's use. ?start=&end= (unix seconds) let the
// fleet total ask every rig for the same period.
func handleElevenUsage(w http.ResponseWriter, r *http.Request) {
	start, end := billingPeriod(time.Now(), *flagElevenResetDay)
	if s, err := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64); err == nil {
		start = time.Unix(s, 0).UTC()
	}
	if e, err := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64); err == nil {
		end = time.Unix(e, 0).UTC()
	}
	writeJSON(w, readUsage(usageDir(), start, end))
}

var (
	fleetUsageMu    sync.Mutex
	fleetUsageCache *FleetUsage
)

// fleetUsage sums every rig's use for the current period; cached for maxAge.
func fleetUsage(maxAge time.Duration) FleetUsage {
	fleetUsageMu.Lock()
	defer fleetUsageMu.Unlock()
	if fleetUsageCache != nil && time.Since(fleetUsageCache.At) < maxAge {
		return *fleetUsageCache
	}
	now := time.Now()
	start, end := billingPeriod(now, *flagElevenResetDay)
	rigs := make([]RigUsage, len(fleetRigs))
	var wg sync.WaitGroup
	for i, rig := range fleetRigs {
		wg.Add(1)
		go func(i int, rig Rig) {
			defer wg.Done()
			var u RigUsage
			if isSelf(rig) {
				u = readUsage(usageDir(), start, end)
			} else {
				url := fmt.Sprintf("http://%s:%d/api/elevenlabs-usage?start=%d&end=%d", rig.Host, rig.Port, start.Unix(), end.Unix())
				if err := fleetGetJSON(url, &u); err != nil {
					u = RigUsage{Err: err.Error()}
				}
			}
			u.Rig, u.Label = rig.Name, rig.Label
			rigs[i] = u
		}(i, rig)
	}
	wg.Wait()
	fu := sumUsage(rigs, start, end, *flagElevenMonthly, now)
	fu.Source = "skyeye"
	fu.SkyEyeCredits, fu.SkyEyeToday, fu.SkyEyeProjected = fu.Credits, fu.Today, fu.Projected
	fu.SkyEyeByVoice, fu.SkyEyeByDay = fu.ByVoice, fu.ByDay
	if a, err := fetchElevenAccount(now); err == nil {
		applyAccount(&fu, a, now)
	} else {
		fu.AccountErr = err.Error()
	}
	fleetUsageCache = &fu
	return fu
}

func sumUsage(rigs []RigUsage, start, end time.Time, quota int, now time.Time) FleetUsage {
	fu := FleetUsage{PeriodStart: start, PeriodEnd: end, Quota: quota, Rigs: rigs, At: now,
		ByVoice: map[string]int{}, ByDay: map[string]int{}}
	for _, u := range rigs {
		fu.Credits += u.Credits
		for k, v := range u.ByVoice {
			fu.ByVoice[k] += v
		}
		for k, v := range u.ByDay {
			fu.ByDay[k] += v
		}
		if !u.First.IsZero() && (fu.First.IsZero() || u.First.Before(fu.First)) {
			fu.First = u.First
		}
	}
	fu.Today = fu.ByDay[now.UTC().Format("2006-01-02")]
	if quota > 0 {
		fu.Percent = 100 * float64(fu.Credits) / float64(quota)
	}
	// Daily rate since counting began (or the period start), carried to the end.
	from := start
	if fu.First.After(from) {
		from = fu.First
	}
	if days := now.Sub(from).Hours() / 24; days >= 1 {
		fu.Projected = int(float64(fu.Credits) / days * end.Sub(from).Hours() / 24)
	} else {
		fu.Projected = fu.Credits
	}
	switch {
	case fu.Percent >= 95:
		fu.Level = "error"
	case fu.Percent >= 80:
		fu.Level = "warn"
	default:
		fu.Level = "ok"
	}
	return fu
}

func handleElevenFleet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, fleetUsage(60*time.Second))
}

// elevenUsageAlerts warns ahead of the OpenAI switch at --elevenlabs-stop-at
// (operator 2026-10-16: "flip to open AI once we hit 127k. We should see that
// coming though"):
//
//	past the switch         error  the roles are on OpenAI until the reset
//	within 10,000 of it     warn   how far, and the day at the servers' pace
//	pace reaches it         warn   the day it will, before the reset
//
// then the plan's own 80 % / 95 % when there is no switch.
func elevenUsageAlerts() []Alert {
	return usageAlerts(fleetUsage(5*time.Minute), *flagElevenStopAt, time.Now())
}

// elevenCapNear: the warning band below the switch.
const elevenCapNear = 10000

func usageAlerts(fu FleetUsage, stopAt int, now time.Time) []Alert {
	if fu.Quota <= 0 {
		return nil
	}
	alert := func(level, format string, a ...any) []Alert {
		return []Alert{{Level: level, Source: "ElevenLabs", Message: fmt.Sprintf(format, a...), Time: now}}
	}
	reset := fu.PeriodEnd.Local().Format("Mon 2 Jan 15:04")
	if stopAt > 0 {
		if fu.Credits >= stopAt {
			return alert("error", "ElevenLabs %s used - past the %s switch: SkyEye is speaking with OpenAI voices until the reset %s",
				thousands(fu.Credits), thousands(stopAt), reset)
		}
		hit, ok := capDay(fu, stopAt, now)
		switch {
		case fu.Credits >= stopAt-elevenCapNear && ok:
			return alert("warn", "ElevenLabs %s used - %s to the %s OpenAI switch, about %s at the servers' pace",
				thousands(fu.Credits), thousands(stopAt-fu.Credits), thousands(stopAt), hit.Local().Format("Mon 2 Jan"))
		case fu.Credits >= stopAt-elevenCapNear:
			return alert("warn", "ElevenLabs %s used - %s to the %s OpenAI switch",
				thousands(fu.Credits), thousands(stopAt-fu.Credits), thousands(stopAt))
		case ok:
			return alert("warn", "ElevenLabs on pace to reach the %s OpenAI switch about %s (resets %s) - %s used",
				thousands(stopAt), hit.Local().Format("Mon 2 Jan"), reset, thousands(fu.Credits))
		}
		return nil
	}
	msg := fmt.Sprintf("ElevenLabs credits %s of %s this month (%.0f%%) - at 100%% every role drops to OpenAI voices",
		thousands(fu.Credits), thousands(fu.Quota), fu.Percent)
	switch fu.Level {
	case "error", "warn":
		return alert(fu.Level, "%s", msg)
	}
	if fu.Projected > fu.Quota && fu.Credits > 0 {
		return alert("warn", "ElevenLabs on pace for %s credits this month, plan is %s", thousands(fu.Projected), thousands(fu.Quota))
	}
	return nil
}

// capDay is when the servers' daily burn reaches stopAt, if that is before
// the period resets. The servers' rate, not the account's, so a day of voice
// auditions does not bring the date in (see applyAccount).
func capDay(fu FleetUsage, stopAt int, now time.Time) (time.Time, bool) {
	from := fu.First
	if from.IsZero() || from.Before(fu.PeriodStart) {
		from = fu.PeriodStart
	}
	days := now.Sub(from).Hours() / 24
	if days < 1 {
		days = 1
	}
	rate := float64(fu.SkyEyeCredits) / days // credits a day
	if rate <= 0 {
		return time.Time{}, false
	}
	hit := now.Add(time.Duration(float64(stopAt-fu.Credits) / rate * 24 * float64(time.Hour)))
	if !fu.PeriodEnd.IsZero() && !hit.Before(fu.PeriodEnd) {
		return time.Time{}, false
	}
	return hit, true
}

func thousands(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// topVoices orders a credits-by-voice map, largest first (for tests and logs).
func topVoices(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	return keys
}
