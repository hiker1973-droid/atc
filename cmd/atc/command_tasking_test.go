package main

import (
	"testing"
	"time"
)

func newTestTaskingBoard(targets []taskingTarget) (*taskingBoard, *time.Time) {
	clock := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	b := newTaskingBoard(targets)
	b.now = func() time.Time { return clock }
	b.intn = func(int) int { return 0 } // always the first target still free
	return b, &clock
}

// The draw is random, but never the target the flight has just worked while
// another is free.
func TestTaskingRandomNeverRepeats(t *testing.T) {
	b := newTaskingBoard(caucasusTaskingTargets)
	last := ""
	for i := 0; i < 200; i++ {
		got := b.Response("request tasking", "Raider 11", "Command")
		if got == last {
			t.Fatalf("draw %d repeated %q", i, got)
		}
		last = got
		b.Response("copy", "Raider 11", "Command")
	}
}

// The whole exchange: request, tasking, readback, cleared hot.
func TestTaskingExchange(t *testing.T) {
	const channel = "Command"
	b, _ := newTestTaskingBoard(caucasusTaskingTargets)

	// A "copy" with no tasking outstanding is not Command's to answer.
	if got := b.Response("Command, Raider 11, copy", "Raider 11", channel); got != "" {
		t.Errorf("copy before any tasking = %q, want silence", got)
	}

	want := "Raider 11, Command, proceed to IP Anvil, target 1, Enguri Bridge."
	if got := b.Response("Command, Raider 11, request tasking", "Raider 11", channel); got != want {
		t.Errorf("request = %q, want %q", got, want)
	}
	// Asking again before the readback repeats the tasking rather than moving on.
	if got := b.Response("Command, Raider 11, requesting tasking", "Raider 11", channel); got != want {
		t.Errorf("repeat request = %q, want %q", got, want)
	}
	// A radio check in the middle is not a readback.
	if got := b.Response("Command, Raider 11, how copy", "Raider 11", channel); got != "" {
		t.Errorf("how copy = %q, want silence from tasking", got)
	}

	want = "Raider 11, Command, you are cleared hot."
	if got := b.Response("Command, Raider 11 copy", "Raider 11", channel); got != want {
		t.Errorf("readback = %q, want %q", got, want)
	}
	// Cleared once; a second "copy" gets nothing.
	if got := b.Response("Command, Raider 11, copy", "Raider 11", channel); got != "" {
		t.Errorf("second copy = %q, want silence", got)
	}

	// Asking after the readback moves to the next target.
	want = "Raider 11, Command, proceed to IP Brick, target 2, Rioni Rail Bridge."
	if got := b.Response("Command, Raider 11, request tasking", "Raider 11", channel); got != want {
		t.Errorf("next request = %q, want %q", got, want)
	}
	// Reading back the IP name counts without a "copy".
	if got := b.Response("Command, Raider 11, IP Brick", "Raider 11", channel); got != "Raider 11, Command, you are cleared hot." {
		t.Errorf("IP readback = %q", got)
	}
}

// Two flights never get the same target while both hold one.
func TestTaskingSkipsHeldTargets(t *testing.T) {
	b, clock := newTestTaskingBoard(caucasusTaskingTargets[:2])

	if got := b.Response("request tasking", "Raider 11", "Command"); got != "Raider 11, Command, proceed to IP Anvil, target 1, Enguri Bridge." {
		t.Errorf("Raider 11 = %q", got)
	}
	if got := b.Response("request tasking", "Viper 21", "Command"); got != "Viper 21, Command, proceed to IP Brick, target 2, Rioni Rail Bridge." {
		t.Errorf("Viper 21 = %q", got)
	}
	// Raider is done with Anvil; Brick is still Viper's, so Raider gets Anvil back.
	b.Response("copy", "Raider 11", "Command")
	if got := b.Response("request tasking", "Raider 11", "Command"); got != "Raider 11, Command, proceed to IP Anvil, target 1, Enguri Bridge." {
		t.Errorf("Raider 11 second = %q", got)
	}
	b.Response("copy", "Raider 11", "Command")

	// Once Viper's hold lapses Brick is free again.
	*clock = clock.Add(taskingHoldTime + time.Minute)
	if got := b.Response("request tasking", "Raider 11", "Command"); got != "Raider 11, Command, proceed to IP Brick, target 2, Rioni Rail Bridge." {
		t.Errorf("Raider 11 after hold = %q", got)
	}
}

func TestTaskingAckWindow(t *testing.T) {
	b, clock := newTestTaskingBoard(caucasusTaskingTargets)
	b.Response("request tasking", "Raider 11", "Command")
	*clock = clock.Add(taskingAckWindow + time.Second)
	if got := b.Response("copy", "Raider 11", "Command"); got != "" {
		t.Errorf("late copy = %q, want silence", got)
	}
}

func TestTaskingNoTargetsOnThisMap(t *testing.T) {
	if taskingTargetsForMap("pg") != nil {
		t.Fatal("pg should have no tasking targets")
	}
	if len(taskingTargetsForMap("Caucasus")) != 5 {
		t.Fatal("caucasus should have five tasking targets")
	}
	b, _ := newTestTaskingBoard(nil)
	if got := b.Response("request tasking", "Raider 11", "Command"); got != "Raider 11, Command, negative tasking at this time." {
		t.Errorf("no targets = %q", got)
	}
	if got := b.Response("copy", "Raider 11", "Command"); got != "" {
		t.Errorf("copy with no targets = %q, want silence", got)
	}
}

// "Raider 11 copy" with no comma must not carry the "copy" into the callsign.
func TestTaskingReadbackCallsign(t *testing.T) {
	for _, text := range []string{"Command, Raider 11 copy", "Command Raider 11 copy", "Command, Viper 21 wilco, proceeding IP Anvil", "Command Raider 11 request tasking"} {
		cs := extractCallsignSkippingAddress(text, "command")
		if cs != "Raider 11" && cs != "Viper 21" {
			t.Errorf("extractCallsignSkippingAddress(%q) = %q", text, cs)
		}
	}
}
