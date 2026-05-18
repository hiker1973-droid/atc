// Scud / launch-warning watcher — optional standalone mode of atc.exe.
// Run via --scudwatch-only (see run_scudwatch.bat). Reads Tacview real-time
// telemetry, watches for ballistic (Scud) and general missile launches, and
// broadcasts TTS alerts on a dedicated SRS frequency via ExternalAudio.
package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// scudwatchEvent carries a weapon observation from the Tacview feed to a monitor.
type scudwatchEvent struct {
	id        string
	name      string
	typ       string
	coalition string
	lon, lat  float64
	altFt     float64
}

// startScudMonitor announces ballistic / Scud-family launches in AWACS
// brevity format. Consumes events from ch until ctx is cancelled.
func startScudMonitor(ctx context.Context, ch <-chan scudwatchEvent, announce func(string), bullseye orb.Point, callsign string) {
	cs := speakableCallsign(callsign)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			brg, nm := bearingRangeFromRef(bullseye, ev.lon, ev.lat)
			text := fmt.Sprintf(
				"All stations, %s, Scud launch, Scud launch, bullseye %s for %d, angels %d, tracking.",
				cs, spokenBearing(brg), int(nm+0.5), int(ev.altFt/1000+0.5),
			)
			announce(text)
		}
	}
}

// startLaunchWarningMonitor announces generic missile launches. One call per
// launch, no repetition — keeps the channel quiet during busy fights.
func startLaunchWarningMonitor(ctx context.Context, ch <-chan scudwatchEvent, announce func(string), bullseye orb.Point, callsign string) {
	cs := speakableCallsign(callsign)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			brg, nm := bearingRangeFromRef(bullseye, ev.lon, ev.lat)
			name := ev.name
			if name == "" {
				name = "missile"
			}
			text := fmt.Sprintf(
				"All stations, %s, missile launch, %s, bullseye %s for %d.",
				cs, name, spokenBearing(brg), int(nm+0.5),
			)
			announce(text)
		}
	}
}

// speakableCallsign turns "Darkstar-1-1" into "Darkstar 1 1" so OpenAI TTS
// pronounces the digits naturally instead of saying "dash".
func speakableCallsign(callsign string) string {
	return strings.ReplaceAll(callsign, "-", " ")
}

