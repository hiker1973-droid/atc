package debrief

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The report's optional pages (operator 2026-10-24: "Do 1, 2, 3, 4, 5, 6, 7
// but put them as checkbox options when you load a miz, tacview and log.
// make the options for what we have as well"): a comms score for each
// pilot, a page per pilot, currency, weapon effectiveness, losses, the
// carrier recovery and the tankers. Everything here is counted on this PC
// from the server's events, Tacview and Readback's radio logs; Claude only
// writes about it.

// Options are the report's contents, as ticked on the Reports page.
type Options struct {
	Summary     bool `json:"summary"`
	Engagements bool `json:"engagements"`
	Pilots      bool `json:"pilots"`  // the pilots table and the session's greenie board
	Written     bool `json:"written"` // Claude's write-up
	Discord     bool `json:"discord"`
	Comms       bool `json:"comms"`
	PilotPages  bool `json:"pilotPages"`
	Currency    bool `json:"currency"`
	Weapons     bool `json:"weapons"`
	Losses      bool `json:"losses"`
	Recovery    bool `json:"recovery"`
	Tanker      bool `json:"tanker"`
}

// AllOptions is every page.
func AllOptions() Options {
	return Options{true, true, true, true, true, true, true, true, true, true, true, true}
}

// Extras is what the optional pages show.
type Extras struct {
	Comms         []CommsScore
	Unheard       int // calls not understood: not anyone's in particular
	Losses        []Loss
	Weapons       []WeaponRow
	Ground        []GroundRow
	FriendlyShots int
	Recoveries    []Recovery
	Tanker        []TankerRow
	Currency      []CurrencyRow
	CurrencyAt    time.Time
	HistoryRuns   int               // the mission runs the currency is counted from
	CallsignOf    map[string]string // pilot key -> radio callsign
}

// ComputeExtras works out the pages the options ask for.
func ComputeExtras(s *Session, rad *Radio, history []EventSession, o Options) *Extras {
	x := &Extras{CallsignOf: map[string]string{}}
	if s == nil {
		return x
	}
	if rad != nil {
		x.CallsignOf = matchCallsigns(s, rad)
		x.Unheard = rad.Unheard
	}
	if o.Comms || o.PilotPages {
		x.Comms = commsScores(s, rad, x.CallsignOf)
	}
	if o.Losses || o.PilotPages {
		x.Losses = losses(s, rad)
	}
	if o.Weapons || o.PilotPages {
		x.weapons(s)
	}
	if o.Recovery || o.PilotPages {
		x.Recoveries = recoveries(s, rad, x.CallsignOf)
	}
	if o.Tanker || o.PilotPages {
		x.Tanker = tankers(s, rad, x.CallsignOf)
	}
	if o.Currency || o.PilotPages {
		x.Currency, x.HistoryRuns, x.CurrencyAt = currency(history, s)
	}
	return x
}

var modexRE = regexp.MustCompile(`\d{3}`)

// matchCallsigns pairs the callsigns heard on the radio with the DCS
// players: the same callsign in the player's name ("RAIDER 302 | HIKER"),
// the modex anywhere in it, or - one player and one callsign - each other.
func matchCallsigns(s *Session, rad *Radio) map[string]string {
	out := map[string]string{}
	heard := map[string]string{} // key -> the way it was said most
	count := map[string]map[string]int{}
	for _, l := range rad.Lines {
		k := CallsignKey(l.Callsign)
		if k == "" || l.Kind != "pilot" {
			continue
		}
		if count[k] == nil {
			count[k] = map[string]int{}
		}
		count[k][l.Callsign]++
	}
	for k, m := range count {
		best, n := "", 0
		for c, v := range m {
			if v > n || (v == n && c < best) {
				best, n = c, v
			}
		}
		heard[k] = best
	}
	taken := map[string]bool{}
	for _, p := range s.Pilots {
		name := p.Key
		if i := strings.Index(name, "|"); i >= 0 {
			name = name[:i]
		}
		if k := CallsignKey(name); k != "" && heard[k] != "" {
			out[p.Key], taken[k] = heard[k], true
			continue
		}
		for _, d := range modexRE.FindAllString(p.Key, -1) {
			if c := heard["#"+d]; c != "" && !taken["#"+d] {
				out[p.Key], taken["#"+d] = c, true
				break
			}
		}
	}
	if len(s.Pilots) == 1 && len(heard) == 1 && len(out) == 0 {
		for _, c := range heard {
			out[s.Pilots[0].Key] = c
		}
	}
	return out
}

