package debrief

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/miz"
)

// Inputs is what a report is written from. Any source may be missing.
type Inputs struct {
	Squadron string
	Focus    string // the person's own note: "focus on Venom flight's recovery"
	From, To time.Time
	Tacview  *Recording
	Server   *ServerLog
	Radio    *Radio
	Mission  *miz.Briefing // the .miz: what was planned
	Session  *Session      // the analysis (Analyze)
}

// AIText is what Claude writes; the layout, numbers, tables and maps are the
// report's own.
type AIText struct {
	BLUF            string   `json:"bluf"`
	WhatWon         []Point2 `json:"what_won"`
	EngagementNotes []string `json:"engagement_notes"`
	Overview        string   `json:"overview"`
	Planned         string   `json:"planned"`
	Sustain         []string `json:"sustain"`
	Improve         []string `json:"improve"`
	Recommendations []Action `json:"recommendations"`
	Comms           string   `json:"comms"`
	Safety          string   `json:"safety"`
	Server          string   `json:"server"`
	Discord         string   `json:"discord"`
}

// Point2 is a bold lead and its sentence: "Launch high and fast." ...
type Point2 struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Action is a recommendation and who owns it.
type Action struct {
	Text  string `json:"text"`
	Owner string `json:"owner"`
}

// System is the instruction Claude gets with every report: a standard
// military after action report in the squadron's debrief style.
const System = `You are the operations officer of a DCS World virtual squadron, writing the After Action Report (AAR) for a session. The squadron flies with Readback ATC, an AI air traffic control system whose towers, ATIS, Command, AWACS, Marshal and LSO talk to pilots over SRS radio; speech recognition turns pilots' calls into text, so a garbled pilot line may be a recognition fault rather than the pilot's.

The records give you the mission plan, every shot (time, shooter, target, weapon, range, the shooter's altitude and speed at launch, result), every kill, per-pilot stats, safety events, carrier grades, the radio log and server errors. All numbers are already computed and are printed in the report's own tables and maps: do not restate tables, and never invent or recompute numbers. Use specific figures from the records only where they make a point. Kills with "likely" were matched from impact points. Times are local, as given.

Write like the squadron's own debriefs: short, direct, specific and constructive; refer to pilots by their short names; each point backed by what the records show. Plain sentences, no Markdown.

Answer with ONLY a JSON object, no other text, with these fields:
{
 "bluf": "Bottom line up front: the outcome in one or two sentences.",
 "what_won": [{"title": "Launch high and fast.", "text": "The evidence and the lesson in one or two sentences."}],  // 3-5 points: what decided the fights (or, with no fights, what decided the session)
 "engagement_notes": ["2-3 sentences per engagement, in order: how it went and the moments that decided it."],  // exactly one per engagement listed; empty list if none
 "overview": "Mission overview: purpose, package, conditions (2-4 sentences).",
 "planned": "What was planned, from the mission briefing and tasks, and how the session compared (2-4 sentences). Say so if the plan was not provided.",
 "sustain": ["What went well and should be repeated."],  // 2-5
 "improve": ["What fell short."],  // 2-5
 "recommendations": [{"text": "A concrete action for next time.", "owner": "Flight leads"}],  // 3-6; owner such as Flight leads, All pilots, Mission designer, ATC admin, Squadron CO
 "comms": "Air traffic control and radio: calls not understood, procedure, readbacks, the comms report; whether faults were the pilots' phrasing or Readback's recognition (2-4 sentences). Empty if no radio records.",
 "safety": "One or two sentences on the safety items, with the corrective action. Empty if none.",
 "server": "Only server problems that affected the flying; empty otherwise.",
 "discord": "A Discord post announcing the AAR: under 900 characters, Discord Markdown (**bold**, bullet lines with -), a bold title line with the mission and date, the result, 2-4 key points with names, and a line saying the full AAR PDF is attached. No @everyone, no links."
}`

