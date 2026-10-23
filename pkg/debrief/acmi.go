// Package debrief turns a session's records - a Tacview recording, the DCS
// server's dcs.log and Readback's own radio logs - into a compact text the
// launcher's Reports page sends to Claude for a written debrief (operator
// 2026-10-23: "lets add reporting functions", "load dcs log and tacview").
// Everything is summarised here, on the PC: a recording of hundreds of MB
// becomes a few KB of flights, shots and losses.
package debrief

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Flight is one aircraft in a Tacview recording.
type Flight struct {
	ID        string
	Pilot     string // the callsign or player name Tacview shows
	Aircraft  string
	Group     string
	Coalition string
	Human     bool // DCS marks client aircraft with a player name unlike the unit name
	FirstSeen time.Time
	LastSeen  time.Time
	MaxAltFt  float64
	Takeoffs  []string // "18:31:05Z Senaki-Kolkhi"
	Landings  []string
	Fired     map[string]int // weapon type -> count
	Kills     []string       // "18:40:12Z MiG-29A (Enemies)" - likely, from the weapon's impact point
	Fate      string         // landed | destroyed | ... at the end of the recording
	KilledBy  string
}

// Loss is a destroyed object that was not a tracked aircraft (ground units,
// ships) or any aircraft, for the totals.
type Loss struct {
	At        time.Time
	Name      string
	Type      string
	Coalition string
	By        string // likely shooter's pilot, if a weapon hit near it
}

// Recording is the summary of a Tacview .acmi.
type Recording struct {
	Title        string
	Start, End   time.Time // real-world UTC (RecordingTime + frame offsets)
	MissionTime  string    // ReferenceTime: the in-mission date and time
	Flights      []*Flight
	Losses       []Loss
	WeaponsTotal int
}

type acmiObject struct {
	id, typ, name, pilot, group, coalition, color string
	lon, lat, altM                                float64
	hasPos                                        bool
	first, last                                   time.Time
	flight                                        *Flight
	shooter                                       *Flight // weapons: the aircraft that fired it
	weapon                                        string
	gone                                          bool
}

// ParseACMIFile reads a Tacview recording: text .acmi or the zipped
// .zip.acmi Tacview saves by default.
func ParseACMIFile(path string) (*Recording, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	magic := make([]byte, 2)
	io.ReadFull(f, magic)
	if bytes.Equal(magic, []byte("PK")) {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, fmt.Errorf("the Tacview file looks zipped but cannot be opened: %w", err)
		}
		defer zr.Close()
		for _, zf := range zr.File {
			if strings.HasSuffix(strings.ToLower(zf.Name), ".acmi") || len(zr.File) == 1 {
				rc, err := zf.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return ParseACMI(rc)
			}
		}
		return nil, fmt.Errorf("no .acmi inside the zip")
	}
	f.Seek(0, io.SeekStart)
	return ParseACMI(f)
}

