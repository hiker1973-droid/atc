package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// JTAC: Hitman 1-1 on 61.000 FM. Command sends a flight to the IP of a 9-line
// card (command_tasking.go) and hands it over; the JTAC passes the card, hears
// the run called and clears it hot - or waves it off - from the Tacview
// picture against the card's geometry:
//
//	check-in        -> 9-line by reference to the card ("as fragged"), and
//	                   where the IP is if the jet is not near it
//	"copy"          -> "report IP inbound"
//	"IP inbound"    -> the jet must be near the IP, pointed at the target
//	"in from ..."   -> inside the final attack heading and in range: cleared
//	                   hot; outside it: abort
//	after cleared   -> the watch aborts a run that swings outside the window
//	                   before a weapon comes off
//	"off"           -> egress per line 9
//
// No picture of the caller (Tacview down, callsign not matched) means no
// checks: the exchange runs on the radio calls alone and still clears hot.
// Operator ruling 2026-10-14.

type jtacStage int

const (
	jtacAssigned jtacStage = iota // Command gave the IP; not checked in
	jtacBriefed                   // 9-line passed, readback not heard
	jtacReady                     // readback heard, waiting for IP inbound
	jtacInbound                   // IP inbound, waiting for the in call
	jtacCleared                   // cleared hot, the watch is running
	jtacOff                       // off target; next IP inbound re-attacks
)

const (
	// jtacIPNearNm: "at the IP" for check-in and IP inbound.
	jtacIPNearNm = 5.0
	// jtacInboundTolDeg: IP inbound heading must be within this of the
	// bearing to the target.
	jtacInboundTolDeg = 30.0
	// jtacInMinNm / jtacInMaxNm: range from the target an in call is taken at.
	jtacInMinNm = 0.5
	jtacInMaxNm = 15.0
	// The watch after cleared hot: how often, for how long, the slack outside
	// the card's window before it counts, and how many samples in a row.
	jtacWatchEvery    = 2 * time.Second
	jtacWatchFor      = 4 * time.Minute
	jtacWatchSlackDeg = 10.0
	jtacWatchSamples  = 2
	jtacWatchNoseDeg  = 45.0 // target further off the nose than this = pulling off, not attacking
	jtacWatchStopNm   = 1.5  // inside this the run is over either way
	jtacReleaseRadius = 1.5  // a weapon first seen this close to the jet is its release
	jtacContactMaxAge = 30 * time.Second
	jtacWhisperPrompt = "Hitman 1-1, Raider, Venom, Viper, checking in, ready for 9-line, copy, IP inbound, in from the north, in heading, off, abort, Anvil, Brick, Cobra, Dagger, Eagle"
)

// jtacPicture is what the JTAC needs from Tacview. *tacviewPositions is the
// live one; tests use a fake.
type jtacPicture interface {
	// Contact returns the caller's position and true heading when fresh.
	Contact(callsign string) (pos orb.Point, hdgTrue float64, ok bool)
	WeaponReleasedNear(pos orb.Point, radiusNm float64, since time.Time) bool
}

// tacviewJTAC adapts the Command Tacview store. A nil store is no picture.
type tacviewJTAC struct{ store *tacviewPositions }

func (t tacviewJTAC) Contact(cs string) (orb.Point, float64, bool) {
	if t.store == nil || cs == "" {
		return orb.Point{}, 0, false
	}
	pos, ok := t.store.Get(cs)
	if !ok {
		return orb.Point{}, 0, false
	}
	info := t.store.Info(cs)
	if !info.HasHeading || time.Since(info.UpdatedAt) > jtacContactMaxAge {
		return orb.Point{}, 0, false
	}
	return pos, info.HeadingDeg, true
}

func (t tacviewJTAC) WeaponReleasedNear(pos orb.Point, radiusNm float64, since time.Time) bool {
	return t.store != nil && t.store.WeaponReleasedNear(pos, radiusNm, since)
}

// ── Call recognition ─────────────────────────────────────────────────────────

