// Package state implements the ATC state machine for aircraft and airfields.
package state

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/paulmach/orb"
)

// ── Distance helpers ──────────────────────────────────────────────────────────

const earthRadiusNm = 3440.065

// distanceNm returns the great-circle distance in nautical miles between two
// [lon, lat] points using the haversine formula.
func distanceNm(a, b orb.Point) float64 {
	lat1 := a[1] * math.Pi / 180
	lat2 := b[1] * math.Pi / 180
	dLat := (b[1] - a[1]) * math.Pi / 180
	dLon := (b[0] - a[0]) * math.Pi / 180
	x := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusNm * math.Atan2(math.Sqrt(x), math.Sqrt(1-x))
}

// InboundResult is a tracked inbound aircraft with its current distance to the field.
type InboundResult struct {
	Aircraft   *AircraftState
	DistanceNm float64
}

// ── Aircraft phase ────────────────────────────────────────────────────────────

// AircraftPhase is the ATC phase of flight for a tracked aircraft.
type AircraftPhase int

const (
	PhaseUnknown        AircraftPhase = iota
	PhaseInbound                      // Pilot called inbound; not yet sequenced
	PhaseSequenced                    // Given sequence number
	PhaseClearedLand                  // Cleared to land
	PhaseLanding                      // On short final
	PhaseTaxiIn                       // Landed; taxiing in
	PhaseDeparting                    // Taxi clearance issued; awaiting takeoff clearance
	PhaseHoldingShort                 // Pilot called holding short — ready for takeoff clearance
	PhaseClearedTakeoff               // Cleared for takeoff
	PhaseAirborne                     // Departed; handoff complete
	PhaseGoAround                     // Executing go-around
	PhaseHolding                      // Told to hold / extend
)

// isInboundPhase returns true for phases where an aircraft is approaching to land.
func isInboundPhase(p AircraftPhase) bool {
	switch p {
	case PhaseInbound, PhaseSequenced, PhaseClearedLand, PhaseLanding, PhaseGoAround, PhaseHolding:
		return true
	}
	return false
}

// isDeparturePhase returns true for phases where an aircraft is departing.
func isDeparturePhase(p AircraftPhase) bool {
	switch p {
	case PhaseDeparting, PhaseClearedTakeoff:
		return true
	}
	return false
}

// ── AircraftState ─────────────────────────────────────────────────────────────

// AircraftState tracks ATC state for a single aircraft.
type AircraftState struct {
	// Callsign is the pilot's stated callsign (e.g. "Raider 3-1")
	Callsign string
	// Airframe is the stated aircraft type (e.g. "Hornet")
	Airframe string
	// Phase is the current ATC phase
	Phase AircraftPhase
	// SequenceNumber is the landing sequence (1 = next to land, 0 = unassigned)
	SequenceNumber int
	// LastPosition is the last known position from telemetry [lon, lat]
	LastPosition orb.Point
	// LastAltFt is the last known altitude in feet MSL
	LastAltFt float64
	// LastSpeedKts is the last known indicated airspeed in knots
	LastSpeedKts float64
	// PositionUpdatedAt is when LastPosition was last set from telemetry
	PositionUpdatedAt time.Time
	// HoldingShort tracks if pilot has called holding short (required before proactive T/O clearance)
	HoldingShort bool
	// TakeoffCleared tracks if proactive takeoff clearance has been issued
	TakeoffCleared bool
	// SpeedWarned tracks if a speed warning has been issued this approach
	SpeedWarned bool
	// SpeedWarnedAt tracks when speed warning was last issued (60s cooldown)
	SpeedWarnedAt time.Time
	// LastContact is when the aircraft last transmitted on radio
	LastContact time.Time
	// Runway is the assigned runway designator (e.g. "27")
	Runway string
	// FuelState is the pilot's stated fuel in decimal (e.g. 5.2 = 5200 lbs)
	FuelState float64
	// GoAroundWarned tracks whether a proactive go-around has been issued
	GoAroundWarned bool
}

// HasPosition returns true if this aircraft has a fresh telemetry position
// (updated within the last 30 seconds).
func (a *AircraftState) HasPosition() bool {
	return !a.PositionUpdatedAt.IsZero() && time.Since(a.PositionUpdatedAt) < 30*time.Second
}