// scudwatchTXWorker serializes TTS synthesis + ExternalAudio broadcasts so
// two near-simultaneous events don't step on each other on the same freq.
func scudwatchTXWorker(ctx context.Context, queue <-chan string,
	freqMHz float64, callsign, apiKey, voice, srsPort, externalAudio string) {
	for {
		select {
		case <-ctx.Done():
			return
		case text, ok := <-queue:
			if !ok {
				return
			}
			log.Info().Str("text", text).Msg("scudwatch TX")
			mp3, err := synthesizeSpeech(ctx, apiKey, text, voice, flagTTSSpeed)
			if err != nil {
				log.Error().Err(err).Msg("scudwatch TTS failed")
				continue
			}
			transmitExternalAudioFile(ctx, mp3, freqMHz, callsign, "", srsPort, externalAudio)
			// Gap between back-to-back calls so ExternalAudio finishes cleanly.
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}

// scudwatchTacviewFeed owns a single Tacview TCP connection, parses ACMI
// updates, and fans weapon events out to scudCh / launchCh. Reconnects on
// drop until ctx is cancelled.
func scudwatchTacviewFeed(ctx context.Context, addr string, scudCh, launchCh chan<- scudwatchEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := scudwatchTacviewSession(ctx, addr, scudCh, launchCh); err != nil {
			log.Warn().Err(err).Msg("scudwatch Tacview session ended")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func scudwatchTacviewSession(ctx context.Context, addr string, scudCh, launchCh chan<- scudwatchEvent) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	log.Info().Str("addr", addr).Msg("scudwatch Tacview connected")
	conn.Write([]byte("XtraLib.Stream.0\nTacview.RealTimeTelemetry.0\nvSFG7-Scudwatch\n0\x00"))
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

	// Suppress launch events during the initial world-state dump. Scud
	// alerts still fire so an already-airborne Scud on restart is announced.
	initialStateUntil := time.Now().Add(5 * time.Second)

	type obj struct {
		name, typ, coalition string
		lon, lat, altFt      float64
		hasPos               bool
		firedScud            bool
		firedLaunch          bool
	}
	objects := make(map[string]*obj)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 65536), 65536)
	for scanner.Scan() {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			delete(objects, strings.TrimPrefix(line, "-"))
			continue
		}

		parts := strings.SplitN(line, ",", 2)
		if len(parts) < 2 {
			continue
		}
		id, props := parts[0], parts[1]
		if id == "0" {
			continue // Tacview global-state object (reference lat/lon/time)
		}

		o := objects[id]
		if o == nil {
			o = &obj{}
			objects[id] = o
		}
		if v := extractACMIProp(props, "Name"); v != "" {
			o.name = v
		}
		if v := extractACMIProp(props, "Type"); v != "" {
			o.typ = v
		}
		if v := extractACMIProp(props, "Coalition"); v != "" {
			o.coalition = v
		}
		if t := extractACMIProp(props, "T"); t != "" {
			c := strings.Split(t, "|")
			if len(c) >= 3 {
				fmt.Sscanf(c[0], "%f", &o.lon)
				fmt.Sscanf(c[1], "%f", &o.lat)
				var altM float64
				fmt.Sscanf(c[2], "%f", &altM)
				o.altFt = altM * 3.28084
				o.hasPos = true
			}
		}

		// Filter: weapons with a position. Only announce enemy launches so
		// friendly AAM shots don't spam the channel.
		if !o.hasPos {
			continue
		}
		if !strings.Contains(o.typ, "Missile") && !isScudName(o.name) {
			continue
		}
		if o.coalition != "" && !strings.EqualFold(o.coalition, "Enemies") && !strings.EqualFold(o.coalition, "Red") {
			continue
		}

		ev := scudwatchEvent{
			id: id, name: o.name, typ: o.typ, coalition: o.coalition,
			lon: o.lon, lat: o.lat, altFt: o.altFt,
		}
		scud := isScudWeapon(o.name, o.typ)
		if scud && !o.firedScud {
			o.firedScud = true
			select {
			case scudCh <- ev:
			default:
				log.Warn().Str("id", id).Msg("scud channel full")
			}
			continue
		}
		if !scud && !o.firedLaunch && time.Now().After(initialStateUntil) {
			o.firedLaunch = true
			select {
			case launchCh <- ev:
			default:
				log.Warn().Str("id", id).Msg("launch channel full")
			}
		}
	}
	return scanner.Err()
}

func isScudName(name string) bool {
	return strings.Contains(strings.ToLower(name), "scud")
}

func isScudWeapon(name, typ string) bool {
	if isScudName(name) {
		return true
	}
	if strings.Contains(strings.ToLower(typ), "ballistic") {
		return true
	}
	return false
}

// bearingRangeFromRef returns (true-bearing-deg, range-nm) from ref to (lon,lat).
func bearingRangeFromRef(ref orb.Point, lon, lat float64) (float64, float64) {
	lat1 := ref[1] * math.Pi / 180
	lat2 := lat * math.Pi / 180
	dLon := (lon - ref[0]) * math.Pi / 180

	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	brg := math.Atan2(y, x) * 180 / math.Pi
	if brg < 0 {
		brg += 360
	}

	a := math.Sin((lat2-lat1)/2)*math.Sin((lat2-lat1)/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return brg, 3440.065 * c // earth radius, nm
}

// spokenBearing formats a bearing digit-by-digit for TTS, e.g. 315 → "three-one-five".
func spokenBearing(deg float64) string {
	n := int(deg+0.5) % 360
	if n < 0 {
		n += 360
	}
	digits := fmt.Sprintf("%03d", n)
	words := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "niner"}
	parts := make([]string, 0, 3)
	for _, c := range digits {
		parts = append(parts, words[c-'0'])
	}
	return strings.Join(parts, "-")
}

// cardinalDirection returns "north", "north-east", ... for a 0–360 bearing.
func cardinalDirection(deg float64) string {
	n := int(math.Mod(deg+22.5+360, 360)/45) % 8
	return []string{"north", "north-east", "east", "south-east", "south", "south-west", "west", "north-west"}[n]
}
