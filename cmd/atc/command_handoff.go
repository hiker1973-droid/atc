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
	"sync/atomic"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
)

const (
	handoffThresholdNm   = 30.0
	handoffCheckInterval = 30 * time.Second
	pilotIdleTimeout     = 60 * time.Minute
	// handoffMaxCloseNm: more than this closer in one check is not a jet
	// flying in, it is a respawn on a parking spot (Day 2026-10-19 17:21
	// Raider 302 "contact Senaki tower" at 0.28 nm; Foothold Raider 303 at
	// 0.32 nm from Incirlik four times, 18-19 Oct).
	handoffMaxCloseNm = 25.0
)

// positionJumped: the pilot went from lastDistNm to nowNm in one check faster
// than any jet flies. An infinite lastDistNm is the tasked pilot's RTB call,
// which hands off from wherever he is, so it never counts as a jump.
func positionJumped(lastDistNm, nowNm float64) bool {
	return !math.IsInf(lastDistNm, 1) && lastDistNm-nowNm > handoffMaxCloseNm
}

// carrierLaunchRadiusNm: a pilot Command first sees this close to the boat
// launched from it (they check in with Command right after Marshal releases
// them at ~7 DME), so their recovery is Marshal however near a field is.
const carrierLaunchRadiusNm = 20.0

// homeFieldRadiusNm: a land-based pilot Command first sees this close to a
// field took off from it (towers hand departures to Command at ~7 DME), and
// that field is his recovery. On Day 2026-10-20 Venom 201 and 020, out of
// Senaki, were sent to Kutaisi tower at 18:49, 14 minutes after takeoff,
// because their route to work passed inside 30 nm of Kutaisi; Venom 200 got
// the same at 19:31, mid-sortie. All of them landed back at Senaki.
const homeFieldRadiusNm = 20.0

type trackedPilot struct {
	callsign   string
	lastSeen   time.Time
	lastDistNm float64
	handedOff  bool
	// carrierBased is decided once, on the first check with both the pilot
	// and a carrier on scope.
	carrierBased   bool
	carrierChecked bool
	// home is the field a land-based pilot launched from, decided on the
	// first check with the pilot on scope; nil when he was first seen away
	// from every field (then the nearest field is his recovery).
	home        *airfield.Airfield
	homeChecked bool
	// tasked: the pilot asked Command for tasking and has not since made a
	// recovery call (RTB, fence out, off station). A tasked pilot is going
	// to work, not home, so the proactive recovery handoff stays quiet.
	tasked bool
}

type pilotTracker struct {
	mu     sync.Mutex
	pilots map[string]*trackedPilot
	// inFlight counts Command transmissions heard but not yet answered
	// (Whisper, reply, TTS, TX). Set by voiceNetLoop, read by the watcher.
	inFlight atomic.Int32
}

func newPilotTracker() *pilotTracker {
	return &pilotTracker{pilots: make(map[string]*trackedPilot)}
}

// rtbPhrases mark a pilot asking Command for the recovery handoff. Shared with
// commandResponse so the reply and the watcher agree on what an RTB call is.
var rtbPhrases = []string{"rtb", "returning to base", "request recovery", "request handoff",
	"request tower", "request marshal", "inbound home plate", "bingo home plate"}

// isRecoveryCall: the pilot is done working and heading home.
func isRecoveryCall(lower string) bool {
	return containsAny(lower, rtbPhrases...) ||
		containsAny(lower, "fence out", "fence-out", "fenceout", "fencing out",
			"off station", "off-station", "offstation", "departing station")
}

// Note records that this callsign has transmitted text to Command.
func (t *pilotTracker) Note(callsign, text string) {
	if callsign == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.pilots[callsign]
	if !ok {
		p = &trackedPilot{callsign: callsign, lastDistNm: -1}
		t.pilots[callsign] = p
	}
	p.lastSeen = time.Now()
	lower := strings.ToLower(text)
	switch {
	case isRecoveryCall(lower):
		if p.tasked {
			// The crossing may have been skipped while tasked; count the
			// pilot as outside so the next check hands him off from
			// wherever he is.
			p.lastDistNm = math.Inf(1)
		}
		p.tasked = false
	case isTaskingRequest(lower):
		p.tasked = true
	}
}

// handoffGate is what the watcher does with a pilot who has just closed
// through the handoff threshold.
type handoffGate int

const (
	handoffFire  handoffGate = iota
	handoffDefer             // Command is mid-reply: keep the crossing armed for the next check
	handoffSkip              // the pilot has been sent to work: drop this crossing
)