// ── FlightMode ────────────────────────────────────────────────────────────────

// FlightMode indicates the current operating mode based on weather and time.
type FlightMode int

const (
	ModeVFR FlightMode = iota // VFR — overhead break
	ModeIFR                   // IFR — straight-in (night, IMC, or low vis)
)

// ── AirfieldState ─────────────────────────────────────────────────────────────

// AirfieldState tracks the full ATC picture for one airfield.
type AirfieldState struct {
	mu sync.RWMutex

	Airfield *airfield.Airfield

	ActiveRunway     string
	WindFromMag      float64
	WindKts          float64
	AltimeterInHg    float64
	VisibilityNm     float64
	WeatherUpdatedAt time.Time
	FlightMode       FlightMode
	CeilingFt        float64
	IsNight          bool

	LandingQueue   []*AircraftState
	DepartureQueue []*AircraftState

	tracked map[string]*AircraftState
}

// NewAirfieldState creates an ATC state manager for the given airfield.
func NewAirfieldState(af *airfield.Airfield) *AirfieldState {
	s := &AirfieldState{
		Airfield:      af,
		AltimeterInHg: 29.92,
		VisibilityNm:  10,
		tracked:       make(map[string]*AircraftState),
	}
	s.ActiveRunway = af.RunwayPairs[0].Primary.Designator
	return s
}

// UpdateWeather refreshes wind and altimeter. windFromTrue is in degrees true;
// MagVar is applied internally to derive the magnetic direction.
func (s *AirfieldState) UpdateWeather(windFromTrue, windKts, altimeterInHg, visNm float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	windFromMag := windFromTrue - s.Airfield.MagVar
	if windFromMag < 0 {
		windFromMag += 360
	}
	s.WindFromMag = windFromMag
	s.WindKts = windKts
	s.AltimeterInHg = altimeterInHg
	s.VisibilityNm = visNm
	s.WeatherUpdatedAt = time.Now()
	activeRwy := s.Airfield.ActiveRunway(windFromMag, windKts)
	if activeRwy.Designator != s.ActiveRunway {
		s.ActiveRunway = activeRwy.Designator
	}
}

// UpdatePosition updates the telemetry position and speed for a known aircraft.
// Only updates aircraft already tracked by ATC (i.e. that have checked in).
// position is [lon, lat]; altFt is MSL altitude in feet.
func (s *AirfieldState) UpdatePosition(callsign string, position orb.Point, altFt, speedKts float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ac, ok := s.tracked[callsign]
	if !ok {
		return // Not a tracked aircraft — ignore
	}
	ac.LastPosition = position
	ac.LastAltFt = altFt
	ac.LastSpeedKts = speedKts
	ac.PositionUpdatedAt = time.Now()
}

// ── Deckboss State ───────────────────────────────────────────────────────────

// CatStatus tracks the state of a single catapult.
type CatStatus int

const (
	CatFree    CatStatus = iota // available
	CatTaxying                  // aircraft taxying to cat
	CatTension                  // under tension
	CatLaunched                 // launched, clearing
)

// CatState tracks one catapult.
type CatState struct {
	Number    int
	Status    CatStatus
	Callsign  string    // aircraft assigned
	UpdatedAt time.Time
}

// DeckbossState manages the carrier deck — cats and conga line.
type DeckbossState struct {
	mu        sync.Mutex
	Cats      [4]*CatState // cats 1-4
	CongaLine []string     // callsigns waiting in order
}

// NewDeckbossState creates a fresh deckboss state with all cats free.
func NewDeckbossState() *DeckbossState {
	ds := &DeckbossState{}
	for i := 0; i < 4; i++ {
		ds.Cats[i] = &CatState{Number: i + 1, Status: CatFree}
	}
	return ds
}

// AssignCat finds the first free cat and assigns the callsign.
// Returns cat number (1-4) or 0 if all cats are busy.
func (ds *DeckbossState) AssignCat(callsign string) int {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for _, cat := range ds.Cats {
		if cat.Status == CatFree {
			cat.Status = CatTaxying
			cat.Callsign = callsign
			cat.UpdatedAt = time.Now()
			return cat.Number
		}
	}
	return 0
}

