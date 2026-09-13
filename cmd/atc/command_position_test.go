package main

import (
	"math"
	"strings"
	"testing"

	"github.com/paulmach/orb"
)

func feedACMI(p *miniTacviewParser, lines ...string) {
	for _, l := range lines {
		p.handleLine(l)
	}
}

// ACMI only sends a property when it changes. The old mini tracker keyed each
// line by that line's own Pilot/Name, so a T=-only update (every frame after
// the first) was dropped and Command's positions froze at first sighting.
func TestMiniTacviewParserFollowsDeltaFrames(t *testing.T) {
	store := newTacviewPositions()
	p := newMiniTacviewParser(store)
	feedACMI(p,
		"0,ReferenceLongitude=56,ReferenceLatitude=26",
		"a1,T=0.5|0.5|6000,Type=Air+FixedWing,Name=FA-18C_hornet,Pilot=Raider 11 | Jedi,Coalition=Allies",
		"a1,T=0.6||6100",
	)
	pos, ok := store.Get("Raider 11")
	if !ok {
		t.Fatal("Raider 11 not in store")
	}
	if math.Abs(pos[0]-56.6) > 1e-9 || math.Abs(pos[1]-26.5) > 1e-9 {
		t.Errorf("position = %v, want [56.6 26.5] (lat kept from the earlier frame)", pos)
	}
	info := store.Info("RAIDER 11")
	if math.Abs(info.AltFt-6100*3.28084) > 1e-6 || info.Coalition != "Allies" || info.Name != "FA-18C_hornet" {
		t.Errorf("info = %+v", info)
	}
	feedACMI(p, "-a1")
	if _, ok := store.Get("Raider 11"); ok {
		t.Error("destroyed object still in store")
	}
}

func TestIsTanker(t *testing.T) {
	cases := []struct {
		callsign, model string
		want            bool
	}{
		{"Texaco11", "KC135MPRS", true},
		{"Tanker 1", "KC-130", true},
		{"Arco 1-1", "S-3B Tanker", true},
		{"Shell21", "", true},
		{"Enemy tanker", "IL-78M", true},
		{"Raider 11", "FA-18C_hornet", false},
		{"Viper 21", "F-16C_50", false},
	}
	for _, tc := range cases {
		if got := isTanker(tc.callsign, tc.model); got != tc.want {
			t.Errorf("isTanker(%q, %q) = %v, want %v", tc.callsign, tc.model, got, tc.want)
		}
	}
}

func positionTestStore() *tacviewPositions {
	store := newTacviewPositions()
	feedACMI(newMiniTacviewParser(store),
		"0,ReferenceLongitude=0,ReferenceLatitude=0",
		"b1,T=0|0|0,Type=Navaid+Static+Bullseye,Coalition=Allies",
		"b2,T=1|1|0,Type=Navaid+Static+Bullseye,Coalition=Enemies",
		// 30 nm due north of the blue bullseye.
		"a1,T=0|0.5|6000,Type=Air+FixedWing,Name=FA-18C_hornet,Pilot=Raider 11 | Jedi,Coalition=Allies",
		// Blue tanker 30 nm due east of Raider 11 at 24,000 ft.
		"k1,T=0.5|0.5|7315.2,Type=Air+FixedWing,Name=KC135MPRS,Pilot=Texaco11,Coalition=Allies",
		// Red tanker much closer — must never be offered.
		"k2,T=0|0.6|7000,Type=Air+FixedWing,Name=IL-78M,Pilot=Red tanker,Coalition=Enemies",
	)
	return store
}

func TestCommandPositionResponse(t *testing.T) {
	const ch = "vSFG-7-Command"
	store := positionTestStore()
	noVar := func(orb.Point) float64 { return 0 }

	cases := []struct {
		name  string
		text  string
		cs    string
		store *tacviewPositions
		mag   magVarFn
		want  []string
	}{
		{"bullseye", "Command, Raider 11, say bullseye", "Raider 11", store, noVar, []string{"bullseye zero-zero-zero for 30"}},
		{"bullseye magnetic", "Command, Raider 11, request bullseye", "Raider 11", store, func(orb.Point) float64 { return 5 }, []string{"bullseye three-five-five for 30"}},
		{"tanker", "Command, Raider 11, where's the tanker", "Raider 11", store, noVar, []string{"Texaco11 bears zero-niner-zero for 30", "angels 24"}},
		{"unknown pilot", "Command, Venom 21, say bullseye", "Venom 21", store, noVar, []string{"no radar contact"}},
		{"no tacview", "Command, Raider 11, request tanker", "Raider 11", nil, noVar, []string{"no radar picture"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commandPositionResponse(tc.text, tc.cs, ch, tc.store, tc.mag)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("got %q, missing %q", got, w)
				}
			}
		})
	}

	empty := newTacviewPositions()
	feedACMI(newMiniTacviewParser(empty), "a1,T=0|0.5|6000,Type=Air+FixedWing,Pilot=Raider 11 | Jedi,Coalition=Allies")
	if got := commandPositionResponse("Command, Raider 11, request tanker", "Raider 11", ch, empty, noVar); !strings.Contains(got, "negative tanker") {
		t.Errorf("no tanker on scope: got %q", got)
	}
	if got := commandPositionResponse("Command, Raider 11, say bullseye", "Raider 11", ch, empty, noVar); !strings.Contains(got, "no bullseye") {
		t.Errorf("no bullseye on scope: got %q", got)
	}
	if got := commandPositionResponse("Command, Raider 11, fence in", "Raider 11", ch, store, noVar); got != "" {
		t.Errorf("non-position call should fall through, got %q", got)
	}
}
