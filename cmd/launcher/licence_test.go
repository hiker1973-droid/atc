package main

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/license"
)

type memLicStore struct{ start, last time.Time }

func (m *memLicStore) Load() (time.Time, time.Time) { return m.start, m.last }
func (m *memLicStore) Save(s, l time.Time) error    { m.start, m.last = s, l; return nil }

// Past the trial with no key, Start is refused with the reason; a key
// entered on the Settings page lets it through again.
func TestLicenceGatesStart(t *testing.T) {
	root := withRoot(t)
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now()
	store := &memLicStore{start: now.Add(-15 * 24 * time.Hour)}
	old := newLicenceChecker
	newLicenceChecker = func() *license.Checker {
		return &license.Checker{Dir: root, Pub: pub, Store: store, Now: func() time.Time { return now }}
	}
	t.Cleanup(func() { newLicenceChecker = old; licChecked = time.Time{} })
	licChecked = time.Time{}

	reached := false
	h := requireLicence(func(w http.ResponseWriter, r *http.Request) { reached = true })
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/api/start?name=x", nil))
	if reached || rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), "trial has ended") {
		t.Fatalf("expired trial: %d %q reached=%v", rec.Code, rec.Body.String(), reached)
	}

	key, _ := license.Sign(license.Licence{Squadron: "VFA-103", ID: "t", Issued: "2026-10-23"}, priv)
	if _, err := newLicenceChecker().Activate(key); err != nil {
		t.Fatal(err)
	}
	licChecked = time.Time{}
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/api/start?name=x", nil))
	if !reached {
		t.Fatalf("licensed start refused: %d %q", rec.Code, rec.Body.String())
	}
}
