package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// JTAC: Hitman 1-1 on 61.000 FM, and Hitman 2-1 on 62.000 FM when
// --jtac2-freq is set (one range_north sector each). Command hands a flight asking for tasking to
// the JTAC (command_tasking.go); the JTAC draws one of the 9-line cards at
// random, passes it (IP included, as in the real flow), hears
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
	jtacAssigned jtacStage = iota // no 9-line passed yet
	jtacBriefed                   // 9-line passed, readback not heard
	jtacReady                     // readback heard, waiting for IP inbound
	jtacInbound                   // IP inbound, waiting for the in call
	jtacCleared                   // cleared hot, the watch is running
	jtacOff                       // off target; next IP inbound re-attacks
)

const (
	// jtacIPNearNm: "at the IP" for check-in - inside it the brief leaves out
	// the bearing and range to the IP.
	jtacIPNearNm = 5.0
	// jtacIPInboundNm: an IP inbound call is accepted this far from the IP
	// (operator 2026-10-15: "change IP inbound 20 miles"; was jtacIPNearNm).
	jtacIPInboundNm = 20.0
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
	// jtacWhisperCalls: the net's calls, between its callsign and its IP
	// names in jtacWhisperPrompt.
	jtacWhisperCalls = "Raider, Venom, Viper, checking in, request tasking, ready for 9-line, copy, IP inbound, in from the north, in heading, pickle, rifle, off, abort, request BDA"
)

