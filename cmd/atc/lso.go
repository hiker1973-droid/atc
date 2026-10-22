package main

import (
	"bufio"
	"context"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/lso"
)

// netHandled, returned as the reply from a voiceNetLoop handler, means the
// handler dealt with the call itself (Paddles queues its own TX) - no reply,
// and not an intent miss.
const netHandled = "<paddles handled>"

// Paddles (--lso-only): SkyEye's LSO, its own role on its own frequency
// (operator 2026-10-17: 128.300, Hornet COMM2 CH4, OpenAI voice, talking from
// the first build). It reads Tacview itself, runs pkg/lso on every player jet
// near the carrier and speaks the calls; ball calls are heard on the same net.

var (
	flagLSOOnly    bool
	flagLSOFreq    float64
	flagLSOName    string
	flagLSOVoice   string
	flagLSOLateral float64
	// flagLSODeckbossFreq is where "green deck" sends the landed jet.
	flagLSODeckbossFreq float64
	// flagElevenPaddlesVoice: Paddles' ElevenLabs voice (operator 2026-10-18:
	// "change the paddles to ... marshal_2_EL-Matilda-F.mp3"); OpenAI
	// --lso-voice is the fallback.
	flagElevenPaddlesVoice string
)

// registerLSOFlags is called from the flag block in main.go.
func registerLSOFlags(f *flag.FlagSet) {
	f.BoolVar(&flagLSOOnly, "lso-only", false, "Run as the carrier LSO (Paddles) only")
	f.Float64Var(&flagLSOFreq, "lso-freq", 128.3, "Paddles frequency in MHz")
	f.StringVar(&flagLSOName, "lso-name", "Paddles", "Paddles SRS client name")
	f.StringVar(&flagLSOVoice, "lso-voice", "ash", "Paddles OpenAI TTS voice (the fallback when ElevenLabs is off)")
	f.StringVar(&flagElevenPaddlesVoice, "elevenlabs-paddles-voice", elMatilda,
		`ElevenLabs voice ID for Paddles (default Matilda; "" = OpenAI only)`)
	f.Float64Var(&flagLSOLateral, "lso-landing-lateral-m", -4,
		"Landing point offset from the carrier centreline in metres, starboard positive (calibrate from logged traps)")
	f.Float64Var(&flagLSODeckbossFreq, "lso-deckboss-freq", 128.6,
		`Deckboss frequency in MHz that Paddles hands a landed jet to on "green deck" (0 = name only)`)
	f.StringVar(&flagLSOGrpcAddr, "lso-grpc-addr", "localhost:50051",
		`DCS-gRPC for DCS's own LSO grades and wires, logged beside Paddles' ("" = off; Night is :50053)`)
}

const styleLSO = "Speak as a US Navy landing signal officer on the platform. " +
	"Calm, firm, very short. Even and steady for corrections; urgent and loud for wave off and bolter."

// styleLSOPower is take B of three (operator 2026-10-17: "be more stern with
// power", then "B").
const styleLSOPower = "Speak as a US Navy landing signal officer on the platform. " +
	"Stern and urgent. The pilot is settling low in close. Bark the word like an order, loud and firm, with a hard edge."

// styleLSOPowerUrgent is the repeat ("POWER!") and the sinking call: take B,
// louder (DCS: "Power" with more urgent inflection, then "power, Power, POWER").
const styleLSOPowerUrgent = "Speak as a US Navy landing signal officer on the platform. " +
	"Very urgent and loud, almost shouting. The pilot is still low and sinking at the ramp. " +
	"Each word harder than the last, a hard edge, no softness."

func lsoVoice() voiceProfile {
	vp := voiceProfile{voice: flagLSOVoice, speed: flagTTSSpeed, instructions: styleLSO}
	if flagElevenPaddlesVoice != "" {
		vp.eleven = elPaddles(flagElevenPaddlesVoice)
	}
	return vp
}

// lsoVoiceFor is the delivery for one call: the power calls get their own.
func lsoVoiceFor(text string) voiceProfile {
	vp := lsoVoice()
	switch lso.Call(text) {
	case lso.CallPower:
		vp.instructions = styleLSOPower
	case lso.CallPowerUrgent, lso.CallPowerSink:
		vp.instructions = styleLSOPowerUrgent
	}
	return vp
}

// lsoTX is one queued call. Calls older than their limit when the radio frees
// up are dropped: "power" two seconds late is wrong.
type lsoTX struct {
	text   string
	queued time.Time
}

