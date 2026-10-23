// Package license is SkyEye ATC's 14-day trial and licence keys (operator
// 2026-10-23: "adding a demo mode for 14 days is good before we require them
// to purchase a key", then "ok build licsencing and 14 day demo").
//
// A key is a small signed statement - squadron, id, issue date, optional
// expiry - checked offline against the public key built in below. Keys are
// made by cmd/licensekey with the private key, which lives only on the
// operator's PC (never in this repo, never in SkyEye-Setup.exe), so a key
// cannot be forged or edited.
//
// Without a key an install runs a 14-day trial counted from its first start.
// A git checkout - the vSFG-7 rigs and the dev box, built from the private
// repo - is a source install and is never checked: the rigs must not go
// quiet when a trial would have run out.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TrialDays is the trial length.
const TrialDays = 14

// keyPrefix starts every key, so a pasted key is recognisable.
const keyPrefix = "SKYEYE1-"

// Licence is what a key says.
type Licence struct {
	Squadron string `json:"sq"`
	ID       string `json:"id"`
	Issued   string `json:"iss"`           // 2006-01-02
	Expires  string `json:"exp,omitempty"` // 2006-01-02, "" = never
}

// ExpiresAt is the end of the expiry day (UTC), zero for a perpetual key.
func (l Licence) ExpiresAt() time.Time {
	if l.Expires == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", l.Expires)
	if err != nil {
		return time.Unix(0, 0)
	}
	return t.Add(24 * time.Hour)
}

// Sign makes a key (cmd/licensekey).
func Sign(l Licence, priv ed25519.PrivateKey) (string, error) {
	payload, err := json.Marshal(l)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(priv, payload)
	enc := base64.RawURLEncoding
	return keyPrefix + enc.EncodeToString(payload) + "." + enc.EncodeToString(sig), nil
}

// ErrBadKey: not a SkyEye key, or one that was altered.
var ErrBadKey = errors.New("that is not a valid SkyEye licence key")

// Parse checks a key's signature against pub and returns what it says.
// Spaces and line breaks from copying are ignored.
func Parse(key string, pub ed25519.PublicKey) (Licence, error) {
	key = strings.Join(strings.Fields(key), "")
	if !strings.HasPrefix(key, keyPrefix) {
		return Licence{}, ErrBadKey
	}
	body, sigStr, ok := strings.Cut(strings.TrimPrefix(key, keyPrefix), ".")
	if !ok {
		return Licence{}, ErrBadKey
	}
	enc := base64.RawURLEncoding
	payload, err1 := enc.DecodeString(body)
	sig, err2 := enc.DecodeString(sigStr)
	if err1 != nil || err2 != nil || len(sig) != ed25519.SignatureSize || !ed25519.Verify(pub, payload, sig) {
		return Licence{}, ErrBadKey
	}
	var l Licence
	if err := json.Unmarshal(payload, &l); err != nil || l.ID == "" {
		return Licence{}, ErrBadKey
	}
	return l, nil
}

// State is an install's standing.
type State string

const (
	Source   State = "source"   // git checkout: not checked
	Licensed State = "licensed" // a valid key
	Trial    State = "trial"
	Expired  State = "expired" // trial over, or the key's expiry passed
)

// Status is what the launcher shows and both programs enforce.
type Status struct {
	State      State     `json:"state"`
	DaysLeft   int       `json:"daysLeft,omitempty"` // trial, or a key with an expiry
	TrialStart time.Time `json:"trialStart,omitempty"`
	Licence    *Licence  `json:"licence,omitempty"`
	Message    string    `json:"message"`
}

// Allowed reports whether roles may run.
func (s Status) Allowed() bool { return s.State != Expired }

// File is licence.json in the install folder: the key, and the trial dates
// (also kept in the registry, see Store).
type File struct {
	Key        string    `json:"key,omitempty"`
	TrialStart time.Time `json:"trialStart,omitempty"`
	LastSeen   time.Time `json:"lastSeen,omitempty"`
}