// gateHandoff keeps the proactive recovery handoff from contradicting
// Command's own replies. On DCS3 2026-10-15 Raider 302 asked for tasking at
// 12:11:42, the handoff fired at 12:11:43 while Whisper still had his call
// ("contact Senaki tower ... good landing"), and Command's reply followed at
// 12:11:47 ("contact Hitman one one ... for tasking").
func gateHandoff(p *trackedPilot, busy bool) handoffGate {
	if busy {
		return handoffDefer
	}
	if p.tasked {
		return handoffSkip
	}
	return handoffFire
}

// tacviewContactInfo is what Command keeps per callsign beyond position:
// enough to pick out a tanker and to match a pilot to their side's bullseye.
type tacviewContactInfo struct {
	AltFt      float64
	Name       string // ACMI Name= — the aircraft model, e.g. "KC135MPRS"
	Coalition  string // ACMI Coalition=, e.g. "Allies" - the recorder's own label, NOT the side
	Color      string // ACMI Color=, "Blue" / "Red" - the side (our servers label blue "Enemies")
	HeadingDeg float64 // true; valid only when HasHeading
	HasHeading bool
	UpdatedAt  time.Time
}

// tacviewWeapon is a weapon object's first sighting — where a bomb or missile
// came off the jet. The JTAC uses it to tell a release from a wrong run-in.
type tacviewWeapon struct {
	Pos  orb.Point
	Seen time.Time
}

type tacviewPositions struct {
	mu       sync.RWMutex
	pos      map[string]orb.Point
	info     map[string]tacviewContactInfo
	bullseye map[string]orb.Point // coalition → bullseye (Navaid+Static+Bullseye objects)
	weapons  []tacviewWeapon      // recent releases, oldest first

	// tacview_ground.go
	ground     map[string]*tacviewGround // object id → ground object
	impacts    []tacviewImpact
	missionRef string // ReferenceTime of the mission the ground picture is from
}

// weaponMemory is how long a release stays on the list.
const weaponMemory = 5 * time.Minute

// NoteWeapon records a weapon's first position.
func (t *tacviewPositions) NoteWeapon(p orb.Point, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	keep := t.weapons[:0]
	for _, w := range t.weapons {
		if now.Sub(w.Seen) < weaponMemory {
			keep = append(keep, w)
		}
	}
	t.weapons = append(keep, tacviewWeapon{Pos: p, Seen: now})
}

// WeaponReleasedNear reports whether a weapon first appeared within radiusNm
// of pos at or after since.
func (t *tacviewPositions) WeaponReleasedNear(pos orb.Point, radiusNm float64, since time.Time) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, w := range t.weapons {
		if !w.Seen.Before(since) && haversineNm(pos, w.Pos) <= radiusNm {
			return true
		}
	}
	return false
}

func newTacviewPositions() *tacviewPositions {
	return &tacviewPositions{
		pos:      make(map[string]orb.Point),
		info:     make(map[string]tacviewContactInfo),
		bullseye: make(map[string]orb.Point),
	}
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

// SetInfo records altitude, model and coalition for a callsign.
func (t *tacviewPositions) SetInfo(cs string, info tacviewContactInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.info[cs] = info
}

// Info returns what is known about a callsign, matched like Get.
func (t *tacviewPositions) Info(cs string) tacviewContactInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if info, ok := t.info[cs]; ok {
		return info
	}
	for k, v := range t.info {
		if strings.EqualFold(k, cs) {
			return v
		}
	}
	return tacviewContactInfo{}
}

func (t *tacviewPositions) Remove(cs string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pos, cs)
	delete(t.info, cs)
}

// SetBullseye records a coalition's bullseye.
func (t *tacviewPositions) SetBullseye(coalition string, p orb.Point) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bullseye[coalition] = p
}

// Bullseye returns the bullseye for coalition. With no coalition known it
// answers only when the picture holds exactly one bullseye — never a guess
// between two sides.
func (t *tacviewPositions) Bullseye(coalition string) (orb.Point, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if coalition != "" {
		p, ok := t.bullseye[coalition]
		return p, ok
	}
	if len(t.bullseye) == 1 {
		for _, p := range t.bullseye {
			return p, true
		}
	}
	return orb.Point{}, false
}

type tankerContact struct {
	callsign string
	pos      orb.Point
	info     tacviewContactInfo
}

// isTanker spots a tanker by DCS model name or by the standard tanker
// callsigns (AI tankers are usually named Texaco / Arco / Shell).
func isTanker(callsign, model string) bool {
	m := strings.ToLower(model)
	cs := strings.ToLower(callsign)
	return containsAny(m, "kc-135", "kc135", "kc-130", "kc130", "kc-10", "kc10", "kc-46", "kc46", "s-3b tanker", "il-78") ||
		containsAny(cs, "texaco", "arco", "shell")
}

