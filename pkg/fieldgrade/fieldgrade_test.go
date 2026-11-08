package fieldgrade

import (
	"fmt"
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
var flyModel = "FA-18C_hornet"

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
		tr.Update("Raider 302", flyModel, Sample{At: at, Lat: lat, Lon: lon, AltM: ground + h, PitchDeg: 8.1 - 3, HasPitch: true})
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

// A Viper landing at Senaki 09 logs a track the pass card can draw.
func TestTrackString(t *testing.T) {
	flyModel = "F-16C_50"
	defer func() { flyModel = "FA-18C_hornet" }()
	r := one(t, fly(t, glide(3.0), center))
	if r.CourseDeg < 99 || r.CourseDeg > 102 {
		t.Errorf("course %.1f, want ~100.7", r.CourseDeg)
	}
	pts := strings.Split(r.Track, ",")
	if len(pts) < 20 || len(pts) > maxTrackPoints {
		t.Fatalf("%d track points, want 20..%d", len(pts), maxTrackPoints)
	}
	var before, ft, right, kts float64
	if _, err := fmt.Sscanf(pts[0], "%f:%f:%f:%f", &before, &ft, &right, &kts); err != nil || before > 2*nm {
		t.Errorf("first point %q", pts[0])
	}
	t.Logf("SUMMARY %s", r.Summary())
	t.Logf("TRACK %s", r.Track)
}

// The landing reference's two standards: the same touchdown grades differently for a Hornet and a Viper.
func TestProfiles(t *testing.T) {
	td := func(model string, past, sink, pitch float64) string {
		r := Result{Model: model, Outcome: "landed", HasTD: true, TDPastM: past, TDSinkFpm: sink, TDPitchDeg: pitch, HasTDPitch: true,
			HasRollout: true, RolloutNm: 1.2}
		return grade(r)
	}
	cases := []struct {
		model             string
		past, sink, pitch float64
		want              string
	}{
		{"F-16C_50", 300, 250, 11, "_OK_"},              // the Viper's zone and sink
		{"FA-18C_hornet", 300, 250, 6, "_OK_"},
		{"F-16C_50", 500, 250, 11, "_OK_"},              // 1600 ft: Viper OK to 2000 ft...
		{"FA-18C_hornet", 500, 250, 6, "OK: (LNG)TD"},   // ...the Hornet's OK ends at 1500 ft
		{"FA-18C_hornet", 700, 250, 6, "(OK): LNGTD"},   // long over 2000 ft
		{"F-16C_50", 700, 250, 11, "OK: (LNG)TD"},       // Viper long is over 2500 ft; 700 m = 2300 ft
		{"FA-18C_hornet", 300, 700, 6, "_OK_"},          // Hornet sink OK to 800 fpm
		{"F-16C_50", 300, 500, 11, "OK: (HARD)"},        // Viper over 450 is firm...
		{"F-16C_50", 300, 800, 11, "(OK): HARD"},        // ...over 700 is hard
		{"F-16C_50", 300, 250, 16, "(OK): TAIL"},        // pitch over 15: tail strike
		{"FA-18C_hornet", 300, 250, 16, "_OK_"},         // not graded for the Hornet
		{"F-16C_50", 30, 250, 11, "(OK): SHTTD"},        // under 200 ft short
		{"F-14B", 500, 700, 6, "_OK_"},                   // Tomcat: touchdown to 2000 ft, sink as the Hornet
		{"F-14B", 700, 700, 6, "OK: (LNG)TD"},   // 2300 ft: a little long
	}
	for _, c := range cases {
		if got := td(c.model, c.past, c.sink, c.pitch); got != c.want {
			t.Errorf("%s %.0f m %.0f fpm %.0f deg: %q, want %q", c.model, c.past, c.sink, c.pitch, got, c.want)
		}
	}
	if ProfileFor("F-14B").Name != "F-14" || ProfileFor("F-16C_50").GlideDeg != 2.75 || ProfileFor("FA-18C_hornet").GlideDeg != 3.0 || ProfileFor("").Name != "F/A-18C" {
		t.Error("profile selection")
	}
}