// SetCatStatus updates the status of a cat by number.
func (ds *DeckbossState) SetCatStatus(catNum int, status CatStatus) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if catNum < 1 || catNum > 4 {
		return
	}
	ds.Cats[catNum-1].Status = status
	ds.Cats[catNum-1].UpdatedAt = time.Now()
}

// FreeCat marks a cat as free and clears the callsign. Returns the freed callsign.
func (ds *DeckbossState) FreeCat(callsign string) int {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for _, cat := range ds.Cats {
		if cat.Callsign == callsign {
			num := cat.Number
			cat.Status = CatFree
			cat.Callsign = ""
			cat.UpdatedAt = time.Now()
			return num
		}
	}
	return 0
}

// GetCatByCallsign returns the cat number for a callsign, or 0 if not found.
func (ds *DeckbossState) GetCatByCallsign(callsign string) int {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for _, cat := range ds.Cats {
		if cat.Callsign == callsign {
			return cat.Number
		}
	}
	return 0
}

// AllCatsBusy returns true if all 4 cats are occupied.
func (ds *DeckbossState) AllCatsBusy() bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for _, cat := range ds.Cats {
		if cat.Status == CatFree {
			return false
		}
	}
	return true
}

// EnqueueConga adds a callsign to the conga line. Returns position (1-based).
func (ds *DeckbossState) EnqueueConga(callsign string) int {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for i, cs := range ds.CongaLine {
		if cs == callsign {
			return i + 1
		}
	}
	ds.CongaLine = append(ds.CongaLine, callsign)
	return len(ds.CongaLine)
}

// DequeueConga removes and returns the next callsign from the conga line.
func (ds *DeckbossState) DequeueConga() string {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if len(ds.CongaLine) == 0 {
		return ""
	}
	next := ds.CongaLine[0]
	ds.CongaLine = ds.CongaLine[1:]
	return next
}

// CongaLen returns the number of aircraft waiting in the conga line.
func (ds *DeckbossState) CongaLen() int {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return len(ds.CongaLine)
}

// ── Deckboss State ───────────────────────────────────────────────────────────

// CatStatus tracks a single catapult.
type MarshalAircraft struct {
	Callsign    string
	Position    int     // 1 = #1, 2 = #2, etc
	Angels      int     // assigned stack altitude
	FuelState   float64
	Phase       string  // "inbound", "holding", "charlie", "commencing", "initial", "pushing"
	UpdatedAt   time.Time
}

// MarshalStack manages the carrier recovery stack.
type MarshalStack struct {
	mu       sync.Mutex
	aircraft map[string]*MarshalAircraft // callsign → aircraft
	DeckClear bool
}

// NewMarshalStack creates a new marshal stack.
func NewMarshalStack() *MarshalStack {
	return &MarshalStack{aircraft: make(map[string]*MarshalAircraft)}
}

// StackAngels returns the assigned angels for a given position.
// Position 1 = angels 6, increments by 1000ft per position up to angels 12.
func StackAngels(position int) int {
	angels := 5 + position // pos 1 = 6, pos 2 = 7, etc
	if angels > 12 {
		angels = 12
	}
	return angels
}

// Enqueue adds or updates an aircraft in the stack and returns their position and angels.
func (s *MarshalStack) Enqueue(callsign string, fuelState float64) (position, angels int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ac, ok := s.aircraft[callsign]; ok {
		// Already in stack — just update fuel
		ac.FuelState = fuelState
		ac.UpdatedAt = time.Now()
		return ac.Position, ac.Angels
	}
	// Assign next position
	position = len(s.aircraft) + 1
	angels = StackAngels(position)
	s.aircraft[callsign] = &MarshalAircraft{
		Callsign:  callsign,
		Position:  position,
		Angels:    angels,
		FuelState: fuelState,
		Phase:     "inbound",
		UpdatedAt: time.Now(),
	}
	return position, angels
}

// SetPhase updates an aircraft phase in the stack.
func (s *MarshalStack) SetPhase(callsign, phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ac, ok := s.aircraft[callsign]; ok {
		ac.Phase = phase
		ac.UpdatedAt = time.Now()
	}
}

// Remove deletes an aircraft from the stack. Position resequencing and stack
// collapse (step-downs into the freed slot) happen separately via CollapseStack
// so the caller can announce the new altitudes on the radio.
func (s *MarshalStack) Remove(callsign string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.aircraft, callsign)
}

