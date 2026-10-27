package main

import (
	"strconv"
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

// dmeDistanceRe matches "7 DME", "12 mile", "15 miles", "10 mile DME".
// Anchored on the unit word so unrelated digits (channels, headings) don't
// trigger a false position-report.
var dmeDistanceRe = regexp.MustCompile(`\b(\d{1,3})[\s\-]+(?:dme|miles?(?:\s+dme)?)\b`)

// rangeDistanceRe matches "range 25" — the pilot's answer to the see-you-at-ten
// reply "report your altitude and range" ("angels 9, range 25", DCS3
// 2026-10-14). Without it Marshal asked for a report it could not hear.
var rangeDistanceRe = regexp.MustCompile(`\brange[\s,]+(\d{1,3})\b`)

var dmeWordNums = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"twelve": 12, "fifteen": 15, "twenty": 20, "thirty": 30,
}

// extractDMEDistance parses a DME distance from "Marshal, Raider 39, 7 DME"
// or word forms like "seven miles". Returns 0 when no recognizable distance
// is present.
func extractDMEDistance(lower string) int {
	for _, re := range []*regexp.Regexp{dmeDistanceRe, rangeDistanceRe} {
		if m := re.FindStringSubmatch(lower); m != nil {
			var v int
			fmt.Sscanf(m[1], "%d", &v)
			return v
		}
	}
	for word, val := range dmeWordNums {
		if strings.Contains(lower, word+" dme") || strings.Contains(lower, word+" mile") {
			return val
		}
	}
	return 0
}

// isDepartureClearCall recognises the check-in a *departing* aircraft makes on
// the Marshal freq after Deckboss pushes them over — "Union Marshal, Raider 32,
// clear seven miles".
//
// This has to be tested ahead of the inbound " dme"/" mile" case, which matches
// the same distance shape and would otherwise answer a jet on its way out with
// "radar contact, seven DME, continue inbound".
//
// The discriminator is the word "clear": aircraft recovering to the boat report
// a bare distance ("Marshal, Raider 39, seven DME"), while a departure reports
// being clear of the zone. Any inbound-phase keyword vetoes the match, so a
// recovery call that happens to contain "clear" can't be read as a departure —
// the veto is what keeps this safe, since "clear" on its own is a common word.
func isDepartureClearCall(lower string) bool {
	if !strings.Contains(lower, "clear") {
		return false
	}
	if containsAny(lower, "marking mom", "marking moms", "commencing", "platform",
		"initial", "established", "see you at", "paddles", "bolter", "state") {
		return false
	}
	// Either a distance ("clear seven miles") or an explicit clear-of phrasing
	// ("clear of mother", or the flown "clear mothers" with no "of").
	return containsAny(lower, " mile", " dme", "clear of", "outbound",
		"clear mother", "clear mom", "clear the boat")
}

// marshalAddressRe matches the address a pilot opens a Marshal call with.
// Whisper rarely hears "Union" cleanly: one pilot on 2026-09-14 came out as
// "Unit Marshall", "Here Marshall", "In here, Marshall", "Leader, Marshal" and
// "You to Marshall", and on 2026-09-17 as "You there, Marshall", "You did
// Marshal", "Inner Marshall" and "Ma- either Marshal". So up to two short filler words may precede
// "Marshal"/"Marshall". The filler list is closed on purpose — our own TX comes
// back as "<callsign>, Union Marshal, …" and a callsign is never one of these
// words, so the echo still fails to match. Up to three since DCS3 2026-10-17
// 12:56:19 "You and your marshal, Raider 312, angels 2000, 8.8 DME".
//
// "Junior Marshall" (Day 2026-10-23 17:10:28) and "Senior Marshal" (2026-10-24
// 08:01:51) are the same "Union", both answering "say state".
//
// "You're in Marshall, Raider 319, platform" (Night 2026-10-25 14:35:56) was
// dropped: the apostrophe splits off "re", now a filler too.
var marshalAddressRe = regexp.MustCompile(`(?i)^\W*(?:(?:union|unit|onion|junior|senior|here|hear|in|inner|leader|you|your|re|youre|to|there|did|either|ma|the|hey|uh|um|okay|ok|and|so|need)\W+){0,3}marshall?\b\W*`)

// unaddressedStackCallsign accepts one call that skips the address: a
// callsign-led "Raider 312, established, angels 2, entry point 3, state 6.5"
// (DCS3 2026-10-17 12:57:14 - dropped, so the jet never got Charlie). Only an
// established/entering-stack call, and never one that names Marshal: every
// Marshal TX is "<callsign>, Union Marshal, ...", so its echo stays out.
func unaddressedStackCallsign(text string) string {
	lower := strings.ToLower(text)
	// The ack's own words too, in case Whisper garbles "Union Marshal" in an echo.
	if containsAny(lower, "marshal", "union", "onion", "copy", "roger", "charlie", "hold for", "in the stack") ||
		!establishedRe.MatchString(lower) {
		return ""
	}
	m := callsignTokenRe.FindStringSubmatchIndex(text)
	if m == nil || strings.TrimLeft(text[:m[0]], " ,.") != "" || notCallsignWords[strings.ToLower(text[m[2]:m[3]])] {
		return ""
	}
	return marshalCallsignFromText("Marshal, " + text)
}

// splitMarshalAddress reports whether text opens with the Marshal address and
// returns what follows it.
func splitMarshalAddress(text string) (rest string, ok bool) {
	loc := marshalAddressRe.FindStringIndex(text)
	if loc == nil {
		return "", false
	}
	return text[loc[1]:], true
}

// callsignTokenRe finds a "<word> <number>" callsign — "Raider 331",
// "Raider033", "Venom 2-1-2".
var callsignTokenRe = regexp.MustCompile(`(?i)\b([a-z]{3,})[\s-]*(\d(?:[\s-]?\d){0,3})\b`)

// notCallsignWords are words that precede a number in a Marshal call without
// being a callsign ("marking moms 350", "angels 16", "state 3.5").
var notCallsignWords = map[string]bool{
	"angels": true, "angel": true, "state": true, "mom": true, "moms": true,
	"mother": true, "mothers": true, "for": true, "dme": true, "mile": true,
	"miles": true, "bearing": true, "radial": true, "channel": true,
	"button": true, "case": true, "runway": true, "brc": true, "heading": true,
	"altimeter": true, "see": true, "you": true, "ten": true, "marshal": true,
	"marshall": true, "platform": true, "passing": true, "cat": true,
	"minutes": true, "plus": true, "minus": true, "point": true,
}

