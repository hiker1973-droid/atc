package debrief

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// The optional pages (extras.go): drawn from Report.X, each left out (and
// said so on the AAR page) when the session has nothing for it.

// opts is what the report shows; a report made before the options had the
// summary, engagements, pilots, the write-up and the Discord post.
func (r *Report) opts() Options {
	if r.Opt == nil {
		return Options{Summary: true, Engagements: true, Pilots: true, Written: true, Discord: true}
	}
	return *r.Opt
}

func (r *Report) skip(format string, a ...any) string {
	r.skipped = append(r.skipped, fmt.Sprintf(format, a...))
	return ""
}

func (r *Report) when(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return r.Session.Clock(t)
}

func mins(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%d min", int(d.Round(time.Minute).Minutes()))
}

func square(s *Session, t Trap) string {
	c := boardColours[t.Grade.Grade]
	label := ""
	if t.Wire > 0 {
		label = fmt.Sprint(t.Wire)
	}
	return fmt.Sprintf(`<span class="sq" style="background:%s;color:%s" title="%s %s">%s</span>`, c[0], c[1], esc(s.Clock(t.At)), esc(t.Raw), label)
}

func status(st string) string {
	if st == "" {
		return `<span class="muted">-</span>`
	}
	return `<span class="st ` + st + `">` + map[string]string{"current": "Current", "due": "Due", "lapsed": "Lapsed"}[st] + `</span>`
}

func pctBar(n, of int) string {
	if of == 0 {
		return "-"
	}
	p := 100 * n / of
	return fmt.Sprintf(`<span class="bar"><i style="width:%d%%"></i></span> %d%%`, p, p)
}

// plural: "passes" for one is "pass", "sorties" "sortie", "traps" "trap".
func plural(n int, s string) string {
	switch {
	case n != 1:
		return s
	case strings.HasSuffix(s, "sses"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "ies"):
		return s[:len(s)-1]
	case strings.HasSuffix(s, "s"):
		return s[:len(s)-1]
	}
	return s
}

func nmOrDash(v float64) string {
	if v < 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", v)
}

// ---- 1. Comms score ----

