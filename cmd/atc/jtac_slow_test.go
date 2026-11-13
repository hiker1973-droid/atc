package main

import (
	"strings"
	"testing"
)

func TestSplitNineLine(t *testing.T) {
	pre, slow, post := splitNineLine("Raider 1, target 3, armour. line 1, IP Alpha. line 2, heading zero nine zero. line 3, distance two zero miles. Read back.")
	if pre != "Raider 1, target 3, armour." || slow == "" || post != "" || slow[:6] != "line 1" {
		t.Fatalf("bad split: %q | %q | %q", pre, slow, post)
	}
	if _, s, _ := splitNineLine("copy, cleared hot."); s != "" {
		t.Fatalf("no 9-line expected, got %q", s)
	}
	base := voiceProfile{speed: 1.2, eleven: elevenVoice{ID: "x", Speed: 1.08}, lineSlow: 0.88}
	v := slowedVoice(base, 0.88)
	if v.speed >= base.speed || v.eleven.Speed >= 1.08 || v.lineSlow != 0 {
		t.Fatalf("slowedVoice wrong: %+v", v)
	}
}

func TestBriefReadbackLenient(t *testing.T) {
	tg := taskingTarget{IP: "Knife", RunIn: 312, ElevFt: 2615}
	// Block 1: Whisper lost the IP's name (Raider 312, Day 2026-11-12).
	if !briefReadbackOK("ip, 3-1-2, 8.7 miles", tg, 1, 0) && nineData(tg).DistNm > 0 {
		t.Log("distance not matched without data; fine")
	}
	if briefReadbackOK("heading 312", tg, 1, 0) && nineData(tg).DistNm == 0 {
		t.Error("heading alone accepted on the first try with no distance to back it")
	}
	if !briefReadbackOK("heading 312", tg, 1, 1) {
		t.Error("heading alone refused after a miss")
	}
	if briefReadbackOK("heading 100", tg, 1, 1) {
		t.Error("wrong heading accepted")
	}
}

func TestSoleBriefing(t *testing.T) {
	b := newTaskingBoard([]taskingTarget{{IP: "Knife"}})
	b.flights["raider 312"] = &taskingAssignment{stage: jtacBriefed}
	if k := b.soleBriefing("Hitman 1-1, repeat."); k != "raider 312" {
		t.Errorf("sole briefing = %q", k)
	}
	if k := b.soleBriefing("Raider 312, say again"); k != "" {
		t.Errorf("not addressed to Hitman: %q", k)
	}
	b.flights["raider 302"] = &taskingAssignment{stage: jtacBrief2}
	if k := b.soleBriefing("Hitman 1-1, repeat."); k != "" {
		t.Errorf("two briefing flights: %q", k)
	}
}

func TestJTACCallsignHitmanIn(t *testing.T) {
	for _, s := range []string{"Hitman in 1-1, Raider 312, repeat.", "Hitman 1-1, Raider 3-1-2, say again."} {
		if cs := jtacCallsign(s); cs != "Raider 312" && cs != "Raider 3-1-2" {
			t.Errorf("%q -> %q", s, cs)
		}
	}
}

// "Request direct attack": permission granted with no 9-line, then in hot is
// cleared with the usual checks (operator 2026-11-12).
func TestJTACDirectAttack(t *testing.T) {
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	pic.place(tgt.TgtPos, 169, 20, 349)
	got := b.JTACResponse("Hitman 1-1, Raider 11, request direct attack", "Raider 11", hitman, pic)
	if !strings.Contains(got, "permission granted, direct attack. Target 1,") || !strings.HasSuffix(got, "Call in hot.") || strings.Contains(got, "line 1") {
		t.Fatalf("direct attack = %q", got)
	}
	pic.place(tgt.TgtPos, 169, 5, 349)
	if got := b.JTACResponse("Hitman, Raider 11, in hot", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, cleared hot." {
		t.Errorf("in hot after direct attack = %q", got)
	}
}
