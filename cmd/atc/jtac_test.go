package main

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
)

// fakePicture is a Tacview picture with one jet in it.
type fakePicture struct {
	pos      orb.Point
	hdgTrue  float64
	have     bool
	released bool
	ground   []tacviewGround
	impacts  []tacviewImpact
}

func (f *fakePicture) Contact(string) (orb.Point, float64, bool) { return f.pos, f.hdgTrue, f.have }
func (f *fakePicture) WeaponReleasedNear(orb.Point, float64, time.Time) bool {
	return f.released
}
func (f *fakePicture) GroundObjects() []tacviewGround { return f.ground }
func (f *fakePicture) Impacts(since time.Time) []tacviewImpact {
	var out []tacviewImpact
	for _, i := range f.impacts {
		if !i.At.Before(since) {
			out = append(out, i)
		}
	}
	return out
}

// place puts the jet nm miles from ref on magnetic bearing brgMag (from ref),
// heading hdgMag, using target 1's variation.
func (f *fakePicture) place(ref orb.Point, brgMag, nm, hdgMag float64) {
	const mv = 7.21
	b := (brgMag + mv) * math.Pi / 180
	lat := ref[1] + nm*math.Cos(b)/60
	lon := ref[0] + nm*math.Sin(b)/(60*math.Cos(ref[1]*math.Pi/180))
	f.pos, f.hdgTrue, f.have = orb.Point{lon, lat}, math.Mod(hdgMag+mv, 360), true
}

const hitman = "Hitman one one"

func newJTACBoard(t *testing.T) (*taskingBoard, *time.Time) {
	t.Helper()
	b, clock := newTestTaskingBoard(caucasusTaskingTargets) // always target 1, Anvil
	b.jtac = &jtacNet{Spoken: hitman, FreqMHz: 61.0}
	return b, clock
}

// With a JTAC, Command names no target: it hands the flight to Hitman, who
// draws one at check-in. Coming back to Command for more gets a new target.
func TestJTACCommandHandsOff(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	want := "Raider 11, Command, contact Hitman one one on six one point zero zero zero for tasking."
	if got := b.Response("Command, Raider 11, request tasking", "Raider 11", "Command"); got != want {
		t.Errorf("request = %q, want %q", got, want)
	}
	if got := b.Response("Command, Raider 11, copy", "Raider 11", "Command"); got != "Raider 11, Command, switch Hitman one one, good hunting." {
		t.Errorf("Command readback = %q", got)
	}
	if got := b.Response("Command, Raider 11, copy", "Raider 11", "Command"); got != "" {
		t.Errorf("second copy = %q, want silence", got)
	}
	if got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic); !strings.Contains(got, "target 1, Enguri Bridge") {
		t.Fatalf("first check-in = %q", got)
	}
	// Re-checking in mid-run repeats the same card.
	if got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic); !strings.Contains(got, "target 1,") {
		t.Errorf("repeat check-in = %q, want the same target", got)
	}
	b.Response("Command, Raider 11, request tasking", "Raider 11", "Command")
	if got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic); !strings.Contains(got, "target 2, Rioni Rail Bridge") {
		t.Errorf("check-in after a new Command request = %q, want the next target", got)
	}
}

// Asking Hitman directly for more after a run also draws a new target.
func TestJTACRequestTaskingAfterRun(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 11, checking in", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, in from the south", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, off", "Raider 11", hitman, pic)
	if got := b.JTACResponse("Hitman, Raider 11, request tasking", "Raider 11", hitman, pic); !strings.Contains(got, "target 2,") {
		t.Errorf("request after run = %q, want target 2", got)
	}
}

