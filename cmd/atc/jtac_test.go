package main

import (
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
}

func (f *fakePicture) Contact(string) (orb.Point, float64, bool) { return f.pos, f.hdgTrue, f.have }
func (f *fakePicture) WeaponReleasedNear(orb.Point, float64, time.Time) bool {
	return f.released
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
	if got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic); !strings.Contains(got, "9-line target 1, Enguri Bridge") {
		t.Fatalf("first check-in = %q", got)
	}
	// Re-checking in mid-run repeats the same card.
	if got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic); !strings.Contains(got, "target 1,") {
		t.Errorf("repeat check-in = %q, want the same target", got)
	}
	b.Response("Command, Raider 11, request tasking", "Raider 11", "Command")
	if got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic); !strings.Contains(got, "9-line target 2, Rioni Rail Bridge") {
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
	for _, want := range []string{"Raider 11, Hitman one one, type 2 control, 9-line target 1, Enguri Bridge, as fragged.",
		"Elevation 445", "final attack heading three-zero-niner to three-four-niner", "egress back to Anvil, angels 15", "Read back."} {
		if !strings.Contains(got, want) {
			t.Errorf("check-in = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "bears") {
		t.Errorf("check-in at the IP should not give the IP's bearing: %q", got)
	}

	if got := b.JTACResponse("Hitman, Raider 11, copy", "Raider 11", hitman, pic); got != "Raider 11, Hitman one one, good readback. Report IP inbound." {
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