func (r *Report) commsPage() string {
	x := r.X
	if x == nil || len(x.Comms) == 0 {
		if r.Radio == nil || len(r.Radio.Lines) == 0 {
			return r.skip("Comms score: no Readback radio logs for the session.")
		}
		return r.skip("Comms score: no calls with a callsign Readback could hear.")
	}
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("Comms score", "every aircraft heard on Readback's frequencies · "+esc(r.Session.ZoneNote())))
	b.WriteString(`<div class="cap">SCORE OUT OF 100 · calls answered (60%) · handoffs followed and readbacks given when asked for (40%) · 5 off for each "say again" and comms-report flag (3 for a changed callsign)</div>`)
	b.WriteString(`<table class="res wide"><tr><th>Callsign</th><th>Pilot</th><th class="n">Score</th><th>Grade</th><th class="n">Last report</th><th class="n">Calls</th><th>Answered</th><th>Handoffs, readbacks</th><th class="n">Say again</th><th>Flags</th><th>Heard on</th></tr>`)
	for _, c := range x.Comms {
		prev := `<span class="muted">-</span>`
		if c.Prev >= 0 {
			d := c.Score - c.Prev
			cls := map[bool]string{true: "up", false: "down"}[d >= 0]
			prev = fmt.Sprintf(`%d <span class="%s">%s%d</span>`, c.Prev, cls, map[bool]string{true: "▲ +", false: "▼ "}[d >= 0], d)
		}
		rb := `<span class="muted">none given</span>`
		if c.Asked > 0 {
			rb = fmt.Sprintf("%d / %d", c.Done, c.Asked)
		}
		fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td>%s</td><td class="n"><b>%d</b></td><td class="g%s">%s</td><td class="n">%s</td><td class="n">%d</td><td>%s</td><td>%s</td><td class="n">%d</td><td>%s</td><td>%s</td></tr>`,
			esc(c.Callsign), esc(firstNonEmpty(c.Pilot, "-")), c.Score, strings.ToLower(strings.Fields(c.Grade())[0]), c.Grade(), prev, c.Calls,
			pctBar(c.Answered, c.Calls), rb, c.SayAgain, esc(firstNonEmpty(strings.Join(c.Flags, ", "), "-")), esc(strings.Join(c.Stations, ", ")))
	}
	b.WriteString(`</table>`)
	var notes []string
	for _, c := range x.Comms {
		for _, n := range c.Notes {
			notes = append(notes, `<b>`+esc(c.Callsign)+`</b> `+esc(n))
		}
	}
	if len(notes) > 0 {
		if len(notes) > 24 {
			notes = append(notes[:24], fmt.Sprintf("and %d more", len(notes)-24))
		}
		b.WriteString(`<h3>What cost points</h3><ul class="tight">`)
		for _, n := range notes {
			b.WriteString(`<li>` + n + `</li>`)
		}
		b.WriteString(`</ul>`)
	}
	unheard := ""
	if x.Unheard > 0 {
		unheard = fmt.Sprintf(" %d call%s could not be understood at all and count against no one.", x.Unheard, map[bool]string{true: "", false: "s"}[x.Unheard == 1])
	}
	b.WriteString(`<p class="note">Pilot calls are as speech recognition heard them, so a garbled call can be the recognition rather than the pilot; a modex heard two ways ("Raider 302", "reader 302") is one aircraft.` + unheard + `</p>`)
	b.WriteString(`</section>`)
	return b.String()
}

// ---- 5. Losses ----

func (r *Report) lossesPage() string {
	x := r.X
	if x == nil || len(x.Losses) == 0 {
		if !r.Session.HasEvents {
			return r.skip("Losses: needs the DCS server's dcs.log.")
		}
		return r.skip("Losses: no player aircraft were lost.")
	}
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("Losses", r.sourcesLine()))
	by := map[string]int{}
	for _, l := range x.Losses {
		by[l.Cause]++
	}
	b.WriteString(`<div class="chips">`)
	for _, c := range []string{"Shot down", "Crashed", "Ejected", "Pilot killed"} {
		if by[c] > 0 {
			fmt.Fprintf(&b, `<span class="chip"><b>%d</b> %s</span>`, by[c], strings.ToLower(c))
		}
	}
	b.WriteString(`</div><table class="res"><tr><th>Time</th><th>Pilot</th><th>Aircraft</th><th>Cause</th><th>How</th><th>Ejected</th><th>CSAR</th></tr>`)
	yes := func(v bool) string { return map[bool]string{true: "yes", false: "-"}[v] }
	for _, l := range x.Losses {
		ej := yes(l.Ejected)
		if l.Died && !l.Ejected {
			ej = "killed"
		}
		fmt.Fprintf(&b, `<tr><td>%s</td><td class="%s"><b>%s</b></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			r.when(l.At), l.Side[:min(1, len(l.Side))], esc(l.Pilot), esc(l.Aircraft), esc(l.Cause), esc(l.Detail), ej, yes(l.CSAR))
	}
	b.WriteString(`</table><p class="note">Shot down: DCS's kill, with the shot's range and altitudes from Tacview. Crashed or ejected without a kill: from the server's events, with the last altitude and descent from Tacview. CSAR: Readback called a CSAR launch within ten minutes.</p>`)
	b.WriteString(`</section>`)
	return b.String()
}

// ---- 4. Weapon effectiveness ----

