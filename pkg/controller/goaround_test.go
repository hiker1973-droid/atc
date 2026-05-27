package controller

import (
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

func TestGoAroundDebounce(t *testing.T) {
	c := NewATCController("Test Tower", &airfield.Airfield{})

	if !c.canIssueGoAround("Venom 211") {
		t.Fatal("first call must be allowed (no prior TX)")
	}

	c.recordGoAround("Venom 211")

	if c.canIssueGoAround("Venom 211") {
		t.Error("second call within cooldown must be blocked")
	}

	if !c.canIssueGoAround("Raider 039") {
		t.Error("cooldown is per-callsign — different callsign must be allowed")
	}

	c.goAroundMu.Lock()
	c.goAroundLastTx["Venom 211"] = time.Now().Add(-2 * GoAroundCooldown)
	c.goAroundMu.Unlock()

	if !c.canIssueGoAround("Venom 211") {
		t.Error("after cooldown expiry the callsign must be allowed again")
	}
}
