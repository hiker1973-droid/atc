package debrief

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

// Report is everything one After Action Report page set is drawn from.
type Report struct {
	Session  *Session
	AI       *AIText // nil until Claude has written (the preview)
	Facts    []Fact
	ReportNo string
	Squadron string
	LogoURI  string // the squadron logo, embedded
	Mission  string
	Sources  []string
	Written  time.Time
	Model    string
	Radio    *Radio
}

// Greenie board squares: the squadron's colours (docs/paddles-report).
var boardColours = map[string][2]string{
	"_OK_": {"#00843d", "#ffffff"}, "OK": {"#2fb34a", "#ffffff"}, "(OK)": {"#f2c80f", "#1f3350"},
	"---": {"#8b5a2b", "#ffffff"}, "C": {"#c62828", "#ffffff"}, "B": {"#2f6db5", "#ffffff"},
	"WO": {"#222222", "#ffffff"}, "WOFD": {"#c8d0db", "#1f3350"},
}

func esc(s string) string { return html.EscapeString(s) }

// HTML is the whole report as one self-contained page (landscape Letter
// when printed): the summary, a page per engagement, the pilots and the
// greenie board, then the AAR.
func (r *Report) HTML() string {
	s := r.Session
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>` + esc(r.ReportNo+" "+r.Mission) + `</title><style>` + reportCSS + `</style></head><body>`)
	b.WriteString(r.summaryPage())
	for _, en := range s.Engagements {
		b.WriteString(r.engagementPage(en))
	}
	b.WriteString(r.pilotsPage())
	b.WriteString(r.aarPage())
	b.WriteString(`</body></html>`)
	return b.String()
}

func (r *Report) pending(what string) string {
	return `<p class="pending">` + esc(what) + ` - written by Claude when you press Write the AAR.</p>`
}

func (r *Report) header(title, sub string) string {
	logo := ""
	if r.LogoURI != "" {
		logo = `<img class="logo" src="` + r.LogoURI + `" alt="">`
	}
	return fmt.Sprintf(`<div class="hd">%s<div class="hdt"><h1>%s</h1><div class="sub">%s</div></div><div class="no"><b>%s</b>AFTER ACTION REPORT</div></div>`,
		logo, esc(title), sub, esc(r.ReportNo))
}

func (r *Report) sourcesLine() string {
	s := r.Session
	src := "from the server dcs.log and Tacview"
	switch {
	case !s.HasEvents && s.Rec != nil:
		src = "from Tacview (no server dcs.log: kills not scored)"
	case s.HasEvents && s.Rec == nil:
		src = "from the server dcs.log (no Tacview: no maps)"
	}
	if r.Radio != nil && len(r.Radio.Lines) > 0 {
		src += " and Readback's radio logs"
	}
	return esc(r.Squadron) + " · " + src + " · " + esc(s.ZoneNote())
}

func (r *Report) summaryPage() string {
	s := r.Session
	date := s.Start.In(s.Loc).Format("2 Jan 2006")
	var b strings.Builder
	b.WriteString(`<section class="page">`)
	b.WriteString(r.header(r.Mission+" · "+date, r.sourcesLine()))

	// The headline.
	head := ""
	switch {
	case s.PvP && len(s.Engagements) > 0:
		bw, rw := 0, 0
		for _, en := range s.Engagements {
			switch en.Winner {
			case "Blue":
				bw++
			case "Red":
				rw++
			}
		}
		bn, rn := "", ""
		if s.BlueName != "" {
			bn = " (" + esc(s.BlueName) + ")"
		}
		if s.RedName != "" {
			rn = " (" + esc(s.RedName) + ")"
		}
		head = fmt.Sprintf(`<span class="b">Blue%s</span> %d &nbsp;·&nbsp; <span class="r">Red%s</span> %d`, bn, bw, rn, rw)
	case s.HasEvents:
		air, down, sorties := 0, 0, 0
		for _, p := range s.Pilots {
			air += p.Kills + p.Ground
			down += p.ShotDown
			sorties += p.Sorties
		}
		head = fmt.Sprintf(`%d sorties · %d kills · %d lost`, sorties, air, down)
	default:
		sorties := 0
		for _, p := range s.Pilots {
			sorties += p.Sorties
		}
		head = fmt.Sprintf(`%d pilots · %d sorties`, len(s.Pilots), sorties)
	}
	b.WriteString(`<div class="big">` + head + `</div>`)

	b.WriteString(`<div class="grid"><div class="left">`)
	nk := 0
	for _, k := range s.Kills {
		if k.Air {
			nk++
		}
	}
	if mp := s.OverviewMap(600, 330); mp != "" {
		cap := "EVERY PLAYER SORTIE · the fields"
		if nk > 0 {
			cap = fmt.Sprintf("ALL %d AIR KILLS · ✕ in the killer's colour · every player sortie · the fields", nk)
		}
		b.WriteString(`<div class="cap">` + cap + `</div>` + mp)
	}
	title := "What won it"
	if !s.PvP || len(s.Engagements) == 0 {
		title = "Key points"
	}
	b.WriteString(`<h3>` + title + `</h3>`)
	if r.AI == nil {
		b.WriteString(r.pending("The points that decided the session"))
	} else {
		if r.AI.BLUF != "" {
			b.WriteString(`<p class="bluf"><b>BLUF:</b> ` + esc(r.AI.BLUF) + `</p>`)
		}
		b.WriteString(`<ol class="won">`)
		for _, p := range r.AI.WhatWon {
			b.WriteString(`<li><b>` + esc(p.Title) + `</b> ` + esc(p.Text) + `</li>`)
		}
		b.WriteString(`</ol>`)
	}
	b.WriteString(`</div><div class="right">`)

	if len(s.Engagements) > 0 {
		b.WriteString(`<table class="res"><tr><th>Engagement</th><th>Teams</th><th class="n">Lost B / R</th><th>Winner</th></tr>`)
		for _, en := range s.Engagements {
			cls := map[string]string{"Blue": "b", "Red": "r"}[en.Winner]
			fmt.Fprintf(&b, `<tr><td>%d · %s</td><td>%d v %d</td><td class="n">%d / %d</td><td class="%s">%s</td></tr>`,
				en.N, s.Clock(en.FirstShot), len(en.Blue), len(en.Red), en.LostBlue, en.LostRed, cls, en.Winner)
		}
		b.WriteString(`</table>`)
	}
	if s.HasEvents {
		b.WriteString(`<table class="res"><tr><th>Pilot</th><th class="n">Kills</th><th class="n">Died</th><th class="n">Sorties</th></tr>`)
		for i, p := range s.Pilots {
			if i == 14 {
				fmt.Fprintf(&b, `<tr><td colspan="4" class="muted">and %d more on the pilots page</td></tr>`, len(s.Pilots)-14)
				break
			}
			fmt.Fprintf(&b, `<tr><td class="%s">%s</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td></tr>`,
				p.Side[:min(1, len(p.Side))], esc(p.Name), p.Kills+p.Ground, p.ShotDown, p.Sorties)
		}
		b.WriteString(`</table>`)
	}
	if len(s.Missiles) > 0 {
		b.WriteString(`<table class="res"><tr><th>Side</th><th>Weapon</th><th class="n">Fired</th><th class="n">Hit</th></tr>`)
		for _, m := range s.Missiles {
			fmt.Fprintf(&b, `<tr><td class="%s">%s</td><td>%s</td><td class="n">%d</td><td class="n">%d</td></tr>`,
				m.Side[:min(1, len(m.Side))], capitalize(m.Side), esc(m.Weapon), m.Fired, m.Hit)
		}
		b.WriteString(`</table><p class="note">Counted by the side the jet flew for. Blue-on-blue and red-on-red shots are under Safety, not here.</p>`)
	}
	b.WriteString(`<h3>Safety</h3>`)
	if len(s.Safety) == 0 {
		b.WriteString(`<p class="note">No blue-on-blue shots, ground impacts or ejections recorded.</p>`)
	} else {
		b.WriteString(`<p class="note">`)
		for i, it := range s.Safety {
			if i > 0 {
				b.WriteString(`<br>`)
			}
			b.WriteString(`<b>` + esc(it.Kind) + `:</b> ` + esc(it.Text))
		}
		b.WriteString(`</p>`)
		if r.AI != nil && r.AI.Safety != "" {
			b.WriteString(`<p class="note">` + esc(r.AI.Safety) + `</p>`)
		}
	}
	b.WriteString(`</div></div></section>`)
	return b.String()
}

func (r *Report) engagementPage(en *Engagement) string {
	s := r.Session
	var b strings.Builder
	win := en.Winner + " wins"
	if en.Winner == "Draw" {
		win = "Draw"
	}
	b.WriteString(`<section class="page">`)
	sub := ""
	if !en.Takeoff.IsZero() {
		sub = "Takeoff " + s.Clock(en.Takeoff) + " · "
	}
	last := en.LastEvent
	if n := len(en.Kills); n > 0 {
		last = en.Kills[n-1].At
		sub += "first shot " + s.Clock(en.FirstShot) + " · last kill " + s.Clock(last)
	} else {
		sub += "first shot " + s.Clock(en.FirstShot) + " · last shot " + s.Clock(last)
	}
	sub += ` · <span class="b">Blue</span> ` + esc(strings.Join(en.Blue, ", ")) + ` &nbsp;v&nbsp; <span class="r">Red</span> ` + esc(strings.Join(en.Red, ", "))
	b.WriteString(r.header(fmt.Sprintf("Engagement %d: %s · Blue lost %d, Red lost %d", en.N, win, en.LostBlue, en.LostRed), sub))
	b.WriteString(`<div class="grid"><div class="left">`)
	fight, legend := s.FightMap(en, 600, 500)
	if fight != "" {
		b.WriteString(`<div class="cap">THE FIGHT · tracks from about a minute before the first shot · dashed = missiles (○ = launch) · ✕ = kill, in the killer's colour · start altitude next to each name</div>`)
		b.WriteString(fight)
		b.WriteString(`<div class="legend">`)
		for _, l := range legend {
			b.WriteString(`<span class="sw" style="background:` + l.Colour + `"></span>` + esc(l.Name))
		}
		b.WriteString(`</div>`)
	}
	if r.AI != nil && en.N-1 < len(r.AI.EngagementNotes) {
		b.WriteString(`<p class="note" style="width:600px">` + esc(r.AI.EngagementNotes[en.N-1]) + `</p>`)
	} else if r.AI == nil {
		b.WriteString(r.pending("How the engagement went"))
	}
	b.WriteString(`</div><div class="right">`)
	if routes := s.RoutesMap(en, 362, 250); routes != "" {
		b.WriteString(`<div class="cap">ROUTES IN · the whole engagement from each field</div>` + routes)
	}
	b.WriteString(`<div class="cap" style="margin-top:8px">EVERY SHOT · range nm · shooter alt / kt at launch</div>`)
	b.WriteString(`<table class="shots"><tr><th>Time</th><th>Shooter</th><th>Target</th><th>Wpn</th><th class="n">nm</th><th class="n">alt / kt</th><th>Result</th></tr>`)
	for _, sh := range en.Shots {
		rng, ak := "-", "-"
		if sh.RangeNM >= 0 {
			rng = fmt.Sprintf("%.1f", sh.RangeNM)
		}
		if sh.AltFt > 0 {
			ak = fmt.Sprintf("%.0fk/%.0f", sh.AltFt/1000, sh.Kt)
		}
		res := esc(sh.Result)
		if sh.KillN > 0 {
			res = "<b>" + res + "</b>"
		}
		fmt.Fprintf(&b, `<tr><td>%s</td><td style="color:%s;font-weight:600">%s</td><td>%s</td><td>%s</td><td class="n">%s</td><td class="n">%s</td><td>%s</td></tr>`,
			s.Clock(sh.At), sideColour[sh.Side], esc(sh.Shooter), esc(firstNonEmpty(sh.Target, "-")), esc(strings.TrimPrefix(sh.Weapon, "AIM-")), rng, ak, res)
	}
	b.WriteString(`</table></div></div></section>`)
	return b.String()
}

