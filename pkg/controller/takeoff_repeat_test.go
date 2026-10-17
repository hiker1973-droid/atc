package controller

import (
	"strings"
	"testing"
)

// A jet already cleared that asks again gets its clearance repeated, not a
// spacing hold counted from its own clearance (Venom 021, Day 2026-10-16:
// "hold short, departure spacing zero seconds").
func TestTakeoffRequestAfterClearance(t *testing.T) {
	_, sent, handle := departureController(t)
	handle("Test Tower, Venom 021, request departure")
	if got := (*sent)[len(*sent)-1]; !cleared(got) {
		t.Fatalf("first request -> %q", got)
	}
	handle("Test Tower, Venom 021, request departure")
	if got := (*sent)[len(*sent)-1]; !cleared(got) || strings.Contains(got, "spacing") {
		t.Errorf("second request -> %q, want the clearance repeated", got)
	}
	// Another jet inside the window is still held.
	handle("Test Tower, Venom 022, request departure")
	if got := (*sent)[len(*sent)-1]; !strings.Contains(got, "spacing") {
		t.Errorf("next jet -> %q, want a spacing hold", got)
	}
}

func cleared(got string) bool {
	return strings.Contains(got, "for departure") || strings.Contains(got, "for takeoff")
}