// The whole run with a picture: check-in, readback, IP inbound at the IP,
// in call inside the window, cleared hot, off.
func TestJTACFullRun(t *testing.T) {
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	b.Response("Command, Raider 11, request tasking", "Raider 11", "Command")

	pic.place(tgt.IPPos, 0, 1, 90) // at Anvil
	got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic)
	if want := "Raider 11, Hitman one one, target 1, Enguri Bridge, IP Anvil. Read back."; got != want {
		t.Errorf("check-in = %q, want %q", got, want)
	}

	// Our own brief heard back must not pass as the readback.
	if got := b.JTACResponse("Raider 11, Hitman 1-1, target 1, Enguri Bridge, IP Anvil, read back.", "Raider 11", hitman, pic); got != "" {
		t.Errorf("echoed brief = %q, want no reply", got)
	}

	if got := b.JTACResponse("Hitman, Raider 11, target 1, Anvil", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, good readback. Report IP inbound." {
		t.Errorf("readback = %q", got)
	}

	pic.place(tgt.IPPos, 0, 1, 329)
	if got := b.JTACResponse("Hitman, Raider 11, IP inbound", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, continue." {
		t.Errorf("IP inbound = %q", got)
	}

	// 5 nm short of the target on the run-in (target bears 329 = jet is on 149 from it).
	pic.place(tgt.TgtPos, 149, 5, 329)
	if got := b.JTACResponse("Hitman, Raider 11, in from the southeast", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, cleared hot." {
		t.Errorf("in call = %q", got)
	}

	pic.place(tgt.TgtPos, 149, 1, 200)
	if got := b.JTACResponse("Hitman, Raider 11, off", "Raider 11", hitman, pic); !strings.Contains(got, "copy off. Egress back to Anvil, angels 15.") {
		t.Errorf("off = %q", got)
	}
}

// An in call outside the final attack heading is waved off.
func TestJTACInOutsideWindowAborts(t *testing.T) {
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 11, checking in", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic)

	pic.place(tgt.TgtPos, 90, 5, 270) // coming in from the east, heading 270: window is 309-349
	got := b.JTACResponse("Hitman, Raider 11, in from the east", "Raider 11", hitman, pic)
	if !strings.Contains(got, "abort, abort, abort. Outside final attack heading") {
		t.Errorf("in call outside window = %q, want abort", got)
	}
	// After the wave-off the flight goes round again from the IP.
	pic.place(tgt.TgtPos, 149, 5, 335)
	if got := b.JTACResponse("Hitman, Raider 11, in from the southeast", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, cleared hot." {
		t.Errorf("re-attack in call = %q", got)
	}
}

// IP inbound away from the IP is refused with where the IP is.
func TestJTACInboundNotAtIP(t *testing.T) {
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 11, checking in", "Raider 11", hitman, pic)
	pic.place(tgt.IPPos, 270, 20, 90) // 20 nm west of Anvil
	got := b.JTACResponse("Hitman, Raider 11, IP inbound", "Raider 11", hitman, pic)
	if !strings.Contains(got, "negative, you are not at Anvil. Anvil bears zero-niner-zero for 20.") {
		t.Errorf("IP inbound far from IP = %q", got)
	}
}

// Check-in far from the IP gets the 9-line plus where the IP is.
func TestJTACCheckInGivesIPBearing(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	pic.place(caucasusTaskingTargets[0].IPPos, 180, 30, 0) // 30 nm south of Anvil
	got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic)
	if !strings.Contains(got, "Anvil bears zero-zero-zero for 30.") {
		t.Errorf("check-in far out = %q", got)
	}
}

// No Tacview picture: the exchange still runs and clears hot.
func TestJTACNoPictureClearsHot(t *testing.T) {
	b, _ := newJTACBoard(t)
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 11, checking in", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic)
	if got := b.JTACResponse("Hitman, Raider 11, IP inbound", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, continue." {
		t.Errorf("IP inbound = %q", got)
	}
	if got := b.JTACResponse("Hitman, Raider 11, in from the south", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, cleared hot." {
		t.Errorf("in call = %q", got)
	}
}

// An in call before any 9-line is not cleared.
func TestJTACInWithoutCheckIn(t *testing.T) {
	b, _ := newJTACBoard(t)
	got := b.JTACResponse("Hitman, Raider 11, in from the south", "Raider 11", hitman, &fakePicture{})
	if !strings.Contains(got, "no 9-line passed") {
		t.Errorf("in without check-in = %q", got)
	}
}

func clearedFlight(t *testing.T) (*taskingBoard, *fakePicture) {
	t.Helper()
	b, _ := newJTACBoard(t)
	tgt := caucasusTaskingTargets[0]
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 11, checking in", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic)
	pic.place(tgt.TgtPos, 149, 6, 329)
	if got := b.JTACResponse("Hitman, Raider 11, in from the southeast", "Raider 11", hitman, pic); !strings.Contains(got, "cleared hot") {
		t.Fatalf("setup in call = %q", got)
	}
	return b, pic
}

