// Package controller defines the ATC controller, intent parser, and conflict detection.
package controller

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/state"
	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// ── Conflict thresholds ───────────────────────────────────────────────────────

const (
	// HoldShortRadiusNm — if any inbound is within this distance, hold departures.
	HoldShortRadiusNm = 20.0
	// GoAroundRadiusNm — if an inbound is within this distance and a departure
	// is rolling/cleared, issue a proactive go-around.
	GoAroundRadiusNm = 5.0
	// TrafficAdvisoryRadiusNm — report traffic to inbounds within this range.
	TrafficAdvisoryRadiusNm = 10.0
	// MonitorInterval is how often the background conflict monitor runs.
	MonitorInterval = 10 * time.Second
)

// ── Request types ─────────────────────────────────────────────────────────────

type RequestType int

const (
	RequestUnknown      RequestType = iota
	RequestInbound                  // "inbound" / "initial"
	RequestTakeoffClear             // "request takeoff" / "ready for departure"
	RequestTaxiClear                // "request taxi"
	RequestLandingClear             // "on final" / "request landing"
	RequestGoAround                 // "going around"
	RequestReadback                 // "wilco" / "roger" / "copy"
	RequestRadioCheck               // "radio check"
	RequestAltitude                 // "request altitude" / "altitude check"
	RequestClearTraffic             // "clear traffic" / "clear of traffic" (CTAF departure)
	RequestDistanceCheck            // "7 DME" / "cleared airspace" — post-departure handoff
	RequestHoldingShort             // "holding short runway XX" — at hold short point, ready to proceed
	RequestDistanceInitial          // "30 mile initial" / "10 mile initial" — inbound position report
	RequestOverhead                 // "overhead" / "initial" — overhead the field
	RequestDownwind                 // "downwind runway XX" — on the downwind leg
	RequestBase                     // "base runway XX" — turning base
	RequestTrafficInSight           // "traffic in sight" — pilot has visual on called traffic
	RequestNegativeContact          // "negative contact" — pilot cannot see called traffic
	RequestBreak                    // "break" — pilot commencing overhead break turn
	RequestStraightIn               // "straight in" / "ILS" / "approach" — IFR straight-in
	RequestRunwayVacated            // "runway vacated" / "clear of runway" — closes landing sequence
	RequestEmergency                // "mayday" / "pan pan" / "emergency" — priority handling
	RequestRadarCheck               // "radar check" — read back Tacview-derived angels/range/bearing
	RequestStartup                  // "request startup" / "ready for startup" — engine-start approval (Ground)
	RequestPushingCommand           // "pushing command" — pilot-initiated freq change to Command, courtesy ack
)

// ATCRequest is a parsed pilot transmission.
type ATCRequest struct {
	Callsign  string
	Airframe  string
	Type      RequestType
	FuelState float64
	Raw       string
}

// ── Controller ────────────────────────────────────────────────────────────────

// TacviewContact holds full state for any aircraft seen in Tacview.
type TacviewContact struct {
	Callsign     string
	Lon, Lat     float64
	AltFt        float64
	SpeedKts     float64
	HeadingDeg   float64
	VertSpeedFpm float64  // positive = climbing
	UpdatedAt    time.Time
	// Detected intent
	DetectedPhase string // "departing", "inbound", "holding", "taxiing", ""
	PhaseWarnedAt time.Time
}

// IntentMiss records a single transmission that fell through to RequestUnknown,
// i.e. a pilot call we transcribed but couldn't classify. Surfaced via the
// dashboard so silent fall-throughs are visible without grepping the log.
type IntentMiss struct {
	At       time.Time `json:"at"`
	Callsign string    `json:"callsign"`
	Raw      string    `json:"raw"`
}

const intentMissBufferSize = 50

// ATCController manages ATC for one airfield — state, conflict detection,
// clearance logic, and proactive monitoring.
type ATCController struct {
	mu sync.Mutex

	allPositions   map[string]*TacviewContact
	allPositionsMu sync.RWMutex

	airfieldCallsign   string
	airfieldState      *state.AirfieldState
	composer           *composer.ATCComposer
	stalePruneInterval time.Duration
	staleThreshold     time.Duration

	// transmitFn is set by the application layer to send text through TTS → SRS.
	// When nil, responses are logged only (useful for testing).
	transmitFn func(ctx context.Context, text string)

	// intentMisses keeps the last intentMissBufferSize unrecognized transmissions.
	// Total miss count is unbounded and tracked separately.
	intentMissMu    sync.Mutex
	intentMisses    []IntentMiss
	intentMissCount int64
}

// NewATCController creates a controller for the given airfield.
func NewATCController(
	airfieldCallsign string,
	af *airfield.Airfield,
) *ATCController {
	allPos := make(map[string]*TacviewContact)
	return &ATCController{
		allPositions: allPos,
		airfieldCallsign:   airfieldCallsign,
		airfieldState:      state.NewAirfieldState(af),
		composer:           composer.NewATCComposer(airfieldCallsign),
			stalePruneInterval: 2 * time.Minute,
		staleThreshold:     10 * time.Minute,
	}
}

