package debrief

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Session is everything the report shows, worked out from the records.
type Session struct {
	Start, End  time.Time
	Loc         *time.Location // times on the report are this PC's local time
	HasEvents   bool           // the server's event lines were there (else Tacview only)
	Rec         *Recording     // may be nil
	Pilots      []*PilotStat
	Shots       []*Shot
	Kills       []*Kill
	Engagements []*Engagement
	Missiles    []MissileStat
	Safety      []SafetyItem
	Traps       []Trap
	Fields      []Field // where players took off and landed
	PvP         bool    // players on both sides
	BlueName    string  // "Venom": the blue players' squadron word, for the headline
	RedName     string
	// MissionStart is when the mission run began (the Discord post's T+
	// times); ejected holds each player's ejections by short name.
	MissionStart time.Time
	ejected      map[string][]time.Time
	es           *EventSession // the server's events, for the losses and tanker pages
}

// PilotStat is one player's session.
type PilotStat struct {
	Key       string // the full name DCS uses
	Name      string // short: "Bacon"
	Side      string
	Aircraft  []string
	Sorties   int
	Flight    time.Duration
	Landings  int
	Traps     int
	Kills     int // air
	Ground    int // ground and sea kills
	ShotDown  int
	Crashes   int
	Ejections int
	Shots     int
	Hits      int
	Log       []LogEntry // the pilot page's flight log, from the server events
}

// LogEntry is one line of a pilot's flight log.
type LogEntry struct {
	At   time.Time
	Text string
}

func (p *PilotStat) log(at time.Time, format string, a ...any) {
	p.Log = append(p.Log, LogEntry{At: at, Text: fmt.Sprintf(format, a...)})
}

// Shot is one missile, bomb or rocket fired by or at a player.
type Shot struct {
	At         time.Time
	Shooter    string // short name, or the AI unit type
	ShooterKey string
	Side       string
	Target     string
	TargetSide string
	Weapon     string
	RangeNM    float64 // -1 when not known
	AltFt, Kt  float64 // the shooter's at launch
	Hit        bool
	KillN      int // the kill number in its engagement, 0 if none
	Result     string
	Friendly   bool
	human      bool // a player fired it
	jet, tgt   *Object
	missile    *Object
	eventID    int
}

// Kill is one aircraft destroyed with a player involved.
type Kill struct {
	At         time.Time
	Killer     string
	KillerSide string
	Victim     string
	VictimSide string
	VictimType string
	Weapon     string
	Air        bool
	Lat, Lon   float64
	AltFt      float64
	N          int // number within its engagement
	Engagement int
	shot       *Shot
}

// Engagement is a fight: shots and kills close together in time.
type Engagement struct {
	N         int
	Takeoff   time.Time
	FirstShot time.Time
	LastEvent time.Time
	Blue, Red []string
	LostBlue  int
	LostRed   int
	Winner    string // Blue, Red, Draw
	Shots     []*Shot
	Kills     []*Kill
	from, to  float64 // recording seconds: the fight (map window)
	routeFrom float64 // from the takeoff (routes-in map)
	jets      []*Object
}

// MissileStat is fired and hit by side and weapon.
type MissileStat struct {
	Side, Weapon string
	Fired, Hit   int
}

// SafetyItem is a blue-on-blue, a crash into the ground, an ejection.
type SafetyItem struct {
	At   time.Time
	Kind string // Blue-on-blue, Red-on-red, Terrain, Ejection
	Text string
}

// Trap is one carrier landing graded by DCS's LSO.
type Trap struct {
	At    time.Time
	Pilot string
	Place string
	Grade
}

// Field is a place players flew from, for the maps.
type Field struct {
	Name     string
	Lat, Lon float64
	Side     string
}

const nm = 1852.0