// jtacPicture is what the JTAC needs from Tacview. *tacviewPositions is the
// live one; tests use a fake.
type jtacPicture interface {
	// Contact returns the caller's position and true heading when fresh.
	Contact(callsign string) (pos orb.Point, hdgTrue float64, ok bool)
	WeaponReleasedNear(pos orb.Point, radiusNm float64, since time.Time) bool
	// For the BDA (jtac_bda.go).
	GroundObjects() []tacviewGround
	Impacts(since time.Time) []tacviewImpact
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

func (t tacviewJTAC) GroundObjects() []tacviewGround {
	if t.store == nil {
		return nil
	}
	return t.store.GroundObjects()
}

func (t tacviewJTAC) Impacts(since time.Time) []tacviewImpact {
	if t.store == nil {
		return nil
	}
	return t.store.Impacts(since)
}

// ── Call recognition ─────────────────────────────────────────────────────────

var (
	// "in from the north", "in hot", "in heading 330", "rolling in", "Raider 11 in".
	jtacInRe    = regexp.MustCompile(`(?:^|[\s,])in(?:\s+(?:from|hot|heading|on|the|with)\b|[\s,.!]*$)|\brolling in\b`)
	jtacOffRe   = regexp.MustCompile(`\boff\b`)
	jtacAbortRe = regexp.MustCompile(`\babort`)
	// jtacReleaseCallRe: the pilot's release call, "pickle" for bombs, "rifle"
	// for a guided missile ("Hitman 1-1, Raider 302, pickle times 4").
	jtacReleaseCallRe = regexp.MustCompile(`\b(pickle|rifle)\b`)
	jtacIPRe    = regexp.MustCompile(`\b(?:ip|i\.p\.|i p|eye pee)\b`)
)

func isJTACCheckIn(lower string) bool {
	return isTaskingRequest(lower) || containsAny(lower, "check in", "checking in", "check-in",
		"ready for 9", "ready for nine", "ready for the 9", "ready for the nine", "request 9", "request nine",
		"send the 9", "send the nine", "ready to copy", "send it")
}

// isTargetReadback: the pilot reads back the target or the IP ("target 3",
// "Fuel Depot", "Cobra").
func isTargetReadback(lower string, t taskingTarget) bool {
	if strings.Contains(lower, "target "+strconv.Itoa(t.Number)) {
		return true
	}
	if w, ok := targetWords[t.Number]; ok && strings.Contains(lower, "target "+w) {
		return true
	}
	return containsAny(lower, strings.ToLower(t.IP), strings.ToLower(t.Name))
}

// targetWords spells the card numbers the way Whisper sometimes writes them.
var targetWords = map[int]string{1: "one", 2: "two", 3: "three", 4: "four", 5: "five"}

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

	if containsAny(lower, radioCheckPhrases...) {
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
		switch {
		case cur == nil:
			cur = &taskingAssignment{target: b.pick(key, now)}
			b.flights[key] = cur
		case cur.redraw || cur.stage >= jtacCleared && isTaskingRequest(lower):
			cur.target = b.pick(key, now)
		}
		cur.redraw = false
		cur.issued, cur.awaiting, cur.stage = now, false, jtacBriefed
		t := b.targets[cur.target]
		// Target and IP only (operator 2026-10-14: "Hitman just needs to read
		// the target not the whole 9 line ( we can do that later)"). The rest of
		// the card - control, elevation, attack headings, egress - is on the
		// pilot's kneeboard and still checked against the Tacview picture.
		resp := say("target %d, %s, IP %s.", t.Number, t.Name, t.IP)
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
	case !cur.effectsAsked.IsZero() && now.Sub(cur.effectsAsked) <= bdaEffectsWindow && !isBDARequest(lower) && pilotEffects(lower) != 0:
		// The answer to "say your observed effects".
		cur.effectsAsked = time.Time{}
		if pilotEffects(lower) > 0 {
			return say("copy, %s. %s Report ready for further tasking.", oneOf(b.intn, bdaCopyGood), oneOf(b.intn, bdaThanks))
		}
		return say("copy, no effects. "+oneOf(b.intn, bdaReattack), t.IP)

	case isBDARequest(lower):
		// Impacts since cleared hot, or since the brief when there was none: on
		// 2026-10-15 Raider 302 released twice with no in call and was told
		// "negative BDA, no attack yet" (16:10:10, 16:13:17).
		since := cur.clearedAt
		if since.IsZero() {
			since = cur.issued
		}
		imps := flightImpacts(pic.Impacts(since), callsign, t)
		if cur.clearedAt.IsZero() && len(imps) == 0 {
			return say("negative BDA, no attack yet.")
		}
		cur.bdaDone = true
		return b.bda(cur, callsign, name, t, pic, imps, now)

	case jtacAbortRe.MatchString(lower):
		cur.stage = jtacReady
		return say("copy abort. Report IP inbound from %s for re-attack.", t.IP)

	case jtacReleaseCallRe.MatchString(lower):
		// Operator 2026-10-15: "Raider 302, Hitman, copy pickle, will report
		// BDA". Only after cleared hot; the BDA itself goes out from the watch
		// (bdaDue) once the weapons are down.
		word := jtacReleaseCallRe.FindString(lower)
		switch {
		case cur.stage == jtacInbound:
			// Released off IP inbound without an in call (Raider 302 flew it
			// that way all sortie, 2026-10-15). Take the release as the attack:
			// acknowledge it and start the clock, so the automatic BDA comes.
			b.clear(cur, now)
			return say("copy %s, will report BDA.", word)
		case cur.stage < jtacCleared:
			// Not cleared - after an abort, or before IP inbound. Silence here
			// left the pilot calling "pickle" into a dead net (16:12:26, 16:12:35).
			return say("negative, you are not cleared. Re-attack from %s.", t.IP)
		}
		return say("copy %s, will report BDA.", word)

	case jtacOffRe.MatchString(lower):
		if cur.stage < jtacInbound {
			return ""
		}
		// Once per pass. On 2026-10-15 Hitman's own "copy off" came back on
		// 61.0 and was answered again every ~16 s for five minutes; own_echo.go
		// stops that, and this makes a second "off" harmless either way.
		if cur.offAcked {
			return ""
		}
		cur.offAcked = true
		cur.stage = jtacOff
		return say("copy off. Egress back to %s, %s. Report IP inbound for re-attack.", t.IP, t.Egress)

	case isJTACInbound(lower) && !b.inboundIsInCall(cur, lower, callsign, t, pic):
		pos, hdg, ok := pic.Contact(callsign)
		if ok {
			brgIP, nmIP := bearingRangeFromRef(pos, t.IPPos[0], t.IPPos[1])
			brgT, _ := bearingRangeFromRef(pos, t.TgtPos[0], t.TgtPos[1])
			if nmIP > jtacIPInboundNm {
				log.Info().Str("callsign", callsign).Float64("ipNm", nmIP).Msg("JTAC: IP inbound refused, not at the IP")
				return say("negative, you are not at %s. %s bears %s for %d. Report IP inbound from %s.",
					t.IP, t.IP, spokenBearing(toMagnetic(brgIP, t.MagVar)), int(nmIP+0.5), t.IP)
			}
			// Accepted up to 20 nm out, which can be well off the run-in line:
			// on 2026-10-15 Raider 302 ran in on 109 for Brick's 169 and was
			// aborted at the in call. Tell him the run-in up front.
			if magT := toMagnetic(brgT, t.MagVar); !inWindow(magT, float64(t.FAHLow), float64(t.FAHHigh), jtacWatchSlackDeg) {
				cur.stage = jtacInbound
				return say("continue. Run in heading %s from %s.", spokenBearing(float64(t.RunIn)), t.IP)
			}
			if angleDiff(hdg, brgT) > jtacInboundTolDeg {
				cur.stage = jtacInbound
				return say("continue. Check heading, target bears %s.", spokenBearing(toMagnetic(brgT, t.MagVar)))
			}
		}
		cur.stage = jtacInbound
		return say("continue.")

	case jtacInRe.MatchString(lower) || isJTACInbound(lower):
		// isJTACInbound only reaches here when inboundIsInCall said so.
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

	case isTaskingAck(lower) || isTargetReadback(lower, t):
		if cur.stage != jtacBriefed {
			return ""
		}
		// Our own brief heard back carries the target and the IP too. It leads
		// with the callsign and names Hitman mid-string ("Raider 302, Hitman
		// 1-1, target 3, ..."); a pilot leads with Hitman or leaves it out.
		// Seen live 2026-10-14 11:48:39: an unknown SRS origin returned the
		// brief word for word and was taken as a good readback.
		if i := strings.Index(lower, "hitman"); i > 0 {
			log.Info().Str("callsign", callsign).Str("text", text).Msg("JTAC: readback names Hitman mid-call, treated as our own echo")
			return ""
		}
		cur.stage = jtacReady
		return say("good readback. Report IP inbound.")
	}
	return ""
}

