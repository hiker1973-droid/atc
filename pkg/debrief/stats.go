package debrief

import (
	"sort"
	"strings"
	"time"
)

// Squadron statistics over many sessions (operator 2026-10-23: "we should
// be able to do a greenieboard, flight stats", "maybe do a separate
// page/tab and export function"): every mission run in the server's dcs.log
// files, added up per pilot, and every graded carrier pass.

// Totals is the squadron's numbers for a period.
type Totals struct {
	From, To time.Time
	Sessions int
	Pilots   []*PilotStat
	Passes   []Trap
}

// Board is one pilot's greenie board row.
type Board struct {
	Pilot  string
	Avg    float64 // grade points, foul-deck wave-offs not scored; -1 with none scored
	Passes []Trap
	Traps  int
}

// AddUp totals the mission runs that started between from and to (either
// may be zero for no limit). Runs seen twice (a log and its rotated copy)
// count once.
func AddUp(runs []EventSession, from, to time.Time, loc *time.Location) *Totals {
	t := &Totals{From: from, To: to}
	seen := map[string]bool{}
	byKey := map[string]*PilotStat{}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Start.Before(runs[j].Start) })
	for i := range runs {
		run := &runs[i]
		if (!from.IsZero() && run.Start.Before(from)) || (!to.IsZero() && !run.Start.Before(to)) || len(run.Events) == 0 {
			continue
		}
		id := run.Start.UTC().Format(time.RFC3339)
		if seen[id] {
			continue
		}
		seen[id] = true
		t.Sessions++
		s := &Session{Loc: loc, Start: run.Start, End: run.End()}
		s.fromEvents(run)
		for _, p := range s.Pilots {
			tot := byKey[p.Key]
			if tot == nil {
				tot = &PilotStat{Key: p.Key, Name: p.Name, Side: p.Side}
				byKey[p.Key] = tot
				t.Pilots = append(t.Pilots, tot)
			}
			for _, a := range p.Aircraft {
				tot.Aircraft = addUnique(tot.Aircraft, a)
			}
			tot.Sorties += p.Sorties
			tot.Flight += p.Flight
			tot.Landings += p.Landings
			tot.Traps += p.Traps
			tot.Kills += p.Kills
			tot.Ground += p.Ground
			tot.ShotDown += p.ShotDown
			tot.Crashes += p.Crashes
			tot.Ejections += p.Ejections
			tot.Shots += p.Shots
			tot.Hits += p.Hits
		}
		t.Passes = append(t.Passes, s.Traps...)
	}
	sort.Slice(t.Pilots, func(i, j int) bool {
		if t.Pilots[i].Flight != t.Pilots[j].Flight {
			return t.Pilots[i].Flight > t.Pilots[j].Flight
		}
		return t.Pilots[i].Name < t.Pilots[j].Name
	})
	return t
}

// GreenieBoard ranks the pilots by average grade points, then passes.
func (t *Totals) GreenieBoard() []Board {
	by := map[string]*Board{}
	var out []*Board
	for _, p := range t.Passes {
		b := by[p.Pilot]
		if b == nil {
			b = &Board{Pilot: p.Pilot}
			by[p.Pilot] = b
			out = append(out, b)
		}
		b.Passes = append(b.Passes, p)
		if p.Wire > 0 {
			b.Traps++
		}
	}
	for _, b := range out {
		sum, n := 0.0, 0
		for _, p := range b.Passes {
			if pts, ok := GradePoints(p.Grade.Grade); ok {
				sum += pts
				n++
			}
		}
		b.Avg = -1
		if n > 0 {
			b.Avg = sum / float64(n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Avg != out[j].Avg {
			return out[i].Avg > out[j].Avg
		}
		if len(out[i].Passes) != len(out[j].Passes) {
			return len(out[i].Passes) > len(out[j].Passes)
		}
		return strings.ToLower(out[i].Pilot) < strings.ToLower(out[j].Pilot)
	})
	res := make([]Board, len(out))
	for i, b := range out {
		res[i] = *b
	}
	return res
}

// BoardColours are the greenie board squares: background and text.
func BoardColours(grade string) (string, string) {
	c, ok := boardColours[grade]
	if !ok {
		c = boardColours["---"]
	}
	return c[0], c[1]
}