// ShortName turns a DCS player name into what the squadron calls them:
// "RAIDER 031 | BACON" -> "Bacon".
func ShortName(p string) string {
	p = strings.TrimSpace(p)
	if i := strings.LastIndex(p, "|"); i >= 0 {
		p = strings.TrimSpace(p[i+1:])
	} else if parts := strings.Split(p, "-"); !strings.Contains(p, " ") && len(parts) > 1 && strings.Trim(parts[0], "0123456789") == "" {
		// "406-B.P-CVW5" (a squadron number first): the one part that is
		// letters only is the callsign
		var only []string
		for _, part := range strings.Split(p, "-") {
			if strings.IndexFunc(part, unicode.IsLetter) >= 0 && strings.IndexFunc(part, unicode.IsDigit) < 0 {
				only = append(only, part)
			}
		}
		if len(only) == 1 {
			p = only[0]
		}
	}
	if p == strings.ToUpper(p) {
		w := strings.Fields(p)
		for i, x := range w {
			if len(x) > 3 && strings.IndexFunc(x, func(r rune) bool { return !unicode.IsLetter(r) }) < 0 { // letters only: "BACON"
				r := []rune(strings.ToLower(x))
				r[0] = unicode.ToUpper(r[0])
				w[i] = string(r)
			}
		}
		p = strings.Join(w, " ")
	}
	return p
}

// Analyze builds the session from whatever records there are.
func Analyze(rec *Recording, srv *ServerLog, from, to time.Time, loc *time.Location) *Session {
	if loc == nil {
		loc = time.Local
	}
	s := &Session{Rec: rec, Loc: loc, Start: from, End: to}
	var recStart time.Time
	if rec != nil {
		recStart = rec.Start
		s.Start, s.End = rec.Start, rec.End()
	}
	es := srv.SessionFor(recStart, from, to)
	if es != nil && len(es.Events) > 0 {
		s.HasEvents, s.es = true, es
		if rec == nil {
			s.Start, s.End = es.Start, es.End()
		}
		s.fromEvents(es)
	} else if rec != nil {
		s.fromTacview()
	}
	s.findEngagements()
	s.missileStats()
	s.safety(es)
	s.sides()
	return s
}

func (s *Session) pilot(key, side string) *PilotStat {
	for _, p := range s.Pilots {
		if p.Key == key {
			if p.Side == "" {
				p.Side = side
			}
			return p
		}
	}
	p := &PilotStat{Key: key, Name: ShortName(key), Side: side}
	s.Pilots = append(s.Pilots, p)
	return p
}

func addUnique(l []string, v string) []string {
	if v == "" {
		return l
	}
	for _, x := range l {
		if x == v {
			return l
		}
	}
	return append(l, v)
}

// jetOf is the player's aircraft in the recording at real time at: Tacview
// names a player's jet after the player. A killed jet can leave the
// recording a beat before its kill event, so the nearest within 15 s.
func (s *Session) jetOf(pilot string, at time.Time) *Object {
	if s.Rec == nil || pilot == "" {
		return nil
	}
	t := s.Rec.Sec(at)
	var best *Object
	bestD := 15.0
	for _, o := range s.Rec.Aircraft {
		if o.Pilot != pilot {
			continue
		}
		d := math.Max(math.Max(o.Start-t, t-o.End), 0)
		if d < bestD || (d == bestD && best != nil && o.Start > best.Start) {
			best, bestD = o, d
		}
	}
	return best
}