// After cleared hot, a run that swings outside the window while still
// pointed at the target is aborted, but only after two samples.
func TestJTACWatchAbortsOffAxisRun(t *testing.T) {
	b, pic := clearedFlight(t)
	tgt := caucasusTaskingTargets[0]
	// 5 nm east of the target, nose on it: heading 270, outside 309-349 + 10.
	pic.place(tgt.TgtPos, 90, 5, 270)
	if out := b.jtacWatch(hitman, pic); len(out) != 0 {
		t.Fatalf("first sample aborted already: %v", out)
	}
	out := b.jtacWatch(hitman, pic)
	if len(out) != 1 || !strings.HasPrefix(out[0], "Raider 11, Hitman one one, abort, abort, abort.") {
		t.Fatalf("second sample = %v, want one abort", out)
	}
	if out := b.jtacWatch(hitman, pic); len(out) != 0 {
		t.Errorf("abort repeated: %v", out)
	}
}

// Pulling off (target well off the nose) and a seen release are not aborts.
func TestJTACWatchIgnoresPullOffAndRelease(t *testing.T) {
	b, pic := clearedFlight(t)
	tgt := caucasusTaskingTargets[0]
	pic.place(tgt.TgtPos, 149, 4, 60) // turning away hard
	for i := 0; i < 3; i++ {
		if out := b.jtacWatch(hitman, pic); len(out) != 0 {
			t.Fatalf("pull-off aborted: %v", out)
		}
	}

	b, pic = clearedFlight(t)
	pic.released = true
	pic.place(tgt.TgtPos, 90, 5, 270)
	for i := 0; i < 3; i++ {
		if out := b.jtacWatch(hitman, pic); len(out) != 0 {
			t.Fatalf("aborted after release: %v", out)
		}
	}
}