// SetTransmitFn wires in the TTS→SRS pipeline callback.
func (c *ATCController) SetTransmitFn(fn func(ctx context.Context, text string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transmitFn = fn
}

// Run starts the background weather, prune, and conflict-monitor loops.
// staggerSec adds an initial delay before the monitor loop starts — use this
// to prevent multiple tower instances from hitting Tacview simultaneously.
func (c *ATCController) Run(ctx context.Context) {
	go c.pruneLoop(ctx)
	go func() {
		// Stagger monitor start by airfield to spread Tacview load
		stagger := map[string]time.Duration{
			"OMDM": 0,
			"OMAM": 5 * time.Second,
			"OMAL": 10 * time.Second,
		}
		if delay, ok := stagger[c.airfieldState.Airfield.ICAO]; ok && delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
		c.monitorLoop(ctx)
	}()
}

// SetActiveRunway overrides the wind-calculated active runway with the ATIS-reported one.
// This aligns our ATC with what ATIS is actually broadcasting.
func (c *ATCController) SetActiveRunway(designator string) {
	c.airfieldState.SetActiveRunway(designator)
	log.Info().
		Str("airfield", c.airfieldState.Airfield.ICAO).
		Str("runway", designator).
		Msg("active runway set from ATIS")
}

// UpdateFlightConditions passes ATIS-parsed weather into the airfield state.
func (c *ATCController) UpdateFlightConditions(ceilingFt, visNm float64, isNight bool) {
	c.airfieldState.UpdateFlightConditions(ceilingFt, visNm, isNight)
	mode := c.airfieldState.GetFlightMode()
	modeStr := "VFR"
	if mode == state.ModeIFR { modeStr = "Night" }
	if mode == state.ModeIFR   { modeStr = "IMC"   }
	log.Info().
		Str("airfield", c.airfieldState.Airfield.ICAO).
		Str("mode", modeStr).
		Float64("ceilingFt", ceilingFt).
		Float64("visNm", visNm).
		Bool("isNight", isNight).
		Msg("flight conditions updated from ATIS")
}

// UpdateWeather updates wind and altimeter for all managed airfields.
// windFromTrue is the true wind direction in degrees, windKts in knots,
// altimeterInHg in inches of mercury, visNm in nautical miles. If the new
// wind makes a different runway most-into-wind, the controller transmits a
// proactive wind-shift / runway-change announcement on the tower freq.
func (c *ATCController) UpdateWeather(windFromTrue, windKts, altimeterInHg, visNm float64) {
	prev := c.airfieldState.ActiveRunway
	c.airfieldState.UpdateWeather(windFromTrue, windKts, altimeterInHg, visNm)
	now := c.airfieldState.ActiveRunway
	log.Debug().
		Str("airfield", c.airfieldState.Airfield.ICAO).
		Str("activeRunway", now).
		Float64("windKts", windKts).
		Msg("weather updated — active runway set")
	if prev != "" && now != "" && prev != now {
		log.Info().
			Str("airfield", c.airfieldState.Airfield.ICAO).
			Str("from", prev).
			Str("to", now).
			Float64("windKts", windKts).
			Msg("wind shift — active runway changed, broadcasting to tower")
		text := c.composer.WindShift(now, c.airfieldState.WindFromMag, windKts)
		c.transmit(context.Background(), text)
	}
}

// UpdatePosition feeds a telemetry position update into the airfield state.
// Called by the application layer on every telemetry tick for every aircraft.
// position is [lon, lat]; altFt is altitude in feet MSL.
func (c *ATCController) UpdatePosition(callsign string, position orb.Point, altFt, speedKts float64) {
	c.airfieldState.UpdatePosition(callsign, position, altFt, speedKts)
}

// HandleRequest processes a parsed ATCRequest and transmits the appropriate response.
func (c *ATCController) HandleRequest(ctx context.Context, req *ATCRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := c.airfieldState
	ac := s.GetOrCreate(req.Callsign)
	ac.LastContact = time.Now()
	if req.Airframe != "" {
		ac.Airframe = req.Airframe
	}
	if req.FuelState > 0 {
		ac.FuelState = req.FuelState
	}

	var response string

	switch req.Type {

	case RequestInbound:
		seqNum := s.EnqueueLanding(ac)
		response = c.sequencedArrivalResponse(req.Callsign, s, seqNum)


	case RequestLandingClear:
		response = c.composer.ClearedToLand(req.Callsign, s.ActiveRunway, s.WindFromMag, s.WindKts)


	case RequestStartup:
		response = c.composer.StartupApproval(req.Callsign, s.AltimeterInHg)

	case RequestTaxiClear:
		s.EnqueueDeparture(ac)
		response = c.composer.TaxiClearance(req.Callsign, s.ActiveRunway, s.AltimeterInHg)

	case RequestTakeoffClear:
		response = c.handleTakeoffRequest(ctx, req.Callsign, ac, s)

	case RequestHoldingShort:
		// Pilot is at the hold short point — apply conflict check then either
		// clear them onto the runway or hold with traffic advisory.
		s.EnqueueDeparture(ac)
		response = c.handleHoldingShortRequest(ctx, req.Callsign, ac, s)

	case RequestDistanceInitial:
		seqNum := s.EnqueueLanding(ac)
		response = c.sequencedArrivalResponse(req.Callsign, s, seqNum)


	case RequestOverhead:
		seqNum := s.EnqueueLanding(ac)
		brk := s.Airfield.BreakDirections[s.ActiveRunway]
		response = c.composer.OverheadAck(req.Callsign, s.ActiveRunway, brk, seqNum-1)


	case RequestDownwind:
		seqNum := 0
		if ac := s.Get(req.Callsign); ac != nil { seqNum = ac.SequenceNumber - 1 }
		response = c.composer.DownwindAck(req.Callsign, s.ActiveRunway, seqNum)


	case RequestBase:
		if ac := s.Get(req.Callsign); ac != nil && ac.SequenceNumber > 1 {
			response = c.composer.BaseAck(req.Callsign, s.ActiveRunway, ac.SequenceNumber)
		} else {
			response = c.composer.ClearedToLand(req.Callsign, s.ActiveRunway, s.WindFromMag, s.WindKts)
		}

	case RequestTrafficInSight:
		return // basic mode — not active

	case RequestBreak:
		seqNum := 0
		if ac := s.Get(req.Callsign); ac != nil { seqNum = ac.SequenceNumber - 1 }
		response = c.composer.BreakAck(req.Callsign, s.ActiveRunway, seqNum)


	case RequestStraightIn:
		s.EnqueueLanding(ac)
		response = c.composer.StraightInApproved(req.Callsign, s.ActiveRunway, s.AltimeterInHg)


	case RequestRunwayVacated:
		s.Remove(req.Callsign)
		response = c.composer.RunwayVacated(req.Callsign, s.ActiveRunway)


	case RequestEmergency:
		// Emergency — clear the pattern, priority landing clearance.
		// Remove all other aircraft from cleared-to-land state; they hold.
		ac.Phase = state.PhaseInbound
		response = c.composer.EmergencyAck(
			req.Callsign, s.ActiveRunway, s.WindFromMag, s.WindKts, s.AltimeterInHg,
		)

	case RequestNegativeContact:
		return // basic mode — not active

	case RequestDistanceCheck:
		// Pilot called their departure distance — issue handoff to command.
		s.Remove(req.Callsign)
		response = c.composer.HandoffToCommand(
			req.Callsign,
			s.Airfield.HandoffCallsign,
			s.Airfield.HandoffFreqMHz,
			s.Airfield.HandoffPreset,
		)

	case RequestClearTraffic:
		// Pilot called clear of traffic (CTAF departure) — issue departure release and remove from tracking.
		s.Remove(req.Callsign)
		response = c.composer.DepartureRelease(
			req.Callsign,
			s.Airfield.DepartureDistNm,
			s.Airfield.DepartureAngels,
		)

	case RequestGoAround:
		response = c.composer.GoAround(req.Callsign, s.ActiveRunway)


	case RequestAltitude:
		return // basic mode — not active

	case RequestRadarCheck:
		c.allPositionsMu.RLock()
		contact := c.allPositions[req.Callsign]
		var lon, lat, altFt float64
		hasContact := false
		if contact != nil && !contact.UpdatedAt.IsZero() {
			lon, lat, altFt = contact.Lon, contact.Lat, contact.AltFt
			hasContact = true
		}
		// Diagnostic dump on miss: list every callsign Tacview is reporting so
		// we can see exactly what DCS is exporting vs what the pilot said.
		var known []string
		if !hasContact {
			for cs := range c.allPositions {
				known = append(known, cs)
			}
		}
		c.allPositionsMu.RUnlock()
		if !hasContact {
			log.Warn().
				Str("askedFor", req.Callsign).
				Int("knownCount", len(known)).
				Strs("knownCallsigns", known).
				Msg("radar check miss — Tacview callsign mismatch")
			response = c.composer.RadarCheckNoContact(req.Callsign)
			break
		}
		fieldCenter := s.Airfield.Center
		aircraftPt := orb.Point{lon, lat}
		distNm := int(math.Round(haversineNm(aircraftPt, fieldCenter)))
		bearingDeg := int(math.Round(bearingDegFromTo(fieldCenter, aircraftPt)))
		bearingDeg = ((bearingDeg % 360) + 360) % 360
		angels := int(math.Round(altFt / 1000.0))
		if angels < 0 {
			angels = 0
		}
		response = c.composer.RadarCheck(req.Callsign, angels, distNm, bearingDeg)

	case RequestRadioCheck:
		response = c.composer.RadioCheck(req.Callsign)

	case RequestPushingCommand:
		s.Remove(req.Callsign)
		response = c.composer.PushingCommandAck(req.Callsign)

	case RequestReadback:
		return // Silent acknowledge

	default:
		c.recordIntentMiss(req.Callsign, req.Raw)
		response = c.composer.UnableToUnderstand(req.Callsign)
	}

	if response != "" {
		c.transmit(ctx, response)
	}
}

// handleTakeoffRequest applies conflict detection before issuing a takeoff clearance.
// Returns the clearance or hold-short instruction as a string.
func (c *ATCController) handleTakeoffRequest(
	ctx context.Context,
	callsign string,
	ac *state.AircraftState,
	s *state.AirfieldState,
) string {
	// Check for inbounds within the hold-short radius.
	inbounds := s.InboundsWithinNm(HoldShortRadiusNm)

	if len(inbounds) > 0 {
		// Conflict — hold short and report the traffic.
		closest := inbounds[0]
		log.Info().
			Str("departure", callsign).
			Str("conflict", closest.Aircraft.Callsign).
			Float64("distNm", closest.DistanceNm).
			Msg("departure held short — inbound traffic within hold-short radius")

		s.EnqueueDeparture(ac)
		return c.composer.HoldShortTraffic(
			callsign,
			s.ActiveRunway,
			closest.Aircraft.Callsign,
			closest.DistanceNm,
		)
	}

	// No conflict — clear for takeoff. If this is the first call and the
	// aircraft was never in the departure queue, enqueue then immediately
	// clear: an empty runway with no inbound conflict means no reason to
	// make them hold short.
	trafficOnFinal := s.LandingQueueLen()
	if s.ClearForTakeoff(callsign) == nil {
		s.EnqueueDeparture(ac)
		s.ClearForTakeoff(callsign)
	}
	return c.composer.ClearedForTakeoff(
		callsign, s.ActiveRunway, s.WindFromMag, s.WindKts, trafficOnFinal,
	)
}

// LineUpWaitRadiusNm — inbound within this distance: enter runway but wait, do not take off.
const LineUpWaitRadiusNm = 15.0

// handleHoldingShortRequest checks for conflicts and issues one of three responses:
//   - Inbound < LineUpWaitRadiusNm  → hold short with traffic advisory
//   - Inbound LineUpWait–HoldShort  → line up and wait (enter runway, hold position)
//   - No inbound conflict           → proceed to runway, cleared for takeoff
func (c *ATCController) handleHoldingShortRequest(
	ctx context.Context,
	callsign string,
	ac *state.AircraftState,
	s *state.AirfieldState,
) string {
	inbounds := s.InboundsWithinNm(HoldShortRadiusNm)

	if len(inbounds) > 0 {
		closest := inbounds[0]

		if closest.DistanceNm <= LineUpWaitRadiusNm {
			// Too close — hold short entirely.
			log.Info().
				Str("departure", callsign).
				Str("conflict", closest.Aircraft.Callsign).
				Float64("distNm", closest.DistanceNm).
				Msg("holding short — inbound within hold-short radius")
			return c.composer.HoldShortTraffic(
				callsign, s.ActiveRunway,
				closest.Aircraft.Callsign, closest.DistanceNm,
			)
		}

		// Inbound is between LineUpWaitRadiusNm and HoldShortRadiusNm —
		// safe to enter the runway, not safe to take off yet.
		log.Info().
			Str("departure", callsign).
			Str("conflict", closest.Aircraft.Callsign).
			Float64("distNm", closest.DistanceNm).
			Msg("line up and wait — inbound in outer zone")
		s.EnqueueDeparture(ac)
		return c.composer.LineUpAndWait(
			callsign, s.ActiveRunway,
			closest.Aircraft.Callsign, closest.DistanceNm,
		)
	}

	// No conflict — proceed to runway, cleared for takeoff.
	trafficOnFinal := s.LandingQueueLen()
	s.ClearForTakeoff(callsign)
	return c.composer.ProceedToRunway(
		callsign, s.ActiveRunway, s.WindFromMag, s.WindKts, trafficOnFinal,
	)
}

// monitorLoop runs on MonitorInterval and issues proactive calls:
//   - Go-around warning when an inbound is within GoAroundRadiusNm and a
//     departure is cleared/rolling.
//   - Traffic advisories to inbounds approaching each other within
//     TrafficAdvisoryRadiusNm.
func (c *ATCController) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(MonitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkConflicts(ctx)
		}
	}
}

