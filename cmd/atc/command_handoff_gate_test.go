package main

import (
	"math"
	"testing"
)

// DCS3 2026-10-15 12:11: Raider 302's tasking request was still with Whisper
// when the recovery handoff fired, so he heard "contact Senaki tower, good
// landing" and then "contact Hitman one one for tasking".
func TestHandoffDefersWhileACallIsBeingAnswered(t *testing.T) {
	p := &trackedPilot{callsign: "Raider 302"}
	if g := gateHandoff(p, true); g != handoffDefer {
		t.Fatalf("gate with a call in flight = %v, want defer", g)
	}
	if g := gateHandoff(p, false); g != handoffFire {
		t.Fatalf("gate on a quiet net = %v, want fire", g)
	}
}

func TestTaskedPilotIsNotHandedOffUntilRTB(t *testing.T) {
	tr := newPilotTracker()
	tr.Note("Raider 302", "Command, Raider 302, request tasking.")
	p := tr.pilots["Raider 302"]
	if !p.tasked {
		t.Fatal("tasking request did not mark the pilot tasked")
	}
	if g := gateHandoff(p, false); g != handoffSkip {
		t.Fatalf("gate for a tasked pilot = %v, want skip", g)
	}

	// A crossing skipped while tasked leaves him inside the threshold; the
	// RTB has to re-arm it or he is never handed off.
	p.lastDistNm = 12
	tr.Note("Raider 302", "Command, Raider 302, RTB.")
	if p.tasked {
		t.Fatal("RTB did not clear tasked")
	}
	if !math.IsInf(p.lastDistNm, 1) {
		t.Errorf("lastDistNm after RTB = %v, want +Inf (crossing re-armed)", p.lastDistNm)
	}
	if g := gateHandoff(p, false); g != handoffFire {
		t.Errorf("gate after RTB = %v, want fire", g)
	}
}

func TestRecoveryCallsClearTasked(t *testing.T) {
	for _, text := range []string{
		"Command, Raider 302, fence out, state 3.2.",
		"Command, Raider 302, off station.",
		"Command, Raider 302, returning to base.",
	} {
		tr := newPilotTracker()
		tr.Note("Raider 302", "Command, Raider 302, request tasking.")
		tr.Note("Raider 302", text)
		if tr.pilots["Raider 302"].tasked {
			t.Errorf("%q left the pilot tasked", text)
		}
	}
}

// Command's own replies echo back on 282 (seen 2026-10-15). None of them may
// flip the pilot's state.
func TestCommandEchoesDoNotChangeTasked(t *testing.T) {
	tr := newPilotTracker()
	tr.Note("Raider 302", "Raider 302, vSFG-7-Command, contact Hitman one one on six one point zero zero zero for tasking.")
	if tr.pilots["Raider 302"].tasked {
		t.Error("the tasking handoff reply's echo marked the pilot tasked")
	}
	tr.Note("Raider 302", "Command, Raider 302, check in.")
	if tr.pilots["Raider 302"].tasked {
		t.Error("a check-in marked the pilot tasked")
	}
}
