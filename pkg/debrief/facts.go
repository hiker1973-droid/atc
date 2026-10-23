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
	start, end := sessionStart(in), in.To
	if in.Tacview != nil && !in.Tacview.End.IsZero() {
		end = in.Tacview.End
	}
	if !start.IsZero() {
		add("Date", start.UTC().Format("2 January 2006"))
		add("Time", fmt.Sprintf("%s - %s (%s)", start.UTC().Format("1504Z"), end.UTC().Format("1504Z"), dur(end.Sub(start))))
	}
	if m := in.Mission; m != nil && m.Date != "" {
		add("Mission time", m.Date+", "+m.StartTime+" local")
	}
	add("Weather", WeatherLine(in.Mission))

	if r := in.Tacview; r != nil {
		var pilots, types []string
		seenP, seenT := map[string]bool{}, map[string]bool{}
		sorties, losses := 0, 0
		weapons := map[string]int{}
		airKills, groundKills := 0, 0
		side := ""
		for _, f := range r.Flights {
			if !f.Human {
				continue
			}
			side = f.Coalition
			if !seenP[f.Pilot] {
				seenP[f.Pilot] = true
				pilots = append(pilots, f.Pilot)
			}
			if !seenT[f.Aircraft] {
				seenT[f.Aircraft] = true
				types = append(types, f.Aircraft)
			}
			sorties += len(f.Takeoffs)
			if strings.HasPrefix(f.Fate, "destroyed") {
				losses++
			}
			for k, n := range f.Fired {
				weapons[k] += n
			}
		}
		for _, l := range r.Losses {
			if l.By == "" || !seenP[l.By] {
				continue
			}
			if strings.Contains(l.Type, "Air") {
				airKills++
			} else {
				groundKills++
			}
		}
		sort.Strings(types)
		add("Players", fmt.Sprintf("%d (%s)", len(pilots), strings.Join(types, ", ")))
		add("Sorties", fmt.Sprintf("%d takeoffs", sorties))
		add("Weapons expended", weaponLine(weapons))
		add("Kills (likely)", fmt.Sprintf("%d air, %d ground/sea", airKills, groundKills))
		friendlyLost := 0
		for _, l := range r.Losses {
			if side != "" && l.Coalition == side && strings.Contains(l.Type, "Air") {
				friendlyLost++
			}
		}
		add("Losses", fmt.Sprintf("%d player aircraft, %d friendly aircraft in all", losses, friendlyLost))
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