// SpeedLimitKts is the maximum airspeed within SpeedLimitNm of the field.
const SpeedLimitKts = 350.0
const SpeedLimitNm  = 10.0
const SpeedCooldown = 60 * time.Second

// checkConflicts scans the current airfield state for safety-critical situations
// and issues proactive ATC calls where needed.
func (c *ATCController) checkConflicts(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := c.airfieldState

	// ── Tacview phase detection ─────────────────────────────────────────────────
	// Auto-detect aircraft intent from Tacview data and update ATC state
	c.allPositionsMu.Lock()
	for _, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) > 15*time.Second {
			continue
		}
		phase := detectAircraftPhase(contact, s.Airfield.Center, float64(s.Airfield.ElevationFt))
		if phase == contact.DetectedPhase {
			continue
		}
		contact.DetectedPhase = phase
		dist := haversineNm(orb.Point{contact.Lon, contact.Lat}, s.Airfield.Center)
		log.Debug().
			Str("callsign", contact.Callsign).
			Str("phase", phase).
			Float64("distNm", dist).
			Float64("altFt", contact.AltFt).
			Float64("speedKts", contact.SpeedKts).
			Float64("vsFpm", contact.VertSpeedFpm).
			Msg("Tacview: aircraft phase detected")
	}
	c.allPositionsMu.Unlock()

	// ── Speed check ───────────────────────────────────────────────────────────
	// Check ALL Tacview-tracked aircraft, not just those who've called in
	c.allPositionsMu.RLock()
	var speedWarnings []string
	for _, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) > 30*time.Second {
			continue // stale data
		}
		if contact.SpeedKts <= SpeedLimitKts {
			continue
		}
		// Check distance to field
		pt := orb.Point{contact.Lon, contact.Lat}
		dist := haversineNm(pt, s.Airfield.Center)
		if dist > SpeedLimitNm {
			continue
		}
		speedWarnings = append(speedWarnings, contact.Callsign)
	}
	c.allPositionsMu.RUnlock()
	for _, cs := range speedWarnings {
		real := s.GetOrCreate(cs)
		if real.SpeedWarned && time.Since(real.SpeedWarnedAt) < SpeedCooldown {
			continue
		}
		real.SpeedWarned = true
		real.SpeedWarnedAt = time.Now()
		log.Warn().Str("callsign", cs).Msg("speed limit exceeded — issuing warning")
		c.transmit(ctx, c.composer.SpeedWarning(cs))
	}

	// ── Go-around check ───────────────────────────────────────────────────────
	// If any inbound is within GoAroundRadiusNm AND there is a departure
	// currently cleared/rolling, issue a go-around to the inbound.
	closeFinals := s.InboundsWithinNm(GoAroundRadiusNm)
	departures := s.ActiveDepartures()

	if len(closeFinals) > 0 && len(departures) > 0 {
		for _, result := range closeFinals {
			ac := result.Aircraft
			if ac.GoAroundWarned {
				continue // Already issued — don't repeat
			}
			departure := departures[0]
			log.Warn().
				Str("inbound", ac.Callsign).
				Float64("distNm", result.DistanceNm).
				Str("departure", departure.Callsign).
				Msg("conflict detected — issuing go-around")

			ac.GoAroundWarned = true
			// Re-enqueue as inbound after go-around
			s.Remove(ac.Callsign)
			fresh := s.GetOrCreate(ac.Callsign)
			fresh.Airframe = ac.Airframe
			s.EnqueueLanding(fresh)

			response := c.composer.GoAroundConflict(
				ac.Callsign,
				s.ActiveRunway,
				departure.Callsign,
			)
			c.transmit(ctx, response)
		}
	}

	// ── Departure release check ───────────────────────────────────────────────
	// If a departure is held short and all inbounds have cleared the hold-short
	// radius, proactively clear the first departure for takeoff.
	if s.DepartureQueueLen() > 0 {
		inboundsNear := s.InboundsWithinNm(HoldShortRadiusNm)
		if len(inboundsNear) == 0 {
			next := s.NextDeparture()
			if next != nil && !next.TakeoffCleared && next.HoldingShort {
				// Mark cleared before transmitting to prevent double-issue
				next.TakeoffCleared = true
				cleared := s.ClearForTakeoff(next.Callsign)
				if cleared != nil {
					trafficOnFinal := s.LandingQueueLen()
					response := c.composer.ClearedForTakeoff(
						next.Callsign,
						s.ActiveRunway,
						s.WindFromMag,
						s.WindKts,
						trafficOnFinal,
					)
					log.Info().
						Str("callsign", next.Callsign).
						Str("runway", s.ActiveRunway).
						Msg("proactive departure clearance — inbound traffic clear")
					c.transmit(ctx, response)
				}
			}
		}
	}
}

