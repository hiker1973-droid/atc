package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

// ── Log broadcast hub ──────────────────────────────────────────────────────

type logHub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func newLogHub() *logHub {
	return &logHub{clients: make(map[chan []byte]struct{})}
}

func (h *logHub) subscribe() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *logHub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *logHub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// Global log hub — written to by main ATC loop
var globalLogHub = newLogHub()

// LogEntry is a structured log entry broadcast to dashboard clients.
type LogEntry struct {
	Time string `json:"time"`
	Type string `json:"type"` // rx, tx, sys, msh, dkb, err
	Msg  string `json:"msg"`
}

func broadcastLog(typ, msg string) {
	entry := LogEntry{
		Time: time.Now().UTC().Format("15:04:05"),
		Type: typ,
		Msg:  msg,
	}
	b, _ := json.Marshal(entry)
	globalLogHub.broadcast(b)
}

// ── Status snapshot ────────────────────────────────────────────────────────

type TowerStatus struct {
	Callsign     string   `json:"callsign"`
	ICAO         string   `json:"icao"`
	FreqMHz      float64  `json:"freqMHz"`
	Online       bool     `json:"online"`
	ActiveRunway string   `json:"activeRunway"`
	AvailableRunways []string `json:"availableRunways"`
	FlightMode   string   `json:"flightMode"`
	RecoveryCase int      `json:"recoveryCase"` // 1/2/3 for carrier ops; same value on tower roles
	CeilingFt    float64  `json:"ceilingFt"`
	VisibNm      float64  `json:"visibNm"`
	AltimeterInHg float64 `json:"altimeterInHg"`
	WindDir      float64  `json:"windDir"`
	WindKts      float64  `json:"windKts"`
	IsNight      bool     `json:"isNight"`
}

type PatternAircraft struct {
	Callsign string  `json:"callsign"`
	Phase    string  `json:"phase"`
	SeqNum   int     `json:"seqNum"`
	DistNm   float64 `json:"distNm"`
	SpeedKts float64 `json:"speedKts"`
	AltFt    float64 `json:"altFt"`
	FuelState float64 `json:"fuelState"`
}

// DepartureEntry is one row in the departure queue, ordered slot 1 = head.
// State is computed from the aircraft's flag bits:
//   cleared    — TakeoffCleared (TX2 issued, rolling)
//   luaw       — HoldingShort + AutoReleaseAt is in the future (TX1 acked,
//                inside the 5s LUAW gap before auto-release)
//   hold-short — HoldingShort + AutoReleaseAt is past or zero (waiting on
//                the proactive monitor; e.g. spacing window or traffic gate
//                held the auto-release)
//   queued     — in the queue but not yet at the hold short
type DepartureEntry struct {
	Callsign  string `json:"callsign"`
	Slot      int    `json:"slot"`      // 1-indexed position; slot 1 = next out
	State     string `json:"state"`     // queued | hold-short | luaw | cleared
	SecsToGo  int    `json:"secsToGo"`  // seconds remaining on LUAW auto-release, 0 if not in luaw
}

type MarshalEntry struct {
	Callsign     string  `json:"callsign"`
	Position     int     `json:"position"`
	Angels       int     `json:"angels"`       // assigned stack angels
	CurAngels    int     `json:"curAngels"`    // live altitude from Tacview, /1000 ft
	DistNm       int     `json:"distNm"`       // range to mother
	BearingDeg   int     `json:"bearingDeg"`   // bearing from carrier to aircraft (true)
	FuelState    float64 `json:"fuelState"`
	Phase        string  `json:"phase"`
}

type CatEntry struct {
	Num      int    `json:"num"`
	Callsign string `json:"callsign"`
	Status   string `json:"status"` // free, staged, busy
}