var (
	// "in from the north", "in hot", "in heading 330", "rolling in", "Raider 11 in".
	jtacInRe    = regexp.MustCompile(`(?:^|[\s,])in(?:\s+(?:from|hot|heading|on|the|with)\b|[\s,.!]*$)|\brolling in\b`)
	jtacOffRe   = regexp.MustCompile(`\boff\b`)
	jtacAbortRe = regexp.MustCompile(`\babort`)
	jtacIPRe    = regexp.MustCompile(`\b(?:ip|i\.p\.|i p|eye pee)\b`)
)

func isJTACCheckIn(lower string) bool {
	return isTaskingRequest(lower) || containsAny(lower, "check in", "checking in", "check-in",
		"ready for 9", "ready for nine", "ready for the 9", "ready for the nine", "request 9", "request nine",
		"send the 9", "send the nine", "ready to copy", "send it")
}

func isJTACInbound(lower string) bool {
	if !strings.Contains(lower, "inbound") && !containsAny(lower, "leaving the ip", "departing the ip", "leaving ip", "departing ip") {
		return false
	}
	return true
}

// ── The exchange ─────────────────────────────────────────────────────────────

// JTACResponse answers one call on the JTAC net. Returns "" for calls that are
// not the JTAC's. name is how the JTAC calls itself ("Hitman one one").
func (b *taskingBoard) JTACResponse(text, callsign, name string, pic jtacPicture) string {
	lower := strings.ToLower(text)
	key := strings.ToLower(strings.TrimSpace(callsign))
	if key == "" {
		return ""
	}
	say := func(format string, a ...interface{}) string {
		return fmt.Sprintf("%s, %s, ", callsign, name) + fmt.Sprintf(format, a...)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	cur := b.flights[key]

	if containsAny(lower, "radio check", "comm check", "comms check", "how copy") {
		return say("loud and clear.")
	}
	if len(b.targets) == 0 {
		if isJTACCheckIn(lower) {
			return say("negative tasking at this time.")
		}
		return ""
	}

	// Check-in: pass the 9-line. A flight that skipped Command gets a target
	// here; one that finished its target and asks again gets the next one.
	if isJTACCheckIn(lower) {
		if cur == nil || cur.stage == jtacOff && isTaskingRequest(lower) {
			cur = &taskingAssignment{target: b.pick(key, now)}
			b.flights[key] = cur
		}
		cur.issued, cur.awaiting, cur.stage = now, false, jtacBriefed
		t := b.targets[cur.target]
		resp := say("%s control, 9-line target %d, %s, as fragged. Elevation %d, final attack heading %s to %s, egress back to %s, %s.",
			t.Control, t.Number, t.Name, t.ElevFt, spokenBearing(float64(t.FAHLow)), spokenBearing(float64(t.FAHHigh)), t.IP, t.Egress)
		if pos, _, ok := pic.Contact(callsign); ok {
			if brg, nm := bearingRangeFromRef(pos, t.IPPos[0], t.IPPos[1]); nm > jtacIPNearNm {
				resp += fmt.Sprintf(" %s bears %s for %d.", t.IP, spokenBearing(toMagnetic(brg, t.MagVar)), int(nm+0.5))
			}
		}
		return resp + " Read back."
	}

	if cur == nil || cur.stage == jtacAssigned {
		// Talking to the JTAC without checking in.
		if jtacInRe.MatchString(lower) || isJTACInbound(lower) {
			return say("negative, no 9-line passed. Check in for tasking.")
		}
		return ""
	}
	t := b.targets[cur.target]

	switch {
	case jtacAbortRe.MatchString(lower):
		cur.stage = jtacReady
		return say("copy abort. Report IP inbound from %s for re-attack.", t.IP)

	case jtacOffRe.MatchString(lower):
		if cur.stage < jtacInbound {
			return ""
		}
		cur.stage = jtacOff
		return say("copy off. Egress back to %s, %s. Report IP inbound for re-attack.", t.IP, t.Egress)

	case isJTACInbound(lower):
		pos, hdg, ok := pic.Contact(callsign)
		if ok {
			brgIP, nmIP := bearingRangeFromRef(pos, t.IPPos[0], t.IPPos[1])
			brgT, _ := bearingRangeFromRef(pos, t.TgtPos[0], t.TgtPos[1])
			if nmIP > jtacIPNearNm {
				log.Info().Str("callsign", callsign).Float64("ipNm", nmIP).Msg("JTAC: IP inbound refused, not at the IP")
				return say("negative, you are not at %s. %s bears %s for %d. Report IP inbound from %s.",
					t.IP, t.IP, spokenBearing(toMagnetic(brgIP, t.MagVar)), int(nmIP+0.5), t.IP)
			}
			if angleDiff(hdg, brgT) > jtacInboundTolDeg {
				cur.stage = jtacInbound
				return say("continue. Check heading, target bears %s.", spokenBearing(toMagnetic(brgT, t.MagVar)))
			}
		}
		cur.stage = jtacInbound
		return say("continue.")

	case jtacInRe.MatchString(lower):
		pos, hdg, ok := pic.Contact(callsign)
		if !ok {
			log.Info().Str("callsign", callsign).Msg("JTAC: no picture of the caller, cleared hot unchecked")
			b.clear(cur, now)
			return say("cleared hot.")
		}
		_, nm := bearingRangeFromRef(pos, t.TgtPos[0], t.TgtPos[1])
		mag := toMagnetic(hdg, t.MagVar)
		log.Info().Str("callsign", callsign).Float64("tgtNm", nm).Float64("hdgMag", mag).
			Int("fahLow", t.FAHLow).Int("fahHigh", t.FAHHigh).Msg("JTAC: in call")
		switch {
		case nm > jtacInMaxNm:
			cur.stage = jtacReady
			return say("abort, abort, abort. You are %d miles from the target. Report IP inbound from %s.", int(nm+0.5), t.IP)
		case !inWindow(mag, float64(t.FAHLow), float64(t.FAHHigh), 0):
			cur.stage = jtacReady
			return say("abort, abort, abort. Outside final attack heading, %s to %s. Re-attack from %s.",
				spokenBearing(float64(t.FAHLow)), spokenBearing(float64(t.FAHHigh)), t.IP)
		case nm < jtacInMinNm:
			cur.stage = jtacReady
			return say("abort, abort, abort. Too close. Re-attack from %s.", t.IP)
		}
		b.clear(cur, now)
		return say("cleared hot.")

	case isTaskingAck(lower) || strings.Contains(lower, fmt.Sprint(t.ElevFt)):
		if cur.stage != jtacBriefed {
			return ""
		}
		cur.stage = jtacReady
		return say("good readback. Report IP inbound.")
	}
	return ""
}

func (b *taskingBoard) clear(cur *taskingAssignment, now time.Time) {
	cur.stage, cur.clearedAt, cur.offAxis = jtacCleared, now, 0
}

// jtacWatch runs once per tick: any flight cleared hot that swings outside the
// attack window while still pointed at the target, before a weapon comes off
// it, is aborted. Returns the aborts to transmit.
func (b *taskingBoard) jtacWatch(name string, pic jtacPicture) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	var out []string
	for key, cur := range b.flights {
		if cur.stage != jtacCleared {
			continue
		}
		if now.Sub(cur.clearedAt) > jtacWatchFor {
			cur.stage = jtacOff
			continue
		}
		pos, hdg, ok := pic.Contact(key)
		if !ok {
			continue
		}
		t := b.targets[cur.target]
		if pic.WeaponReleasedNear(pos, jtacReleaseRadius, cur.clearedAt) {
			log.Info().Str("callsign", key).Msg("JTAC: release seen, watch ends")
			cur.stage = jtacOff
			continue
		}
		brgT, nm := bearingRangeFromRef(pos, t.TgtPos[0], t.TgtPos[1])
		if nm < jtacWatchStopNm {
			cur.stage = jtacOff
			continue
		}
		mag := toMagnetic(hdg, t.MagVar)
		if inWindow(mag, float64(t.FAHLow), float64(t.FAHHigh), jtacWatchSlackDeg) || angleDiff(hdg, brgT) > jtacWatchNoseDeg {
			cur.offAxis = 0
			continue
		}
		cur.offAxis++
		if cur.offAxis < jtacWatchSamples {
			continue
		}
		log.Info().Str("callsign", key).Float64("hdgMag", mag).Float64("tgtNm", nm).Msg("JTAC: run outside the attack window, abort")
		cur.stage, cur.offAxis = jtacReady, 0
		out = append(out, fmt.Sprintf("%s, %s, abort, abort, abort. Outside final attack heading. Re-attack from %s.",
			displayCallsign(key), name, t.IP))
	}
	return out
}

