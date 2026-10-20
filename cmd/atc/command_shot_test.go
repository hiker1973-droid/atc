package main

import (
	"strings"
	"testing"

	"github.com/paulmach/orb"
)

// Day 2026-10-19 16:59-17:00, Raider 302: shots and the kill were intent misses.
func TestCommandShotResponse(t *testing.T) {
	store := newTacviewPositions()
	store.Set("Raider 302", orb.Point{41.6, 42.0})
	for in, want := range map[string]string{
		"302 Fox 3, 26 for 73, 26,000 MiG-29.": "Raider 302, vSFG-7-Command, ",
		"302 Fox 3, 13 for 135.":               "Fox three",
		"302, splash MiG-29.":                  "splash",
		"Command, Raider 302, fox two.":        "Fox two",
	} {
		cs := joinCallsignDigits(extractCallsignSkippingAddress(in, commandAddresses...))
		got := commandReply(in, cs, "vSFG-7-Command", store)
		if !strings.Contains(got, want) || !strings.HasPrefix(got, "Raider 302, ") {
			t.Errorf("%q (callsign %q): %q, want it to contain %q", in, cs, got, want)
		}
	}
	if got := commandShotResponse("Command, Raider 302, request tasking.", "Raider 302", "vSFG-7-Command", store); got != "" {
		t.Errorf("non-shot call answered: %q", got)
	}
	store.Set("Venom 302", orb.Point{41.7, 42.1})
	if got := store.CallsignByNumber("302"); got != "" {
		t.Errorf("two jets end in 302, got %q", got)
	}
}

// Day 2026-10-19 19:28-19:45: Raider 302's attack calls to Command were misses.
func TestCommandAttackCalls(t *testing.T) {
	store := newTacviewPositions()
	store.Set("Raider 302", orb.Point{41.6, 42.0})
	for in, want := range map[string]string{
		"Command, Raider 302 proceeding to IP Granite.":  "Raider 302, vSFG-7-Command, ",
		"Command, Raider 302 inbound artillery battery.": "Raider 302, vSFG-7-Command, ",
		"Command, Raider 302 in hot.":                    "cleared hot",
		"Command, Raider 302, pickle times two.":         "pickle",
		"Command, Raider 302, rifle.":                    "rifle",
	} {
		cs := joinCallsignDigits(extractCallsignSkippingAddress(in, commandAddresses...))
		got := commandReply(in, cs, "vSFG-7-Command", store)
		if !strings.Contains(got, want) || !strings.HasPrefix(got, "Raider 302, ") {
			t.Errorf("%q (callsign %q): %q, want it to contain %q", in, cs, got, want)
		}
	}
	if got := commandReply("Command, M203 copies.", "M203", "vSFG-7-Command", store); got != "" {
		t.Errorf("acknowledgement answered: %q", got)
	}
}
