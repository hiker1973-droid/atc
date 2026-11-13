package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
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

func TestSpokenGridDropsZone(t *testing.T) {
	if got := spokenGrid("37T GJ 31827 70457"); got != "Golf Juliet, three one eight two seven, seven zero four five seven" {
		t.Errorf("spokenGrid = %q", got)
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

// Two wrong location readbacks: Hitman says line 6 once more and moves on to
// lines 7-9 instead of holding the flight (operator 2026-11-13).
func TestJTACLocationMissCap(t *testing.T) {
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	b.Response("Command, Raider 11, request tasking", "Raider 11", "Command")
	pic.place(tgt.IPPos, 0, 1, 90)
	b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic) // block 1
	if got := b.JTACResponse("Hitman, Raider 11, location Golf Juliet 11111 22222", "Raider 11", hitman, pic); !strings.Contains(got, "negative, readback incorrect. line 6") {
		t.Fatalf("first miss = %q", got)
	}
	got := b.JTACResponse("Hitman, Raider 11, location Golf Juliet 11111 22222", "Raider 11", hitman, pic)
	if !strings.Contains(got, "Continue. line 7") && !strings.Contains(got, "Continue. line 7, no mark") {
		t.Fatalf("second miss = %q", got)
	}
	if got := b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic); !strings.Contains(got, "confirmed. Report IP inbound.") {
		t.Errorf("block 3 = %q", got)
	}
}

// "Say again location" / "grid" repeats line 6 only.
func TestJTACSayAgainLocation(t *testing.T) {
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	b.Response("Command, Raider 11, request tasking", "Raider 11", "Command")
	pic.place(tgt.IPPos, 0, 1, 90)
	b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic)
	for _, call := range []string{"say again location", "say again the grid", "repeat location"} {
		got := b.JTACResponse("Hitman, Raider 11, "+call, "Raider 11", hitman, pic)
		if !strings.Contains(got, "line 6, location Golf Juliet") || strings.Contains(got, "line 4") {
			t.Errorf("%q = %q", call, got)
		}
	}
}

func TestLineSixRe(t *testing.T) {
	_, slow, _ := splitNineLine("Raider 1, line 4, x. line 5, y. line 6, location Golf Juliet, one two, three four. line 7, z. Read back.")
	loc := lineSixRe.FindStringIndex(slow)
	if loc == nil || slow[loc[0]:loc[1]] != "line 6, location Golf Juliet, one two, three four." {
		t.Fatalf("line 6 span: %v in %q", loc, slow)
	}
}

// Day 2026-11-13: Whisper heard "Hitman" as "Demand"; "reporting to IP Knife" is an IP call.
func TestJTACDemandAddress(t *testing.T) {
	for _, s := range []string{"Demand 21, Raider 302 reporting in, reporting to IP knife.", "Hit man 2-1, Raider 302, in hot."} {
		if cs := jtacCallsign(s); cs != "Raider 302" && cs != "Raider 3-0-2" {
			t.Errorf("%q -> %q", s, cs)
		}
	}
	if !isJTACInbound("raider 302 reporting in, reporting to ip knife.") {
		t.Error("reporting to IP knife is an IP inbound call")
	}
}

func TestJTACOffStationAndBDARequest(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	if got := b.JTACResponse("Hitman 1-1, Raider 11, request BDA", "Raider 11", hitman, pic); !strings.Contains(got, "negative BDA, no attack yet") {
		t.Errorf("BDA before any attack = %q", got)
	}
	b.JTACResponse("Hitman 1-1, Raider 11, request direct attack", "Raider 11", hitman, pic)
	if _, ok := b.flights["raider 11"]; !ok {
		t.Fatal("flight not on the board")
	}
	got := b.JTACResponse("Hitman 1-1, Raider 11, winchester, off station", "Raider 11", hitman, pic)
	if !strings.Contains(got, "copy winchester. You are released from the net") {
		t.Errorf("release = %q", got)
	}
	if _, ok := b.flights["raider 11"]; ok {
		t.Error("flight should be dropped")
	}
	b.JTACResponse("Hitman 1-1, Raider 12, request direct attack", "Raider 12", hitman, pic)
	if k := b.soleFlight("Hitman 21, pickle times two."); k != "raider 12" {
		t.Errorf("soleFlight = %q", k)
	}
	if k := b.soleFlight("Hitman 21, copy."); k != "" {
		t.Errorf("non-call soleFlight = %q", k)
	}
}