func (r *Report) pilotsPage() string {
	s := r.Session
	if len(s.Pilots) == 0 && len(s.Traps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("Pilots and carrier", r.sourcesLine()))
	if len(s.Pilots) > 0 {
		b.WriteString(`<div class="cap">EVERY PLAYER · sorties are takeoffs · flight time from takeoff to landing, crash, ejection or leaving the jet</div>`)
		b.WriteString(`<table class="res wide"><tr><th>Pilot</th><th>Side</th><th>Aircraft</th><th class="n">Sorties</th><th class="n">Flight</th><th class="n">Landings</th><th class="n">Traps</th><th class="n">Air kills</th><th class="n">Ground</th><th class="n">Shot down</th><th class="n">Crashes</th><th class="n">Ejections</th><th class="n">Shots</th><th class="n">Hits</th></tr>`)
		ps := append([]*PilotStat(nil), s.Pilots...)
		sort.SliceStable(ps, func(i, j int) bool {
			if ps[i].Side != ps[j].Side {
				return ps[i].Side < ps[j].Side
			}
			return ps[i].Name < ps[j].Name
		})
		for _, p := range ps {
			fmt.Fprintf(&b, `<tr><td class="%s"><b>%s</b></td><td>%s</td><td>%s</td><td class="n">%d</td><td class="n">%s</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td></tr>`,
				p.Side[:min(1, len(p.Side))], esc(p.Name), capitalize(p.Side), esc(strings.Join(p.Aircraft, ", ")), p.Sorties, hm(p.Flight),
				p.Landings, p.Traps, p.Kills, p.Ground, p.ShotDown, p.Crashes, p.Ejections, p.Shots, p.Hits)
		}
		b.WriteString(`</table>`)
	}
	if len(s.Traps) > 0 {
		b.WriteString(`<h3>Greenie board · this session</h3><table class="res"><tr><th>Pilot</th><th class="n">Avg</th><th class="n">Passes</th><th>Passes in order</th></tr>`)
		by := map[string][]Trap{}
		var names []string
		for _, t := range s.Traps {
			if by[t.Pilot] == nil {
				names = append(names, t.Pilot)
			}
			by[t.Pilot] = append(by[t.Pilot], t)
		}
		sort.Strings(names)
		for _, n := range names {
			var pts []float64
			var sq strings.Builder
			for _, t := range by[n] {
				if p, ok := GradePoints(t.Grade.Grade); ok {
					pts = append(pts, p)
				}
				c := boardColours[t.Grade.Grade]
				label := ""
				if t.Wire > 0 {
					label = fmt.Sprint(t.Wire)
				}
				fmt.Fprintf(&sq, `<span class="sq" style="background:%s;color:%s" title="%s %s">%s</span>`, c[0], c[1], esc(s.Clock(t.At)), esc(t.Raw), label)
			}
			avg := "-"
			if len(pts) > 0 {
				sum := 0.0
				for _, p := range pts {
					sum += p
				}
				avg = fmt.Sprintf("%.2f", sum/float64(len(pts)))
			}
			fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td class="n">%s</td><td class="n">%d</td><td>%s</td></tr>`, esc(n), avg, len(by[n]), sq.String())
		}
		b.WriteString(`</table><p class="note">`)
		for _, g := range []string{"_OK_", "OK", "(OK)", "---", "B", "WO", "C", "WOFD"} {
			c := boardColours[g]
			fmt.Fprintf(&b, `<span class="sq" style="background:%s;color:%s">&nbsp;</span>%s &nbsp; `, c[0], c[1], esc(map[string]string{"WOFD": "WO foul deck (not scored)"}[g]+map[bool]string{true: "", false: g}[g == "WOFD"]))
		}
		b.WriteString(`· the number is the wire · grades from DCS's LSO</p>`)
	}
	b.WriteString(`</section>`)
	return b.String()
}

func (r *Report) aarPage() string {
	s := r.Session
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("After Action Report · "+r.Mission, r.sourcesLine()))
	b.WriteString(`<div class="cols"><div>`)
	b.WriteString(`<table class="data">`)
	for _, f := range r.Facts {
		b.WriteString(`<tr><th>` + esc(f.Label) + `</th><td>` + esc(f.Value) + `</td></tr>`)
	}
	b.WriteString(`</table>`)
	sec := func(title, text string) {
		b.WriteString(`<h2>` + esc(title) + `</h2>`)
		if r.AI == nil {
			b.WriteString(r.pending(title))
		} else if strings.TrimSpace(text) == "" {
			b.WriteString(`<p class="muted">Nothing recorded.</p>`)
		} else {
			b.WriteString(`<p>` + esc(text) + `</p>`)
		}
	}
	bluf := ""
	if r.AI != nil {
		bluf = r.AI.BLUF
	}
	if bluf != "" {
		b.WriteString(`<p class="bluf"><b>BLUF:</b> ` + esc(bluf) + `</p>`)
	}
	ov, pl := "", ""
	if r.AI != nil {
		ov, pl = r.AI.Overview, r.AI.Planned
	}
	sec("1. Mission overview", ov)
	sec("2. What was planned", pl)
	b.WriteString(`<h2>3-5. What happened, results, flights</h2><p>`)
	if len(s.Engagements) > 0 {
		fmt.Fprintf(&b, `%d engagement%s: see the engagement pages for every shot, the tracks and the kills, and the summary page for the results.`, len(s.Engagements), map[bool]string{true: "", false: "s"}[len(s.Engagements) == 1])
	} else {
		b.WriteString(`No air-to-air engagements were recorded.`)
	}
	b.WriteString(` The pilots page has every player's sorties, flight time and results.</p>`)
	b.WriteString(`</div><div>`)
	b.WriteString(`<h2>6. Analysis · air traffic control and comms</h2>`)
	if r.Radio != nil && len(r.Radio.Lines) > 0 {
		calls, tx := 0, 0
		for _, l := range r.Radio.Lines {
			switch l.Kind {
			case "pilot":
				calls++
			case "atc":
				tx++
			}
		}
		fmt.Fprintf(&b, `<p class="note">Readback heard %d pilot calls and made %d transmissions; %d calls were not understood.</p>`, calls, tx, r.Radio.Unheard)
	}
	if r.AI == nil {
		b.WriteString(r.pending("Air traffic control and comms"))
	} else if r.AI.Comms != "" {
		b.WriteString(`<p>` + esc(r.AI.Comms) + `</p>`)
	} else {
		b.WriteString(`<p class="muted">No radio records.</p>`)
	}
	if r.AI != nil && r.AI.Server != "" {
		b.WriteString(`<p><b>Server:</b> ` + esc(r.AI.Server) + `</p>`)
	}
	b.WriteString(`<h2>7. Sustain / Improve</h2>`)
	if r.AI == nil {
		b.WriteString(r.pending("Sustain and improve"))
	} else {
		b.WriteString(`<div class="si"><div><b>Sustain</b><ul>`)
		for _, x := range r.AI.Sustain {
			b.WriteString(`<li>` + esc(x) + `</li>`)
		}
		b.WriteString(`</ul></div><div><b>Improve</b><ul>`)
		for _, x := range r.AI.Improve {
			b.WriteString(`<li>` + esc(x) + `</li>`)
		}
		b.WriteString(`</ul></div></div>`)
	}
	b.WriteString(`<h2>8. Recommendations and follow-up actions</h2>`)
	if r.AI == nil {
		b.WriteString(r.pending("Recommendations"))
	} else {
		b.WriteString(`<ol>`)
		for _, a := range r.AI.Recommendations {
			owner := ""
			if a.Owner != "" {
				owner = ` <span class="owner">[` + esc(a.Owner) + `]</span>`
			}
			b.WriteString(`<li>` + esc(a.Text) + owner + `</li>`)
		}
		b.WriteString(`</ol>`)
	}
	b.WriteString(`<div class="sign"><div>Prepared by (name, callsign) / date</div><div>Reviewed and approved (Commanding Officer) / date</div></div>`)
	b.WriteString(`</div></div>`)
	written := ""
	if !r.Written.IsZero() {
		written = " on " + r.Written.In(s.Loc).Format("2 Jan 2006 15:04")
	}
	model := ""
	if r.Model != "" {
		model = " by " + esc(r.Model)
	}
	b.WriteString(`<p class="sources">Prepared with Readback ATC` + esc(written) + model + ` from ` + esc(strings.Join(r.Sources, "; ")) +
		`. Shots, kills, sorties and landings are as DCS logged them on the server; ranges, altitudes and tracks come from Tacview; pilot radio calls are as speech recognition heard them.</p>`)
	b.WriteString(`</section>`)
	return b.String()
}

