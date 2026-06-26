package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/controller"
)

// towerDashboardPortByICAO maps an airfield ICAO to the dashboard port of the
// tower process that owns that airfield's authoritative state. ATIS polls this
// to keep the broadcasted runway aligned with whatever the tower's dashboard
// has set, since the two processes don't share an ATCController.
var towerDashboardPortByICAO = map[string]int{
	"OMDM": 6001,
	"OMAM": 6002,
	"OMAL": 6003,
}

// fetchTowerRunway returns the active runway reported by the tower /status
// endpoint for the given ICAO, or "" when there's no paired tower (Liwa,
// Kish) or the tower is offline. Short timeout — we never want ATIS to
// stall its broadcast cadence on a hung HTTP call.
func fetchTowerRunway(icao string) string {
	port, ok := towerDashboardPortByICAO[icao]
	if !ok {
		return ""
	}
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/status", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var data struct {
		Tower struct {
			ActiveRunway string `json:"activeRunway"`
		} `json:"tower"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}
	return data.Tower.ActiveRunway
}

type towerWeather struct {
	WindDir, WindKts, CeilFt, VisNm, AltInHg float64
	IsNight                                  bool
}

// fetchTowerWeather pulls the live weather block from the paired tower's
// /status. Returns ok=false for Liwa/Kish (no paired tower) or any HTTP
// failure. Sanity-guarded — altimeter outside [25,32] inHg means the tower
// hasn't seeded weather yet, so we skip rather than zero out the ATIS state.
func fetchTowerWeather(icao string) (towerWeather, bool) {
	var zero towerWeather
	port, ok := towerDashboardPortByICAO[icao]
	if !ok {
		return zero, false
	}
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/status", port))
	if err != nil {
		return zero, false
	}
	defer resp.Body.Close()
	var data struct {
		Tower struct {
			CeilingFt     float64 `json:"ceilingFt"`
			VisibNm       float64 `json:"visibNm"`
			AltimeterInHg float64 `json:"altimeterInHg"`
			WindDir       float64 `json:"windDir"`
			WindKts       float64 `json:"windKts"`
			IsNight       bool    `json:"isNight"`
		} `json:"tower"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return zero, false
	}
	t := data.Tower
	if t.AltimeterInHg < 25.0 || t.AltimeterInHg > 32.0 {
		return zero, false
	}
	return towerWeather{
		WindDir: t.WindDir, WindKts: t.WindKts,
		CeilFt: t.CeilingFt, VisNm: t.VisibNm,
		AltInHg: t.AltimeterInHg, IsNight: t.IsNight,
	}, true
}

type atisStation struct {
	Name      string
	FreqMHz   float64
	Voice     string
	ICAO      string  // for RealWeather lookup
	Advisory  string  // static advisory text
	ILS       string  // e.g. "ILS 110.70 RWY 09. ILS 110.75 RWY 27."
	TACAN     string  // e.g. "TACAN 99X."
}

type atisState struct {
	mu          sync.Mutex
	ident       int     // 0=Alpha, 1=Bravo... cycles 0-25
	altimeter   float64
	windFrom    float64
	windKts     float64
	ceilingFt   float64
	activeRwy   string
	cachedMP3   []byte
	cachePath   string
}

func (s *atisState) weatherKey() string {
	return fmt.Sprintf("%.2f|%.0f|%.0f|%.0f|%s", s.altimeter, s.windFrom, s.windKts, s.ceilingFt, s.activeRwy)
}

func identWord(n int) string {
	words := []string{"Alpha","Bravo","Charlie","Delta","Echo","Foxtrot","Golf","Hotel",
		"India","Juliet","Kilo","Lima","Mike","November","Oscar","Papa",
		"Quebec","Romeo","Sierra","Tango","Uniform","Victor","Whiskey","X-ray","Yankee","Zulu"}
	if n < 0 || n >= len(words) { return "Alpha" }
	return words[n]
}

