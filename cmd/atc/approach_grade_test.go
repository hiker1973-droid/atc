package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/fieldgrade"
)

// Senaki's runway from the airfield data: 09/27, landing on 09 flies ~100.7
// true in lat/lon (DCS yaw reads 095: its grid is ~6 deg off true there).
func TestGradeFieldSenaki(t *testing.T) {
	f := gradeField(airfield.UGKS)
	if len(f.Ends) != 2 || f.Ends[0].Name != "09" || f.Ends[1].Name != "27" {
		t.Fatalf("ends %+v", f.Ends)
	}
	if c := f.Ends[0].CourseDeg; c < 99.5 || c > 102 {
		t.Errorf("09 course %.1f, want ~100.7 true", c)
	}
	if l := f.Ends[0].LengthM; l < 2000 || l > 2600 {
		t.Errorf("length %.0f m", l)
	}
}

// FIELD_REPLAY=<acmi> replays a Tacview recording through the Senaki grader.
func TestFieldReplay(t *testing.T) {
	path := os.Getenv("FIELD_REPLAY")
	if path == "" {
		t.Skip("FIELD_REPLAY not set")
	}
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	tr := fieldgrade.NewTracker(gradeField(airfield.UGKS))
	var start, clock time.Time
	tr.Done = func(r fieldgrade.Result) { t.Logf("%s %s", clock.Format("15:04:05"), r.Summary()) }
	tv := newFieldTacview(tr)
	tv.now = func() time.Time { return clock }
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "0,RecordingTime="); ok {
			if rt, err := time.Parse(time.RFC3339Nano, v); err == nil {
				start, clock = rt.Local(), rt.Local()
			}
		}
		if strings.HasPrefix(line, "#") {
			if s, err := strconv.ParseFloat(line[1:], 64); err == nil {
				clock = start.Add(time.Duration(s * float64(time.Second)))
			}
		}
		tv.handleLine(line)
	}
	tr.Expire(clock.Add(time.Hour))
}
