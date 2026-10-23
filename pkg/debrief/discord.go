package debrief

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The AAR's Discord post, in the form the squadron already posts (operator
// 2026-10-23: "can you make the discord post option look like this", the
// BFM Venom vs Raider post): a header, the teams, KILLS and LOSSES by
// fight with T+ mission times, SHOT STATS per pilot and side, then HOW IT
// WENT, OUTCOME and TAKEAWAYS. Everything but the last three is worked out
// here; Claude writes those (AIText.Discord*). Discord takes 2,000
// characters a message, so it comes as several posts.

const discordLimit = 1900

// DiscordPosts is the post, split into messages Discord will take.
func (r *Report) DiscordPosts() []string {
	var secs []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			secs = append(secs, s)
		}
	}
	add(r.discordHeader())
	add(r.discordKills())
	add(r.discordLosses())
	add(r.discordShots())
	add(r.discordStory())
	return pack(secs, discordLimit)
}

// side labels: "Venom" / "Raider" from the players' groups, else Blue / Red.
func (r *Report) sideName(side string) string {
	s := r.Session
	switch side {
	case "blue":
		if s.BlueName != "" {
			return s.BlueName
		}
		return "Blue"
	case "red":
		if s.RedName != "" {
			return s.RedName
		}
		return "Red"
	}
	return "?"
}

