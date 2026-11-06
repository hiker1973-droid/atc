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
// greenie board draws its circle from that.

type lsoCaseSource struct {
	marshal string // Marshal dashboard host:port ("" = tower weather only)

	mu  sync.Mutex
	val int // 1/2/3, or 0 until the first answer
	src string
}

// get is the case now (0 = not known); it never blocks on the network.
func (s *lsoCaseSource) get() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.val
}

func (s *lsoCaseSource) set(n int, src string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n != s.val || src != s.src {
		log.Info().Int("case", n).Str("from", src).Msg("Paddles: recovery case")
	}
	s.val, s.src = n, src
}

// run refreshes the case every 20 s until ctx ends.
func (s *lsoCaseSource) run(ctx context.Context) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		if n := marshalRecoveryCase(s.marshal); n > 0 {
			s.set(n, "Marshal")
		} else if n := towerRecoveryCase(); n > 0 {
			s.set(n, "tower weather")
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// marshalRecoveryCase reads the Marshal dashboard's /status ("" or any failure = 0).
func marshalRecoveryCase(addr string) int {
	if addr == "" {
		return 0
	}
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://%s/status", addr))
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var data struct {
		Tower struct {
			RecoveryCase int `json:"recoveryCase"`
		} `json:"tower"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.Tower.RecoveryCase < 1 || data.Tower.RecoveryCase > 3 {
		return 0
	}
	return data.Tower.RecoveryCase
}

// towerRecoveryCase applies the weather-and-night rule to the first tower on
// this host that has weather.
func towerRecoveryCase() int {
	icaos := make([]string, 0, len(towerDashboardPortByICAO))
	for icao := range towerDashboardPortByICAO {
		icaos = append(icaos, icao)
	}
	sort.Strings(icaos)
	for _, icao := range icaos {
		if w, ok := fetchTowerWeather(icao); ok {
			return int(state.ComputeRecoveryCase(w.CeilFt, w.VisNm, w.IsNight))
		}
	}
	return 0
}
