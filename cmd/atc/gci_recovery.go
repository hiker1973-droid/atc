package main

import (
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Quiet during recoveries (operator 2026-10-21). SkyEye calls THREATs and the
// automatic PICTURE with no idea a jet is in the Marshal stack, on Paddles'
// glideslope or in a tower's pattern - talking over the calls that matter
// most then. Our own Tacview tracker (the one Command runs) knows where the
// carrier and the fields are, so a friendly that is
//
//   - within recoveryCarrierNm of the carrier and below recoveryCarrierFt, or
//   - within recoveryFieldNm of one of this theatre's fields and below
//     recoveryFieldAGL above it,
//
// gets no THREAT call, and the automatic PICTURE waits while anyone on the
// GCI frequency is recovering (pkg/gci/controller hooks RecoveryQuiet and
// AutoPictureQuiet, set by runGCI). Answers to a pilot's own request are
// never held: they asked.
const (
	recoveryCarrierNm  = 12.0    // Case I stack overhead and the pattern
	recoveryCarrierFt  = 10000.0 // Marshal stack angels 2-9
	recoveryFieldNm    = 10.0
	recoveryFieldAGL   = 4000.0
	recoveryStaleAfter = 30 * time.Second
)

// gciFields is the theatre's airfields, recovered to by the GCI's pilots.
// Kola lists every Operation WHITE NIGHT field, not just the one in play.
func gciFields(theatre string) []*airfield.Airfield {
	switch strings.ToLower(theatre) {
	case "caucasus":
		return airfield.Caucasus
	case "syria":
		return airfield.Syria
	case "germany":
		return airfield.Germany
	case "iraq":
		return airfield.Iraq
	case "kola":
		return []*airfield.Airfield{airfield.ENBO, airfield.ENDU, airfield.ENNA, airfield.EFRO}
	}
	return airfield.PersianGulf
}

type gciRecovery struct {
	store  *tacviewPositions
	fields []*airfield.Airfield
	now    func() time.Time

	mu   sync.Mutex
	last map[string]string // tracker key -> where they are recovering ("" = not)
}

func newGCIRecovery(store *tacviewPositions, fields []*airfield.Airfield) *gciRecovery {
	return &gciRecovery{store: store, fields: fields, now: time.Now, last: map[string]string{}}
}

// trackerKey is how the mini Tacview tracker files a pilot: the DCS name
// before the first "|" ("Raider 302 | Jedi" -> "Raider 302").
func trackerKey(name string) string {
	if i := strings.Index(name, "|"); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// where reports where a pilot is recovering to, "" when they are not.
func (r *gciRecovery) where(contactName string) string {
	key := trackerKey(contactName)
	pos, ok := r.store.Get(key)
	if !ok {
		return ""
	}
	info := r.store.Info(key)
	if !info.UpdatedAt.IsZero() && r.now().Sub(info.UpdatedAt) > recoveryStaleAfter {
		return ""
	}
	if boat, ok := r.store.Carrier(); ok && haversineNm(pos, boat) <= recoveryCarrierNm && info.AltFt <= recoveryCarrierFt {
		return "the carrier"
	}
	for _, f := range r.fields {
		if haversineNm(pos, f.Center) <= recoveryFieldNm && info.AltFt <= float64(f.ElevationFt)+recoveryFieldAGL {
			return f.Name
		}
	}
	return ""
}

// quiet is the RecoveryQuiet hook: true while the pilot is recovering. It
// logs once when a pilot starts and stops, not on every radar sweep.
func (r *gciRecovery) quiet(contactName string) bool {
	at := r.where(contactName)
	key := trackerKey(contactName)
	r.mu.Lock()
	prev, seen := r.last[key]
	r.last[key] = at
	r.mu.Unlock()
	if at != prev && (seen || at != "") {
		if at != "" {
			log.Info().Str("pilot", contactName).Str("at", at).Msg("GCI: recovering - threat calls and the automatic picture held")
		} else {
			log.Info().Str("pilot", contactName).Msg("GCI: no longer recovering - threat calls back on")
		}
	}
	return at != ""
}

// anyRecovering is the AutoPictureQuiet hook: someone on the GCI frequency is
// recovering.
func (r *gciRecovery) anyRecovering(names []string) bool {
	for _, n := range names {
		if r.quiet(n) {
			return true
		}
	}
	return false
}
