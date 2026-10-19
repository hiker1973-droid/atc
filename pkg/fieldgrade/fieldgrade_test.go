package fieldgrade

import (
	"math"
	"strings"
	"testing"
	"time"
)

// Senaki as measured from Tacview (pkg/airfield UGKS): 09/27, 100.7 true.
func senaki() Field {
	return NewField("Senaki", 43,
		[2]Threshold{{"09", 42.24277, 42.0342}, {"27", 42.23901, 42.0610}})
}

// fly runs a straight-in to 09 at 70 m/s from 3 nm and returns the results.
// path gives the height above the runway (m) at metres before the threshold;
// right the offset from the centreline. The jet touches down where path
// reaches 0 and rolls out at 3 m/s^2.
func fly(t *testing.T, path, right func(before float64) float64) []Result {
	t.Helper()
	f := senaki()
	e := f.Ends[0]
	tr := NewTracker(f)
	var out []Result
	tr.Done = func(r Result) { out = append(out, r) }
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ground := f.ElevM + refAboveRunwayM
	ce, cn := math.Sin(e.CourseDeg*math.Pi/180), math.Cos(e.CourseDeg*math.Pi/180)
	pos := func(before, r float64) (float64, float64) {
		de := -before*ce + r*cn
		dn := -before*cn - r*ce
		return e.Lat + dn/110540, e.Lon + de/(111320*math.Cos(e.Lat*math.Pi/180))
	}
	v := 70.0
	before := 3 * nm
	for i := 0; i < 2000 && v > 5; i++ {
		h := path(before)
		if h <= 0 {
			h = 0
			v -= 3 * 0.2
		}
		lat, lon := pos(before, right(before))
		tr.Update("Raider 302", "FA-18C_hornet", Sample{At: at, Lat: lat, Lon: lon, AltM: ground + h, PitchDeg: 8.1 - 3, HasPitch: true})
		before -= v * 0.2
		at = at.Add(200 * time.Millisecond)
	}
	tr.Expire(at.Add(time.Minute))
	return out
}

func glide(deg float64) func(float64) float64 {
	return func(before float64) float64 {
		return math.Tan(deg*math.Pi/180) * (before + AimM)
	}
}

func center(float64) float64 { return 0 }

func one(t *testing.T, rs []Result) Result {
	t.Helper()
	if len(rs) != 1 {
		t.Fatalf("got %d results: %+v", len(rs), rs)
	}
	return rs[0]
}

func TestStraightInOnPath(t *testing.T) {
	r := one(t, fly(t, glide(3), center))
	if r.Outcome != "landed" || r.Runway != "09" {
		t.Fatalf("%s", r.Summary())
	}
	if !strings.HasPrefix(r.Grade, "_OK_") && !strings.HasPrefix(r.Grade, "OK") {
		t.Errorf("on path: grade %q; %s", r.Grade, r.Summary())
	}
	if math.Abs(r.TDPastM-AimM) > 40 || math.Abs(r.TDRightM) > 2 {
		t.Errorf("touchdown %.0f m past, %+.0f m right", r.TDPastM, r.TDRightM)
	}
}

func TestHighAndLong(t *testing.T) {
	// 4.5 degrees to a point 900 m down the runway: high everywhere, long.
	path := func(before float64) float64 { return math.Tan(4.5*math.Pi/180) * (before + 1000) }
	r := one(t, fly(t, path, center))
	for _, want := range []string{"HIM", "LNGTD"} {
		if !strings.Contains(r.Grade, want) {
			t.Errorf("grade %q lacks %s", r.Grade, want)
		}
	}
	if !strings.HasPrefix(r.Grade, "---") {
		t.Errorf("grade %q, want ---", r.Grade)
	}
}

func TestShortIsCut(t *testing.T) {
	// Aimed 150 m short of the threshold.
	path := func(before float64) float64 { return math.Tan(3*math.Pi/180) * (before - 150) }
	r := one(t, fly(t, path, center))
	if !strings.HasPrefix(r.Grade, "C") || !strings.Contains(r.Grade, "SHTTD") {
		t.Errorf("grade %q, want C with SHTTD", r.Grade)
	}
}

func TestLinedUpRight(t *testing.T) {
	r := one(t, fly(t, glide(3), func(float64) float64 { return 25 }))
	if !strings.Contains(r.Grade, "LURTD") {
		t.Errorf("25 m right: grade %q", r.Grade)
	}
}

func TestGoAround(t *testing.T) {
	// Down to 150 ft at 1/2 nm, then climbing away.
	path := func(before float64) float64 {
		if before > 0.5*nm {
			return glide(3)(before)
		}
		return glide(3)(0.5*nm) + (0.5*nm-before)*0.15
	}
	r := one(t, fly(t, path, center))
	if r.Outcome != "go-around" || !strings.HasPrefix(r.Grade, "WO") {
		t.Errorf("%s", r.Summary())
	}
}

// A departure off 09 climbs out over the far end: no pass.
func TestDepartureIgnored(t *testing.T) {
	f := senaki()
	tr := NewTracker(f)
	n := 0
	tr.Done = func(Result) { n++ }
	e := f.Ends[0]
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ce, cn := math.Sin(e.CourseDeg*math.Pi/180), math.Cos(e.CourseDeg*math.Pi/180)
	for along := 0.0; along < 8000; along += 14 {
		h := math.Max(0, (along-1200)*0.15)
		lat, lon := e.Lat+along*cn/110540, e.Lon+along*ce/(111320*math.Cos(e.Lat*math.Pi/180))
		tr.Update("Raider 302", "FA-18C_hornet", Sample{At: at, Lat: lat, Lon: lon, AltM: f.ElevM + refAboveRunwayM + h})
		at = at.Add(200 * time.Millisecond)
	}
	tr.Expire(at.Add(time.Minute))
	if n != 0 {
		t.Errorf("departure graded %d times", n)
	}
}
