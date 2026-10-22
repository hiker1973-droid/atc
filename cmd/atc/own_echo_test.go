package main

import (
	"strings"
	"testing"
)

func resetOwnTX() {
	ownTXMu.Lock()
	ownTXLog = nil
	ownTXMu.Unlock()
}

// The 2026-10-15 14:25 loop: every way Whisper wrote our "copy off" back.
func TestOwnEchoCatchesTheHitmanLoop(t *testing.T) {
	resetOwnTX()
	noteOwnTX(61, "Raider 302, Hitman one one, copy off. Egress back to Cobra, angels 14. Report IP inbound for re-attack.")
	for _, heard := range []string{
		"Raider 302, Hitman 1-1, copy off. Egress back to Cobra, Angels 14. Report IP inbound for re-attack.",
		"Raider 302, Hitman 11, copy off. Egress back to Cobra. Angels 14. Report IP inbound for re-attack.",
		"Raider 302, Hitman 111, copy off. Egress back to Cobra, Angels 14. Report IP inbound for re-attack.",
		"Raider 302, Hitland 11. Copy off. Egress back to Cobra. Angels 14. Report IP inbound for re-attack.",
	} {
		if !isOwnEcho(61, heard) {
			t.Errorf("not caught as our echo: %q", heard)
		}
	}
	if isOwnEcho(282, "Raider 302, Hitman 1-1, copy off. Egress back to Cobra, Angels 14. Report IP inbound for re-attack.") {
		t.Error("an echo on 61 was matched on 282")
	}
}

// Pilot calls that reuse our words must still get through.
func TestOwnEchoLetsPilotCallsThrough(t *testing.T) {
	resetOwnTX()
	noteOwnTX(61, "Raider 302, Hitman one one, target 3, Fuel Depot, IP Cobra. Cobra bears two-niner-four for 26. Read back.")
	noteOwnTX(61, "Raider 302, Hitman one one, copy off. Egress back to Cobra, angels 14. Report IP inbound for re-attack.")
	for _, heard := range []string{
		"Hitman 1-1, Raider 302, copy, fuel depot, IP Cobra.",
		"Hitman 1-1, Raider 3-0-2, copy IP Cobra, target 3.",
		"Hitman 1-1, Raider 302, IP inbound.",
		"Hitman 1-1, Raider 302 off.",
	} {
		if isOwnEcho(61, heard) {
			t.Errorf("pilot call dropped as our echo: %q", heard)
		}
	}
}

// Day 2026-10-22 17:19: a repeated radio check shares the callsign and the
// station name with our reply, and was dropped four times.
func TestOwnEchoLetsRepeatedRadioChecksThrough(t *testing.T) {
	resetOwnTX()
	noteOwnTX(280, "Raider 302, Red-Command, loud and clear.")
	noteOwnTX(282, "Raider 302, vSFG-7-Command, five by five.")
	for _, c := range []struct {
		freq  float64
		heard string
	}{
		{280, "Command, Raider 302, comm check."},
		{280, "Red command, Raider 302, comm check."},
		{280, "Red Command, Raider 302, com check."},
		{280, "Red Command Raider 302 comm check."},
		{282, "vSFG-7 Command, Raider 302, radio check."},
	} {
		if isOwnEcho(c.freq, c.heard) {
			t.Errorf("radio check dropped as our echo: %q", c.heard)
		}
	}
	if got := strings.Join(echoMessage("Raider 302, Hitland 11. Copy off. Egress 360.100"), " "); got != "copy off egress 360100" {
		t.Errorf("echoMessage = %q", got)
	}
}

func TestOwnEchoCatchesTheBrief(t *testing.T) {
	resetOwnTX()
	noteOwnTX(61, "Raider 302, Hitman one one, target 3, Fuel Depot, IP Cobra. Cobra bears two-niner-four for 26. Read back.")
	if !isOwnEcho(61, "Rider 302, Hitman 1-1, Target 3, Fuel Depot, IP Cobra. Cobra bears 294 for 26. Read back.") {
		t.Error("the 14:15:22 brief echo was not caught")
	}
}

// Foothold 2026-10-18 11:47 and 12:50: Command's spoken frequency is eight
// words, Whisper's "360.100" two, so the echo only just passed the length test.
func TestOwnEchoSpokenFrequency(t *testing.T) {
	resetOwnTX()
	noteOwnTX(282, "Raider 303, vSFG-7-Command, contact Incirlik tower on three six zero point one zero zero, switching now approved, good landing.")
	for _, heard := range []string{
		"Raider 303, VSFG-7 Command, contact Intelic Tower on 360.100. Switching now approved. Good landing.",
		"Raider 303, BSFG7 command, contact Intelic Tower on 360.100, switching now approved. Good landing.",
	} {
		if !isOwnEcho(282, heard) {
			t.Errorf("handoff echo not caught: %q", heard)
		}
	}
	if got := strings.Join(echoWords("two-niner-four for 26, 360.100, point five, vSFG-7-Command, Hitman one one"), " "); got != "294 for 26 360100 point 5 vsfg 7 command hitman 11" {
		t.Errorf("echoWords = %q", got)
	}
}

func TestJoinCallsignDigits(t *testing.T) {
	for in, want := range map[string]string{
		"Raider 3-0-2": "Raider 302",
		"Raider 3 0 2": "Raider 302",
		"Raider 302":   "Raider 302",
		"Venom 2-0-3":  "Venom 203",
		"Raider":       "Raider",
		"":             "",
	} {
		if got := joinCallsignDigits(in); got != want {
			t.Errorf("joinCallsignDigits(%q) = %q, want %q", in, got, want)
		}
	}
	if got := jtacCallsign("Hitman 1-1, Raider 3-0-2, IP inbound."); got != "Raider 302" {
		t.Errorf("jtacCallsign = %q, want Raider 302", got)
	}
}

func TestUndoubleCallsign(t *testing.T) {
	scope := map[string]bool{"Raider 302": true, "Raider 311": true, "Venom 33": true}
	known := func(c string) bool { return scope[c] }
	for in, want := range map[string]string{
		"Raider 3022": "Raider 302", // DCS3 2026-10-15 14:54:27
		"Raider 3311": "Raider 311", // 2026-07-02
		"Raider 302":  "Raider 302", // already right
		"Raider 4455": "Raider 4455", // no repair on scope
		"Venom 33":    "Venom 33",   // known, and too short to touch
		"Raider":      "Raider",
		"":            "",
	} {
		if got := undoubleCallsign(in, known); got != want {
			t.Errorf("undoubleCallsign(%q) = %q, want %q", in, got, want)
		}
	}
	if got := undoubleCallsign("Raider 3022", nil); got != "Raider 3022" {
		t.Errorf("no picture = %q", got)
	}
}