func lsoMaxAge(text string) time.Duration {
	if lso.IsWaveOff(lso.Call(text)) {
		return 3 * time.Second
	}
	switch lso.Call(text) {
	case lso.CallBolter:
		return 3 * time.Second
	case lso.CallRogerBall, lso.CallClara:
		return 4 * time.Second
	}
	return 1500 * time.Millisecond
}

// After the ball the tracker may make a call every second, which is faster
// than one radio call takes to say: on Day 2026-10-20 19:15 Raider 319 heard
// "Right for lineup" four times and "A little high" six, up to 1.8 s late,
// with three more dropped as too late. So the radio says the newest call in
// the queue (a wave-off or bolter call first, whatever is newer), and not the
// call it has just finished saying.
const lsoRepeatGap = 2500 * time.Millisecond

func lsoUrgent(text string) bool {
	return lso.IsWaveOff(lso.Call(text)) || lso.Call(text) == lso.CallBolter
}

// lsoLatest takes everything waiting behind tx and returns the call to say.
func lsoLatest(tx lsoTX, q chan lsoTX) lsoTX {
	for {
		select {
		case nx := <-q:
			switch {
			case lsoUrgent(tx.text) && !lsoUrgent(nx.text):
				log.Info().Str("text", nx.text).Msg("Paddles: call dropped, wave-off first")
			default:
				if !lsoUrgent(tx.text) {
					log.Info().Str("text", tx.text).Msg("Paddles: call dropped, superseded")
				}
				tx = nx
			}
		default:
			return tx
		}
	}
}

// lsoRepeat: tx is the call just said, again, too soon to be news. Wave-offs
// are always repeated.
func lsoRepeat(tx, last lsoTX, now time.Time) bool {
	return !lsoUrgent(tx.text) && tx.text == last.text && now.Sub(last.queued) < lsoRepeatGap
}

func runLSO(ctx context.Context, apiKey string) {
	vp := lsoVoice()
	srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)

	// Render every fixed call now so a call costs only the ExternalAudio start.
	for _, c := range lso.AllCalls {
		if _, err := synthesizeSpeech(ctx, apiKey, string(c), lsoVoiceFor(string(c))); err != nil {
			log.Warn().Err(err).Str("text", string(c)).Msg("Paddles: TTS prewarm failed")
		}
	}
	log.Info().Int("calls", len(lso.AllCalls)).Msg("Paddles: calls rendered")

	txq := make(chan lsoTX, 8)
	go func() {
		var last lsoTX // the call last transmitted, queued = when it ended
		for {
			select {
			case <-ctx.Done():
				return
			case tx := <-txq:
				tx = lsoLatest(tx, txq)
				if lsoRepeat(tx, last, time.Now()) {
					log.Info().Str("text", tx.text).Msg("Paddles: call dropped, just said")
					continue
				}
				if age := time.Since(tx.queued); age > lsoMaxAge(tx.text) {
					log.Info().Str("text", tx.text).Dur("late", age).Msg("Paddles: call dropped, too late")
					continue
				}
				mp3, err := synthesizeSpeech(ctx, apiKey, tx.text, lsoVoiceFor(tx.text))
				if err != nil {
					log.Error().Err(err).Str("text", tx.text).Msg("Paddles: TTS failed")
					continue
				}
				log.Info().Str("text", tx.text).Float64("freq", flagLSOFreq).Dur("lag", time.Since(tx.queued)).Msg("Paddles TX")
				noteOwnTX(flagLSOFreq, tx.text)
				transmitExternalAudioFile(ctx, mp3, flagLSOFreq, flagLSOName, srsHost, srsPort, flagExternalAudio)
				last = lsoTX{text: tx.text, queued: time.Now()}
			}
		}
	}()
	say := func(c lso.Call) {
		if c == "" {
			return
		}
		select {
		case txq <- lsoTX{text: string(c), queued: time.Now()}:
		default:
			log.Warn().Str("text", string(c)).Msg("Paddles: TX queue full")
		}
	}

	p := lso.NimitzParams()
	p.LandingLateralM = flagLSOLateral
	feed := newLSOFeed(p, say)
	// The foul-deck wave-off carries his callsign, so render it as his pass
	// starts: at a quarter mile there is no time to wait for TTS.
	feed.tr.OnPass = func(cs string) {
		go func() {
			c := string(lso.FoulDeckCall(cs))
			if _, err := synthesizeSpeech(ctx, apiKey, c, lsoVoiceFor(c)); err != nil {
				log.Warn().Err(err).Str("text", c).Msg("Paddles: TTS prewarm failed")
			}
		}()
	}

	go voiceNetLoop(ctx, flagSRSAddr, flagLSOFreq, flagLSOName, apiKey, flagEAMPassword, vp, flagExternalAudio,
		lsoWhisperPrompt, nil, func(text string) (string, string) {
			return feed.heard(text)
		})
	go runLSOTacview(ctx, flagTacviewAddr, feed)
	go runLSOGrpc(ctx, flagLSOGrpcAddr, feed.truth)
	log.Info().Float64("freq", flagLSOFreq).Str("name", flagLSOName).Str("voice", flagLSOVoice).
		Float64("lateralM", flagLSOLateral).Msg("Paddles starting")
	<-ctx.Done()
}