// tPlus is a time as mission time, "T+48:56" (minutes past 60 stay minutes).
func (r *Report) tPlus(at time.Time) string {
	start := r.Session.MissionStart
	if start.IsZero() {
		start = r.Session.Start
	}
	d := at.Sub(start)
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("T+%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

// tWidth is the T+ column's width: the longest time and two spaces.
func (r *Report) tWidth() int {
	w := len("T+00:00")
	for _, k := range r.Session.Kills {
		w = max(w, len(r.tPlus(k.At)))
	}
	return w + 2
}

var versionRE = regexp.MustCompile(`\bV\d+(\.\d+)*\b`)

func (r *Report) discordHeader() string {
	s := r.Session
	var b strings.Builder
	title := strings.ToUpper(versionRE.ReplaceAllString(r.Mission, ""))
	title = strings.TrimSpace(strings.Trim(strings.TrimSpace(title), "-·"))
	title = strings.NewReplacer(" - ", " — ", " VS ", " vs ").Replace(title) // "BFM — VENOM vs RAIDER"
	fmt.Fprintf(&b, "# :airplane: %s — AAR\n", title)

	count := map[string]int{}
	pilots := map[string][]string{}
	aircraft := map[string][]string{}
	for _, p := range s.Pilots {
		count[p.Side]++
		pilots[p.Side] = append(pilots[p.Side], p.Name)
		for _, a := range p.Aircraft {
			aircraft[p.Side] = addUnique(aircraft[p.Side], a)
		}
	}
	field := map[string]string{}
	for _, f := range s.Fields {
		if field[f.Side] == "" {
			field[f.Side] = f.Name
		}
	}
	var line []string
	if s.PvP {
		line = append(line, fmt.Sprintf("%d v %d", count["blue"], count["red"]))
		if field["blue"] != "" && field["red"] != "" {
			line = append(line, field["blue"]+" vs "+field["red"])
		}
	} else if len(s.Pilots) > 0 {
		line = append(line, fmt.Sprintf("%d pilots", len(s.Pilots)))
	}
	line = append(line, s.Start.In(s.Loc).Format("Jan 2"))
	fmt.Fprintf(&b, "**%s**\n", strings.Join(line, " · "))
	sub := []string{fmt.Sprintf("%d min", int(s.End.Sub(s.Start).Minutes())), fmt.Sprintf("%d pilots", len(s.Pilots))}
	if v := versionRE.FindString(r.Mission); v != "" {
		sub = append(sub, v)
	}
	fmt.Fprintf(&b, "*%s*\n", strings.Join(sub, " · "))
	if len(s.Pilots) > 0 {
		b.WriteString("\n")
		for _, side := range []string{"blue", "red"} {
			if len(pilots[side]) == 0 {
				continue
			}
			sort.Strings(pilots[side])
			where := ""
			if field[side] != "" {
				where = " (" + field[side] + ")"
			}
			sq := map[string]string{"blue": ":blue_square:", "red": ":red_square:"}[side]
			team := ""
			if n := r.sideName(side); n != capitalize(side) {
				team = n + " — "
			}
			fmt.Fprintf(&b, "**%s %s%s:** %s%s (%s)\n", sq, strings.ToUpper(side), where, team, strings.Join(pilots[side], ", "), strings.Join(aircraft[side], ", "))
		}
	}
	return b.String()
}

// airKills is the session's air kills by engagement.
func (r *Report) fights() []*Engagement {
	var out []*Engagement
	for _, en := range r.Session.Engagements {
		if len(en.Kills) > 0 || len(en.Shots) > 0 {
			out = append(out, en)
		}
	}
	return out
}

func pad(s string, n int) string {
	if r := []rune(s); len(r) < n {
		return s + strings.Repeat(" ", n-len(r))
	}
	return s
}

// shotDownAt is when a player was shot down, for "already dead" notes.
func (r *Report) shotDownAt(name string, before time.Time) (time.Time, bool) {
	var last time.Time
	for _, k := range r.Session.Kills {
		if k.Victim == name && k.At.Before(before) && k.At.After(last) {
			last = k.At
		}
	}
	return last, !last.IsZero() && before.Sub(last) < 3*time.Minute
}

func (r *Report) discordKills() string {
	s := r.Session
	fights := r.fights()
	total := 0
	by := map[string]int{}
	for _, en := range fights {
		for _, k := range en.Kills {
			total++
			by[k.KillerSide]++
		}
	}
	if total == 0 {
		return ""
	}
	var b strings.Builder
	var split []string
	for _, side := range []string{"blue", "red"} {
		if by[side] > 0 || s.PvP {
			split = append(split, fmt.Sprintf("%s %d", r.sideName(side), by[side]))
		}
	}
	fmt.Fprintf(&b, "## :red_circle: KILLS — %d confirmed (%s)\n```\n", total, strings.Join(split, ", "))
	for i, en := range fights {
		if len(en.Kills) == 0 {
			continue
		}
		if len(fights) > 1 {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "FIGHT %d\n", en.N)
		}
		kills := append([]*Kill(nil), en.Kills...)
		sort.SliceStable(kills, func(a, c int) bool { return kills[a].At.Before(kills[c].At) })
		for _, k := range kills {
			note := ""
			if t, dead := r.shotDownAt(k.Killer, k.At); dead && k.At.Sub(t) > 2*time.Second {
				note = fmt.Sprintf("  (hit after %s was already dead)", k.Killer)
			}
			fmt.Fprintf(&b, "%s%s > %s %s%s\n", pad(r.tPlus(k.At), r.tWidth()), pad(k.Killer, 7), pad(k.Victim, 7), shortWpn(k.Weapon), note)
		}
	}
	b.WriteString("```\n")
	// The top shooter.
	type st struct {
		name           string
		kills, shots   int
		deaths, shotAt int
	}
	stats := r.shotTable()
	var best *st
	for _, p := range stats {
		if best == nil || p.kills > best.kills || (p.kills == best.kills && p.shots < best.shots) {
			best = &st{p.name, p.kills, p.shots, p.deaths, p.shotAt}
		}
	}
	if best != nil && best.kills > 1 {
		tail := "never touched"
		if best.deaths > 0 {
			tail = fmt.Sprintf("shot down %d", best.deaths)
		}
		fmt.Fprintf(&b, "**%s: %d kills from %d shots, %s.**\n", best.name, best.kills, best.shots, tail)
	}
	// Hits on players that DCS never scored as kills.
	for _, en := range fights {
		for _, sh := range en.Shots {
			if !sh.Hit || sh.KillN > 0 || sh.Friendly || sh.Target == "" {
				continue
			}
			killed := false
			for _, k := range s.Kills {
				if k.Victim == sh.Target && k.At.Sub(sh.At) > -5*time.Second && k.At.Sub(sh.At) < 3*time.Minute {
					killed = true
				}
			}
			if !killed {
				fmt.Fprintf(&b, "Not credited: %s hit %s at %s in Fight %d, but no kill was logged.\n", sh.Shooter, sh.Target, r.tPlus(sh.At), en.N)
			}
		}
	}
	return b.String()
}

func (r *Report) discordLosses() string {
	fights := r.fights()
	n := 0
	for _, en := range fights {
		n += len(en.Kills)
	}
	if n == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## :skull: LOSSES — %d\n```\n", n)
	for i, en := range fights {
		if len(en.Kills) == 0 {
			continue
		}
		if len(fights) > 1 {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "FIGHT %d\n", en.N)
		}
		kills := append([]*Kill(nil), en.Kills...)
		sort.SliceStable(kills, func(a, c int) bool { return kills[a].At.Before(kills[c].At) })
		for _, k := range kills {
			fate := "KIA"
			for _, t := range r.Session.ejected[k.Victim] {
				if d := t.Sub(k.At); d > -20*time.Second && d < 60*time.Second {
					fate = "ejected"
				}
			}
			fmt.Fprintf(&b, "%s%s %s %s %s\n", pad(r.tPlus(k.At), r.tWidth()), pad(k.Victim, 7), pad(shortWpn(k.Weapon), 9), pad(k.Killer, 7), fate)
		}
	}
	b.WriteString("```")
	return b.String()
}