// Raider 302 asked for BDA 67 s after pickle: standby, and the pass stays open.
func TestJTACBDAEarlyRequestStandby(t *testing.T) {
	b, clock := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	pic.place(tgt.TgtPos, 169, 20, 349)
	b.JTACResponse("Hitman 1-1, Raider 11, request direct attack", "Raider 11", hitman, pic)
	pic.place(tgt.TgtPos, 169, 5, 349)
	b.JTACResponse("Hitman 1-1, Raider 11, in hot", "Raider 11", hitman, pic)
	*clock = clock.Add(60 * time.Second)
	got := b.JTACResponse("Hitman 1-1, Raider 11, request BDA", "Raider 11", hitman, pic)
	if !strings.Contains(got, "standby for BDA") || b.flights["raider 11"].bdaDone {
		t.Errorf("early BDA request = %q (bdaDone %v)", got, b.flights["raider 11"].bdaDone)
	}
	*clock = clock.Add(90 * time.Second)
	got = b.JTACResponse("Hitman 1-1, Raider 11, request BDA", "Raider 11", hitman, pic)
	if !strings.Contains(got, "BDA") || strings.Contains(got, "standby") {
		t.Errorf("late BDA request = %q", got)
	}
}

// Day 2026-11-13: Raider 302's four GBU-12s landed on target 9 while Hitman had
// him on target 5; the BDA said "no impacts observed". It now says where they hit.
func TestJTACBDAImpactsOffTarget(t *testing.T) {
	b, clock := newTestTaskingBoard(casTICTargets)
	b.jtacs = []*jtacNet{{Spoken: hitman, FreqMHz: 61.0}}
	var five, nine taskingTarget
	for _, tg := range casTICTargets {
		if tg.Phase == 1 && tg.Number == 5 {
			five = tg
		}
		if tg.Phase == 1 && tg.Number == 9 {
			nine = tg
		}
	}
	cur := &taskingAssignment{target: 0, clearedAt: *clock, stage: jtacCleared}
	for i, tg := range b.targets {
		if tg.Number == 5 && tg.Phase == 1 {
			cur.target = i
		}
	}
	b.flights["raider 302"] = cur
	pic := &fakePicture{impacts: []tacviewImpact{{Pos: nine.TgtPos, At: clock.Add(40 * time.Second), Shooter: "Raider 302"}}}
	*clock = clock.Add(75 * time.Second)
	got := b.JTACResponse("Hitman 2-1, Raider 302, request BDA", "Raider 302", hitman, pic)
	if !strings.Contains(got, "impacted on target 9, not target 5") || strings.Contains(got, "no impacts") {
		t.Errorf("off-target BDA = %q (five %d nine %d)", got, five.Number, nine.Number)
	}
}

// A re-attack by direct attack keeps the flight's target.
func TestJTACDirectAttackKeepsTarget(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	b.JTACResponse("Hitman 1-1, Raider 11, request direct attack", "Raider 11", hitman, pic)
	first := b.flights["raider 11"].target
	b.flights["raider 11"].stage = jtacOff
	got := b.JTACResponse("Hitman 1-1, Raider 11, request direct attack", "Raider 11", hitman, pic)
	if b.flights["raider 11"].target != first || !strings.Contains(got, "direct attack") {
		t.Errorf("target %d -> %d, reply %q", first, b.flights["raider 11"].target, got)
	}
}

// ...unless the target is destroyed: then a direct attack draws a new one.
func TestJTACDirectAttackAfterDestroyed(t *testing.T) {
	b, _ := newTestTaskingBoard(casTICTargets)
	b.jtacs = []*jtacNet{{Spoken: hitman, FreqMHz: 61.0}}
	pic := &fakePicture{}
	b.JTACResponse("Hitman 1-1, Raider 11, request direct attack", "Raider 11", hitman, pic)
	b.flights["raider 11"].stage = jtacOff
	got := b.JTACResponse("Hitman 1-1, Raider 11, target destroyed", "Raider 11", hitman, pic)
	if !strings.Contains(got, "copy target destroyed") || !b.flights["raider 11"].destroyed {
		t.Fatalf("destroyed call = %q", got)
	}
	b.JTACResponse("Hitman 1-1, Raider 11, request direct attack", "Raider 11", hitman, pic)
	if b.flights["raider 11"].destroyed {
		t.Error("new assignment should clear the destroyed flag")
	}
}