// Store keeps the trial dates where a reinstall does not reach (the
// registry on Windows). Zero times mean nothing stored.
type Store interface {
	Load() (start, lastSeen time.Time)
	Save(start, lastSeen time.Time) error
}

// Checker works out an install's Status.
type Checker struct {
	Dir   string            // the install folder (beside atc.exe / launcher.exe)
	Pub   ed25519.PublicKey // PublicKey unless a test says otherwise
	Store Store             // RegistryStore unless a test says otherwise
	Now   func() time.Time
}

// NewChecker is the real checker for an install folder.
func NewChecker(dir string) *Checker {
	return &Checker{Dir: dir, Pub: PublicKey, Store: RegistryStore{}, Now: time.Now}
}

func (c *Checker) path() string { return filepath.Join(c.Dir, "licence.json") }

func (c *Checker) readFile() File {
	var f File
	if b, err := os.ReadFile(c.path()); err == nil {
		_ = json.Unmarshal(b, &f)
	}
	return f
}

func (c *Checker) writeFile(f File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.path() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path())
}

// IsSource reports a git checkout (the vSFG-7 rigs, the dev box).
func IsSource(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func daysLeft(until, now time.Time) int {
	d := until.Sub(now)
	if d <= 0 {
		return 0
	}
	return int((d + 24*time.Hour - time.Nanosecond) / (24 * time.Hour)) // part of a day counts
}

// Check works out the Status, starting the trial on first use. It records
// when it last ran, so setting the clock back does not buy days.
func (c *Checker) Check() Status {
	if IsSource(c.Dir) {
		return Status{State: Source, Message: "Source install: licence not checked."}
	}
	now := c.Now()
	f := c.readFile()
	if f.Key != "" {
		if l, err := Parse(f.Key, c.Pub); err == nil {
			exp := l.ExpiresAt()
			if exp.IsZero() {
				return Status{State: Licensed, Licence: &l, Message: "Licensed to " + l.Squadron + "."}
			}
			if now.Before(exp) {
				return Status{State: Licensed, Licence: &l, DaysLeft: daysLeft(exp, now),
					Message: fmt.Sprintf("Licensed to %s until %s.", l.Squadron, l.Expires)}
			}
			return Status{State: Expired, Licence: &l,
				Message: fmt.Sprintf("The licence for %s ended on %s. Enter a new key in Settings.", l.Squadron, l.Expires)}
		}
	}
	// Trial: the earliest start either place knows, the latest time seen.
	rs, rl := c.Store.Load()
	start := earliest(f.TrialStart, rs)
	last := latest(f.LastSeen, rl)
	if start.IsZero() {
		start = now
	}
	if now.Before(last) {
		now = last // the clock went back: count from the latest time seen
	}
	f.TrialStart, f.LastSeen = start, now
	c.Store.Save(start, now)
	c.writeFile(f)
	end := start.Add(TrialDays * 24 * time.Hour)
	if now.Before(end) {
		left := daysLeft(end, now)
		return Status{State: Trial, TrialStart: start, DaysLeft: left,
			Message: fmt.Sprintf("Trial: %d day%s left. Enter a licence key in Settings to keep SkyEye running.", left, plural(left))}
	}
	return Status{State: Expired, TrialStart: start,
		Message: "The 14-day trial has ended. Enter a licence key in Settings to start SkyEye again."}
}

// Activate checks a key and saves it.
func (c *Checker) Activate(key string) (Status, error) {
	l, err := Parse(key, c.Pub)
	if err != nil {
		return c.Check(), err
	}
	if exp := l.ExpiresAt(); !exp.IsZero() && !c.Now().Before(exp) {
		return c.Check(), fmt.Errorf("that key ended on %s", l.Expires)
	}
	f := c.readFile()
	f.Key = strings.Join(strings.Fields(key), "")
	if err := c.writeFile(f); err != nil {
		return c.Check(), err
	}
	return c.Check(), nil
}

func earliest(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero() || a.Before(b):
		return a
	}
	return b
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