func pilotByCallsign(s *Session, callsignOf map[string]string, key string) *PilotStat {
	for _, p := range s.Pilots {
		if c := callsignOf[p.Key]; c != "" && CallsignKey(c) == key {
			return p
		}
	}
	return nil
}

// ---- 1. Comms score ----

// CommsScore is one aircraft's radio work in the session.
type CommsScore struct {
	Key      string   `json:"key"`
	Callsign string   `json:"callsign"`
	Pilot    string   `json:"pilot,omitempty"` // the player's short name, when matched
	Calls    int      `json:"calls"`
	Answered int      `json:"answered"` // calls a controller answered (an acknowledgement needs none)
	SayAgain int      `json:"sayAgain"`
	Asked    int      `json:"asked"` // handoffs given and readbacks asked for
	Done     int      `json:"done"`  // ... checked in on the next frequency, read back
	Flags    []string `json:"flags,omitempty"`
	Stations []string `json:"stations"`
	Score    int      `json:"score"`
	Prev     int      `json:"prev"` // the last saved report's score, -1 for none
	Notes    []string `json:"-"`    // what cost points
}

// Grade is the score in words.
func (c CommsScore) Grade() string {
	switch {
	case c.Score >= 90:
		return "Excellent"
	case c.Score >= 75:
		return "Good"
	case c.Score >= 60:
		return "Fair"
	}
	return "Needs work"
}

var (
	sayAgainRE = regexp.MustCompile(`(?i)say again|unable to copy|you were broken|no callsign received`)
	// What a controller asks a pilot to do on the radio. The squadron does
	// not read back tower clearances: a readback is scored only where the
	// controller asks for one (Hitman), and Readback says "good readback".
	readbackAskRE  = regexp.MustCompile(`(?i)\bread ?back\b`)
	readbackGoodRE = regexp.MustCompile(`(?i)\bgood read ?back\b`)
	handoffRE      = regexp.MustCompile(`(?i)\b(?:push|switch to|contact)\s+(command|marshal|paddles|departure|approach|tower|ground|darkstar|overlord|magic|union|texaco|shell|arco|hitman)\b`)
	// An acknowledgement needs no answer.
	ackRE = regexp.MustCompile(`(?i)\b(signals? charlie|wilco|roger|copy|copies|good day|thanks|thank you|g'day)\b`)
)

var flagWords = map[string]string{"callsign_changed": "changed callsign", "no_clearance": "no clearance", "near_miss": "near miss"}