const reportCSS = `
@page { size: 11in 8.5in; margin: 0.4in; }
* { box-sizing: border-box; }
html { background: #e9ebee; }
body { font-family: "Segoe UI", Arial, sans-serif; color: #1c232b; margin: 0; font-size: 9.5pt; line-height: 1.32; }
.page { width: 10.2in; height: 7.68in; overflow: hidden; margin: 14px auto; padding: 0.15in 0.2in; background: #fff; box-shadow: 0 2px 12px rgba(0,0,0,.16); position: relative; }
.page.flow { height: auto; min-height: 7.68in; overflow: visible; }
.hd { display: flex; align-items: center; gap: 12px; margin-bottom: 2px; }
.hd .logo { height: 44px; max-width: 160px; object-fit: contain; }
.hdt { flex: 1; min-width: 0; }
.hd .no { text-align: right; font-size: 7.5pt; letter-spacing: .08em; color: #55606b; white-space: nowrap; }
.hd .no b { display: block; font-size: 10pt; letter-spacing: 0; color: #1c232b; }
h1 { font-size: 16pt; margin: 0; } h2 { font-size: 10.5pt; text-transform: uppercase; letter-spacing: .04em; margin: 10px 0 3px; border-bottom: 1.4px solid #1c232b; padding-bottom: 1px; }
h3 { font-size: 11pt; margin: 8px 0 3px; }
.sub { color: #55606b; font-size: 9pt; }
.big { font-size: 22pt; font-weight: 700; margin: 2px 0 6px; }
.grid { display: flex; gap: 14px; } .left { flex: 0 0 600px; width: 600px; } .right { flex: 1 1 auto; min-width: 0; }
.cols { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; }
.cap { font-size: 7.5pt; font-weight: 700; letter-spacing: .4px; color: #55606b; margin-bottom: 3px; }
svg.map, svg.fmap { display: block; border: 1px solid #b8c0c8; width: 100%; height: auto; }
.left svg.fmap { width: 600px; height: 500px; }
.b { color: #1f5fa8; font-weight: 700; } .r { color: #c0392b; font-weight: 700; } .muted { color: #7a808a; }
table { border-collapse: collapse; width: 100%; margin-bottom: 7px; }
th { text-align: left; font-size: 7.5pt; color: #55606b; border-bottom: 1.4px solid #333; padding: 2px 4px; }
td { border-bottom: 1px solid #dde2e6; padding: 2px 4px; font-size: 8.5pt; vertical-align: top; }
table.shots td { font-size: 7.6pt; padding: 1.5px 3px; white-space: nowrap; } table.shots th { padding: 1.5px 3px; }
td.n, th.n { text-align: right; font-variant-numeric: tabular-nums; }
table.data th { width: 1.25in; background: #f1f2f4; border: 1px solid #d6d9de; font-size: 8pt; color: #3a3f47; }
table.data td { border: 1px solid #d6d9de; }
table.wide td { white-space: nowrap; }
.note { font-size: 8.5pt; margin: 4px 0; } ol.won { margin: 2px 0; padding-left: 18px; } ol.won li { margin-bottom: 2px; }
.bluf { margin: 4px 0 6px; }
.pending { color: #8a6d1f; background: #fff8e1; border: 1px dashed #e0c36a; padding: 4px 8px; font-size: 8.5pt; margin: 4px 0; }
.legend { font-size: 8.5pt; margin-top: 4px; }
.sw { display: inline-block; width: 18px; height: 4px; margin: 0 4px 2px 10px; vertical-align: middle; }
.sq { display: inline-block; width: 18px; height: 18px; line-height: 18px; text-align: center; font-size: 8pt; font-weight: 700; margin: 1px; border-radius: 2px; vertical-align: middle; }
.si { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; } .si ul { margin: 2px 0; padding-left: 16px; }
ol { padding-left: 18px; margin: 3px 0; } li { margin-bottom: 2px; }
.owner { color: #55606b; font-weight: 600; white-space: nowrap; }
.sign { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-top: 22px; font-size: 8pt; color: #3a3f47; }
.sign div { border-top: 1px solid #1c232b; padding-top: 3px; }
.sources { margin-top: 12px; font-size: 7.5pt; color: #7a808a; border-top: 1px solid #d6d9de; padding-top: 4px; }
.grat { font: 7.5px "Segoe UI", Arial; fill: #5c7184; }
.lab { font: 600 10.5px "Segoe UI", Arial; fill: #1c232b; paint-order: stroke; stroke: #fff; stroke-width: 2.6px; stroke-linejoin: round; }
.lab.blue { fill: #17477f; } .lab.red { fill: #9c2a1e; }
.kn { font: 700 10px "Segoe UI", Arial; fill: #fff; }
@media print {
  html { background: #fff; }
  .page { margin: 0; padding: 0; box-shadow: none; width: auto; page-break-after: always; }
  .page:last-child { page-break-after: auto; }
  .page.flow { min-height: 0; }
  tr, li { break-inside: avoid; }
  .sq, table.data th, .pending { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
}
`

