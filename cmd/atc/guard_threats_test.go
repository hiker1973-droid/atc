package main

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

// acmi feeds ACMI lines to a parser over a fresh store.
func acmiStore(t *testing.T, lines ...string) *tacviewPositions {
	t.Helper()
	store := newTacviewPositions()
	p := newMiniTacviewParser(store)
	for _, l := range lines {
		p.handleLine(l)
	}
	return store
}

// at stamps every contact as seen at t (the tests run a fake clock).
func at(cs []tacviewContact, t time.Time) []tacviewContact {
	for i := range cs {
		cs[i].Seen = t
	}
	return cs
}

const (
	// A Hornet over the Norwegian coast, a Flanker 30 nm east, an SA-6 battery 9 nm east.
	acmiPlayer  = "300,T=14.0|67.0|6096,Pilot=Raider 302 |Jedi,Name=FA-18C_hornet,Type=Air+FixedWing,Color=Blue,Group=Raider-3"
	acmiFlanker = "100,T=14.0|67.0|7620,Name=Su-27,Type=Air+FixedWing,Color=Red,Group=RED SU-27 CAP-1"
	acmiKub1    = "200,T=14.4|67.0|10,Name=Kub 2P25 ln,Type=Ground+Vehicle,Color=Red,Group=SAM 1"
	acmiKub2    = "201,T=14.41|67.0|10,Name=Kub 1S91 str,Type=Ground+Vehicle,Color=Red,Group=SAM 1"
	acmiPatriot = "202,T=14.3|67.0|10,Name=Patriot ln,Type=Ground+Vehicle,Color=Blue,Group=Pat"
)

func TestContactsParsed(t *testing.T) {
	store := acmiStore(t, acmiPlayer, acmiFlanker, acmiKub1, acmiKub2, acmiPatriot)
	cs := store.Contacts(0)
	if len(cs) != 5 {
		t.Fatalf("contacts = %d, want 5", len(cs))
	}
	var player, flanker int
	for _, c := range cs {
		if c.IsPlayer() && c.Callsign() == "Raider 302" && c.Group == "Raider-3" {
			player++
		}
		if c.IsAir() && c.IsRed() && c.Name == "Su-27" && c.Group == "RED SU-27 CAP-1" {
			flanker++
		}
	}
	if player != 1 || flanker != 1 {
		t.Errorf("player=%d flanker=%d", player, flanker)
	}
	// A kill leaves the table.
	p := newMiniTacviewParser(store)
	p.handleLine("-100")
	for _, c := range store.Contacts(0) {
		if c.ID == "100" {
			t.Error("a removed object stayed in the contacts")
		}
	}
}

func TestSAMSitesGroupBatteries(t *testing.T) {
	store := acmiStore(t, acmiKub1, acmiKub2, acmiPatriot)
	sites := samSites(store.Contacts(0))
	if len(sites) != 1 || sites[0].Label != "SA-6" || sites[0].N != 2 {
		t.Errorf("sites = %+v, want one SA-6 battery of 2 (blue Patriot ignored)", sites)
	}
}

func TestGuardWarnings(t *testing.T) {
	saveMap := flagMap
	defer func() { flagMap = saveMap }()
	flagMap = "kola"

	store := acmiStore(t, acmiPlayer, acmiKub1, acmiKub2)
	w := newGuardWatch("Guard")
	now := time.Now()
	got := w.scan(now, store.Contacts(0))
	if len(got) != 1 || !strings.Contains(got[0].Text, "SAM threat, SA-6") || got[0].Callsign != "Raider 302" {
		t.Fatalf("first scan = %+v", got)
	}
	if again := w.scan(now.Add(time.Second), store.Contacts(0)); len(again) != 0 {
		t.Errorf("a pilot got two warnings inside the gap: %+v", again)
	}
	// 25 s on: the SAM is still on cooldown; a Flanker has appeared.
	store2 := acmiStore(t, acmiPlayer, acmiKub1, acmiKub2, "100,T=14.0|67.5|7620,Name=Su-27,Type=Air+FixedWing,Color=Red,Group=G")
	got = w.scan(now.Add(25*time.Second), at(store2.Contacts(0), now.Add(25*time.Second)))
	if len(got) != 1 || !strings.Contains(got[0].Text, "unidentified aircraft") || !strings.Contains(got[0].Text, "angels 25") {
		t.Fatalf("second scan = %+v", got)
	}
	// And not again until its cooldown runs out.
	if again := w.scan(now.Add(60*time.Second), at(store2.Contacts(0), now.Add(60*time.Second))); len(again) != 0 {
		t.Errorf("repeated inside the cooldown: %+v", again)
	}
	// Inside 15 nm it is called once more.
	store3 := acmiStore(t, acmiPlayer, "100,T=14.0|67.2|7620,Name=Su-27,Type=Air+FixedWing,Color=Red,Group=G")
	got = w.scan(now.Add(90*time.Second), at(store3.Contacts(0), now.Add(90*time.Second)))
	if len(got) != 1 || !strings.Contains(got[0].Text, "unidentified aircraft") {
		t.Errorf("near call = %+v", got)
	}
}