// NearestTanker returns the closest tanker to from. A tanker of another
// coalition is skipped when both coalitions are known.
func (t *tacviewPositions) NearestTanker(from orb.Point, coalition string) (tankerContact, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var best tankerContact
	bestNm := math.MaxFloat64
	found := false
	for cs, p := range t.pos {
		info := t.info[cs]
		if !isTanker(cs, info.Name) {
			continue
		}
		if coalition != "" && info.Coalition != "" && info.Coalition != coalition {
			continue
		}
		if d := haversineNm(from, p); d < bestNm {
			best, bestNm, found = tankerContact{cs, p, info}, d, true
		}
	}
	return best, found
}

// Carrier returns the carrier's position if one is on scope. Mirrors the
// two-tier match in controller.findCarrierContact: prefer a named CVN, fall
// back to a generic "carrier" group label, since missions export the ship
// either way ("CVN-72 ABE" vs "Carrier strike group-5").
func (t *tacviewPositions) Carrier() (orb.Point, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var fallback orb.Point
	var haveFallback bool
	for name, p := range t.pos {
		lower := strings.ToLower(name)
		switch {
		case strings.Contains(lower, "cvn"),
			strings.Contains(lower, "lincoln"),
			strings.Contains(lower, "stennis"),
			strings.Contains(lower, "roosevelt"),
			strings.Contains(lower, "washington"),
			strings.Contains(lower, "vinson"):
			return p, true
		case strings.Contains(lower, "carrier"):
			if !haveFallback {
				fallback, haveFallback = p, true
			}
		}
	}
	return fallback, haveFallback
}

// miniObject is one ACMI object's last known state. ACMI real-time telemetry
// only sends a property when it changes — Pilot/Name/Type/Coalition arrive
// once, and later T= frames leave unchanged subfields empty — so the state has
// to persist per object id between lines.
type miniObject struct {
	lon, lat, altFt      float64
	hasLon, hasLat       bool
	hdg                  float64
	hasHdg               bool
	pilot, name, typ, co string
	color                string
	weaponNoted          bool
	parent               string    // ACMI Parent= (a weapon's launcher), object id
	shooter              string    // store key of the jet a weapon came off
	last                 orb.Point // a weapon's last position = its impact
	impactNoted          bool
}

// miniTacviewParser turns ACMI lines into store updates. Split out of
// runMiniTacview so the parsing can be tested without a socket.
type miniTacviewParser struct {
	store          *tacviewPositions
	keys           map[string]string // object id → store key
	objs           map[string]*miniObject
	refLat, refLon float64
	refSet         bool
	now            func() time.Time // nil = time.Now; tests pin it
}

// died handles an object leaving the picture or reported destroyed: a ground
// object is a kill, a weapon is an impact. Either can arrive twice (event,
// then removal); the second is ignored.
func (p *miniTacviewParser) died(id string) {
	o := p.objs[id]
	if o == nil {
		return
	}
	switch {
	case o.weaponNoted && !o.impactNoted:
		o.impactNoted = true
		p.store.AddImpact(tacviewImpact{Pos: o.last, At: p.clock(), Shooter: o.shooter})
	case isGroundType(o.typ):
		p.store.GroundDead(id)
	}
}

func (p *miniTacviewParser) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func newMiniTacviewParser(store *tacviewPositions) *miniTacviewParser {
	return &miniTacviewParser{
		store: store,
		keys:  make(map[string]string),
		objs:  make(map[string]*miniObject),
	}
}