// transmit sends a text response through the registered TTS→SRS callback.
func (c *ATCController) transmit(ctx context.Context, text string) {
	if c.transmitFn != nil {
		c.transmitFn(ctx, text)
	} else {
		log.Info().
			Str("airfield", c.airfieldState.Airfield.ICAO).
			Str("tx", text).
			Msg("ATC transmit (no TTS wired)")
	}
}

// weatherLoop periodically fetches weather from DCS-gRPC.
func (c *ATCController) weatherLoop(ctx context.Context) {
	ticker := time.NewTicker(120 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refreshWeather(ctx)
		}
	}
}

// refreshWeather updates weather from DCS-gRPC.
// NOTE: DCS-gRPC weather API varies by version — using default values until
// the correct API endpoint is confirmed for your DCS-gRPC version.
func (c *ATCController) refreshWeather(ctx context.Context) {
	// Default to calm/standard conditions until gRPC weather is wired
	// Active runway will be set by wind once gRPC weather API is confirmed
	// Weather comes from ATIS — no update needed here
}

// pruneLoop periodically removes stale aircraft from state.
func (c *ATCController) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(c.stalePruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.airfieldState.PruneStale(c.staleThreshold)
		}
	}
}

// ── Intent Parser ─────────────────────────────────────────────────────────────

// towerKeywordAliases returns common Whisper misrecognitions for a tower callsign.
func towerKeywordAliases(callsign string) []string {
	aliases := map[string][]string{
		"al minhad tower": {"almanad", "al-minhad", "minhad", "minot", "minhot", "el minhad", "el minha", "elmina", "elm and", "helm a", "el mina", "alma nad", "al minad", "helmet", "helmet on", "helmont", "el menad", "elman", "elma", "el menon", "elmenon", "el menod", "elmena"},
		"al dhafra tower": {"dhafra", "al dhafra", "alfra", "al-dhafra", "ldaf", "ldafa", "ldot", "el dhafra", "delta for", "delta offer", "delta tower", "al dafra", "dafra", "altitude", "altitude offer", "al dafna", "dafna"},
		"al ain tower":    {"al ain", "alain", "al-ain", "aline", "alan", "el ain"},
	}
	lower := strings.ToLower(callsign)
	if a, ok := aliases[lower]; ok {
		return a
	}
	// Generic: return all words longer than 3 chars
	var out []string
	for _, p := range strings.Fields(lower) {
		if len(p) > 3 && p != "tower" && p != "traffic" {
			out = append(out, p)
		}
	}
	return out
}