// ReservedAngels returns the currently-assigned stack altitudes for all aircraft,
// excluding excludeCallsign (typically the caller's own slot when re-checking).
// Slots with Angels == 0 (not yet assigned) are skipped.
func (s *MarshalStack) ReservedAngels(excludeCallsign string) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int, 0, len(s.aircraft))
	for cs, ac := range s.aircraft {
		if cs == excludeCallsign {
			continue
		}
		if ac.Angels > 0 {
			out = append(out, ac.Angels)
		}
	}
	return out
}

// StepDown describes a stack altitude reassignment produced by CollapseStack.
type StepDown struct {
	Callsign  string
	OldAngels int
	NewAngels int
}

// CollapseStack packs remaining aircraft into consecutive altitudes starting at
// minAngels, ordered by their current Angels ascending so relative order is
// preserved. Returns the list of aircraft whose assigned altitude changed —
// caller is expected to transmit a step-down clearance for each.
//
// Aircraft with Angels == 0 (not yet assigned) are ignored. Position numbers
// are also resequenced 1..N to match the new ordering.
func (s *MarshalStack) CollapseStack(minAngels int) []StepDown {
	s.mu.Lock()
	defer s.mu.Unlock()
	sorted := make([]*MarshalAircraft, 0, len(s.aircraft))
	for _, ac := range s.aircraft {
		if ac.Angels <= 0 {
			continue
		}
		sorted = append(sorted, ac)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Angels < sorted[j].Angels
	})
	var changes []StepDown
	target := minAngels
	for i, ac := range sorted {
		ac.Position = i + 1
		if ac.Angels != target {
			changes = append(changes, StepDown{
				Callsign:  ac.Callsign,
				OldAngels: ac.Angels,
				NewAngels: target,
			})
			ac.Angels = target
		}
		target++
	}
	return changes
}

// SetAngels updates the assigned angels for an aircraft already in the stack.
func (s *MarshalStack) SetAngels(callsign string, angels int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ac, ok := s.aircraft[callsign]; ok {
		ac.Angels = angels
	}
}

// GetAircraft returns a copy of an aircraft state.
func (s *MarshalStack) GetAircraft(callsign string) (*MarshalAircraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ac, ok := s.aircraft[callsign]
	if !ok {
		return nil, false
	}
	copy := *ac
	return &copy, true
}

// Count returns the number of aircraft in the stack.
func (s *MarshalStack) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.aircraft)
}

// AirfieldStateSnapshot is a read-only copy of key airfield state for external use.

// GetWindFrom returns wind direction degrees magnetic.
func (s *AirfieldState) GetWindFrom() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.WindFromMag
}

// GetWindKts returns wind speed knots.
func (s *AirfieldState) GetWindKts() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.WindKts
}

// GetWeatherState returns ceiling and altimeter for external use.
func (s *AirfieldState) GetWeatherState() (ceilingFt float64, altimeterInHg float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CeilingFt, s.AltimeterInHg
}

// GetVisibilityNm returns current visibility in nautical miles.
func (s *AirfieldState) GetVisibilityNm() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.VisibilityNm
}

// SetActiveRunway directly sets the active runway from ATIS data.
func (s *AirfieldState) SetActiveRunway(designator string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Validate against known runways
	for _, pair := range s.Airfield.RunwayPairs {
		if pair.Primary.Designator == designator || pair.Reciprocal.Designator == designator {
			s.ActiveRunway = designator
			return
		}
	}
	// If not found in pairs, set anyway — ATIS is authoritative
	s.ActiveRunway = designator
}

// UpdateFlightConditions updates ceiling, visibility and night flag,
// then recalculates the FlightMode.
func (s *AirfieldState) UpdateFlightConditions(ceilingFt, visNm float64, isNight bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CeilingFt = ceilingFt
	s.VisibilityNm = visNm
	s.IsNight = isNight
	switch {
	case ceilingFt < 1500 || visNm < 3:
		s.FlightMode = ModeIFR
	default:
		s.FlightMode = ModeVFR
	}
}

// GetFlightMode returns the current flight mode.
func (s *AirfieldState) GetFlightMode() FlightMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.FlightMode
}

