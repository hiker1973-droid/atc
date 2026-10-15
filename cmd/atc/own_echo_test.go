package main

import "testing"

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

func TestOwnEchoCatchesTheBrief(t *testing.T) {
	resetOwnTX()
	noteOwnTX(61, "Raider 302, Hitman one one, target 3, Fuel Depot, IP Cobra. Cobra bears two-niner-four for 26. Read back.")
	if !isOwnEcho(61, "Rider 302, Hitman 1-1, Target 3, Fuel Depot, IP Cobra. Cobra bears 294 for 26. Read back.") {
		t.Error("the 14:15:22 brief echo was not caught")
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