// fromEvents reads what DCS scored.
func (s *Session) fromEvents(es *EventSession) {
	byID := map[int]*ServerEvent{}
	for i := range es.Events {
		byID[es.Events[i].ID] = &es.Events[i]
	}
	name := func(p Party) string {
		if p.Human() {
			return ShortName(p.Pilot)
		}
		if p.Unit != "" {
			return p.Unit
		}
		return p.Pilot
	}
	s.MissionStart = es.Start
	s.ejected = map[string][]time.Time{}
	takeoff := map[string]time.Time{}
	land := func(key string, at time.Time) {
		if t0, ok := takeoff[key]; ok {
			if j := s.jetOf(key, t0); j != nil && s.Rec.Time(j.End).Before(at) {
				at = s.Rec.Time(j.End) // left the jet (re-slotted) before the next event
			}
			if at.After(t0) {
				s.pilot(key, "").Flight += at.Sub(t0)
			}
			delete(takeoff, key)
		}
	}
	var shots []*ServerEvent
	for i := range es.Events {
		e := &es.Events[i]
		if s.Rec != nil && (e.At.Before(s.Start.Add(-time.Minute)) || e.At.After(s.End.Add(time.Minute))) {
			continue // outside the recording
		}
		key := e.Init.Pilot
		human := e.Init.Human()
		switch e.Type {
		case "takeoff":
			if human {
				p := s.pilot(key, e.Init.Side())
				p.Sorties++
				p.Aircraft = addUnique(p.Aircraft, AircraftName(e.Init.Unit))
				p.log(e.At, "Took off from %s in the %s", firstNonEmpty(e.Place, "the field"), AircraftName(e.Init.Unit))
				land(key, e.At) // a sortie still open: it ended when that jet did
				takeoff[key] = e.At
				s.field(e.Place, key, e.At, e.Init.Side())
			}
		case "land":
			if human {
				p := s.pilot(key, e.Init.Side())
				p.Landings++
				p.log(e.At, "Landed at %s", firstNonEmpty(e.Place, "a field"))
				land(key, e.At)
				s.field(e.Place, key, e.At, e.Init.Side())
			}
		case "crash", "pilot dead", "eject":
			if human {
				p := s.pilot(key, e.Init.Side())
				switch e.Type {
				case "crash":
					p.Crashes++
					p.log(e.At, "The jet crashed")
				case "eject":
					p.Ejections++
					s.ejected[p.Name] = append(s.ejected[p.Name], e.At)
					p.log(e.At, "Ejected")
				default:
					p.log(e.At, "Pilot killed")
				}
				land(key, e.At)
			}
		case "landing quality mark":
			if human {
				g := ParseGrade(e.Comment)
				s.Traps = append(s.Traps, Trap{At: e.At, Pilot: ShortName(key), Place: e.Place, Grade: g})
				p := s.pilot(key, e.Init.Side())
				if g.Wire > 0 {
					p.Traps++
				}
				p.log(e.At, "Carrier pass at %s: %s", firstNonEmpty(e.Place, "the carrier"), firstNonEmpty(g.Raw, g.Grade))
			}
		case "refuel", "refuel stop":
			if human {
				p := s.pilot(key, e.Init.Side())
				p.log(e.At, "%s", map[string]string{"refuel": "Plugged in on the tanker", "refuel stop": "Off the tanker"}[e.Type])
			}
		case "shot":
			if human || e.Target.Human() {
				shots = append(shots, e)
			}
		}
	}
	for key := range takeoff { // still flying at the end
		land(key, s.End)
	}

	// Shots, with their hits.
	shotOf := map[int]*Shot{}
	for i, e := range shots {
		sh := &Shot{At: e.At, Shooter: name(e.Init), ShooterKey: e.Init.Pilot, Side: e.Init.Side(), Weapon: e.Weapon,
			RangeNM: -1, eventID: e.ID, human: e.Init.Human()}
		if e.Target.Pilot != "" || e.Target.Unit != "" {
			sh.Target, sh.TargetSide = name(e.Target), e.Target.Side()
			sh.Friendly = sh.Side != "" && sh.Side == sh.TargetSide
		}
		// a hit linked to this shot, or (DCS often links 0) the same shooter,
		// target and weapon before that pair's next shot
		next := math.Inf(1)
		for _, f := range shots[i+1:] {
			if f.Init.Pilot == e.Init.Pilot && f.Weapon == e.Weapon && f.Target.Pilot == e.Target.Pilot {
				next = f.T
				break
			}
		}
		for j := range es.Events {
			h := &es.Events[j]
			if h.Type == "hit" && (h.Linked == e.ID || (h.Linked == 0 && h.Init.Pilot == e.Init.Pilot && h.Target.Pilot == e.Target.Pilot &&
				h.Weapon == e.Weapon && h.T > e.T && h.T < math.Min(next, e.T+160))) {
				sh.Hit = true
				break
			}
		}
		if e.Init.Human() {
			p := s.pilot(e.Init.Pilot, e.Init.Side())
			p.Shots++
			if sh.Hit {
				p.Hits++
			}
		}
		s.geometry(sh, e.Init.Pilot, e.Target.Pilot)
		s.Shots = append(s.Shots, sh)
		shotOf[e.ID] = sh
	}

	// Kills: kill -> hit -> shot, so the credited shooter is the one DCS scored.
	for i := range es.Events {
		e := &es.Events[i]
		if e.Type != "kill" || !(e.Init.Human() || e.Target.Human()) {
			continue
		}
		if s.Rec != nil && (e.At.Before(s.Start.Add(-time.Minute)) || e.At.After(s.End.Add(time.Minute))) {
			continue
		}
		k := &Kill{At: e.At, Killer: name(e.Init), KillerSide: e.Init.Side(), Victim: name(e.Target), VictimSide: e.Target.Side(),
			VictimType: AircraftName(e.Target.Unit), Weapon: e.Weapon, Air: e.Target.Air}
		at := e.At
		if h := byID[e.Linked]; h != nil {
			at = h.At
			if sh := shotOf[h.Linked]; sh != nil {
				k.shot = sh
				if sh.Shooter != k.Killer { // the shooter whose missile it was
					k.Killer, k.KillerSide = sh.Shooter, sh.Side
				}
			}
		}
		if k.shot == nil { // DCS did not link it: the last hit shot by this shooter on this target
			for j := len(s.Shots) - 1; j >= 0; j-- {
				sh := s.Shots[j]
				if sh.Shooter == k.Killer && sh.Target == k.Victim && sh.Hit && !sh.At.After(e.At) && e.At.Sub(sh.At) < 3*time.Minute {
					k.shot = sh
					break
				}
			}
		}
		if s.Rec != nil {
			var vj *Object
			if e.Target.Human() {
				vj = s.jetOf(e.Target.Pilot, at)
			} else {
				vj = s.nearestOfType(e.Target.Unit, at, k.shot)
			}
			if vj != nil {
				p := vj.At(s.Rec.Sec(at))
				k.Lat, k.Lon, k.AltFt = p.Lat, p.Lon, p.AltM*3.28084
			}
		}
		// Credit the player whose weapon it was (DCS may log no initiator for a
		// missile still flying after its shooter died).
		creditKey, creditSide := "", ""
		switch {
		case k.shot != nil && k.shot.human:
			creditKey, creditSide = k.shot.ShooterKey, k.shot.Side
		case e.Init.Human():
			creditKey, creditSide = e.Init.Pilot, e.Init.Side()
		}
		if creditKey != "" {
			p := s.pilot(creditKey, creditSide)
			if k.Air {
				p.Kills++
			} else {
				p.Ground++
			}
			p.log(e.At, "Killed %s (%s) with %s", k.Victim, firstNonEmpty(k.VictimType, "?"), firstNonEmpty(k.Weapon, "guns"))
		}
		if e.Target.Human() {
			p := s.pilot(e.Target.Pilot, e.Target.Side())
			p.ShotDown++
			p.log(e.At, "Shot down by %s with %s", k.Killer, firstNonEmpty(k.Weapon, "guns"))
		}
		s.Kills = append(s.Kills, k)
	}
}