// inboundIsInCall: a bare "inbound" (no "IP") from a flight that has already
// called IP inbound is its in call. Raider 302 flew that way on 2026-10-15
// ("Hitman 1-1, Raider 302, inbound", 16:10:31 and 16:11:38) and got
// "continue" each time, never cleared hot. Only near the target (inside
// jtacInMaxNm) or with no picture to check, so a repeated IP inbound from
// the IP is still answered as one.
func (b *taskingBoard) inboundIsInCall(cur *taskingAssignment, lower, callsign string, t taskingTarget, pic jtacPicture) bool {
	if cur.stage != jtacInbound || jtacIPRe.MatchString(lower) {
		return false
	}
	pos, _, ok := pic.Contact(callsign)
	if !ok {
		return true
	}
	_, nm := bearingRangeFromRef(pos, t.TgtPos[0], t.TgtPos[1])
	return nm <= jtacInMaxNm
}

func (b *taskingBoard) clear(cur *taskingAssignment, now time.Time) {
	cur.stage, cur.clearedAt, cur.offAxis, cur.bdaDone, cur.offAcked = jtacCleared, now, 0, false, false
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
		if text := b.bdaDue(key, cur, name, pic, now); text != "" {
			out = append(out, text)
		}
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
	cs := extractCallsignSkippingAddress(text, "hitman one-one", "hitman one one", "hitman 1-1", "hitman 1 1", "hitman 11",
		"hitman two-one", "hitman two one", "hitman 2-1", "hitman 2 1", "hitman 21", "hitman")
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
	return joinCallsignDigits(strings.TrimSpace(cs[:cut]))
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

// jtacConfig is one JTAC net as the flags describe it.
type jtacConfig struct {
	FreqMHz float64
	Name    string // SRS client name, "Hitman-1-1"
	Spoken  string // "Hitman one one"
	Sector  string // with two JTACs, the range_north sector this one runs
	Voice   voiceProfile
}

// jtacConfigs: Hitman 1-1 (--jtac-freq), plus Hitman 2-1 when --jtac2-freq is set.
func jtacConfigs() []jtacConfig {
	cfgs := []jtacConfig{{FreqMHz: flagJTACFreq, Name: flagJTACName, Spoken: flagJTACSpoken, Sector: flagJTACSector, Voice: jtacVoice()}}
	if flagJTAC2Freq > 0 {
		cfgs = append(cfgs, jtacConfig{FreqMHz: flagJTAC2Freq, Name: flagJTAC2Name, Spoken: flagJTAC2Spoken, Sector: flagJTAC2Sector, Voice: jtac2Voice()})
	}
	return cfgs
}

// startJTACs runs the JTAC nets beside Command, in the same process so they
// share the tasking: Command hands the flight over, the JTAC works it. One
// JTAC works Command's board, the whole range; with two, each gets its own
// board of its sector's targets and Command picks between them
// (taskingBoard.chooseJTAC). store may be nil (no Tacview), which leaves the
// JTACs without their checks.
func startJTACs(ctx context.Context, apiKey string, store *tacviewPositions) {
	board := commandTaskingBoard()
	cfgs := jtacConfigs()
	for _, c := range cfgs {
		n := &jtacNet{Spoken: c.Spoken, FreqMHz: c.FreqMHz}
		if len(cfgs) > 1 {
			ts := sectorTargets(board.targets, c.Sector)
			if len(ts) == 0 {
				log.Warn().Str("jtac", c.Name).Str("sector", c.Sector).Msg("JTAC sector has no targets on this map - giving it the whole range")
				ts = board.targets
			}
			n.board = newTaskingBoard(ts)
		}
		board.jtacs = append(board.jtacs, n)
		startJTAC(ctx, apiKey, store, c, n.workBoard(board))
	}
}

// startJTAC runs one JTAC net working board.
func startJTAC(ctx context.Context, apiKey string, store *tacviewPositions, c jtacConfig, board *taskingBoard) {
	pic := tacviewJTAC{store: store}
	srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)
	tx := func(text string) {
		mp3, err := synthesizeSpeech(ctx, apiKey, text, c.Voice)
		if err != nil {
			log.Error().Err(err).Str("jtac", c.Name).Msg("JTAC TTS failed")
			return
		}
		broadcastLog("tx", fmt.Sprintf("JTAC TX → %q", text))
		// Same message as a reply so the tail shows the watch's BDA and aborts;
		// before 2026-10-15 they went to the dashboard only.
		log.Info().Str("text", text).Float64("freq", c.FreqMHz).Msg("Command TX")
		noteOwnTX(c.FreqMHz, text)
		transmitExternalAudioFile(ctx, mp3, c.FreqMHz, c.Name, srsHost, srsPort, flagExternalAudio)
	}
	go voiceNetLoop(ctx, flagSRSAddr, c.FreqMHz, c.Name, apiKey, flagEAMPassword, c.Voice, flagExternalAudio,
		jtacWhisperPrompt(c.Name, board.targets), nil, func(text string) (string, string) {
			cs := jtacCallsign(text)
			return cs, board.JTACResponse(text, cs, c.Spoken, pic)
		})
	go runJTACWatch(ctx, board, c.Spoken, pic, tx)
	log.Info().Float64("freq", c.FreqMHz).Str("name", c.Name).Str("sector", c.Sector).Int("targets", len(board.targets)).
		Bool("tacview", store != nil).Msg("JTAC net starting")
}