// extractFuelState parses a fuel state from text e.g. "state 5.6" → 5.6
func extractFuelState(lower string) float64 {
	idx := strings.Index(lower, "state")
	if idx < 0 {
		return 0
	}
	after := strings.TrimSpace(lower[idx+5:])
	var val float64
	if n, _ := fmt.Sscanf(after, "%f", &val); n == 1 && val > 0 && val < 30 {
		return val
	}
	return 0
}

// isModexReadback returns true for short transmissions that are just a callsign
// acknowledgment — e.g. "032", "Raider 032", "Venom 201". No response needed.
func isModexReadback(lower string) bool {
	// Strip common callsign prefixes
	for _, prefix := range []string{"raider", "reader", "radar", "rater", "venom", "vino", "knight", "hitman", "devil"} {
		lower = strings.TrimSpace(strings.TrimPrefix(lower, prefix))
	}
	// What remains should be 2-3 digits only
	lower = strings.TrimSpace(lower)
	if len(lower) < 2 || len(lower) > 4 {
		return false
	}
	for _, c := range lower {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ParseIntent converts a raw STT transcript into an ATCRequest.
// Returns nil if the transmission is not addressed to this tower.
func ParseIntent(text string, towerCallsign string) *ATCRequest {
	lower := strings.ToLower(text)

	// Extract keywords from callsign for fuzzy matching.
	// "Al Minhad Tower" → check for "minhad" or "al minhad"
	// Whisper often mishears airfield names so we match on the most distinctive word.
	parts := strings.Fields(strings.ToLower(towerCallsign))
	// Use the longest part as the primary keyword (skips "al", "tower" etc)
	primaryKey := parts[0]
	for _, p := range parts {
		if len(p) > len(primaryKey) && p != "tower" && p != "traffic" {
			primaryKey = p
		}
	}
	// Also build common Whisper misrecognitions
	aliases := towerKeywordAliases(towerCallsign)
	
	isCTAF := false
	isDirect := false
	for _, kw := range append([]string{primaryKey}, aliases...) {
		if strings.Contains(lower, kw+" traffic") || strings.Contains(lower, kw+"traffic") {
			isCTAF = true
		}
		// Accept "[field] tower" OR "[field] traffic" as direct address — pilots use both
		if strings.Contains(lower, kw+" tower") || strings.Contains(lower, kw+"tower") ||
			strings.Contains(lower, kw+" traffic") || strings.Contains(lower, kw+"traffic") ||
			strings.Contains(lower, strings.ToLower(towerCallsign)) {
			isDirect = true
		}
	}

	// Also accept bare "tower" or "traffic" address without field name
	if !isDirect && !isCTAF {
		if strings.HasPrefix(lower, "tower,") || strings.HasPrefix(lower, "tower ") ||
			strings.HasPrefix(lower, "traffic,") || strings.HasPrefix(lower, "traffic ") {
			isDirect = true
		}
	}

	if !isDirect && !isCTAF {
		return nil
	}
	req := &ATCRequest{
		Raw:      text,
		Callsign: extractCallsign(text, towerCallsign),
		Airframe: extractAirframe(lower),
	}
	switch {
	case containsAny(lower, "clear traffic", "clear of traffic", "airborne", "departing"):
		// Departure release — pilot is clear of the pattern (works on tower or CTAF)
		req.Type = RequestClearTraffic
	case containsAny(lower, "seven dme", "7 dme", "seven miles", "7 miles", "cleared airspace", "five miles", "5 miles") ||
		(containsAny(lower, "dme", "d.m.e", "d m e") && !containsAny(lower, "initial", "inbound")):
		// Post-departure distance check-in. Whisper often transcribes "7 DME"
		// as "seven mile DME" — catch any DME mention that isn't an inbound
		// pattern entry so it doesn't fall through to RequestDistanceInitial.
		req.Type = RequestDistanceCheck
	case containsDistanceMiles(lower) && containsAny(lower, "initial", "mile", "inbound"):
		// 3 mile initial = overhead break entry; others = inbound position report
		dist := extractDistanceMilesInt(lower)
		if dist <= 3 {
			req.Type = RequestOverhead
		} else {
			req.Type = RequestDistanceInitial
		}
	case containsAny(lower, "overhead", "over the field", "initial overhead"):
		req.Type = RequestOverhead
	case containsAny(lower, "downwind"):
		req.Type = RequestDownwind
	case containsAny(lower, "base", "turning base", "right base", "left base", "base final"):
		req.Type = RequestBase
	case containsAny(lower, "traffic in sight", "visual", "tally"):
		req.Type = RequestTrafficInSight
	case containsAny(lower, "negative contact", "no contact", "no joy"):
		req.Type = RequestNegativeContact
	case containsAny(lower, "mayday", "pan pan", "emergency", "declaring emergency"):
		req.Type = RequestEmergency
	case containsAny(lower,
		// Standard
		"runway vacated", "vacated", "clear of runway", "off the runway",
		// Clear runway variants
		"clear runway", "cleared runway", "clearing runway", "runway clear",
		"runway cleared", "cleared the runway", "clear of", "clear active",
		"cleared active", "clearing active", "runway is clear", "runway is vacated",
		// Clear traffic variants (post-departure)
		"clear traffic", "cleared traffic", "clearing traffic", "clear of traffic",
		"cleared of traffic", "clear the pattern", "clear pattern",
		// Off runway variants
		"off runway", "off the active", "exiting runway", "exited runway",
		"runway exit", "leaving runway", "left the runway",
		// Phonetic mishears
		"clear active runway", "clear of active"):
		req.Type = RequestRunwayVacated
	case containsAny(lower, "straight in", "straight-in", "ils", "instrument approach", "rnav"):
		req.Type = RequestStraightIn
	case containsAny(lower, "break"):
		// Only classify as break if not part of "radio check" etc.
		if !containsAny(lower, "radio", "check") {
			req.Type = RequestBreak
		}
	case containsAny(lower, "inbound") && !containsAny(lower, "initial", "mile"):
		// "...inbound" alone — treat as distance initial at unknown distance
		req.Type = RequestDistanceInitial
	case containsAny(lower, "holding short", "hold short", "short of runway", "at the hold"):
		req.Type = RequestHoldingShort
	case containsAny(lower, "request takeoff", "request departure", "ready for departure", "ready for takeoff", "lineup"):
		req.Type = RequestTakeoffClear
	case containsAny(lower, "request startup", "ready for startup", "ready to start", "request start"):
		req.Type = RequestStartup
	case containsAny(lower, "pushing command", "pushing to command", "switching command", "switching to command", "push command"):
		// Pilot is announcing a freq change to Command — courtesy ack, no
		// need to re-issue freq/preset (handoff was already given at 7 DME).
		req.Type = RequestPushingCommand
	case containsAny(lower, "request taxi", "request ground", "taxi to", "ready to taxi"):
		req.Type = RequestTaxiClear
	case containsAny(lower, "on final", "final", "request landing", "cleared to land"):
		req.Type = RequestLandingClear
	case containsAny(lower, "going around", "go around", "missed approach"):
		req.Type = RequestGoAround
	case containsAny(lower, "radar check", "radar contact", "request radar", "radar service"):
		req.Type = RequestRadarCheck
	case containsAny(lower, "request altitude", "altitude check", "altitude request", "what altitude", "assigned altitude"):
		req.Type = RequestAltitude


	case containsAny(lower, "radio check", "comm check", "comms check", "comcheck", "comp check", "how copy"):
		req.Type = RequestRadioCheck
	case containsAny(lower, "wilco", "roger", "copy", "affirm", "negative"):
		req.Type = RequestReadback
	case isModexReadback(lower):
		// Short numeric-only transmission e.g. "032" or "Raider 032" — pilot acknowledging
		req.Type = RequestReadback
	default:
		req.Type = RequestUnknown
	}
	return req
}

// normalizeCallsign fixes known Whisper mishearings of squadron callsigns.
func normalizeCallsign(text string) string {
	replacements := [][]string{
		// Raider mishears
		{"reader", "Raider"},
		{"raider", "Raider"},
		{"radar", "Raider"},
		{"rater", "Raider"},
		{"raiders", "Raider"},
		// Venom mishears
		{"venom", "Venom"},
		{"vino", "Venom"},
		{"venue", "Venom"},
		{"demon", "Venom"},
	}
	lower := strings.ToLower(text)
	for _, pair := range replacements {
		if strings.Contains(lower, pair[0]) {
			idx := strings.Index(lower, pair[0])
			text = text[:idx] + pair[1] + text[idx+len(pair[0]):]
			lower = strings.ToLower(text)
		}
	}
	return text
}

func extractCallsign(text, towerCallsign string) string {
	text = normalizeCallsign(text)
	lower := strings.ToLower(text)
	towerLower := strings.ToLower(towerCallsign)
	idx := strings.Index(lower, towerLower)
	if idx < 0 {
		// Tower name not found — bare "traffic/tower, [callsign], ..." format
		// Extract second comma-delimited segment as callsign
		parts := strings.Split(text, ",")
		if len(parts) >= 2 {
			cs := strings.TrimSpace(parts[1])
			if cs != "" && !strings.Contains(strings.ToLower(cs), "traffic") && !strings.Contains(strings.ToLower(cs), "tower") {
				return cs
			}
		}
		return ""
	}
	before := strings.TrimRight(strings.TrimSpace(text[:idx]), ", ")
	if before != "" {
		parts := strings.Split(before, ",")
		if cs := strings.TrimSpace(parts[len(parts)-1]); cs != "" {
			return cs
		}
	}
	after := strings.TrimLeft(strings.TrimSpace(text[idx+len(towerCallsign):]), ", ")
	parts := strings.Split(after, ",")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func extractAirframe(lower string) string {
	for key, name := range map[string]string{
		"hornet": "Hornet", "f-18": "Hornet", "f/a-18": "Hornet",
		"viper": "Viper", "f-16": "Viper",
		"warthog": "Warthog", "hog": "Warthog", "a-10": "Warthog",
	} {
		if strings.Contains(lower, key) {
			return name
		}
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ── Distance extraction helpers ──────────────────────────────────────────────────

// numberWords maps spoken numbers to integers for distance extraction.
var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"fifteen": 15, "twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
}

// distanceMilesRe matches "3 mile", "12 miles", "8 nm", "30 nautical" etc.
// The digit must immediately precede a miles/nm token so unrelated numbers
// (altitudes, channels, callsign suffixes) don't trigger a false positive.
var distanceMilesRe = regexp.MustCompile(`\b(\d{1,3})\s*(?:miles?|nm|nautical)\b`)

// containsDistanceMiles returns true if the text contains a numeric or word distance.
func containsDistanceMiles(lower string) bool {
	if distanceMilesRe.MatchString(lower) {
		return true
	}
	for word := range numberWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

// extractDistanceMilesInt is an alias for extractDistanceMiles for clarity.
func extractDistanceMilesInt(text string) int {
	return extractDistanceMiles(text)
}

// extractDistanceMiles pulls a nautical mile distance from a transmission.
// Returns 0 if none found.
func extractDistanceMiles(text string) int {
	lower := strings.ToLower(text)
	if m := distanceMilesRe.FindStringSubmatch(lower); m != nil {
		val := 0
		fmt.Sscanf(m[1], "%d", &val)
		return val
	}
	for word, val := range numberWords {
		if strings.Contains(lower, word) {
			return val
		}
	}
	return 0
}

// ── Coordinate helpers ────────────────────────────────────────────────────────

const (
	pgOriginLat  = 25.8674
	pgOriginLon  = 56.0941
	metersPerDeg = 111320.0
)

func latToX(lat float64) float64 {
	return (lat - pgOriginLat) * metersPerDeg
}

func lonToZ(lon float64) float64 {
	return (lon - pgOriginLon) * metersPerDeg * math.Cos(pgOriginLat*math.Pi/180)
}

func (c *ATCController) String() string {
	return fmt.Sprintf("ATCController[%s]", c.airfieldState.Airfield.ICAO)
}

// UpdateWeatherFromLua updates airfield weather from the ATCWeather.lua UDP export.
// windFromTrue is degrees true; MagVar is applied internally.
// windKts is in knots. altimeterInHg is in inches of mercury.
// Routes through UpdateWeather so wind-shift / runway-change detection fires
// on mid-mission weather updates too.
func (c *ATCController) UpdateWeatherFromLua(windFromTrue, windKts, altimeterInHg float64) {
	c.UpdateWeather(windFromTrue, windKts, altimeterInHg, 10.0)
}

// SortedInboundsByDistance returns callsigns of current inbounds sorted by
// distance to the field — closest first. Uses Tacview data when available,
// falls back to arrival queue order.
func (c *ATCController) SortedInboundsByDistance() []string {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()

	type distEntry struct {
		callsign string
		dist     float64
	}
	var entries []distEntry
	for cs, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) > 30*time.Second {
			continue
		}
		// Only include aircraft that are inbound (descending toward field)
		phase := contact.DetectedPhase
		if phase == "inbound" || phase == "pattern" {
			dist := haversineNm(orb.Point{contact.Lon, contact.Lat}, c.airfieldState.Airfield.Center)
			entries = append(entries, distEntry{cs, dist})
		}
	}
	// Sort closest first
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].dist < entries[i].dist {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.callsign
	}
	return result
}

// NearestInboundAhead returns the callsign and distance of the nearest inbound
// aircraft that is closer to the field than the given callsign. Returns "" if none.
func (c *ATCController) NearestInboundAhead(callsign string) (string, float64) {
	sorted := c.SortedInboundsByDistance()
	c.allPositionsMu.RLock()
	myContact := c.allPositions[callsign]
	c.allPositionsMu.RUnlock()

	if myContact == nil {
		return "", 0
	}
	myDist := haversineNm(orb.Point{myContact.Lon, myContact.Lat}, c.airfieldState.Airfield.Center)

	for _, cs := range sorted {
		if cs == callsign {
			continue
		}
		c.allPositionsMu.RLock()
		contact := c.allPositions[cs]
		c.allPositionsMu.RUnlock()
		if contact == nil {
			continue
		}
		theirDist := haversineNm(orb.Point{contact.Lon, contact.Lat}, c.airfieldState.Airfield.Center)
		if theirDist < myDist {
			return cs, theirDist
		}
	}
	return "", 0
}

// sequencedArrivalResponse builds the inbound/3-mile-initial reply with traffic
// awareness. When seqNum > 1 and Tacview can identify the lead, name them and
// give the in-trail distance. Otherwise fall back to a generic count.
func (c *ATCController) sequencedArrivalResponse(callsign string, s *state.AirfieldState, seqNum int) string {
	if seqNum <= 1 {
		return c.composer.InboundAck(callsign, s.ActiveRunway, s.WindFromMag, s.WindKts, s.AltimeterInHg, 0)
	}
	leadCS, leadDist := c.NearestInboundAhead(callsign)
	if leadCS != "" {
		return c.composer.SequencedInitialAck(callsign, 0, s.ActiveRunway, 0, s.AltimeterInHg, seqNum, leadCS, int(leadDist+0.5))
	}
	return c.composer.InboundAck(callsign, s.ActiveRunway, s.WindFromMag, s.WindKts, s.AltimeterInHg, seqNum-1)
}

// UpdateAnyPosition records position for ANY Tacview aircraft and updates ATC state.
func (c *ATCController) UpdateAnyPosition(callsign string, lon, lat, altFt, speedKts, headingDeg, vertSpeedFpm float64) {
	c.allPositionsMu.Lock()
	if c.allPositions[callsign] == nil {
		c.allPositions[callsign] = &TacviewContact{Callsign: callsign}
	}
	contact := c.allPositions[callsign]
	contact.Lon = lon
	contact.Lat = lat
	contact.AltFt = altFt
	contact.SpeedKts = speedKts
	contact.HeadingDeg = headingDeg
	contact.VertSpeedFpm = vertSpeedFpm
	contact.UpdatedAt = time.Now()
	c.allPositionsMu.Unlock()
	// Also update tracked state if this aircraft has checked in
	c.airfieldState.UpdatePosition(callsign, orb.Point{lon, lat}, altFt, speedKts)
}

// SetWeather injects static weather — used by ATIS-only Training VM mode.
func (c *ATCController) SetWeather(windDir, windKts, ceilFt, altInHg float64) {
	c.airfieldState.UpdateWeather(windDir, windKts, altInHg, ceilFt)
}

// GetAirfieldStateSnapshot returns a dashboard-friendly state snapshot.
func (c *ATCController) GetAirfieldStateSnapshot() state.AirfieldStateSnapshot {
	return c.airfieldState.GetAirfieldStateSnapshot()
}

// GetPatternAircraft returns aircraft currently in the landing queue.
func (c *ATCController) GetPatternAircraft() []*state.AircraftState {
	return c.airfieldState.GetLandingQueueSnapshot()
}

// IsAircraftAirborne returns true if Tacview shows the aircraft above 500ft AGL and climbing.
func (c *ATCController) IsAircraftAirborne(callsign string) bool {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	for cs, contact := range c.allPositions {
		if !strings.EqualFold(cs, callsign) {
			continue
		}
		if time.Since(contact.UpdatedAt) > 30*time.Second {
			continue
		}
		if contact.AltFt > 500 && contact.VertSpeedFpm > 0 {
			return true
		}
	}
	return false
}

// GetCarrierBRC returns the carrier's current magnetic heading (BRC) from Tacview.
// Returns -1 if carrier not found.
func (c *ATCController) GetCarrierBRC() float64 {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	// Look for CVN-72 or any carrier-named contact
	for callsign, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) > 60*time.Second {
			continue
		}
		lower := strings.ToLower(callsign)
		if strings.Contains(lower, "cvn") || strings.Contains(lower, "lincoln") ||
			strings.Contains(lower, "carrier") || strings.Contains(lower, "stennis") {
			return contact.HeadingDeg
		}
	}
	return -1
}

