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
	"github.com/vsfg7/atc/pkg/terrain"
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
	// As our servers' recorder writes it: Color is the side, Coalition is not
	// (blue is labelled "Enemies").
	label := map[string]string{"Blue": "Enemies", "Red": "Allies"}[side]
	store.SetInfo(key, tacviewContactInfo{Name: typ, Color: side, Coalition: label, AltFt: alt, UpdatedAt: at})
}

func hostile(id uint64, lat, altFt float64) sim.Updated {
	return sim.Updated{
		Labels: trackfiles.Labels{ID: id, Name: "Bandit", Coalition: coalitions.Red, ACMIName: "Su-27"},
		Frame:  trackfiles.Frame{Point: orb.Point{37.0, lat}, Altitude: unit.Length(altFt) * unit.Foot},
	}
}

// Operator 2026-10-21: the picture is what the mission's AWACS can see -
// fused over every AWACS of ours that is up ("combine all the AWACS").
func TestGCICoverage(t *testing.T) {
	c, store, fades, now := covFixture(t)
	// 1 degree of latitude = 60 nm. An E-3 low at 43N, an E-2 high at 41N.
	putAC(store, "Darkstar 1-1", "E-3A", "Blue", 43.0, 10000, now)
	putAC(store, "Wizard 1-1", "E-2C", "Blue", 41.0, 25000, now)
	putAC(store, "Raider 302", "FA-18C_hornet", "Blue", 43.0, 20000, now)
	radars := c.platforms()
	if len(radars) != 2 || gciRadarsText(radars) != "E-3 Darkstar 1-1 + E-2 Wizard 1-1" {
		t.Fatalf("platforms %q", gciRadarsText(radars))
	}
	for i, tc := range []struct {
		name    string
		lat, ft float64
		want    bool
	}{
		{"high, 150 nm north of the E-3", 45.5, 30000, true},
		{"high, 240 nm north - beyond the E-3's 217, 360 from the E-2", 47.0, 30000, false},
		{"low, 150 nm north of the E-3 - below its 10,000 ft horizon, 270 from the E-2", 45.5, 100, false},
		{"low, 100 nm north of the E-3 - inside its horizon", 44.666, 100, true},
		{"low, 150 nm south of the E-2 - only the E-2 at 25,000 ft sees it", 38.5, 100, true},
	} {
		if got := c.filter(hostile(uint64(100+i), tc.lat, tc.ft)); got != tc.want {
			t.Errorf("%s: visible %v, want %v", tc.name, got, tc.want)
		}
	}
	// Friendlies are always on scope.
	f := hostile(9, 49.0, 100)
	f.Labels.Coalition = coalitions.Blue
	if !c.filter(f) {
		t.Error("friendly hidden")
	}
	// A seen contact that drops out of every AWACS' coverage is faded once.
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

func TestGCIPlatforms(t *testing.T) {
	c, store, _, now := covFixture(t)
	putAC(store, "AWACS_BLUE-1", "E-3A", "Blue", 43.0, 30000, now) // Foothold's naming
	putAC(store, "AWACS_RED-1", "E-3A", "Red", 43.0, 30000, now) // theirs
	if got := gciRadarsText(c.platforms()); got != "E-3 AWACS_BLUE-1" {
		t.Errorf("platforms %q", got)
	}
	// On the deck, or stale, is not up.
	putAC(store, "AWACS_BLUE-1", "E-3A", "Blue", 43.0, 200, now)
	if len(c.platforms()) != 0 {
		t.Error("an AWACS on the ground counted")
	}
	putAC(store, "AWACS_BLUE-1", "E-3A", "Blue", 43.0, 30000, now.Add(-2*time.Minute))
	if len(c.platforms()) != 0 {
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
	putAC(store, "Darkstar 1-1", "E-3A", "Blue", 43.0, 30000, now)
	if on, changed, _ := c.onStation(); on || changed {
		t.Errorf("announced before the debounce: on %v changed %v", on, changed)
	}
	later := now.Add(gciStationDebounce + time.Second)
	c.now = func() time.Time { return later }
	store.SetInfo("Darkstar 1-1", tacviewContactInfo{Name: "E-3A", Color: "Blue", Coalition: "Enemies", AltFt: 30000, UpdatedAt: later})
	if on, changed, r := c.onStation(); !on || !changed || gciRadarsText(r) != "E-3 Darkstar 1-1" {
		t.Errorf("after debounce: on %v changed %v %q", on, changed, gciRadarsText(r))
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

// Phase 2: the Caucasus grid masks a low jet behind the main ridge from an
// E-3 over Kutaisi; without the grid (other maps) only the horizon counts.
func TestGCITerrainMasking(t *testing.T) {
	g, err := terrain.ForMap("caucasus")
	if err != nil {
		t.Fatal(err)
	}
	e3 := gciRadar{Key: "Darkstar 1-1", Type: "E-3A", Pos: orb.Point{42.5, 42.2}, AltFt: 25000, RangeNm: 217}
	nalchik := orb.Point{43.6, 43.5}
	if e3.sees(nalchik, 2500, g) {
		t.Error("low jet behind the ridge seen with terrain")
	}
	if !e3.sees(nalchik, 2500, nil) {
		t.Error("low jet not seen without terrain (horizon only)")
	}
	if !e3.sees(nalchik, 30000, g) {
		t.Error("high jet masked")
	}
}

// The lines our Caucasus server's recorder really writes (Tacview 2026-10-17):
// blue AWACS carry Color=Blue but Coalition=Enemies. Matching on Coalition kept
// the GCI off station on Day and Night all of 2026-10-21. Wizard 1-1 is on the
// deck (22 m) and does not count.
func TestGCIPlatformsFromACMI(t *testing.T) {
	store := newTacviewPositions()
	p := newMiniTacviewParser(store)
	for _, l := range []string{
		"0,ReferenceLongitude=33",
		"0,ReferenceLatitude=39",
		"d402,T=4.1618674|3.8801666|22.32||0.8|100.9|240390.89|-242199.16|98,Type=Air+FixedWing,Name=E-2C,Pilot=Wizard 1-1,Group=Wizard 1,Color=Blue,Coalition=Enemies,Country=us,AGL=22.32",
		"d602,T=6.5197572|3.5891781|9749.69||0.2|128.5|435521.53|-262272.12|124,Type=Air+FixedWing,Name=E-3A,Pilot=Darkstar 1-1,Group=Darkstar 1,Color=Blue,Coalition=Enemies,Country=us,AGL=9749.69",
		"d702,T=11.1922666|2.5097789|7312.82|0.4|0.1|26.1|835183.5|-341846.88|18.6,Type=Air+FixedWing,Name=E-2C,Pilot=Overlord 1-1,Group=Overlord 1,Color=Blue,Coalition=Enemies,Country=us,AGL=5795.69",
	} {
		p.handleLine(l)
	}
	c := newGCICoverage(store, "Darkstar", coalitions.Blue, nil)
	if got := gciRadarsText(c.platforms()); got != "E-3 Darkstar 1-1 + E-2 Overlord 1-1" {
		t.Errorf("blue radars %q", got)
	}
	if got := newGCICoverage(store, "Darkstar", coalitions.Red, nil).platforms(); len(got) != 0 {
		t.Errorf("red side sees blue AWACS: %v", got)
	}
}
