package license

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type memStore struct{ start, last time.Time }

func (m *memStore) Load() (time.Time, time.Time) { return m.start, m.last }
func (m *memStore) Save(s, l time.Time) error    { m.start, m.last = s, l; return nil }

func testChecker(t *testing.T) (*Checker, ed25519.PrivateKey, *time.Time) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	c := &Checker{Dir: t.TempDir(), Pub: pub, Store: &memStore{}, Now: func() time.Time { return now }}
	return c, priv, &now
}

func TestSignParse(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	key, err := Sign(Licence{Squadron: "VFA-103", ID: "x1", Issued: "2026-10-23"}, priv)
	if err != nil {
		t.Fatal(err)
	}
	// Pasted with a line break in the middle still reads.
	l, err := Parse(key[:30]+"\r\n "+key[30:], pub)
	if err != nil || l.Squadron != "VFA-103" {
		t.Fatalf("%+v %v", l, err)
	}
	// One changed character anywhere breaks it.
	bad := []byte(key)
	bad[len(keyPrefix)+5] ^= 1
	if _, err := Parse(string(bad), pub); err == nil {
		t.Fatal("altered key accepted")
	}
	other, _, _ := ed25519.GenerateKey(nil)
	if _, err := Parse(key, other); err == nil {
		t.Fatal("key from another signer accepted")
	}
	if _, err := Parse("hello", pub); err == nil {
		t.Fatal("junk accepted")
	}
}

func TestTrial(t *testing.T) {
	c, _, now := testChecker(t)
	s := c.Check()
	if s.State != Trial || s.DaysLeft != 14 || !s.Allowed() {
		t.Fatalf("day 0: %+v", s)
	}
	*now = now.Add(13*24*time.Hour + time.Hour)
	if s := c.Check(); s.State != Trial || s.DaysLeft != 1 {
		t.Fatalf("day 13: %+v", s)
	}
	*now = now.Add(24 * time.Hour)
	if s := c.Check(); s.State != Expired || s.Allowed() {
		t.Fatalf("day 14: %+v", s)
	}
	// Setting the clock back does not reopen it.
	*now = now.Add(-10 * 24 * time.Hour)
	if s := c.Check(); s.State != Expired {
		t.Fatalf("clock back: %+v", s)
	}
}

// Deleting licence.json (or reinstalling) does not restart the trial: the
// registry keeps the start.
func TestTrialSurvivesReinstall(t *testing.T) {
	c, _, now := testChecker(t)
	c.Check()
	*now = now.Add(20 * 24 * time.Hour)
	os.Remove(filepath.Join(c.Dir, "licence.json"))
	if s := c.Check(); s.State != Expired {
		t.Fatalf("after reinstall: %+v", s)
	}
}

func TestActivate(t *testing.T) {
	c, priv, now := testChecker(t)
	*now = now.Add(30 * 24 * time.Hour) // trial long over
	c.Check()
	if _, err := c.Activate("SKYEYE1-junk.junk"); err == nil {
		t.Fatal("junk activated")
	}
	key, _ := Sign(Licence{Squadron: "VFA-103", ID: "a", Issued: "2026-11-01", Expires: "2027-11-01"}, priv)
	s, err := c.Activate(key)
	if err != nil || s.State != Licensed || !s.Allowed() || !strings.Contains(s.Message, "VFA-103") {
		t.Fatalf("%+v %v", s, err)
	}
	*now = time.Date(2027, 11, 2, 1, 0, 0, 0, time.UTC)
	if s := c.Check(); s.State != Expired {
		t.Fatalf("after the key's end: %+v", s)
	}
	perpetual, _ := Sign(Licence{Squadron: "vSFG-7", ID: "p", Issued: "2026-10-23"}, priv)
	if s, err := c.Activate(perpetual); err != nil || s.State != Licensed || s.DaysLeft != 0 {
		t.Fatalf("perpetual %+v %v", s, err)
	}
}

func TestSourceInstallNotChecked(t *testing.T) {
	c, _, now := testChecker(t)
	os.Mkdir(filepath.Join(c.Dir, ".git"), 0o755)
	*now = now.Add(365 * 24 * time.Hour)
	if s := c.Check(); s.State != Source || !s.Allowed() {
		t.Fatalf("%+v", s)
	}
}

func TestBuiltInPublicKey(t *testing.T) {
	if len(PublicKey) != ed25519.PublicKeySize || PublicKey.Equal(make(ed25519.PublicKey, ed25519.PublicKeySize)) {
		t.Fatal("no public key built in: every licence key would be rejected")
	}
}