// GetCarrierPosition returns the carrier lon/lat from Tacview.
func (c *ATCController) GetCarrierPosition() (lon, lat float64, found bool) {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	for callsign, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) > 60*time.Second {
			continue
		}
		lower := strings.ToLower(callsign)
		if strings.Contains(lower, "cvn") || strings.Contains(lower, "lincoln") ||
			strings.Contains(lower, "carrier") {
			return contact.Lon, contact.Lat, true
		}
	}
	return 0, 0, false
}

// AssignMarshalAngels picks the lowest unoccupied stack altitude in [minAngels, maxAngels].
// A slot is "occupied" if it appears in reservedAngels (already-assigned stack slots,
// passed in by the caller) or if Tacview shows any aircraft within 50nm of the carrier
// at that rounded altitude. Falls back to maxAngels when every slot is taken.
func (c *ATCController) AssignMarshalAngels(minAngels, maxAngels int, reservedAngels []int) int {
	if minAngels > maxAngels {
		return maxAngels
	}
	occupied := make(map[int]bool)
	for _, a := range reservedAngels {
		occupied[a] = true
	}
	if carLon, carLat, found := c.GetCarrierPosition(); found {
		carrierPt := orb.Point{carLon, carLat}
		c.allPositionsMu.RLock()
		for _, contact := range c.allPositions {
			if time.Since(contact.UpdatedAt) > 30*time.Second {
				continue
			}
			if haversineNm(orb.Point{contact.Lon, contact.Lat}, carrierPt) > 50 {
				continue
			}
			angels := int(math.Round(contact.AltFt / 1000.0))
			occupied[angels] = true
		}
		c.allPositionsMu.RUnlock()
	}
	for a := minAngels; a <= maxAngels; a++ {
		if !occupied[a] {
			return a
		}
	}
	return maxAngels
}