// spellRunwayWords verbalizes a runway designator ("27" -> "two seven",
// "31L" -> "three one left") for ATIS speech. Mirrors composer.spellRunway,
// which is unexported, to keep ATIS runway phrasing identical to the tower's.
func spellRunwayWords(designator string) string {
	suffix := ""
	digits := designator
	switch {
	case strings.HasSuffix(designator, "L"):
		suffix = " left"
		digits = designator[:len(designator)-1]
	case strings.HasSuffix(designator, "R"):
		suffix = " right"
		digits = designator[:len(designator)-1]
	case strings.HasSuffix(designator, "C"):
		suffix = " center"
		digits = designator[:len(designator)-1]
	}
	dw := map[rune]string{
		'0': "zero", '1': "one", '2': "two", '3': "three",
		'4': "four", '5': "five", '6': "six", '7': "seven",
		'8': "eight", '9': "niner",
	}
	spoken := make([]string, 0, len(digits))
	for _, d := range digits {
		if w, ok := dw[d]; ok {
			spoken = append(spoken, w)
		}
	}
	return strings.Join(spoken, " ") + suffix
}

func buildATISText(station *atisStation, state *atisState) string {
	ident := identWord(state.ident)
	windStr := fmt.Sprintf("wind %03.0f at %d", state.windFrom, int(state.windKts))
	if state.windKts < 3 { windStr = "wind calm" }
	altStr := fmt.Sprintf("%.2f", state.altimeter)
	ceilStr := ""
	if state.ceilingFt > 0 && state.ceilingFt < 9999 {
		ceilStr = fmt.Sprintf(", ceiling %d", int(state.ceilingFt/100)*100)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s information %s. ", station.Name, ident))
	if station.TACAN != "" { sb.WriteString(station.TACAN + " ") }
	if station.ILS != "" { sb.WriteString(station.ILS + " ") }
	sb.WriteString(fmt.Sprintf("%s%s. Altimeter %s. ", windStr, ceilStr, altStr))
	// Announce the live active runway pulled from the paired tower (state.activeRwy
	// is synced via fetchTowerRunway each broadcast). Only stations that have a
	// paired tower get a runway line — Liwa/Kish are advisory-only and have no
	// authoritative runway to mirror.
	if _, paired := towerDashboardPortByICAO[station.ICAO]; paired && state.activeRwy != "" {
		sb.WriteString(fmt.Sprintf("Active runway %s. ", spellRunwayWords(state.activeRwy)))
	}
	sb.WriteString(station.Advisory + " ")
	sb.WriteString(fmt.Sprintf("Advise on initial contact you have information %s.", ident))
	text := sb.String()
	log.Debug().Str("station", station.Name).Str("text", text).Msg("ATIS text built")
	return text
}

// atisLoop broadcasts a single ATIS station. Caches MP3 to disk per weather state.
func atisLoop(ctx context.Context, station *atisStation, apiKey, eamPassword, srsAddr string,
	atcCtrl *controller.ATCController, broadcastIntervalSec int) {

	cacheDir := `C:\SkyeyeATC\atis_cache`
	os.MkdirAll(cacheDir, 0755)

	state := &atisState{
		// Bilingual cache file — different from old "atis_<ICAO>.mp3" so the
		// English-only caches from before the EN+AR change get superseded
		// on first run instead of replayed indefinitely.
		cachePath: filepath.Join(cacheDir, fmt.Sprintf("atis_%s_bilingual.mp3", station.ICAO)),
	}

	// Load cached MP3 from disk if exists
	if data, err := os.ReadFile(state.cachePath); err == nil {
		state.cachedMP3 = data
		log.Info().Str("station", station.Name).Msg("ATIS loaded from disk cache")
	}

	srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
	lastKey := ""
	ticker := time.NewTicker(time.Duration(broadcastIntervalSec) * time.Second)
	defer ticker.Stop()

	var broadcasting sync.Mutex

	broadcast := func() {
		if !broadcasting.TryLock() {
			log.Warn().Str("station", station.Name).Msg("ATIS broadcast already in progress — skipping")
			return
		}
		defer broadcasting.Unlock()
		// Sync weather from the paired tower before reading our own controller
		// state. Stations with no paired tower (Liwa, Kish) keep the boot
		// --static-* values. Mirrors the runway-poll pattern below.
		if tw, ok := fetchTowerWeather(station.ICAO); ok {
			atcCtrl.SetFullWeather(tw.WindDir, tw.WindKts, tw.CeilFt, tw.VisNm, tw.AltInHg, tw.IsNight)
		}
		state.mu.Lock()
		// Update weather from controller
		ceil, alt := atcCtrl.GetWeatherState()
		state.altimeter = alt
		state.ceilingFt = ceil
		// Get wind from airfield state
		state.windFrom = atcCtrl.GetWindFrom()
		state.windKts  = atcCtrl.GetWindKts()
		state.activeRwy = atcCtrl.GetActiveRunway()
		// Prefer the paired tower's active runway when it's reachable — that
		// lets the launcher /runway dropdown propagate into the next ATIS
		// broadcast. Stations with no paired tower (Liwa, Kish) keep the
		// atcCtrl value.
		if rwy := fetchTowerRunway(station.ICAO); rwy != "" && rwy != state.activeRwy {
			log.Info().Str("station", station.Name).Str("icao", station.ICAO).
				Str("from", state.activeRwy).Str("to", rwy).
				Msg("ATIS picked up runway change from tower dashboard")
			state.activeRwy = rwy
		} else if rwy != "" {
			state.activeRwy = rwy
		}

		currentKey := state.weatherKey()
		weatherChanged := currentKey != lastKey

		// Regenerate TTS if weather changed or no cache yet
		if weatherChanged || state.cachedMP3 == nil {
			enText := buildATISText(station, state)
			log.Info().Str("station", station.Name).Str("ident", identWord(state.ident)).
				Bool("weatherChanged", weatherChanged).Msg("ATIS generating new audio (EN+AR)")

			enMP3, err := synthesizeSpeechAPI(ctx, apiKey, enText, station.Voice, 0.97)
			if err != nil {
				log.Error().Err(err).Str("station", station.Name).Msg("ATIS English TTS failed")
				state.mu.Unlock()
				return
			}

			// Translate to Arabic and synthesize. Failures degrade gracefully
			// to English-only — no broadcast disruption.
			var arMP3 []byte
			if arText, terr := translateToArabic(ctx, apiKey, enText); terr != nil {
				log.Warn().Err(terr).Str("station", station.Name).Msg("ATIS Arabic translate failed — broadcasting English only")
			} else if mp3, mErr := synthesizeSpeechAPI(ctx, apiKey, arText, station.Voice, 0.97); mErr != nil {
				log.Warn().Err(mErr).Str("station", station.Name).Msg("ATIS Arabic TTS failed — broadcasting English only")
			} else {
				arMP3 = mp3
			}

			// Merge EN+AR through ffmpeg so the result is a single re-encoded
			// MP3 stream. The previous byte-concat approach (two MP3 files
			// stitched at the byte level) caused DCS-SR-ExternalAudio's
			// decoder to play both languages simultaneously — apparently
			// because the second file's ID3 header looked like a new parallel
			// stream. Re-encoding via ffmpeg with a brief silence guarantees
			// serial playback. On ffmpeg failure fall back to EN-only rather
			// than risk the overlap bug.
			combined := enMP3
			if arMP3 != nil {
				if merged, mErr := concatMP3WithSilence(enMP3, arMP3, flagFFmpeg, 1.2); mErr == nil {
					combined = merged
				} else {
					log.Warn().Err(mErr).Str("station", station.Name).
						Msg("EN+AR ffmpeg concat failed — broadcasting English only")
				}
			}

			state.cachedMP3 = combined
			_ = os.WriteFile(state.cachePath, combined, 0644)
			if weatherChanged {
				state.ident = (state.ident + 1) % 26
				lastKey = currentKey
			}
		} else {
			log.Debug().Str("station", station.Name).Msg("ATIS weather unchanged — rebroadcasting cached audio")
		}
		// Always broadcast every cycle regardless of weather change

		mp3 := state.cachedMP3
		state.mu.Unlock()

		// Broadcast cached MP3
		transmitExternalAudioFile(ctx, mp3, station.FreqMHz, station.Name, srsHost, srsPort,
			flagExternalAudio)
	}

	// Initial broadcast
	broadcast()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			broadcast()
		}
	}
}

