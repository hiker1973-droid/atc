package airfield

import (
	"math"
	"testing"
)

// Operation WHITE NIGHT runs only the field in play (operator 2026-10-09). FieldsForMap
// drives the Command handoff scan, so a parked field creeping into the Kola slice would
// have Command hand pilots to a tower nobody is running.
func TestKolaLiveFields(t *testing.T) {
	want := []struct {
		icao        string
		tower, atis float64
	}{
		{"ENBO", 251.150, 249.150},
	}
	for _, alias := range []string{"kola", "KO", " Kola "} {
		got := FieldsForMap(alias)
		if len(got) != len(want) {
			t.Fatalf("FieldsForMap(%q) has %d fields, want %d", alias, len(got), len(want))
		}
		for i, w := range want {
			af := got[i]
			if af.ICAO != w.icao || af.TowerFreqMHz != w.tower || af.ATISFreqMHz != w.atis {
				t.Errorf("field %d = %s %.3f/%.3f, want %s %.3f/%.3f", i, af.ICAO, af.TowerFreqMHz, af.ATISFreqMHz, w.icao, w.tower, w.atis)
			}
		}
	}
}

// All four definitions, parked or not. Headings are magnetic and must be a reciprocal
// pair that agrees with the runway number; magvar is per field because it runs from
// 8°E at Bodo to 15°E at Banak, and one map-wide value would put a heading 7° out.
func TestKolaFieldData(t *testing.T) {
	cases := []struct {
		af     *Airfield
		elev   int
		magvar float64
		a      string
		hdgA   float64
		b      string
		hdgB   float64
	}{
		{ENBO, 33, 8.0, "07", 70, "25", 250},
		{ENDU, 234, 10.8, "28", 278, "10", 98},
		{ENNA, 17, 14.8, "34", 341, "16", 161},
		{EFRO, 636, 13.6, "21", 204, "03", 24},
	}
	for _, c := range cases {
		if ByICAO(c.af.ICAO) != c.af {
			t.Errorf("ByICAO(%q) does not return the Kola entry", c.af.ICAO)
		}
		if c.af.ElevationFt != c.elev || c.af.MagVar != c.magvar {
			t.Errorf("%s elevation/magvar = %d/%.1f, want %d/%.1f", c.af.ICAO, c.af.ElevationFt, c.af.MagVar, c.elev, c.magvar)
		}
		p := c.af.RunwayPairs[0]
		if p.Primary.Designator != c.a || p.Primary.MagneticHeading != c.hdgA || p.Reciprocal.Designator != c.b || p.Reciprocal.MagneticHeading != c.hdgB {
			t.Errorf("%s runway = %s %.0f / %s %.0f, want %s %.0f / %s %.0f", c.af.ICAO, p.Primary.Designator, p.Primary.MagneticHeading,
				p.Reciprocal.Designator, p.Reciprocal.MagneticHeading, c.a, c.hdgA, c.b, c.hdgB)
		}
		if d := math.Abs(math.Mod(p.Primary.MagneticHeading-p.Reciprocal.MagneticHeading+360, 360) - 180); d > 0.5 {
			t.Errorf("%s runway ends are not reciprocal (%.1f off)", c.af.ICAO, d)
		}
		for _, r := range []Runway{p.Primary, p.Reciprocal} {
			var num float64
			for _, ch := range r.Designator {
				if ch >= '0' && ch <= '9' {
					num = num*10 + float64(ch-'0')
				}
			}
			if d := math.Abs(math.Mod(r.MagneticHeading-num*10+540, 360) - 180); d > 12 {
				t.Errorf("%s runway %s heading %.0f is %.0f° from its number", c.af.ICAO, r.Designator, r.MagneticHeading, d)
			}
		}
		if c.af.TowerFreqMHz-c.af.ATISFreqMHz != 2.0 {
			t.Errorf("%s ATIS %.3f is not tower %.3f minus two", c.af.ICAO, c.af.ATISFreqMHz, c.af.TowerFreqMHz)
		}
	}
}