func commsScores(s *Session, rad *Radio, callsignOf map[string]string) []CommsScore {
	if rad == nil {
		return nil
	}
	by := map[string]*CommsScore{}
	forms := map[string]map[string]int{}
	var order []string
	get := func(l RadioLine) *CommsScore {
		k := CallsignKey(l.Callsign)
		if k == "" {
			return nil
		}
		c := by[k]
		if c == nil {
			c = &CommsScore{Key: k, Callsign: l.Callsign, Prev: -1}
			by[k], forms[k] = c, map[string]int{}
			order = append(order, k)
		}
		if l.Kind == "pilot" {
			forms[k][l.Callsign]++
		}
		return c
	}
	clock := func(t time.Time) string { return s.Clock(t) }
	note := func(c *CommsScore, format string, a ...any) {
		if len(c.Notes) < 8 {
			c.Notes = append(c.Notes, fmt.Sprintf(format, a...))
		}
	}
	lines := rad.Lines
	for i, l := range lines {
		c := get(l)
		if c == nil {
			continue
		}
		switch l.Kind {
		case "pilot":
			c.Calls++
			c.Stations = addUnique(c.Stations, l.Station)
			answered := ackRE.MatchString(l.Text) && !strings.Contains(strings.ToLower(l.Text), "request")
			for j := i - 1; j >= 0 && l.At.Sub(lines[j].At) <= 45*time.Second; j-- {
				// the reply to what a controller just said ("taxi runway 26") needs no answer
				if r := lines[j]; r.Station == l.Station && CallsignKey(r.Callsign) == c.Key {
					answered = answered || r.Kind == "atc"
					break
				}
			}
			for _, r := range lines[i+1:] {
				if answered || r.At.Sub(l.At) > 12*time.Second {
					break
				}
				if r.Kind == "atc" && r.Station == l.Station && (r.Callsign == "" || CallsignKey(r.Callsign) == c.Key) {
					answered = true
				}
			}
			if answered {
				c.Answered++
			} else {
				note(c, "%s %s: no answer to %q", clock(l.At), l.Station, clipRunes(l.Text, 70))
			}
		case "atc":
			if sayAgainRE.MatchString(l.Text) {
				c.SayAgain++
			}
			switch m := handoffRE.FindStringSubmatch(l.Text); {
			case readbackAskRE.MatchString(l.Text) && !readbackGoodRE.MatchString(l.Text):
				c.Asked++
				done := false
				for _, r := range lines[i+1:] {
					if r.At.Sub(l.At) > 3*time.Minute {
						break
					}
					if r.Kind == "atc" && r.Station == l.Station && CallsignKey(r.Callsign) == c.Key && readbackGoodRE.MatchString(r.Text) {
						done = true
						break
					}
				}
				if done {
					c.Done++
				} else {
					note(c, "%s %s: readback asked for, no good readback", clock(l.At), l.Station)
				}
			case m != nil:
				c.Asked++
				done := false
				for _, r := range lines[i+1:] {
					if r.At.Sub(l.At) > 10*time.Minute {
						break
					}
					if r.Kind == "pilot" && r.Station != l.Station && CallsignKey(r.Callsign) == c.Key {
						done = true
						break
					}
				}
				if done {
					c.Done++
				} else {
					note(c, "%s %s: handed to %s, no call there within 10 minutes", clock(l.At), l.Station, capitalize(strings.ToLower(m[1])))
				}
			}
		case "report":
			if l.Report != "" {
				c.Flags = append(c.Flags, firstNonEmpty(flagWords[l.Report], strings.ReplaceAll(l.Report, "_", " ")))
			}
		}
	}
	var out []CommsScore
	for _, k := range order {
		c := by[k]
		if c.Calls == 0 {
			continue // only spoken to, never heard from
		}
		best, n := c.Callsign, 0 // the way it was said most, with its word
		for f, v := range forms[k] {
			if hasLetter(f) && (v > n || (v == n && f < best)) {
				best, n = f, v
			}
		}
		c.Callsign = best
		score := float64(c.Answered) / float64(c.Calls)
		if c.Asked > 0 {
			score = 0.6*score + 0.4*float64(c.Done)/float64(c.Asked)
		}
		pts := 100*score - 5*float64(c.SayAgain)
		for _, f := range c.Flags {
			if f == "changed callsign" {
				pts -= 3
			} else {
				pts -= 5
			}
		}
		c.Score = int(math.Round(math.Max(0, math.Min(100, pts))))
		if p := pilotByCallsign(s, callsignOf, k); p != nil {
			c.Pilot = p.Name
		}
		out = append(out, *c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Callsign < out[j].Callsign
	})
	return out
}

// ---- 5. Losses ----

// Loss is one player aircraft lost, and how.
type Loss struct {
	At       time.Time
	Pilot    string // short name
	Key      string
	Side     string
	Aircraft string
	Cause    string // Shot down, Crashed, Ejected, Pilot killed
	Detail   string
	Ejected  bool
	Died     bool
	CSAR     bool
}

var csarRE = regexp.MustCompile(`(?i)\bCSAR\b`)