func (r *Report) weaponsPage() string {
	x := r.X
	if x == nil || (len(x.Weapons) == 0 && len(x.Ground) == 0) {
		return r.skip("Weapon effectiveness: no player shots recorded.")
	}
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("Weapon effectiveness", r.sourcesLine()))
	if len(x.Weapons) > 0 {
		b.WriteString(`<div class="cap">EVERY PLAYER SHOT BY WEAPON · ranges and altitudes at launch, from Tacview</div>`)
		b.WriteString(`<table class="res"><tr><th>Side</th><th>Weapon</th><th class="n">Fired</th><th class="n">Hits</th><th>Hit rate</th><th class="n">Kills</th><th class="n">Avg range, hits (nm)</th><th class="n">Avg range, misses (nm)</th><th class="n">Avg launch alt (k ft)</th></tr>`)
		for _, w := range x.Weapons {
			alt := "-"
			if w.AltK >= 0 {
				alt = fmt.Sprintf("%.0f", w.AltK)
			}
			fmt.Fprintf(&b, `<tr><td class="%s">%s</td><td><b>%s</b></td><td class="n">%d</td><td class="n">%d</td><td>%s</td><td class="n">%d</td><td class="n">%s</td><td class="n">%s</td><td class="n">%s</td></tr>`,
				w.Side[:min(1, len(w.Side))], capitalize(w.Side), esc(w.Weapon), w.Fired, w.Hits, pctBar(w.Hits, w.Fired), w.Kill, nmOrDash(w.RangeHit), nmOrDash(w.RangeMiss), alt)
		}
		b.WriteString(`</table>`)
	}
	if len(x.Ground) > 0 {
		b.WriteString(`<h3>Ground and sea targets destroyed</h3><table class="res"><tr><th>Weapon</th><th class="n">Kills</th><th>Targets</th></tr>`)
		for _, g := range x.Ground {
			fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td class="n">%d</td><td>%s</td></tr>`, esc(g.Weapon), g.Kills, esc(g.Targets))
		}
		b.WriteString(`</table>`)
	}
	ff := "No blue-on-blue or red-on-red shots."
	if x.FriendlyShots > 0 {
		ff = fmt.Sprintf("%d shot%s at a friendly aircraft %s left out of these numbers: see Safety on the summary page.", x.FriendlyShots,
			map[bool]string{true: "", false: "s"}[x.FriendlyShots == 1], map[bool]string{true: "is", false: "are"}[x.FriendlyShots == 1])
	}
	src := "Hits and kills are as DCS scored them on the server."
	if !r.Session.HasEvents {
		src = "No server dcs.log: hits are matched from Tacview (a missile ending next to a jet that then left the recording)."
	}
	b.WriteString(`<p class="note">` + esc(ff) + " " + esc(src) + ` Guns are counted only where DCS logged a shot.</p></section>`)
	return b.String()
}

// ---- 6. Carrier recovery ----

func (r *Report) recoveryPage() string {
	x := r.X
	s := r.Session
	if x == nil || len(x.Recoveries) == 0 {
		if !s.HasEvents {
			return r.skip("Carrier recovery: needs the DCS server's dcs.log (the LSO grades are in it).")
		}
		return r.skip("Carrier recovery: no carrier passes in the session.")
	}
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("Carrier recovery", "Marshal from Readback's radio logs · passes and grades from DCS's LSO · "+esc(s.ZoneNote())))
	for _, rc := range x.Recoveries {
		span := r.when(rc.Start)
		if d := rc.End.Sub(rc.Start); d >= time.Minute {
			span = fmt.Sprintf("%s to %s (%s)", r.when(rc.Start), r.when(rc.End), mins(d))
		}
		fmt.Fprintf(&b, `<h3>%s · %s</h3><div class="chips">`, esc(rc.Place), span)
		scored := rc.Passes - rc.FoulDeck
		chip := func(n int, what string) {
			fmt.Fprintf(&b, `<span class="chip"><b>%d</b> %s</span>`, n, plural(n, what))
		}
		chip(rc.Passes, "passes")
		chip(rc.Traps, "traps")
		chip(rc.Bolters, "bolters")
		chip(rc.WaveOffs, "wave-offs")
		if rc.FoulDeck > 0 {
			chip(rc.FoulDeck, "foul-deck wave-offs")
		}
		if scored > 0 {
			fmt.Fprintf(&b, `<span class="chip">boarding rate <b>%d%%</b></span>`, 100*rc.Traps/scored)
		}
		if rc.Interval > 0 {
			fmt.Fprintf(&b, `<span class="chip">%s between passes</span>`, mins(rc.Interval))
		}
		if rc.AvgPoints >= 0 {
			fmt.Fprintf(&b, `<span class="chip">average grade <b>%.2f</b></span>`, rc.AvgPoints)
		}
		b.WriteString(`</div><table class="res wide"><tr><th>Pilot</th><th>Callsign</th><th>Checked in</th><th>Charlie</th><th>Commencing</th><th>Initial</th><th>First pass</th><th>Trapped</th><th class="n">Commencing to trap</th><th class="n">Passes</th><th>Grades</th></tr>`)
		for _, ap := range rc.Approaches {
			ct := "-"
			if !ap.Commencing.IsZero() && !ap.Trap.IsZero() {
				ct = mins(ap.Trap.Sub(ap.Commencing))
			}
			var sq strings.Builder
			for _, g := range ap.Grades {
				sq.WriteString(square(s, g))
			}
			fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class="n">%s</td><td class="n">%d</td><td>%s</td></tr>`,
				esc(ap.Pilot), esc(firstNonEmpty(ap.Callsign, "-")), r.when(ap.CheckIn), r.when(ap.Charlie), r.when(ap.Commencing), r.when(ap.Initial),
				r.when(ap.FirstPass), r.when(ap.Trap), ct, ap.Passes, sq.String())
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`<p class="note">A recovery is one carrier's passes with no gap over 25 minutes. Boarding rate is traps over passes, foul-deck wave-offs left out. Marshal times need the pilot's callsign matched to the DCS player; "-" where it was not heard or not matched.</p></section>`)
	return b.String()
}