type shotRow struct {
	name, side                   string
	shots, kills, deaths, shotAt int
}

// shotTable: every player's shots, kills, deaths and missiles fired at them
// (blue-on-blue left out).
func (r *Report) shotTable() []*shotRow {
	s := r.Session
	rows := map[string]*shotRow{}
	var order []*shotRow
	row := func(name, side string) *shotRow {
		if x := rows[name]; x != nil {
			return x
		}
		x := &shotRow{name: name, side: side}
		rows[name] = x
		order = append(order, x)
		return x
	}
	for _, p := range s.Pilots {
		row(p.Name, p.Side).deaths = p.ShotDown
		rows[p.Name].kills = p.Kills
	}
	for _, sh := range s.Shots {
		if sh.Friendly {
			continue
		}
		if x := rows[sh.Shooter]; x != nil && sh.human {
			x.shots++
		}
		if x := rows[sh.Target]; x != nil {
			x.shotAt++
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].side != order[j].side {
			return order[i].side < order[j].side
		}
		if order[i].kills != order[j].kills {
			return order[i].kills > order[j].kills
		}
		return order[i].shots > order[j].shots
	})
	return order
}

func (r *Report) discordShots() string {
	rows := r.shotTable()
	total := 0
	families := map[string]int{}
	for _, sh := range r.Session.Shots {
		if sh.human && !sh.Friendly {
			total++
			families[WeaponFamily(sh.Weapon)]++
		}
	}
	if total == 0 {
		return ""
	}
	what := "missiles"
	if len(families) == 1 {
		for f := range families {
			what = map[string]string{"AIM-120": "AMRAAMs", "AIM-9X": "Sidewinders", "AIM-9M": "Sidewinders", "AIM-7": "Sparrows", "AIM-54": "Phoenixes"}[f]
			if what == "" {
				what = f + "s"
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## :bar_chart: SHOT STATS — %d %s fired\n```\nPILOT    SHOTS  KILLS  DEATHS  SHOT AT\n", total, what)
	var ratios []string
	for i, side := range []string{"blue", "red"} {
		var sum shotRow
		any := false
		for _, x := range rows {
			if x.side != side {
				continue
			}
			any = true
			fmt.Fprintf(&b, "%s %5d  %5d  %6d  %7d\n", pad(x.name, 8), x.shots, x.kills, x.deaths, x.shotAt)
			sum.shots += x.shots
			sum.kills += x.kills
			sum.deaths += x.deaths
			sum.shotAt += x.shotAt
		}
		if !any {
			continue
		}
		fmt.Fprintf(&b, "-------------------------------------\n%s %5d  %5d  %6d  %7d\n", pad(strings.ToUpper(r.sideName(side)), 8), sum.shots, sum.kills, sum.deaths, sum.shotAt)
		if i == 0 {
			b.WriteString("\n")
		}
		if sum.kills > 0 {
			ratios = append(ratios, fmt.Sprintf("%s: 1 kill per %s shots.", r.sideName(side), trimFloat(float64(sum.shots)/float64(sum.kills))))
		} else if sum.shots > 0 {
			ratios = append(ratios, fmt.Sprintf("%s: no kills from %d shots.", r.sideName(side), sum.shots))
		}
	}
	b.WriteString("```\n")
	if len(ratios) > 0 {
		b.WriteString(strings.Join(ratios, " ") + "\n")
	}
	// The most-shot-at pilot who was never shot down.
	var hero *shotRow
	for _, x := range rows {
		if x.deaths == 0 && x.shotAt >= 3 && (hero == nil || x.shotAt > hero.shotAt) {
			hero = x
		}
	}
	if hero != nil {
		fmt.Fprintf(&b, "**%s defeated all %d missiles fired at him.**\n", hero.name, hero.shotAt)
	}
	return b.String()
}

func (r *Report) discordStory() string {
	var b strings.Builder
	ai := r.AI
	fights := r.fights()
	if ai != nil && len(ai.DiscordFights) > 0 && len(fights) > 0 {
		b.WriteString("## :crossed_swords: HOW IT WENT\n")
		for i, en := range fights {
			if i >= len(ai.DiscordFights) {
				break
			}
			d := en.LastEvent.Sub(en.FirstShot)
			by := map[string]int{}
			for _, k := range en.Kills {
				by[k.KillerSide]++
			}
			score := ""
			switch {
			case by["blue"] >= by["red"]:
				score = fmt.Sprintf("%s %d–%d", r.sideName("blue"), by["blue"], by["red"])
			default:
				score = fmt.Sprintf("%s %d–%d", r.sideName("red"), by["red"], by["blue"])
			}
			fmt.Fprintf(&b, "**Fight %d (%d min %02d s, %s).** %s\n\n", en.N, int(d.Minutes()), int(d.Seconds())%60, score, strings.TrimSpace(ai.DiscordFights[i]))
		}
	}
	if ai != nil && strings.TrimSpace(ai.DiscordOutcome) != "" {
		b.WriteString("## :dart: OUTCOME\n" + strings.TrimSpace(ai.DiscordOutcome) + "\n\n")
	}
	if ai != nil && len(ai.DiscordTake) > 0 {
		b.WriteString("## :clipboard: TAKEAWAYS\n")
		for _, t := range ai.DiscordTake {
			b.WriteString("- " + strings.TrimSpace(t) + "\n")
		}
		b.WriteString("\n")
	}
	if b.Len() > 0 {
		b.WriteString("*Full AAR attached.*")
	}
	return b.String()
}

func shortWpn(w string) string {
	if w == "" {
		return "guns"
	}
	return w
}

func trimFloat(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(s, ".0")
}

// pack joins sections into messages under the limit, splitting a section
// that is too long on its lines (keeping code blocks closed).
func pack(secs []string, limit int) []string {
	var out []string
	cur := ""
	flush := func() {
		if strings.TrimSpace(cur) != "" {
			out = append(out, strings.TrimSpace(cur))
		}
		cur = ""
	}
	for _, sec := range secs {
		if len(cur)+len(sec)+2 <= limit {
			if cur != "" {
				cur += "\n\n"
			}
			cur += sec
			continue
		}
		flush()
		if len(sec) <= limit {
			cur = sec
			continue
		}
		inCode := false
		for _, line := range strings.Split(sec, "\n") {
			if len(cur)+len(line)+1 > limit-4 {
				if inCode {
					cur += "\n```"
				}
				flush()
				if inCode {
					cur = "```"
				}
			}
			if cur != "" {
				cur += "\n"
			}
			cur += line
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inCode = !inCode
			}
		}
	}
	flush()
	return out
}