func (p *miniTacviewParser) handleLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
		return
	}
	if strings.HasPrefix(line, "-") {
		// Object destroyed — remove from the store if we keyed it.
		id := strings.TrimPrefix(line, "-")
		p.died(id)
		if key, ok := p.keys[id]; ok {
			p.store.Remove(key)
		}
		delete(p.keys, id)
		delete(p.objs, id)
		return
	}
	parts := strings.SplitN(line, ",", 2)
	if len(parts) < 2 {
		return
	}
	id, rest := parts[0], parts[1]
	// Object "0" carries the global ReferenceLatitude/ReferenceLongitude.
	if id == "0" {
		for _, fld := range strings.Split(rest, ",") {
			k, v, _ := strings.Cut(fld, "=")
			switch k {
			case "ReferenceLatitude":
				p.refLat, _ = strconv.ParseFloat(v, 64)
				p.refSet = true
			case "ReferenceLongitude":
				p.refLon, _ = strconv.ParseFloat(v, 64)
				p.refSet = true
			case "ReferenceTime":
				p.store.NewMission(v)
			case "Event":
				// "Destroyed|<id>|..." — the object may stay in the stream.
				if ev, rest, ok := strings.Cut(v, "|"); ok && ev == "Destroyed" {
					target, _, _ := strings.Cut(rest, "|")
					p.died(target)
				}
			}
		}
		return
	}

	o := p.objs[id]
	if o == nil {
		o = &miniObject{}
		p.objs[id] = o
	}
	for _, fld := range strings.Split(rest, ",") {
		k, v, ok := strings.Cut(fld, "=")
		if !ok {
			continue
		}
		switch k {
		case "T":
			// lon|lat|alt[|...] — an empty subfield is unchanged since the last frame.
			coords := strings.Split(v, "|")
			if len(coords) > 0 && coords[0] != "" {
				if f, err := strconv.ParseFloat(coords[0], 64); err == nil {
					o.lon, o.hasLon = f, true
				}
			}
			if len(coords) > 1 && coords[1] != "" {
				if f, err := strconv.ParseFloat(coords[1], 64); err == nil {
					o.lat, o.hasLat = f, true
				}
			}
			if len(coords) > 2 && coords[2] != "" {
				if f, err := strconv.ParseFloat(coords[2], 64); err == nil {
					o.altFt = f * 3.28084
				}
			}
			// lon|lat|alt|roll|pitch|yaw|u|v|heading for aircraft; the
			// 6-field form carries yaw only.
			hi := -1
			switch len(coords) {
			case 9:
				hi = 8
			case 6:
				hi = 5
			}
			if hi > 0 && coords[hi] != "" {
				if f, err := strconv.ParseFloat(coords[hi], 64); err == nil {
					o.hdg, o.hasHdg = math.Mod(f+360, 360), true
				}
			}
		case "Pilot":
			o.pilot = v
		case "Name":
			o.name = v
		case "Type":
			o.typ = v
		case "Coalition":
			o.co = v
		case "Color":
			o.color = v
		case "Parent":
			o.parent = v
		}
	}
	if !o.hasLon || !o.hasLat {
		return
	}
	pt := orb.Point{o.lon, o.lat}
	if p.refSet {
		pt = orb.Point{o.lon + p.refLon, o.lat + p.refLat}
	}
	if strings.Contains(o.typ, "Bullseye") {
		p.store.SetBullseye(o.co, pt)
		return
	}
	// Bombs and missiles: where they first appear (the release) and where
	// they end (the impact, recorded by died).
	if strings.Contains(o.typ, "Weapon") {
		if !o.weaponNoted {
			o.weaponNoted = true
			p.store.NoteWeapon(pt, p.clock())
			if key, ok := p.keys[o.parent]; ok && o.parent != "" {
				o.shooter = key
			} else {
				o.shooter = p.store.NearestPilot(pt, jtacReleaseRadius)
			}
		}
		o.last = pt
		return
	}
	if isGroundType(o.typ) {
		p.store.SetGround(id, pt, o.name)
		return
	}
	// Pilot field uses "Raider 032 |Jedi" — keep the callsign half only.
	key := o.pilot
	if key == "" {
		key = o.name
	}
	if i := strings.Index(key, "|"); i >= 0 {
		key = strings.TrimSpace(key[:i])
	}
	if key == "" {
		return
	}
	p.keys[id] = key
	p.store.Set(key, pt)
	p.store.SetInfo(key, tacviewContactInfo{AltFt: o.altFt, Name: o.name, Coalition: o.co, Color: o.color,
		HeadingDeg: o.hdg, HasHeading: o.hasHdg, UpdatedAt: p.clock()})
}

