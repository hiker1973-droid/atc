package main

import (
	"strings"
	"testing"
)

// The three recognition gaps from Raider 302's first flight on the new build
// (DCS3 2026-10-14): Command and Hitman ignored "com check" / "comcheck" /
// "comp check", and "request tasking" came back from Whisper as "request
// taxiing". (The third, Tbilisi's tower name, is in pkg/controller.)
func TestCommandAndHitmanRadioCheckVariants(t *testing.T) {
	for _, call := range []string{"Command, Raider 302, com check", "Command, Raider 302, comcheck", "Command, Raider 302, comp check"} {
		if got := commandResponse(call, "Raider 302", "Command"); !strings.Contains(got, "Raider 302, Command,") {
			t.Errorf("Command %q -> %q", call, got)
		}
	}
	b := newTaskingBoard(caucasusTaskingTargets)
	for _, call := range []string{"Hitman, Raider 302, com check", "Hitman, Raider 302, comp check"} {
		if got := b.JTACResponse(call, "Raider 302", "Hitman one one", tacviewJTAC{}); got != "Raider 302, Hitman one one, loud and clear." {
			t.Errorf("Hitman %q -> %q", call, got)
		}
	}
}

func TestTaskingHeardAsTaxiing(t *testing.T) {
	if !isTaskingRequest("command, raider 302, request taxiing") {
		t.Error(`"request taxiing" is not taken as a tasking request`)
	}
	if !strings.Contains(jtacWhisperPrompt, "request tasking") {
		t.Error("Hitman's Whisper prompt lacks \"request tasking\"")
	}
}
