package main

import (
	"strings"
	"testing"
)

// CAS-LGB TIC mission: nine sites, phase 1 (RED force) then phase 2 (static),
// "established" gets the run-in heading.
func TestCASTICFlow(t *testing.T) {
	if len(casTICTargets) != 18 {
		t.Fatalf("want 18 9-lines, got %d", len(casTICTargets))
	}
	b, _ := newTestTaskingBoard(casTICTargets)
	b.jtacs = []*jtacNet{{Spoken: hitman, FreqMHz: 61.0}}
	pic := &fakePicture{}

	r := b.JTACResponse("Hitman one one, Raider 302, checking in", "Raider 302", hitman, pic)
	tg := b.targets[b.flights["raider 302"].target]
	if tg.Phase != 1 || !strings.Contains(r, "troops in contact") || !strings.Contains(r, tg.Spoken) || !strings.Contains(r, "IP "+tg.IP) {
		t.Fatalf("phase 1 brief: %q", r)
	}
	b.JTACResponse("copy", "Raider 302", hitman, pic)

	r = b.JTACResponse("Raider 302, established IP "+tg.IP, "Raider 302", hitman, pic)
	if !strings.Contains(r, "Run in heading") || !strings.Contains(r, "final attack heading") {
		t.Fatalf("established: %q", r)
	}
	r = b.JTACResponse("Raider 302, established IP Dagger", "Raider 302", hitman, pic)
	if !strings.Contains(r, "Run in heading") {
		t.Fatalf("unknown IP name still gets the heading: %q", r)
	}
	if r = b.JTACResponse("Raider 302, off", "Raider 302", hitman, pic); !strings.Contains(r, "copy off") {
		t.Fatalf("off: %q", r)
	}

	// Asking again moves to the same site's static target.
	r = b.JTACResponse("Hitman one one, Raider 302, ready for 9-line", "Raider 302", hitman, pic)
	t2 := b.targets[b.flights["raider 302"].target]
	if t2.Phase != 2 || t2.Number != tg.Number || !strings.Contains(r, "next target") {
		t.Fatalf("phase 2: %q %+v", r, t2)
	}
}

// A second flight never draws the site another flight holds, in either phase.
func TestCASTICSiteHeld(t *testing.T) {
	b, _ := newTestTaskingBoard(casTICTargets)
	b.flights["a"] = &taskingAssignment{target: 3, issued: b.now()} // site 2, phase 2
	free, _, _ := b.free("b", b.now())
	for _, i := range free {
		if b.targets[i].Phase == 2 || b.targets[i].Number == b.targets[3].Number {
			t.Fatalf("free list has %+v", b.targets[i])
		}
	}
	if len(free) != 8 {
		t.Fatalf("free %d want 8", len(free))
	}
}

// Every 9-line has a BDA table with objects (bridges aside), and both sector
// boards of the two JTACs together cover all nine sites.
func TestCASTICTables(t *testing.T) {
	for _, tg := range casTICTargets {
		bt := bdaTargetFor(tg)
		if len(bt.Objects) == 0 && !bt.Bridge && tg.BDAKey != 0 {
			t.Errorf("site %d phase %d has no BDA objects", tg.Number, tg.Phase)
		}
	}
	seen := map[int]bool{}
	for _, s := range []string{"Kuban Valley", "Cherkessk"} {
		for _, tg := range sectorTargets(casTICTargets, s) {
			seen[tg.Number] = true
		}
	}
	if len(seen) != 9 {
		t.Fatalf("sectors cover %d sites", len(seen))
	}
}
