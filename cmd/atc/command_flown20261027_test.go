package main

import (
	"strings"
	"testing"
)

// Command calls from the Night and Foothold logs of 2026-10-27 - 28.
func TestCommandFlown20261027(t *testing.T) {
	name, freq := flagCommandGCIName, flagCommandGCIFreq
	flagCommandGCIName, flagCommandGCIFreq = "Darkstar", 264
	defer func() { flagCommandGCIName, flagCommandGCIFreq = name, freq }()

	reply := func(text string) string {
		cs := joinCallsignDigits(extractCallsignSkippingAddress(text, commandAddresses...))
		return commandReply(text, cs, "vSFG-7-Command", nil)
	}
	// A tower call on the wrong radio is not Command's.
	for _, text := range []string{"Tower, Venom 212 inbound.", "Senaki Tower, Venom 201, request taxi.", "Senaki Ground, Raider 302, request startup."} {
		if got := reply(text); got != "" {
			t.Errorf("%q answered %q, want silence", text, got)
		}
	}
	// A picture request goes to the AWACS.
	got := reply("Commander 303, request picture.")
	if !strings.HasPrefix(got, "303, vSFG-7-Command,") || !strings.Contains(got, "Darkstar, two six four point zero") {
		t.Errorf("picture request answered %q", got)
	}
	// Fence in keeps its own reply (which names the AWACS after it).
	if got := reply("Command, Raider 303, fencing in air-to-air."); strings.Contains(got, "picture is with") {
		t.Errorf("fence in answered %q", got)
	}
	// Check-ins still answered.
	if got := reply("Command, Raider 303 checking in."); !strings.Contains(got, "Raider 303") {
		t.Errorf("check-in answered %q", got)
	}
}
