package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
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
		// No "ball" (Day 2026-10-18): three-quarter mile and a state.
		"Raider 312, three-quarter mile, 5.4.": lso.CallRogerBall,
		"312, 3/4 mile, full, state 5.1.":      lso.CallRogerBall,
	} {
		said = nil
		_, resp := f.heard(in)
		if resp != netHandled || len(said) != 1 || said[0] != want {
			t.Errorf("heard(%q) = %q, said %v; want %q", in, resp, said, want)
		}
	}
	for _, in := range []string{"Paddles, Raider 302, radio check.", "Paddles, Raider 302 comp check."} {
		if _, resp := f.heard(in); resp != "Raider 302, Paddles, loud and clear." {
			t.Errorf("heard(%q) = %q", in, resp)
		}
	}
	// "302 in the break" (Day 2026-10-18): copy, call abeam.
	for in, want := range map[string]string{
		"302 in the break.":                  "302, Paddles, roger, call abeam.",
		"3-0-2 in the break.":                "302, Paddles, roger, call abeam.",
		"Paddles, Raider 302, in the break.": "Raider 302, Paddles, roger, call abeam.",
	} {
		said = nil
		if _, resp := f.heard(in); resp != want || len(said) != 0 {
			t.Errorf("heard(%q) = %q (said %v), want %q", in, resp, said, want)
		}
	}
	// "302 abeam" (operator 2026-10-18): Paddles contact, set up the ball call.
	for in, want := range map[string]string{
		"302 abeam.":                  "Paddles contact, 302. In the groove at three-quarter mile, call the ball.",
		"302, a beam.":                "Paddles contact, 302. In the groove at three-quarter mile, call the ball.",
		"Paddles, Raider 302, abeam.": "Paddles contact, Raider 302. In the groove at three-quarter mile, call the ball.",
		"3-0-2 beam, 5.8.":            "Paddles contact, 302. In the groove at three-quarter mile, call the ball.",
	} {
		said = nil
		if _, resp := f.heard(in); resp != want || len(said) != 0 {
			t.Errorf("heard(%q) = %q (said %v), want %q", in, resp, said, want)
		}
	}
	// No modex: chatter, not a call to answer.
	for _, in := range []string{"Abeam.", "In the break.", "Three-quarter mile, 5.4."} {
		said = nil
		if _, resp := f.heard(in); resp != "" || len(said) != 0 {
			t.Errorf("heard(%q) answered %q / %v", in, resp, said)
		}
	}
}

// "Red deck" / "green deck" from the jet that just landed: no reply, but the
// tracker holds the deck red until green (operator 2026-10-18).
func TestLSODeckCalls(t *testing.T) {
	var said []lso.Call
	f := newLSOFeed(lso.NimitzParams(), func(c lso.Call) { said = append(said, c) })
	for in, red := range map[string]bool{
		"Red deck.":             true,
		"Raider 312, red deck.": true,
		"312, read deck.":       true,
		"Green deck.":           false,
		"312, green deck.":      false,
		"Deck is green.":        false,
	} {
		f.tr.SetRedDeck(!red, time.Now())
		_, resp := f.heard(in)
		if red && resp != netHandled || !red && !strings.Contains(resp, "roger green deck, contact Deckboss") || len(said) != 0 {
			t.Errorf("heard(%q) = %q, said %v", in, resp, said)
		}
		if got := f.tr.RedDeck(time.Now()); got != red {
			t.Errorf("heard(%q): red deck = %v, want %v", in, got, red)
		}
	}
}

