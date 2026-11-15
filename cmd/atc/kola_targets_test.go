package main

import (
	"strings"
	"testing"
)

// Hitman on --map kola works the seven Kola Training target areas, and only while that mission is loaded.
func TestKolaTaskingTargets(t *testing.T) {
	oldMiz, oldWatch := flagMizPath, flagMizWatchFile
	defer func() { flagMizPath, flagMizWatchFile = oldMiz, oldWatch }()
	flagMizWatchFile = ""

	flagMizPath = ""
	if got := len(taskingTargetsForMap("kola")); got != 7 {
		t.Fatalf("kola, mission unknown: %d targets, want 7", got)
	}
	flagMizPath = `C:\Missions\vSFG-7_Training_Mission_KO V1.0 BASE FILE.miz`
	if got := len(taskingTargetsForMap("kola")); got != 7 {
		t.Fatalf("kola training: %d targets, want 7", got)
	}
	flagMizPath = `C:\Missions\White Night V1.miz`
	if taskingTargetsForMap("kola") != nil {
		t.Fatal("another Kola mission must have no Hitman targets")
	}
	if taskingTargetsForMap("kola-foothold") != nil {
		t.Fatal("Foothold Kola must have no Hitman targets")
	}
}

func TestKolaTargetData(t *testing.T) {
	sectors := map[string]int{}
	ips := map[string]bool{}
	for _, tg := range kolaTaskingTargets {
		sectors[tg.Sector]++
		if ips[tg.IP] {
			t.Errorf("IP %s used twice", tg.IP)
		}
		ips[tg.IP] = true
		d := nineData(tg)
		if d.Desc == "" || !strings.Contains(d.Grid, " ") || d.Mark == "" {
			t.Errorf("target %d: incomplete 9-line %+v", tg.Number, d)
		}
		if d.DistNm < 14.5 || d.DistNm > 15.5 {
			t.Errorf("target %d: IP is %.1f nm out, want 15", tg.Number, d.DistNm)
		}
		bt := bdaTargetFor(tg)
		if tg.Number != 3 && len(bt.Objects) == 0 {
			t.Errorf("target %d: no BDA objects", tg.Number)
		}
		if tg.FAHLow == tg.FAHHigh || tg.RunIn < 0 || tg.RunIn > 359 {
			t.Errorf("target %d: bad run-in %d (%d-%d)", tg.Number, tg.RunIn, tg.FAHLow, tg.FAHHigh)
		}
	}
	if sectors["North-Lapland"] != 3 || sectors["South-Lapland"] != 4 {
		t.Errorf("sectors = %v", sectors)
	}
	// The Kola set must not disturb the Caucasus lookups that share jtacNineLines' keys.
	if nineData(caucasusTaskingTargets[1]).Grid == nineData(kolaTaskingTargets[1]).Grid {
		t.Error("Caucasus and Kola target 2 share a grid")
	}
}

// A Kola flight is handed to Hitman, briefed the Kola target and cleared hot on it.
func TestKolaJTACBrief(t *testing.T) {
	b, _ := newTestTaskingBoard(kolaTaskingTargets)
	b.jtacs = []*jtacNet{{Spoken: hitman, FreqMHz: 61.0}}
	pic := &fakePicture{}
	if got := b.Response("Command, Raider 11, request tasking", "Raider 11", "Command"); !strings.Contains(got, "Hitman one one") {
		t.Fatalf("request = %q", got)
	}
	b.Response("Command, Raider 11, copy", "Raider 11", "Command")
	got := b.JTACResponse("Hitman 1-1, Raider 11, checking in", "Raider 11", hitman, pic)
	if !strings.Contains(got, "Aurora") && !strings.Contains(got, "ordnance") {
		t.Fatalf("check-in = %q", got)
	}
}