// Build turns the inputs into the message sent to Claude.
func Build(in Inputs) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	squadron := in.Squadron
	if squadron == "" {
		squadron = "the squadron"
	}
	s := in.Session
	w("# Session records for %s", squadron)
	w("Mission: %s", MissionName(in))
	if s != nil && !s.Start.IsZero() {
		w("Session: %s %s to %s, %s.", s.Start.In(s.Loc).Format("Mon 2 Jan 2006"), s.Clock(s.Start), s.Clock(s.End), s.ZoneNote())
	}
	if f := strings.TrimSpace(in.Focus); f != "" {
		w("\nThe squadron asks: %s", f)
	}

	if m := in.Mission; m != nil {
		w("\n## Mission plan (mission file %s)", m.File)
		w("Map: %s. In-mission date %s, start %s.", m.Theatre, m.Date, m.StartTime)
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
				if i == 60 {
					fl = append(fl, fmt.Sprintf("... and %d more", len(m.Flights)-60))
					break
				}
				fl = append(fl, fmt.Sprintf("%s (%s, %dx %s, %s)", f.Group, f.Coalition, f.Slots, f.Aircraft, f.Task))
			}
			w("\n### Flights with player slots (%d)\n%s", len(m.Flights), strings.Join(fl, "; "))
		}
	}

	if s != nil {
		src := "the server's DCS event log"
		if !s.HasEvents {
			src = "Tacview only (no server events: kills not scored, shots matched by geometry)"
		}
		w("\n## What happened (from %s%s)", src, map[bool]string{true: " and Tacview", false: ""}[s.HasEvents && s.Rec != nil])
		w("\n### Pilots")
		w("pilot | side | aircraft | sorties | flight time | landings | traps | air kills | ground kills | shot down | crashes | ejections | shots | hits")
		for _, p := range s.Pilots {
			w("%s | %s | %s | %d | %s | %d | %d | %d | %d | %d | %d | %d | %d | %d", p.Name, p.Side, strings.Join(p.Aircraft, "/"),
				p.Sorties, hm(p.Flight), p.Landings, p.Traps, p.Kills, p.Ground, p.ShotDown, p.Crashes, p.Ejections, p.Shots, p.Hits)
		}
		for _, en := range s.Engagements {
			w("\n### Engagement %d: %s · Blue lost %d, Red lost %d", en.N, winLine(en), en.LostBlue, en.LostRed)
			if !en.Takeoff.IsZero() {
				w("Takeoff %s, first shot %s, last event %s. Blue %s v Red %s.", s.Clock(en.Takeoff), s.Clock(en.FirstShot), s.Clock(en.LastEvent),
					strings.Join(en.Blue, ", "), strings.Join(en.Red, ", "))
			} else {
				w("First shot %s, last event %s. Blue %s v Red %s.", s.Clock(en.FirstShot), s.Clock(en.LastEvent), strings.Join(en.Blue, ", "), strings.Join(en.Red, ", "))
			}
			w("time | shooter (side) | target | weapon | range nm | shooter alt ft / kt | result")
			for _, sh := range en.Shots {
				rng := "-"
				if sh.RangeNM >= 0 {
					rng = fmt.Sprintf("%.1f", sh.RangeNM)
				}
				w("%s | %s (%s) | %s | %s | %s | %s / %.0f | %s", s.Clock(sh.At)+sh.At.In(s.Loc).Format(":05"), sh.Shooter, sh.Side,
					firstNonEmpty(sh.Target, "-"), sh.Weapon, rng, thousands(sh.AltFt, 100), sh.Kt, sh.Result)
			}
			for _, k := range en.Kills {
				w("Kill %d: %s %s (%s) killed %s (%s, %s) with %s at %s ft", k.N, s.Clock(k.At), k.Killer, k.KillerSide, k.Victim,
					k.VictimSide, k.VictimType, firstNonEmpty(k.Weapon, "guns or unknown"), thousands(k.AltFt, 100))
			}
		}
		var ground []string
		for _, k := range s.Kills {
			if !k.Air {
				ground = append(ground, fmt.Sprintf("%s %s killed %s (%s) with %s", s.Clock(k.At), k.Killer, k.Victim, k.VictimType, firstNonEmpty(k.Weapon, "?")))
			}
		}
		if len(ground) > 0 {
			if len(ground) > 80 {
				ground = append(ground[:80], fmt.Sprintf("... and %d more", len(ground)-80))
			}
			w("\n### Ground and sea kills\n%s", strings.Join(ground, "\n"))
		}
		if len(s.Missiles) > 0 {
			w("\n### Weapons by side")
			for _, m := range s.Missiles {
				w("%s %s: fired %d, hit %d", m.Side, m.Weapon, m.Fired, m.Hit)
			}
		}
		if len(s.Safety) > 0 {
			w("\n### Safety")
			for _, it := range s.Safety {
				w("%s: %s", it.Kind, it.Text)
			}
		}
		if len(s.Traps) > 0 {
			w("\n### Carrier landings graded by the DCS LSO")
			for _, t := range s.Traps {
				w("%s %s at %s: %s", s.Clock(t.At), t.Pilot, t.Place, t.Raw)
			}
		}
	}

	if rad := in.Radio; rad != nil && len(rad.Lines) > 0 {
		w("\n## Readback radio log (%d lines; %d calls not understood)", len(rad.Lines), rad.Unheard)
		if rad.Truncated > 0 {
			w("(the first %d lines of the window are left out)", rad.Truncated)
		}
		w("PILOT = what speech recognition heard; ATC = what Readback said; REPORT = Readback's comms report.")
		loc := time.Local
		if s != nil {
			loc = s.Loc
		}
		for _, l := range rad.Lines {
			tag := map[string]string{"pilot": "PILOT", "atc": "ATC", "report": "REPORT", "unheard": "UNHEARD"}[l.Kind]
			w("%s [%s] %s: %s", l.At.In(loc).Format("15:04:05"), l.Station, tag, l.Text)
		}
	}

	if sv := in.Server; sv != nil && len(sv.Problems) > 0 {
		w("\n## DCS server errors (grouped, most frequent first)")
		for i, p := range sv.Problems {
			if i == 25 {
				break
			}
			w("- %dx %s %s: %s", p.Count, p.Level, p.Source, p.Text)
		}
	}
	return b.String()
}

func winLine(en *Engagement) string {
	if en.Winner == "Draw" {
		return "draw"
	}
	return en.Winner + " wins"
}

func hm(d time.Duration) string {
	if d <= 0 {
		return "0:00"
	}
	return fmt.Sprintf("%d:%02d", int(d.Hours()), int(d.Minutes())%60)
}

// ParseAI reads Claude's answer: the JSON object, even with text or code
// fences around it. With no usable JSON, the whole answer becomes the
// overview so nothing is lost.
func ParseAI(text string) AIText {
	var a AIText
	i, j := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if i >= 0 && j > i && json.Unmarshal([]byte(text[i:j+1]), &a) == nil {
		return a
	}
	return AIText{Overview: strings.TrimSpace(text)}
}