// IsDeckClear returns true if no aircraft are on final or the runway at the carrier.
func (c *ATCController) IsDeckClear() bool {
	carLon, carLat, found := c.GetCarrierPosition()
	if !found {
		return true // no carrier data — assume clear
	}
	carrierPt := orb.Point{carLon, carLat}
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	for _, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) > 30*time.Second {
			continue
		}
		dist := haversineNm(orb.Point{contact.Lon, contact.Lat}, carrierPt)
		if dist < 1.0 && contact.AltFt < 1000 {
			return false
		}
	}
	return true
}

// GetAirfieldState returns a snapshot of the airfield state for ATIS use.
func (c *ATCController) GetAirfieldState() state.AirfieldStateSnapshot {
	return c.airfieldState.GetAirfieldStateSnapshot()
}

// GetWindFrom returns wind direction in degrees magnetic.
func (c *ATCController) GetWindFrom() float64 {
	return c.airfieldState.GetWindFrom()
}

// GetWindKts returns wind speed in knots.
func (c *ATCController) GetWindKts() float64 {
	return c.airfieldState.GetWindKts()
}

// GetActiveRunway returns the current active runway designator.
func (c *ATCController) GetActiveRunway() string {
	return c.airfieldState.ActiveRunway
}

// GetWeatherState returns current ceiling and altimeter for marshal use.
func (c *ATCController) GetWeatherState() (ceilingFt float64, altimeterInHg float64) {
	return c.airfieldState.GetWeatherState()
}

