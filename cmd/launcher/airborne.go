// Air picture — who is airborne, and where they are relative to the bullseye.
//
// The launcher reads Tacview real-time telemetry (the same ACMI stream the
// scudwatch role consumes) and keeps a live roster of aircraft. Doing it here
// rather than inside atc.exe means the air picture is a launcher-only change:
// no ATC binary rebuild, no roles restarted mid-mission.
//
// Two things about the stream that are easy to get wrong:
//
//   - Positions are deltas from a reference point declared in the header
//     (ReferenceLatitude / ReferenceLongitude), not absolute coordinates.
//   - Side is the Color property, not Coalition. DCS missions routinely map
//     blue to "Enemies" — reading Coalition picks the wrong bullseye.
package main

import (
	"bufio"
	"fmt"
	"math"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Contact is one aircraft in the air picture.
type Contact struct {
	ID       string  `json:"id"`
	Callsign string  `json:"callsign"`
	Aircraft string  `json:"aircraft"`
	Group    string  `json:"group,omitempty"`
	Color    string  `json:"color"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	AltFt    int     `json:"altFt"`
	AGLFt    int     `json:"aglFt"`
	Heading  int     `json:"heading"`
	Airborne bool    `json:"airborne"`
	// Bearing and range from the blue bullseye, true degrees and nautical
	// miles. Zeroed when no bullseye has been seen yet.
	BullBrg int `json:"bullBrg"`
	BullNm  int `json:"bullNm"`
}

// AirPicture is the snapshot served at /api/airborne.
type AirPicture struct {
	Connected bool      `json:"connected"`
	Err       string    `json:"err,omitempty"`
	At        time.Time `json:"at"`
	Bullseye  *LatLon   `json:"bullseye,omitempty"`
	Contacts  []Contact `json:"contacts"`
	OnGround  int       `json:"onGround"`
}

type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// airSource is one Tacview feed: this box, or a rig on the LAN.
//
// The launcher watches every rig's feed itself rather than asking each rig's
// launcher for its own picture. Tacview is reachable across the LAN, so this
// way only the exposed launcher needs the air-picture code — the other rigs
// keep whatever launcher build they are on, and the picker still shows a live
// picture for all of them.
type airSource struct {
	addr string
	mu   sync.Mutex
	pic  AirPicture
}

func (s *airSource) get() AirPicture {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pic
}

func (s *airSource) set(p AirPicture) {
	s.mu.Lock()
	s.pic = p
	s.mu.Unlock()
}

func (s *airSource) setErr(err error) {
	s.set(AirPicture{Connected: false, Err: err.Error(), At: time.Now(), Contacts: []Contact{}})
}

// run keeps the session up, reconnecting on drop.
func (s *airSource) run() {
	for {
		if err := airborneSession(s.addr, s); err != nil {
			s.setErr(err)
		}
		time.Sleep(10 * time.Second)
	}
}

var (
	airMu      sync.Mutex
	airSources = map[string]*airSource{}
)

// startAirborneSources opens one feed for this box and one per remote rig.
// Call after fleetRigs is parsed.
func startAirborneSources() {
	add := func(key, addr string) {
		if addr == "" {
			return
		}
		s := &airSource{addr: addr, pic: AirPicture{Contacts: []Contact{}}}
		airMu.Lock()
		airSources[key] = s
		airMu.Unlock()
		go s.run()
	}
	add("", *flagTacviewAddr) // "" is this box, matching the UI rig value
	for _, r := range fleetRigs {
		if isSelf(r) {
			continue
		}
		add(r.Name, net.JoinHostPort(r.Host, strconv.Itoa(*flagTacviewPort)))
	}
}

// handleAirborne serves ?rig=<name>, or this box when the parameter is absent.
func handleAirborne(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("rig")
	airMu.Lock()
	s, ok := airSources[key]
	airMu.Unlock()
	if !ok {
		http.Error(w, "no air picture configured for that rig", http.StatusNotFound)
		return
	}
	writeJSON(w, s.get())
}

// tvObject is the accumulated state of one object in the stream. Updates are
// sparse — a line carries only the fields that changed — so state persists
// across lines until the object is removed.
type tvObject struct {
	Type       string
	Name       string
	Pilot      string
	Group      string
	Color      string
	Lon, Lat   float64
	AltM       float64
	AGLM       float64
	Heading    float64
	HasPos     bool
	HasHeading bool
}

func airborneSession(addr string, src *airSource) error {
	if addr == "" {
		return fmt.Errorf("no --tacview-addr configured")
	}
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("XtraLib.Stream.0\nTacview.RealTimeTelemetry.0\nvSFG7-Launcher\n0\x00")); err != nil {
		return err
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

	var refLat, refLon float64
	objects := map[string]*tvObject{}
	lastPublish := time.Time{}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 65536), 1024*1024)
	for scanner.Scan() {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			delete(objects, strings.TrimPrefix(line, "-"))
			continue
		}

		fields := splitACMI(line)
		if len(fields) == 0 {
			continue
		}
		id := fields[0]

		// Object 0 is the global header, carrying the reference point every
		// other position is relative to.
		if id == "0" {
			for _, f := range fields[1:] {
				k, v, ok := strings.Cut(f, "=")
				if !ok {
					continue
				}
				switch k {
				case "ReferenceLatitude":
					refLat, _ = strconv.ParseFloat(v, 64)
				case "ReferenceLongitude":
					refLon, _ = strconv.ParseFloat(v, 64)
				}
			}
			continue
		}

		o := objects[id]
		if o == nil {
			o = &tvObject{}
			objects[id] = o
		}
		for _, f := range fields[1:] {
			k, v, ok := strings.Cut(f, "=")
			if !ok {
				continue
			}
			switch k {
			case "T":
				applyTransform(o, v)
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
			case "AGL":
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					o.AGLM = f
				}
			}
		}

		// Republish at most every 2s — the stream is far chattier than anyone
		// needs to watch.
		if time.Since(lastPublish) >= 2*time.Second {
			publishAirPicture(src, objects, refLat, refLon)
			lastPublish = time.Now()
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("tacview stream closed")
}

// applyTransform parses the T= positional field:
// Longitude|Latitude|Altitude|Roll|Pitch|Yaw|U|V|Heading. Any component may be
// empty, meaning "unchanged since the last update".
func applyTransform(o *tvObject, v string) {
	parts := strings.Split(v, "|")
	set := func(i int, dst *float64) bool {
		if i >= len(parts) || parts[i] == "" {
			return false
		}
		f, err := strconv.ParseFloat(parts[i], 64)
		if err != nil {
			return false
		}
		*dst = f
		return true
	}
	gotLon := set(0, &o.Lon)
	gotLat := set(1, &o.Lat)
	if gotLon || gotLat {
		o.HasPos = true
	}
	set(2, &o.AltM)
	// Index 8 is Heading. Shorter transforms (ground objects) have no such
	// field; index 4 is Pitch and must not be mistaken for it.
	if set(8, &o.Heading) {
		o.HasHeading = true
	}
}

// splitACMI splits a record on commas, honouring the backslash escape the ACMI
// text format uses for commas inside values.
func splitACMI(line string) []string {
	var out []string
	var cur strings.Builder
	esc := false
	for _, r := range line {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\':
			esc = true
		case r == ',':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	out = append(out, cur.String())
	return out
}

func publishAirPicture(src *airSource, objects map[string]*tvObject, refLat, refLon float64) {
	var bull *LatLon
	for _, o := range objects {
		if strings.Contains(o.Type, "Bullseye") && strings.EqualFold(o.Color, "Blue") && o.HasPos {
			bull = &LatLon{Lat: refLat + o.Lat, Lon: refLon + o.Lon}
			break
		}
	}

	contacts := []Contact{}
	onGround := 0
	for id, o := range objects {
		if !strings.HasPrefix(o.Type, "Air+") || !o.HasPos {
			continue
		}
		lat, lon := refLat+o.Lat, refLon+o.Lon
		agl := o.AGLM * 3.28084
		airborne := agl > 100
		if !airborne {
			onGround++
			continue
		}
		c := Contact{
			ID:       id,
			Callsign: firstNonEmpty(o.Pilot, o.Group, o.Name, id),
			Aircraft: o.Name,
			Group:    o.Group,
			Color:    o.Color,
			Lat:      lat,
			Lon:      lon,
			AltFt:    int(math.Round(o.AltM * 3.28084)),
			AGLFt:    int(math.Round(agl)),
			Airborne: true,
		}
		if o.HasHeading {
			c.Heading = ((int(math.Round(o.Heading)) % 360) + 360) % 360
		}
		if bull != nil {
			brg, nm := bearingRange(bull.Lat, bull.Lon, lat, lon)
			c.BullBrg, c.BullNm = int(math.Round(brg)), int(math.Round(nm))
		}
		contacts = append(contacts, c)
	}

	// Nearest the bullseye first; without one, alphabetical so the list is at
	// least stable between polls.
	sort.Slice(contacts, func(i, j int) bool {
		if bull != nil && contacts[i].BullNm != contacts[j].BullNm {
			return contacts[i].BullNm < contacts[j].BullNm
		}
		return contacts[i].Callsign < contacts[j].Callsign
	})

	src.set(AirPicture{
		Connected: true,
		At:        time.Now(),
		Bullseye:  bull,
		Contacts:  contacts,
		OnGround:  onGround,
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// bearingRange returns the initial true bearing (degrees) and great-circle
// distance (nautical miles) from one point to another.
func bearingRange(lat1, lon1, lat2, lon2 float64) (float64, float64) {
	const earthNm = 3440.065
	rad := math.Pi / 180
	p1, p2 := lat1*rad, lat2*rad
	dLon := (lon2 - lon1) * rad
	dLat := p2 - p1

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(p1)*math.Cos(p2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	dist := 2 * earthNm * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	y := math.Sin(dLon) * math.Cos(p2)
	x := math.Cos(p1)*math.Sin(p2) - math.Sin(p1)*math.Cos(p2)*math.Cos(dLon)
	brg := math.Atan2(y, x) / rad
	if brg < 0 {
		brg += 360
	}
	return brg, dist
}
