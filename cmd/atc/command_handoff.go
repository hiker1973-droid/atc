package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
)

const (
	handoffThresholdNm   = 30.0
	handoffCheckInterval = 30 * time.Second
	pilotIdleTimeout     = 60 * time.Minute
)

type trackedPilot struct {
	callsign   string
	lastSeen   time.Time
	lastDistNm float64
	handedOff  bool
}

type pilotTracker struct {
	mu     sync.Mutex
	pilots map[string]*trackedPilot
}

func newPilotTracker() *pilotTracker {
	return &pilotTracker{pilots: make(map[string]*trackedPilot)}
}

// Note records that this callsign has transmitted to Command.
func (t *pilotTracker) Note(callsign string) {
	if callsign == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.pilots[callsign]; ok {
		p.lastSeen = time.Now()
		return
	}
	t.pilots[callsign] = &trackedPilot{
		callsign:   callsign,
		lastSeen:   time.Now(),
		lastDistNm: -1,
	}
}

type tacviewPositions struct {
	mu  sync.RWMutex
	pos map[string]orb.Point
}

func newTacviewPositions() *tacviewPositions {
	return &tacviewPositions{pos: make(map[string]orb.Point)}
}

func (t *tacviewPositions) Get(cs string) (orb.Point, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	p, ok := t.pos[cs]
	if ok {
		return p, true
	}
	// Tacview sometimes carries uppercased callsigns; try case-insensitive.
	csLower := strings.ToLower(cs)
	for k, v := range t.pos {
		if strings.ToLower(k) == csLower {
			return v, true
		}
	}
	return orb.Point{}, false
}

func (t *tacviewPositions) Set(cs string, p orb.Point) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pos[cs] = p
}

func (t *tacviewPositions) Remove(cs string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pos, cs)
}

// runMiniTacview is a stripped-down Tacview consumer for Command. It only
// maintains a callsign → lat/lon map — no phase detection, conflict logic,
// or controller wiring. Mirrors the connection/handshake of the main
// tacviewLoop in main.go but drops everything we don't need.
func runMiniTacview(ctx context.Context, addr string, store *tacviewPositions) {
	objectNames := make(map[string]string)
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
		log.Info().Str("addr", addr).Msg("Command: Tacview mini-tracker connected")

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		conn.Write([]byte("XtraLib.Stream.0\nTacview.RealTimeTelemetry.0\nvSFG7-Command\n0\x00"))

		var refLat, refLon float64
		var refSet bool

		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 65536), 65536)
		for scanner.Scan() {
			conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
			select {
			case <-ctx.Done():
				conn.Close()
				return
			default:
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "-") {
				// Object destroyed — remove from map if we have it by name.
				id := strings.TrimPrefix(line, "-")
				if name, ok := objectNames[id]; ok {
					store.Remove(name)
					delete(objectNames, id)
				}
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) < 2 {
				continue
			}
			id := parts[0]
			rest := parts[1]
			// Object "0" carries the global ReferenceLatitude/ReferenceLongitude.
			if id == "0" {
				for _, fld := range strings.Split(rest, ",") {
					if strings.HasPrefix(fld, "ReferenceLatitude=") {
						refLat, _ = strconv.ParseFloat(strings.TrimPrefix(fld, "ReferenceLatitude="), 64)
						refSet = true
					}
					if strings.HasPrefix(fld, "ReferenceLongitude=") {
						refLon, _ = strconv.ParseFloat(strings.TrimPrefix(fld, "ReferenceLongitude="), 64)
						refSet = true
					}
				}
				continue
			}
			// Per-object lines: T=lon|lat|alt|... and Pilot=... / Name=...
			var pilot, name string
			var lon, lat float64
			var hasPos bool
			for _, fld := range strings.Split(rest, ",") {
				switch {
				case strings.HasPrefix(fld, "T="):
					coords := strings.Split(strings.TrimPrefix(fld, "T="), "|")
					if len(coords) >= 2 {
						l, errL := strconv.ParseFloat(coords[0], 64)
						a, errA := strconv.ParseFloat(coords[1], 64)
						if errL == nil && errA == nil {
							lon, lat, hasPos = l, a, true
						}
					}
				case strings.HasPrefix(fld, "Pilot="):
					pilot = strings.TrimPrefix(fld, "Pilot=")
				case strings.HasPrefix(fld, "Name="):
					name = strings.TrimPrefix(fld, "Name=")
				}
			}
			if !hasPos {
				continue
			}
			if refSet {
				lon += refLon
				lat += refLat
			}
			// Pilot field uses "Raider 032 |Jedi" — keep the callsign half only.
			key := pilot
			if key == "" {
				key = name
			}
			if i := strings.Index(key, "|"); i >= 0 {
				key = strings.TrimSpace(key[:i])
			}
			if key == "" {
				continue
			}
			objectNames[id] = key
			store.Set(key, orb.Point{lon, lat})
		}
		conn.Close()
		log.Warn().Msg("Command: Tacview mini-tracker disconnected, reconnecting in 5s")
		time.Sleep(5 * time.Second)
	}
}

