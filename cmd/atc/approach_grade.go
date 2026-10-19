package main

import (
	"bufio"
	"context"
	flag "github.com/spf13/pflag"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/fieldgrade"
)

// Field approach grading (pkg/fieldgrade): the tower grades every player
// landing and go-around at its own field, Paddles-style, into its log for
// the daily report. Nothing goes on the radio (operator 2026-10-18: "report
// only for test. Senaki is the only field").

var flagApproachGrade bool

func registerApproachGradeFlags(f *flag.FlagSet) {
	f.BoolVar(&flagApproachGrade, "approach-grade", false,
		"Grade player landings at this field from Tacview into the log (report only, nothing said)")
}

// gradeField is the airfield's runways as fieldgrade sees them.
func gradeField(af *airfield.Airfield) fieldgrade.Field {
	var pairs [][2]fieldgrade.Threshold
	for _, p := range af.RunwayPairs {
		pairs = append(pairs, [2]fieldgrade.Threshold{
			{Name: p.Primary.Designator, Lat: p.Primary.ThresholdLatLon[1], Lon: p.Primary.ThresholdLatLon[0]},
			{Name: p.Reciprocal.Designator, Lat: p.Reciprocal.ThresholdLatLon[1], Lon: p.Reciprocal.ThresholdLatLon[0]},
		})
	}
	return fieldgrade.NewField(af.Name, float64(af.ElevationFt), pairs...)
}

// fieldTacview turns ACMI lines into fieldgrade updates for player jets.
type fieldTacview struct {
	tr             *fieldgrade.Tracker
	objs           map[string]*lsoObj
	refLat, refLon float64
	lastExpire     time.Time
	now            func() time.Time
}

func newFieldTacview(tr *fieldgrade.Tracker) *fieldTacview {
	return &fieldTacview{tr: tr, objs: make(map[string]*lsoObj), now: time.Now}
}

func (t *fieldTacview) handleLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
		return
	}
	if strings.HasPrefix(line, "-") {
		delete(t.objs, strings.TrimPrefix(line, "-"))
		return
	}
	id, rest, ok := strings.Cut(line, ",")
	if !ok {
		return
	}
	if id == "0" {
		for _, fld := range strings.Split(rest, ",") {
			k, v, _ := strings.Cut(fld, "=")
			switch k {
			case "ReferenceLatitude":
				t.refLat, _ = strconv.ParseFloat(v, 64)
			case "ReferenceLongitude":
				t.refLon, _ = strconv.ParseFloat(v, 64)
			}
		}
		return
	}
	o := t.objs[id]
	if o == nil {
		o = &lsoObj{}
		t.objs[id] = o
	}
	moved := false
	for _, fld := range strings.Split(rest, ",") {
		k, v, ok := strings.Cut(fld, "=")
		if !ok {
			continue
		}
		switch k {
		case "T":
			moved = true
			c := strings.Split(v, "|")
			num := func(i int) (float64, bool) {
				if i >= len(c) || c[i] == "" {
					return 0, false
				}
				f, err := strconv.ParseFloat(c[i], 64)
				return f, err == nil
			}
			if f, ok := num(0); ok {
				o.lon, o.hasLon = f, true
			}
			if f, ok := num(1); ok {
				o.lat, o.hasLat = f, true
			}
			if f, ok := num(2); ok {
				o.altM = f
			}
			if len(c) == 6 || len(c) == 9 {
				if f, ok := num(4); ok {
					o.pitch, o.hasPitch = f, true
				}
			}
		case "Pilot":
			o.pilot = v
		case "Name":
			o.name = v
		case "Type":
			o.typ = v
		}
	}
	// Player jets only: DCS writes a human's Pilot as "Raider 302 | Jedi".
	if !moved || !o.hasLat || !o.hasLon || !strings.Contains(o.typ, "FixedWing") || !strings.Contains(o.pilot, "|") {
		return
	}
	now := t.now()
	cs := strings.TrimSpace(o.pilot[:strings.Index(o.pilot, "|")])
	t.tr.Update(cs, o.name, fieldgrade.Sample{At: now, Lat: o.lat + t.refLat, Lon: o.lon + t.refLon,
		AltM: o.altM, PitchDeg: o.pitch, HasPitch: o.hasPitch})
	if now.Sub(t.lastExpire) > time.Second {
		t.tr.Expire(now)
		t.lastExpire = now
	}
}

func logFieldGrade(r fieldgrade.Result) {
	log.Info().Str("callsign", r.Callsign).Str("runway", r.Runway).Str("grade", r.Grade).
		Str("summary", r.Summary()).Msg("Approach graded")
}

// runFieldGrade has its own Tacview connection, like Paddles.
func runFieldGrade(ctx context.Context, addr string, af *airfield.Airfield) {
	if addr == "" {
		log.Warn().Msg("Approach grading: no --tacview-addr, off")
		return
	}
	f := gradeField(af)
	log.Info().Str("field", f.Name).Int("runway ends", len(f.Ends)).Msg("Approach grading on (report only)")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		conn.Write([]byte("XtraLib.Stream.0\nTacview.RealTimeTelemetry.0\nvSFG7-Approach\n0\x00"))
		tr := fieldgrade.NewTracker(f)
		tr.Done = logFieldGrade
		tv := newFieldTacview(tr)
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 65536), 65536)
		for sc.Scan() {
			conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
			select {
			case <-ctx.Done():
				conn.Close()
				return
			default:
			}
			tv.handleLine(sc.Text())
		}
		conn.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