type StatusSnapshot struct {
	Timestamp        string            `json:"timestamp"`
	MissionTime      string            `json:"missionTime,omitempty"`      // UTC ISO
	MissionTimeLocal string            `json:"missionTimeLocal,omitempty"` // airfield-local HH:MM
	Tower            TowerStatus       `json:"tower"`
	Pattern     []PatternAircraft `json:"pattern"`
	Departures  []DepartureEntry  `json:"departures"`
	Marshal     []MarshalEntry    `json:"marshal"`
	Cats        []CatEntry        `json:"cats"`
	CongaLine   []string          `json:"congaLine"`
	TacviewContacts int           `json:"tacviewContacts"`
	TTSCacheHits    int64         `json:"ttsCacheHits"`
	TTSCacheMisses  int64         `json:"ttsCacheMisses"`
	IntentMissCount  int64                  `json:"intentMissCount"`
	IntentMissRecent []controller.IntentMiss `json:"intentMissRecent"`
}

// ── Dashboard server ───────────────────────────────────────────────────────

type dashboardServer struct {
	port      int
	callsign  string
	af        *airfield.Airfield
	atcCtrl   *controller.ATCController
	marshStack *state.MarshalStack
	deckState  *state.DeckbossState
	srv       *http.Server
}

func newDashboardServer(port int, callsign string, af *airfield.Airfield,
	atcCtrl *controller.ATCController, marshStack *state.MarshalStack,
	deckState *state.DeckbossState) *dashboardServer {
	return &dashboardServer{
		port: port, callsign: callsign, af: af,
		atcCtrl: atcCtrl, marshStack: marshStack, deckState: deckState,
	}
}

func (ds *dashboardServer) run(ctx context.Context) {
	mux := http.NewServeMux()

	// CORS middleware wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(204)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/status", cors(ds.handleStatus))
	mux.HandleFunc("/runway", cors(ds.handleRunway))
	mux.HandleFunc("/weather", cors(ds.handleWeather))
	mux.HandleFunc("/ws/log", ds.handleWSLog)
	mux.HandleFunc("/health", cors(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "callsign": ds.callsign})
	}))

	ds.srv = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", ds.port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		ds.srv.Close()
	}()

	log.Info().Int("port", ds.port).Str("callsign", ds.callsign).Msg("Dashboard listening")
	if err := ds.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Dashboard server error")
	}
}