func TestJTACCallsign(t *testing.T) {
	for in, want := range map[string]string{
		"Hitman 1-1, Raider 11, checking in":     "Raider 11",
		"Hitman 1-1 Raider 11 in from the north": "Raider 11",
		"Hitman, Raider 11, IP inbound":          "Raider 11",
		"Hitman one one, Venom 21 off":           "Venom 21",
		"Hitman 1-1, Raider 11 in":               "Raider 11",
		"Hitman 1-1 Raider 11 abort abort abort": "Raider 11",
	} {
		if got := jtacCallsign(in); got != want {
			t.Errorf("jtacCallsign(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInWindow(t *testing.T) {
	cases := []struct {
		hdg, lo, hi, slack float64
		want               bool
	}{
		{329, 309, 349, 0, true}, {300, 309, 349, 0, false}, {300, 309, 349, 10, true},
		{5, 350, 30, 0, true}, {340, 350, 30, 0, false}, {345, 350, 30, 10, true},
	}
	for _, c := range cases {
		if got := inWindow(c.hdg, c.lo, c.hi, c.slack); got != c.want {
			t.Errorf("inWindow(%v, %v-%v, ±%v) = %v", c.hdg, c.lo, c.hi, c.slack, got)
		}
	}
}

// The tracker keeps heading and notes weapon releases.
func TestMiniTacviewHeadingAndWeapons(t *testing.T) {
	store := newTacviewPositions()
	p := newMiniTacviewParser(store)
	p.handleLine("0,ReferenceLongitude=41,ReferenceLatitude=42")
	p.handleLine("a1,T=0.9|0.4|5000|0|0|336|1|2|335.5,Type=Air+FixedWing,Pilot=Raider 11,Name=FA-18C_hornet,Coalition=Enemies")
	info := store.Info("Raider 11")
	if !info.HasHeading || info.HeadingDeg != 335.5 {
		t.Errorf("heading = %v %v, want 335.5", info.HeadingDeg, info.HasHeading)
	}
	pos, _ := store.Get("Raider 11")
	p.handleLine("b2,T=0.9|0.4|4900|0|0|336,Type=Weapon+Bomb,Name=GBU-31")
	if _, ok := store.Get("GBU-31"); ok {
		t.Error("a bomb was stored as a contact")
	}
	if !store.WeaponReleasedNear(pos, 1.5, time.Now().Add(-time.Minute)) {
		t.Error("release not seen")
	}
}

func TestSRSModulation(t *testing.T) {
	for f, want := range map[float64]string{61.0: "FM", 60.0: "FM", 30.0: "FM", 128.6: "AM", 282.0: "AM", 306.2: "AM"} {
		if got := srsModulationName(f); got != want {
			t.Errorf("srsModulationName(%v) = %s, want %s", f, got, want)
		}
	}
}

// ── BDA ──────────────────────────────────────────────────────────────────────

// offset moves p n metres north and e metres east.
func offset(p orb.Point, n, e float64) orb.Point {
	return orb.Point{p[0] + e/(111320*math.Cos(p[1]*math.Pi/180)), p[1] + n/111320}
}

// groundAt puts every placed object of a target in the picture, alive, a few
// metres off where the builder put it.
func groundAt(num int) []tacviewGround {
	var g []tacviewGround
	for i, o := range caucasusBDA[num].Objects {
		g = append(g, tacviewGround{ID: fmt.Sprint(100 + i), Pos: offset(o.Pos, 4, -3), Alive: true})
	}
	return g
}

// A clean pass on the fuel depot: Hitman waits for the bombs to settle, then
// gives impacts and the kill count, priority objects first, once.
func TestJTACBDAAfterPass(t *testing.T) {
	b, clock := newJTACBoard(t)
	b.intn = func(n int) int { return 2 } // third free target = 3, Fuel Depot
	tgt := caucasusTaskingTargets[2]
	pic := &fakePicture{}
	b.JTACResponse("Hitman, Raider 11, checking in", "Raider 11", hitman, pic)
	b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic) // no picture: cleared hot unchecked
	if got := b.JTACResponse("Hitman, Raider 11, in from the east", "Raider 11", hitman, pic); !strings.Contains(got, "cleared hot") {
		t.Fatalf("in call = %q", got)
	}
	pic.ground = groundAt(3)
	pic.ground[0].Alive, pic.ground[1].Alive, pic.ground[8].Alive = false, false, false // two large tanks, a fuel truck
	*clock = clock.Add(20 * time.Second)
	pic.impacts = []tacviewImpact{
		{Pos: offset(tgt.TgtPos, 30, 0), At: *clock, Shooter: "Raider 11"},
		{Pos: offset(tgt.TgtPos, 0, 200), At: *clock, Shooter: "Raider 11"},
		{Pos: offset(tgt.TgtPos, 0, 0), At: *clock, Shooter: "Venom 21"}, // someone else's
	}
	if out := b.jtacWatch(hitman, pic); len(out) != 0 {
		t.Fatalf("BDA before the bombs settled: %v", out)
	}
	*clock = clock.Add(bdaSettle)
	out := b.jtacWatch(hitman, pic)
	want := "Raider 11, Hitman one one, BDA. 2 impacts, closest 30 metres north of the aim point. Large tanks, 2 of 4 destroyed. 3 of 14 on target destroyed."
	if len(out) != 1 || out[0] != want {
		t.Fatalf("BDA = %v\nwant %q", out, want)
	}
	if out := b.jtacWatch(hitman, pic); len(out) != 0 {
		t.Errorf("BDA repeated: %v", out)
	}
	// Asked for again, it is given again.
	if got := b.JTACResponse("Hitman, Raider 11, request BDA", "Raider 11", hitman, pic); got != want {
		t.Errorf("request BDA = %q", got)
	}
}

func TestBDAText(t *testing.T) {
	t1, t2, t4 := caucasusTaskingTargets[0], caucasusTaskingTargets[1], caucasusTaskingTargets[3]
	now := time.Now()
	hit := func(p orb.Point) []tacviewImpact { return []tacviewImpact{{Pos: p, At: now}} }

	// Bridge: no damage picture, the checkpoint is counted.
	g := groundAt(1)
	g[0].Alive = false
	got := bdaFor("Raider 11", hitman, t1, g, hit(t1.TgtPos))
	if want := "Raider 11, Hitman one one, BDA. Impact on the aim point. Unable to assess the bridge. Checkpoint, 1 of 4 destroyed."; got != want {
		t.Errorf("Enguri = %q", got)
	}
	// Rioni: an impact on the road bridge is called out.
	got = bdaFor("Raider 11", hitman, t2, nil, hit(rioniRoadBridge))
	if !strings.Contains(got, "Impact on the no-strike road bridge.") || !strings.Contains(got, "Unable to assess the bridge.") {
		t.Errorf("Rioni = %q", got)
	}
	// Command post with nothing in the picture: impacts only.
	got = bdaFor("Raider 11", hitman, t4, nil, hit(offset(t4.TgtPos, -150, 0)))
	if want := "Raider 11, Hitman one one, BDA. Impact 150 metres south of the aim point. Unable to assess damage."; got != want {
		t.Errorf("no picture = %q", got)
	}
	// Single primary object, then everything down.
	g = groundAt(4)
	g[0].Alive = false
	if got := bdaKillText(caucasusBDA[4], g); got != "Command building destroyed. 1 of 14 on target destroyed." {
		t.Errorf("command building = %q", got)
	}
	for i := range g {
		g[i].Alive = false
	}
	if got := bdaKillText(caucasusBDA[4], g); got != "Target destroyed. Good work." {
		t.Errorf("all down = %q", got)
	}
}

// Every way a kill can show up in the stream is taken, and a new mission
// clears the picture.
func TestMiniTacviewGroundKillsAndImpacts(t *testing.T) {
	store := newTacviewPositions()
	p := newMiniTacviewParser(store)
	p.handleLine("0,ReferenceTime=2026-10-14T10:00:00Z")
	p.handleLine("0,ReferenceLongitude=41,ReferenceLatitude=42")
	p.handleLine("a1,T=0.80|0.36|3000|0|0|300|1|2|300,Type=Air+FixedWing,Pilot=Raider 11,Name=FA-18C_hornet")
	p.handleLine("g1,T=0.806|0.365|10,Type=Ground+Static+Building,Name=Tank")
	p.handleLine("g2,T=0.807|0.365|10,Type=Ground+Static+Building,Name=Tank")
	p.handleLine("g3,T=0.808|0.365|10,Type=Ground+Heavy+Armor+Vehicle,Name=ATZ-10")
	p.handleLine("g4,T=0.809|0.365|10,Type=Ground+Static+Building,Name=Tank")
	if n := len(store.GroundObjects()); n != 4 {
		t.Fatalf("ground objects = %d, want 4", n)
	}
	p.handleLine("-g1")                   // removed
	p.handleLine("0,Event=Destroyed|g2|") // event
	p.handleLine("g3,Name=ATZ-10 wreck")  // turned into a wreck (no position change)
	p.handleLine("g3,T=0.808|0.365|10")
	alive := map[string]bool{}
	for _, g := range store.GroundObjects() {
		alive[g.ID] = g.Alive
	}
	if alive["g1"] || alive["g2"] || alive["g3"] || !alive["g4"] {
		t.Errorf("alive = %v, want only g4", alive)
	}
	// A bomb with a Parent, and one without (named by proximity).
	p.handleLine("w1,T=0.80|0.36|2900,Type=Weapon+Bomb,Name=GBU-31,Parent=a1")
	p.handleLine("w1,T=0.806|0.365|20")
	p.handleLine("-w1")
	p.handleLine("w2,T=0.801|0.361|2900,Type=Weapon+Bomb,Name=Mk-84")
	p.handleLine("w2,T=0.807|0.364|15")
	p.handleLine("0,Event=Destroyed|w2|")
	p.handleLine("-w2") // second report of the same end: ignored
	imps := store.Impacts(time.Time{})
	if len(imps) != 2 || imps[0].Shooter != "Raider 11" || imps[1].Shooter != "Raider 11" {
		t.Fatalf("impacts = %+v", imps)
	}
	if d := metresBetween(imps[0].Pos, orb.Point{41.806, 42.365}); d > 1 {
		t.Errorf("impact %f m from its last position", d)
	}
	// Same mission resent (reconnect): kills stay. New mission: cleared.
	p.handleLine("0,ReferenceTime=2026-10-14T10:00:00Z")
	if len(store.Impacts(time.Time{})) != 2 {
		t.Error("reconnect cleared the impacts")
	}
	p.handleLine("0,ReferenceTime=2026-10-14T12:00:00Z")
	if len(store.GroundObjects()) != 0 || len(store.Impacts(time.Time{})) != 0 {
		t.Error("new mission kept the old picture")
	}
}