const lsoWhisperPrompt = "Paddles, Raider, Venom, in the break, abeam, three-quarter mile, Hornet ball, Tomcat ball, Clara, ball, red deck, green deck, radio check, state"

// lsoBallRe: "302, Hornet ball, 5.5" (Whisper also writes "Hornetball" and,
// on 306.3 on 2026-10-16/17, "bolt" for "ball").
var (
	lsoBallRe   = regexp.MustCompile(`(?i)\b(?:hornet|tomcat|viper|rhino|super hornet|goshawk)\s*(?:ball|bolt|bowl)\b|\bball\b`)
	lsoClaraRe  = regexp.MustCompile(`(?i)\bclara\b|\bclarence\b`)
	lsoModexRe  = regexp.MustCompile(`\b(\d)[\s-]?(\d)[\s-]?(\d)\b`)
	lsoRadioChk = regexp.MustCompile(`(?i)radio check|comm check|com check|comms check|comp check|comcheck|how copy`)
	// lsoAbeamRe: "302 abeam" (Whisper: "a beam", or just "beam").
	lsoAbeamRe = regexp.MustCompile(`(?i)\ba[\s-]?beam\b|\bbeam\b`)
	// lsoBreakRe: "302 in the break" (Day 2026-10-18, three times, unanswered).
	lsoBreakRe = regexp.MustCompile(`(?i)\b(?:in the|at the|on the|in) break\b|\bbreaking\b`)
	// lsoThreeQuarterRe + lsoStateRe: a ball call without "ball" - "Raider
	// 312, three-quarter mile, 5.4" and "312, 3/4 mile, full, state 5.1" (Day
	// 2026-10-18, both unanswered): the three-quarter mile point and a state.
	lsoThreeQuarterRe = regexp.MustCompile(`(?i)\b(?:three[\s-]?quarters?|3/4|3 quarters?)[\s-]?(?:of a )?miles?\b`)
	lsoStateRe        = regexp.MustCompile(`(?i)\b\d{1,2}\.\d\b|\bstate\b`)
	// lsoCallsignRe is a full callsign ("Raider 302"), not a fragment of the call.
	lsoCallsignRe = regexp.MustCompile(`^[A-Za-z]+ \d{1,3}$`)
	// lsoRedDeckRe / lsoGreenDeckRe: the jet that just landed calls "red deck",
	// then "green deck" once clear of the landing area (operator 2026-10-18).
	lsoRedDeckRe   = regexp.MustCompile(`(?i)\b(?:red|read)[\s-]?(?:deck|dec|dek)\b|\bdeck (?:is )?red\b`)
	lsoGreenDeckRe = regexp.MustCompile(`(?i)\bgreen[\s-]?(?:deck|dec|dek)\b|\bdeck (?:is )?green\b`)
)

// lsoFeed is the shared state: the tracker (Tacview goroutine) and the radio
// (voiceNetLoop goroutine) both touch it.
type lsoFeed struct {
	mu    sync.Mutex
	tr    *lso.Tracker
	say   func(lso.Call)
	truth *lsoTruth
}

func newLSOFeed(p lso.Params, say func(lso.Call)) *lsoFeed {
	f := &lsoFeed{tr: lso.NewTracker(p), say: say, truth: newLSOTruth()}
	f.tr.Done = func(summary string) {
		log.Info().Str("pass", summary).Msg("Paddles: pass")
		f.truth.pass(summary, time.Now())
	}
	return f
}

func (f *lsoFeed) lock()   { f.mu.Lock() }
func (f *lsoFeed) unlock() { f.mu.Unlock() }