// ---- 7. Tankers ----

func (r *Report) tankerPage() string {
	x := r.X
	if x == nil || len(x.Tanker) == 0 {
		return r.skip("Tankers: no refuelling in the server's events and no tanker calls.")
	}
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("Tankers", "plugs from the server's refuel events · calls from Readback's tanker frequencies · "+esc(r.Session.ZoneNote())))
	b.WriteString(`<table class="res wide"><tr><th>Pilot</th><th>Callsign</th><th>Tanker</th><th>Checked in</th><th>Asked for</th><th>First contact</th><th class="n">Check-in to contact</th><th class="n">Plugs</th><th class="n">Re-plugs</th><th class="n">Time in the basket</th></tr>`)
	for _, t := range x.Tanker {
		ct := "-"
		if !t.CheckIn.IsZero() && !t.FirstPlug.IsZero() && t.FirstPlug.After(t.CheckIn) {
			ct = mins(t.FirstPlug.Sub(t.CheckIn))
		}
		fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class="n">%s</td><td class="n">%d</td><td class="n">%d</td><td class="n">%s</td></tr>`,
			esc(firstNonEmpty(t.Pilot, "-")), esc(firstNonEmpty(t.Callsign, "-")), esc(firstNonEmpty(t.Tanker, "-")), r.when(t.CheckIn), esc(firstNonEmpty(t.Requested, "-")),
			r.when(t.FirstPlug), ct, t.Plugs, max(0, t.Plugs-t.Visits), mins(t.Connected))
	}
	b.WriteString(`</table><p class="note">A plug is a contact DCS logged; a re-plug is a second contact on the same visit (a disconnect). DCS does not log the fuel passed, so "asked for" is the pilot's request. Tanker calls need the callsign matched to the DCS player to share a row.</p></section>`)
	return b.String()
}

// ---- 3. Currency ----

