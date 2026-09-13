package controller

import (
	"context"
	"time"

	"github.com/vsfg7/atc/pkg/composer"
)

// SayAgainWindow is how old the tower's last transmission to a callsign may
// be and still be repeated on "say again". Past it the pilot is told there is
// nothing on file rather than hearing a stale clearance read back.
const SayAgainWindow = 2 * time.Minute

type lastTransmission struct {
	text string
	at   time.Time
}

// transmitTo sends text like transmit and remembers it as the last thing said
// to callsign, so a later "say again" can repeat it. Guarded by its own mutex
// because the proactive monitor transmits outside HandleRequest's lock.
func (c *ATCController) transmitTo(ctx context.Context, callsign, text string) {
	c.lastTxMu.Lock()
	if c.lastTx == nil {
		c.lastTx = make(map[string]lastTransmission)
	}
	c.lastTx[callsign] = lastTransmission{text: text, at: time.Now()}
	c.lastTxMu.Unlock()
	c.transmit(ctx, text)
}

// lastTransmissionTo returns the last text sent to callsign, if it was sent
// within SayAgainWindow.
func (c *ATCController) lastTransmissionTo(callsign string) (string, bool) {
	c.lastTxMu.Lock()
	defer c.lastTxMu.Unlock()
	tx, ok := c.lastTx[callsign]
	if !ok || time.Since(tx.at) > SayAgainWindow {
		return "", false
	}
	return tx.text, true
}

// parseOption reads a touch and go / low approach / the option out of a
// lowercased transcript. It is parsed for every call, not just landing
// requests, so "turning base, touch and go" gets the option clearance too.
func parseOption(lower string) string {
	switch {
	case containsAny(lower, "touch and go", "touch-and-go", "touch and goes", "touch n go"):
		return composer.OptionTouchAndGo
	case containsAny(lower, "low approach"):
		return composer.OptionLowApproach
	case containsWord(lower, "the option", "request option"):
		return composer.OptionTheOption
	}
	return composer.OptionNone
}