// AllAircraftWithinNm returns ALL tracked aircraft (any phase) within distNm.
// Used for proximity-based proactive triggers like speed checks.
func (s *AirfieldState) AllAircraftWithinNm(distNm float64) []InboundResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	center := s.Airfield.Center
	var results []InboundResult
	for _, ac := range s.tracked {
		if !ac.HasPosition() {
			continue
		}
		d := distanceNm(ac.LastPosition, center)
		if d <= distNm {
			copy := *ac
			results = append(results, InboundResult{Aircraft: &copy, DistanceNm: d})
		}
	}
	return results
}

// InboundsWithinNm returns all inbound-phase aircraft with a fresh telemetry
// position within nm nautical miles of the airfield, sorted closest-first.
func (s *AirfieldState) InboundsWithinNm(nm float64) []InboundResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	center := s.Airfield.Center
	var results []InboundResult
	for _, ac := range s.tracked {
		if !isInboundPhase(ac.Phase) || !ac.HasPosition() {
			continue
		}
		d := distanceNm(ac.LastPosition, center)
		if d <= nm {
			results = append(results, InboundResult{Aircraft: ac, DistanceNm: d})
		}
	}
	// Sort closest first
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].DistanceNm < results[j-1].DistanceNm; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	return results
}

// ClosestInboundNm returns the distance in nm of the closest inbound aircraft,
// or -1 if no inbounds have a valid position.
func (s *AirfieldState) ClosestInboundNm() float64 {
	inbounds := s.InboundsWithinNm(200)
	if len(inbounds) == 0 {
		return -1
	}
	return inbounds[0].DistanceNm
}

// ActiveDepartures returns all aircraft currently cleared for takeoff or rolling.
func (s *AirfieldState) ActiveDepartures() []*AircraftState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*AircraftState
	for _, ac := range s.tracked {
		if ac.Phase == PhaseClearedTakeoff {
			out = append(out, ac)
		}
	}
	return out
}

// GetOrCreate returns the AircraftState for callsign, creating it if new.
func (s *AirfieldState) GetOrCreate(callsign string) *AircraftState {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.tracked[callsign]
	if !ok {
		a = &AircraftState{Callsign: callsign, Phase: PhaseUnknown}
		s.tracked[callsign] = a
	}
	return a
}

// Get returns the AircraftState for callsign, or nil if not tracked.
func (s *AirfieldState) Get(callsign string) *AircraftState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tracked[callsign]
}

// AirfieldStateSnapshot is a JSON-friendly copy of key airfield state.
type AirfieldStateSnapshot struct {
	FreqMHz       float64
	ActiveRunway  string
	FlightMode    FlightMode
	WindFromMag   float64
	WindKts       float64
	AltimeterInHg float64
	CeilingFt     float64
	IsNight       bool
}

// GetAirfieldStateSnapshot returns a safe copy of airfield state for the dashboard.
func (s *AirfieldState) GetAirfieldStateSnapshot() AirfieldStateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return AirfieldStateSnapshot{
		ActiveRunway:  s.ActiveRunway,
		FlightMode:    s.FlightMode,
		WindFromMag:   s.WindFromMag,
		WindKts:       s.WindKts,
		AltimeterInHg: s.AltimeterInHg,
		CeilingFt:     s.CeilingFt,
		IsNight:       s.IsNight,
	}
}

// GetAll returns all aircraft in the marshal stack.
func (s *MarshalStack) GetAll() []*MarshalAircraft {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*MarshalAircraft, 0, len(s.aircraft))
	for _, ac := range s.aircraft {
		copy := *ac
		result = append(result, &copy)
	}
	return result
}

// GetCat returns the CatState for a given cat number.
func (d *DeckbossState) GetCat(num int) CatState {
	d.mu.Lock()
	defer d.mu.Unlock()
	if num >= 1 && num <= 4 {
		return *d.Cats[num-1]
	}
	return CatState{}
}

// GetCongaLine returns a copy of the conga line.
func (d *DeckbossState) GetCongaLine() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]string, len(d.CongaLine))
	copy(result, d.CongaLine)
	return result
}

// GetLandingQueueSnapshot returns a safe copy of the landing queue.
func (s *AirfieldState) GetLandingQueueSnapshot() []*AircraftState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*AircraftState, len(s.LandingQueue))
	for i, ac := range s.LandingQueue {
		copy := *ac
		result[i] = &copy
	}
	return result
}

