package main

// Marshal's stack track: where each jet in the stack flew, from the Tacview feed the role already reads.
//
// Operator 2026-11-06: the Marshal card's Case I Stack View should show the real track, not only the planned
// pattern ("build it with the Tacview track"). Marshal logged a handful of radar fixes per approach; this
// samples every jet in the stack every 10 s and logs the samples once a minute (and when the jet leaves the
// stack) as "Marshal: stack track": "unix:rangeNm:bearingMag:altFt|..." from the carrier. The card reads these
// lines; the Paddles pass pattern covers the last five minutes into the groove.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

const (
	marshalTrackEvery = 10 * time.Second
	marshalTrackFlush = 6 // samples per log line (one minute)
)

// marshalTrackSample is one "unix:rangeNm:bearingMag:altFt" sample.
func marshalTrackSample(at time.Time, distNm, bearingMag, altFt float64) string {
	return fmt.Sprintf("%d:%.2f:%.0f:%.0f", at.Unix(), distNm, bearingMag, altFt)
}

func watchMarshalTracks(ctx context.Context, stack *state.MarshalStack, atcCtrl *controller.ATCController) {
	buf := map[string][]string{}
	flush := func(cs string) {
		if len(buf[cs]) > 0 {
			log.Info().Str("callsign", cs).Str("track", strings.Join(buf[cs], "|")).Msg("Marshal: stack track")
		}
		delete(buf, cs)
	}
	tk := time.NewTicker(marshalTrackEvery)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tk.C:
			inStack := map[string]bool{}
			for _, ac := range stack.GetAll() {
				inStack[ac.Callsign] = true
				alt, dist, brg, ok := atcCtrl.CallerCarrierFix(ac.Callsign)
				if !ok {
					continue
				}
				buf[ac.Callsign] = append(buf[ac.Callsign], marshalTrackSample(now, dist, atcCtrl.ToMagnetic(brg), alt))
				if len(buf[ac.Callsign]) >= marshalTrackFlush {
					flush(ac.Callsign)
				}
			}
			for cs := range buf {
				if !inStack[cs] {
					flush(cs)
				}
			}
		}
	}
}