// nearestOfType finds an AI aircraft of a type near where a shot ended.
func (s *Session) nearestOfType(unit string, at time.Time, sh *Shot) *Object {
	if s.Rec == nil {
		return nil
	}
	t := s.Rec.Sec(at)
	var best *Object
	bestD := math.Inf(1)
	for _, o := range s.Rec.Aircraft {
		if o.Name != unit || !o.Alive(t-5) {
			continue
		}
		d := 0.0
		if sh != nil && sh.missile != nil {
			d = dist3(o.At(t), sh.missile.At(sh.missile.End))
		}
		if d < bestD {
			best, bestD = o, d
		}
	}
	return best
}

// geometry finds the shot's jets and missile in the recording: range to the
// target, the shooter's altitude and speed at launch.
func (s *Session) geometry(sh *Shot, shooter, target string) {
	if s.Rec == nil {
		return
	}
	t := s.Rec.Sec(sh.At)
	sh.jet = s.jetOf(shooter, sh.At)
	if target != "" {
		sh.tgt = s.jetOf(target, sh.At)
	}
	if sh.jet != nil {
		p := sh.jet.At(t)
		sh.AltFt, sh.Kt = p.AltM*3.28084, sh.jet.SpeedKt(t)
		bestD := math.Inf(1)
		for _, w := range s.Rec.Weapons {
			if math.Abs(w.Start-t) > 1.5 {
				continue
			}
			if d := dist3(w.Track[0], p); d < bestD && d < 5000 {
				sh.missile, bestD = w, d
			}
		}
	}
	if sh.jet != nil && sh.tgt != nil {
		sh.RangeNM = dist3(sh.jet.At(t), sh.tgt.At(t)) / nm
	}
}

