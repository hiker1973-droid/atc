package main

import (
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
)

func newTestAreaBoard(toJTAC func(string, []int) *jtacNet) (*rangeAreaBoard, *time.Time) {
	clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	return &rangeAreaBoard{areas: caucasusRangeAreas, working: make(map[string]rangeWork),
		now: func() time.Time { return clock }, toJTAC: toJTAC}, &clock
}

func noMagVar(orb.Point) float64 { return 0 }

func TestRangeAreaIn(t *testing.T) {
	for in, want := range map[string]string{
		"command, raider 302, proceeding to alpha":           "A",
		"command, raider 302, heading for target area bravo": "B",
		"raider 302 en route to area c":                      "C",
		"raider 302, inbound echo":                           "E",
		"raider 302 proceeding to the target area alfa":      "A",
		"raider 302 proceeding to range juliett":             "J",
		"raider 302 proceeding to a tanker":                  "",
		"raider 302, request tasking":                        "",
	} {
		got := ""
		if i := rangeAreaIn(caucasusRangeAreas, in); i >= 0 {
			got = caucasusRangeAreas[i].Letter
		}
		if got != want {
			t.Errorf("rangeAreaIn(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every letter on the card is in the table, in order.
func TestRangeAreaTable(t *testing.T) {
	if len(caucasusRangeAreas) != 15 {
		t.Fatalf("%d areas, want A-O", len(caucasusRangeAreas))
	}
	for i, a := range caucasusRangeAreas {
		if a.Letter != string(rune('A'+i)) {
			t.Errorf("area %d is %q", i, a.Letter)
		}
		if len(a.NineLine) == 0 && a.What == "" {
			t.Errorf("area %s has nothing to say", a.Letter)
		}
	}
}

func TestRangeAreaReply(t *testing.T) {
	r, clock := newTestAreaBoard(nil)
	pic := &fakePicture{}
	got := r.Response("Command, Raider 302, proceeding to Alpha", "Raider 302", "Command", pic, noMagVar)
	if want := "Raider 302, Command, copy Alpha, Maykop airfield, SA-6 site and parked aircraft. Threat SA-6. Report off target."; got != want {
		t.Errorf("Alpha = %q", got)
	}
	// Bearing and range with a picture; a second flight hears who is there.
	pic.pos, pic.have = orb.Point{40.0356, 44.6777 - 1}, true // 60 nm due south of Alpha
	got = r.Response("Command, Venom 21, heading to Alpha", "Venom 21", "Command", pic, noMagVar)
	if !strings.Contains(got, "Alpha bears zero-zero-zero for 60.") {
		t.Errorf("bearing = %q", got)
	}
	if !strings.Contains(got, "Be advised, Raider 302 working Alpha.") {
		t.Errorf("deconfliction = %q", got)
	}
	// No threat said as such.
	if got := r.Response("Command, Raider 302, proceeding to Foxtrot", "Raider 302", "Command", nil, noMagVar); !strings.Contains(got, "armour and infantry. No radar threat.") {
		t.Errorf("Foxtrot = %q", got)
	}
	// Off: always answered. Naming another area does not free the one held.
	if got := r.Response("Command, Raider 302, off Alpha", "Raider 302", "Command", nil, noMagVar); got != "Raider 302, Command, copy off Alpha. Say intentions." {
		t.Errorf("off another area = %q", got)
	}
	if got := r.Response("Command, Raider 302, off target", "Raider 302", "Command", nil, noMagVar); got != "Raider 302, Command, copy off Foxtrot. Say intentions." {
		t.Errorf("off target = %q", got)
	}
	if got := r.Response("Command, Raider 302, off target", "Raider 302", "Command", nil, noMagVar); got != "Raider 302, Command, copy off target. Say intentions." {
		t.Errorf("second off = %q", got)
	}
	for _, c := range []string{"Command, Hawk 11, off and egressing", "Command, Hawk 11, off, egress north", "Command, Hawk 11, off safe", "Command, Hawk 11, coming off target", "Command, Hawk 11, egressing target area"} {
		if got := r.Response(c, "Hawk 11", "Command", nil, noMagVar); got != "Hawk 11, Command, copy off target. Say intentions." {
			t.Errorf("%q -> %q", c, got)
		}
	}
	r.intn = func(int) int { return 1 }
	if got := r.Response("Command, Hawk 11, off target", "Hawk 11", "Command", nil, noMagVar); got != "Hawk 11, Command, copy off target. Report RTB or request further tasking." {
		t.Errorf("second wording = %q", got)
	}
	r.intn = nil
	// Past the hold nobody is still working it.
	*clock = clock.Add(rangeAreaHold + time.Minute)
	if got := r.Response("Command, Raider 11, proceeding to Alpha", "Raider 11", "Command", nil, noMagVar); strings.Contains(got, "Be advised") {
		t.Errorf("stale deconfliction = %q", got)
	}
}

// The 9-line areas go to the JTAC that runs them, with the target drawn.
func TestRangeAreaNineLineToJTAC(t *testing.T) {
	cmd, kuban, cherkessk := newTwoJTACBoard(t)
	r, _ := newTestAreaBoard(cmd.handOffTarget)
	pic := &fakePicture{}
	got := r.Response("Command, Raider 302, proceeding to Hotel", "Raider 302", "Command", nil, noMagVar)
	if want := "Raider 302, Command, Hotel is nine-line target 4. Contact Hitman two one on six two point zero zero zero for tasking."; got != want {
		t.Errorf("Hotel = %q", got)
	}
	if got := cmd.Response("Command, Raider 302, copy", "Raider 302", "Command"); got != "Raider 302, Command, switch Hitman two one, good hunting." {
		t.Errorf("readback = %q", got)
	}
	if got := cherkessk.JTACResponse("Hitman 2-1, Raider 302, checking in", "Raider 302", "Hitman two one", pic); !strings.Contains(got, "target 4, Fuel Depot") {
		t.Errorf("check-in = %q, want the target Command named", got)
	}
	got = r.Response("Command, Venom 21, heading to Delta", "Venom 21", "Command", nil, noMagVar)
	if want := "Venom 21, Command, Delta is nine-line targets 1 and 2. Contact Hitman one one on six one point zero zero zero for tasking."; got != want {
		t.Errorf("Delta = %q", got)
	}
	if got := kuban.JTACResponse("Hitman 1-1, Venom 21, checking in", "Venom 21", hitman, pic); !strings.Contains(got, "target 1,") && !strings.Contains(got, "target 2,") {
		t.Errorf("Delta check-in = %q", got)
	}
	// No JTAC: the flight is told to ask for tasking.
	r2, _ := newTestAreaBoard(nil)
	if got := r2.Response("Command, Raider 302, proceeding to India", "Raider 302", "Command", nil, noMagVar); got != "Raider 302, Command, India is nine-line target 5. Request tasking." {
		t.Errorf("India without JTAC = %q", got)
	}
}

// "Permission to proceed" goes up to Overlord Actual: Command stands the
// flight by, Overlord comes up with Irene, the copy gets Command's brief.
func TestRangeAreaOverlord(t *testing.T) {
	r, clock := newTestAreaBoard(nil)
	r.pending = make(map[string]*overlordRequest)
	var queued []func()
	var delay time.Duration
	r.after = func(d time.Duration, f func()) { delay = d; queued = append(queued, f) }
	var sent []string
	r.overlordTX = func(text string) { sent = append(sent, text) }

	if got := r.Response("Command, Raider 302, permission to proceed to Alpha", "Raider 302", "Command", nil, noMagVar); got != "Raider 302, Command, stand by for Overlord Actual." {
		t.Fatalf("permission = %q", got)
	}
	if delay != overlordDelay || len(queued) != 1 {
		t.Fatalf("Overlord scheduled %v x%d", delay, len(queued))
	}
	// A copy before Overlord has spoken is not the copy of Irene.
	if got := r.Response("Command, Raider 302, copy", "Raider 302", "Command", nil, noMagVar); got != "" {
		t.Errorf("early copy = %q", got)
	}
	*clock = clock.Add(overlordDelay)
	queued[0]()
	if len(sent) != 1 || sent[0] != "Raider 302, Overlord Actual, you are Irene for Alpha, report copy." {
		t.Fatalf("Overlord = %v", sent)
	}
	got := r.Response("Overlord Actual, Raider 302, copy Irene", "Raider 302", "Command", nil, noMagVar)
	if want := "Raider 302, Command, copy Irene. Alpha, Maykop airfield, SA-6 site and parked aircraft. Threat SA-6. Report off target."; got != want {
		t.Errorf("after Irene = %q", got)
	}
	// Someone already there: Overlord says no.
	r.Response("Command, Venom 21, permission to proceed to Alpha", "Venom 21", "Command", nil, noMagVar)
	queued[1]()
	if len(sent) != 2 || sent[1] != "Venom 21, Overlord Actual, negative Irene, Raider 302 working Alpha. Request another area." {
		t.Errorf("busy area = %v", sent)
	}
	if got := r.Response("Command, Venom 21, copy", "Venom 21", "Command", nil, noMagVar); got != "" {
		t.Errorf("copy after negative = %q", got)
	}
	// A 9-line area: Irene, then the JTAC handoff.
	r.Response("Command, Venom 21, request permission to proceed to Delta", "Venom 21", "Command", nil, noMagVar)
	queued[2]()
	if got := r.Response("Command, Venom 21, copies", "Venom 21", "Command", nil, noMagVar); got != "Venom 21, Command, copy Irene. Delta is nine-line targets 1 and 2. Request tasking." {
		t.Errorf("9-line after Irene = %q", got)
	}
	// Telling, not asking, still gets the brief straight away.
	if got := r.Response("Command, Hawk 11, proceeding to Golf", "Hawk 11", "Command", nil, noMagVar); !strings.HasPrefix(got, "Hawk 11, Command, copy Golf") {
		t.Errorf("no permission asked = %q", got)
	}
}

// The ways pilots ask for, or announce, an area (operator 2026-10-16: "lets
// think of variations to this that people will use").
func TestRangeAreaPhrasings(t *testing.T) {
	ask := []string{
		"Command, Raider 302, permission to proceed to Alpha",
		"Command, Raider 302, request permission to proceed to Alpha",
		"Command, Raider 302, requesting permission to proceed to target area Alpha",
		"Command, Raider 302, permission to proceed Alpha",
		"Command, Raider 302, request to proceed to Alpha",
		"Command, Raider 302, request proceed Alpha",
		"Command, Raider 302, request clearance to Alpha",
		"Command, Raider 302, request clearance into Alpha",
		"Command, Raider 302, requesting entry into area Alpha",
		"Command, Raider 302, request to enter Alpha",
		"Command, Raider 302, permission to enter range Alpha",
		"Command, Raider 302, request to push Alpha",
		"Command, Raider 302, permission to push to Alpha",
		"Command, Raider 302, request to work Alpha",
		"Command, Raider 302, request to engage Alpha",
		"Command, Raider 302, request to attack target Alpha",
		"Command, Raider 302, request Alpha",
		"Command, Raider 302, requesting Alpha",
		"Command, Raider 302, two ship, request area Alpha",
		"Command, Raider 302, can we proceed to Alpha?",
		"Command, Raider 302, are we cleared to Alpha?",
		"Command, Raider 302, request clearance to the Alpha target area",
		"Command, Raider 302, permission to proceed to area A",
		"Command, Raider 302, permission to proceed to Alfa",
		"Command, Raider 302, request weapons free Alpha",
	}
	tell := []string{
		"Command, Raider 302, proceeding to Alpha",
		"Command, Raider 302, pushing Alpha",
		"Command, Raider 302, en route Alpha",
		"Command, Raider 302, heading to the Alpha area",
		"Command, Raider 302, inbound Alpha",
		"Command, Raider 302, entering Alpha",
		"Command, Raider 302, setting up for Alpha",
		"Command, Raiders 302, fencing in air-to-ground for target Alpha, state 10.7.", // Day 2026-10-16 16:39
	}
	none := []string{
		"Command, Raider 302, request tasking",
		"Command, Raider 302, request tanker",
		"Command, Raider 302, request bullseye",
		"Command, Raider 302, proceeding to the tanker",
		"Command, Raider 302, request vector to Mother",
	}
	run := func(call string) string {
		r, _ := newTestAreaBoard(nil)
		r.pending = make(map[string]*overlordRequest)
		r.after = func(time.Duration, func()) {}
		r.overlordTX = func(string) {}
		return r.Response(call, "Raider 302", "Command", nil, noMagVar)
	}
	for _, c := range ask {
		if got := run(c); got != "Raider 302, Command, stand by for Overlord Actual." {
			t.Errorf("%q -> %q, want Overlord", c, got)
		}
	}
	for _, c := range tell {
		if got := run(c); !strings.HasPrefix(got, "Raider 302, Command, copy Alpha,") {
			t.Errorf("%q -> %q, want the brief", c, got)
		}
	}
	for _, c := range none {
		if got := run(c); got != "" {
			t.Errorf("%q -> %q, want silence", c, got)
		}
	}
	// A phonetic word in the callsign names no area.
	r, _ := newTestAreaBoard(nil)
	if got := r.Response("Command, Mike 11, request tanker", "Mike 11", "Command", nil, noMagVar); got != "" {
		t.Errorf("Mike 11 tanker request = %q", got)
	}
	// "copies, proceeding Alpha" is the copy of Irene, not a new call.
	r.pending = make(map[string]*overlordRequest)
	var later func()
	r.after = func(_ time.Duration, f func()) { later = f }
	r.overlordTX = func(string) {}
	r.Response("Command, Raider 302, request Alpha", "Raider 302", "Command", nil, noMagVar)
	later()
	if got := r.Response("Command, Raider 302 copies, proceeding Alpha", "Raider 302", "Command", nil, noMagVar); !strings.HasPrefix(got, "Raider 302, Command, copy Irene. Alpha,") {
		t.Errorf("copy with the area = %q", got)
	}
}

// Overlord alternates "Irene" and "cleared to engage"; Command's brief follows.
func TestOverlordWordings(t *testing.T) {
	r, _ := newTestAreaBoard(nil)
	r.pending = make(map[string]*overlordRequest)
	var later []func()
	r.after = func(_ time.Duration, f func()) { later = append(later, f) }
	var sent []string
	r.overlordTX = func(text string) { sent = append(sent, text) }
	r.intn = func(int) int { return 1 }
	r.Response("Command, Raider 302, request Bravo", "Raider 302", "Command", nil, noMagVar)
	later[0]()
	if sent[0] != "Raider 302, Overlord Actual, you are cleared to engage Bravo, report copy." {
		t.Errorf("Overlord = %q", sent[0])
	}
	if got := r.Response("Command, Raider 302, copy cleared to engage", "Raider 302", "Command", nil, noMagVar); !strings.HasPrefix(got, "Raider 302, Command, copy, cleared to engage. Bravo, Krasnodar Pashkovsky airfield") {
		t.Errorf("brief = %q", got)
	}
	r.Response("Command, Venom 21, request Bravo", "Venom 21", "Command", nil, noMagVar)
	later[1]()
	if sent[1] != "Venom 21, Overlord Actual, negative, not cleared to engage, Raider 302 working Bravo. Request another area." {
		t.Errorf("negative = %q", sent[1])
	}
}

// Closing out a run in brevity: Winchester, Bingo, Joker, RTB.
func TestCommandBrevityCloseOut(t *testing.T) {
	saveMap := flagMap
	flagMap = "caucasus"
	defer func() { flagMap = saveMap }()
	commandRangeAreaBoard() // built for Caucasus
	b := commandRangeAreaBoard()
	b.mu.Lock()
	saveAreas, saveWork := b.areas, b.working
	b.areas, b.working = caucasusRangeAreas, map[string]rangeWork{"raider 302": {area: 0, at: b.now()}}
	b.mu.Unlock()
	defer func() { b.mu.Lock(); b.areas, b.working = saveAreas, saveWork; b.mu.Unlock() }()

	got := commandBrevityResponse("Command, Raider 302, off Alpha, Winchester, RTB", "Raider 302", "Command", nil, noMagVar)
	if !strings.HasPrefix(got, "Raider 302, Command, copy off Alpha, Winchester, RTB.") || !strings.HasSuffix(got, "Expect recovery handoff at three zero miles.") {
		t.Errorf("off + Winchester = %q", got)
	}
	if b.Leave("Raider 302") != "" {
		t.Error("Alpha still held after Winchester")
	}
	if got := commandBrevityResponse("Command, Raider 302, Winchester RTB", "Raider 302", "Command", nil, noMagVar); !strings.HasPrefix(got, "Raider 302, Command, copy Winchester, RTB.") {
		t.Errorf("Winchester RTB = %q", got)
	}
	if got := commandBrevityResponse("Command, Raider 302, Bingo, RTB", "Raider 302", "Command", nil, noMagVar); got != "Raider 302, Command, copy Bingo, RTB. Expect recovery handoff at three zero miles." {
		t.Errorf("Bingo = %q", got)
	}
	if got := commandBrevityResponse("Command, Raider 302, off target, RTB", "Raider 302", "Command", nil, noMagVar); got != "Raider 302, Command, copy off target, RTB. Expect recovery handoff at three zero miles." {
		t.Errorf("off target RTB = %q", got)
	}
	if got := commandBrevityResponse("Command, Raider 302, Joker", "Raider 302", "Command", nil, noMagVar); !strings.Contains(got, "Joker") {
		t.Errorf("Joker = %q", got)
	}
	// Plain RTB keeps commandResponse's reply; a fuel state keeps the fuel reply.
	if got := commandBrevityResponse("Command, Raider 302, RTB", "Raider 302", "Command", nil, noMagVar); got != "" {
		t.Errorf("plain RTB = %q, want it left to commandResponse", got)
	}
	if got := commandBrevityResponse("Command, Raider 302, bingo, state 1.8", "Raider 302", "Command", nil, noMagVar); got != "" {
		t.Errorf("bingo with state = %q", got)
	}
	if got := commandReply("Command, Raider 302, RTB", "Raider 302", "Command", nil); !strings.Contains(got, "RTB") && !strings.Contains(got, "returning to base") {
		t.Errorf("plain RTB through commandReply = %q", got)
	}
}