func (ds *dashboardServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Tower status from airfield state
	ceil, altim := ds.atcCtrl.GetWeatherState()
	s := ds.atcCtrl.GetAirfieldStateSnapshot()

	// Prefer mission time from Tacview for the night flag — real-world server
	// UTC has no relationship to DCS mission time. Fall back to the
	// --static-night flag (carried on s.IsNight) when Tacview hasn't synced yet.
	// Operator setting: Tacview is configured to publish UTC+3:30, so the
	// mission wall clock is the reported value plus 3 hours 30 minutes.
	const tzOffsetMinutes = 3*60 + 30
	isNight := s.IsNight
	var missionTimeISO, missionTimeLocal string
	if mt, ok := ds.atcCtrl.GetMissionTime(); ok {
		local := mt.UTC().Add(time.Duration(tzOffsetMinutes) * time.Minute)
		isNight = local.Hour() < 6 || local.Hour() >= 18
		missionTimeISO = mt.UTC().Format(time.RFC3339)
		missionTimeLocal = local.Format("15:04")
	}

	modeStr := "VFR"
	switch s.FlightMode {
	case state.ModeIFR:
		modeStr = "IFR"
	}
	if isNight && s.FlightMode != state.ModeIFR {
		modeStr = "VFR Night"
	}

	available := make([]string, 0, len(ds.af.RunwayPairs)*2)
	for _, p := range ds.af.RunwayPairs {
		available = append(available, p.Primary.Designator, p.Reciprocal.Designator)
	}

	// Refresh recovery case against live mission-time night flag so the
	// dashboard reflects night-only transitions without waiting for the
	// Marshal loop's 30s tick. Cheap — single lock + comparison.
	_, _ = ds.atcCtrl.RefreshRecoveryCase()

	tower := TowerStatus{
		Callsign:         ds.callsign,
		ICAO:             ds.af.ICAO,
		FreqMHz:          s.FreqMHz,
		Online:           true,
		ActiveRunway:     s.ActiveRunway,
		AvailableRunways: available,
		FlightMode:       modeStr,
		RecoveryCase:     int(ds.atcCtrl.GetRecoveryCase()),
		CeilingFt:        ceil,
		VisibNm:          s.VisibilityNm,
		AltimeterInHg:    altim,
		WindDir:          s.WindFromMag,
		WindKts:          s.WindKts,
		IsNight:          isNight,
	}

	// Pattern aircraft from Tacview
	pattern := []PatternAircraft{}
	for _, ac := range ds.atcCtrl.GetPatternAircraft() {
		pattern = append(pattern, PatternAircraft{
			Callsign:  ac.Callsign,
			Phase:     fmt.Sprintf("%d", int(ac.Phase)),
			SeqNum:    ac.SequenceNumber,
			DistNm:    0,
			SpeedKts:  ac.LastSpeedKts,
			AltFt:     ac.LastAltFt,
			FuelState: ac.FuelState,
		})
	}

	// Departure queue — slot 1 is head of slice. State derived from the
	// holding-short/auto-release/cleared flag bits set by handleHoldingShortRequest
	// and scheduleAutoRelease. See DepartureEntry doc for state semantics.
	now := time.Now()
	departures := []DepartureEntry{}
	for i, ac := range ds.atcCtrl.GetDepartureQueue() {
		entry := DepartureEntry{
			Callsign: ac.Callsign,
			Slot:     i + 1,
		}
		switch {
		case ac.TakeoffCleared:
			entry.State = "cleared"
		case ac.HoldingShort && !ac.AutoReleaseAt.IsZero() && now.Before(ac.AutoReleaseAt):
			entry.State = "luaw"
			entry.SecsToGo = int(ac.AutoReleaseAt.Sub(now).Round(time.Second).Seconds())
		case ac.HoldingShort:
			entry.State = "hold-short"
		default:
			entry.State = "queued"
		}
		departures = append(departures, entry)
	}

	// Marshal stack — augment each entry with live alt + distance to mother.
	// LookupCallerRelativeToCarrier returns angels (current alt / 1000), range
	// nm and bearing from the carrier; the assigned stack altitude stays in
	// the Angels field for comparison.
	marshal := []MarshalEntry{}
	if ds.marshStack != nil {
		for _, ac := range ds.marshStack.GetAll() {
			entry := MarshalEntry{
				Callsign:  ac.Callsign,
				Position:  ac.Position,
				Angels:    ac.Angels,
				FuelState: ac.FuelState,
				Phase:     ac.Phase,
			}
			if curAng, dist, brg, ok := ds.atcCtrl.LookupCallerRelativeToCarrier(ac.Callsign); ok {
				entry.CurAngels = curAng
				entry.DistNm = dist
				entry.BearingDeg = brg
			}
			marshal = append(marshal, entry)
		}
	}

	// Cat status
	cats := []CatEntry{}
	conga := []string{}
	if ds.deckState != nil {
		for i := 0; i < 4; i++ {
			cat := ds.deckState.GetCat(i + 1)
			statusStr := "free"
			if cat.Callsign != "" {
				statusStr = "staged"
			}
			cats = append(cats, CatEntry{
				Num:      i + 1,
				Callsign: cat.Callsign,
				Status:   statusStr,
			})
		}
		conga = ds.deckState.GetCongaLine()
	}

	hits, misses := globalTTSCache.stats()
	recentMisses, missCount := ds.atcCtrl.GetIntentMisses()

	snap := StatusSnapshot{
		Timestamp:        time.Now().UTC().Format("15:04:05"),
		MissionTime:      missionTimeISO,
		MissionTimeLocal: missionTimeLocal,
		Tower:            tower,
		Pattern:          pattern,
		Departures:       departures,
		Marshal:          marshal,
		Cats:             cats,
		CongaLine:        conga,
		TacviewContacts:  ds.atcCtrl.TacviewContactCount(),
		TTSCacheHits:     hits,
		TTSCacheMisses:   misses,
		IntentMissCount:  missCount,
		IntentMissRecent: recentMisses,
	}

	json.NewEncoder(w).Encode(snap)
}

