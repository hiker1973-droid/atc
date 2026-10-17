package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/lso"
)

func TestLSOBallCalls(t *testing.T) {
	var said []lso.Call
	f := newLSOFeed(lso.NimitzParams(), func(c lso.Call) { said = append(said, c) })
	for in, want := range map[string]lso.Call{
		"302, Hornet ball, 5.5.": lso.CallRogerBall,
		"302 Hornetball 6.1.":    lso.CallRogerBall,
		"V0-2 Hornetball 6.2.":   lso.CallRogerBall,
		"312, Hornet bolt, 4.4.": lso.CallRogerBall, // Whisper's "bolt"
		"302, Clara, 5.0.":       lso.CallClara,
	} {
		said = nil
		_, resp := f.heard(in)
		if resp != netHandled || len(said) != 1 || said[0] != want {
			t.Errorf("heard(%q) = %q, said %v; want %q", in, resp, said, want)
		}
	}
	if _, resp := f.heard("Paddles, Raider 302, radio check."); resp != "Raider 302, Paddles, loud and clear." {
		t.Errorf("radio check reply = %q", resp)
	}
	for _, in := range []string{"Red deck.", "302 beam.", "Green deck."} {
		said = nil
		if _, resp := f.heard(in); resp != "" || len(said) != 0 {
			t.Errorf("heard(%q) answered %q / %v", in, resp, said)
		}
	}
}

// A carrier steaming on 100 at 15 m/s and a player Hornet flying the groove,
// as ACMI lines: the parser finds the boat, works out its heading and speed,
// and the groove produces calls (the jet is flown high, so "You're high").
func TestLSOTacviewGroove(t *testing.T) {
	var said []lso.Call
	f := newLSOFeed(lso.NimitzParams(), func(c lso.Call) { said = append(said, c) })
	tv := newLSOTacview(f)
	clock := time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)
	tv.now = func() time.Time { return clock }
	tv.handleLine("0,ReferenceLongitude=41,ReferenceLatitude=42")
	tv.handleLine("a02,T=0|0|0,Type=Sea+Watercraft+AircraftCarrier,Name=CVN_72,Pilot=ABE")
	tv.handleLine("3d04,T=0|0|500,Type=Air+FixedWing,Name=FA-18C_hornet,Pilot=Raider 302 | Thrash")
	p := lso.NimitzParams()
	hdg := 100.0
	axis := (hdg + p.AngledDeckDeg) * math.Pi / 180
	cosLat := math.Cos(42 * math.Pi / 180)
	astern := 2300.0
	for i := 0; i < 120; i++ {
		clock = clock.Add(210 * time.Millisecond)
		d := 15 * float64(i) * 0.21
		se, sn := math.Sin(hdg*math.Pi/180), math.Cos(hdg*math.Pi/180)
		shipE, shipN := se*d, sn*d
		tv.handleLine(fmt.Sprintf("a02,T=%.7f|%.7f|0", shipE/(111320*cosLat), shipN/110540))
		// Landing point is (stern + 3 wire) along the ship's heading.
		land := p.SternM + p.WiresFromSternM[2]
		le, ln := shipE+se*land, shipN+sn*land
		je, jn := le-math.Sin(axis)*astern, ln-math.Cos(axis)*astern
		h := math.Tan((p.GlideslopeDeg+1.6)*math.Pi/180)*astern + p.DeckHeightM + p.HookBelowRefM
		tv.handleLine(fmt.Sprintf("3d04,T=%.7f|%.7f|%.1f|0|4|%.1f", je/(111320*cosLat), jn/110540, h, hdg+p.AngledDeckDeg))
		astern -= 70 * 0.21
	}
	s, ok := tv.ship()
	if !ok || math.Abs(s.HeadingDeg-hdg) > 1 || math.Abs(s.SpeedMps-15) > 0.5 {
		t.Fatalf("ship = %+v %v, want heading 100 at 15 m/s", s, ok)
	}
	high := false
	for _, c := range said {
		if c == lso.CallHigh || c == lso.CallLittleHigh {
			high = true
		}
	}
	if !high {
		t.Errorf("said %v, want a high call", said)
	}
}

// LSO_REPLAY=<acmi capture> replays a real Tacview stream through the parser
// (e.g. the 20 s Day capture of 2026-10-17): it must find the carrier and not
// crash; any passes are logged.
func TestLSOReplay(t *testing.T) {
	path := os.Getenv("LSO_REPLAY")
	if path == "" {
		t.Skip("LSO_REPLAY not set")
	}
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	f := newLSOFeed(lso.NimitzParams(), func(c lso.Call) { t.Logf("call %q", c) })
	tv := newLSOTacview(f)
	clock := time.Now()
	tv.now = func() time.Time { return clock }
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			clock = clock.Add(70 * time.Millisecond)
		}
		tv.handleLine(line)
	}
	if tv.carrierID == "" {
		t.Fatal("carrier not found")
	}
	s, ok := tv.ship()
	t.Logf("carrier %s: %+v (%v), in groove %v", tv.carrierID, s, ok, f.tr.InGroove())
}

// An aircraft on the landing area fouls the deck; one that sits there past
// foulMaxAge is parked and does not.
func TestLSODeckFoul(t *testing.T) {
	tv := newLSOTacview(newLSOFeed(lso.NimitzParams(), func(lso.Call) {}))
	now := time.Date(2026, 10, 17, 15, 0, 0, 0, time.UTC)
	tv.onDeck["a1"] = [2]time.Time{now.Add(-10 * time.Second), now}
	if !tv.deckFoul(now) {
		t.Error("jet that just trapped: want foul")
	}
	tv.onDeck["a1"] = [2]time.Time{now.Add(-2 * time.Minute), now}
	if tv.deckFoul(now) {
		t.Error("parked for 2 min: want clear")
	}
	tv.onDeck["a1"] = [2]time.Time{now.Add(-20 * time.Second), now.Add(-5 * time.Second)}
	if tv.deckFoul(now) || len(tv.onDeck) != 0 {
		t.Error("left the area 5 s ago: want clear and forgotten")
	}
}