// Discord is the post that goes with the AAR on the squadron's Discord:
// Claude's, or one made from the numbers. Kept under Discord's 2,000.
func (r *Report) Discord() string {
	if r.AI != nil && strings.TrimSpace(r.AI.Discord) != "" {
		return clipRunes(strings.TrimSpace(r.AI.Discord), 1900)
	}
	s := r.Session
	var b strings.Builder
	fmt.Fprintf(&b, "**%s · %s · %s**\n", r.ReportNo, r.Mission, s.Start.In(s.Loc).Format("2 Jan 2006"))
	if s.PvP && len(s.Engagements) > 0 {
		bw, rw := 0, 0
		for _, en := range s.Engagements {
			switch en.Winner {
			case "Blue":
				bw++
			case "Red":
				rw++
			}
		}
		fmt.Fprintf(&b, "Blue %s %d · Red %s %d\n", s.BlueName, bw, s.RedName, rw)
	}
	sorties := 0
	for _, p := range s.Pilots {
		sorties += p.Sorties
	}
	fmt.Fprintf(&b, "- %d pilots, %d sorties, %d engagements\n", len(s.Pilots), sorties, len(s.Engagements))
	n := 0
	for _, p := range s.Pilots {
		if p.Kills+p.Ground == 0 || n == 3 {
			continue
		}
		fmt.Fprintf(&b, "- %s: %d kills\n", p.Name, p.Kills+p.Ground)
		n++
	}
	if len(s.Safety) > 0 {
		fmt.Fprintf(&b, "- Safety: %d items to review\n", len(s.Safety))
	}
	b.WriteString("Full AAR attached.")
	return b.String()
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