// handleRunway lets the dashboard switch the airfield's active runway.
// POST /runway?to=<designator> — the designator must appear in
// af.RunwayPairs as either a primary or reciprocal end.
func (ds *dashboardServer) handleRunway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	to := r.URL.Query().Get("to")
	if to == "" {
		http.Error(w, "missing 'to' parameter", http.StatusBadRequest)
		return
	}
	valid := false
	for _, p := range ds.af.RunwayPairs {
		if p.Primary.Designator == to || p.Reciprocal.Designator == to {
			valid = true
			break
		}
	}
	if !valid {
		http.Error(w, "runway not found at "+ds.af.ICAO, http.StatusBadRequest)
		return
	}
	ds.atcCtrl.SetActiveRunway(to)
	log.Info().Str("rwy", to).Str("icao", ds.af.ICAO).Msg("Active runway set via dashboard")
	broadcastLog("sys", "Active runway: "+to+" ("+ds.af.ICAO+")")
	json.NewEncoder(w).Encode(map[string]string{"activeRunway": to})
}

// handleWeather lets the dashboard push a manual weather override.
// POST /weather?windDir=&windKts=&ceilFt=&visNm=&altInHg=
// All five params required. Routes through SetFullWeather; isNight is
// preserved from current state (mission time drives night on /status anyway).
func (ds *dashboardServer) handleWeather(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	parse := func(key string, lo, hi float64) (float64, error) {
		v := q.Get(key)
		if v == "" {
			return 0, fmt.Errorf("missing '%s'", key)
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("'%s' not a number: %v", key, err)
		}
		if f < lo || f > hi {
			return 0, fmt.Errorf("'%s'=%v out of range [%v,%v]", key, f, lo, hi)
		}
		return f, nil
	}
	windDir, err := parse("windDir", 0, 360)
	if err == nil {
		var windKts, ceilFt, visNm, altInHg float64
		windKts, err = parse("windKts", 0, 200)
		if err == nil {
			ceilFt, err = parse("ceilFt", 0, 60000)
			if err == nil {
				visNm, err = parse("visNm", 0, 50)
				if err == nil {
					altInHg, err = parse("altInHg", 25.0, 32.0)
					if err == nil {
						snap := ds.atcCtrl.GetAirfieldStateSnapshot()
						ds.atcCtrl.SetFullWeather(windDir, windKts, ceilFt, visNm, altInHg, snap.IsNight)
						log.Info().
							Float64("windDir", windDir).
							Float64("windKts", windKts).
							Float64("ceilFt", ceilFt).
							Float64("visNm", visNm).
							Float64("altInHg", altInHg).
							Str("icao", ds.af.ICAO).
							Msg("Weather set via dashboard")
						broadcastLog("sys", fmt.Sprintf("Weather set: %03.0f@%02.0f, %.0fft, %.1fnm, %.2f\" (%s)",
							windDir, windKts, ceilFt, visNm, altInHg, ds.af.ICAO))
						json.NewEncoder(w).Encode(map[string]interface{}{
							"windDir": windDir, "windKts": windKts,
							"ceilFt": ceilFt, "visNm": visNm, "altInHg": altInHg,
						})
						return
					}
				}
			}
		}
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

// handleWSLog streams log entries to connected WebSocket clients.
// Uses server-sent events (SSE) instead of WebSocket — no external lib needed.
func (ds *dashboardServer) handleWSLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	ch := globalLogHub.subscribe()
	defer globalLogHub.unsubscribe(ch)

	// Send keepalive comment every 15s to prevent timeout
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	log.Debug().Str("remote", r.RemoteAddr).Msg("Dashboard SSE client connected")

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}
