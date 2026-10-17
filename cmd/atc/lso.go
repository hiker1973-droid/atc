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
)

// registerLSOFlags is called from the flag block in main.go.
func registerLSOFlags(f *flag.FlagSet) {
	f.BoolVar(&flagLSOOnly, "lso-only", false, "Run as the carrier LSO (Paddles) only")
	f.Float64Var(&flagLSOFreq, "lso-freq", 128.3, "Paddles frequency in MHz")
	f.StringVar(&flagLSOName, "lso-name", "Paddles", "Paddles SRS client name")
	f.StringVar(&flagLSOVoice, "lso-voice", "ash", "Paddles OpenAI TTS voice")
	f.Float64Var(&flagLSOLateral, "lso-landing-lateral-m", 0,
		"Landing point offset from the carrier centreline in metres, starboard positive (calibrate from logged traps)")
}

const styleLSO = "Speak as a US Navy landing signal officer on the platform. " +
	"Calm, firm, very short. Even and steady for corrections; urgent and loud for wave off and bolter."

func lsoVoice() voiceProfile {
	return voiceProfile{voice: flagLSOVoice, speed: flagTTSSpeed, instructions: styleLSO}
}

// lsoTX is one queued call. Calls older than their limit when the radio frees
// up are dropped: "power" two seconds late is wrong.
type lsoTX struct {
	text   string
	queued time.Time
}

func lsoMaxAge(text string) time.Duration {
	switch lso.Call(text) {
	case lso.CallWaveOff, lso.CallBolter:
		return 3 * time.Second
	case lso.CallRogerBall, lso.CallClara:
		return 4 * time.Second
	}
	return 1500 * time.Millisecond
}

func runLSO(ctx context.Context, apiKey string) {
	vp := lsoVoice()
	srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)

	// Render every fixed call now so a call costs only the ExternalAudio start.
	for _, c := range lso.AllCalls {
		if _, err := synthesizeSpeech(ctx, apiKey, string(c), vp); err != nil {
			log.Warn().Err(err).Str("text", string(c)).Msg("Paddles: TTS prewarm failed")
		}
	}
	log.Info().Int("calls", len(lso.AllCalls)).Msg("Paddles: calls rendered")

	txq := make(chan lsoTX, 8)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case tx := <-txq:
				if age := time.Since(tx.queued); age > lsoMaxAge(tx.text) {
					log.Info().Str("text", tx.text).Dur("late", age).Msg("Paddles: call dropped, too late")
					continue
				}
				mp3, err := synthesizeSpeech(ctx, apiKey, tx.text, vp)
				if err != nil {
					log.Error().Err(err).Str("text", tx.text).Msg("Paddles: TTS failed")
					continue
				}
				log.Info().Str("text", tx.text).Float64("freq", flagLSOFreq).Dur("lag", time.Since(tx.queued)).Msg("Paddles TX")
				noteOwnTX(flagLSOFreq, tx.text)
				transmitExternalAudioFile(ctx, mp3, flagLSOFreq, flagLSOName, srsHost, srsPort, flagExternalAudio)
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

	go voiceNetLoop(ctx, flagSRSAddr, flagLSOFreq, flagLSOName, apiKey, flagEAMPassword, vp, flagExternalAudio,
		lsoWhisperPrompt, nil, func(text string) (string, string) {
			return feed.heard(text)
		})
	go runLSOTacview(ctx, flagTacviewAddr, feed)
	log.Info().Float64("freq", flagLSOFreq).Str("name", flagLSOName).Str("voice", flagLSOVoice).
		Float64("lateralM", flagLSOLateral).Msg("Paddles starting")
	<-ctx.Done()
}

const lsoWhisperPrompt = "Paddles, Raider, Venom, Hornet ball, Tomcat ball, Clara, ball, radio check, state"

// lsoBallRe: "302, Hornet ball, 5.5" (Whisper also writes "Hornetball" and,
// on 306.3 on 2026-10-16/17, "bolt" for "ball").
var (
	lsoBallRe   = regexp.MustCompile(`(?i)\b(?:hornet|tomcat|viper|rhino|super hornet|goshawk)\s*(?:ball|bolt|bowl)\b|\bball\b`)
	lsoClaraRe  = regexp.MustCompile(`(?i)\bclara\b|\bclarence\b`)
	lsoModexRe  = regexp.MustCompile(`\b(\d)[\s-]?(\d)[\s-]?(\d)\b`)
	lsoRadioChk = regexp.MustCompile(`(?i)radio check|comm check|com check|comms check|how copy`)
)

// lsoFeed is the shared state: the tracker (Tacview goroutine) and the radio
// (voiceNetLoop goroutine) both touch it.
type lsoFeed struct {
	mu  sync.Mutex
	tr  *lso.Tracker
	say func(lso.Call)
}

func newLSOFeed(p lso.Params, say func(lso.Call)) *lsoFeed {
	f := &lsoFeed{tr: lso.NewTracker(p), say: say}
	f.tr.Done = func(summary string) { log.Info().Str("pass", summary).Msg("Paddles: pass") }
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
	case lsoClaraRe.MatchString(text) || lsoBallRe.MatchString(text):
		clara := lsoClaraRe.MatchString(text)
		f.lock()
		c := f.tr.BallCall(modex, clara, time.Now())
		f.unlock()
		log.Info().Str("modex", modex).Bool("clara", clara).Str("text", text).Msg("Paddles: ball call")
		f.say(c)
		return modex, netHandled
	case lsoRadioChk.MatchString(text):
		cs := extractCallsignSkippingAddress(text, "paddles", "patels", "paddle")
		if cs == "" {
			return "", ""
		}
		return cs, cs + ", Paddles, loud and clear."
	}
	return "", ""
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
}

func newLSOTacview(feed *lsoFeed) *lsoTacview {
	return &lsoTacview{feed: feed, objs: make(map[string]*lsoObj), now: time.Now}
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
	// Player jets only: DCS writes a human's Pilot as "Raider 302 | Jedi".
	if !strings.Contains(o.typ, "FixedWing") || !strings.Contains(o.pilot, "|") {
		return
	}
	s, ok := t.ship()
	if !ok {
		return
	}
	// Only bother inside ~3 nm of the boat.
	if math.Abs(lat-s.Lat) > 0.06 || math.Abs(lon-s.Lon) > 0.08 {
		return
	}
	cs := strings.TrimSpace(o.pilot[:strings.Index(o.pilot, "|")])
	t.feed.lock()
	c := t.feed.tr.Update(cs, o.name, s, lso.Sample{At: now, Lat: lat, Lon: lon, AltM: o.altM, PitchDeg: o.pitch, HasPitch: o.hasPitch})
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