// marshalCallsignFromText pulls the caller's callsign out of a Marshal call:
// the first "<word> <number>" after the address, so "Marshal, see you at ten.
// Raider 331, see you at ten." yields "Raider 331" rather than "see you at
// ten." Returns "" when the call isn't addressed to Marshal or carries no
// "<word> <number>" at all — squadron callsigns always have the number, and a
// numberless guess ("signal Charlie") would be answered and echo back.
func marshalCallsignFromText(text string) string {
	rest, ok := splitMarshalAddress(text)
	if !ok {
		return ""
	}
	rest = normalizeCallsignLocal(rest)
	for _, m := range callsignTokenRe.FindAllStringSubmatch(rest, -1) {
		if notCallsignWords[strings.ToLower(m[1])] {
			continue
		}
		digits := strings.NewReplacer(" ", "", "-", "").Replace(m[2])
		word := strings.ToUpper(m[1][:1]) + m[1][1:]
		return word + " " + digits
	}
	return ""
}

// marshalIntent is what a Marshal call asks for.
type marshalIntent int

const (
	marshalNone marshalIntent = iota
	marshalRadioCheck
	marshalRadarCheck
	marshalBRCRequest
	marshalMarkingMom
	marshalSeeYouAtTen
	marshalDepartureClear
	marshalDME
	marshalState
	marshalEstablished
	marshalCommencing
	marshalPlatform
	marshalPushCommand
	marshalPushPaddles
	marshalInitial
	marshalCheckIn
	// marshalReadback is a readback or a plain "copy": no reply, and not an
	// intent miss in the logs.
	marshalReadback
	// Case III approach reports (marshal_case3.go): needles, coupled,
	// command control.
	marshalNeedles
	marshalACLSReport
	// marshalPushTime is a holding jet asking for its approach time
	// ("requesting push time", Night 2026-10-25 18:05:09, an intent miss).
	marshalPushTime
	// marshalBolter and marshalAbeam: the Case III bolter pattern
	// (marshal_case3.go).
	marshalBolter
	marshalAbeam
)

// pushTimeRequestRe is a request for the Case III approach time. A readback
// ("push time 01") has no request word, so it stays a readback.
var pushTimeRequestRe = regexp.MustCompile(`\b(?:request(?:ing)?|say(?: again)?|need|confirm|what(?:'s| is))\s+(?:(?:my|our|the|a|new|an)\s+)?(?:push|approach|commence|commencement|eat)(?:\s+time)?\b|\b(?:push|approach) time\s*\?`)

// case3ReadbackRe is the pilot reading back a Case III marshal: "marshal on
// the 263, 21 DME, angels 6, expected approach time 23" - it holds a DME, so
// without this it read as a DME report.
var case3ReadbackRe = regexp.MustCompile(`\b(?:approach time|commence at|push time|marshal on the)\b`)

// establishedRe covers "established angels 2" and Whisper's flown variants
// "establish angels 2" and "established, angels two", plus "entering stack
// position 3, angels 2" (flown on DCS3 2026-10-14, missed twice). Our own ack
// "<callsign>, Union Marshal, entering stack at angels ..." leads with the
// callsign, so the address guard still drops it as an echo.
//
// "Raider 303 overhead, angels 2" (Day 2026-10-20 19:02:00, an intent miss) is
// the same report.
var establishedRe = regexp.MustCompile(`\bestablish(?:ed)?\b|\bentering (?:the )?stack\b|\boverhead\b.*\bangels?\b`)

// brcWordRe catches a BRC question however it's phrased: "BRC", "B R C",
// "B.R.C.", and Whisper's "VRC".
var brcWordRe = regexp.MustCompile(`\b(?:brc|vrc|b\.?\s?r\.?\s?c)\b`)

// brcReadbackRe is a BRC word followed by a heading: the pilot reading the BRC
// back, not asking for it ("understands BRC 096, altimeter 3000").
var brcReadbackRe = regexp.MustCompile(`\b(?:brc|vrc)\b,? (?:is )?\d{2,3}\b`)

// charlieRequestRe is a holding jet asking for Charlie.
var charlieRequestRe = regexp.MustCompile(`\b(?:request|requesting|ready for|say) charlie\b`)

// readbackSeeYouRe is the pilot reading back "report see me at ten" however
// Whisper punctuates it (Day 2026-10-18 14:40:25 "Will report. See you at
// 10." was answered "copy visual at ten miles" at 67 nm; 10:09 "I'll call you
// at 10."). Matched on the call with punctuation turned to spaces.
var readbackSeeYouRe = regexp.MustCompile(`\b(?:report|wilco|call)\s+(?:see\s+)?(?:you|me)\s+at\s+(?:10|ten)\b`)

// ackOnlyRe is a call that ends in a bare acknowledgement: "Raider 302,
// copy." / "copy signal Charlie" (Day 2026-10-18 14:46:26, 15:57:40).
var ackOnlyRe = regexp.MustCompile(`\b(?:copy|copies|roger|wilco)(?:\s+(?:signal\s+)?charlie)?\s*$`)

// bareStateRe is a state with no "state": the modex, then the fuel, and
// nothing after (Day 2026-10-18 14:45:31 "Union Marshall Raider 302, 6.6."
// answering "say state").
var bareStateRe = regexp.MustCompile(`\b\d{3}\s+(\d{1,2}(?:\.| point )\d)\s*$`)