// heard handles a transmission on the Paddles net. Ball calls go to the TX
// queue (one radio, no overlap with the groove calls) and return netHandled.
func (f *lsoFeed) heard(text string) (string, string) {
	modex := ""
	if m := lsoModexRe.FindStringSubmatch(text); m != nil {
		modex = m[1] + m[2] + m[3]
	}
	switch {
	case lsoRedDeckRe.MatchString(text) || lsoGreenDeckRe.MatchString(text):
		// Red deck: no reply, Paddles keeps the deck foul. Green deck: the
		// deck is clear, and the landed jet goes to Deckboss (operator
		// 2026-10-18). Green wins if a call has both ("red deck... green deck").
		red := !lsoGreenDeckRe.MatchString(text)
		f.lock()
		f.tr.SetRedDeck(red, time.Now())
		last := f.tr.LastTrap(time.Now())
		f.unlock()
		log.Info().Bool("red", red).Str("text", text).Msg("Paddles: deck call")
		if red {
			return modex, netHandled
		}
		cs := extractCallsignSkippingAddress(text, "paddles", "patels", "paddle")
		if !lsoCallsignRe.MatchString(cs) {
			cs = modex
		}
		if cs == "" {
			cs = last
		}
		return cs, composer.PaddlesGreenDeck(cs, flagLSODeckbossFreq)
	case lsoClaraRe.MatchString(text) || lsoBallRe.MatchString(text) ||
		lsoThreeQuarterRe.MatchString(text) && lsoStateRe.MatchString(text) && modex != "":
		clara := lsoClaraRe.MatchString(text)
		f.lock()
		c := f.tr.BallCall(modex, clara, time.Now())
		f.unlock()
		log.Info().Str("modex", modex).Bool("clara", clara).Str("text", text).Msg("Paddles: ball call")
		f.say(c)
		return modex, netHandled
	case lsoAbeamRe.MatchString(text):
		// "302 abeam" (operator 2026-10-18): Paddles takes the jet and sets up
		// the ball call. Needs the modex or callsign, so chatter about the
		// beam isn't answered.
		cs := extractCallsignSkippingAddress(text, "paddles", "patels", "paddle")
		if !lsoCallsignRe.MatchString(cs) {
			cs = modex
		}
		if cs == "" {
			return "", ""
		}
		// Wording: operator 2026-10-22.
		return cs, lsoAbeamReply(cs)
	case lsoBreakRe.MatchString(text):
		// "302 in the break": Paddles copies and asks for the abeam call,
		// which sets up the ball call.
		cs := extractCallsignSkippingAddress(text, "paddles", "patels", "paddle")
		if !lsoCallsignRe.MatchString(cs) {
			cs = modex
		}
		if cs == "" {
			return "", ""
		}
		return cs, cs + ", Paddles, roger, call abeam."
	case lsoRadioChk.MatchString(text):
		cs := extractCallsignSkippingAddress(text, "paddles", "patels", "paddle")
		if cs == "" {
			return "", ""
		}
		return cs, cs + ", Paddles, loud and clear."
	}
	return "", ""
}

// lsoAbeamReply is Paddles' answer to "302 abeam" (operator 2026-10-22:
// "Paddles contact Raider XXX, Hornet in the groove, at 3/4 of a mile call the ball").
func lsoAbeamReply(cs string) string {
	return "Paddles contact, " + cs + ", Hornet in the groove, at three-quarters of a mile call the ball."
}

// lsoObj is one ACMI object's last state (properties arrive only on change).
type lsoObj struct {
	lat, lon, altM float64
	hasLat, hasLon bool
	pitch          float64
	hasPitch       bool
	hdg            float64
	pilot, name    string
	typ            string
}

type shipFix struct {
	at       time.Time
	lat, lon float64
}

// lsoTacview turns ACMI lines into tracker updates.
type lsoTacview struct {
	feed           *lsoFeed
	objs           map[string]*lsoObj
	refLat, refLon float64
	carrierID      string
	track          []shipFix // the carrier's recent positions, for heading and speed
	lastExpire     time.Time
	now            func() time.Time

	// onDeck: aircraft on the landing area (object id -> first seen there,
	// last seen there). One that sits there longer than foulMaxAge is parked,
	// not fouling the deck.
	onDeck   map[string][2]time.Time
	lastFoul bool
}