func (r *Report) currencyPage() string {
	x := r.X
	if x == nil || len(x.Currency) == 0 {
		return r.skip("Currency: no players in the session.")
	}
	if x.HistoryRuns == 0 {
		return r.skip("Currency: no DCS server logs on this PC to count from (Stats, under Logs, can take uploads).")
	}
	var b strings.Builder
	b.WriteString(`<section class="page flow">`)
	b.WriteString(r.header("Currency", fmt.Sprintf("from the %d mission runs in the DCS server logs on this PC, to %s", x.HistoryRuns, x.CurrencyAt.In(r.Session.Loc).Format("2 Jan 2006"))))
	b.WriteString(`<table class="res wide"><tr><th>Pilot</th><th>Last sortie</th><th class="n">Sorties 30 d</th><th class="n">Landings 30 d</th><th>Flying</th><th>Last trap</th><th class="n">Traps 30 d</th><th class="n">Traps 90 d</th><th>Carrier</th><th class="n">Tanker plugs 30 d</th></tr>`)
	ago := func(t time.Time) string {
		if t.IsZero() {
			return `<span class="muted">none on record</span>`
		}
		d := int(x.CurrencyAt.Sub(t).Hours() / 24)
		w := map[bool]string{true: "today", false: fmt.Sprintf("%d days ago", d)}[d == 0]
		return t.In(r.Session.Loc).Format("2 Jan") + ` <span class="muted">(` + w + `)</span>`
	}
	for _, c := range x.Currency {
		lt := `<span class="muted">-</span>`
		if c.Carrier {
			lt = ago(c.LastTrap)
		}
		fmt.Fprintf(&b, `<tr><td><b>%s</b></td><td>%s</td><td class="n">%d</td><td class="n">%d</td><td>%s</td><td>%s</td><td class="n">%d</td><td class="n">%d</td><td>%s</td><td class="n">%d</td></tr>`,
			esc(c.Pilot), ago(c.LastSortie), c.Sorties30, c.Landings30, status(c.FlyStatus), lt, c.Traps30, c.Traps90, status(c.TrapStatus), c.Plugs30)
	}
	b.WriteString(`</table><p class="note"><span class="st current">Current</span> within 30 days · <span class="st due">Due</span> 31 to 60 days · <span class="st lapsed">Lapsed</span> over 60 days or none on record. Flying counts a takeoff; carrier counts a trap (a wire), for pilots who fly a carrier type or have trapped. Only the logs on this PC count: DCS keeps its older logs as dated .zip files beside dcs.log.</p></section>`)
	return b.String()
}

// ---- 2. A page per pilot ----

