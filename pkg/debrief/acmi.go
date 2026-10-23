// Package debrief turns a session's records - the DCS server's dcs.log, a
// Tacview recording, the mission file and Readback's own radio logs - into
// an After Action Report (operator 2026-10-23: "lets add reporting
// functions", "load dcs log and tacview", "we want it to be a after action
// report", then "like this example": the BFM Venom vs Raider debrief).
// The server's event lines give what DCS scored (shots, hits, kills,
// takeoffs, landings, grades); Tacview gives the geometry and the tracks for
// the maps. Everything is computed on the PC; Claude only writes the words.
package debrief

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Point is one sample of an object's track.
type Point struct {
	T        float64 // seconds since the recording started
	Lat, Lon float64
	AltM     float64 // above sea level
	AGL      float64 // above the ground, when Tacview gives it (-1 when not)
	U, V     float64 // DCS flat coordinates, metres (0,0 when not given)
}

// Object is one thing in a recording: an aircraft, a missile, a parachute.
type Object struct {
	ID, Type, Name, Pilot, Group, Color string
	Start, End                          float64 // seconds; End is when it was removed (or the recording's end)
	Removed                             bool
	Track                               []Point
}

// Side is "blue", "red", or "" from Tacview's colour (the Coalition field is
// relative to whoever recorded it).
func (o *Object) Side() string {
	switch strings.ToLower(o.Color) {
	case "blue":
		return "blue"
	case "red":
		return "red"
	}
	return ""
}

// IsAircraft: fixed wing or helicopter.
func (o *Object) IsAircraft() bool {
	return strings.HasPrefix(o.Type, "Air+") && (strings.Contains(o.Type, "FixedWing") || strings.Contains(o.Type, "Rotorcraft"))
}

// IsWeapon: a missile, bomb or rocket (shells and bullets are left out).
func (o *Object) IsWeapon() bool {
	return strings.HasPrefix(o.Type, "Weapon") &&
		(strings.Contains(o.Type, "Missile") || strings.Contains(o.Type, "Bomb") || strings.Contains(o.Type, "Rocket"))
}

// At is the track sample nearest t.
func (o *Object) At(t float64) Point {
	tr := o.Track
	if len(tr) == 0 {
		return Point{T: t, AGL: -1}
	}
	k := sort.Search(len(tr), func(i int) bool { return tr[i].T >= t })
	switch {
	case k == 0:
		return tr[0]
	case k == len(tr):
		return tr[len(tr)-1]
	case t-tr[k-1].T < tr[k].T-t:
		return tr[k-1]
	}
	return tr[k]
}

// SpeedKt is the ground speed around t, from the track.
func (o *Object) SpeedKt(t float64) float64 {
	a, b := o.At(t-1.5), o.At(t+0.5)
	dt := b.T - a.T
	if dt < 0.1 {
		return 0
	}
	return dist3(a, b) / dt * 1.94384
}

// Alive reports whether the object existed at t.
func (o *Object) Alive(t float64) bool { return t >= o.Start && t <= o.End }

// Recording is a parsed Tacview recording.
type Recording struct {
	Title       string
	MissionTime string    // ReferenceTime: the in-mission date and time
	Start       time.Time // RecordingTime: real UTC at t = 0
	Duration    float64   // seconds
	Comments    string
	Objects     map[string]*Object
	Aircraft    []*Object // in order of appearance
	Weapons     []*Object
	Chutes      []*Object
}

// Time is the real time of recording second t.
func (r *Recording) Time(t float64) time.Time {
	return r.Start.Add(time.Duration(t * float64(time.Second)))
}

// Sec is the recording second of real time at.
func (r *Recording) Sec(at time.Time) float64 { return at.Sub(r.Start).Seconds() }

// End is the real time the recording stops.
func (r *Recording) End() time.Time { return r.Time(r.Duration) }

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
			// A recording Tacview is still writing (the newest one on a live
			// server) has no central directory yet: read the first entry's
			// stream straight from its local header, as far as it goes.
			f.Seek(0, io.SeekStart)
			return parseStreamedZip(f)
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

// parseStreamedZip reads a zip's first entry from its local header, without
// the central directory; a stream that stops early (still being written)
// gives what was recorded so far.
func parseStreamedZip(f io.Reader) (*Recording, error) {
	hdr := make([]byte, 30)
	if _, err := io.ReadFull(f, hdr); err != nil || !bytes.Equal(hdr[:4], []byte("PK\x03\x04")) {
		return nil, fmt.Errorf("the Tacview file looks zipped but cannot be opened")
	}
	method := binary.LittleEndian.Uint16(hdr[8:])
	skip := int64(binary.LittleEndian.Uint16(hdr[26:])) + int64(binary.LittleEndian.Uint16(hdr[28:]))
	if _, err := io.CopyN(io.Discard, f, skip); err != nil {
		return nil, err
	}
	var body io.Reader = f
	if method == 8 {
		body = flate.NewReader(f)
	} else if method != 0 {
		return nil, fmt.Errorf("the Tacview zip uses compression method %d", method)
	}
	return ParseACMI(tolerantReader{body})
}

// tolerantReader ends cleanly where a cut-off deflate stream stops.
type tolerantReader struct{ r io.Reader }

func (t tolerantReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if err != nil && err != io.EOF {
		return n, io.EOF
	}
	return n, err
}