const (
	foulMaxAge = 90 * time.Second
	// onDeckKeep: a jet stays on the landing area at its last known position
	// until Tacview shows it outside, removes it, or this long passes with no
	// update. Tacview sends a position only when it changes, so the old rule
	// ("not seen for 3 s = gone") forgot a jet stopped on the deck: Raider 303,
	// parked on the edge of the area (Day 2026-10-20 19:12-19:22), kept
	// re-entering it as new, fouled the deck for another 90 s each time, and
	// the log flipped foul/clear several times a second.
	onDeckKeep = 10 * time.Minute
)

func newLSOTacview(feed *lsoFeed) *lsoTacview {
	return &lsoTacview{feed: feed, objs: make(map[string]*lsoObj), onDeck: make(map[string][2]time.Time), now: time.Now}
}

// isOurCarrier: CVN_72 (Abe) and the other Nimitz hulls, by ACMI Name.
func isOurCarrier(name, typ string) bool {
	n := strings.ToLower(name)
	return strings.Contains(typ, "AircraftCarrier") && (strings.HasPrefix(n, "cvn_") || strings.Contains(n, "stennis") ||
		strings.Contains(n, "nimitz") || strings.Contains(n, "vinson") || strings.Contains(n, "lincoln") ||
		strings.Contains(n, "washington") || strings.Contains(n, "roosevelt") || strings.Contains(n, "truman"))
}

// ship is the carrier's position, heading and speed from its last ~15 s.
func (t *lsoTacview) ship() (lso.Ship, bool) {
	if len(t.track) < 2 {
		return lso.Ship{}, false
	}
	a, b := t.track[0], t.track[len(t.track)-1]
	dt := b.at.Sub(a.at).Seconds()
	if dt < 3 {
		return lso.Ship{}, false
	}
	dn := (b.lat - a.lat) * 110540
	de := (b.lon - a.lon) * 111320 * math.Cos(b.lat*math.Pi/180)
	s := lso.Ship{Lat: b.lat, Lon: b.lon, SpeedMps: math.Hypot(de, dn) / dt}
	if s.SpeedMps > 1 {
		s.HeadingDeg = math.Mod(math.Atan2(de, dn)*180/math.Pi+360, 360)
	} else if o := t.objs[t.carrierID]; o != nil {
		s.HeadingDeg = o.hdg // dead in the water: fall back to its yaw
	}
	return s, true
}

