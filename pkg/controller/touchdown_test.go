package controller

import (
	"context"
	"strings"
	"testing"
)

// "Wheels down" after landing: report clear of the active, and the automatic
// "report clear" chase does not ask again (operator 2026-10-16).
func TestTouchdownAsksForClearCall(t *testing.T) {
	c, sent, handle := departureController(t)
	handle("Test Tower, Raider 303, wheels down")
	if len(*sent) == 0 {
		t.Fatal("no reply to wheels down")
	}
	got := (*sent)[len(*sent)-1]
	if !strings.HasPrefix(got, "Raider 303, Test Tower, ") ||
		!(strings.Contains(got, "report clear of the active") || strings.Contains(got, "report clear of runway") || strings.Contains(got, "report runway vacated")) {
		t.Errorf("wheels down -> %q", got)
	}
	c.mu.Lock()
	ac := c.airfieldState.Get("Raider 303")
	chased := ac != nil && ac.RunwayChased
	c.mu.Unlock()
	if !chased {
		t.Error("the runway chase would ask again")
	}
	// Then the clear call closes it as before.
	handle("Test Tower, Raider 303, clear of the active")
	if got := (*sent)[len(*sent)-1]; !strings.Contains(got, "taxi to parking") {
		t.Errorf("clear of the active -> %q", got)
	}
	_ = context.Background
}