func (r *Report) pilotPages() string {
	s := r.Session
	if len(s.Pilots) == 0 {
		return r.skip("Pilot pages: no players in the session.")
	}
	x := r.X
	if x == nil {
		x = &Extras{}
	}
	ps := append([]*PilotStat(nil), s.Pilots...)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	var b strings.Builder
	for _, p := range ps {
		call := x.CallsignOf[p.Key]
		sub := []string{capitalize(p.Side), strings.Join(p.Aircraft, ", ")}
		if call != "" {
			sub = append([]string{"on the radio as " + call}, sub...)
		}
		b.WriteString(`<section class="page flow">`)
		b.WriteString(r.header("Pilot · "+p.Name, esc(strings.Join(sub, " · "))))
		b.WriteString(`<div class="chips">`)
		for _, c := range []struct {
			v    string
			what string
		}{{fmt.Sprint(p.Sorties), "sorties"}, {hm(p.Flight), "flight time"}, {fmt.Sprint(p.Landings), "landings"}, {fmt.Sprint(p.Traps), "traps"},
			{fmt.Sprint(p.Kills), "air kills"}, {fmt.Sprint(p.Ground), "ground kills"}, {fmt.Sprint(p.ShotDown), "shot down"},
			{fmt.Sprintf("%d / %d", p.Hits, p.Shots), "hits / shots"}} {
			if c.v == "1" {
				c.what = plural(1, c.what)
			}
			fmt.Fprintf(&b, `<span class="chip"><b>%s</b> %s</span>`, c.v, c.what)
		}
		b.WriteString(`</div><div class="cols"><div>`)
		b.WriteString(`<h3>Flight log</h3>`)
		if len(p.Log) == 0 {
			b.WriteString(`<p class="muted">No server events for this pilot (the server's dcs.log has them).</p>`)
		} else {
			b.WriteString(`<table class="res"><tr><th>Time</th><th>Event</th></tr>`)
			log := append([]LogEntry(nil), p.Log...)
			sort.SliceStable(log, func(i, j int) bool { return log[i].At.Before(log[j].At) })
			for i, e := range log {
				if i == 40 {
					fmt.Fprintf(&b, `<tr><td colspan="2" class="muted">and %d more</td></tr>`, len(log)-40)
					break
				}
				fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td></tr>`, r.when(e.At), esc(e.Text))
			}
			b.WriteString(`</table>`)
		}
		b.WriteString(`</div><div>`)

		// Comms
		b.WriteString(`<h3>Comms</h3>`)
		var cs *CommsScore
		for i := range x.Comms {
			if call != "" && x.Comms[i].Key == CallsignKey(call) {
				cs = &x.Comms[i]
			}
		}
		if cs == nil {
			b.WriteString(`<p class="muted">Not heard on Readback's frequencies under a callsign matched to this pilot.</p>`)
		} else {
			rb := "no handoffs or readbacks asked for"
			if cs.Asked > 0 {
				rb = fmt.Sprintf("%d of %d handoffs and readbacks done", cs.Done, cs.Asked)
			}
			fmt.Fprintf(&b, `<p><b class="g%s">%d · %s</b> &nbsp; %d calls, %d answered · %s · %d "say again"</p>`,
				strings.ToLower(strings.Fields(cs.Grade())[0]), cs.Score, cs.Grade(), cs.Calls, cs.Answered, rb, cs.SayAgain)
			if len(cs.Notes) > 0 {
				b.WriteString(`<ul class="tight">`)
				for _, n := range cs.Notes {
					b.WriteString(`<li>` + esc(n) + `</li>`)
				}
				b.WriteString(`</ul>`)
			}
		}

		// Carrier
		var passes []Trap
		for _, t := range s.Traps {
			if t.Pilot == p.Name {
				passes = append(passes, t)
			}
		}
		if len(passes) > 0 {
			b.WriteString(`<h3>Carrier passes</h3><p>`)
			for _, t := range passes {
				b.WriteString(square(s, t))
			}
			b.WriteString(`</p><table class="res">`)
			for _, t := range passes {
				fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td><td>%s</td></tr>`, r.when(t.At), esc(t.Place), esc(t.Raw))
			}
			b.WriteString(`</table>`)
		}

		// Weapons
		type wr struct{ fired, hit int }
		ws := map[string]*wr{}
		var worder []string
		for _, sh := range s.Shots {
			if sh.ShooterKey != p.Key || !sh.human {
				continue
			}
			w := firstNonEmpty(WeaponFamily(sh.Weapon), "Guns")
			if ws[w] == nil {
				ws[w] = &wr{}
				worder = append(worder, w)
			}
			ws[w].fired++
			if sh.Hit {
				ws[w].hit++
			}
		}
		if len(worder) > 0 {
			b.WriteString(`<h3>Weapons</h3><table class="res"><tr><th>Weapon</th><th class="n">Fired</th><th class="n">Hits</th><th>Hit rate</th></tr>`)
			for _, w := range worder {
				fmt.Fprintf(&b, `<tr><td>%s</td><td class="n">%d</td><td class="n">%d</td><td>%s</td></tr>`, esc(w), ws[w].fired, ws[w].hit, pctBar(ws[w].hit, ws[w].fired))
			}
			b.WriteString(`</table>`)
		}

		// Tanker
		for _, t := range x.Tanker {
			if (t.Pilot == p.Name) || (call != "" && CallsignKey(t.Callsign) == CallsignKey(call)) {
				fmt.Fprintf(&b, `<h3>Tanker</h3><p>%s · checked in %s · asked for %s · %d plug%s, %s in the basket</p>`,
					esc(firstNonEmpty(t.Tanker, "tanker")), r.when(t.CheckIn), esc(firstNonEmpty(t.Requested, "-")), t.Plugs,
					map[bool]string{true: "", false: "s"}[t.Plugs == 1], mins(t.Connected))
				break
			}
		}

		// Losses
		var ls []string
		for _, l := range x.Losses {
			if l.Key == p.Key || l.Pilot == p.Name {
				ls = append(ls, r.when(l.At)+" "+l.Cause+": "+l.Detail)
			}
		}
		if len(ls) > 0 {
			b.WriteString(`<h3>Losses</h3><ul class="tight">`)
			for _, l := range ls {
				b.WriteString(`<li>` + esc(l) + `</li>`)
			}
			b.WriteString(`</ul>`)
		}

		// Currency
		for _, c := range x.Currency {
			if c.Key == p.Key && x.HistoryRuns > 0 {
				b.WriteString(`<h3>Currency</h3><p>Flying ` + status(c.FlyStatus))
				if c.Carrier {
					b.WriteString(` &nbsp; Carrier ` + status(c.TrapStatus))
				}
				fmt.Fprintf(&b, ` &nbsp; %d sorties and %d traps in the last 30 days</p>`, c.Sorties30, c.Traps30)
			}
		}
		b.WriteString(`</div></div></section>`)
	}
	return b.String()
}
