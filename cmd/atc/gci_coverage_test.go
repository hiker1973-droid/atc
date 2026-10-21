package main

import (
	"math"
	"testing"
	"time"

	"github.com/martinlindhe/unit"
	"github.com/paulmach/orb"

	"github.com/vsfg7/atc/pkg/gci/coalitions"
	"github.com/vsfg7/atc/pkg/gci/sim"
	"github.com/vsfg7/atc/pkg/gci/trackfiles"
)

func TestRadarHorizon(t *testing.T) {
	// E-3 at 30,000 ft against a jet at 500 ft: about 240 nm.
	if d := radarHorizonNm(30000, 500); math.Abs(d-240.5) > 1 {
		t.Errorf("horizon %v", d)
	}
}

func covFixture(t *testing.T) (*gciCoverage, *tacviewPositions, chan sim.Faded, time.Time) {
	store := newTacviewPositions()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	fades := make(chan sim.Faded, 8)
	c := newGCICoverage(store, "Darkstar", coalitions.Blue, fades)
	c.now = func() time.Time { return now }
	return c, store, fades, now
}

func putAC(store *tacviewPositions, key, typ, side string, lat, alt float64, at time.Time) {
	store.Set(key, orb.Point{37.0, lat})
	store.SetInfo(key, tacviewContactInfo{Name: typ, Coalition: side, AltFt: alt, UpdatedAt: at})
}

func hostile(id uint64, lat, altFt float64) sim.Updated {
	return sim.Updated{
		Labels: trackfiles.Labels{ID: id, Name: "Bandit", Coalition: coalitions.Red, ACMIName: "Su-27"},
		Frame:  trackfiles.Frame{Point: orb.Point{37.0, lat}, Altitude: unit.Length(altFt) * unit.Foot},
	}
}

// Operator 2026-10-21: the picture is what the mission's AWACS can see.
func TestGCICoverage(t *testing.T) {
	c, store, fades, now := covFixture(t)
	// AWACS at 43N; 1 degree of latitude = 60 nm.
	putAC(store, "Overlord 1-1", "E-2C", "Allies", 43.0, 25000, now) // another AWACS
	putAC(store, "Darkstar 1-1", "E-3A", "Allies", 43.0, 10000, now) // ours, low orbit
	putAC(store, "Raider 302", "FA-18C_hornet", "Allies", 43.0, 20000, now)
	r, ok := c.platform()
	if !ok || r.Key != "Darkstar 1-1" || r.RangeNm != 217 {
		t.Fatalf("platform %+v %v - want the E-3 named for the GCI", r, ok)
	}
	for _, tc := range []struct {
		name    string
		lat, ft float64
		want    bool
	}{
		{"high, 150 nm", 45.5, 30000, true},
		{"high, 240 nm - beyond the E-3's 217", 47.0, 30000, false},
		{"low, 150 nm - below a 10,000 ft AWACS' horizon (~135 nm)", 45.5, 100, false},
		{"low, 100 nm - inside it", 44.666, 100, true},
	} {
		if got := c.filter(hostile(uint64(100+len(tc.name)), tc.lat, tc.ft)); got != tc.want {
			t.Errorf("%s: visible %v, want %v", tc.name, got, tc.want)
		}
	}
	// Friendlies are always on scope.
	f := hostile(9, 49.0, 100)
	f.Labels.Coalition = coalitions.Blue
	if !c.filter(f) {
		t.Error("friendly hidden")
	}
	// A seen contact that drops out of coverage is faded once.
	c.filter(hostile(2, 45.0, 30000))
	if c.filter(hostile(2, 47.5, 30000)) {
		t.Fatal("should be out of range")
	}
	select {
	case fd := <-fades:
		if fd.ID != 2 {
			t.Errorf("faded %d", fd.ID)
		}
	case <-time.After(time.Second):
		t.Error("no FADED for a contact leaving coverage")
	}
	c.filter(hostile(2, 47.6, 30000)) // still out: no second fade
	select {
	case fd := <-fades:
		t.Errorf("second fade %d", fd.ID)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestGCIPlatformAnyAWACS(t *testing.T) {
	c, store, _, now := covFixture(t)
	putAC(store, "AWACS_BLUE-1", "E-3A", "Allies", 43.0, 30000, now) // Foothold's naming
	putAC(store, "AWACS_RED-1", "E-3A", "Enemies", 43.0, 30000, now) // theirs
	if r, ok := c.platform(); !ok || r.Key != "AWACS_BLUE-1" {
		t.Errorf("platform %+v %v", r, ok)
	}
	// On the deck, or stale, is not on station.
	putAC(store, "AWACS_BLUE-1", "E-3A", "Allies", 43.0, 200, now)
	if _, ok := c.platform(); ok {
		t.Error("an AWACS on the ground counted")
	}
	putAC(store, "AWACS_BLUE-1", "E-3A", "Allies", 43.0, 30000, now.Add(-2*time.Minute))
	if _, ok := c.platform(); ok {
		t.Error("a stale AWACS counted")
	}
}

// Off station: everything hostile hidden quietly, announced after the debounce.
func TestGCIOnStation(t *testing.T) {
	c, store, fades, now := covFixture(t)
	if on, changed, _ := c.onStation(); on || changed {
		t.Fatalf("initial: on %v changed %v", on, changed)
	}
	if c.filter(hostile(3, 44.0, 30000)) {
		t.Error("hostile seen with no AWACS up")
	}
	select {
	case <-fades:
		t.Error("fade sent while off station")
	case <-time.After(100 * time.Millisecond):
	}
	putAC(store, "Darkstar 1-1", "E-3A", "Allies", 43.0, 30000, now)
	if on, changed, _ := c.onStation(); on || changed {
		t.Errorf("announced before the debounce: on %v changed %v", on, changed)
	}
	later := now.Add(gciStationDebounce + time.Second)
	c.now = func() time.Time { return later }
	store.SetInfo("Darkstar 1-1", tacviewContactInfo{Name: "E-3A", Coalition: "Allies", AltFt: 30000, UpdatedAt: later})
	if on, changed, r := c.onStation(); !on || !changed || r.Key != "Darkstar 1-1" {
		t.Errorf("after debounce: on %v changed %v %+v", on, changed, r)
	}
	if on, changed, _ := c.onStation(); !on || changed {
		t.Errorf("announced twice: on %v changed %v", on, changed)
	}
}

func TestGCIOffStationReplies(t *testing.T) {
	if got := gciOffStation("raider 3 0 2", "Darkstar"); got != "RAIDER 3 0 2, DARKSTAR, off station, no picture." {
		t.Errorf("%q", got)
	}
}