// jtacWhisperPrompt biases Whisper towards the net's calls: its own callsign
// as pilots write it ("Hitman-2-1" -> "Hitman 2-1") and its IP names.
func jtacWhisperPrompt(srsName string, targets []taskingTarget) string {
	ips := make([]string, len(targets))
	for i, t := range targets {
		ips[i] = t.IP
	}
	return strings.Replace(srsName, "-", " ", 1) + ", " + jtacWhisperCalls + ", " + strings.Join(ips, ", ")
}

// bdaDue returns the flight's BDA once its weapons are down and have settled,
// once per pass. Caller holds b.mu.
func (b *taskingBoard) bdaDue(key string, cur *taskingAssignment, name string, pic jtacPicture, now time.Time) string {
	if cur.bdaDone || cur.clearedAt.IsZero() || now.Sub(cur.clearedAt) > bdaWindow {
		return ""
	}
	t := b.targets[cur.target]
	cs := displayCallsign(key)
	imps := flightImpacts(pic.Impacts(cur.clearedAt), cs, t)
	if len(imps) == 0 {
		return ""
	}
	last := imps[0].At
	for _, i := range imps[1:] {
		if i.At.After(last) {
			last = i.At
		}
	}
	if now.Sub(last) < bdaSettle {
		return ""
	}
	cur.bdaDone = true
	log.Info().Str("callsign", cs).Int("impacts", len(imps)).Int("target", t.Number).Msg("JTAC: BDA")
	return b.bda(cur, cs, name, t, pic, imps, now)
}

// bda builds the flight's BDA call; one that could not assess leaves the
// flight's own effects report awaited. Caller holds b.mu.
func (b *taskingBoard) bda(cur *taskingAssignment, cs, name string, t taskingTarget, pic jtacPicture, imps []tacviewImpact, now time.Time) string {
	text, out := bdaFor(cs, name, t, pic.GroundObjects(), imps, b.intn)
	cur.effectsAsked = time.Time{}
	if out == bdaUnable {
		cur.effectsAsked = now
	}
	return text
}

// oneOf picks one of the wordings. The modulo keeps a test's fixed intn in range.
func oneOf(pick func(int) int, opts []string) string { return opts[pick(len(opts))%len(opts)] }

// jtacVoice is Hitman's TTS voice: his own delivery (a ground party on a
// manpack, not a controller at a console), jtacSpeedUp over the common speed.
func jtacVoice() voiceProfile {
	return voiceProfile{voice: flagJTACVoice, speed: flagTTSSpeed * jtacSpeedUp, instructions: flagVoiceStyleJTAC,
		eleven: elHard(flagElevenJTACVoice, "Hitman", jtacSpeedUp)}
}

// jtacSpeedUp: Hitman 1-1 8% quicker (operator 2026-10-16: "speed Hitman
// 1-1 up by 5-10%"; was 1.0). Hitman 2-1 keeps Command's read.
const jtacSpeedUp = 1.08

// jtac2Voice is Hitman 2-1's: the read the operator picked it on (round 18,
// Command's settings), an American OpenAI fallback.
func jtac2Voice() voiceProfile {
	ev := elCommand(flagElevenJTAC2Voice)
	ev.Name = "Hitman 2-1"
	return voiceProfile{voice: flagJTAC2Voice, speed: flagTTSSpeed, instructions: flagVoiceStyleJTAC2, eleven: ev}
}