// "Green deck" hands the landed jet to Deckboss, by the callsign he gave,
// else his modex (operator 2026-10-18).
func TestLSOGreenDeckHandoff(t *testing.T) {
	defer func(v float64) { flagLSODeckbossFreq = v }(flagLSODeckbossFreq)
	flagLSODeckbossFreq = 128.6
	f := newLSOFeed(lso.NimitzParams(), func(lso.Call) {})
	for in, want := range map[string]string{
		"Paddles, Raider 302, green deck.": "Raider 302, Paddles, roger green deck, contact Deckboss, one two eight point six.",
		"302, green deck.":                 "302, Paddles, roger green deck, contact Deckboss, one two eight point six.",
	} {
		if _, resp := f.heard(in); resp != want {
			t.Errorf("heard(%q) = %q, want %q", in, resp, want)
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
	start := time.Now()
	clock := start
	tv.now = func() time.Time { return clock }
	// Log lines carry the recording's clock, so a pass can be matched to a
	// pilot's debrief (Tactical Paddles) by time.
	zerolog.TimestampFunc = func() time.Time { return clock }
	defer func() { zerolog.TimestampFunc = time.Now }()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "0,RecordingTime="); ok {
			if rt, err := time.Parse(time.RFC3339Nano, v); err == nil {
				start, clock = rt.Local(), rt.Local()
			}
		}
		if strings.HasPrefix(line, "#") {
			// A recording carries frame times; a raw capture may not.
			if s, err := strconv.ParseFloat(line[1:], 64); err == nil {
				clock = start.Add(time.Duration(s * float64(time.Second)))
			} else {
				clock = clock.Add(70 * time.Millisecond)
			}
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
	if !tv.deckFoul(now, "") {
		t.Error("jet that just trapped: want foul")
	}
	tv.onDeck["a1"] = [2]time.Time{now.Add(-2 * time.Minute), now}
	if tv.deckFoul(now, "") {
		t.Error("parked for 2 min: want clear")
	}
	// Stopped on the deck: Tacview sends nothing while it doesn't move.
	tv.onDeck["a1"] = [2]time.Time{now.Add(-20 * time.Second), now.Add(-15 * time.Second)}
	if !tv.deckFoul(now, "") {
		t.Error("stopped on the wires, no update for 15 s: want foul")
	}
	tv.onDeck["a1"] = [2]time.Time{now.Add(-20 * time.Minute), now.Add(-11 * time.Minute)}
	if tv.deckFoul(now, "") || len(tv.onDeck) != 0 {
		t.Error("no update for 11 min: want clear and forgotten")
	}
	// Over the ramp the landing jet itself is in the area (every trap in the
	// 2026-10-17 Day recording was waved off "foul deck" by itself).
	tv.onDeck["a2"] = [2]time.Time{now.Add(-time.Second), now}
	if tv.deckFoul(now, "a2") {
		t.Error("only the landing jet itself: want clear")
	}
}

// Raider 303 parked on the edge of the landing area, Day 2026-10-20
// 19:12-19:22, with Tacview updates seconds apart: once parked he stays
// parked, and the logged deck state doesn't flip with whose update came in.
func TestLSODeckFoulParkedSparse(t *testing.T) {
	tv := newLSOTacview(newLSOFeed(lso.NimitzParams(), func(lso.Call) {}))
	t0 := time.Date(2026, 10, 20, 19, 12, 30, 0, time.UTC)
	tv.onDeck["303"] = [2]time.Time{t0, t0}
	for i := 1; i <= 120; i++ { // ten minutes, an update every 5 s
		now := t0.Add(time.Duration(i) * 5 * time.Second)
		seen := tv.onDeck["303"]
		seen[1] = now
		tv.onDeck["303"] = seen
		foul := tv.deckFoul(now, "319")
		if parked := now.Sub(t0) >= foulMaxAge; foul == parked {
			t.Fatalf("t+%v: foul for 319 = %v", now.Sub(t0), foul)
		}
		if tv.deckFoul(now, "303") {
			t.Fatalf("t+%v: 303 fouls his own pass", now.Sub(t0))
		}
		if tv.lastFoul != !(now.Sub(t0) >= foulMaxAge) {
			t.Fatalf("t+%v: logged deck state %v flips with the caller", now.Sub(t0), tv.lastFoul)
		}
	}
}

// After the ball the newest call wins and the call just said is not said again.
func TestLSOQueueFlown20261020(t *testing.T) {
	q := make(chan lsoTX, 8)
	now := time.Now()
	for _, c := range []string{"A little high.", "A little high.", "Right for lineup.", "Come left."} {
		q <- lsoTX{text: c, queued: now}
	}
	first := <-q
	if got := lsoLatest(first, q); got.text != "Come left." || len(q) != 0 {
		t.Errorf("latest = %q, %d left", got.text, len(q))
	}
	wo := string(lso.FoulDeckCall("Raider 319"))
	q <- lsoTX{text: wo, queued: now}
	q <- lsoTX{text: "Come left.", queued: now}
	if got := lsoLatest(lsoTX{text: "Power!", queued: now}, q); got.text != wo {
		t.Errorf("wave-off lost behind %q", got.text)
	}
	last := lsoTX{text: "A little high.", queued: now}
	if !lsoRepeat(lsoTX{text: "A little high."}, last, now.Add(time.Second)) {
		t.Error("same call 1 s after: want dropped")
	}
	if lsoRepeat(lsoTX{text: "A little high."}, last, now.Add(3*time.Second)) {
		t.Error("same call 3 s after: want said")
	}
	if lsoRepeat(lsoTX{text: wo}, lsoTX{text: wo, queued: now}, now) {
		t.Error("wave-off repeated: want said")
	}
}