// extractACMIProp extracts a value from ACMI property string.
// e.g. extractACMIProp("T=1.23|45.6|100,Name=Raider032", "Name") → "Raider032"
func extractACMIProp(props, key string) string {
	prefix := key + "="
	for _, field := range strings.Split(props, ",") {
		if strings.HasPrefix(field, prefix) {
			return field[len(prefix):]
		}
	}
	return ""
}

// ── ATIS listener ─────────────────────────────────────────────────────────────
// Connects to SRS on the ATIS frequency, transcribes broadcasts via Whisper,
// parses ceiling/visibility/time, and updates the ATC flight mode (VFR/Night/IMC).

func parseAndUpdateATISReal(text string, atcCtrl *controller.ATCController) {
	lower := strings.ToLower(text)

	// Parse active runway from ATIS — "active runway XX" or "active runway departure XX"
	// or "runway XX" — capture the first runway designator after "runway"
	if idx := strings.Index(lower, "active runway"); idx >= 0 {
		after := strings.TrimSpace(lower[idx+13:])
		// Skip "departure" keyword if present
		after = strings.TrimPrefix(after, "departure")
		after = strings.TrimSpace(after)
		// Extract runway designator — 2 digits + optional L/R/C
		words := strings.Fields(after)
		if len(words) > 0 {
			rwy := strings.ToUpper(words[0])
			// Validate — should be 2-3 chars, start with digit
			if len(rwy) >= 2 && rwy[0] >= '0' && rwy[0] <= '3' {
				log.Info().Str("runway", rwy).Msg("ATIS: active runway parsed")
				atcCtrl.SetActiveRunway(rwy)
			}
		}
	}

	// Parse ceiling — try "ceiling XXXX" first, fall back to "cloud base XXXX"
	// Handles: "ceiling 2900 feet", "ceiling 7 thousand", "cloud base 1900"
	ceilingFt := 9999.0 // default clear
	parseCeilingVal := func(after string) float64 {
		after = strings.TrimSpace(after)
		var val float64
		var unit string
		fmt.Sscanf(after, "%f %s", &val, &unit)
		if val > 10 { // sanity check — must be > 10 to be a real ceiling
			if strings.HasPrefix(unit, "thousand") {
				return val * 1000
			}
			return val
		}
		return 0
	}
	if idx := strings.Index(lower, "ceiling"); idx >= 0 {
		if v := parseCeilingVal(lower[idx+7:]); v > 0 {
			ceilingFt = v
		}
	} else if idx := strings.Index(lower, "cloud base"); idx >= 0 {
		// Fall back to cloud base if no "ceiling" keyword
		if v := parseCeilingVal(lower[idx+10:]); v > 0 {
			ceilingFt = v
		}
	}
	// Only update if we actually parsed a ceiling — don't overwrite good data with 9999
	if ceilingFt == 9999.0 && !strings.Contains(lower, "ceiling") && !strings.Contains(lower, "cloud base") {
		log.Debug().Msg("ATIS: no ceiling data in this broadcast — skipping ceiling update")
		// Don't call UpdateFlightConditions — preserve existing mode
		return
	}

	// Parse visibility — "visibility X miles" or "visibility X"
	visNm := 10.0 // default unlimited
	if idx := strings.Index(lower, "visibility"); idx >= 0 {
		after := lower[idx+10:]
		var miles float64
		if n, _ := fmt.Sscanf(strings.TrimSpace(after), "%f", &miles); n == 1 {
			visNm = miles
		}
	}

	// Night check — parse time from ATIS "zero three hours zulu" style
	// Simple approach: use system time (UTC) and field lat for sunrise/sunset
	isNight := isNightTime(time.Now().UTC())

	log.Info().
		Float64("ceilingFt", ceilingFt).
		Float64("visNm", visNm).
		Bool("isNight", isNight).
		Msg("ATIS parsed — updating flight conditions")

	atcCtrl.UpdateFlightConditions(ceilingFt, visNm, isNight)
}