// LandingQueueLen returns the number of aircraft in the landing queue.
func (s *AirfieldState) LandingQueueLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.LandingQueue)
}

// EnqueueLanding adds an aircraft to the landing queue and returns its sequence number.
func (s *AirfieldState) EnqueueLanding(ac *AircraftState) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range s.LandingQueue {
		if q.Callsign == ac.Callsign {
			return q.SequenceNumber
		}
	}
	s.LandingQueue = append(s.LandingQueue, ac)
	ac.SequenceNumber = len(s.LandingQueue)
	ac.Phase = PhaseSequenced
	ac.Runway = s.ActiveRunway
	return ac.SequenceNumber
}

// EnqueueDeparture adds an aircraft to the departure queue.
func (s *AirfieldState) EnqueueDeparture(ac *AircraftState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range s.DepartureQueue {
		if q.Callsign == ac.Callsign {
			return
		}
	}
	s.DepartureQueue = append(s.DepartureQueue, ac)
	ac.Phase = PhaseDeparting
	ac.Runway = s.ActiveRunway
}

// NextToLand returns the aircraft next in the landing sequence, or nil if empty.
func (s *AirfieldState) NextToLand() *AircraftState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.LandingQueue) == 0 {
		return nil
	}
	return s.LandingQueue[0]
}

// ClearToLand moves an aircraft to ClearedLand and rebuilds sequence numbers.
func (s *AirfieldState) ClearToLand(callsign string) *AircraftState {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, ac := range s.LandingQueue {
		if ac.Callsign == callsign {
			ac.Phase = PhaseClearedLand
			s.LandingQueue = append(s.LandingQueue[:i], s.LandingQueue[i+1:]...)
			for j, q := range s.LandingQueue {
				q.SequenceNumber = j + 1
			}
			return ac
		}
	}
	return nil
}

// ClearForTakeoff marks aircraft as takeoff cleared. Returns true if successful.
func (s *AirfieldState) ClearForTakeoff(callsign string) *AircraftState {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, ac := range s.DepartureQueue {
		if ac.Callsign == callsign {
			ac.Phase = PhaseClearedTakeoff
			s.DepartureQueue = append(s.DepartureQueue[:i], s.DepartureQueue[i+1:]...)
			return ac
		}
	}
	return nil
}

// Remove drops an aircraft from all tracking.
func (s *AirfieldState) Remove(callsign string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tracked, callsign)
	s.LandingQueue = filterQueue(s.LandingQueue, callsign)
	s.DepartureQueue = filterQueue(s.DepartureQueue, callsign)
	for i, q := range s.LandingQueue {
		q.SequenceNumber = i + 1
	}
}

// PruneStale removes aircraft that haven't transmitted in > threshold.
func (s *AirfieldState) PruneStale(threshold time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for cs, ac := range s.tracked {
		if !ac.LastContact.IsZero() && now.Sub(ac.LastContact) > threshold {
			delete(s.tracked, cs)
			s.LandingQueue = filterQueue(s.LandingQueue, cs)
			s.DepartureQueue = filterQueue(s.DepartureQueue, cs)
		}
	}
	for i, q := range s.LandingQueue {
		q.SequenceNumber = i + 1
	}
}
// SetHoldingShort marks the first queued departure as holding short.
func (s *AirfieldState) SetHoldingShort(callsign string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ac := range s.DepartureQueue {
		if ac.Callsign == callsign {
			ac.HoldingShort = true
			ac.Phase = PhaseHoldingShort
			return
		}
	}
}

// NextDeparture returns the first aircraft in the departure queue, or nil if empty.
func (s *AirfieldState) NextDeparture() *AircraftState {
	if len(s.DepartureQueue) == 0 {
		return nil
	}
	return s.DepartureQueue[0]
}

// DepartureQueueLen returns the current departure queue length.
func (s *AirfieldState) DepartureQueueLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.DepartureQueue)
}

func filterQueue(q []*AircraftState, callsign string) []*AircraftState {
	out := q[:0]
	for _, ac := range q {
		if ac.Callsign != callsign {
			out = append(out, ac)
		}
	}
	return out
}