// field remembers where a player took off or landed, at the jet's position.
func (s *Session) field(place, pilot string, at time.Time, side string) {
	if place == "" {
		return
	}
	for _, f := range s.Fields {
		if f.Name == place {
			return
		}
	}
	if j := s.jetOf(pilot, at); j != nil {
		p := j.At(s.Rec.Sec(at))
		s.Fields = append(s.Fields, Field{Name: place, Lat: p.Lat, Lon: p.Lon, Side: side})
	}
}

// fromTacview is the fallback with no server events: shots from the
// missiles in the recording (the nearest aircraft at launch fired it, the
// nearest enemy aircraft it passed was its target) and sorties from AGL.
// DCS's own kills are not in a recording, so kills are left out.
func (s *Session) fromTacview() {
	r := s.Rec
	human := func(o *Object) bool {
		return o.Pilot != "" && o.Group != "" && !strings.HasPrefix(o.Pilot, o.Group) && o.Pilot != o.Name
	}
	for _, o := range r.Aircraft {
		if !human(o) {
			continue
		}
		p := s.pilot(o.Pilot, o.Side())
		p.Aircraft = addUnique(p.Aircraft, AircraftName(o.Name))
		airborne, t0 := false, 0.0
		for _, pt := range o.Track {
			if pt.AGL < 0 {
				continue
			}
			switch {
			case !airborne && pt.AGL > 50:
				airborne, t0 = true, pt.T
				p.Sorties++
			case airborne && pt.AGL < 3 && o.SpeedKt(pt.T) < 60:
				airborne = false
				p.Landings++
				p.Flight += time.Duration((pt.T - t0) * float64(time.Second))
			}
		}
		if airborne {
			p.Flight += time.Duration((o.End - t0) * float64(time.Second))
		}
	}
	for _, w := range r.Weapons {
		if !strings.Contains(w.Type, "Missile") {
			continue
		}
		launch := w.Track[0]
		var shooter *Object
		bestD := 3000.0
		for _, a := range r.Aircraft {
			if !a.Alive(w.Start) {
				continue
			}
			if d := dist3(a.At(w.Start), launch); d < bestD {
				shooter, bestD = a, d
			}
		}
		if shooter == nil || !human(shooter) {
			continue
		}
		sh := &Shot{At: r.Time(w.Start), Shooter: ShortName(shooter.Pilot), ShooterKey: shooter.Pilot, Side: shooter.Side(),
			Weapon: w.Name, RangeNM: -1, jet: shooter, missile: w, human: true}
		pt := shooter.At(w.Start)
		sh.AltFt, sh.Kt = pt.AltM*3.28084, shooter.SpeedKt(w.Start)
		var tgt *Object
		miss := math.Inf(1)
		for _, a := range r.Aircraft {
			if a == shooter || a.Side() == shooter.Side() || !a.Alive(w.Start) {
				continue
			}
			for _, mp := range w.Track {
				if d := dist3(a.At(mp.T), mp); d < miss {
					tgt, miss = a, d
				}
			}
		}
		if tgt != nil && miss < 10*nm {
			sh.tgt, sh.TargetSide = tgt, tgt.Side()
			sh.Target = ShortName(firstNonEmpty(tgt.Pilot, tgt.Name))
			if !human(tgt) {
				sh.Target = tgt.Name
			}
			sh.RangeNM = dist3(pt, tgt.At(w.Start)) / nm
			sh.Hit = miss < 60 && tgt.Removed && tgt.End-w.End < 10
		}
		p := s.pilot(shooter.Pilot, shooter.Side())
		p.Shots++
		if sh.Hit {
			p.Hits++
		}
		s.Shots = append(s.Shots, sh)
	}
}