// marshalFlat is a lowercased call with its punctuation turned to spaces,
// except a decimal point inside a number ("6.6").
func marshalFlat(lower string) string {
	var b strings.Builder
	rs := []rune(lower)
	for i, r := range rs {
		switch {
		case r == '.' && i > 0 && i+1 < len(rs) && unicode.IsDigit(rs[i-1]) && unicode.IsDigit(rs[i+1]):
			b.WriteRune(r)
		case r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':':
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// bareMarshalState is the fuel in a state given without the word "state", or 0.
func bareMarshalState(lower string) float64 {
	m := bareStateRe.FindStringSubmatch(marshalFlat(lower))
	if m == nil {
		return 0
	}
	var v float64
	if _, err := fmt.Sscanf(strings.Replace(m[1], " point ", ".", 1), "%f", &v); err != nil || v <= 0 || v >= 30 {
		return 0
	}
	return v
}

// unlabelledMarkingMomRe is "<radial> for <range>" with no "marking mom's".
var unlabelledMarkingMomRe = regexp.MustCompile(`\b\d{3} for \d{1,3}\b`)

// classifyMarshalCall maps a lowercased, address-led call to its intent. Order
// matters and mirrors the original switch: e.g. the departure check must beat
// the DME report, and a BRC word inside a marking-mom's call is still a
// check-in.
func classifyMarshalCall(lower string, fuelState float64) marshalIntent {
	flat := marshalFlat(lower)
	switch {
	case containsAny(lower, "radio check", "comm check", "comms check", "com check", "comp check", "comcheck", "how copy"):
		return marshalRadioCheck
	case containsAny(lower, "radar check", "radar contact", "request radar", "radar service"):
		return marshalRadarCheck
	case containsAny(lower, "say brc", "request brc", "brc check", "check brc", "confirm brc", "current brc",
		"what's brc", "what is brc", "say bearing", "say mother's heading", "mother's heading", "mothers heading",
		"say heading of mother", "base recovery course"):
		return marshalBRCRequest
	case containsAny(lower, "marking mom", "marking moms", "marking mum", "marking bomb", "marking bombs", "mark your mom", "mark your mum",
		// DCS3 2026-10-15 14:53:37: "marker mom's 300 for 27" got only "state 7.6, copy".
		"marker mom", "marker mum", "marker bomb"):
		return marshalMarkingMom
	case containsAny(lower, "report see you at", "report see me at", "wilco see you at", "wilco see me at",
		// DCS3 2026-10-17 13:35:03 at 72 nm: "BRC 08096, Angels stack at
		// Angels 3, will call, see you at 10" got "roger visual on mother".
		"will call see you at", "will call, see you at", "will call you at", "call you at 10", "call you at ten"):
		// The pilot reading back "report see me at ten", not making the call.
		// DCS3 2026-10-15 14:54:27: "BRC 96, Marshalling at Angels 2, we'll
		// report see you at 10" at 29 nm was answered "copy visual at ten
		// miles". No reply: the readback needs none.
		return marshalReadback
	case readbackSeeYouRe.MatchString(flat):
		return marshalReadback
	case containsAny(lower, "see you at 10", "see you at ten", "see me at 10", "see me at ten"):
		return marshalSeeYouAtTen
	case isDepartureClearCall(lower):
		return marshalDepartureClear
	case containsAny(lower, "pushing command", "switching command", "push command", "switch command",
		"pushing strike", "switching strike",
		// Day 2026-10-18 06:52:02: "Raider 303 leaving airspace, signing out
		// for command" was an intent miss.
		"leaving airspace", "leaving the airspace", "signing out", "signing off", "checking out"):
		// After the departure-clear call ("clear Mother's, push command" is
		// that) but ahead of the DME report: "seven miles out, pushing command"
		// is a jet leaving, and was answered "contact 7 DME, continue" (DCS3
		// 2026-10-15 14:50:46). A bare distance is still a DME report.
		return marshalPushCommand
	case establishedRe.MatchString(lower) || charlieRequestRe.MatchString(lower):
		// Ahead of the DME and state reports: DCS3 2026-10-17 10:52 "entering
		// stack position 3, fuel state 4.9" and "established angels 2, entry
		// point 1, state 5.3" were both answered "state, copy", so neither jet
		// was ever holding and the Charlie watcher had nobody to signal.
		// "request Charlie" (10:53:18, an intent miss) is the same call.
		return marshalEstablished
	case pushTimeRequestRe.MatchString(lower):
		return marshalPushTime
	case bolterCallRe.MatchString(lower):
		return marshalBolter
	case strings.Contains(lower, "abeam"):
		return marshalAbeam
	case containsAny(lower, "initial"):
		// Ahead of the DME report: "three mile initial" (DCS3 2026-10-17
		// 11:04:31) got "contact 3 DME, continue" instead of paddles.
		return marshalInitial
	case case3ReadbackRe.MatchString(lower) && strings.Contains(lower, "angels"):
		return marshalReadback
	case needlesRe.MatchString(lower):
		return marshalNeedles
	case containsAny(lower, "coupled", "command control"):
		return marshalACLSReport
	case containsAny(lower, "commencing"):
		// Ahead of the state report: the card's call is "commencing, altimeter
		// 30.16, state 9.5" (DimOn card 26), which read as a state report.
		return marshalCommencing
	case unlabelledMarkingMomRe.MatchString(lower) && strings.Contains(lower, "angels"):
		// "Raider 312 is 221 for 59, Angels 28, state 6.4" (DCS3 2026-10-17
		// 10:40:44) is a marking mom's without the words; it got "state, copy".
		return marshalMarkingMom
	case containsAny(lower, " dme", " mile") || rangeDistanceRe.MatchString(lower):
		return marshalDME
	case containsAny(lower, "state") && fuelState > 0:
		return marshalState
	case bareStateRe.MatchString(flat) && fuelState > 0:
		// "Raider 302, 6.6" answering "say state".
		return marshalState
	case containsAny(lower, "commencing"):
		return marshalCommencing
	case containsAny(lower, "platform", "passing five thousand", "passing 5000", "at platform"):
		return marshalPlatform
	case containsAny(lower, "pushing paddles", "switching paddles", "push paddles", "switch paddles",
		"pushing lso", "switching lso", "pushing button", "pushing channel"):
		return marshalPushPaddles
	case brcReadbackRe.MatchString(lower):
		// The pilot reading back "BRC 096 ... Marshal at angels 2" - DCS3
		// 2026-10-17 10:42:17 and 10:45:53 were both answered "BRC 096" again.
		return marshalReadback
	case brcWordRe.MatchString(lower):
		return marshalBRCRequest
	case containsAny(lower, "checking in", "check in", "checkin"):
		return marshalCheckIn
	case ackOnlyRe.MatchString(flat):
		return marshalReadback
	}
	return marshalNone
}

// marshalZoneName derives the control-zone name spoken in the departure release
// from the role callsign: "Union Marshal" → "Union". Keeps the zone and the
// controller named consistently off one constant instead of a second literal
// that could drift out of sync with it.
func marshalZoneName() string {
	if i := strings.Index(marshalCallsign, " "); i > 0 {
		return marshalCallsign[:i]
	}
	return ""
}

// marshalAngelsBand is the stack's altitude band for the recovery case:
// angels 2-9 overhead in Case 1 and 2, angels 6 and up in Case 3.
func marshalAngelsBand(c state.RecoveryCase) (lo, hi int) {
	if c == state.CaseThree {
		return marshalCase3MinAngels, marshalCase3MaxAngels
	}
	return marshalMinAngels, marshalMaxAngels
}

const (
	marshalCallsign = "Union Marshal"
	// Stack altitude band — Marshal assigns the lowest unoccupied angel in
	// [marshalMinAngels, marshalMaxAngels]. "Unoccupied" considers both stack
	// reservations and any Tacview contact within 50nm of the carrier.
	marshalMinAngels = 2
	marshalMaxAngels = 9
	// Case 3 stacks from angels 6 up, each at angels + 15 DME on the holding
	// radial (operator 2026-10-25: Night, Raider 319 was given "angels two"
	// for Case Three; "yes fix case 3").
	marshalCase3MinAngels = 6
	marshalCase3MaxAngels = 20
	// Case 3 EAT scheduling — first aircraft commences at now+lead, each
	// subsequent slot adds interval; lead gives the pilot time to enter the
	// hold. Interval 10 min (operator 2026-10-26, was 1 since 2026-10-25).
	marshalCase3LeadTimeMin = 10
	marshalCase3IntervalMin = 10
	// The least notice a new approach time gives a jet already in the hold.
	marshalCase3MinNotice = 2 * time.Minute
	// Final bearing = BRC - offset (deck-angle compensation so the lineup
	// lands on the angled runway, not the bow). Typical Case 3 offset is
	// 9-10° left of BRC.
	marshalFinalBearingOffset = 9
)

// magneticDeg converts a true bearing in whole degrees to magnetic for speech.
func magneticDeg(atcCtrl *controller.ATCController, trueDeg int) int {
	return int(math.Round(atcCtrl.ToMagnetic(float64(trueDeg)))) % 360
}

// marshalLoop handles the carrier marshal stack on a dedicated SRS frequency.
func marshalLoop(ctx context.Context, srsAddr string, freqMHz float64, apiKey, eamPassword, voice string,
	txCooldown *int64, atcCtrl *controller.ATCController, stack *state.MarshalStack) {

	comp := composer.NewATCComposer(marshalCallsign)

	transmit := func(text string) {
		log.Info().Str("text", text).Msg("Marshal TX")
		atomic.StoreInt64(txCooldown, time.Now().Add(estimateTTSDuration(text)).UnixNano())
		mp3, err := synthesizeSpeech(ctx, apiKey, text, marshalVoice(voice))
		if err != nil {
			log.Error().Err(err).Msg("Marshal TTS failed")
			return
		}
		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		transmitExternalAudioFile(ctx, mp3, freqMHz, "OMDM-MSH", srsHost, srsPort, flagExternalAudio)
	}

	// Recovery-case watcher — every 30s, recompute Case from live weather +
	// mission-time night flag. On transition, log it and (if anyone is in the
	// stack) broadcast an advisory call. Runs for the lifetime of the role.
	go func() {
		// Prime the state so the first tick doesn't fire a spurious transition
		// against the zero value.
		atcCtrl.RefreshRecoveryCase()
		// Compare against the case this watcher last saw, not RefreshRecoveryCase's
		// old value: dashboard /status polls and case overrides refresh the case
		// too, and would otherwise swallow every transition before this tick.
		last := atcCtrl.GetRecoveryCase()
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				_, current := atcCtrl.RefreshRecoveryCase()
				if current == last {
					continue
				}
				old := last
				last = current
				log.Info().Int("from", int(old)).Int("to", int(current)).Str("label", composer.CaseLabel(int(current))).Msg("Marshal: recovery case changed")
				if len(stack.GetAll()) > 0 {
					transmit(comp.MarshalCaseChange(composer.CaseLabel(int(current))))
				}
			}
		}
	}()

	go watchCharlie(ctx, stack, comp, transmit, atcCtrl) // marshal_charlie.go
	go watchCase3Paddles(ctx, comp, transmit, atcCtrl)   // marshal_case3.go

	// DEBUG: with --marshal-test-tx, transmit "test" every 30s to verify the
	// outbound SRS path independently of pilot audio. Leave the flag off in
	// production — every tick blocks the cooldown window for a few seconds.
	if flagMarshalTestTx {
		log.Warn().Msg("Marshal: --marshal-test-tx enabled, transmitting test every 30s")
		go func() {
			tk := time.NewTicker(30 * time.Second)
			defer tk.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tk.C:
					transmit("test")
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tcpConn, err := dialSRSKeepAlive(ctx, srsAddr)
		if err != nil {
			log.Debug().Msg("Marshal: SRS connect failed, retrying in 10s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		guidLen := 22
		guid := srsClientGUID("vsfg7msh", 14)
		if len(guid) > guidLen {
			guid = guid[:guidLen]
		}
		for len(guid) < guidLen {
			guid += "0"
		}
		// UDP setup — mirror Tower's ResolveUDPAddr + DialUDP path exactly. The
		// previous string-form net.Dial("udp", "localhost:5008") path bound the
		// socket via Go's default resolver, which on Windows preferred the IPv6
		// ::1 address. SRS routed audio to the IPv4 source-port of the client
		// list entry, so packets never reached us — SkyEye on the same SRS/freq
		// received voice fine. Aligning the UDP create call to ResolveUDPAddr +
		// DialUDP removes that resolver variable and binds the same way Tower
		// does (verified working). Confirmed via bisect with SkyEye 2026-05-16.
		udpResolved, err := net.ResolveUDPAddr("udp", srsAddr)
		if err != nil {
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		udpConn, err := net.DialUDP("udp", nil, udpResolved)
		if err != nil {
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		freqHz := freqMHz * 1e6
		syncMsg := buildSync(guid, marshalCallsign, freqHz)
		tcpConn.Write(syncMsg)
		eamMsg := buildEAM(guid, marshalCallsign, freqHz, eamPassword)
		tcpConn.Write(eamMsg)
		// Prime the UDP NAT mapping immediately. Without this the first
		// keepalive doesn't fire for 10s, so SRS never learns our UDP source
		// address and routes nothing to us until then. SkyEye's SendPing at
		// the end of initialize() does the same.
		udpConn.Write([]byte(guid))
		log.Info().Float64("freq", freqMHz).Msg("Marshal registered on SRS")

		// UDP-only keepalive — matches Tower's srsLoop. Earlier revision
		// also re-sent a TCP Sync every 10s; SRS treats that as a fresh
		// client registration and tears down the audio-routing entry,
		// dropping voice during the re-register gap. The TCP reader below
		// already replies to server pings (MsgType=1) with a Sync echo,
		// which is the correct protocol behavior for keeping the entry
		// alive without re-registering.
		keepaliveStop := make(chan struct{})
		go func() {
			defer close(keepaliveStop)
			tk := time.NewTicker(10 * time.Second)
			defer tk.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tk.C:
					udpConn.Write([]byte(guid))
				}
			}
		}()

		// TCP reader — responds to server pings and detects disconnect.
		// Without this Marshal silently zombies on a dead socket and the
		// SRS server removes it from the client list after first missed
		// ping, even though the goroutine keeps writing keepalives.
		tcpDone := make(chan struct{})
		go func() {
			defer close(tcpDone)
			reader := bufio.NewReader(tcpConn)
			for {
				tcpConn.SetReadDeadline(time.Now().Add(90 * time.Second))
				line, err := reader.ReadBytes('\n')
				if err != nil {
					return
				}
				srsNames.note(line) // SRS client names: srs_names.go
				var msg map[string]interface{}
				if json.Unmarshal(line, &msg) == nil {
					if msgType, ok := msg["MsgType"].(float64); ok && int(msgType) == 1 {
						tcpConn.Write(syncMsg)
					}
				}
			}
		}()

		transmissions := make(map[string]*transmission)
		udpBuf := make([]byte, 4096)
		flushTicker := time.NewTicker(500 * time.Millisecond)

		connDone := false
		for !connDone {
			select {
			case <-ctx.Done():
				tcpConn.Close()
				udpConn.Close()
				flushTicker.Stop()
				return
			case <-tcpDone:
				log.Warn().Msg("Marshal: SRS disconnected — reconnecting")
				connDone = true
			case <-flushTicker.C:
				now := time.Now()
				for origin, tx := range transmissions {
					if now.Sub(tx.lastPacket) > 400*time.Millisecond && len(tx.opusFrames) > 9 {
						if rxMuted(txCooldown, freqMHz) {
							delete(transmissions, origin)
							continue
						}
						frames := tx.opusFrames
						delete(transmissions, origin)
						go func(f [][]byte) {
							text, err := transcribeFramesRetry(ctx, apiKey, f, carrierWhisperPrompt, "Marshal", isWhisperHallucination)
							if err != nil {
								// Was a silent return: a failing Whisper call
								// looked exactly like nobody transmitting.
								log.Warn().Err(err).Int("frames", len(f)).Msg("Marshal: transcription failed")
								return
							}
							if text == "" {
								log.Debug().Int("frames", len(f)).Msg("Marshal: empty transcription")
								return
							}
							// Filter Whisper hallucinations — prompt echo and nonsense
							if isWhisperHallucination(text) {
								log.Info().Str("text", text).Msg("Marshal: hallucination filtered")
								return
							}
							text = srsCorrect(text, origin)
							log.Info().Str("text", text).Msg("Marshal heard")
							cs := undoubleCallsign(marshalCallsignFromText(text), func(c string) bool {
								return atcCtrl.HasRecentContact(c, time.Minute)
							})
							handleMarshalCall(text, cs, stack, comp, transmit, atcCtrl)
						}(frames)
					}
				}
			default:
				udpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, readErr := udpConn.Read(udpBuf)
				if readErr != nil {
					continue
				}
				log.Debug().Int("bytes", n).Msg("Marshal UDP packet received")
				if n < 6 {
					log.Debug().Int("bytes", n).Msg("Marshal UDP packet too short — ignoring")
					continue
				}
				// SRS UDP voice packet header is [pktLen(2)] [audioLen(2)] [freqSegLen(2)].
				// Earlier revision read audioLen from offset 4 — that's freqSegLen, a
				// tiny value (typically 10 = one freq entry), so opusBytes received only
				// ~10 bytes of header garbage and Whisper never returned a usable
				// transcription. Tower's srsLoop reads from offset 2; aligning here.
				audioLen := int(binary.LittleEndian.Uint16(udpBuf[2:4]))
				if audioLen <= 0 || 6+audioLen > n {
					log.Debug().Int("bytes", n).Int("audioLen", audioLen).Msg("Marshal UDP audioLen rejected")
					continue
				}
				origin := extractOriginFromUDP(udpBuf[:n])
				opusBytes := make([]byte, audioLen)
				copy(opusBytes, udpBuf[6:6+audioLen])
				log.Debug().Str("origin", origin).Int("audioLen", audioLen).Msg("Marshal UDP voice frame accepted")
				if transmissions[origin] == nil {
					transmissions[origin] = &transmission{}
				}
				transmissions[origin].opusFrames = append(transmissions[origin].opusFrames, opusBytes)
				transmissions[origin].lastPacket = time.Now()
			}
		}

		flushTicker.Stop()
		tcpConn.Close()
		udpConn.Close()
		log.Debug().Msg("Marshal: SRS disconnected, reconnecting in 5s")
		time.Sleep(5 * time.Second)
	}
}

// handleMarshalCall processes a recognized marshal transmission.
func handleMarshalCall(text, callsign string, stack *state.MarshalStack, comp *composer.ATCComposer, transmit func(string), atcCtrl *controller.ATCController) {
	lower := strings.ToLower(text)
	// Self-echo guard: pilot calls lead with the address — "Marshal", "Union
	// Marshal", or one of Whisper's garbles of it (see marshalAddressRe).
	// Marshal's own TX has the inverse shape "<callsign>, Union Marshal, …", so
	// a call that doesn't open with the address is our echo coming back through
	// SRS, or traffic for someone else. Without this we self-loop on responses
	// containing "state X" (CopyState ack → retranscribed → fires CopyState
	// again). Logged at info: at debug these drops were invisible, and on
	// 2026-09-14 they read in the log as Marshal ignoring the pilot.
	if _, ok := splitMarshalAddress(text); !ok {
		cs := unaddressedStackCallsign(text)
		if cs == "" {
			cs = unaddressedApproachCallsign(text, time.Now()) // marshal_case3.go
		}
		if cs == "" {
			if asked, fuel := pendingStateAnswer(text, time.Now()); asked != "" {
				// The answer to "say state" with the address garbled (marshal_recent.go).
				log.Info().Str("callsign", asked).Str("text", text).Msg("Marshal: state answer without the address - accepted")
				noteMarshalWorked(asked, time.Now())
				transmit(comp.MarshalCopyState(stackTXCallsign(stack, asked), fuel))
				return
			}
			log.Info().Str("text", text).Msg("Marshal: not addressed to Marshal — ignored")
			return
		}
		log.Info().Str("callsign", cs).Str("text", text).Msg("Marshal: stack call without the address - accepted")
		callsign = cs
	}
	fuelState := extractFuelStateMarshal(lower)
	if fuelState == 0 {
		fuelState = bareMarshalState(lower)
	}
	intent := classifyMarshalCall(lower, fuelState)
	if marshalResolvesAlias(intent) {
		callsign = resolveStackCallsign(callsign, lower, stack) // marshal_alias.go
	}
	callsign = fixMarshalCallsign(callsign, intent, stack, time.Now()) // marshal_recent.go
	if intent != marshalNone && intent != marshalReadback {
		noteMarshalWorked(callsign, time.Now())
	}
	// Every response leads with the callsign. Without one the TX would open
	// ", Union Marshal, …" and its echo would pass the guard above.
	if callsign == "" {
		log.Info().Str("text", text).Msg("Marshal: no callsign in call — ignored")
		return
	}
	// A section leads with "<lead> flight" (marshal_section.go).
	say := stackTXCallsign(stack, callsign)
	ceilingFt, altimeter := atcCtrl.GetWeatherState()
	visNm := atcCtrl.GetVisibilityNm()
	switch intent {
	case marshalRadioCheck:
		log.Info().Str("callsign", callsign).Msg("Marshal: radio check")
		transmit(comp.RadioCheck(say))

	case marshalRadarCheck:
		rAng, rDist, rBrg, rFound := atcCtrl.LookupCallerRelativeToCarrier(callsign)
		rBrg = magneticDeg(atcCtrl, rBrg)
		log.Info().Str("callsign", callsign).Bool("radarFound", rFound).Int("radarAngels", rAng).Int("radarDistNm", rDist).Int("radarBearingMag", rBrg).Msg("Marshal: radar check")
		if rFound {
			transmit(comp.MarshalRadarCheck(say, rAng, rDist, rBrg))
		} else {
			transmit(comp.MarshalRadarCheckNoContact(say))
		}

	case marshalBRCRequest:
		// Magnetic, off the carrier nearest the caller. Both were wrong on
		// 2026-09-14: the true heading was spoken, and a second CVN on the map
		// would have been ignored.
		brc := atcCtrl.ToMagnetic(atcCtrl.GetCarrierBRCFor(callsign))
		log.Info().Str("callsign", callsign).Float64("brcMag", brc).Msg("Marshal: BRC request")
		transmit(comp.MarshalSayBRC(say, brc))

	case marshalMarkingMom:
		pos, _ := stack.Enqueue(callsign, fuelState)
		// "Raider 302 holding hands with 032, marking mom's ...": one slot for
		// the section (operator 2026-10-20).
		if wing := marshalWingmen(text, callsign); len(wing) > 0 {
			joinSection(stack, callsign, wing)
			say = stackTXCallsign(stack, callsign)
		}
		// Pull live recovery case from state so phraseology stays in sync with
		// any mid-recovery transition the watcher already announced.
		recCase := atcCtrl.GetRecoveryCase()
		lo, hi := marshalAngelsBand(recCase)
		reserved := stack.ReservedAngels(callsign)
		stackAngels := atcCtrl.AssignMarshalAngels(lo, hi, reserved)
		stack.SetAngels(callsign, stackAngels)
		brc := atcCtrl.ToMagnetic(atcCtrl.GetCarrierBRCFor(callsign))
		rAng, rDist, rBrg, rFound := atcCtrl.LookupCallerRelativeToCarrier(callsign)
		rBrg = magneticDeg(atcCtrl, rBrg)
		caseLabel := composer.CaseLabel(int(recCase))
		// Build stack summary for response
		stackInfo := ""
		all := stack.GetAll()
		if len(all) > 1 {
			stackInfo = fmt.Sprintf(" Stack has %d aircraft.", len(all))
		}
		if recCase == state.CaseThree {
			// Case 3: assign holding radial (reciprocal of BRC) and an EAT
			// keyed off stack position. If BRC unknown (carrier off scope)
			// fall back to radial 0 — composer still spells it, and the
			// BRC clause is omitted in the response.
			// The marshal radial is the final bearing's reciprocal (the
			// squadron's DimOn card 26: "FB + 180"), not the BRC's - 9 degrees
			// apart on the angled deck.
			radial := 0
			if brc >= 0 {
				radial = (int(math.Round(brc)) - marshalFinalBearingOffset + 180 + 360) % 360
			}
			eat := case3EAT(stack, callsign, time.Now().Add(time.Duration(marshalCase3LeadTimeMin+(pos-1)*marshalCase3IntervalMin)*time.Minute))
			stack.SetAssignedRadial(callsign, radial)
			stack.SetEAT(callsign, eat)
			log.Info().Str("callsign", callsign).Int("position", pos).Int("stackAngels", stackAngels).Int("radial", radial).Time("eat", eat).Float64("brcMag", brc).Float64("ceiling", ceilingFt).Float64("vis", visNm).Bool("radarFound", rFound).Str("case", caseLabel).Msg("Marshal: Case 3 aircraft checking in")
			transmit(comp.MarshalMarkingMomCase3(say, stackAngels, radial, eat.Minute(), altimeter, ceilingFt, visNm, brc, rAng, rDist, rBrg, rFound) + stackInfo)
		} else {
			log.Info().Str("callsign", callsign).Int("position", pos).Int("stackAngels", stackAngels).Ints("reserved", reserved).Float64("brcMag", brc).Float64("ceiling", ceilingFt).Float64("vis", visNm).Bool("radarFound", rFound).Int("radarAngels", rAng).Int("radarDistNm", rDist).Int("radarBearingMag", rBrg).Str("case", caseLabel).Msg("Marshal: aircraft checking in")
			transmit(comp.MarshalMarkingMom(say, pos, stackAngels, altimeter, ceilingFt, visNm, brc, rAng, rDist, rBrg, rFound, caseLabel) + stackInfo)
		}

	case marshalCheckIn:
		// Bare "checking in" with no marking mom's — flown twice on 2026-09-14
		// and answered with silence. Radar picture plus a prompt for marking
		// mom's; no stack slot until the pilot gives it.
		rAng, rDist, rBrg, rFound := atcCtrl.LookupCallerRelativeToCarrier(callsign)
		rBrg = magneticDeg(atcCtrl, rBrg)
		log.Info().Str("callsign", callsign).Bool("radarFound", rFound).Int("radarDistNm", rDist).Int("radarBearingMag", rBrg).Msg("Marshal: check-in without marking mom's")
		transmit(comp.MarshalCheckIn(say, rAng, rDist, rBrg, rFound))

	case marshalSeeYouAtTen:
		if fuelState > 0 {
			// "see you at 10, state 6.6": the state is already given, don't ask.
			transmit(comp.MarshalCopyState(say, fuelState))
			break
		}
		noteMarshalAskedState(callsign, time.Now())
		transmit(comp.MarshalRadarContact(say, 10))

	case marshalDepartureClear:
		// Departing aircraft, handed over by Deckboss with the airborne ack.
		// Marshal releases them from the carrier's control zone and pushes them
		// to Command for tasking. Deliberately no stack interaction: a departure
		// is leaving, so enqueuing it would put a phantom in the recovery stack
		// and skew the angels assignment for aircraft actually coming back.
		//
		// classifyMarshalCall tests this ahead of the DME report — see
		// isDepartureClearCall.
		dist := extractDMEDistance(lower)
		log.Info().Str("callsign", callsign).Int("distNm", dist).
			Str("to", flagHandoffCommandName).Float64("freq", flagHandoffCommandFreq).
			Msg("Marshal: departure clear of zone, pushing to command")
		transmit(comp.MarshalDepartureClear(say, marshalZoneName(),
			flagHandoffCommandName, flagHandoffCommandFreq, flagHandoffCommandPreset))

	case marshalDME:
		dist := extractDMEDistance(lower)
		if dist <= 0 {
			log.Info().Str("text", text).Msg("Marshal: DME-shaped call but no parseable distance — dropped")
			break
		}
		_, _, _, rFound := atcCtrl.LookupCallerRelativeToCarrier(callsign)
		log.Info().Str("callsign", callsign).Int("distNm", dist).Bool("radarFound", rFound).Msg("Marshal: DME position report")
		transmit(comp.MarshalAckDME(say, dist, rFound, fuelState))

	case marshalState:
		transmit(comp.MarshalCopyState(say, fuelState))

	case marshalEstablished:
		if _, ok := stack.GetAircraft(callsign); !ok {
			// A jet in the stack that never marked mom's with us. It used to
			// be acked "angels six, position one" and held nowhere, so it
			// never got Charlie. Put it in the stack at the angels it reports
			// (or the next free one) so the Charlie watcher can see it.
			stack.Enqueue(callsign, fuelState)
			angels := 0
			if m := angelsInCallRe.FindStringSubmatch(lower); m != nil {
				angels, _ = strconv.Atoi(m[1])
			}
			if lo, hi := marshalAngelsBand(atcCtrl.GetRecoveryCase()); angels < lo || angels > hi {
				angels = atcCtrl.AssignMarshalAngels(lo, hi, stack.ReservedAngels(callsign))
			}
			stack.SetAngels(callsign, angels)
			log.Info().Str("callsign", callsign).Int("angels", angels).Msg("Marshal: established without marking mom's - added to the stack")
		}
		stack.SetPhase(callsign, "holding")
		angels := 6
		position := 1
		radial := 0
		var eat time.Time
		if ac, ok := stack.GetAircraft(callsign); ok {
			angels = ac.Angels
			position = ac.Position
			radial = ac.AssignedRadial
			eat = ac.EAT
		}
		if atcCtrl.GetRecoveryCase() == state.CaseThree {
			// Case 3: commence on EAT, not Charlie. If EAT was never set
			// (Case 3 transition mid-recovery, pilot never got a Case 3
			// marking-mom), compute one now from current stack position.
			eat = case3HoldEAT(stack, callsign, position, eat, time.Now()) // marshal_case3.go
			log.Info().Str("callsign", callsign).Int("angels", angels).Int("position", position).Int("radial", radial).Time("eat", eat).Msg("Marshal: Case 3 established in hold")
			transmit(comp.MarshalEstablishedAckCase3(say, angels, radial, eat.Minute()))
		} else if atcCtrl.IsDeckClear() && marshalCharlie.nextCharlie(stack.GetAll(), time.Now()) == callsign {
			stack.SetPhase(callsign, "charlie")
			marshalCharlie.given(time.Now())
			transmit(comp.MarshalSignalCharlie(say))
		} else {
			transmit(comp.MarshalEstablishedAck(say, angels, position))
		}

	case marshalCommencing:
		stack.SetPhase(callsign, "commencing")
		stack.Remove(callsign)
		if atcCtrl.GetRecoveryCase() == state.CaseThree {
			brc := atcCtrl.ToMagnetic(atcCtrl.GetCarrierBRCFor(callsign))
			finalBearing := 0
			brcDeg := -1
			if brc >= 0 {
				brcDeg = int(math.Round(brc)) % 360
				finalBearing = (brcDeg - marshalFinalBearingOffset + 360) % 360
			}
			_, _, _, onRadar := atcCtrl.LookupCallerRelativeToCarrier(callsign)
			log.Info().Str("callsign", callsign).Float64("brcMag", brc).Int("finalBearing", finalBearing).Float64("fuelState", fuelState).Bool("radarContact", onRadar).Msg("Marshal: Case 3 commencing, descend to platform")
			transmit(comp.MarshalCopyCommencingCase3(say, fuelState, finalBearing, brcDeg, onRadar))
			trackCase3Approach(callsign, say, time.Now()) // Paddles at three miles (marshal_case3.go)
		} else {
			transmit(comp.MarshalCopyCommencing(say, fuelState))
		}
		// Internal stack collapse only — pack remaining aircraft down to fill
		// the vacated slot so the next "marking moms" gets the correct angels.
		// Per 07.png Marshal does not transmit step-down clearances.
		lo, _ := marshalAngelsBand(atcCtrl.GetRecoveryCase()) // Case 3 stays at angels 6 and up
		for _, sd := range stack.CollapseStack(lo) {
			log.Info().Str("callsign", sd.Callsign).Int("from", sd.OldAngels).Int("to", sd.NewAngels).Msg("Marshal: stack step-down (internal, no TX)")
		}

	case marshalPlatform:
		// Case 3 platform call — pilot is descending through 5000 ft inside
		// ~20 nm. Ack with final bearing + dirty-up reminder. No state
		// transition needed; pilot continues to glideslope intercept and
		// hands off to LSO at ball call. Only meaningful in Case 3; in Case
		// 1/2 we still ack (rare but harmless) on the same final bearing
		// math so the pilot isn't ignored.
		brc := atcCtrl.ToMagnetic(atcCtrl.GetCarrierBRCFor(callsign))
		finalBearing := 0
		if brc >= 0 {
			finalBearing = (int(math.Round(brc)) - marshalFinalBearingOffset + 360) % 360
		}
		stack.SetPhase(callsign, "platform")
		log.Info().Str("callsign", callsign).Float64("brcMag", brc).Int("finalBearing", finalBearing).Str("case", composer.CaseLabel(int(atcCtrl.GetRecoveryCase()))).Msg("Marshal: platform call")
		_, _, _, onRadar := atcCtrl.LookupCallerRelativeToCarrier(callsign)
		transmit(comp.MarshalAtPlatform(say, finalBearing, !onRadar))

	case marshalNeedles:
		// No ACLS here: a needles report is acknowledged, and with negative
		// needles the jet flies the ILS ("bullseye"), as the card has it.
		negative := containsAny(lower, "negative needles", "no needles", "needles negative")
		log.Info().Str("callsign", callsign).Bool("negative", negative).Str("text", text).Msg("Marshal: Case 3 needles report")
		transmit(comp.MarshalNeedlesAck(say, negative))

	case marshalACLSReport:
		what := "coupled"
		if strings.Contains(lower, "command control") {
			what = "command control"
		}
		log.Info().Str("callsign", callsign).Str("report", what).Msg("Marshal: Case 3 ACLS report")
		transmit(comp.MarshalApproachAck(say, what))

	case marshalPushCommand:
		// Pilot's courtesy call on the departure push issued above. Short ack
		// only. Sits after the DME case for the same reason as the paddles ack
		// below, and our own departure TX ends in "push <command> for tasking",
		// which contains this trigger — the address-led guard at the top of the
		// handler drops that echo before it reaches here.
		log.Info().Str("callsign", callsign).Msg("Marshal: pilot-initiated command handoff ack")
		transmit(comp.HandoffAck(say, flagHandoffCommandName))

	case marshalPushPaddles:
		// Pilot's courtesy call announcing they're leaving Marshal for the
		// LSO — the other half of the §6 handoff. Short ack only; the
		// destination was already issued so repeating it burns radio time.
		// Deliberately placed after the DME case so a call like
		// "Marshal, Raider 39, 7 DME, switching channel 4" still reads as a
		// DME position report.
		log.Info().Str("callsign", callsign).Msg("Marshal: pilot-initiated paddles handoff ack")
		transmit(comp.HandoffAck(say, "paddles"))

	case marshalInitial:
		// 3nm initial — pilot is rolling on the boat, hand off to LSO/Paddles.
		// Marshal's last call before pilot pushes to the LSO freq. TACAN
		// button reference was dropped — pilots know to switch.
		log.Info().Str("callsign", callsign).Msg("Marshal: 3nm initial, handing off to LSO")
		transmit(comp.MarshalPushButton(say))

	case marshalBolter:
		if atcCtrl.GetRecoveryCase() != state.CaseThree {
			log.Info().Str("callsign", callsign).Msg("Marshal: bolter call outside Case 3 - to Paddles")
			transmit(comp.MarshalPushButton(say))
			break
		}
		what := "bolter"
		if !strings.Contains(lower, "bolter") {
			what = "wave off"
		}
		told := case3CalledPattern(callsign, say, case3AsternTrue(atcCtrl.GetCarrierBRCFor(callsign)), time.Now())
		log.Info().Str("callsign", callsign).Str("call", what).Bool("alreadyTold", told).Msg("Marshal: Case 3 bolter pattern")
		if told {
			transmit(comp.MarshalApproachAck(say, what))
		} else {
			transmit(comp.MarshalCase3Bolter(say, what))
		}

	case marshalAbeam:
		if atcCtrl.GetRecoveryCase() != state.CaseThree {
			log.Info().Str("callsign", callsign).Msg("Marshal: abeam call outside Case 3 - to Paddles")
			transmit(comp.MarshalPushButton(say))
			break
		}
		brcTrue := atcCtrl.GetCarrierBRCFor(callsign)
		brc := atcCtrl.ToMagnetic(brcTrue)
		finalBearing := 0
		if brc >= 0 {
			finalBearing = (int(math.Round(brc)) - marshalFinalBearingOffset + 360) % 360
		}
		case3CalledPattern(callsign, say, case3AsternTrue(brcTrue), time.Now())
		log.Info().Str("callsign", callsign).Int("finalBearing", finalBearing).Msg("Marshal: Case 3 abeam in the bolter pattern")
		transmit(comp.MarshalCase3Abeam(say, finalBearing))

	case marshalPushTime:
		ac, inStack := stack.GetAircraft(callsign)
		switch {
		case atcCtrl.GetRecoveryCase() != state.CaseThree:
			log.Info().Str("callsign", callsign).Msg("Marshal: push time asked outside Case 3")
			transmit(comp.MarshalNoPushTime(say, composer.CaseLabel(int(atcCtrl.GetRecoveryCase()))))
		case !inStack:
			log.Info().Str("callsign", callsign).Msg("Marshal: push time asked by a jet not in the stack")
			transmit(comp.MarshalSayMarkingMoms(say))
		default:
			eat := case3HoldEAT(stack, callsign, ac.Position, ac.EAT, time.Now())
			log.Info().Str("callsign", callsign).Time("eat", eat).Msg("Marshal: Case 3 push time")
			transmit(comp.MarshalPushTime(say, eat.Minute()))
		}

	case marshalReadback:
		log.Info().Str("callsign", callsign).Str("text", text).Msg("Marshal: readback, no reply")

	default:
		log.Info().Str("callsign", callsign).Str("text", text).Msg("Marshal intent miss")
	}
}
