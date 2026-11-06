package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/state"
)

// Which recovery case a pass was flown under (operator 2026-11-06: "is there a
// way we can determine a Case 1, 2 or 3"). Paddles' own process has no weather,
// so it asks the Marshal's dashboard: that case is the one the operator pinned
// from the launcher, else the weather-and-night rule (state.ComputeRecoveryCase).
// If the Marshal is not answering it applies the same rule to the weather a
// tower on this rig reports. The pass line then says ", case III" and the
// greenie board draws its circle from that. The same poll keeps the weather and
// the field for the pass card ("Paddles: pass context").

// lsoStatus is what the Marshal's (or a tower's) /status says: the case and the weather.
type lsoStatus struct {
	Case                                        int
	ICAO                                        string
	CeilingFt, VisNm, WindDir, WindKts, AltInHg float64
	IsNight                                     bool
}

type lsoCaseSource struct {
	marshal string // Marshal dashboard host:port ("" = tower weather only)

	mu   sync.Mutex
	val  int // 1/2/3, or 0 until the first answer
	src  string
	info lsoStatus
}

// get is the case now (0 = not known); it never blocks on the network.
func (s *lsoCaseSource) get() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.val
}

// context is the weather and field at the last poll, for the pass card's text (nil until the first answer).
func (s *lsoCaseSource) context() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.val == 0 {
		return nil
	}
	return map[string]any{"icao": s.info.ICAO, "ceilingFt": s.info.CeilingFt, "visNm": s.info.VisNm, "windDir": s.info.WindDir,
		"windKts": s.info.WindKts, "altInHg": s.info.AltInHg, "isNight": s.info.IsNight}
}

func (s *lsoCaseSource) set(st lsoStatus, src string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.Case != s.val || src != s.src {
		log.Info().Int("case", st.Case).Str("from", src).Msg("Paddles: recovery case")
	}
	s.val, s.src, s.info = st.Case, src, st
}

// run refreshes the case every 20 s until ctx ends.
func (s *lsoCaseSource) run(ctx context.Context) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		if st, ok := marshalStatus(s.marshal); ok {
			s.set(st, "Marshal")
		} else if st, ok := towerStatus(); ok {
			s.set(st, "tower weather")
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// marshalStatus reads the Marshal dashboard's /status ("" or any failure = not ok).
func marshalStatus(addr string) (lsoStatus, bool) {
	if addr == "" {
		return lsoStatus{}, false
	}
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://%s/status", addr))
	if err != nil {
		return lsoStatus{}, false
	}
	defer resp.Body.Close()
	var data struct {
		Tower struct {
			ICAO          string  `json:"icao"`
			RecoveryCase  int     `json:"recoveryCase"`
			CeilingFt     float64 `json:"ceilingFt"`
			VisibNm       float64 `json:"visibNm"`
			AltimeterInHg float64 `json:"altimeterInHg"`
			WindDir       float64 `json:"windDir"`
			WindKts       float64 `json:"windKts"`
			IsNight       bool    `json:"isNight"`
		} `json:"tower"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.Tower.RecoveryCase < 1 || data.Tower.RecoveryCase > 3 {
		return lsoStatus{}, false
	}
	t := data.Tower
	return lsoStatus{Case: t.RecoveryCase, ICAO: t.ICAO, CeilingFt: t.CeilingFt, VisNm: t.VisibNm, WindDir: t.WindDir,
		WindKts: t.WindKts, AltInHg: t.AltimeterInHg, IsNight: t.IsNight}, true
}

// marshalRecoveryCase is just the case (0 = not known).
func marshalRecoveryCase(addr string) int {
	st, _ := marshalStatus(addr)
	return st.Case
}

// towerStatus applies the weather-and-night rule to the first tower on this
// host that has weather.
func towerStatus() (lsoStatus, bool) {
	icaos := make([]string, 0, len(towerDashboardPortByICAO))
	for icao := range towerDashboardPortByICAO {
		icaos = append(icaos, icao)
	}
	sort.Strings(icaos)
	for _, icao := range icaos {
		if w, ok := fetchTowerWeather(icao); ok {
			return lsoStatus{Case: int(state.ComputeRecoveryCase(w.CeilFt, w.VisNm, w.IsNight)), ICAO: icao, CeilingFt: w.CeilFt,
				VisNm: w.VisNm, WindDir: w.WindDir, WindKts: w.WindKts, AltInHg: w.AltInHg, IsNight: w.IsNight}, true
		}
	}
	return lsoStatus{}, false
}
