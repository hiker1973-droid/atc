package main

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

// Case I/II "signal Charlie" (operator 2026-10-16). A jet that reports
// established while the deck is busy is told "stand by for Charlie"; before
// this nothing ever followed, so Raider 302 commenced on his own (Day
// 2026-10-16 22:02). The watcher below gives Charlie to the lowest jet holding
// in the stack once the deck is clear, one jet at a time:
//   - no Charlie inside charlieSpacing of the last one, and
//   - none while a jet given Charlie has not yet called commencing, unless it
//     got it more than charlieStale ago (he never called, or the call was lost).
const (
	charlieSpacing = 60 * time.Second
	charlieStale   = 3 * time.Minute
	charlieTick    = 10 * time.Second
)

type charlieGate struct {
	mu   sync.Mutex
	last time.Time
}

var marshalCharlie = &charlieGate{}

// given records a Charlie transmitted at now (also from the established call).
func (g *charlieGate) given(now time.Time) {
	g.mu.Lock()
	g.last = now
	g.mu.Unlock()
}

// nextCharlie picks who gets Charlie at now, or "" for nobody yet.
func (g *charlieGate) nextCharlie(stack []*state.MarshalAircraft, now time.Time) string {
	g.mu.Lock()
	last := g.last
	g.mu.Unlock()
	if !last.IsZero() && now.Sub(last) < charlieSpacing {
		return ""
	}
	var holding []*state.MarshalAircraft
	for _, ac := range stack {
		switch ac.Phase {
		case "charlie":
			if now.Sub(ac.UpdatedAt) < charlieStale {
				return ""
			}
		case "holding":
			holding = append(holding, ac)
		}
	}
	if len(holding) == 0 {
		return ""
	}
	sort.Slice(holding, func(i, j int) bool {
		if holding[i].Angels != holding[j].Angels {
			return holding[i].Angels < holding[j].Angels
		}
		return holding[i].Position < holding[j].Position
	})
	return holding[0].Callsign
}

// watchCharlie runs for the life of the Marshal role.
func watchCharlie(ctx context.Context, stack *state.MarshalStack, comp *composer.ATCComposer, transmit func(string), atcCtrl *controller.ATCController) {
	tk := time.NewTicker(charlieTick)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}
		if atcCtrl.GetRecoveryCase() == state.CaseThree || !atcCtrl.IsDeckClear() {
			continue
		}
		now := time.Now()
		cs := marshalCharlie.nextCharlie(stack.GetAll(), now)
		if cs == "" {
			continue
		}
		stack.SetPhase(cs, "charlie")
		marshalCharlie.given(now)
		log.Info().Str("callsign", cs).Msg("Marshal: deck clear - signal Charlie")
		transmit(comp.MarshalSignalCharlie(cs))
	}
}
