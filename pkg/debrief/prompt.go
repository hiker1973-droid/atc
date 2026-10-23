package debrief

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/miz"
)

// Inputs is what a debrief is written from. Any source may be missing.
type Inputs struct {
	Squadron string
	Focus    string // the person's own note: "focus on Venom flight's recovery"
	From, To time.Time
	Tacview  *Recording
	Server   *ServerLog
	Radio    *Radio
	Mission  *miz.Briefing // the .miz: what was planned
}

// System is the instruction Claude gets with every report: a standard
// military after action report (operator 2026-10-23: "we want it to be a
// after action report", standard military AAR, branded PDF).
const System = `You are the operations officer of a DCS World virtual squadron, writing the After Action Report (AAR) for a session. The squadron flies with Readback ATC, an AI air traffic control system: its towers, ATIS, Command, AWACS, Marshal and LSO talk to pilots over SRS radio, and speech recognition turns the pilots' calls into text (so a garbled pilot line may be a recognition fault, not the pilot's).

Write the AAR from the records provided: the mission file (the plan), the Tacview recording (what happened in the air), Readback's radio log and comms report, and the DCS server log. Use only what the records show and say plainly when something is uncertain or not recorded. Kills are matched to shooters from weapon impact points, so call them "likely". Times are UTC (Z).

The report's header and data block (unit, mission, date, times, weather, sorties, weapons, kills, losses) are printed above your text from the same records, so do not repeat them as a table or a title. Start directly with section 1 and use exactly these Markdown headings, in this order:

## 1. Mission Overview
Begin with a line "**BLUF:** ..." (bottom line up front: the outcome in one or two sentences). Then the mission's purpose, the package that flew and the conditions.
## 2. What Was Planned
The intent and tasks from the mission briefing, the flights available and any focus the squadron gave. If the mission file is missing, say what can be inferred and that the plan was not provided.
## 3. What Happened
A chronological account, with a short timeline of key events (as a bullet list "HH:MMZ - event"): takeoffs, check-ins, tasking, engagements, kills and losses, recoveries, landings.
## 4. Results
Effects against the plan: sorties flown, kills (likely) and losses, weapons expended and their effectiveness, objectives met or not.
## 5. Flight Summaries
One "### <flight or callsign>" subsection per player flight: aircraft, what it did, results, and its radio work (procedure, readbacks, brevity, missed or wrong calls).
## 6. Analysis
Why things happened as they did: the causes behind the successes and the problems, including air traffic control and comms (calls not understood, near misses, clearance problems; the pilot's phrasing or Readback's recognition), and server issues only if they affected the flying.
## 7. Sustain / Improve
Two lists: "**Sustain**" (what went well and should be repeated) and "**Improve**" (what fell short), each point specific.
## 8. Recommendations and Follow-up Actions
A numbered list of concrete actions for the next session, each ending with who owns it in brackets, e.g. "[Flight leads]", "[Mission designer]", "[All pilots]", "[ATC admin]".

Write in the concise, factual, constructive style of a real squadron AAR. Refer to pilots by callsign.`