type acmiState struct {
	v       [9]float64
	has     [9]bool
	agl     float64
	lastT   float64
	sampled bool
}

// ParseACMI reads a text ACMI 2.x stream.
func ParseACMI(r io.Reader) (*Recording, error) {
	rec := &Recording{Objects: map[string]*Object{}}
	state := map[string]*acmiState{}
	var refLat, refLon, t float64

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	header := 0
	var pending strings.Builder
	for sc.Scan() {
		line := sc.Text()
		if strings.HasSuffix(line, `\`) && !strings.HasSuffix(line, `\\`) { // continued on the next line
			pending.WriteString(strings.TrimSuffix(line, `\`) + "\n")
			continue
		}
		if pending.Len() > 0 {
			line = pending.String() + line
			pending.Reset()
		}
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
				t = v
			}
			continue
		case '-':
			if o := rec.Objects[line[1:]]; o != nil && !o.Removed {
				o.Removed, o.End = true, t
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
					if tt, err := time.Parse(time.RFC3339, v); err == nil {
						rec.Start = tt
					}
				case "ReferenceTime":
					rec.MissionTime = v
					if rec.Start.IsZero() {
						if tt, err := time.Parse(time.RFC3339, v); err == nil {
							rec.Start = tt
						}
					}
				case "ReferenceLongitude":
					refLon, _ = strconv.ParseFloat(v, 64)
				case "ReferenceLatitude":
					refLat, _ = strconv.ParseFloat(v, 64)
				case "Title":
					rec.Title = v
				case "Comments":
					rec.Comments = v
				}
			}
			continue
		}
		o := rec.Objects[id]
		st := state[id]
		if o == nil {
			o = &Object{ID: id, Start: t}
			rec.Objects[id] = o
			st = &acmiState{agl: -1, lastT: -1e9}
			state[id] = st
		}
		o.End = t
		moved := false
		for _, p := range splitProps(props) {
			k, v, _ := strings.Cut(p, "=")
			switch k {
			case "T":
				parts := strings.Split(v, "|")
				var idx []int
				switch len(parts) {
				case 3:
					idx = []int{0, 1, 2}
				case 5:
					idx = []int{0, 1, 2, 6, 7}
				case 6:
					idx = []int{0, 1, 2, 3, 4, 5}
				default:
					idx = []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
				}
				for i, x := range parts {
					if i < len(idx) && x != "" {
						if f, err := strconv.ParseFloat(x, 64); err == nil {
							st.v[idx[i]], st.has[idx[i]] = f, true
						}
					}
				}
				moved = true
			case "AGL":
				st.agl, _ = strconv.ParseFloat(v, 64)
			case "Type":
				o.Type = v
			case "Name":
				o.Name = v
			case "Pilot":
				o.Pilot = v
			case "Group":
				o.Group = v
			case "Color":
				o.Color = v
			}
		}
		if !moved || !st.has[0] || !st.has[1] {
			continue
		}
		// Tracks for aircraft (1 s), weapons (0.25 s) and parachutes; the rest
		// keep only where they first appeared.
		every := -1.0
		switch {
		case o.IsWeapon():
			every = 0.25
		case o.IsAircraft() || strings.Contains(o.Type, "Parachutist"):
			every = 1
		}
		if (every < 0 && st.sampled) || (every >= 0 && t-st.lastT < every) {
			continue
		}
		st.lastT, st.sampled = t, true
		o.Track = append(o.Track, Point{T: t, Lat: st.v[1] + refLat, Lon: st.v[0] + refLon, AltM: st.v[2], AGL: st.agl, U: st.v[6], V: st.v[7]})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	rec.Duration = t
	for _, o := range rec.Objects {
		if !o.Removed {
			o.End = t
		}
		switch {
		case o.IsAircraft() && len(o.Track) > 0:
			rec.Aircraft = append(rec.Aircraft, o)
		case o.IsWeapon() && len(o.Track) > 0:
			rec.Weapons = append(rec.Weapons, o)
		case strings.Contains(o.Type, "Parachutist") && len(o.Track) > 0:
			rec.Chutes = append(rec.Chutes, o)
		}
	}
	for _, l := range [][]*Object{rec.Aircraft, rec.Weapons, rec.Chutes} {
		sort.Slice(l, func(i, j int) bool { return l[i].Start < l[j].Start || (l[i].Start == l[j].Start && l[i].ID < l[j].ID) })
	}
	return rec, nil
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

// dist3 is the 3-D distance in metres: DCS's flat metres when both points
// have them, else from latitude and longitude.
func dist3(a, b Point) float64 {
	if (a.U != 0 || a.V != 0) && (b.U != 0 || b.V != 0) {
		return math.Sqrt((a.U-b.U)*(a.U-b.U) + (a.V-b.V)*(a.V-b.V) + (a.AltM-b.AltM)*(a.AltM-b.AltM))
	}
	return math.Hypot(groundM(a, b), a.AltM-b.AltM)
}

func groundM(a, b Point) float64 {
	const R = 6371000.0
	lat := (a.Lat + b.Lat) / 2 * math.Pi / 180
	x := (b.Lon - a.Lon) * math.Pi / 180 * math.Cos(lat) * R
	y := (b.Lat - a.Lat) * math.Pi / 180 * R
	return math.Hypot(x, y)
}
