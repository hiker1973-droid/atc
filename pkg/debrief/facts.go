package debrief

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/miz"
)

// Fact is one line of the AAR's data block (Unit, Mission, Sorties, ...).
// The block is computed here from the records, not written by Claude, so
// its numbers are exact.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ReportNumber is the AAR's number: the session's start, "AAR-20261023-0015".
func ReportNumber(in Inputs) string {
	t := sessionStart(in)
	if t.IsZero() {
		t = time.Now()
	}
	return "AAR-" + t.UTC().Format("20060102-1504")
}

func sessionStart(in Inputs) time.Time {
	if in.Session != nil && !in.Session.Start.IsZero() {
		return in.Session.Start
	}
	if in.Tacview != nil && !in.Tacview.Start.IsZero() {
		return in.Tacview.Start
	}
	return in.From
}

// MissionName is the best name for the session: the mission file's, else
// the recording's title.
func MissionName(in Inputs) string {
	name := "Session"
	switch {
	case in.Mission != nil && in.Mission.Name != "":
		name = in.Mission.Name
	case in.Tacview != nil && in.Tacview.Title != "":
		name = in.Tacview.Title
	}
	return strings.Join(strings.Fields(strings.ReplaceAll(name, "_", " ")), " ") // file names: "vSFG-7_Training_Mission_CA V1.36"
}

// WeatherLine is the mission's start weather in one line.
func WeatherLine(m *miz.Briefing) string {
	if m == nil {
		return ""
	}
	w := m.Weather
	if w.SourcePath == "" {
		return ""
	}
	parts := []string{fmt.Sprintf("wind %03.0f° %.0f kt", w.WindDirTrue, w.WindKts)}
	if w.VisNm > 0 {
		parts = append(parts, fmt.Sprintf("visibility %.0f nm", math.Min(w.VisNm, 50)))
	}
	switch {
	case w.Clouds != "":
		parts = append(parts, strings.TrimSpace(w.Clouds))
	case w.CeilFt < 25000:
		parts = append(parts, fmt.Sprintf("ceiling %.0f ft", w.CeilFt))
	default:
		parts = append(parts, "no significant cloud")
	}
	parts = append(parts, fmt.Sprintf("%.0f°C", w.TempC))
	if w.AltInHg > 0 {
		parts = append(parts, fmt.Sprintf("QNH %.2f", w.AltInHg))
	}
	return strings.Join(parts, ", ")
}

// Facts is the AAR data block.
func Facts(in Inputs, sources []string) []Fact {
	var out []Fact
	add := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			out = append(out, Fact{label, value})
		}
	}
	add("Unit", in.Squadron)
	add("Mission", MissionName(in))
	theatre := ""
	if in.Mission != nil {
		theatre = in.Mission.Theatre
	}
	add("Theatre", theatre)
	if s := in.Session; s != nil && !s.Start.IsZero() {
		add("Date", s.Start.In(s.Loc).Format("Monday 2 January 2006"))
		add("Time", fmt.Sprintf("%s - %s %s (%s)", s.Clock(s.Start), s.Clock(s.End), strings.TrimPrefix(s.ZoneNote(), "times "), dur(s.End.Sub(s.Start))))
	}
	if m := in.Mission; m != nil && m.Date != "" {
		add("Mission time", m.Date+", "+m.StartTime+" in the mission")
	}
	add("Weather", WeatherLine(in.Mission))

	if s := in.Session; s != nil && len(s.Pilots) > 0 {
		var types []string
		sorties, air, ground := 0, 0, 0
		down := map[string]int{}
		var flight time.Duration
		weapons := map[string]int{}
		for _, p := range s.Pilots {
			for _, a := range p.Aircraft {
				types = addUnique(types, a)
			}
			sorties += p.Sorties
			air += p.Kills
			ground += p.Ground
			down[p.Side] += p.ShotDown
			flight += p.Flight
		}
		for _, m := range s.Missiles {
			weapons[m.Weapon] += m.Fired
		}
		sort.Strings(types)
		add("Players", fmt.Sprintf("%d (%s)", len(s.Pilots), strings.Join(types, ", ")))
		add("Sorties", fmt.Sprintf("%d, %s flying", sorties, hm(flight)))
		add("Weapons expended", weaponLine(weapons))
		if s.HasEvents {
			add("Kills", fmt.Sprintf("%d air, %d ground/sea", air, ground))
			if s.PvP {
				add("Losses", fmt.Sprintf("Blue %d, Red %d players shot down", down["blue"], down["red"]))
			} else {
				add("Losses", fmt.Sprintf("%d players shot down", down["blue"]+down["red"]+down[""]))
			}
		}
		if len(s.Engagements) > 0 {
			add("Engagements", fmt.Sprintf("%d", len(s.Engagements)))
		}
		if len(s.Traps) > 0 {
			add("Carrier", fmt.Sprintf("%d graded passes", len(s.Traps)))
		}
	}
	if rad := in.Radio; rad != nil {
		calls, tx := 0, 0
		for _, l := range rad.Lines {
			switch l.Kind {
			case "pilot":
				calls++
			case "atc":
				tx++
			}
		}
		add("Radio", fmt.Sprintf("%d pilot calls, %d controller transmissions, %d not understood", calls, tx, rad.Unheard))
	}
	add("Sources", strings.Join(sources, "; "))
	return out
}

func weaponLine(m map[string]int) string {
	if len(m) == 0 {
		return "none recorded"
	}
	type kv struct {
		k string
		n int
	}
	var list []kv
	total := 0
	for k, n := range m {
		list = append(list, kv{k, n})
		total += n
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n || (list[i].n == list[j].n && list[i].k < list[j].k) })
	var parts []string
	for i, x := range list {
		if i == 6 {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, fmt.Sprintf("%d %s", x.n, x.k))
	}
	return fmt.Sprintf("%d (%s)", total, strings.Join(parts, ", "))
}

func dur(d time.Duration) string {
	if d <= 0 {
		return "0 min"
	}
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d h %02d min", h, m)
}
