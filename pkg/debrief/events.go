package debrief

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The DCS server writes every simulation event to its dcs.log as a
// "Scripting (Main): event:type=shot,initiatorPilotName=...,t=..." line:
// shots with their target, hits, kills, takeoffs and landings with the
// place, crashes, ejections, deaths and the LSO's carrier grades. These are
// what DCS itself scored, so the report's kills, shots and sorties come from
// here; Tacview adds the geometry (where, how high, how fast, the tracks).
// A game client's dcs.log has none of them: it has to be the server's.

// Party is one side of an event: the shooter, the target, the pilot.
type Party struct {
	Pilot     string // a player's name ("RAIDER 031 | BACON"), or the unit type for AI
	Unit      string // the unit type, "FA-18C_hornet"
	Coalition int    // 1 red, 2 blue, 0 neutral
	Air       bool   // ws_type1 1: an aircraft
}

// Human reports whether a player (not the AI) was in it: DCS names an AI
// pilot after its unit type.
func (p Party) Human() bool { return p.Pilot != "" && p.Pilot != p.Unit }

// Side is "blue", "red" or "".
func (p Party) Side() string {
	switch p.Coalition {
	case 1:
		return "red"
	case 2:
		return "blue"
	}
	return ""
}

// ServerEvent is one event: line.
type ServerEvent struct {
	ID, Linked int
	T          float64   // mission seconds
	At         time.Time // real time (UTC)
	Type       string    // shot, hit, kill, takeoff, land, crash, eject, pilot dead, landing quality mark, ...
	Init       Party
	Target     Party
	Weapon     string
	Place      string
	Comment    string
}

// EventSession is the events of one mission run (mission start to the next).
type EventSession struct {
	Start  time.Time // the "mission start" line's real time (UTC)
	Events []ServerEvent
}

// End is the time of the session's last event.
func (s *EventSession) End() time.Time {
	if len(s.Events) == 0 {
		return s.Start
	}
	return s.Events[len(s.Events)-1].At
}

var kvRE = regexp.MustCompile(`(\w+)=([^,]*)`)

// parseEventLine reads the part after "event:".
func parseEventLine(s string) ServerEvent {
	kv := map[string]string{}
	for _, m := range kvRE.FindAllStringSubmatch(s, -1) {
		kv[m[1]] = strings.TrimSpace(m[2])
	}
	num := func(k string) int { n, _ := strconv.Atoi(kv[k]); return n }
	party := func(prefix, pilot string) Party {
		return Party{Pilot: kv[pilot], Unit: kv[prefix+"_unit_type"], Coalition: num(prefix + "_coalition"), Air: kv[prefix+"_ws_type1"] == "1"}
	}
	t, _ := strconv.ParseFloat(kv["t"], 64)
	return ServerEvent{ID: num("event_id"), Linked: num("linked_event_id"), T: t, Type: kv["type"],
		Init: party("initiator", "initiatorPilotName"), Target: party("target", "targetPilotName"),
		Weapon: kv["weapon"], Place: firstNonEmpty(kv["placeDisplayName"], kv["place"]), Comment: kv["comment"]}
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}

// SessionFor picks the mission run the report is about: the one that
// started nearest a Tacview recording's start, else the one that overlaps
// the window most, else the last.
func (s *ServerLog) SessionFor(recStart, from, to time.Time) *EventSession {
	if s == nil || len(s.Sessions) == 0 {
		return nil
	}
	if !recStart.IsZero() {
		var best *EventSession
		bestD := 5 * time.Minute
		for i := range s.Sessions {
			d := s.Sessions[i].Start.Sub(recStart)
			if d < 0 {
				d = -d
			}
			if d < bestD {
				best, bestD = &s.Sessions[i], d
			}
		}
		if best != nil {
			return best
		}
		// The recording may start after the mission did: the run it falls in.
		for i := len(s.Sessions) - 1; i >= 0; i-- {
			if !s.Sessions[i].Start.After(recStart) {
				return &s.Sessions[i]
			}
		}
	}
	if !from.IsZero() {
		var best *EventSession
		var bestO time.Duration
		for i := range s.Sessions {
			a, b := s.Sessions[i].Start, s.Sessions[i].End()
			if a.Before(from) {
				a = from
			}
			if b.After(to) {
				b = to
			}
			if o := b.Sub(a); o > bestO {
				best, bestO = &s.Sessions[i], o
			}
		}
		if best != nil {
			return best
		}
	}
	return &s.Sessions[len(s.Sessions)-1]
}

// Grade is one LSO grade from a "landing quality mark":
// "LSO: GRADE:_OK_ : WIRE# 2", "LSO: GRADE:WO  WOFDIC".
type Grade struct {
	Grade string // _OK_, OK, (OK), ---, C, B, WO, WOFD
	Wire  int
	Raw   string
}

var gradeRE = regexp.MustCompile(`GRADE:\s*(_OK_|\(OK\)|OK|---|--|B|C|WO|NC)`)
var wireRE = regexp.MustCompile(`WIRE#\s*(\d)`)

// ParseGrade reads the LSO comment.
func ParseGrade(comment string) Grade {
	g := Grade{Raw: strings.TrimSpace(strings.TrimPrefix(comment, "LSO:"))}
	if m := gradeRE.FindStringSubmatch(comment); m != nil {
		g.Grade = m[1]
	}
	if m := wireRE.FindStringSubmatch(comment); m != nil {
		g.Wire, _ = strconv.Atoi(m[1])
	}
	if g.Grade == "WO" && strings.Contains(comment, "WOFD") {
		g.Grade = "WOFD" // foul deck: not the pilot's
	}
	if g.Grade == "--" {
		g.Grade = "---"
	}
	if g.Grade == "" {
		g.Grade = "---"
	}
	return g
}

// GradePoints is the usual squadron scale (docs/paddles-report); a foul-deck
// wave-off is not scored.
func GradePoints(g string) (float64, bool) {
	p, ok := map[string]float64{"_OK_": 5, "OK": 4, "(OK)": 3, "B": 2.5, "---": 2, "WO": 1, "C": 0}[g]
	return p, ok
}
