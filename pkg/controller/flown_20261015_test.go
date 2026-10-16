package controller

import (
	"strings"
	"testing"
)

// Raider 302 at Senaki, DCS3 2026-10-15, as Whisper wrote it.

// No comma after "traffic": the callsign came out empty and the tower said
// ", Senaki Tower, runway zero niner is clear, welcome back" (16:03:14).
func TestCallsignWithoutCommaAfterTraffic(t *testing.T) {
	req := ParseIntent("Senaki traffic Raider 302, clear active 09.", "Senaki Tower")
	if req == nil {
		t.Fatal("not parsed")
	}
	if req.Callsign != "Raider 302" {
		t.Errorf("callsign = %q, want Raider 302", req.Callsign)
	}
}

// A call with no callsign at all is never answered with a blank callsign
// (", Senaki Tower, ..."); since the operator's 2026-10-15 ruling it gets one
// "say again with full callsign" and is not acted on.
func TestNoCallsignNoTransmission(t *testing.T) {
	_, sent, handle := newRecordingController(t)
	handle("Senaki traffic, clear active 09.")
	for _, s := range *sent {
		if strings.HasPrefix(strings.TrimSpace(s), ",") {
			t.Errorf("transmitted with a blank callsign: %q", s)
		}
	}
	if len(*sent) != 1 || !strings.Contains((*sent)[0], "full callsign") {
		t.Errorf("want only 'say again with full callsign': %q", *sent)
	}
}

// "10-mile initial" was read as the break ("break acknowledged", 16:17:15).
func TestTenMileInitialWithHyphen(t *testing.T) {
	req := ParseIntent("Senaki traffic, Raider 302, 10-mile initial for 09.", "Senaki Tower")
	if req == nil {
		t.Fatal("not parsed")
	}
	if req.Type != RequestDistanceInitial || req.DistanceNm != 10 {
		t.Errorf("type = %d distance = %d, want RequestDistanceInitial at 10", req.Type, req.DistanceNm)
	}
	if req := ParseIntent("Senaki traffic, Raider 302, left break for 09.", "Senaki Tower"); req == nil || req.Type != RequestBreak {
		t.Errorf("the real break call no longer reads as the break")
	}
}