func TestGuardWarnsOnlyAirbornePlayers(t *testing.T) {
	flagMapSave := flagMap
	defer func() { flagMap = flagMapSave }()
	flagMap = "kola"
	// On the ramp (alt 100 m = 328 ft), and an AI blue jet with no "|".
	store := acmiStore(t,
		"300,T=14.0|67.0|100,Pilot=Raider 302 |Jedi,Name=FA-18C_hornet,Type=Air+FixedWing,Color=Blue,Group=Raider-3",
		"301,T=14.0|67.0|6096,Pilot=Mauler11,Name=F-16C_50,Type=Air+FixedWing,Color=Blue,Group=Mauler",
		acmiKub1)
	if got := newGuardWatch("Guard").scan(time.Now(), store.Contacts(0)); len(got) != 0 {
		t.Errorf("warned a jet on the ramp or an AI jet: %+v", got)
	}
	// A parked red Flanker (80 m up) is no unidentified aircraft.
	parked := acmiStore(t, acmiPlayer, "100,T=14.0|67.2|80,Name=Su-27,Type=Air+FixedWing,Color=Red,Group=SEV1 Flanker")
	if got := newGuardWatch("Guard").scan(time.Now(), parked.Contacts(0)); len(got) != 0 {
		t.Errorf("warned about a parked jet: %+v", got)
	}
}

func TestLuaForOrders(t *testing.T) {
	for _, lua := range []string{luaTurn(`RED "SU-27" CAP`, 277.5, 60), luaFollow("RED SU-27 CAP-1", "Raider-3"), luaLand("RED SU-27 CAP-1", "Bodo")} {
		if !strings.Contains(lua, "setTask") || !strings.HasSuffix(strings.TrimSpace(lua), `return "ok"`) {
			t.Errorf("Lua missing setTask / ok:\n%s", lua)
		}
	}
	if q := luaQuote(`a"b\c`); q != `"a\"b\\c"` {
		t.Errorf("luaQuote = %s", q)
	}
	if !strings.Contains(luaTurn("X", 90, 60), "111120") { // 60 nm in metres
		t.Error("turn distance not in metres")
	}
}

// fakeAI records the orders it is given.
type fakeAI struct{ calls []string }

func (f *fakeAI) Turn(_ context.Context, g string, hdg, nm float64) error {
	f.calls = append(f.calls, "turn "+g+" "+strconv.Itoa(int(hdg+0.5)))
	return nil
}
func (f *fakeAI) Follow(_ context.Context, g, l string) error {
	f.calls = append(f.calls, "follow "+g+" "+l)
	return nil
}
func (f *fakeAI) Land(_ context.Context, g, ab string) error {
	f.calls = append(f.calls, "land "+g+" "+ab)
	return nil
}

func TestHailAndOrders(t *testing.T) {
	saveMap := flagMap
	defer func() { flagMap = saveMap }()
	flagMap = "kola"
	store := acmiStore(t, acmiPlayer, "100,T=14.0|67.4|7620,Name=Su-27,Type=Air+FixedWing,Color=Red,Group=RED SU-27 CAP-1")
	for _, c := range store.Contacts(0) {
		if c.IsPlayer() {
			store.Set("Raider 302", c.Pos)
		}
	}
	ai := &fakeAI{}
	h := newGuardHail(store, ai)
	now := time.Now()

	// An order with no hail is Guard's to answer.
	if r := h.handle("Raider 302, turn right heading 2 7 0", "Raider 302", now); !strings.Contains(r.Guard, "not hailed") {
		t.Errorf("order without hail = %+v", r)
	}
	r := h.handle("Guard, Raider 302, unidentified aircraft, identify", "Raider 302", now)
	if !strings.Contains(r.AI, "Flanker") || !strings.Contains(r.AI, "Russian Air Force") {
		t.Fatalf("hail = %+v", r)
	}
	for _, c := range []struct {
		text, ai, call string
	}{
		{"Raider 302, turn right heading 2 7 0", "turning right heading 2 7 0", "turn RED SU-27 CAP-1 27"},
		{"Raider 302, leave the area", "leaving the area", "turn RED SU-27 CAP-1"},
		{"Raider 302, follow me", "following you", "follow RED SU-27 CAP-1 Raider-3"},
		{"Raider 302, land at Bodo", "landing at Bodo", "land RED SU-27 CAP-1 Bodo"},
	} {
		ai.calls = nil
		r := h.handle(c.text, "Raider 302", now.Add(time.Minute))
		if !strings.Contains(r.AI, c.ai) || r.Do == nil {
			t.Errorf("%q: %+v", c.text, r)
			continue
		}
		if err := r.Do(context.Background()); err != nil || len(ai.calls) != 1 || !strings.HasPrefix(ai.calls[0], c.call) {
			t.Errorf("%q: calls %v err %v, want prefix %q", c.text, ai.calls, err, c.call)
		}
	}
	// A hail goes stale.
	if r := h.handle("Raider 302, follow me", "Raider 302", now.Add(30*time.Minute)); !strings.Contains(r.Guard, "not hailed") {
		t.Errorf("stale hail: %+v", r)
	}
	// Nobody to hail.
	empty := newGuardHail(acmiStore(t, acmiPlayer), ai)
	for _, c := range empty.store.Contacts(0) {
		empty.store.Set("Raider 302", c.Pos)
	}
	if r := empty.handle("Guard, Raider 302, identify", "Raider 302", now); !strings.Contains(r.Guard, "no radar contact") {
		t.Errorf("empty sky: %+v", r)
	}
}

func TestCollapseDigits(t *testing.T) {
	if got := collapseDigits("turn right heading 2 7 0 now, angels 2 5"); got != "turn right heading 270 now, angels 25" {
		t.Errorf("collapseDigits = %q", got)
	}
}