// runMiniTacview is a stripped-down Tacview consumer for Command. It keeps a
// callsign → position/altitude/model/coalition map plus each coalition's
// bullseye — no phase detection, conflict logic, or controller wiring. Mirrors
// the connection/handshake of the main tacviewLoop in main.go.
func runMiniTacview(ctx context.Context, addr string, store *tacviewPositions) {
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

		parser := newMiniTacviewParser(store)
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
			parser.handleLine(scanner.Text())
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
	fields := airfield.FieldsForMap(flagMap)
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
			carrierPos, haveCarrier := store.Carrier()
			if haveCarrier && !pilot.carrierChecked {
				pilot.carrierChecked = true
				pilot.carrierBased = haversineNm(pos, carrierPos) <= carrierLaunchRadiusNm
			}
			if !pilot.homeChecked {
				pilot.homeChecked = true
				if !pilot.carrierBased {
					pilot.home = homeField(pos, fields)
				}
			}
			rec := chooseRecovery(pos, fields, pilot.home, carrierPos, haveCarrier && flagHandoffMarshalFreq > 0, pilot.carrierBased)
			recoveryName, recoveryFreq, recoveryKind, nearestDist := rec.name, rec.freqMHz, rec.kind, rec.distNm
			// Trigger only when closing through the threshold — prevents
			// firing when pilot starts a sortie already within range, and
			// prevents repeat fires when they orbit at the edge.
			if recoveryName != "" && pilot.lastDistNm > handoffThresholdNm && nearestDist <= handoffThresholdNm &&
				positionJumped(pilot.lastDistNm, nearestDist) {
				log.Info().Str("callsign", cs).Float64("dist", nearestDist).Float64("was", pilot.lastDistNm).
					Msg("Command: recovery handoff skipped - position jumped (respawn), not a jet flying in")
				pilot.lastDistNm = nearestDist
				continue
			}
			if recoveryName != "" && pilot.lastDistNm > handoffThresholdNm && nearestDist <= handoffThresholdNm {
				switch gateHandoff(pilot, tracker.inFlight.Load() > 0) {
				case handoffDefer:
					// Leave lastDistNm outside the threshold so the
					// crossing still counts at the next check.
					log.Info().Str("callsign", cs).Float64("dist", nearestDist).
						Msg("Command: recovery handoff deferred - a call is being answered")
					continue
				case handoffSkip:
					log.Info().Str("callsign", cs).Float64("dist", nearestDist).
						Msg("Command: recovery handoff skipped - pilot is tasked, waiting for RTB")
					pilot.lastDistNm = nearestDist
					continue
				}
				// FAA 2-1-17 order: name the facility, then the frequency.
				tail := "switching now approved, good landing."
				if recoveryKind == "marshal" {
					tail = "switching now approved, call marking mom."
				}
				text := fmt.Sprintf(
					"%s, vSFG-7-Command, contact %s on %s, %s",
					cs, recoveryName, formatFreqMHz(recoveryFreq), tail,
				)
				tx(text, cs)
				pilot.handedOff = true
				log.Info().
					Str("callsign", cs).
					Str("to", recoveryName).
					Str("kind", recoveryKind).
					Float64("dist", nearestDist).
					Msg("Command: proactive recovery handoff issued")
			}
			pilot.lastDistNm = nearestDist
		}
		tracker.mu.Unlock()
	}
}

// recoveryPoint is where Command sends an RTB pilot.
type recoveryPoint struct {
	name    string
	freqMHz float64
	kind    string // "tower" | "marshal"
	distNm  float64
}

// homeField is the field within homeFieldRadiusNm of pos, the nearest one if
// several; nil if none.
func homeField(pos orb.Point, fields []*airfield.Airfield) *airfield.Airfield {
	var home *airfield.Airfield
	best := homeFieldRadiusNm
	for _, fld := range fields {
		if d := haversineNm(pos, fld.Center); d <= best {
			home, best = fld, d
		}
	}
	return home
}

// chooseRecovery picks the facility for the recovery handoff: the home field's
// tower when the pilot launched from a field, else the nearest field's tower,
// or Marshal when the boat is nearer. A carrier-based pilot goes
// to Marshal regardless — on 2026-09-14 Raider 331, off CVN-72, was sent to
// Akrotiri tower because the field was 29 nm away and the boat 52.
// marshalAvailable is false when no carrier is on scope or the Marshal handoff
// is disabled. The returned distance is to the chosen facility, which is what
// the handoff threshold is tested against.
func chooseRecovery(pos orb.Point, fields []*airfield.Airfield, home *airfield.Airfield, carrierPos orb.Point, marshalAvailable, carrierBased bool) recoveryPoint {
	if home != nil && !carrierBased {
		return recoveryPoint{name: home.Name + " tower", freqMHz: home.TowerFreqMHz, kind: "tower",
			distNm: haversineNm(pos, home.Center)}
	}
	rp := recoveryPoint{kind: "tower", distNm: 9999}
	for _, fld := range fields {
		if d := haversineNm(pos, fld.Center); d < rp.distNm {
			rp = recoveryPoint{name: fld.Name + " tower", freqMHz: fld.TowerFreqMHz, kind: "tower", distNm: d}
		}
	}
	if marshalAvailable {
		if d := haversineNm(pos, carrierPos); carrierBased || d < rp.distNm {
			rp = recoveryPoint{name: flagHandoffMarshalName, freqMHz: flagHandoffMarshalFreq, kind: "marshal", distNm: d}
		}
	}
	return rp
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
