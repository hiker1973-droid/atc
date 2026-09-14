package main

import "testing"

func TestParseFleetLabels(t *testing.T) {
	got := parseFleet("host@192.168.1.231:7000, dev=vSFG-7 Night Training ATC/ATIS@192.168.1.221:7000 ,192.168.1.50,box=@10.0.0.9")
	want := []Rig{
		{Name: "host", Label: "host", Host: "192.168.1.231", Port: 7000},
		// The label may carry spaces and a slash; the id stays URL-safe.
		{Name: "dev", Label: "vSFG-7 Night Training ATC/ATIS", Host: "192.168.1.221", Port: 7000},
		{Name: "192.168.1.50", Label: "192.168.1.50", Host: "192.168.1.50", Port: 7000},
		// An empty label falls back to the id.
		{Name: "box", Label: "box", Host: "10.0.0.9", Port: 7000},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rigs, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rig %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if r, ok := rigByNameIn(got, "dev"); !ok || r.Label != "vSFG-7 Night Training ATC/ATIS" {
		t.Errorf("lookup by id failed: %+v %v", r, ok)
	}
}

func rigByNameIn(rigs []Rig, name string) (Rig, bool) {
	fleetRigs = rigs
	defer func() { fleetRigs = nil }()
	return rigByName(name)
}