// TacviewContactCount returns the number of active air contacts from Tacview.
func (c *ATCController) TacviewContactCount() int {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	count := 0
	for _, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) < 30*time.Second {
			count++
		}
	}
	return count
}

// IsTacviewActive returns true if we have fresh position data for any aircraft.
func (c *ATCController) IsTacviewActive() bool {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	for _, contact := range c.allPositions {
		if time.Since(contact.UpdatedAt) < 30*time.Second {
			return true
		}
	}
	return false
}

// GetAircraftDistanceNm returns the distance of a callsign from the field center, or -1 if unknown.
func (c *ATCController) GetAircraftDistanceNm(callsign string) float64 {
	c.allPositionsMu.RLock()
	defer c.allPositionsMu.RUnlock()
	if contact, ok := c.allPositions[callsign]; ok {
		if time.Since(contact.UpdatedAt) < 30*time.Second {
			return haversineNm(orb.Point{contact.Lon, contact.Lat}, c.airfieldState.Airfield.Center)
		}
	}
	return -1
}

// detectAircraftPhase returns the detected flight phase based on Tacview data.
// Returns: "departing", "inbound", "pattern", "taxiing", "holding", ""
func detectAircraftPhase(c *TacviewContact, fieldCenter orb.Point, fieldElevFt float64) string {
	dist := haversineNm(orb.Point{c.Lon, c.Lat}, fieldCenter)
	aglFt := c.AltFt - fieldElevFt

	// Taxiing — on ground, slow
	if aglFt < 100 && c.SpeedKts < 60 {
		return "taxiing"
	}
	// Departing — low altitude, climbing, near field
	if dist < 15 && aglFt < 3000 && c.VertSpeedFpm > 300 && c.SpeedKts > 80 {
		return "departing"
	}
	// In pattern — close to field, low alt, not climbing fast
	if dist < 8 && aglFt < 2500 && math.Abs(c.VertSpeedFpm) < 1000 {
		return "pattern"
	}
	// Inbound — descending toward field
	if dist < 40 && dist > 5 && c.VertSpeedFpm < -200 {
		return "inbound"
	}
	// Holding — slow, relatively level, not approaching
	if dist > 5 && dist < 30 && math.Abs(c.VertSpeedFpm) < 200 && c.SpeedKts < 250 {
		return "holding"
	}
	return ""
}

// recordIntentMiss appends an unrecognized transmission to the ring buffer,
// bumps the lifetime counter, and writes a warn-level log line so the silent
// fall-through is visible without dashboard access. Callsign may be empty.
func (c *ATCController) recordIntentMiss(callsign, raw string) {
	if raw == "" {
		return
	}
	miss := IntentMiss{At: time.Now(), Callsign: callsign, Raw: raw}
	c.intentMissMu.Lock()
	c.intentMisses = append(c.intentMisses, miss)
	if len(c.intentMisses) > intentMissBufferSize {
		c.intentMisses = c.intentMisses[len(c.intentMisses)-intentMissBufferSize:]
	}
	c.intentMissCount++
	count := c.intentMissCount
	c.intentMissMu.Unlock()
	log.Warn().
		Str("airfield", c.airfieldState.Airfield.ICAO).
		Str("callsign", callsign).
		Str("raw", raw).
		Int64("total", count).
		Msg("intent miss — pilot transmission not recognized")
}

// GetIntentMisses returns a copy of the recent unrecognized transmissions
// (newest last) along with the lifetime miss count.
func (c *ATCController) GetIntentMisses() ([]IntentMiss, int64) {
	c.intentMissMu.Lock()
	defer c.intentMissMu.Unlock()
	out := make([]IntentMiss, len(c.intentMisses))
	copy(out, c.intentMisses)
	return out, c.intentMissCount
}

// haversineNm returns great-circle distance in nm between two [lon,lat] points.
func haversineNm(a, b orb.Point) float64 {
	const R = 3440.065
	lat1 := a[1] * math.Pi / 180
	lat2 := b[1] * math.Pi / 180
	dLat := (b[1] - a[1]) * math.Pi / 180
	dLon := (b[0] - a[0]) * math.Pi / 180
	x := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * R * math.Atan2(math.Sqrt(x), math.Sqrt(1-x))
}

// bearingDegFromTo returns the initial true bearing in degrees from point a
// to point b. Output is normalized to [0, 360). Both points are [lon, lat].
func bearingDegFromTo(a, b orb.Point) float64 {
	lat1 := a[1] * math.Pi / 180
	lat2 := b[1] * math.Pi / 180
	dLon := (b[0] - a[0]) * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	deg := math.Atan2(y, x) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}