func losses(s *Session, rad *Radio) []Loss {
	var out []Loss
	keyOf := map[string]string{}
	for _, p := range s.Pilots {
		keyOf[p.Name] = p.Key
	}
	for _, k := range s.Kills {
		key := keyOf[k.Victim]
		if key == "" {
			continue // an AI aircraft
		}
		d := fmt.Sprintf("by %s (%s) with %s", k.Killer, capitalize(k.KillerSide), firstNonEmpty(k.Weapon, "guns"))
		if sh := k.shot; sh != nil && sh.RangeNM >= 0 {
			d += fmt.Sprintf(", fired from %.1f nm", sh.RangeNM)
			if sh.AltFt > 0 {
				d += fmt.Sprintf(" at %s ft", thousands(sh.AltFt, 100))
			}
		}
		if k.AltFt > 0 {
			d += fmt.Sprintf("; hit at %s ft", thousands(k.AltFt, 100))
		}
		out = append(out, Loss{At: k.At, Pilot: k.Victim, Key: key, Side: k.VictimSide, Aircraft: k.VictimType, Cause: "Shot down", Detail: d})
	}
	if s.es != nil {
		for _, e := range s.es.Events {
			if !e.Init.Human() || (e.Type != "crash" && e.Type != "eject" && e.Type != "pilot dead") {
				continue
			}
			if s.Rec != nil && (e.At.Before(s.Start.Add(-time.Minute)) || e.At.After(s.End.Add(time.Minute))) {
				continue
			}
			who := ShortName(e.Init.Pilot)
			// part of a loss already listed: the crash after a kill or an ejection
			dup := false
			for i := range out {
				if dt := e.At.Sub(out[i].At).Seconds(); out[i].Pilot == who && dt > -20 && dt < 180 {
					dup = true
				}
			}
			if dup {
				continue
			}
			l := Loss{At: e.At, Pilot: who, Key: e.Init.Pilot, Side: e.Init.Side(), Aircraft: AircraftName(e.Init.Unit)}
			switch e.Type {
			case "crash":
				l.Cause, l.Detail = "Crashed", s.crashDetail(e.Init.Pilot, e.At)
			case "eject":
				l.Cause, l.Detail = "Ejected", "not shot down"
			default:
				l.Cause, l.Detail = "Pilot killed", "not shot down"
			}
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	for i := range out {
		l := &out[i]
		if s.es != nil {
			for _, e := range s.es.Events {
				if ShortName(e.Init.Pilot) != l.Pilot {
					continue
				}
				if dt := e.At.Sub(l.At).Seconds(); dt > -20 && dt < 180 {
					switch e.Type {
					case "eject":
						l.Ejected = true
					case "pilot dead":
						l.Died = true
					}
				}
			}
		}
		if rad != nil {
			for _, r := range rad.Lines {
				if dt := r.At.Sub(l.At); r.Kind == "atc" && dt >= -30*time.Second && dt < 10*time.Minute && csarRE.MatchString(r.Text) {
					l.CSAR = true
					break
				}
			}
		}
	}
	return out
}

// crashDetail: how high and how fast down the jet was, and the field it was
// near (a landing or launch accident).
func (s *Session) crashDetail(pilot string, at time.Time) string {
	text := "not shot down"
	j := s.jetOf(pilot, at)
	if j == nil {
		return text
	}
	t := math.Min(s.Rec.Sec(at), j.End)
	a, b := j.At(t-6), j.At(t)
	if b.T-a.T > 1 {
		text = fmt.Sprintf("not shot down; last seen at about %s ft", thousands(b.AltM*3.28084, 100))
		if fpm := (a.AltM - b.AltM) * 3.28084 / (b.T - a.T) * 60; fpm > 5000 {
			text += fmt.Sprintf(", descending at over %s ft/min", thousands(math.Floor(fpm/5000)*5000, 1))
		}
	}
	for _, f := range s.Fields {
		if groundM(b, Point{Lat: f.Lat, Lon: f.Lon}) < 3*nm {
			text += "; within 3 nm of " + f.Name + " (a landing or launch accident?)"
			break
		}
	}
	return text
}

// ---- 4. Weapon effectiveness ----

// WeaponRow is one weapon's results for one side.
type WeaponRow struct {
	Side, Weapon      string
	Fired, Hits, Kill int
	RangeHit          float64 // average launch range of the hits, nm; -1 none
	RangeMiss         float64
	AltK              float64 // average launch altitude, thousands of ft; -1 none
}

// GroundRow is ground and sea kills by weapon.
type GroundRow struct {
	Weapon  string
	Kills   int
	Targets string // "3 T-72B, 2 ZSU-23-4"
}

func (x *Extras) weapons(s *Session) {
	idx := map[string]*WeaponRow{}
	type acc struct{ hit, miss, alt []float64 }
	sums := map[string]*acc{}
	var order []string
	kills := map[*Shot]int{}
	for _, k := range s.Kills {
		if k.shot != nil {
			kills[k.shot]++
		}
	}
	for _, sh := range s.Shots {
		if !sh.human {
			continue
		}
		if sh.Friendly {
			x.FriendlyShots++
			continue
		}
		key := sh.Side + "|" + WeaponFamily(sh.Weapon)
		w := idx[key]
		if w == nil {
			w = &WeaponRow{Side: sh.Side, Weapon: firstNonEmpty(WeaponFamily(sh.Weapon), "Guns")}
			idx[key], sums[key] = w, &acc{}
			order = append(order, key)
		}
		w.Fired++
		w.Kill += kills[sh]
		a := sums[key]
		if sh.Hit {
			w.Hits++
			if sh.RangeNM >= 0 {
				a.hit = append(a.hit, sh.RangeNM)
			}
		} else if sh.RangeNM >= 0 && sh.Target != "" {
			a.miss = append(a.miss, sh.RangeNM)
		}
		if sh.AltFt > 0 {
			a.alt = append(a.alt, sh.AltFt/1000)
		}
	}
	avg := func(v []float64) float64 {
		if len(v) == 0 {
			return -1
		}
		t := 0.0
		for _, x := range v {
			t += x
		}
		return t / float64(len(v))
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := idx[order[i]], idx[order[j]]
		if a.Side != b.Side {
			return a.Side < b.Side
		}
		return a.Fired > b.Fired
	})
	for _, k := range order {
		w := idx[k]
		w.RangeHit, w.RangeMiss, w.AltK = avg(sums[k].hit), avg(sums[k].miss), avg(sums[k].alt)
		x.Weapons = append(x.Weapons, *w)
	}
	players := map[string]bool{}
	for _, p := range s.Pilots {
		players[p.Name] = true
	}
	g := map[string]map[string]int{}
	var gorder []string
	for _, k := range s.Kills {
		if k.Air || !(players[k.Killer] || (k.shot != nil && k.shot.human)) {
			continue
		}
		w := firstNonEmpty(WeaponFamily(k.Weapon), "Guns or unknown")
		if g[w] == nil {
			g[w] = map[string]int{}
			gorder = append(gorder, w)
		}
		g[w][firstNonEmpty(k.VictimType, k.Victim)]++
	}
	for _, w := range gorder {
		row := GroundRow{Weapon: w}
		var ts []string
		for t, n := range g[w] {
			row.Kills += n
			ts = append(ts, fmt.Sprintf("%d %s", n, t))
		}
		sort.Strings(ts)
		row.Targets = strings.Join(ts, ", ")
		x.Ground = append(x.Ground, row)
	}
	sort.SliceStable(x.Ground, func(i, j int) bool { return x.Ground[i].Kills > x.Ground[j].Kills })
}

// ---- 6. Carrier recovery ----

// Recovery is one carrier's recovery: its passes close together.
type Recovery struct {
	Place      string
	Start, End time.Time // the first Marshal check-in (or the first pass) to the last pass
	Passes     int
	Traps      int
	Bolters    int
	WaveOffs   int
	FoulDeck   int
	Interval   time.Duration // between passes, on average
	AvgPoints  float64       // -1 with none scored
	Approaches []Approach
}

// Approach is one pilot's way through Marshal to the wire.
type Approach struct {
	Pilot, Callsign                       string
	CheckIn, Charlie, Commencing, Initial time.Time
	FirstPass, Trap                       time.Time
	Passes                                int
	Grades                                []Trap
}

func recoveries(s *Session, rad *Radio, callsignOf map[string]string) []Recovery {
	if len(s.Traps) == 0 {
		return nil
	}
	traps := append([]Trap(nil), s.Traps...)
	sort.SliceStable(traps, func(i, j int) bool { return traps[i].At.Before(traps[j].At) })
	var out []Recovery
	cur := map[string]*Recovery{} // place -> the recovery still open
	var order []*Recovery
	for _, t := range traps {
		r := cur[t.Place]
		if r == nil || t.At.Sub(r.End) > 25*time.Minute {
			r = &Recovery{Place: t.Place, Start: t.At}
			cur[t.Place] = r
			order = append(order, r)
		}
		r.End = t.At
		r.Passes++
		switch {
		case t.Wire > 0:
			r.Traps++
		case t.Grade.Grade == "B":
			r.Bolters++
		case t.Grade.Grade == "WOFD":
			r.FoulDeck++
		case t.Grade.Grade == "WO":
			r.WaveOffs++
		}
		var ap *Approach
		for i := range r.Approaches {
			if r.Approaches[i].Pilot == t.Pilot {
				ap = &r.Approaches[i]
			}
		}
		if ap == nil {
			r.Approaches = append(r.Approaches, Approach{Pilot: t.Pilot, FirstPass: t.At})
			ap = &r.Approaches[len(r.Approaches)-1]
		}
		ap.Passes++
		ap.Grades = append(ap.Grades, t)
		if t.Wire > 0 && ap.Trap.IsZero() {
			ap.Trap = t.At
		}
	}
	for _, r := range order {
		var pts []float64
		var prev time.Time
		var gaps []time.Duration
		for _, ap := range r.Approaches {
			for _, g := range ap.Grades {
				if p, ok := GradePoints(g.Grade.Grade); ok {
					pts = append(pts, p)
				}
			}
		}
		for _, t := range traps {
			if t.Place == r.Place && !t.At.Before(r.Start) && !t.At.After(r.End) {
				if !prev.IsZero() {
					gaps = append(gaps, t.At.Sub(prev))
				}
				prev = t.At
			}
		}
		r.AvgPoints = -1
		if len(pts) > 0 {
			sum := 0.0
			for _, p := range pts {
				sum += p
			}
			r.AvgPoints = sum / float64(len(pts))
		}
		if len(gaps) > 0 {
			var sum time.Duration
			for _, g := range gaps {
				sum += g
			}
			r.Interval = sum / time.Duration(len(gaps))
		}
		// Marshal's side: check-in, Charlie, commencing and the three mile initial.
		for i := range r.Approaches {
			ap := &r.Approaches[i]
			key := ""
			for _, p := range s.Pilots {
				if p.Name == ap.Pilot {
					ap.Callsign = callsignOf[p.Key]
					key = CallsignKey(ap.Callsign)
				}
			}
			if rad == nil || key == "" {
				continue
			}
			end := ap.FirstPass
			if !ap.Trap.IsZero() {
				end = ap.Trap
			}
			for _, l := range rad.Lines {
				if CallsignKey(l.Callsign) != key || l.At.Before(ap.FirstPass.Add(-75*time.Minute)) || l.At.After(end) ||
					!strings.Contains(l.Station, "MARSHAL") {
					continue
				}
				low := strings.ToLower(l.Text)
				switch {
				case l.Kind == "pilot" && ap.CheckIn.IsZero():
					ap.CheckIn = l.At
				}
				switch {
				case l.Kind == "atc" && strings.Contains(low, "charlie") && ap.Charlie.IsZero():
					ap.Charlie = l.At
				case l.Kind == "pilot" && strings.Contains(low, "commenc") && ap.Commencing.IsZero():
					ap.Commencing = l.At
				case l.Kind == "pilot" && strings.Contains(low, "initial") && ap.Initial.IsZero():
					ap.Initial = l.At
				}
			}
			if !ap.CheckIn.IsZero() && ap.CheckIn.Before(r.Start) {
				r.Start = ap.CheckIn
			}
		}
		out = append(out, *r)
	}
	return out
}

// ---- 7. Tankers ----

// TankerRow is one aircraft's tanking.
type TankerRow struct {
	Pilot, Callsign, Tanker string
	CheckIn, FirstPlug      time.Time
	Requested               string
	Plugs, Visits           int
	Connected               time.Duration
}

var (
	requestRE = regexp.MustCompile(`(?i)\brequest(?:ing)?\s+(full|[\d,.]+\s*(?:k|thousand)?)\s*(?:pounds|lbs?|pound)?`)
	tankerRE  = regexp.MustCompile(`^(TEXACO|SHELL|ARCO)\b`)
)

func tankers(s *Session, rad *Radio, callsignOf map[string]string) []TankerRow {
	rows := map[string]*TankerRow{} // pilot key, or "r:"+callsign key for the radio only
	var order []string
	row := func(k string) *TankerRow {
		if rows[k] == nil {
			rows[k] = &TankerRow{}
			order = append(order, k)
		}
		return rows[k]
	}
	if s.es != nil {
		open := map[string]time.Time{}
		last := map[string]time.Time{}
		for _, e := range s.es.Events {
			if !e.Init.Human() || (e.Type != "refuel" && e.Type != "refuel stop") {
				continue
			}
			k := e.Init.Pilot
			r := row(k)
			r.Pilot = ShortName(k)
			switch e.Type {
			case "refuel":
				r.Plugs++
				if r.FirstPlug.IsZero() {
					r.FirstPlug = e.At
				}
				if l, ok := last[k]; !ok || e.At.Sub(l) > 10*time.Minute {
					r.Visits++
				}
				open[k] = e.At
			case "refuel stop":
				if t0, ok := open[k]; ok {
					r.Connected += e.At.Sub(t0)
					delete(open, k)
				}
			}
			last[k] = e.At
		}
	}
	if rad != nil {
		for _, l := range rad.Lines {
			if !tankerRE.MatchString(l.Station) {
				continue
			}
			ck := CallsignKey(l.Callsign)
			if ck == "" {
				continue
			}
			k := "r:" + ck
			for pk, c := range callsignOf {
				if CallsignKey(c) == ck {
					k = pk
				}
			}
			r := row(k)
			r.Callsign = firstNonEmpty(r.Callsign, l.Callsign)
			if p := pilotByCallsign(s, callsignOf, ck); p != nil {
				r.Pilot = p.Name
			}
			if l.Kind == "pilot" {
				if r.CheckIn.IsZero() {
					r.CheckIn = l.At
					r.Tanker = capitalize(strings.ToLower(l.Station))
				}
				if m := requestRE.FindStringSubmatch(l.Text); m != nil && r.Requested == "" {
					r.Requested = strings.TrimSpace(m[1])
					if r.Requested != "full" {
						r.Requested += " lb"
					}
				}
			}
		}
	}
	var out []TankerRow
	for _, k := range order {
		r := rows[k]
		if r.Plugs == 0 && r.CheckIn.IsZero() {
			continue
		}
		if r.Callsign == "" {
			r.Callsign = callsignOf[k]
		}
		out = append(out, *r)
	}
	return out
}

// ---- 3. Currency ----

// CurrencyRow is one pilot's recent flying, from the server logs on this PC.
type CurrencyRow struct {
	Pilot                     string
	Key                       string
	LastSortie, LastTrap      time.Time
	Sorties30, Landings30     int
	Traps30, Traps90, Plugs30 int
	Carrier                   bool   // flies a carrier type or has trapped
	FlyStatus, TrapStatus     string // current, due, lapsed
}

var carrierTypes = regexp.MustCompile(`F/A-18|F-14|Su-33|T-45|MiG-29K|Rafale M|E-2|C-2|S-3`)

// Status: within 30 days current, 60 due, else lapsed.
func currencyStatus(last, at time.Time) string {
	switch {
	case last.IsZero():
		return "lapsed"
	case at.Sub(last) <= 30*24*time.Hour:
		return "current"
	case at.Sub(last) <= 60*24*time.Hour:
		return "due"
	}
	return "lapsed"
}

func currency(history []EventSession, s *Session) ([]CurrencyRow, int, time.Time) {
	at := s.End
	if at.IsZero() {
		at = time.Now()
	}
	rows := map[string]*CurrencyRow{}
	var keys []string
	for _, p := range s.Pilots {
		rows[p.Key] = &CurrencyRow{Pilot: p.Name, Key: p.Key, Carrier: carrierTypes.MatchString(strings.Join(p.Aircraft, " ")) || p.Traps > 0}
		keys = append(keys, p.Key)
	}
	seen := map[string]bool{}
	runs := 0
	for i := range history {
		run := &history[i]
		id := run.Start.UTC().Format(time.RFC3339)
		if seen[id] || run.Start.After(at.Add(time.Hour)) || len(run.Events) == 0 {
			continue
		}
		seen[id] = true
		runs++
		for _, e := range run.Events {
			r := rows[e.Init.Pilot]
			if r == nil || e.At.After(at.Add(time.Hour)) {
				continue
			}
			in30 := at.Sub(e.At) <= 30*24*time.Hour
			switch e.Type {
			case "takeoff":
				if e.At.After(r.LastSortie) {
					r.LastSortie = e.At
				}
				if in30 {
					r.Sorties30++
				}
			case "land":
				if in30 {
					r.Landings30++
				}
			case "landing quality mark":
				if ParseGrade(e.Comment).Wire == 0 {
					break
				}
				r.Carrier = true
				if e.At.After(r.LastTrap) {
					r.LastTrap = e.At
				}
				if in30 {
					r.Traps30++
				}
				if at.Sub(e.At) <= 90*24*time.Hour {
					r.Traps90++
				}
			case "refuel":
				if in30 {
					r.Plugs30++
				}
			}
		}
	}
	var out []CurrencyRow
	for _, k := range keys {
		r := rows[k]
		r.FlyStatus = currencyStatus(r.LastSortie, at)
		if r.Carrier {
			r.TrapStatus = currencyStatus(r.LastTrap, at)
		}
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pilot < out[j].Pilot })
	return out, runs, at
}