// findEngagements splits the shots and kills into fights: a new one after
// five quiet minutes.
func (s *Session) findEngagements() {
	type ev struct {
		at   time.Time
		shot *Shot
		kill *Kill
	}
	var evs []ev
	for _, sh := range s.Shots {
		evs = append(evs, ev{at: sh.At, shot: sh})
	}
	for _, k := range s.Kills {
		if k.Air {
			evs = append(evs, ev{at: k.At, kill: k})
		}
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].at.Before(evs[j].at) })
	var cur *Engagement
	for _, e := range evs {
		if cur == nil || e.at.Sub(cur.LastEvent) > 5*time.Minute {
			cur = &Engagement{N: len(s.Engagements) + 1, FirstShot: e.at}
			s.Engagements = append(s.Engagements, cur)
		}
		cur.LastEvent = e.at
		if e.shot != nil {
			cur.Shots = append(cur.Shots, e.shot)
		} else {
			e.kill.Engagement, e.kill.N = cur.N, len(cur.Kills)+1
			cur.Kills = append(cur.Kills, e.kill)
		}
	}
	for _, en := range s.Engagements {
		for _, k := range en.Kills {
			switch k.VictimSide {
			case "blue":
				en.LostBlue++
			case "red":
				en.LostRed++
			}
			if k.shot != nil {
				k.shot.KillN = k.N
			}
		}
		for _, sh := range en.Shots {
			switch {
			case sh.KillN > 0:
				sh.Result = fmt.Sprintf("KILL %d", sh.KillN)
			case sh.Hit:
				sh.Result = "hit"
			case sh.Target == "":
				sh.Result = "no target"
			default:
				sh.Result = "miss"
			}
			if sh.Friendly {
				sh.Result += " (" + sh.Side + "-on-" + sh.Side + ")"
			}
		}
		s.engagementTeams(en)
	}
}

// engagementTeams: the players airborne within 40 nm of the shots in the
// fight's window, the takeoff, and who won.
func (s *Session) engagementTeams(en *Engagement) {
	blue, red := map[string]bool{}, map[string]bool{}
	add := func(name, side string) {
		switch side {
		case "blue":
			blue[name] = true
		case "red":
			red[name] = true
		}
	}
	for _, sh := range en.Shots {
		add(sh.Shooter, sh.Side)
		if sh.Target != "" {
			add(sh.Target, sh.TargetSide)
		}
	}
	if s.Rec != nil {
		en.from = s.Rec.Sec(en.FirstShot) - 60
		en.to = s.Rec.Sec(en.LastEvent) + 20
		var lat, lon float64
		n := 0
		for _, sh := range en.Shots {
			if sh.jet != nil {
				p := sh.jet.At(s.Rec.Sec(sh.At))
				lat, lon, n = lat+p.Lat, lon+p.Lon, n+1
			}
		}
		centre := Point{Lat: lat / math.Max(float64(n), 1), Lon: lon / math.Max(float64(n), 1)}
		en.routeFrom = en.from
		for _, o := range s.Rec.Aircraft {
			if o.End < en.from || o.Start > en.to {
				continue
			}
			isPlayer := false
			for _, p := range s.Pilots {
				if p.Key == o.Pilot {
					isPlayer = true
				}
			}
			involved := false
			for _, sh := range en.Shots {
				if sh.jet == o || sh.tgt == o {
					involved = true
				}
			}
			near := false
			if n > 0 && isPlayer {
				for t := math.Max(en.from, o.Start); t <= math.Min(en.to, o.End); t += 10 {
					if p := o.At(t); groundM(p, centre) < 40*nm && o.SpeedKt(t) > 120 {
						near = true
						break
					}
				}
			}
			if !(involved || near) {
				continue
			}
			en.jets = append(en.jets, o)
			if isPlayer {
				add(ShortName(o.Pilot), o.Side())
			}
			if !isPlayer {
				continue
			}
			// the routes in: from when this jet was last on the ground (or
			// appeared) before the fight, at most 30 minutes back
			start := o.Start
			for i := len(o.Track) - 1; i >= 0; i-- {
				if pt := o.Track[i]; pt.T < en.from && pt.AGL >= 0 && pt.AGL < 5 {
					start = pt.T
					break
				}
			}
			en.routeFrom = math.Min(en.routeFrom, math.Max(start, en.from-30*60))
		}
		if prev := en.N - 2; prev >= 0 && prev < len(s.Engagements) {
			en.routeFrom = math.Max(en.routeFrom, s.Engagements[prev].to)
		}
		if en.routeFrom < en.from-90 {
			en.Takeoff = s.Rec.Time(en.routeFrom)
		}
	}
	for n := range blue {
		en.Blue = append(en.Blue, n)
	}
	for n := range red {
		en.Red = append(en.Red, n)
	}
	sort.Strings(en.Blue)
	sort.Strings(en.Red)
	switch {
	case en.LostRed > en.LostBlue:
		en.Winner = "Blue"
	case en.LostBlue > en.LostRed:
		en.Winner = "Red"
	default:
		en.Winner = "Draw"
	}
}

