package main

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DCS-gRPC/go-bindings/dcs/v0/mission"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DCS's own carrier LSO grades every recovery (S_EVENT_LANDING_QUALITY_MARK,
// what Tactical Paddles reads over DCS-gRPC): "LSO: GRADE:OK  : WIRE# 3".
// Tacview has no wire, so Paddles logs DCS's grade and wire next to its own
// estimate of the same pass - the truth to calibrate the aim point against
// (operator 2026-10-17). Read-only; another client beside Tactical Paddles and
// SkyEye on :50051.

var flagLSOGrpcAddr string

var (
	dcsGradeRe = regexp.MustCompile(`GRADE:\s*(\S+)`)
	dcsWireRe  = regexp.MustCompile(`WIRE#?\s*(\d)`)
)

// parseDCSGrade pulls the grade and wire (0 = none) out of a DCS LSO comment.
func parseDCSGrade(comment string) (grade string, wire int) {
	if m := dcsGradeRe.FindStringSubmatch(comment); m != nil {
		grade = m[1]
	}
	if m := dcsWireRe.FindStringSubmatch(comment); m != nil {
		wire, _ = strconv.Atoi(m[1])
	}
	return grade, wire
}

// lsoTruth pairs Paddles' pass summaries with DCS's grades by callsign,
// whichever arrives first, within lsoTruthWindow.
type lsoTruth struct {
	mu     sync.Mutex
	passes map[string]lsoTruthItem
	dcs    map[string]lsoTruthItem
}

type lsoTruthItem struct {
	at   time.Time
	text string
}

const lsoTruthWindow = 90 * time.Second

func newLSOTruth() *lsoTruth {
	return &lsoTruth{passes: make(map[string]lsoTruthItem), dcs: make(map[string]lsoTruthItem)}
}

// pass is a finished Paddles pass ("Raider 302 (FA-18C_hornet): trap, ...").
func (t *lsoTruth) pass(summary string, now time.Time) {
	cs := summary
	if i := strings.Index(cs, " ("); i > 0 {
		cs = cs[:i]
	}
	t.match(cs, summary, true, now)
}

// grade is a DCS LSO comment for a player.
func (t *lsoTruth) grade(cs, comment string, now time.Time) {
	t.match(cs, comment, false, now)
}

func (t *lsoTruth) match(cs, text string, isPass bool, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	mine, other := t.dcs, t.passes
	if isPass {
		mine, other = t.passes, t.dcs
	}
	o, ok := other[cs]
	if !ok || now.Sub(o.at) > lsoTruthWindow {
		mine[cs] = lsoTruthItem{now, text}
		return
	}
	delete(other, cs)
	paddles, dcs := o.text, text
	if isPass {
		paddles, dcs = text, o.text
	}
	grade, wire := parseDCSGrade(dcs)
	log.Info().Str("callsign", cs).Str("dcsGrade", grade).Int("dcsWire", wire).Str("dcs", dcs).
		Str("paddles", paddles).Msg("Paddles: vs DCS LSO")
}

// runLSOGrpc listens for DCS's landing grades. "" turns it off.
func runLSOGrpc(ctx context.Context, addr string, truth *lsoTruth) {
	if addr == "" {
		return
	}
	// One warning per outage and one line when it is back, not a warning per
	// 30 s retry: a mission restart was 2-6 warnings, and a Paddles dialling
	// a port with no DCS-gRPC behind it (Foothold 2026-10-18) 271.
	var downSince time.Time
	connected := func() {
		if downSince.IsZero() {
			log.Info().Str("addr", addr).Msg("Paddles: DCS-gRPC events connected")
			return
		}
		log.Info().Str("addr", addr).Dur("down", time.Since(downSince).Round(time.Second)).
			Msg("Paddles: DCS-gRPC events back")
		downSince = time.Time{}
	}
	for {
		err := streamLSOGrades(ctx, addr, truth, connected)
		if ctx.Err() != nil {
			return
		}
		if downSince.IsZero() {
			downSince = time.Now()
			log.Warn().Err(err).Str("addr", addr).Msg("Paddles: DCS-gRPC events lost, reconnecting")
		} else {
			log.Debug().Err(err).Str("addr", addr).Msg("Paddles: DCS-gRPC still down")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

func streamLSOGrades(ctx context.Context, addr string, truth *lsoTruth, connected func()) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	stream, err := mission.NewMissionServiceClient(conn).StreamEvents(ctx, &mission.StreamEventsRequest{})
	if err != nil {
		return err
	}
	connected()
	for {
		ev, err := stream.Recv()
		if err != nil {
			return err
		}
		// A player's own landing, logged so a player trap shows whether DCS
		// sends players anything at all: AI traps get a grade, no player trap
		// on Day 16-20 Oct did (the grade is likely worked out on the
		// player's PC in multiplayer and never reaches the server's events).
		if land := ev.GetLand(); land != nil {
			if p := land.GetInitiator().GetUnit().GetPlayerName(); p != "" {
				log.Info().Str("player", p).Str("type", land.GetInitiator().GetUnit().GetType()).
					Str("place", land.GetPlace().GetName()).Msg("Paddles: DCS player landing event")
			}
		}
		lqm := ev.GetLandingQualityMark()
		if lqm == nil {
			continue
		}
		u := lqm.GetInitiator().GetUnit()
		cs := u.GetPlayerName()
		if i := strings.Index(cs, "|"); i >= 0 {
			cs = cs[:i]
		}
		cs = strings.TrimSpace(cs)
		if cs == "" {
			cs = u.GetName() // AI
		}
		grade, wire := parseDCSGrade(lqm.GetComment())
		log.Info().Str("callsign", cs).Str("type", u.GetType()).Str("place", lqm.GetPlace().GetName()).
			Str("grade", grade).Int("wire", wire).Str("comment", lqm.GetComment()).Msg("Paddles: DCS LSO grade")
		truth.grade(cs, lqm.GetComment(), time.Now())
	}
}