// runCommandHandoffWatch periodically inspects pilots that have transmitted
// to Command and TXs a tower-handoff once the pilot is inside the threshold
// AND has been closing (was outside the threshold the previous check).
func runCommandHandoffWatch(
	ctx context.Context,
	tracker *pilotTracker,
	store *tacviewPositions,
	tx func(text, callsign string),
) {
	fields := []*airfield.Airfield{airfield.OMDM, airfield.OMAM, airfield.OMAL}
	ticker := time.NewTicker(handoffCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		now := time.Now()
		tracker.mu.Lock()
		for cs, pilot := range tracker.pilots {
			if now.Sub(pilot.lastSeen) > pilotIdleTimeout {
				delete(tracker.pilots, cs)
				continue
			}
			if pilot.handedOff {
				continue
			}
			pos, ok := store.Get(cs)
			if !ok {
				continue
			}
			var nearestFld *airfield.Airfield
			nearestDist := 9999.0
			for _, fld := range fields {
				d := haversineNm(pos, fld.Center)
				if d < nearestDist {
					nearestDist = d
					nearestFld = fld
				}
			}
			// Trigger only when closing through the threshold — prevents
			// firing when pilot starts a sortie already within range, and
			// prevents repeat fires when they orbit at the edge.
			if pilot.lastDistNm > handoffThresholdNm && nearestDist <= handoffThresholdNm {
				text := fmt.Sprintf(
					"%s, vSFG-7-Command, contact %s tower on %s, switching now approved, good landing.",
					cs, nearestFld.Name, formatFreqMHz(nearestFld.TowerFreqMHz),
				)
				tx(text, cs)
				pilot.handedOff = true
				log.Info().
					Str("callsign", cs).
					Str("tower", nearestFld.Name).
					Float64("dist", nearestDist).
					Msg("Command: proactive tower handoff issued")
			}
			pilot.lastDistNm = nearestDist
		}
		tracker.mu.Unlock()
	}
}

// haversineNm — local copy to avoid pulling the controller package into Command.
func haversineNm(a, b orb.Point) float64 {
	const earthRadiusNm = 3440.065
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	lat1, lat2 := toRad(a[1]), toRad(b[1])
	dLat := lat2 - lat1
	dLon := toRad(b[0]) - toRad(a[0])
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusNm * math.Asin(math.Sqrt(h))
}

// formatFreqMHz renders a freq in a TTS-friendly way: "two eight two point zero zero zero".
func formatFreqMHz(mhz float64) string {
	digits := fmt.Sprintf("%.3f", mhz)
	words := map[rune]string{
		'0': "zero", '1': "one", '2': "two", '3': "three", '4': "four",
		'5': "five", '6': "six", '7': "seven", '8': "eight", '9': "nine",
		'.': "point",
	}
	var out []string
	for _, ch := range digits {
		if w, ok := words[ch]; ok {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}