// AircraftName is how pilots say a DCS type: "FA-18C_hornet" -> "F/A-18C".
func AircraftName(t string) string {
	if n, ok := map[string]string{"FA-18C_hornet": "F/A-18C", "F-16C_50": "F-16C", "F-15ESE": "F-15E", "AV8BNA": "AV-8B",
		"A-10C_2": "A-10C II", "AH-64D_BLK_II": "AH-64D", "M-2000C": "Mirage 2000C", "F-14A-135-GR": "F-14A", "Su-25T": "Su-25T",
		"MiG-29 Fulcrum": "MiG-29A", "Mi-24P": "Mi-24P", "UH-1H": "UH-1H", "OH58D": "OH-58D", "CH-47Fbl1": "CH-47F"}[t]; ok {
		return n
	}
	return strings.ReplaceAll(t, "_", " ")
}

// WeaponFamily groups variants: AIM-120B and C are both AIM-120.
func WeaponFamily(w string) string {
	for _, f := range []string{"AIM-120", "AIM-7", "AIM-54", "R-77", "R-27"} {
		if strings.HasPrefix(w, f) {
			return f
		}
	}
	return w
}

func (s *Session) missileStats() {
	idx := map[string]*MissileStat{}
	var order []string
	for _, sh := range s.Shots {
		if !sh.human || sh.Friendly { // a blue-on-blue shot is in Safety, not the count
			continue
		}
		k := sh.Side + "|" + WeaponFamily(sh.Weapon)
		m := idx[k]
		if m == nil {
			m = &MissileStat{Side: sh.Side, Weapon: WeaponFamily(sh.Weapon)}
			idx[k] = m
			order = append(order, k)
		}
		m.Fired++
		if sh.Hit {
			m.Hit++
		}
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := idx[order[i]], idx[order[j]]
		if a.Side != b.Side {
			return a.Side < b.Side
		}
		return a.Weapon < b.Weapon
	})
	for _, k := range order {
		s.Missiles = append(s.Missiles, *idx[k])
	}
}

// safety: blue-on-blue shots, crashes with no kill (the ground), ejections
// that were not after a kill.
func (s *Session) safety(es *EventSession) {
	for _, sh := range s.Shots {
		if !sh.Friendly || sh.Target == "" {
			continue
		}
		res := map[bool]string{true: "It hit.", false: "It missed."}[sh.Hit]
		if sh.KillN > 0 {
			res = "It killed him."
		}
		where := ""
		if sh.RangeNM >= 0 {
			where = fmt.Sprintf(" from %.1f nm", sh.RangeNM)
			if sh.jet != nil && sh.tgt != nil && onSix(sh.tgt, sh.jet, s.Rec.Sec(sh.At)) {
				where += " on his six"
			}
		}
		side := capitalize(sh.Side)
		s.Safety = append(s.Safety, SafetyItem{At: sh.At, Kind: side + "-on-" + sh.Side,
			Text: fmt.Sprintf("at %s, %s fired %s at %s%s. %s", s.Clock(sh.At), sh.Shooter, shortWeapon(sh.Weapon), sh.Target, where, res)})
	}
	if es == nil {
		return
	}
	for _, e := range es.Events {
		if !e.Init.Human() || (e.Type != "crash" && e.Type != "eject") {
			continue
		}
		// Not when that jet had just been shot down, nor the crash of a jet
		// its pilot had just left.
		skip := false
		for _, k := range s.Kills {
			if d := e.At.Sub(k.At).Seconds(); k.Victim == ShortName(e.Init.Pilot) && d > -20 && d < 120 {
				skip = true
			}
		}
		for _, f := range es.Events {
			if d := e.At.Sub(f.At).Seconds(); e.Type == "crash" && f.Type == "eject" && f.Init.Pilot == e.Init.Pilot && d >= 0 && d < 90 {
				skip = true
			}
		}
		if skip {
			continue
		}
		who := ShortName(e.Init.Pilot)
		switch e.Type {
		case "crash":
			text := fmt.Sprintf("at %s, %s's jet hit the ground", s.Clock(e.At), who)
			if j := s.jetOf(e.Init.Pilot, e.At); j != nil {
				t := math.Min(s.Rec.Sec(e.At), j.End) // the jet can leave the recording before the event
				a, b := j.At(t-6), j.At(t)
				if b.T-a.T > 1 {
					fpm := (a.AltM - b.AltM) * 3.28084 / (b.T - a.T) * 60
					text = fmt.Sprintf("at %s, %s's jet hit the ground at about %s ft", s.Clock(e.At), who, thousands(b.AltM*3.28084, 100))
					if fpm > 5000 {
						text += fmt.Sprintf(", descending at over %s ft/min", thousands(math.Floor(fpm/5000)*5000, 1))
					}
				}
			}
			s.Safety = append(s.Safety, SafetyItem{At: e.At, Kind: "Terrain", Text: text + "."})
		case "eject":
			s.Safety = append(s.Safety, SafetyItem{At: e.At, Kind: "Ejection", Text: fmt.Sprintf("at %s, %s ejected without being shot down.", s.Clock(e.At), who)})
		}
	}
	sort.SliceStable(s.Safety, func(i, j int) bool { return s.Safety[i].At.Before(s.Safety[j].At) })
}