// ParseACMI reads a text ACMI 2.x stream.
func ParseACMI(r io.Reader) (*Recording, error) {
	rec := &Recording{}
	objs := map[string]*acmiObject{}
	var base time.Time
	var offset float64
	now := func() time.Time { return base.Add(time.Duration(offset * float64(time.Second))) }
	var recentWeapons []*acmiObject // removed weapons, for hit attribution

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	header := 0
	var pending string
	for sc.Scan() {
		line := sc.Text()
		if strings.HasSuffix(line, `\`) && !strings.HasSuffix(line, `\\`) { // continued on the next line
			pending += strings.TrimSuffix(line, `\`) + "\n"
			continue
		}
		line, pending = pending+line, ""
		line = strings.TrimPrefix(line, "\ufeff")
		if header < 2 && (strings.HasPrefix(line, "FileType=") || strings.HasPrefix(line, "FileVersion=")) {
			if strings.HasPrefix(line, "FileType=") && !strings.Contains(line, "acmi") {
				return nil, fmt.Errorf("not a Tacview text recording (%s)", line)
			}
			header++
			continue
		}
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		switch line[0] {
		case '#':
			if v, err := strconv.ParseFloat(line[1:], 64); err == nil {
				offset = v
			}
			continue
		case '-':
			if o := objs[line[1:]]; o != nil && !o.gone {
				o.gone, o.last = true, now()
				if o.shooter != nil {
					recentWeapons = append(recentWeapons, o)
					if len(recentWeapons) > 200 {
						recentWeapons = recentWeapons[100:]
					}
				}
				if o.flight != nil {
					o.flight.LastSeen = o.last
				}
			}
			continue
		}
		id, props, ok := strings.Cut(line, ",")
		if !ok {
			continue
		}
		if id == "0" {
			for _, p := range splitProps(props) {
				k, v, _ := strings.Cut(p, "=")
				switch k {
				case "RecordingTime":
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						base = t
					}
				case "ReferenceTime":
					rec.MissionTime = v
					if base.IsZero() {
						if t, err := time.Parse(time.RFC3339, v); err == nil {
							base = t
						}
					}
				case "Title":
					rec.Title = v
				case "Event":
					handleEvent(rec, objs, v, now(), &recentWeapons)
				}
			}
			continue
		}
		o := objs[id]
		if o == nil {
			o = &acmiObject{id: id, first: now()}
			objs[id] = o
		}
		o.last = now()
		for _, p := range splitProps(props) {
			k, v, _ := strings.Cut(p, "=")
			switch k {
			case "T":
				parts := strings.Split(v, "|")
				set := func(i int, dst *float64) {
					if i < len(parts) && parts[i] != "" {
						if f, err := strconv.ParseFloat(parts[i], 64); err == nil {
							*dst = f
						}
					}
				}
				set(0, &o.lon)
				set(1, &o.lat)
				set(2, &o.altM)
				o.hasPos = true
			case "Type":
				o.typ = v
			case "Name":
				o.name = v
			case "Pilot":
				o.pilot = v
			case "Group":
				o.group = v
			case "Coalition":
				o.coalition = v
			case "Color":
				o.color = v
			}
		}
		track(rec, objs, o)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	rec.Start, rec.End = base, now()
	for _, f := range rec.Flights {
		if f.Fate == "" {
			if o := objs[f.ID]; o != nil && o.gone {
				f.Fate = "left the recording"
			} else {
				f.Fate = "still in the air or on the ground at the end"
			}
		}
	}
	sort.Slice(rec.Flights, func(i, j int) bool { return rec.Flights[i].FirstSeen.Before(rec.Flights[j].FirstSeen) })
	return rec, nil
}

// track keeps an aircraft's flight record and notes weapons with the
// aircraft nearest their launch point as the shooter.
func track(rec *Recording, objs map[string]*acmiObject, o *acmiObject) {
	switch {
	case strings.Contains(o.typ, "Air") && (strings.Contains(o.typ, "FixedWing") || strings.Contains(o.typ, "Rotorcraft")):
		if o.flight == nil {
			o.flight = &Flight{ID: o.id, FirstSeen: o.first, Fired: map[string]int{}}
			rec.Flights = append(rec.Flights, o.flight)
		}
		f := o.flight
		f.Pilot, f.Aircraft, f.Group, f.Coalition = o.pilot, o.name, o.group, side(o)
		f.Human = o.pilot != "" && o.group != "" && !strings.HasPrefix(o.pilot, o.group)
		f.LastSeen = o.last
		if alt := o.altM * 3.28084; alt > f.MaxAltFt {
			f.MaxAltFt = alt
		}
	case o.shooter == nil && o.weapon == "" && strings.Contains(o.typ, "Weapon") &&
		(strings.Contains(o.typ, "Missile") || strings.Contains(o.typ, "Bomb") || strings.Contains(o.typ, "Rocket")):
		if !o.hasPos {
			return
		}
		o.weapon = o.name
		if o.weapon == "" {
			o.weapon = o.typ
		}
		rec.WeaponsTotal++
		var best *acmiObject
		bestD := 3000.0 // metres: a launch further than this from any aircraft is a ground launcher's
		for _, a := range objs {
			if a.flight == nil || a.gone || !a.hasPos {
				continue
			}
			if d := distM(a, o); d < bestD {
				best, bestD = a, d
			}
		}
		if best != nil {
			o.shooter = best.flight
			best.flight.Fired[o.weapon]++
		} else {
			o.shooter = &Flight{Pilot: "a ground launcher"}
		}
	}
}

// handleEvent: Tacview's global events, "Kind|objectID|...|text".
func handleEvent(rec *Recording, objs map[string]*acmiObject, v string, at time.Time, recent *[]*acmiObject) {
	parts := strings.Split(v, "|")
	if len(parts) < 2 {
		return
	}
	o := objs[parts[1]]
	text := ""
	if len(parts) > 2 {
		text = parts[len(parts)-1]
	}
	if o == nil {
		return
	}
	stamp := at.UTC().Format("15:04:05Z")
	if o.flight != nil && at.After(o.flight.LastSeen) {
		o.flight.LastSeen = at
	}
	switch parts[0] {
	case "TakenOff":
		if o.flight != nil {
			o.flight.Takeoffs = append(o.flight.Takeoffs, strings.TrimSpace(stamp+" "+airportOf(text)))
		}
	case "Landed":
		if o.flight != nil {
			o.flight.Landings = append(o.flight.Landings, strings.TrimSpace(stamp+" "+airportOf(text)))
			o.flight.Fate = "landed"
		}
	case "Destroyed":
		if o.weapon != "" { // a weapon "destroyed" is its impact
			return
		}
		by := shooterOf(o, at, *recent, objs)
		what := o.name
		if o.flight != nil {
			o.flight.Fate, o.flight.KilledBy = "destroyed at "+stamp, by
			if o.flight.Pilot != "" {
				what = o.flight.Pilot + " (" + o.name + ")"
			}
		}
		rec.Losses = append(rec.Losses, Loss{At: at, Name: what, Type: o.typ, Coalition: side(o), By: by})
		if by != "" {
			for _, a := range objs {
				if a.flight != nil && a.flight.Pilot == by {
					a.flight.Kills = append(a.flight.Kills, fmt.Sprintf("%s %s (%s)", stamp, what, side(o)))
					break
				}
			}
		}
	}
}

// shooterOf: the shooter of the weapon that ended nearest the target in the
// last few seconds, if one ended close enough to count.
func shooterOf(target *acmiObject, at time.Time, recent []*acmiObject, objs map[string]*acmiObject) string {
	var best *acmiObject
	bestD := 600.0
	consider := func(w *acmiObject) {
		if w.shooter == nil || !w.hasPos || !target.hasPos || at.Sub(w.last) > 8*time.Second || w.last.Sub(at) > 2*time.Second {
			return
		}
		if d := distM(w, target); d < bestD {
			best, bestD = w, d
		}
	}
	for _, w := range recent {
		consider(w)
	}
	for _, w := range objs {
		if w.weapon != "" && !w.gone {
			consider(w)
		}
	}
	if best == nil {
		return ""
	}
	who := best.shooter.Pilot
	if who == "" {
		who = best.shooter.Aircraft
	}
	return who
}

func side(o *acmiObject) string {
	if o.coalition != "" {
		return o.coalition
	}
	return o.color
}

func airportOf(text string) string {
	// DCS export: "Venom 1-1 has taken off from Senaki-Kolkhi."
	for _, k := range []string{" from ", " at "} {
		if _, after, ok := strings.Cut(text, k); ok {
			return strings.TrimSuffix(strings.TrimSpace(after), ".")
		}
	}
	return ""
}

// splitProps splits on commas that are not escaped ("\,").
func splitProps(s string) []string {
	var out []string
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			b.WriteByte(s[i+1])
			i++
		case s[i] == ',':
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(s[i])
		}
	}
	return append(out, b.String())
}

func distM(a, b *acmiObject) float64 {
	const R = 6371000.0
	lat1, lat2 := a.lat*math.Pi/180, b.lat*math.Pi/180
	dLat, dLon := lat2-lat1, (b.lon-a.lon)*math.Pi/180
	x := dLon * math.Cos((lat1+lat2)/2)
	h := math.Hypot(x*R, dLat*R)
	return math.Hypot(h, a.altM-b.altM)
}