func (t *lsoTacview) handleLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
		return
	}
	if strings.HasPrefix(line, "-") {
		delete(t.objs, strings.TrimPrefix(line, "-"))
		delete(t.onDeck, strings.TrimPrefix(line, "-"))
		return
	}
	id, rest, ok := strings.Cut(line, ",")
	if !ok {
		return
	}
	if id == "0" {
		for _, fld := range strings.Split(rest, ",") {
			k, v, _ := strings.Cut(fld, "=")
			switch k {
			case "ReferenceLatitude":
				t.refLat, _ = strconv.ParseFloat(v, 64)
			case "ReferenceLongitude":
				t.refLon, _ = strconv.ParseFloat(v, 64)
			case "ReferenceTime":
				t.track, t.carrierID = nil, "" // new mission
			}
		}
		return
	}
	o := t.objs[id]
	if o == nil {
		o = &lsoObj{}
		t.objs[id] = o
	}
	moved := false
	for _, fld := range strings.Split(rest, ",") {
		k, v, ok := strings.Cut(fld, "=")
		if !ok {
			continue
		}
		switch k {
		case "T":
			moved = true
			c := strings.Split(v, "|")
			num := func(i int) (float64, bool) {
				if i >= len(c) || c[i] == "" {
					return 0, false
				}
				f, err := strconv.ParseFloat(c[i], 64)
				return f, err == nil
			}
			if f, ok := num(0); ok {
				o.lon, o.hasLon = f, true
			}
			if f, ok := num(1); ok {
				o.lat, o.hasLat = f, true
			}
			if f, ok := num(2); ok {
				o.altM = f
			}
			// lon|lat|alt|roll|pitch|yaw[|u|v|heading]
			if len(c) == 6 || len(c) == 9 {
				if f, ok := num(4); ok {
					o.pitch, o.hasPitch = f, true
				}
				hi := 5
				if len(c) == 9 {
					hi = 8
				}
				if f, ok := num(hi); ok {
					o.hdg = f
				}
			}
		case "Pilot":
			o.pilot = v
		case "Name":
			o.name = v
		case "Type":
			o.typ = v
		}
	}
	if !moved || !o.hasLat || !o.hasLon {
		return
	}
	now := t.now()
	lat, lon := o.lat+t.refLat, o.lon+t.refLon
	if t.carrierID == "" && isOurCarrier(o.name, o.typ) {
		t.carrierID = id
		log.Info().Str("name", o.name).Str("pilot", o.pilot).Msg("Paddles: carrier found")
	}
	if id == t.carrierID {
		t.track = append(t.track, shipFix{at: now, lat: lat, lon: lon})
		for len(t.track) > 2 && now.Sub(t.track[0].at) > 15*time.Second {
			t.track = t.track[1:]
		}
		return
	}
	if !strings.Contains(o.typ, "FixedWing") && !strings.Contains(o.typ, "Rotorcraft") {
		return
	}
	s, ok := t.ship()
	if !ok {
		return
	}
	// Only bother inside ~5 nm of the boat: the Case I pattern's initial is 3
	// nm astern of it (pkg/lso pattern.go).
	if math.Abs(lat-s.Lat) > 0.09 || math.Abs(lon-s.Lon) > 0.12 {
		return
	}
	smp := lso.Sample{At: now, Lat: lat, Lon: lon, AltM: o.altM, PitchDeg: o.pitch, HasPitch: o.hasPitch}
	// Any aircraft, AI too, on the landing area fouls the deck.
	if t.feed.tr.P.InLandingArea(s, smp) {
		seen, ok := t.onDeck[id]
		if !ok {
			seen[0] = now
		}
		seen[1] = now
		t.onDeck[id] = seen
	} else {
		delete(t.onDeck, id)
	}
	// Player jets only fly passes: DCS writes a human's Pilot as "Raider 302 | Jedi".
	if !strings.Contains(o.typ, "FixedWing") || !strings.Contains(o.pilot, "|") {
		return
	}
	cs := strings.TrimSpace(o.pilot[:strings.Index(o.pilot, "|")])
	t.feed.lock()
	t.feed.tr.SetDeckFoul(t.deckFoul(now, id))
	c := t.feed.tr.Update(cs, o.name, s, smp)
	if now.Sub(t.lastExpire) > time.Second {
		t.feed.tr.Expire(now)
		t.lastExpire = now
	}
	t.feed.unlock()
	if c != "" {
		log.Info().Str("callsign", cs).Str("call", string(c)).Msg("Paddles: groove call")
		t.feed.say(c)
	}
}

// deckFoul: something other than the jet in the groove (self) has been on the
// landing area in the last 3 s, and not so long that it is parked there. Over
// the ramp the landing jet itself is inside the area.
//
// The return value excludes self; the logged deck state does not, so the log
// changes when the deck does, not each time a different jet's update arrives.
func (t *lsoTacview) deckFoul(now time.Time, self string) bool {
	foulForSelf, anyFoul := false, ""
	for id, seen := range t.onDeck {
		switch {
		case now.Sub(seen[1]) > onDeckKeep:
			delete(t.onDeck, id) // left the picture
		case now.Sub(seen[0]) >= foulMaxAge:
			// parked
		default:
			anyFoul = id
			if id != self {
				foulForSelf = true
			}
		}
	}
	if (anyFoul != "") != t.lastFoul {
		t.lastFoul = anyFoul != ""
		name := ""
		if o := t.objs[anyFoul]; o != nil {
			name = o.pilot
		}
		log.Info().Bool("foul", t.lastFoul).Str("by", name).Msg("Paddles: landing area")
	}
	return foulForSelf
}

// runLSOTacview mirrors runMiniTacview's connection and handshake.
func runLSOTacview(ctx context.Context, addr string, feed *lsoFeed) {
	if addr == "" {
		log.Warn().Msg("Paddles: no --tacview-addr - groove calls off, ball calls still answered")
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		log.Info().Str("addr", addr).Msg("Paddles: Tacview connected")
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		conn.Write([]byte("XtraLib.Stream.0\nTacview.RealTimeTelemetry.0\nvSFG7-Paddles\n0\x00"))
		tv := newLSOTacview(feed)
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 65536), 65536)
		for sc.Scan() {
			conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
			select {
			case <-ctx.Done():
				conn.Close()
				return
			default:
			}
			tv.handleLine(sc.Text())
		}
		conn.Close()
		log.Warn().Msg("Paddles: Tacview disconnected, reconnecting in 5s")
		time.Sleep(5 * time.Second)
	}
}