// isNightTime returns true if the given UTC time is between 30min after sunset
// and 30min before sunrise for the Persian Gulf (approx 25°N).
func isNightTime(t time.Time) bool {
	// Simplified: night is 1800-0600 UTC in the Persian Gulf (UTC+4 local)
	// Sunrise ~0200 UTC, Sunset ~1400 UTC year-round approximation
	hour := t.Hour()
	return hour >= 14 || hour < 2
}

// ── Command Channel ───────────────────────────────────────────────────────────
// Listens on the command frequency (default 282.0 MHz), transcribes pilot calls
// via Whisper, and responds with a male OpenAI TTS voice.
//
// Handles: check-in, on-station, off-station, radio check.


// atisOnlyLoop runs all 5 ATIS stations with static weather — for Training VM.
func atisOnlyLoop(ctx context.Context, srsAddr, apiKey, eamPassword string) {
	// Static weather from flags
	windDir  := flagStaticWindDir
	windKts  := flagStaticWindKts
	ceilFt   := flagStaticCeilFt
	altInHg  := flagStaticAltInHg

	if windDir < 0  { windDir = 102 }
	if ceilFt < 0   { ceilFt = 8202 }
	if altInHg <= 0 { altInHg = 29.88 }

	stations := []*atisStation{
		{
			Name: "Al Dhafra ATIS", FreqMHz: 248.200, Voice: "nova", ICAO: "OMAM",
			TACAN: "TACAN 96X. VOR 114.9.",
			ILS: "ILS 111.10 runway 13 left. ILS 109.10 runway 31 left.",
			Advisory: "vSFG-7 training flights in area. Advise information on initial contact.",
		},
		{
			Name: "Al Minhad ATIS", FreqMHz: 248.300, Voice: "shimmer", ICAO: "OMDM",
			TACAN: "TACAN 99X.",
			ILS: "ILS 110.70 runway 09. ILS 110.75 runway 27.",
			Advisory: "vSFG-7 training flights in area. Advise information on initial contact.",
		},
		{
			Name: "Liwa ATIS", FreqMHz: 248.550, Voice: "alloy", ICAO: "OMAB",
			TACAN: "TACAN 121X. VOR 117.4.", ILS: "",
			Advisory: "vSFG-7 training flights in area. Advise information on initial contact.",
		},
		{
			Name: "Al Ain ATIS", FreqMHz: 248.850, Voice: "echo", ICAO: "OMAL",
			TACAN: "TACAN 79X. VOR 112.6.", ILS: "",
			Advisory: "vSFG-7 training flights in area. Advise information on initial contact.",
		},
		{
			Name: "Kish ATIS", FreqMHz: 248.500, Voice: "fable", ICAO: "OIBK",
			TACAN: "",
			ILS: "",
			Advisory: "vSFG-7 training flights in area. Advise information on initial contact.",
		},
	}

	var atcCtrl *controller.ATCController
	if windDir >= 0 {
		// Static weather mode (Training VM)
		atcCtrl = newStaticWeatherController(windDir, windKts, ceilFt, altInHg)
	} else {
		// Live weather mode (Dev VM) — use OMDM airfield controller
		atcCtrl = controller.NewATCController("ATIS", airfield.OMDM)
	}

	var wg sync.WaitGroup
	for i, station := range stations {
		s := station
		delay := 15 + i*37
		wg.Add(1)
		go func(st *atisStation, d int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(d) * time.Second):
			}
			atisLoop(ctx, st, apiKey, eamPassword, srsAddr, atcCtrl, 45)
		}(s, delay)
	}
	log.Info().Int("stations", len(stations)).Str("srs", srsAddr).Msg("ATIS-only broadcaster started")
	wg.Wait()
}