// onSix: the shooter is within 45° of the target's tail.
func onSix(target, shooter *Object, t float64) bool {
	a, b := target.At(t-1), target.At(t)
	hdg := math.Atan2(b.Lon-a.Lon, b.Lat-a.Lat)
	sp := shooter.At(t)
	brg := math.Atan2(sp.Lon-b.Lon, sp.Lat-b.Lat)
	off := math.Abs(math.Remainder(brg-hdg, 2*math.Pi))
	return off > math.Pi*3/4
}

// sides: player-versus-player, and each side's squadron word for the
// headline ("Venom" from groups Venom-1, Venom-2).
func (s *Session) sides() {
	words := map[string]map[string]int{"blue": {}, "red": {}}
	seen := map[string]bool{}
	if s.Rec != nil {
		for _, o := range s.Rec.Aircraft {
			for _, p := range s.Pilots {
				if p.Key == o.Pilot && o.Group != "" && !seen[o.ID] {
					seen[o.ID] = true
					w := strings.FieldsFunc(o.Group, func(r rune) bool { return r == ' ' || r == '-' || unicode.IsDigit(r) })
					if len(w) > 0 && words[o.Side()] != nil {
						words[o.Side()][w[0]]++
					}
				}
			}
		}
	}
	top := func(m map[string]int) string {
		best, n := "", 0
		for w, c := range m {
			if c > n || (c == n && w < best) {
				best, n = w, c
			}
		}
		return best
	}
	s.BlueName, s.RedName = top(words["blue"]), top(words["red"])
	blue, red := false, false
	for _, p := range s.Pilots {
		switch p.Side {
		case "blue":
			blue = true
		case "red":
			red = true
		}
	}
	s.PvP = blue && red
	sort.Slice(s.Pilots, func(i, j int) bool {
		a, b := s.Pilots[i], s.Pilots[j]
		if a.Kills != b.Kills {
			return a.Kills > b.Kills
		}
		if a.ShotDown != b.ShotDown {
			return a.ShotDown < b.ShotDown
		}
		return a.Name < b.Name
	})
}

// Clock is a time on the report: this PC's local time, "19:16".
func (s *Session) Clock(t time.Time) string { return t.In(s.Loc).Format("15:04") }

// ZoneNote says which time the report is in: "times local (UTC-6)".
func (s *Session) ZoneNote() string {
	_, off := s.Start.In(s.Loc).Zone()
	h := off / 3600
	if off%3600 != 0 {
		return fmt.Sprintf("times local (UTC%+.1f)", float64(off)/3600)
	}
	return fmt.Sprintf("times local (UTC%+d)", h)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// shortWeapon is how the report names a weapon in a sentence: "a 9X".
func shortWeapon(w string) string {
	if w == "" {
		return "guns"
	}
	return "a " + strings.TrimPrefix(w, "AIM-")
}

func thousands(v, round float64) string {
	v = math.Round(v/round) * round
	s := fmt.Sprintf("%.0f", v)
	n := len(s)
	if n <= 3 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
