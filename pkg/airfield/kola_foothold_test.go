package airfield

import (
	"math"
	"testing"
)

// Foothold Kola runs five fields (operator 2026-11-14). The Kola slice (White Night, Bodo only) must NOT change.
func TestKolaFootholdFields(t *testing.T) {
	want := []struct {
		icao        string
		tower, atis float64
	}{
		{"ENAN", 250.600, 248.600},
		{"ENBO", 251.150, 249.150},
		{"ENDU", 250.650, 248.650},
		{"ENNA", 250.550, 248.550},
		{"EFRO", 250.950, 248.950},
	}
	for _, alias := range []string{"kola-foothold", "KolaFH", " kola-fh "} {
		got := FieldsForMap(alias)
		if len(got) != len(want) {
			t.Fatalf("FieldsForMap(%q) has %d fields, want %d", alias, len(got), len(want))
		}
		for i, w := range want {
			if got[i].ICAO != w.icao || got[i].TowerFreqMHz != w.tower || got[i].ATISFreqMHz != w.atis {
				t.Errorf("field %d = %s %.3f/%.3f, want %s %.3f/%.3f", i, got[i].ICAO, got[i].TowerFreqMHz, got[i].ATISFreqMHz, w.icao, w.tower, w.atis)
			}
		}
		if TheatreKey(alias) != "kola" {
			t.Errorf("TheatreKey(%q) = %q, want kola", alias, TheatreKey(alias))
		}
	}
	if n := len(FieldsForMap("kola")); n != 1 {
		t.Errorf("the White Night Kola slice grew to %d fields; it must stay Bodo only", n)
	}
	if TheatreOfICAO("ENAN") != "kola" {
		t.Errorf("TheatreOfICAO(ENAN) = %q, want kola", TheatreOfICAO("ENAN"))
	}
}

// Andoya is derived, not surveyed; pin the numbers so a careless edit cannot move them.
func TestAndoyaData(t *testing.T) {
	af := ENAN
	if ByICAO("enan") != af {
		t.Fatal("ByICAO(enan) does not return ENAN")
	}
	if af.ElevationFt != 22 || af.MagVar != 6.3 {
		t.Errorf("elevation/magvar = %d/%.1f, want 22/6.3", af.ElevationFt, af.MagVar)
	}
	p := af.RunwayPairs[0]
	if p.Primary.Designator != "14" || p.Primary.MagneticHeading != 143 || p.Reciprocal.Designator != "32" || p.Reciprocal.MagneticHeading != 323 {
		t.Errorf("runway = %s %.0f / %s %.0f, want 14 143 / 32 323", p.Primary.Designator, p.Primary.MagneticHeading, p.Reciprocal.Designator, p.Reciprocal.MagneticHeading)
	}
	if d := math.Abs(math.Mod(p.Primary.MagneticHeading-p.Reciprocal.MagneticHeading+360, 360) - 180); d > 0.5 {
		t.Errorf("runway ends are not reciprocal (%.1f off)", d)
	}
	// threshold separation ~2,500 m and the thresholds straddle the airbase reference point
	a, b := p.Primary.ThresholdLatLon, p.Reciprocal.ThresholdLatLon
	dlat := (a[1] - b[1]) * 111132.0
	dlon := (a[0] - b[0]) * 111412.0 * math.Cos(a[1]*math.Pi/180)
	if l := math.Hypot(dlat, dlon); math.Abs(l-2500) > 60 {
		t.Errorf("threshold separation %.0f m, want ~2500", l)
	}
	if math.Abs((a[0]+b[0])/2-af.Center[0]) > 0.0005 || math.Abs((a[1]+b[1])/2-af.Center[1]) > 0.0005 {
		t.Error("thresholds are not centred on the airbase reference point")
	}
	if af.TowerFreqMHz-af.ATISFreqMHz != 2.0 {
		t.Errorf("ATIS %.3f is not tower %.3f minus two", af.ATISFreqMHz, af.TowerFreqMHz)
	}
	if port, ok := DashboardPorts["ENAN"]; !ok || port != 6055 {
		t.Errorf("ENAN dashboard port = %d (%v), want 6055", port, ok)
	}
}