// Whisper heard "Hitman 1-1 on Raider, 3-1-2" (Day 2026-11-13): the callsign
// must still be Raider 312.
func TestJTACCallsignSkipsOn(t *testing.T) {
	if got := jtacCallsign("Hitman 1-1 on Raider, 3-1-2, IP inbound."); got != "Raider 312" {
		t.Fatalf("callsign = %q, want Raider 312", got)
	}
}

// Ordnance with no callsign and no address goes to the flight Hitman asked.
func TestJTACLoadAnswerNoCallsign(t *testing.T) {
	b, _ := newJTACBoard(t)
	b.askLoad = true
	pic := &fakePicture{}
	b.JTACResponse("Hitman one one, Raider three one two, checking in", "Raider 312", hitman, pic)
	text := "8 times GBU-12s, playtime 30 minutes."
	if r := b.JTACResponse(text, jtacCallsign(text), hitman, pic); r != "" {
		t.Fatalf("no callsign should not answer directly, got %q", r)
	}
	k := b.soleFlight(text)
	if k == "" {
		t.Fatal("soleFlight should find the flight that was asked for ordnance")
	}
	r := b.JTACResponse(text, displayCallsign(k), hitman, pic)
	if !strings.Contains(strings.ToLower(r), "copy ordnance and playtime") {
		t.Fatalf("answer should release the 9-line, got %q", r)
	}
}

// Line 8 with the wrong friendly distance is read back once, then accepted.
func TestJTACLine8WrongDistance(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 302, checking in", "Raider 302", hitman, pic)
	cur := b.flights["raider 302"]
	tg := b.targets[cur.target]
	d := nineData(tg)
	b.JTACResponse("Hitman, Raider 302, IP "+tg.IP+", heading "+fmt.Sprintf("%03d", tg.RunIn), "Raider 302", hitman, pic)
	b.JTACResponse("Hitman, Raider 302, elevation "+fmt.Sprint(tg.ElevFt)+", location "+d.Grid, "Raider 302", hitman, pic)
	if cur.stage != jtacBrief3 {
		t.Skipf("not at block 3 (stage %v)", cur.stage)
	}
	r := b.JTACResponse("Hitman, Raider 302, line 7 no mark, line 8, 9,900 meter friendly, egress", "Raider 302", hitman, pic)
	if !strings.Contains(strings.ToLower(r), "negative") || !strings.Contains(r, friendMetres(d.Friend)) {
		t.Fatalf("wrong friendly distance should be corrected, got %q", r)
	}
	r = b.JTACResponse("Hitman, Raider 302, line 8, "+friendMetres(d.Friend)+" meters friendly", "Raider 302", hitman, pic)
	if !strings.Contains(strings.ToLower(r), "confirmed") {
		t.Fatalf("correct distance should confirm, got %q", r)
	}
}

// "Hitman 2-1-2, Raider 3-0-2" / "Hitman 1-2, Raider 3-0-2": the digits after
// Hitman mean nothing (Day 2026-11-13, Raider 302).
func TestJTACGarbledAddressDigits(t *testing.T) {
	for _, in := range []string{
		"Hitman 2-1-2, Raider 3-0-2, Kilo Quebec, 6-8-3-6-4-0-7-8-4-2.",
		"Hitman 1-2, Raider 3-0-2, elevation 1-9-0-5.",
		"Hitman 2-1, Raider 3-0-2, copy.",
	} {
		if got := jtacCallsign(in); got != "Raider 302" {
			t.Errorf("jtacCallsign(%q) = %q, want Raider 302", in, got)
		}
	}
}

// A readback with no callsign at all goes to the flight being briefed.
func TestJTACReadbackNoCallsign(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 302, checking in", "Raider 302", hitman, pic)
	text := "Hitman 1-2, elevation 1-9-0-5, two tanks, Kilo Quebec, 7-6-8-5-3."
	if k := b.soleFlight(text); k != "raider 302" {
		t.Fatalf("soleFlight = %q, want raider 302", k)
	}
}

// An in call far out and off the attack heading is cleared hot with the heading.
func TestJTACInCallFarOutGetsHeading(t *testing.T) {
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 11, checking in", "Raider 11", hitman, pic)
	readBackAll(b, "Raider 11", pic)
	pic.place(tgt.TgtPos, 169, 11, 61)
	got := b.JTACResponse("Hitman, Raider 11, in from the south", "Raider 11", hitman, pic)
	if !strings.Contains(got, "cleared hot") || !strings.Contains(got, "Final attack heading") {
		t.Errorf("far-out off-heading in call = %q, want cleared hot with the heading", got)
	}
}
