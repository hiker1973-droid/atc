package main

import "testing"

// A callsign ends at its number (live miss, Day 2026-10-15 09:50: "Command,
// Raider 302 com check" was answered as "Raider 302 com").
func TestCallsignEndsAtItsNumber(t *testing.T) {
	cases := map[string]string{
		"Command, Raider 302 com check.":          "Raider 302",
		"Command, Raider 302 comcheck":            "Raider 302",
		"Command, Raider 3-0-2 how copy":          "Raider 3-0-2",
		"Command Raider 3 0 2 radio check":        "Raider 3 0 2",
		"Command, Viper 21 good morning, inbound": "Viper 21",
		"Command, Raider 11 copy":                 "Raider 11",
		"Command, Venom Flight checking in":       "Venom Flight",
	}
	for text, want := range cases {
		if got := extractCallsignSkippingAddress(text, "command"); got != want {
			t.Errorf("extractCallsignSkippingAddress(%q) = %q, want %q", text, got, want)
		}
	}
	if got := jtacCallsign("Hitman 1-1, Raider 302 com check."); got != "Raider 302" {
		t.Errorf("Hitman: %q", got)
	}
}