// Build turns the inputs into the message sent to Claude.
func Build(in Inputs) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	squadron := in.Squadron
	if squadron == "" {
		squadron = "the squadron"
	}
	w("# Session records for %s", squadron)
	if !in.From.IsZero() {
		w("Session window: %s to %s UTC", in.From.UTC().Format("2006-01-02 15:04"), in.To.UTC().Format("15:04"))
	}
	if f := strings.TrimSpace(in.Focus); f != "" {
		w("\nThe squadron asks: %s", f)
	}

	if m := in.Mission; m != nil {
		w("\n## Mission plan (mission file %s)", m.File)
		w("Mission: %s. Map: %s. In-mission date %s, start %s.", m.Name, m.Theatre, m.Date, m.StartTime)
		if wx := WeatherLine(m); wx != "" {
			w("Weather at start: %s", wx)
		}
		if m.Description != "" {
			w("\n### Briefing\n%s", clip(m.Description, 5000))
		}
		if m.BlueTask != "" {
			w("\n### Blue task\n%s", clip(m.BlueTask, 2500))
		}
		if m.RedTask != "" {
			w("\n### Red task\n%s", clip(m.RedTask, 1500))
		}
		if len(m.Flights) > 0 {
			var fl []string
			for i, f := range m.Flights {
				if i == 80 {
					fl = append(fl, fmt.Sprintf("... and %d more", len(m.Flights)-80))
					break
				}
				fl = append(fl, fmt.Sprintf("%s (%s, %dx %s, %s)", f.Group, f.Coalition, f.Slots, f.Aircraft, f.Task))
			}
			w("\n### Flights with player slots (%d)\n%s", len(m.Flights), strings.Join(fl, "; "))
		}
	}

	if r := in.Tacview; r != nil {
		w("\n## Tacview recording")
		if r.Title != "" {
			w("Mission: %s", r.Title)
		}
		w("Recorded %s to %s UTC (in-mission time %s). Weapons launched in total: %d.",
			r.Start.UTC().Format("2006-01-02 15:04:05"), r.End.UTC().Format("15:04:05"), r.MissionTime, r.WeaponsTotal)
		var humans, ai []*Flight
		for _, f := range r.Flights {
			if f.Human {
				humans = append(humans, f)
			} else if len(f.Takeoffs)+len(f.Landings)+len(f.Kills) > 0 || len(f.Fired) > 0 || strings.HasPrefix(f.Fate, "destroyed") {
				ai = append(ai, f)
			}
		}
		w("\n### Player aircraft (%d)", len(humans))
		for _, f := range humans {
			writeFlight(w, f)
		}
		if len(ai) > 0 {
			w("\n### AI aircraft that did something (%d)", len(ai))
			if len(ai) > 40 {
				ai = ai[:40]
			}
			for _, f := range ai {
				writeFlight(w, f)
			}
		}
		if len(r.Losses) > 0 {
			w("\n### Everything destroyed (%d)", len(r.Losses))
			for i, l := range r.Losses {
				if i == 150 {
					w("- ... and %d more", len(r.Losses)-150)
					break
				}
				by := ""
				if l.By != "" {
					by = ", likely by " + l.By
				}
				w("- %s %s [%s, %s]%s", l.At.UTC().Format("15:04:05Z"), l.Name, l.Type, l.Coalition, by)
			}
		}
	}

	if rad := in.Radio; rad != nil && len(rad.Lines) > 0 {
		w("\n## Readback radio log (%d lines; %d calls not understood)", len(rad.Lines), rad.Unheard)
		if rad.Truncated > 0 {
			w("(the first %d lines of the window are left out)", rad.Truncated)
		}
		w("PILOT = what speech recognition heard; ATC = what Readback said; REPORT = Readback's comms report.")
		for _, l := range rad.Lines {
			tag := map[string]string{"pilot": "PILOT", "atc": "ATC", "report": "REPORT", "unheard": "UNHEARD"}[l.Kind]
			w("%s [%s] %s: %s", l.At.UTC().Format("15:04:05Z"), l.Station, tag, l.Text)
		}
	}

	if s := in.Server; s != nil {
		w("\n## DCS server log (dcs.log)")
		w("Covers %s to %s UTC. Lines by level: %s", s.Start.Format("2006-01-02 15:04:05"), s.End.Format("15:04:05"), levels(s.Levels))
		if len(s.Problems) > 0 {
			w("\n### Errors (grouped, most frequent first)")
			for _, p := range s.Problems {
				w("- %dx %s %s: %s (first %s)", p.Count, p.Level, p.Source, p.Text, p.First.Format("15:04:05Z"))
			}
		}
		if len(s.Events) > 0 {
			w("\n### Session events")
			for _, e := range s.Events {
				w("- %s", e)
			}
		}
	}
	return b.String()
}

func writeFlight(w func(string, ...any), f *Flight) {
	who := f.Pilot
	if who == "" {
		who = "(no pilot name)"
	}
	w("- %s - %s, group %s, %s. Seen %s-%s, max %.0f ft.", who, f.Aircraft, f.Group, f.Coalition,
		f.FirstSeen.UTC().Format("15:04Z"), f.LastSeen.UTC().Format("15:04Z"), f.MaxAltFt)
	if len(f.Takeoffs) > 0 {
		w("  Took off: %s", strings.Join(f.Takeoffs, "; "))
	}
	if len(f.Fired) > 0 {
		var ws []string
		for k, n := range f.Fired {
			ws = append(ws, fmt.Sprintf("%dx %s", n, k))
		}
		sort.Strings(ws)
		w("  Fired: %s", strings.Join(ws, ", "))
	}
	if len(f.Kills) > 0 {
		w("  Likely kills: %s", strings.Join(f.Kills, "; "))
	}
	if len(f.Landings) > 0 {
		w("  Landed: %s", strings.Join(f.Landings, "; "))
	}
	fate := f.Fate
	if f.KilledBy != "" {
		fate += ", likely by " + f.KilledBy
	}
	w("  End: %s", fate)
}

func levels(m map[string]int) string {
	var parts []string
	for k, n := range m {
		parts = append(parts, fmt.Sprintf("%s %d", k, n))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
