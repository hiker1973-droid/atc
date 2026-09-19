package airfield

import "testing"

// Foothold's Syria set is exactly five fields since 2026-09-18 (operator ruling):
// primaries Incirlik, Ramat David, Akrotiri; alternates Bassel Al-Assad, Beirut.
// FieldsForMap drives the Command handoff scan, so a dropped field creeping back
// in would have Command hand pilots to a tower nobody is running.
func TestSyriaFootholdFields(t *testing.T) {
	want := []struct {
		icao        string
		tower, atis float64
	}{
		{"LTAG", 360.100, 360.200},
		{"LLRD", 251.300, 256.150},
		{"LCRA", 252.000, 249.500},
		{"OSLK", 250.600, 249.600},
		{"OLBA", 250.650, 249.700},
	}
	got := FieldsForMap("syria")
	if len(got) != len(want) {
		t.Fatalf("FieldsForMap(\"syria\") has %d fields, want %d", len(got), len(want))
	}
	for i, w := range want {
		af := got[i]
		if af.ICAO != w.icao || af.TowerFreqMHz != w.tower || af.ATISFreqMHz != w.atis {
			t.Errorf("field %d = %s %.3f/%.3f, want %s %.3f/%.3f",
				i, af.ICAO, af.TowerFreqMHz, af.ATISFreqMHz, w.icao, w.tower, w.atis)
		}
		if ByICAO(w.icao) != af {
			t.Errorf("ByICAO(%q) does not return the Syria entry", w.icao)
		}
	}
}

// DimOn Aerodrome Data (01 Feb 2026) corrections of 2026-09-19. Bassel Al-Assad has
// two parallels and the ILS is on 17R, so a calm-wind tower must default to 17R, not
// to a "17" pilots never see painted; the other values are what the tower speaks or
// computes distances from.
func TestDimOnCorrections(t *testing.T) {
	if got := OSLK.ActiveRunway(0, 0).Designator; got != "17R" {
		t.Errorf("OSLK calm-wind runway = %q, want 17R (the ILS runway)", got)
	}
	if got := OSLK.ActiveRunway(170, 10).Designator; got != "17R" {
		t.Errorf("OSLK southerly-wind runway = %q, want 17R (first pair wins the tie)", got)
	}
	if got := OSLK.ActiveRunway(350, 10).Designator; got != "35L" {
		t.Errorf("OSLK northerly-wind runway = %q, want 35L", got)
	}
	cases := []struct {
		af   *Airfield
		elev int
		hdg  float64 // first pair's primary magnetic heading
	}{
		{LTAG, 230, 49}, {LLRD, 146, 141}, {LCRA, 69, 106}, {OSLK, 94, 173}, {OLBA, 39, 164},
		{OMDM, 190, 88}, {UGKO, 148, 67},
	}
	for _, c := range cases {
		if c.af.ElevationFt != c.elev {
			t.Errorf("%s elevation %d, want %d", c.af.ICAO, c.af.ElevationFt, c.elev)
		}
		if h := c.af.RunwayPairs[0].Primary.MagneticHeading; h != c.hdg {
			t.Errorf("%s first runway heading %.0f, want %.0f", c.af.ICAO, h, c.hdg)
		}
	}
}
