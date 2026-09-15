package main

import (
	"math"
	"testing"

	"github.com/paulmach/orb"

	"github.com/vsfg7/atc/pkg/airfield"
)

// offsetNm moves p north and east by the given nm (flat-earth, fine at these ranges).
func offsetNm(p orb.Point, northNm, eastNm float64) orb.Point {
	lat := p[1] + northNm/60
	lon := p[0] + eastNm/(60*math.Cos(p[1]*math.Pi/180))
	return orb.Point{lon, lat}
}

// 2026-09-14, Syria: Raider 331 launched off CVN-72 and called RTB with the
// boat ~52 nm away and Akrotiri ~29 nm. Command sent them to Akrotiri tower.
func TestChooseRecoveryCarrierBasedPilotGoesToMarshal(t *testing.T) {
	fields := []*airfield.Airfield{airfield.LCRA}
	pilot := offsetNm(airfield.LCRA.Center, 29, 0)
	boat := offsetNm(pilot, 0, 52)

	rec := chooseRecovery(pilot, fields, boat, true, true)
	if rec.kind != "marshal" {
		t.Fatalf("carrier-based pilot sent to %q (%s), want marshal", rec.name, rec.kind)
	}
	if math.Abs(rec.distNm-52) > 1.5 {
		t.Errorf("distance = %.1f, want ~52 (to the boat, which the threshold is tested against)", rec.distNm)
	}

	rec = chooseRecovery(pilot, fields, boat, true, false)
	if rec.kind != "tower" || rec.name != airfield.LCRA.Name+" tower" {
		t.Errorf("land-based pilot sent to %q (%s), want %s tower", rec.name, rec.kind, airfield.LCRA.Name)
	}

	rec = chooseRecovery(pilot, fields, boat, false, true)
	if rec.kind != "tower" {
		t.Errorf("with the Marshal handoff unavailable, sent to %q (%s), want the tower", rec.name, rec.kind)
	}
}

// A land-based pilot who happens to be nearer the boat still goes to Marshal —
// the existing nearest-recovery-point behaviour.
func TestChooseRecoveryNearerBoatWins(t *testing.T) {
	fields := []*airfield.Airfield{airfield.LCRA}
	pilot := offsetNm(airfield.LCRA.Center, 40, 0)
	boat := offsetNm(pilot, 0, 10)
	if rec := chooseRecovery(pilot, fields, boat, true, false); rec.kind != "marshal" {
		t.Errorf("sent to %q (%s), want marshal (boat 10 nm, field 40 nm)", rec.name, rec.kind)
	}
}