// runJTACWatch ticks jtacWatch and transmits its aborts.
func runJTACWatch(ctx context.Context, b *taskingBoard, name string, pic jtacPicture, tx func(string)) {
	tk := time.NewTicker(jtacWatchEvery)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			for _, text := range b.jtacWatch(name, pic) {
				tx(text)
			}
		}
	}
}

// displayCallsign title-cases a board key ("raider 11" -> "Raider 11").
func displayCallsign(key string) string {
	words := strings.Fields(key)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// jtacCallsign trims what the generic extractor leaves after a JTAC call's
// callsign: "Raider 11 in from the north" -> "Raider 11".
func jtacCallsign(text string) string {
	cs := extractCallsignSkippingAddress(text, "hitman one-one", "hitman one one", "hitman 1-1", "hitman 1 1", "hitman 11", "hitman")
	lower := strings.ToLower(cs)
	cut := len(cs)
	for _, v := range []string{" in ", " ip", " off", " abort", " at ", " rolling", " tally", " visual", " good "} {
		if i := strings.Index(lower, v); i > 0 && i < cut {
			cut = i
		}
	}
	if strings.HasSuffix(lower[:cut], " in") {
		cut -= 3
	}
	return strings.TrimSpace(cs[:cut])
}

// angleDiff is the smallest difference between two headings, 0-180.
func angleDiff(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// inWindow reports whether hdg lies in the clockwise window low..high,
// widened by slack each side. Handles windows across north (350 to 030).
func inWindow(hdg, low, high, slack float64) bool {
	low, high = math.Mod(low-slack+720, 360), math.Mod(high+slack+720, 360)
	hdg = math.Mod(hdg+720, 360)
	if low <= high {
		return hdg >= low && hdg <= high
	}
	return hdg >= low || hdg <= high
}

// startJTAC runs the JTAC net beside Command, in the same process so both work
// one tasking board: Command assigns the target, the JTAC works it. store may
// be nil (no Tacview), which leaves the JTAC without its checks.
func startJTAC(ctx context.Context, apiKey string, store *tacviewPositions) {
	board := commandTaskingBoard()
	board.jtac = &jtacNet{Spoken: flagJTACSpoken, FreqMHz: flagJTACFreq}
	pic := tacviewJTAC{store: store}
	srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)
	tx := func(text string) {
		mp3, err := synthesizeSpeech(ctx, apiKey, text, commandVoice(flagJTACVoice))
		if err != nil {
			log.Error().Err(err).Msg("JTAC TTS failed")
			return
		}
		broadcastLog("tx", fmt.Sprintf("JTAC TX → %q", text))
		transmitExternalAudioFile(ctx, mp3, flagJTACFreq, flagJTACName, srsHost, srsPort, flagExternalAudio)
	}
	go voiceNetLoop(ctx, flagSRSAddr, flagJTACFreq, flagJTACName, apiKey, flagEAMPassword, flagJTACVoice, flagExternalAudio,
		jtacWhisperPrompt, func(text string) (string, string) {
			cs := jtacCallsign(text)
			return cs, board.JTACResponse(text, cs, flagJTACSpoken, pic)
		})
	go runJTACWatch(ctx, board, flagJTACSpoken, pic, tx)
	log.Info().Float64("freq", flagJTACFreq).Str("name", flagJTACName).Bool("tacview", store != nil).Msg("JTAC net starting")
}
