package miz

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// Briefing is "what was planned" for an after action report: the mission's
// name, map, date and time, briefing texts, weather and the player slots.
type Briefing struct {
	File        string
	Name        string // the sortie name, else the file name
	Theatre     string
	Date        string // in-mission date, "25 Jan 2027"
	StartTime   string // in-mission start, "09:00"
	Description string
	BlueTask    string
	RedTask     string
	Weather     Weather
	Flights     []PlannedFlight // groups with player (Client) slots
}

// PlannedFlight is a group with seats for players.
type PlannedFlight struct {
	Coalition string
	Group     string
	Aircraft  string
	Slots     int
	Task      string
}

var months = []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// ReadBriefing reads a .miz's plan. The weather comes from ReadMizWeather.
func ReadBriefing(mizPath string) (*Briefing, error) {
	zr, err := zip.OpenReader(mizPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	read := func(name string) string {
		for _, f := range zr.File {
			if f.Name == name {
				rc, err := f.Open()
				if err != nil {
					return ""
				}
				defer rc.Close()
				b, _ := io.ReadAll(rc)
				return string(b)
			}
		}
		return ""
	}
	src := read("mission")
	if src == "" {
		return nil, fmt.Errorf("no mission in %s", filepath.Base(mizPath))
	}
	m, err := parseLuaTable(src)
	if err != nil {
		return nil, fmt.Errorf("reading the mission: %w", err)
	}
	dict := map[string]any{}
	if d := read("l10n/DEFAULT/dictionary"); d != "" {
		if t, err := parseLuaTable(d); err == nil {
			dict = t
		}
	}
	text := func(k string) string {
		v := strOf(m, k)
		if strings.HasPrefix(v, "DictKey_") {
			v, _ = dict[v].(string)
		}
		return strings.TrimSpace(v)
	}

	b := &Briefing{File: filepath.Base(mizPath), Theatre: strOf(m, "theatre"),
		Name: text("sortie"), Description: text("descriptionText"),
		BlueTask: text("descriptionBlueTask"), RedTask: text("descriptionRedTask")}
	if b.Name == "" {
		b.Name = strings.TrimSuffix(b.File, filepath.Ext(b.File))
	}
	if d := tbl(m, "date"); d != nil {
		if mo := int(numOf(d, "Month")); mo >= 1 && mo <= 12 {
			b.Date = fmt.Sprintf("%d %s %d", int(numOf(d, "Day")), months[mo], int(numOf(d, "Year")))
		}
	}
	if st, ok := m["start_time"].(float64); ok {
		s := int(st)
		b.StartTime = fmt.Sprintf("%02d:%02d", (s/3600)%24, (s/60)%60)
	}
	if w, err := ReadMizWeather(mizPath); err == nil {
		b.Weather = w
	}

	for _, side := range []string{"blue", "red", "neutrals"} {
		countries := tbl(m, "coalition", side, "country")
		for _, c := range items(countries) {
			for _, kind := range []string{"plane", "helicopter"} {
				for _, g := range items(tbl(c, kind, "group")) {
					gm, _ := g.(map[string]any)
					f := PlannedFlight{Coalition: side, Group: strOf(gm, "name"), Task: strOf(gm, "task")}
					for _, u := range items(tbl(gm, "units")) {
						um, _ := u.(map[string]any)
						if sk := strOf(um, "skill"); sk == "Client" || sk == "Player" {
							f.Slots++
							f.Aircraft = strOf(um, "type")
						}
					}
					if f.Slots > 0 {
						b.Flights = append(b.Flights, f)
					}
				}
			}
		}
	}
	sort.SliceStable(b.Flights, func(i, j int) bool { return b.Flights[i].Coalition < b.Flights[j].Coalition })
	return b, nil
}
