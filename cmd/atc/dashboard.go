package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	Callsign    string  `json:"callsign"`
	ICAO        string  `json:"icao"`
	FreqMHz     float64 `json:"freqMHz"`
	Online      bool    `json:"online"`
	ActiveRunway string `json:"activeRunway"`
	FlightMode  string  `json:"flightMode"`
	CeilingFt   float64 `json:"ceilingFt"`
	VisibNm     float64 `json:"visibNm"`
	AltimeterInHg float64 `json:"altimeterInHg"`
	WindDir     float64 `json:"windDir"`
	WindKts     float64 `json:"windKts"`
	IsNight     bool    `json:"isNight"`
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

type MarshalEntry struct {
	Callsign  string  `json:"callsign"`
	Position  int     `json:"position"`
	Angels    int     `json:"angels"`
	FuelState float64 `json:"fuelState"`
	Phase     string  `json:"phase"`
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(204)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/status", cors(ds.handleStatus))
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
	// Training 1 missions are based in the Emirates (UTC+4); ReferenceTime
	// arrives in UTC, so we shift it for the day/night check and the UI label.
	const tzOffsetHours = 4
	isNight := s.IsNight
	var missionTimeISO, missionTimeLocal string
	if mt, ok := ds.atcCtrl.GetMissionTime(); ok {
		local := mt.UTC().Add(time.Duration(tzOffsetHours) * time.Hour)
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

	tower := TowerStatus{
		Callsign:      ds.callsign,
		ICAO:          ds.af.ICAO,
		FreqMHz:       s.FreqMHz,
		Online:        true,
		ActiveRunway:  s.ActiveRunway,
		FlightMode:    modeStr,
		CeilingFt:     ceil,
		VisibNm:       s.VisibilityNm,
		AltimeterInHg: altim,
		WindDir:       s.WindFromMag,
		WindKts:       s.WindKts,
		IsNight:       isNight,
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

	// Marshal stack
	marshal := []MarshalEntry{}
	if ds.marshStack != nil {
		for _, ac := range ds.marshStack.GetAll() {
			marshal = append(marshal, MarshalEntry{
				Callsign:  ac.Callsign,
				Position:  ac.Position,
				Angels:    ac.Angels,
				FuelState: ac.FuelState,
				Phase:     ac.Phase,
			})
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
