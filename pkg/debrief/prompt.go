package debrief

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Inputs is what a debrief is written from. Any source may be missing.
type Inputs struct {
	Squadron string
	Focus    string // the person's own note: "focus on Venom flight's recovery"
	From, To time.Time
	Tacview  *Recording
	Server   *ServerLog
	Radio    *Radio
}

// System is the instruction Claude gets with every debrief.
const System = `You are the debriefing officer for a DCS World virtual squadron. The squadron flies with Readback ATC, an AI air traffic control system: its towers, ATIS, Command, AWACS, Marshal and LSO talk to pilots over SRS radio, and speech recognition turns the pilots' calls into text (so a garbled pilot line may be a recognition fault, not the pilot's).

Write a debrief of the session from the records provided. Use only what the records show; say when something is uncertain (weapon-to-kill matches are inferred from impact points and are "likely", not certain). Times are UTC.

Format in Markdown with these sections, leaving out any the records cannot support:
## Summary - what was flown, by whom, and how it went, in a short paragraph.
## Timeline - the key moments in order (takeoffs, check-ins, engagements, kills and losses, recoveries).
## Flights - one short subsection per human flight or pilot: what they did, results, and comms (procedure, readbacks, brevity, missed or wrong calls).
## Air traffic control - how the radio went: calls not understood, near misses and clearance problems from the comms report, and whether faults look like the pilot's phrasing or Readback's recognition.
## Server - only if the server log shows problems worth acting on (crashes, script errors, disconnects).
## Lessons and recommendations - specific, actionable points for the next session.

Keep it professional and constructive, like a real squadron debrief. Refer to pilots by callsign.`

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
