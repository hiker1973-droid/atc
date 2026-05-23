// Command atc runs AI-powered Tower ATC for DCS World airfields.
// TX: DCS-SR-ExternalAudio.exe (proven SRS injection)
// RX: SRS TCP sync + UDP voice packets → OGG/Opus → OpenAI Whisper → ParseIntent
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"runtime/debug"
	"path/filepath"
	"io"
	"math"
	"mime/multipart"
	"sync/atomic"
	"strings"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

// transmission buffers incoming Opus voice frames per SRS client origin.
type transmission struct {
	opusFrames  [][]byte
	lastPacket  time.Time
	firstPacket time.Time
}

var (
	flagAirfield      string
	flagCallsign      string
	flagSRSAddr       string
	flagFreqMHz       float64
	flagOpenAIKey     string
	flagLogLevel      string
	flagExternalAudio string
	flagEAMPassword    string
	flagFFmpeg          string
	flagTTSVoice        string
	flagTTSVoiceMale    string
	flagVoiceRotateHrs  int
	flagMarshalFreq     string
	flagDeckbossFreq    string
	flagDashboardPort   int
	flagCommandOnly     bool
	flagATISOnly        bool
	flagNoATIS          bool
	flagStaticWindDir   float64
	flagStaticWindKts   float64
	flagStaticCeilFt    float64
	flagStaticVisSm     float64
	flagStaticAltInHg   float64
	flagStaticTempC     float64
	flagStaticNight     bool
	flagTacviewAddr     string
	flagATISFreq        string
	flagATISBroadcast   bool
	flagCommandFreq     string
	flagCommandName     string
	flagCommandVoice    string
	flagDeckbossVoice   string
	flagMarshalVoice    string
	flagRadioEffect          bool
	flagRadioIntensity       string
	flagTowerRadioIntensity  string
	flagATISRadioIntensity   string
	flagTTSSpeed             float64
	flagScudwatchOnly       bool
	flagScudwatchFreq       string
	flagScudwatchCallsign   string
	flagScudwatchBullseye   string
	flagMarshalOnly         bool
	flagMarshalTestTx       bool
	flagCommandTestTx       bool
	flagPprofPort           int
	flagRunwayRotation      bool
)

func main() {
	// Tune Go GC to return memory to OS more aggressively
	// GOGC=50 runs GC twice as often, keeping heap smaller
	// debug.SetMemoryLimit caps total Go memory usage
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(512 * 1024 * 1024) // 512MB hard cap per instance
	root := &cobra.Command{
		Use:   "atc",
		Short: "vSFG-7 AI Tower ATC for DCS World",
		RunE:  run,
	}
	f := root.Flags()
	f.StringVar(&flagAirfield, "airfield", "OMDM", "ICAO: OMDM, OMAM, OMAL")
	f.StringVar(&flagCallsign, "callsign", "", "Tower callsign (auto from airfield)")
	f.StringVar(&flagSRSAddr, "srs-addr", "localhost:5004", "SRS server address:port")
	f.Float64Var(&flagFreqMHz, "freq", 0, "Override tower frequency MHz")
	f.StringVar(&flagOpenAIKey, "openai-key", "", "OpenAI API key")
	f.StringVar(&flagLogLevel, "log-level", "info", "Log level")

	f.StringVar(&flagCommandFreq, "command-freq", "0",
		"Command channel frequency in MHz (0=disabled)")
	f.StringVar(&flagCommandName, "command-name", "vSFG-7-Command",
		"Command channel SRS name (no spaces)")
	f.BoolVar(&flagNoATIS, "no-atis", false,
		"Disable ATIS broadcaster (use when Training VM handles ATIS)")
	f.BoolVar(&flagATISOnly, "atis-only", false,
		"Run as ATIS broadcaster only — no tower, no marshal, no command")
	f.Float64Var(&flagStaticWindDir, "static-wind-dir", -1, "Static wind direction degrees (Training VM)")
	f.Float64Var(&flagStaticWindKts, "static-wind-kts", 0,  "Static wind speed knots (Training VM)")
	f.Float64Var(&flagStaticCeilFt,  "static-ceil-ft",  -1, "Static ceiling feet (Training VM)")
	f.Float64Var(&flagStaticVisSm,   "static-vis-sm",   10, "Static visibility statute miles (Training VM)")
	f.Float64Var(&flagStaticAltInHg, "static-alt-inhg", 29.92, "Static altimeter inHg (Training VM)")
	f.Float64Var(&flagStaticTempC,   "static-temp-c",   15, "Static temperature Celsius (Training VM)")
	f.BoolVar(&flagStaticNight,      "static-night",    false, "Treat mission as night (sets IsNight + 'VFR Night' mode). Real-world server clock is unrelated to DCS mission time, so this must be set explicitly.")
	f.BoolVar(&flagCommandOnly, "command-only", false,
		"Run as command channel only — no tower, no ATIS, no marshal")
	f.StringVar(&flagCommandVoice, "command-voice", "onyx",
		"OpenAI TTS voice for command channel: onyx recommended for male controller")
	f.BoolVar(&flagATISBroadcast, "atis-broadcast", false,
		"Broadcast ATIS on airfield ATIS frequency (replaces MOOSE ATIS)")
	f.StringVar(&flagATISFreq, "atis-freq", "0",
		"ATIS frequency in MHz to monitor for weather (0=auto from airfield)")
	f.StringVar(&flagTacviewAddr, "tacview-addr", "192.168.1.221:42676",
		"Tacview real-time telemetry address:port")
	f.IntVar(&flagDashboardPort, "dashboard-port", 0,
		"HTTP dashboard port (0=disabled). OMDM=6001, OMAM=6002, OMAL=6003")
	f.StringVar(&flagDeckbossFreq, "deckboss-freq", "0",
		"Deckboss frequency MHz (OMDM only, e.g. 128.6 — DCS carrier UHF)")
	f.StringVar(&flagDeckbossVoice, "deckboss-voice", "ash",
		"OpenAI TTS voice for Deckboss: ash (default, calm authoritative), echo (mid male), ballad, sage, onyx, etc.")
	f.StringVar(&flagMarshalVoice, "marshal-voice", "ballad",
		"OpenAI TTS voice for Marshal: ballad (default, naval-controller feel), verse, sage, onyx, etc.")
	f.StringVar(&flagMarshalFreq, "marshal-freq", "0",
		"Marshal frequency MHz (OMDM only, e.g. 306.3)")
	f.StringVar(&flagTTSVoice, "tts-voice", "nova",
		"OpenAI TTS voice (female slot): alloy, ash, coral, echo, fable, nova, onyx, sage, shimmer")
	f.StringVar(&flagTTSVoiceMale, "tts-voice-male", "onyx",
		"OpenAI TTS voice (male slot): alloy, ash, coral, echo, fable, nova, onyx, sage, shimmer")
	f.IntVar(&flagVoiceRotateHrs, "voice-rotate-hours", 4,
		"Hours between female/male voice rotation on the tower channel (0=disabled)")
	f.StringVar(&flagFFmpeg, "ffmpeg",
		`C:\ffmpeg-master-latest-win64-gpl\bin\ffmpeg.exe`,
		"Path to ffmpeg.exe for audio conversion")
	f.StringVar(&flagEAMPassword, "eam-password", "blue42",
		"SRS External AWACS Mode password")
	f.StringVar(&flagExternalAudio, "external-audio",
		`C:\Program Files\Eagle Dynamics\SRS\ExternalAudio\DCS-SR-ExternalAudio.exe`,
		"Path to DCS-SR-ExternalAudio.exe")
	f.BoolVar(&flagRadioEffect, "radio-effect", true,
		"Apply radio/static effect to TTS audio before SRS injection")
	f.StringVar(&flagRadioIntensity, "radio-effect-intensity", "medium",
		"Radio effect intensity for command/deckboss/marshal: light, medium, heavy, extreme")
	f.StringVar(&flagTowerRadioIntensity, "tower-radio-intensity", "heavy",
		"Radio effect intensity for tower TX: light, medium, heavy, extreme")
	f.StringVar(&flagATISRadioIntensity, "atis-radio-intensity", "light",
		"Radio effect intensity for ATIS TX (kept clean since it's a recorded loop): light, medium, heavy, extreme")
	f.Float64Var(&flagTTSSpeed, "tts-speed", 1.05,
		"TTS playback speed for tower/marshal/command/deckboss (ATIS stays at 0.97 for clarity)")
	f.BoolVar(&flagScudwatchOnly, "scudwatch-only", false,
		"Run as Scud / launch-warning monitor only — no tower, ATIS, deckboss, or marshal")
	f.StringVar(&flagScudwatchFreq, "scudwatch-freq", "282.00",
		"Scudwatch broadcast frequency in MHz")
	f.StringVar(&flagScudwatchCallsign, "scudwatch-callsign", "SENTINEL",
		"Scudwatch SRS display callsign (dashes between syllables, no spaces — e.g. Darkstar-1-1)")
	f.StringVar(&flagScudwatchBullseye, "scudwatch-bullseye", "",
		"Bullseye reference for Scudwatch announcements as 'lat,lon' (default: airfield center)")
	f.BoolVar(&flagMarshalOnly, "marshal-only", false,
		"Run as carrier Marshal only — no tower, ATIS, command, deckboss, scudwatch")
	f.BoolVar(&flagMarshalTestTx, "marshal-test-tx", false,
		"DEBUG: have Marshal transmit \"test\" every 30s (used to verify SRS routing — leave off in production)")
	f.BoolVar(&flagCommandTestTx, "command-test-tx", false,
		"DEBUG: have Command transmit \"command test\" every 30s (used to verify SRS routing — leave off in production)")
	f.IntVar(&flagPprofPort, "pprof-port", 0,
		"Run net/http/pprof on this localhost port for goroutine/heap debugging (0=disabled)")
	f.BoolVar(&flagRunwayRotation, "runway-rotation", true,
		"Rotate active runway every 4h instead of selecting by wind. Disable with --runway-rotation=false to fall back to wind-driven selection.")
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// rotateLogIfNeeded inspects logPath at startup and, if it exceeds 50 MB,
// rolls it: existing .4 is dropped, .3 -> .4, .2 -> .3, .1 -> .2, current
// -> .1, and a fresh empty file is created on the next OpenFile. Errors are
// swallowed silently — rotation is best-effort housekeeping, never worth
// blocking startup over.
func rotateLogIfNeeded(logPath string) {
	const maxBytes = 50 * 1024 * 1024
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxBytes {
		return
	}
	const keep = 4
	// Drop the oldest, then shift each remaining backup one slot older.
	_ = os.Remove(fmt.Sprintf("%s.%d", logPath, keep))
	for i := keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", logPath, i), fmt.Sprintf("%s.%d", logPath, i+1))
	}
	_ = os.Rename(logPath, logPath+".1")
}

func run(cmd *cobra.Command, args []string) error {
	level, _ := zerolog.ParseLevel(flagLogLevel)
	// Write logs to both stderr (console) and a rotating log file
	logDir := "C:\\SkyeyeATC\\logs"
	os.MkdirAll(logDir, 0755)
	// Pick a sane log filename. --airfield defaults to "OMDM" even for
	// ATIS-only / Command-only modes, so we have to let the mode flag win or
	// these processes silently piggyback on atc-omdm.log instead of writing
	// their own file.
	logSlug := strings.ToLower(flagAirfield)
	switch {
	case flagATISOnly:
		logSlug = "atis"
	case flagCommandOnly:
		logSlug = "command"
	case flagMarshalOnly:
		logSlug = "marshal"
	case logSlug == "":
		logSlug = "misc"
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("atc-%s.log", logSlug))
	rotateLogIfNeeded(logPath)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(level)
		log.Warn().Err(err).Msg("could not open log file — logging to stderr only")
	} else {
		multiWriter := zerolog.MultiLevelWriter(
			zerolog.ConsoleWriter{Out: os.Stderr},
			logFile,
		)
		log.Logger = zerolog.New(multiWriter).With().Timestamp().Logger().Level(level)
		log.Info().Str("logFile", logPath).Msg("logging to file")
	}

	if flagPprofPort > 0 {
		addr := fmt.Sprintf("localhost:%d", flagPprofPort)
		go func() {
			log.Info().Str("addr", addr).Int("goroutines", runtime.NumGoroutine()).Msg("pprof debug server starting")
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Warn().Err(err).Msg("pprof server exited")
			}
		}()
	}

	var af *airfield.Airfield
	switch flagAirfield {
	case "OMDM":
		af = airfield.OMDM
	case "OMAM":
		af = airfield.OMAM
	case "OMAL":
		af = airfield.OMAL
	default:
		return fmt.Errorf("unknown airfield %q", flagAirfield)
	}

	callsign := flagCallsign
	if callsign == "" {
		callsign = af.Name + " Tower"
	}
	freqMHz := flagFreqMHz
	if freqMHz == 0 {
		freqMHz = af.TowerFreqMHz
	}

	apiKey := flagOpenAIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("OpenAI API key required")
	}

	if _, err := os.Stat(flagExternalAudio); err != nil {
		log.Warn().Str("path", flagExternalAudio).Msg("ExternalAudio not found at path — TX will fail")
	}

	mode := "tower"
	if flagCommandOnly   { mode = "command-only" }
	if flagATISOnly      { mode = "atis-only" }
	if flagScudwatchOnly { mode = "scudwatch" }
	if flagMarshalOnly   { mode = "marshal-only" }
	if flagDeckbossFreq != "0" && flagDeckbossFreq != "" { mode = "deckboss" }
	log.Info().Str("mode", mode).Str("srs", flagSRSAddr).Msg("vsfg7-atc starting")
	if !flagCommandOnly && !flagATISOnly && !flagScudwatchOnly && !flagMarshalOnly {
		log.Info().
			Str("airfield", af.ICAO).
			Str("callsign", callsign).
			Float64("freqMHz", freqMHz).
			Str("srs", flagSRSAddr).
			Msg("starting vsfg7-atc")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Info().Msg("shutting down")
		cancel()
	}()

	var wg sync.WaitGroup


	// ── ATC controller ────────────────────────────────────────────────────────
	atcCtrl := controller.NewATCController(callsign, af)
	if !flagRunwayRotation {
		atcCtrl.DisableRunwayRotation()
	}
	// Seed weather from --static-* flags so dashboard /status reports real
	// values instead of zeros. UpdateFlightConditions wires CeilingFt /
	// VisibilityNm / IsNight, which UpdateWeather alone does not. IsNight is
	// only the initial seed — once Tacview ReferenceTime + #offset arrive, the
	// dashboard derives day/night from mission time directly.
	atcCtrl.SetFullWeather(flagStaticWindDir, flagStaticWindKts, flagStaticCeilFt, flagStaticVisSm, flagStaticAltInHg, flagStaticNight)
	marshStack := state.NewMarshalStack()

	// ATIS-only mode — Training VM, static weather
	if flagATISOnly {
		log.Info().Str("srs", flagSRSAddr).Msg("ATIS-only mode starting")
		if flagDashboardPort > 0 {

		}
		atisOnlyLoop(ctx, flagSRSAddr, apiKey, flagEAMPassword)
		return nil
	}

	// Scud / launch-warning watch mode — skip tower, ATIS, deckboss, marshal.
	// Runs two monitors off one Tacview connection and broadcasts alerts
	// on flagScudwatchFreq via DCS-SR-ExternalAudio.exe.
	if flagScudwatchOnly {
		var scudFreqMHz float64
		fmt.Sscanf(flagScudwatchFreq, "%f", &scudFreqMHz)
		if scudFreqMHz == 0 {
			scudFreqMHz = 282.0
		}
		_, srsPort, _ := net.SplitHostPort(flagSRSAddr)

		// Bullseye defaults to the selected airfield's center if not given.
		// Format: "lat,lon" — orb.Point is [lon,lat].
		bullseye := af.Center
		if flagScudwatchBullseye != "" {
			var bLat, bLon float64
			if n, _ := fmt.Sscanf(flagScudwatchBullseye, "%f,%f", &bLat, &bLon); n == 2 {
				bullseye = orb.Point{bLon, bLat}
			} else {
				log.Warn().Str("value", flagScudwatchBullseye).Msg("could not parse --scudwatch-bullseye, falling back to airfield center")
			}
		}

		log.Info().
			Float64("freq", scudFreqMHz).
			Str("callsign", flagScudwatchCallsign).
			Float64("bullLat", bullseye[1]).
			Float64("bullLon", bullseye[0]).
			Str("tacview", flagTacviewAddr).
			Msg("Scudwatch mode starting")

		txQueue := make(chan string, 8)
		announce := func(text string) {
			select {
			case txQueue <- text:
			default:
				log.Warn().Str("text", text).Msg("scudwatch TX queue full — dropping")
			}
		}
		scudCh := make(chan scudwatchEvent, 64)
		launchCh := make(chan scudwatchEvent, 64)

		wg.Add(1)
		go func() {
			defer wg.Done()
			scudwatchTXWorker(ctx, txQueue, scudFreqMHz, flagScudwatchCallsign,
				apiKey, flagTTSVoice, srsPort, flagExternalAudio)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			startScudMonitor(ctx, scudCh, announce, bullseye, flagScudwatchCallsign)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			startLaunchWarningMonitor(ctx, launchCh, announce, bullseye, flagScudwatchCallsign)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			scudwatchTacviewFeed(ctx, flagTacviewAddr, scudCh, launchCh)
		}()

		<-ctx.Done()
		close(txQueue)
		wg.Wait()
		log.Info().Msg("Scudwatch offline")
		return nil
	}

	// Marshal-only mode — run the carrier marshal loop standalone so it can
	// be started / stopped independently of any tower instance.
	if flagMarshalOnly {
		if flagMarshalFreq == "0" || flagMarshalFreq == "" {
			return fmt.Errorf("--marshal-freq required with --marshal-only")
		}
		var marshalFreqMHz float64
		fmt.Sscanf(flagMarshalFreq, "%f", &marshalFreqMHz)
		log.Info().
			Str("airfield", af.ICAO).
			Float64("freq", marshalFreqMHz).
			Str("srs", flagSRSAddr).
			Msg("Marshal-only mode starting")

		// atcCtrl / marshStack were already constructed above — marshalLoop
		// needs the controller for weather/state context.
		var marshalCooldown int64
		var tacviewConnected int32
		var tacviewLastData int64

		wg.Add(1)
		go func() {
			defer wg.Done()
			atcCtrl.Run(ctx)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			tacviewLoop(ctx, flagTacviewAddr, atcCtrl, &tacviewConnected, &tacviewLastData)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			marshalLoop(ctx, flagSRSAddr, marshalFreqMHz, apiKey, flagEAMPassword,
				flagMarshalVoice, &marshalCooldown, atcCtrl, marshStack)
		}()

		// Expose marshStack for the ops dashboard when --dashboard-port is set.
		// deckState is nil here — marshal-only mode never populates cats.
		if flagDashboardPort > 0 {
			ds := newDashboardServer(flagDashboardPort, "Marshal", af, atcCtrl, marshStack, nil)
			wg.Add(1)
			go func() {
				defer wg.Done()
				ds.run(ctx)
			}()
			log.Info().Int("port", flagDashboardPort).Msg("Marshal dashboard started")
		}

		<-ctx.Done()
		wg.Wait()
		log.Info().Msg("Marshal offline")
		return nil
	}

	// Command-only mode — skip tower, ATIS, marshal, deckboss
	if flagCommandOnly {
		if flagCommandFreq == "0" || flagCommandFreq == "" {
			return fmt.Errorf("--command-freq required with --command-only")
		}
		var cmdFreq float64
		fmt.Sscanf(flagCommandFreq, "%f", &cmdFreq)
		log.Info().Float64("freq", cmdFreq).Str("name", flagCommandName).Str("srs", flagSRSAddr).Msg("Command channel starting")

		// Phase 3b: Tacview-driven tower handoff. If --tacview-addr is set,
		// spawn a mini Tacview consumer and a watcher that TXs a tower handoff
		// when a tracked pilot crosses 30 NM inbound to any of OMDM/OMAM/OMAL.
		var tracker *pilotTracker
		if flagTacviewAddr != "" {
			tracker = newPilotTracker()
			store := newTacviewPositions()
			go runMiniTacview(ctx, flagTacviewAddr, store)
			srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)
			handoffTX := func(text, _ string) {
				mp3, err := synthesizeSpeech(ctx, apiKey, text, flagCommandVoice, flagTTSSpeed)
				if err != nil {
					log.Error().Err(err).Msg("Command handoff TTS failed")
					return
				}
				transmitExternalAudioFile(ctx, mp3, cmdFreq, flagCommandName, srsHost, srsPort, flagExternalAudio)
			}
			go runCommandHandoffWatch(ctx, tracker, store, handoffTX)
			log.Info().Str("tacview", flagTacviewAddr).Msg("Command: tower-handoff watcher armed")
		}

		commandLoop(ctx, flagSRSAddr, cmdFreq, flagCommandName, apiKey, flagEAMPassword, flagCommandVoice, flagExternalAudio, tracker)
		return nil
	}

	// ── TX: ExternalAudio ─────────────────────────────────────────────────────
	txChan := make(chan string, 16)
	atcCtrl.SetTransmitFn(func(ctx context.Context, text string) {
		select {
		case txChan <- text:
		default:
			log.Warn().Str("text", text).Msg("TX queue full")
		}
	})

	srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)
	var marshalCooldown, deckbossCooldown int64
	var txCooldown int64  // unix nano when cooldown expires
	var tacviewConnected int32 // atomic bool — 1=connected 0=disconnected
	var tacviewLastData int64  // unix nano of last Tacview data received
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case text := <-txChan:
				voice := currentTowerVoice()
				log.Info().Str("text", text).Str("voice", voice).Int("queued", len(txChan)).Msg("TX via OpenAI TTS + ExternalAudio")
				// 10s ceiling on TTS: OpenAI is either fast (~2s) or hung. No
				// SAPI fallback — operator preference is one consistent voice;
				// pilot will repeat if TX drops.
				ttsCtx, ttsCancel := context.WithTimeout(ctx, 10*time.Second)
				ttsStart := time.Now()
				mp3, err := synthesizeSpeech(ttsCtx, apiKey, text, voice, flagTTSSpeed)
				ttsCancel()
				ttsMs := time.Since(ttsStart).Milliseconds()
				if err != nil {
					log.Warn().Err(err).Int64("ttsMs", ttsMs).Str("text", text).Msg("TTS synthesis failed — dropping TX")
					continue
				}
				// Set cooldown HERE, not at dequeue: covers the actual playback
				// window starting now. Setting at dequeue lets a slow TTS expire
				// the cooldown before audio plays, which is how self-loopback got
				// re-introduced via the (now-removed) SAPI fallback path.
				atomic.StoreInt64(&txCooldown, time.Now().Add(estimateTTSDuration(text)).UnixNano())
				eaCtx, eaCancel := context.WithTimeout(ctx, 30*time.Second)
				eaStart := time.Now()
				transmitExternalAudioFile(eaCtx, mp3, freqMHz, af.ICAO+"-ATC", srsHost, srsPort, flagExternalAudio)
				eaCancel()
				eaMs := time.Since(eaStart).Milliseconds()
				log.Info().Int64("ttsMs", ttsMs).Int64("eaMs", eaMs).Msg("TX done")
			}
		}
	}()

	// ── RX: SRS TCP sync + UDP voice → Whisper ────────────────────────────────
	// Skip tower srsLoop if this is a deckboss-only instance
	if flagDeckbossFreq == "0" || flagDeckbossFreq == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Stagger SRS connection by airfield to avoid overwhelming server at startup
			staggerMap := map[string]time.Duration{
				"OMDM": 0,
				"OMAM": 3 * time.Second,
				"OMAL": 6 * time.Second,
			}
			if delay, ok := staggerMap[flagAirfield]; ok && delay > 0 {
				time.Sleep(delay)
			}
			srsLoop(ctx, flagSRSAddr, freqMHz, callsign, apiKey, flagEAMPassword, &txCooldown, atcCtrl)
		}()
	}

	// ── Start controller ──────────────────────────────────────────────────────
	atcCtrl.Run(ctx)

	// ── ATIS weather listener ────────────────────────────────────────────────
	atisFreq := af.ATISFreqMHz
	if flagATISFreq != "0" && flagATISFreq != "" {
		fmt.Sscanf(flagATISFreq, "%f", &atisFreq)
	}
	log.Info().Str("icao", af.ICAO).Msg("ATIS weather parsed from broadcaster")

	// Command channel now runs as a separate process via start_command.bat

	// ── Tacview status monitor ───────────────────────────────────────────────
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if atomic.LoadInt32(&tacviewConnected) == 0 {
					log.Warn().Msg("Tacview telemetry offline — speed warnings and conflict detection disabled")
				} else {
					nano := atomic.LoadInt64(&tacviewLastData)
					if nano == 0 {
						log.Warn().Msg("Tacview connected — no air contacts yet")
					} else if time.Since(time.Unix(0, nano)) > 30*time.Second {
						log.Warn().Msg("Tacview connected but no position data in 30s — is a mission running?")
					} else {
						contactCount := atcCtrl.TacviewContactCount()
						cacheHits, cacheMisses := globalTTSCache.stats()
						log.Info().
							Int("airContacts", contactCount).
							Str("lastData", time.Since(time.Unix(0, nano)).Round(time.Second).String()+" ago").
							Int64("ttsHits", cacheHits).
							Int64("ttsMisses", cacheMisses).
							Msg("Tacview nominal")
					}
				}
			}
		}
	}()

	// ── Tacview telemetry ────────────────────────────────────────────────────
	wg.Add(1)
	go func() {
		defer wg.Done()
		tacviewLoop(ctx, flagTacviewAddr, atcCtrl, &tacviewConnected, &tacviewLastData)
	}()

	log.Info().
		Str("callsign", callsign).
		Float64("freqMHz", freqMHz).
		Msg("ATC online — ready for traffic")

	// Runway rotation ticker — checks every 60s whether the 4h slot has
	// rolled and updates the active runway accordingly. UpdateWeather also
	// reads the rotation, but Tacview-driven weather updates can be sparse
	// in stable conditions, so the ticker is needed to catch slot rollovers.
	if flagRunwayRotation {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					atcCtrl.RotateRunwayIfDue()
				}
			}
		}()
		log.Info().
			Str("airfield", af.ICAO).
			Str("startRunway", atcCtrl.GetActiveRunway()).
			Msg("runway rotation enabled — 4h slots, ignores wind")
	}

	// Start ATIS broadcast loop — OMDM tower only, not deckboss instance.
	// Opt-in via --atis-broadcast so the tower doesn't double up with a
	// dedicated --atis-only broadcaster process running in parallel.
	if flagAirfield == "OMDM" && flagATISBroadcast && !flagCommandOnly && !flagATISOnly && !flagNoATIS && (flagDeckbossFreq == "0" || flagDeckbossFreq == "") {
		atisStations := []*atisStation{
			{
				Name: "Al Dhafra ATIS", FreqMHz: 248.200, Voice: "nova",
				ICAO: "OMAM", TACAN: "TACAN 96X.", ILS: "ILS 111.10 runway 13 left. ILS 109.10 runway 31 left.",
				Advisory: "vSFG-7 traffic in area. Advise information on initial contact.",
			},
			{
				Name: "Al Minhad ATIS", FreqMHz: 248.300, Voice: "shimmer",
				ICAO: "OMDM", TACAN: "TACAN 99X.", ILS: "ILS 110.70 runway 09. ILS 110.75 runway 27.",
				Advisory: "vSFG-7 traffic in area. Advise information on initial contact.",
			},
			{
				Name: "Liwa ATIS", FreqMHz: 248.550, Voice: "alloy",
				ICAO: "OMAB", TACAN: "TACAN 121X.", ILS: "",
				Advisory: "vSFG-7 traffic in area. Advise information on initial contact.",
			},
			{
				Name: "Al Ain ATIS", FreqMHz: 248.850, Voice: "echo",
				ICAO: "OMAL", TACAN: "TACAN 79X.", ILS: "",
				Advisory: "vSFG-7 traffic in area. Advise information on initial contact.",
			},
			{
				Name: "Khasab ATIS", FreqMHz: 248.500, Voice: "fable",
				ICAO: "OOKB", TACAN: "", ILS: "ILS 110.30 runway 08.",
				Advisory: "Active runway takeoff 01 landing 19. Landing aircraft have priority. vSFG-7 traffic in area. Advise information on initial contact.",
			},
		}
		for i, station := range atisStations {
			s := station
			staggerSec := 15 + i*37 // minimum 15s startup delay + prime stagger to avoid sync with 180s timer
			wg.Add(1)
			go func(st *atisStation, delay int) {
				defer wg.Done()
				if delay > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(delay) * time.Second):
					}
				}
				atisLoop(ctx, st, apiKey, flagEAMPassword, flagSRSAddr, atcCtrl, 45)
			}(s, staggerSec)
		}
		log.Info().Int("stations", len(atisStations)).Msg("ATIS broadcaster started")
	}

	// Start deckboss loop if freq configured (OMDM only)
	deckState := state.NewDeckbossState()
	if flagDeckbossFreq != "0" && flagDeckbossFreq != "" {
		var deckFreqMHz float64
		fmt.Sscanf(flagDeckbossFreq, "%f", &deckFreqMHz)
		wg.Add(1)
		go func() {
			defer wg.Done()
			deckbossLoop(ctx, flagSRSAddr, deckFreqMHz, apiKey, flagEAMPassword,
				flagDeckbossVoice, &deckbossCooldown, atcCtrl, deckState)
		}()
		log.Info().Float64("freq", deckFreqMHz).Msg("Deckboss online")
	}

	// Start marshal loop if freq configured (OMDM only)
	if flagMarshalFreq != "0" && flagMarshalFreq != "" {
		var marshalFreqMHz float64
		fmt.Sscanf(flagMarshalFreq, "%f", &marshalFreqMHz)
		wg.Add(1)
		go func() {
			defer wg.Done()
			marshalLoop(ctx, flagSRSAddr, marshalFreqMHz, apiKey, flagEAMPassword, flagMarshalVoice, &marshalCooldown, atcCtrl, marshStack)
		}()
		log.Info().Float64("freq", marshalFreqMHz).Msg("Marshal stack online")
	}

	// Start dashboard HTTP server if port configured
	if flagDashboardPort > 0 {
		ds := newDashboardServer(flagDashboardPort, callsign, af, atcCtrl, marshStack, deckState)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ds.run(ctx)
		}()
		log.Info().Int("port", flagDashboardPort).Msg("Dashboard server started")
	}

	// Pre-warm TTS cache in background — don't block startup. Warm both
	// rotation voices so a flip mid-session doesn't cause a cold-cache spike.
	go prewarmTTSCache(ctx, apiKey, flagTTSVoice, af.RunwayPairs[0].Primary.Designator)
	if flagVoiceRotateHrs > 0 && flagTTSVoiceMale != flagTTSVoice {
		go prewarmTTSCache(ctx, apiKey, flagTTSVoiceMale, af.RunwayPairs[0].Primary.Designator)
	}

	// Warn if Tacview not connected after startup
	go func() {
		time.Sleep(30 * time.Second)
		if atomic.LoadInt32(&tacviewConnected) == 0 {
			log.Warn().Msg("════════════════════════════════════════")
			log.Warn().Msg("TACVIEW OFFLINE — enable real-time telemetry in DCS")
			log.Warn().Msg("Speed warnings and conflict detection DISABLED")
			log.Warn().Msg("════════════════════════════════════════")
		}
	}()

	<-ctx.Done()
	wg.Wait()
	log.Info().Msg("ATC offline")
	return nil
}

// ── SRS loop ──────────────────────────────────────────────────────────────────

const guidLen = 22

// srsLoop manages the SRS TCP+UDP connection lifecycle with reconnection.
// containsAny returns true if s contains any of the given substrings.

// newStaticWeatherController returns a minimal ATCController with fixed weather
// for use by the ATIS-only Training VM mode.
func newStaticWeatherController(windDir, windKts, ceilFt, altInHg float64) *controller.ATCController {
	af := airfield.OMDM // doesn't matter for ATIS-only
	c := controller.NewATCController("ATIS-Static", af)
	c.SetWeather(windDir, windKts, ceilFt, altInHg)
	return c
}

// isWhisperHallucination returns true if Whisper returned the prompt text
// or other known hallucination patterns instead of real speech.
func isWhisperHallucination(text string) bool {
	lower := strings.ToLower(text)
	// Exact prompt echo — any single known prompt fragment triggers rejection
	// Only match when multiple prompt fragments appear together
	// Individual words like "radio check" are legitimate calls
	promptFragments := []string{
		"al minhad tower", "al dhafra tower", "al ain tower",
		"request taxi", "holding short", "cleared for takeoff",
		"runway vacated", "seven dme", "cleared airspace",
	}
	matchCount := 0
	for _, frag := range promptFragments {
		if strings.Contains(lower, frag) {
			matchCount++
		}
	}
	if matchCount >= 3 { // need 3+ matches to be sure it's a prompt echo
		return true
	}
	// Compound fragments that only appear in prompt echoes. Each must be
	// distinctive enough that a real pilot transmission won't ever contain
	// it. "holding short, runway" was previously here but pilots legitimately
	// say "032 holding short, runway 27" — replaced with the longer, prompt-
	// only "holding short, runway, cleared".
	compound := []string{
		"radio check, request taxi",
		"holding short, runway, cleared",
		"base, final, runway",
		"tower, al ain tower",
		"raider, venom, radio",
	}
	for _, c := range compound {
		if strings.Contains(lower, c) {
			return true
		}
	}
	// Too short to be a real call
	if len(strings.TrimSpace(text)) < 6 {
		return true
	}
	// Pure punctuation / numbers with no words
	words := strings.Fields(text)
	if len(words) < 2 {
		return true
	}
	return false
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// extractOriginFromUDP extracts the client GUID from a UDP voice packet.
func extractOriginFromUDP(buf []byte) string {
	if len(buf) < 22 { return "" }
	return string(buf[len(buf)-22:])
}

// extractCallsignSimple extracts callsign from first comma-delimited segment.
func extractCallsignSimple(text string) string {
	text = normalizeCallsignLocal(text)
	parts := strings.Split(text, ",")
	if len(parts) > 0 { return strings.TrimSpace(parts[0]) }
	return ""
}

// extractCallsignSkippingAddress is like extractCallsignSimple but treats the
// first comma segment as the address word (e.g. "Marshal", "Union Marshal",
// "Command") and returns the SECOND segment as the pilot callsign. This is
// the natural shape of pilot calls to non-airfield channels:
//   "Marshal, Raider 032, comm check."   → "Raider 032"
//   "Command, Raider 032 checking in."   → "Raider 032 checking in" → first word(s)
// The `addresses` strings are matched case-insensitively via substring (so
// "Marshall" with the extra L Whisper sometimes adds still matches "marshal").
// If the first segment doesn't match any address, the simple first-segment
// behavior is preserved as a fallback.
func extractCallsignSkippingAddress(text string, addresses ...string) string {
	text = normalizeCallsignLocal(text)
	lower := strings.ToLower(text)
	// Step 1 — strip leading address tokens regardless of comma placement.
	// Whisper produces all of these shapes:
	//   "Marshal, Raider 39, state 5.8"    (comma after address)
	//   "Union Marshal Raider 39, state 5.8" (no comma after address)
	//   "Union Marshal Raider 39 state 5.8"  (no commas at all)
	// Addresses must be passed longest-first so e.g. "union marshal" wins
	// over "marshal" alone. We require the address to be followed by a
	// space, comma, or end-of-string so "marshal" doesn't match inside
	// "marshalling" or similar.
	for _, addr := range addresses {
		addrLower := strings.ToLower(addr)
		if !strings.HasPrefix(lower, addrLower) {
			continue
		}
		rest := text[len(addr):]
		if len(rest) > 0 && rest[0] != ' ' && rest[0] != ',' {
			continue
		}
		rest = strings.TrimLeft(rest, ", ")
		// Step 2 — take everything up to the first comma (request boundary),
		// then trim at the first intent verb. That gives just the callsign.
		if i := strings.Index(rest, ","); i > 0 {
			rest = rest[:i]
		}
		return trimCallsignAtVerb(strings.TrimSpace(rest))
	}
	// Address wasn't at the start — fall back to the simple first-segment
	// behavior (callers can still get something useful even on garbled input).
	parts := strings.Split(text, ",")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

// trimCallsignAtVerb cuts off everything after the first intent-style word in
// a callsign segment. Used when the pilot crams the callsign and the request
// into a single comma segment, e.g. "Raider 032 checking in" → "Raider 032".
// Conservative — only trims at a small set of common verbs so we don't chop a
// legitimate two-word callsign like "Viper Flight".
func trimCallsignAtVerb(s string) string {
	verbs := []string{
		" checking", " check", " requesting", " request", " holding",
		" airborne", " inbound", " departing", " ready", " established",
		" pushing", " switching", " marking", " commencing", " state",
		" current", " say", " what", " comm", " radio", " on station",
		" under", " tension", " shooter",
	}
	lower := strings.ToLower(s)
	cut := len(s)
	for _, v := range verbs {
		if i := strings.Index(lower, v); i > 0 && i < cut {
			cut = i
		}
	}
	return strings.TrimSpace(s[:cut])
}

// transcribeFramesWithPrompt wraps Opus frames and sends to Whisper with a custom prompt.
func transcribeFramesWithPrompt(ctx context.Context, apiKey string, frames [][]byte, prompt string) (string, error) {
	ogg := wrapOpusInOGG(frames)
	wav, err := convertToWAV(flagFFmpeg, ogg)
	if err != nil {
		return "", err
	}
	return whisperTranscribeWithPrompt(ctx, apiKey, wav, "audio.wav", prompt)
}

// transcribeFrames wraps Opus frames and sends to Whisper.
func transcribeFrames(ctx context.Context, apiKey string, frames [][]byte) (string, error) {
	ogg := wrapOpusInOGG(frames)
	wav, err := convertToWAV(flagFFmpeg, ogg)
	if err != nil { return "", err }
	return whisperTranscribe(ctx, apiKey, wav, "audio.wav")
}

// extractFuelStateMarshal parses fuel state from text e.g. "state 5.6".
func extractFuelStateMarshal(lower string) float64 {
	idx := strings.Index(lower, "state")
	if idx < 0 { return 0 }
	after := strings.TrimSpace(lower[idx+5:])
	var val float64
	if n, _ := fmt.Sscanf(after, "%f", &val); n == 1 && val > 0 && val < 30 { return val }
	return 0
}

// normalizeCallsignLocal fixes Whisper mishearings of squadron callsigns.
func normalizeCallsignLocal(text string) string {
	replacements := [][]string{
		// Squadron callsigns
		{"reader", "Raider"}, {"radar", "Raider"}, {"rater", "Raider"}, {"raiders", "Raider"},
		{"vino", "Venom"}, {"venue", "Venom"}, {"demon", "Venom"},
		// Tower callsign mishearings
		{"al dhafra", "Al Dhafra"}, {"dhafra", "Al Dhafra"},
		{"ldaf", "Al Dhafra"}, {"ldafa", "Al Dhafra"}, {"el dhafra", "Al Dhafra"},
		{"al minhad", "Al Minhad"}, {"minhad", "Al Minhad"},
		{"el minhad", "Al Minhad"}, {"el minha", "Al Minhad"},
		{"al ain", "Al Ain"}, {"alan", "Al Ain"}, {"el ain", "Al Ain"},
	}
	lower := strings.ToLower(text)
	for _, pair := range replacements {
		if strings.Contains(lower, pair[0]) {
			idx := strings.Index(lower, pair[0])
			text = text[:idx] + pair[1] + text[idx+len(pair[0]):]
			lower = strings.ToLower(text)
		}
	}
	return text
}

func srsLoop(ctx context.Context, addr string, freqMHz float64, callsign, apiKey, eamPassword string, txCooldown *int64, atcCtrl *controller.ATCController) {
	freqHz := freqMHz * 1e6
	guid := "vsfg7atc" + fmt.Sprintf("%014d", time.Now().UnixNano()%100000000000000)
	if len(guid) > guidLen {
		guid = guid[:guidLen]
	}
	for len(guid) < guidLen {
		guid += "0"
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Info().Str("addr", addr).Msg("connecting to SRS")

		// TCP connection
		tcpConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			log.Warn().Err(err).Msg("SRS TCP failed — retrying in 10s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		// UDP connection (same port)
		udpAddr, _ := net.ResolveUDPAddr("udp", addr)
		udpConn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			log.Warn().Err(err).Msg("SRS UDP failed")
			tcpConn.Close()
			continue
		}

		// Send sync message
		syncMsg := buildSync(guid, callsign, freqHz)
		if _, err := tcpConn.Write(syncMsg); err != nil {
			log.Error().Err(err).Msg("SRS sync write failed")
			tcpConn.Close()
			udpConn.Close()
			continue
		}

		// Send EAM password
		if eamPassword != "" {
			eamMsg := buildEAM(guid, callsign, freqHz, eamPassword)
			if _, err := tcpConn.Write(eamMsg); err != nil {
				log.Error().Err(err).Msg("SRS EAM auth failed")
			} else {
				log.Info().Msg("SRS EAM authenticated")
			}
		}

		log.Info().Str("callsign", callsign).Float64("freqMHz", freqMHz).Msg("SRS registered — listening")

		// Start UDP + TCP ping sender (keeps connection alive)
		pingStop := make(chan struct{})
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-pingStop:
					return
				case <-ticker.C:
					udpConn.Write([]byte(guid))
				}
			}
		}()

		// Start TCP keepalive reader (reads server messages)
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
				// Respond to pings
				var msg map[string]interface{}
				if json.Unmarshal(line, &msg) == nil {
					if msgType, ok := msg["MsgType"].(float64); ok && int(msgType) == 1 {
						// Ping — send sync back
						tcpConn.Write(syncMsg)
					}
				}
			}
		}()

		// Collect voice packets per-origin, transcribe when silence detected
		transmissions := make(map[string]*transmission)
		var txMu sync.Mutex

		// Flush goroutine — check for completed transmissions every 500ms
		flushStop := make(chan struct{})
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-flushStop:
					return
				case <-ticker.C:
					txMu.Lock()
					now := time.Now()
					for origin, tx := range transmissions {
						silent := now.Sub(tx.lastPacket) > 400*time.Millisecond
						tooLong := !tx.firstPacket.IsZero() && now.Sub(tx.firstPacket) > 20*time.Second
						if (silent || tooLong) && len(tx.opusFrames) > 3 {
							frames := tx.opusFrames
							log.Info().Str("origin", origin).Int("frames", len(frames)).Msg("flushing transmission to Whisper")
							delete(transmissions, origin)
							txMu.Unlock()
							go transcribeAndHandle(ctx, apiKey, flagFFmpeg, frames, callsign, atcCtrl)
							txMu.Lock()
						}
					}
					txMu.Unlock()
				}
			}
		}()

		// UDP voice packet loop
		udpBuf := make([]byte, 4096)
		udpDead := false
		for !udpDead {
			select {
			case <-ctx.Done():
				udpDead = true
			case <-tcpDone:
				udpDead = true
			default:
				udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
				n, err := udpConn.Read(udpBuf)
				if err != nil {
					continue
				}
				log.Debug().Int("bytes", n).Msg("UDP packet received")
				if n == guidLen {
					log.Debug().Msg("UDP ping packet — ignoring")
					continue
				}
				if n < 6+1 {
					log.Debug().Int("bytes", n).Msg("UDP packet too small — ignoring")
					continue
				}
				// Decode packet header
				audioLen := binary.LittleEndian.Uint16(udpBuf[2:4])
				if int(audioLen) > n-6 {
					log.Debug().Uint16("audioLen", audioLen).Int("n", n).Msg("audio len exceeds packet — ignoring")
					continue
				}
				opusBytes := make([]byte, audioLen)
				copy(opusBytes, udpBuf[6:6+audioLen])

				// Get origin GUID (last 22 bytes)
				var origin string
				if n >= guidLen {
					origin = string(udpBuf[n-guidLen : n])
				} else {
					origin = "unknown"
				}

				// Check frequency match
				freqMatch := false
				audioEnd := 6 + int(audioLen)
				freqSegLen := binary.LittleEndian.Uint16(udpBuf[4:6])
				log.Debug().
					Uint16("freqSegLen", freqSegLen).
					Float64("targetFreqHz", freqHz).
					Msg("checking frequency")
				for i := 0; i+10 <= int(freqSegLen) && audioEnd+i+10 <= n; i += 10 {
					pktFreq := math.Float64frombits(binary.LittleEndian.Uint64(udpBuf[audioEnd+i : audioEnd+i+8]))
					log.Debug().Float64("pktFreq", pktFreq).Float64("targetFreq", freqHz).Msg("packet frequency")
					if math.Abs(pktFreq-freqHz) < 500 {
						freqMatch = true
						break
					}
				}
				if !freqMatch {
					log.Debug().Msg("frequency mismatch — ignoring packet")
					continue
				}
				log.Debug().Str("origin", origin).Int("opusBytes", len(opusBytes)).Msg("voice packet accepted")

				// TX cooldown — ignore all audio for 4s after we transmit
				if until := atomic.LoadInt64(txCooldown); until > 0 && time.Now().UnixNano() < until {
					log.Debug().Str("origin", origin).Msg("TX cooldown — ignoring")
					continue
				}

				txMu.Lock()
				if transmissions[origin] == nil {
					transmissions[origin] = &transmission{firstPacket: time.Now()}
				}
				transmissions[origin].opusFrames = append(transmissions[origin].opusFrames, opusBytes)
				transmissions[origin].lastPacket = time.Now()
				txMu.Unlock()
			}
		}

		close(pingStop)
		close(flushStop)
		tcpConn.Close()
		udpConn.Close()

		log.Warn().Msg("SRS disconnected — reconnecting in 5s")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// buildEAM sends the External AWACS Mode password to SRS.
// Without this, SRS will not send audio to external clients.
// MsgType 7 = MessageExternalAWACSModePassword
// unitIdForCallsign returns a deterministic, per-role unitId. SRS treats the
// unitId as the in-game DCS object the radio is bonded to; when multiple
// clients share the same unitId, SRS de-duplicates audio routing — whichever
// process most recently refreshed wins. Towers refresh every 10s and were
// shadowing Marshal/Command audio because all clients were hard-coded to
// 100000002. FNV-32 of the callsign keeps the IDs stable across restarts and
// well inside the int32 range SRS expects.
func unitIdForCallsign(callsign string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(callsign))
	// Reserve 100000000..100999999 for our injected radios so they look like
	// normal DCS object IDs without colliding with player slots.
	return 100000000 + int64(h.Sum32()%1000000)
}

func buildEAM(guid, callsign string, freqHz float64, password string) []byte {
	msg := map[string]interface{}{
		"Version": "2.1.0.2",
		"MsgType": 7,
		"ExternalAWACSModePassword": password,
		"Client": map[string]interface{}{
			"ClientGuid":  guid,
			"Name":        callsign,
			"Seat":        0,
			"Coalition":   2,
			"AllowRecord": true,
			"RadioInfo": map[string]interface{}{
				"radios": []map[string]interface{}{
					{
						"freq":       freqHz,
						"modulation": 0,
						"enc":        false,
						"encKey":     0,
						"secFreq":    0.0,
						"retransmit": false,
					},
				},
				"unit":   callsign,
				"unitId": unitIdForCallsign(callsign),
				"iff": map[string]interface{}{
					"control":   0,
					"expansion": false,
					"mode1":     0,
					"mode2":     -1,
					"mode3":     0,
					"mode4":     false,
					"mic":       -1,
					"status":    0,
				},
				"ambient": map[string]interface{}{
					"abAlt":  0.0,
					"abDist": 0.0,
					"vol":    0.0,
				},
			},
		},
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	return data
}

// buildSync creates the SRS sync message — JSON + newline, no length prefix.
func buildSync(guid, callsign string, freqHz float64) []byte {
	msg := map[string]interface{}{
		"Version": "2.1.0.2",
		"MsgType": 2, // Sync
		"Client": map[string]interface{}{
			"ClientGuid":  guid,
			"Name":        callsign,
			"Seat":        0,
			"Coalition":   2,
			"AllowRecord": true,
			"RadioInfo": map[string]interface{}{
				"radios": []map[string]interface{}{
					{
						"freq":       freqHz,
						"modulation": 0,
						"enc":        false,
						"encKey":     0,
						"secFreq":    0.0,
						"retransmit": false,
					},
				},
				"unit":   callsign,
				"unitId": unitIdForCallsign(callsign),
				"iff": map[string]interface{}{
					"control":   0,
					"expansion": false,
					"mode1":     0,
					"mode2":     -1,
					"mode3":     0,
					"mode4":     false,
					"mic":       -1,
					"status":    0,
				},
				"ambient": map[string]interface{}{
					"abAlt":  0.0,
					"abDist": 0.0,
					"vol":    0.0,
				},
			},
		},
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	return data
}

// transcribeAndHandle wraps Opus frames in OGG, sends to Whisper, then routes to ATC.
func transcribeAndHandle(ctx context.Context, apiKey, ffmpegPath string, frames [][]byte, callsign string, atcCtrl *controller.ATCController) {
	log.Info().Int("frames", len(frames)).Msg("converting and sending to Whisper")
	ogg := wrapOpusInOGG(frames)
	log.Debug().Int("oggBytes", len(ogg)).Msg("OGG container built")

	// Convert OGG/Opus to WAV via FFmpeg
	wav, err := convertToWAV(ffmpegPath, ogg)
	if err != nil {
		log.Error().Err(err).Msg("FFmpeg conversion failed")
		return
	}
	log.Debug().Int("wavBytes", len(wav)).Msg("WAV conversion done")

	// Use callsign-aware prompt to guide Whisper recognition. callsign already
	// ends in "Tower"; appending another "Tower" duplicates it in-prompt and
	// nudges Whisper toward prompt-regurgitation hallucinations.
	prompt := fmt.Sprintf("%s, Raider, Venom, radio check, request taxi, holding short, runway, cleared for takeoff, airborne, seven DME, cleared airspace, inbound, overhead, downwind, base, short final, clear active, runway vacated, going around, mayday", callsign)
	text, err := whisperTranscribeWithPrompt(ctx, apiKey, wav, "audio.wav", prompt)
	if err != nil {
		log.Error().Err(err).Msg("Whisper transcription error")
		return
	}
	if text == "" {
		log.Warn().Msg("Whisper returned empty transcription")
		return
	}
	if isWhisperHallucination(text) {
		log.Warn().Str("text", text).Msg("Whisper hallucination — dropping")
		return
	}
	log.Info().Str("text", text).Msg("recognized")
	req := controller.ParseIntent(text, callsign)
	if req == nil {
		return
	}
	log.Info().Str("callsign", req.Callsign).Int("type", int(req.Type)).Msg("ATC request")
	atcCtrl.HandleRequest(ctx, req)
}

// synthesizeSpeech calls OpenAI TTS API and returns MP3 bytes.
// Uses gpt-4o-mini-tts for more natural, expressive voice output.
// Voice options: alloy, ash, coral, echo, fable, onyx, nova, sage, shimmer
// ── TTS Cache ─────────────────────────────────────────────────────────────────
// Caches MP3 bytes keyed by "voice:text" to avoid repeated API calls for
// common phrases. Pre-warmed at startup with the most frequent ATC responses.

type ttsCache struct {
	mu    sync.RWMutex
	items map[string][]byte
	hits  int64
	misses int64
}

var globalTTSCache = &ttsCache{items: make(map[string][]byte)}

func (c *ttsCache) get(voice string, speed float64, text string) ([]byte, bool) {
	key := fmt.Sprintf("%s:%.2f:%s", voice, speed, text)
	c.mu.RLock()
	v, ok := c.items[key]
	c.mu.RUnlock()
	if ok {
		atomic.AddInt64(&c.hits, 1)
	}
	return v, ok
}

func (c *ttsCache) set(voice string, speed float64, text string, mp3 []byte) {
	key := fmt.Sprintf("%s:%.2f:%s", voice, speed, text)
	c.mu.Lock()
	// Evict random entries if cache exceeds 200 items (~5MB)
	if len(c.items) >= 200 {
		count := 0
		for k := range c.items {
			delete(c.items, k)
			count++
			if count >= 50 { break }
		}
	}
	c.items[key] = mp3
	c.mu.Unlock()
	atomic.AddInt64(&c.misses, 1)
}

func (c *ttsCache) stats() (hits, misses int64) {
	return atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses)
}

// prewarmTTSCache pre-generates MP3s for the most common ATC responses.
// Called once at startup — saves API calls for high-frequency phrases.
func prewarmTTSCache(ctx context.Context, apiKey, voice, runway string) {
	rwy := runway // raw designator e.g. "31L"
	phrases := []string{
		// Pattern calls
		fmt.Sprintf("Number one, report downwind."),
		fmt.Sprintf("Number one, report base."),
		fmt.Sprintf("Number one, report final."),
		fmt.Sprintf("Number two, report downwind."),
		fmt.Sprintf("Number two, report base."),
		fmt.Sprintf("Number two, report final."),
		fmt.Sprintf("Number one."),
		fmt.Sprintf("Number two."),
		// Takeoff/landing
		fmt.Sprintf("Runway %s, cleared for takeoff.", rwy),
		fmt.Sprintf("Runway %s, cleared to land.", rwy),
		fmt.Sprintf("Clear, taxi to parking."),
		fmt.Sprintf("Frequency change approved."),
		// Radio check
		"Loud and clear.",
		"Reading you loud and clear.",
		"Five by five, go ahead.",
		// Fallback / unable-to-understand variants (all 3 from composer.UnableToUnderstand)
		"Say again your request.",
		"Unable to copy, say again.",
		"You were broken, say again.",
		// Departure release bodies (new short form: "proceed to angels {N}, contact tower at seven DME.")
		"Proceed to angels five, contact tower at seven DME.",
		"Proceed to angels six, contact tower at seven DME.",
		"Proceed to angels seven, contact tower at seven DME.",
		"Climb to angels five, contact tower at seven DME.",
		"Climb to angels six, contact tower at seven DME.",
		"Climb to angels seven, contact tower at seven DME.",
		"Angels five, contact tower at seven DME.",
		"Angels six, contact tower at seven DME.",
		"Angels seven, contact tower at seven DME.",
	}
	// NOTE: prewarm cache is keyed by exact text. Today the hot-path TX is
	// "{callsign}, {tower}, {body}." so these bare-body entries only hit if
	// the bot ever transmits them without the prefix (it doesn't). To make
	// prewarm actually help, the composer needs to stitch a cached body MP3
	// with a per-callsign+tower prefix MP3 — flagged for follow-up.
	log.Info().Int("phrases", len(phrases)).Msg("pre-warming TTS cache")
	warmed := 0
	for _, phrase := range phrases {
		if _, ok := globalTTSCache.get(voice, flagTTSSpeed, phrase); ok {
			continue
		}
		mp3, err := synthesizeSpeechAPI(ctx, apiKey, phrase, voice, flagTTSSpeed)
		if err != nil {
			log.Warn().Err(err).Str("phrase", phrase).Msg("TTS prewarm failed")
			continue
		}
		globalTTSCache.set(voice, flagTTSSpeed, phrase, mp3)
		warmed++
		// Small delay to avoid rate limiting
		time.Sleep(200 * time.Millisecond)
	}
	log.Info().Int("warmed", warmed).Msg("TTS cache ready")
}

// currentTowerVoice picks the tower TTS voice based on a fixed time bucket so
// pilots hear an alternating female/male controller without operator action.
// Bucket length is flagVoiceRotateHrs; 0 disables rotation (always returns
// flagTTSVoice). Both voices are pre-warmed at startup so a flip doesn't
// produce a cold-cache TX latency spike.
func currentTowerVoice() string {
	if flagVoiceRotateHrs <= 0 {
		return flagTTSVoice
	}
	bucket := time.Now().Unix() / int64(flagVoiceRotateHrs*3600)
	if bucket%2 == 0 {
		return flagTTSVoice
	}
	return flagTTSVoiceMale
}

// estimateTTSDuration approximates how long OpenAI TTS will play `text`, used to
// size the RX cooldown so the bot doesn't transcribe its own transmission. The
// 14 chars/sec rate is empirical at our previous 0.88 speed; at 0.97 actual
// playback is ~15.5 chars/sec, so 14 keeps the estimate conservative (slightly
// over-cools rather than under-cools). The 5s margin covers ExternalAudio.exe
// startup, the playback tail, the mic-click splice, and — critically — the
// Whisper flush gap that arrives AFTER audio stops playing. Without enough
// margin here the bot transcribes its own TX loopback and re-fires the same
// response (observed 2026-05-12 with airborne / freq-change calls repeating
// every 10–13s).
func estimateTTSDuration(text string) time.Duration {
	d := time.Duration(len(text))*time.Second/14 + 5*time.Second
	if d < 6*time.Second {
		return 6 * time.Second
	}
	return d
}

// synthesizeSpeech checks cache first, falls back to API on miss. speed is the
// TTS playback rate (0.97 for ATIS to keep clarity, ~1.05 for tower/marshal/
// command/deckboss for snappier ATC cadence).
func synthesizeSpeech(ctx context.Context, apiKey, text, voice string, speed float64) ([]byte, error) {
	if mp3, ok := globalTTSCache.get(voice, speed, text); ok {
		log.Debug().Str("text", text).Msg("TTS cache hit")
		return mp3, nil
	}
	mp3, err := synthesizeSpeechAPI(ctx, apiKey, text, voice, speed)
	if err != nil {
		return nil, err
	}
	globalTTSCache.set(voice, speed, text, mp3)
	return mp3, nil
}

// synthesizeSpeechAPI calls OpenAI TTS API directly — bypasses cache.
// Uses gpt-4o-mini-tts for more natural cadence than the legacy tts-1 model.
// Voice list is unchanged: alloy, ash, ballad, coral, echo, fable, onyx, nova,
// sage, shimmer, verse.
// translateToArabic asks OpenAI to translate aviation ATIS text to Arabic.
// Returns the translated string, or an error if the API call fails. Caller is
// responsible for falling back gracefully (e.g. broadcast English only).
func translateToArabic(ctx context.Context, apiKey, text string) (string, error) {
	body := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": "Translate the following aviation ATIS broadcast to modern standard Arabic. Use ICAO phraseology conventions where applicable. Numbers should be spelled out as Arabic words for clear TTS pronunciation. Return ONLY the Arabic translation, no preamble or explanation."},
			{"role": "user", "content": text},
		},
		"temperature": 0.2,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("translate API error %d: %s", resp.StatusCode, string(errBody))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func synthesizeSpeechAPI(ctx context.Context, apiKey, text, voice string, speed float64) ([]byte, error) {
	if speed <= 0 {
		speed = 0.97
	}
	body := map[string]interface{}{
		"model": "gpt-4o-mini-tts",
		"input": text,
		"voice": voice,
		"speed": speed,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/audio/speech",
		bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API error %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// convertToWAV converts OGG/Opus bytes to WAV using FFmpeg.
func convertToWAV(ffmpegPath string, oggData []byte) ([]byte, error) {
	// Write OGG to temp file
	tmpOgg, err := os.CreateTemp("", "atc-*.ogg")
	if err != nil {
		return nil, fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmpOgg.Name())
	if _, err := tmpOgg.Write(oggData); err != nil {
		return nil, fmt.Errorf("write temp: %w", err)
	}
	tmpOgg.Close()

	// Output WAV path
	tmpWav := tmpOgg.Name() + ".wav"
	defer os.Remove(tmpWav)

	// Run FFmpeg: ogg → wav 16kHz mono
	cmd := exec.Command(ffmpegPath,
		"-y",
		"-i", tmpOgg.Name(),
		"-ar", "16000",
		"-ac", "1",
		"-f", "wav",
		tmpWav,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, string(out))
	}

	wav, err := os.ReadFile(tmpWav)
	if err != nil {
		return nil, fmt.Errorf("read wav: %w", err)
	}
	return wav, nil
}

// wrapOpusInOGG wraps raw Opus frames in a valid OGG container.
// Includes correct CRC32 checksums required by decoders.
func wrapOpusInOGG(frames [][]byte) []byte {
	var buf bytes.Buffer
	serialNo := uint32(0x12345678)

	writePage := func(data []byte, seqNo uint32, granule int64, headerType byte) {
		// Build page without checksum first
		var page bytes.Buffer
		page.WriteString("OggS")
		page.WriteByte(0)            // version
		page.WriteByte(headerType)   // header type
		binary.Write(&page, binary.LittleEndian, uint64(granule))
		binary.Write(&page, binary.LittleEndian, serialNo)
		binary.Write(&page, binary.LittleEndian, seqNo)
		binary.Write(&page, binary.LittleEndian, uint32(0)) // CRC placeholder
		page.WriteByte(1)            // 1 segment
		page.WriteByte(byte(len(data))) // segment size
		page.Write(data)

		// Calculate CRC32 over entire page
		pageBytes := page.Bytes()
		crc := oggCRC32(pageBytes)
		// Write CRC at offset 22
		binary.LittleEndian.PutUint32(pageBytes[22:26], crc)
		buf.Write(pageBytes)
	}

	seqNo := uint32(0)

	// OpusHead
	var head bytes.Buffer
	head.WriteString("OpusHead")
	head.WriteByte(1)   // version
	head.WriteByte(1)   // channels
	binary.Write(&head, binary.LittleEndian, uint16(312))   // pre-skip
	binary.Write(&head, binary.LittleEndian, uint32(16000)) // input sample rate
	binary.Write(&head, binary.LittleEndian, uint16(0))     // output gain
	head.WriteByte(0)   // channel map family
	writePage(head.Bytes(), seqNo, 0, 0x02) // BOS
	seqNo++

	// OpusTags
	var tags bytes.Buffer
	tags.WriteString("OpusTags")
	binary.Write(&tags, binary.LittleEndian, uint32(7))
	tags.WriteString("vsfg7atc")
	binary.Write(&tags, binary.LittleEndian, uint32(0))
	writePage(tags.Bytes(), seqNo, 0, 0x00)
	seqNo++

	// Audio frames
	granule := int64(0)
	for i, frame := range frames {
		granule += 1920 // 40ms at 48kHz
		head := byte(0x00)
		if i == len(frames)-1 {
			head = 0x04 // EOS
		}
		writePage(frame, seqNo, granule, head)
		seqNo++
	}

	return buf.Bytes()
}

// oggCRC32 calculates the OGG CRC32 checksum.
// OGG uses a specific polynomial: 0x04c11db7
func oggCRC32(data []byte) uint32 {
	var crcTable [256]uint32
	for i := range crcTable {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if r&0x80000000 != 0 {
				r = (r << 1) ^ 0x04c11db7
			} else {
				r <<= 1
			}
		}
		crcTable[i] = r
	}
	crc := uint32(0)
	for _, b := range data {
		crc = (crc << 8) ^ crcTable[((crc>>24)^uint32(b))&0xff]
	}
	return crc
}

// whisperTranscribe sends audio to OpenAI Whisper API for STT.
func whisperTranscribeWithPrompt(ctx context.Context, apiKey string, audio []byte, filename, prompt string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	w.WriteField("model", "gpt-4o-mini-transcribe")
	w.WriteField("language", "en")
	w.WriteField("prompt", prompt)
	w.Close()
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if errObj, ok := result["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return "", fmt.Errorf("Whisper API error: %s", msg)
		}
	}
	text, _ := result["text"].(string)
	return strings.TrimSpace(text), nil
}

func whisperTranscribe(ctx context.Context, apiKey string, audio []byte, filename string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	w.WriteField("model", "gpt-4o-mini-transcribe")
	w.WriteField("language", "en")
	w.WriteField("prompt", "Marshal, Raider, Venom, marking mom, angels, state, established, commencing, pushing, checking in, see you at ten, signal Charlie, BRC, altimeter, radio check, five by five")
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	log.Debug().
		Int("statusCode", resp.StatusCode).
		Str("body", string(body)).
		Msg("Whisper API response")

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if errObj, ok := result["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return "", fmt.Errorf("Whisper API error: %s", msg)
		}
	}
	text, _ := result["text"].(string)
	return text, nil
}


// applyRadioEffect runs the TTS MP3 through ffmpeg to add the tinny, staticky
// feel of an AM comms radio. Clean voice is bandpass-limited to ~radio
// bandwidth, lightly compressed, then mixed with low-level pink noise so the
// carrier hiss is audible under the speech. Returns the original bytes on any
// ffmpeg failure so broadcasts never drop to silence.
func applyRadioEffect(mp3 []byte, ffmpegPath, intensity string) []byte {
	var noiseAmp float64
	var hpf, lpf int
	switch strings.ToLower(intensity) {
	case "light":
		noiseAmp, hpf, lpf = 0.020, 350, 3200
	case "heavy":
		noiseAmp, hpf, lpf = 0.080, 500, 2700
	case "extreme":
		noiseAmp, hpf, lpf = 0.130, 600, 2400
	default: // medium
		noiseAmp, hpf, lpf = 0.040, 400, 3000
	}

	tmpIn, err := os.CreateTemp("", "atc-radio-in-*.mp3")
	if err != nil {
		return mp3
	}
	defer os.Remove(tmpIn.Name())
	if _, err := tmpIn.Write(mp3); err != nil {
		tmpIn.Close()
		return mp3
	}
	tmpIn.Close()

	tmpOut := tmpIn.Name() + ".out.mp3"
	defer os.Remove(tmpOut)

	filter := fmt.Sprintf(
		"[0:a]highpass=f=%d,lowpass=f=%d,"+
			"acompressor=threshold=-24dB:ratio=8:attack=5:release=60,"+
			"volume=1.15[voice];"+
			"anoisesrc=color=pink:amplitude=%.3f:duration=0[noise];"+
			"[voice][noise]amix=inputs=2:duration=first:dropout_transition=0[out]",
		hpf, lpf, noiseAmp,
	)

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-i", tmpIn.Name(),
		"-filter_complex", filter,
		"-map", "[out]",
		"-c:a", "libmp3lame",
		"-q:a", "4",
		tmpOut,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warn().Err(err).Str("output", string(out)).Msg("radio effect ffmpeg failed — using clean audio")
		return mp3
	}
	processed, err := os.ReadFile(tmpOut)
	if err != nil || len(processed) == 0 {
		return mp3
	}
	return processed
}

// addMicClicks splices a synthetic PTT key-up click (~70ms white-noise
// transient bandpassed to 1500–4500 Hz) onto the start of the voice and a
// squelch-tail noise burst (~180ms pink noise with fade-out) onto the end.
// Both clicks are generated entirely by ffmpeg's anoisesrc so no audio assets
// need bundling. Returns the original bytes on any ffmpeg failure so live
// broadcasts never drop to silence.
func addMicClicks(mp3 []byte, ffmpegPath string) []byte {
	tmpIn, err := os.CreateTemp("", "atc-clicks-in-*.mp3")
	if err != nil {
		return mp3
	}
	defer os.Remove(tmpIn.Name())
	if _, err := tmpIn.Write(mp3); err != nil {
		tmpIn.Close()
		return mp3
	}
	tmpIn.Close()

	tmpOut := tmpIn.Name() + ".out.mp3"
	defer os.Remove(tmpOut)

	// Normalize voice to 44100/mono so concat can match the synthetic clicks.
	filter :=
		"[0:a]aresample=44100,aformat=channel_layouts=mono[voice];" +
			"anoisesrc=color=white:amplitude=0.55:duration=0.07:sample_rate=44100," +
			"highpass=f=1500,lowpass=f=4500," +
			"afade=t=in:st=0:d=0.005,afade=t=out:st=0.05:d=0.02[click_up];" +
			"anoisesrc=color=pink:amplitude=0.4:duration=0.18:sample_rate=44100," +
			"highpass=f=300,lowpass=f=4000," +
			"afade=t=out:st=0.02:d=0.16[click_dn];" +
			"[click_up][voice][click_dn]concat=n=3:v=0:a=1[out]"

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-i", tmpIn.Name(),
		"-filter_complex", filter,
		"-map", "[out]",
		"-c:a", "libmp3lame",
		"-q:a", "4",
		tmpOut,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warn().Err(err).Str("output", string(out)).Msg("mic-click splice ffmpeg failed — using clean audio")
		return mp3
	}
	processed, err := os.ReadFile(tmpOut)
	if err != nil || len(processed) == 0 {
		return mp3
	}
	return processed
}

// concatMP3WithSilence re-encodes two MP3 byte slices into a single MP3 with
// `silenceSec` seconds of silence between them. Used by ATIS to merge EN+AR
// into one clean stream — byte-level MP3 concat is fragile and can cause some
// decoders (DCS-SR-ExternalAudio) to play the two halves simultaneously.
func concatMP3WithSilence(en, ar []byte, ffmpegPath string, silenceSec float64) ([]byte, error) {
	tmpEn, err := os.CreateTemp("", "atc-atis-en-*.mp3")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpEn.Name())
	if _, err := tmpEn.Write(en); err != nil {
		tmpEn.Close()
		return nil, err
	}
	tmpEn.Close()

	tmpAr, err := os.CreateTemp("", "atc-atis-ar-*.mp3")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpAr.Name())
	if _, err := tmpAr.Write(ar); err != nil {
		tmpAr.Close()
		return nil, err
	}
	tmpAr.Close()

	tmpOut := tmpEn.Name() + ".out.mp3"
	defer os.Remove(tmpOut)

	filter := fmt.Sprintf(
		"[0:a]aresample=44100,aformat=channel_layouts=mono[en];"+
			"[1:a]aresample=44100,aformat=channel_layouts=mono[ar];"+
			"anullsrc=r=44100:cl=mono,atrim=duration=%.3f[sil];"+
			"[en][sil][ar]concat=n=3:v=0:a=1[out]",
		silenceSec,
	)

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-i", tmpEn.Name(),
		"-i", tmpAr.Name(),
		"-filter_complex", filter,
		"-map", "[out]",
		"-c:a", "libmp3lame",
		"-q:a", "4",
		tmpOut,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %v: %s", err, string(out))
	}
	data, err := os.ReadFile(tmpOut)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("ffmpeg produced empty output")
	}
	return data, nil
}

// transmitExternalAudioFile injects a pre-generated MP3 file via ExternalAudio.
// ctx bounds the subprocess so a hung ExternalAudio.exe can't stall the TX queue.
func transmitExternalAudioFile(ctx context.Context, mp3 []byte, freqMHz float64, callsign, srsHost, srsPort, exePath string) {
	if flagRadioEffect {
		// Per-role intensity:
		// - Tower callsigns end in "-ATC" (e.g. OMAM-ATC) → heavier static
		//   for that punchy ATC sound.
		// - ATIS station callsigns end with " ATIS" (e.g. "Liwa ATIS",
		//   "Khasab ATIS") → kept clean since it's a recorded loop.
		// - Everything else (OMDM-MSH, OMDM-DKB, vSFG-7-Command) gets the
		//   general operational intensity.
		intensity := flagRadioIntensity
		if strings.HasSuffix(callsign, "-ATC") {
			intensity = flagTowerRadioIntensity
		} else if strings.HasSuffix(callsign, " ATIS") {
			intensity = flagATISRadioIntensity
		}
		mp3 = applyRadioEffect(mp3, flagFFmpeg, intensity)
		mp3 = addMicClicks(mp3, flagFFmpeg)
	}
	// Write MP3 to temp file
	tmp, err := os.CreateTemp("", "atc-tts-*.mp3")
	if err != nil {
		log.Error().Err(err).Msg("TTS temp file error")
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(mp3); err != nil {
		log.Error().Err(err).Msg("TTS temp write error")
		tmp.Close()
		return
	}
	tmp.Close()

	safeName := strings.ReplaceAll(callsign, " ", "-")
	args := []string{
		"--file", tmpPath,
		"--freqs", fmt.Sprintf("%.3f", freqMHz),
		"--modulations", "AM",
		"-c", "2",
		"-n", safeName,
		"-p", srsPort,
		"-v", "1",
	}
	log.Debug().Strs("args", args).Msg("ExternalAudio file TX")
	out, err := exec.CommandContext(ctx, exePath, args...).CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			log.Warn().Err(ctx.Err()).Msg("ExternalAudio file TX timed out — killed")
		} else {
			log.Error().Err(err).Str("output", string(out)).Msg("ExternalAudio file error")
		}
	} else {
		log.Debug().Msg("ExternalAudio file TX ok")
	}
}

// transmitExternalAudio calls DCS-SR-ExternalAudio.exe to transmit text on SRS.
func transmitExternalAudio(text string, freqMHz float64, callsign, srsHost, srsPort, exePath string) {
	// Flag names from ExternalAudio --help:
	// --freqs (MHz), --modulations, -c coalition, -n name, -p port, -g gender
	// Note: no --server flag — it connects to localhost by default.
	// For remote SRS, ExternalAudio must run on the SRS server machine.
	safeName := strings.ReplaceAll(callsign, " ", "-")
	args := []string{
		"--text", text,
		"--freqs", fmt.Sprintf("%.3f", freqMHz),
		"--modulations", "AM",
		"-c", "2",
		"-n", safeName,
		"-p", srsPort,
		"-g", "female",
		"-v", "1",
	}
	log.Debug().Strs("args", args).Msg("ExternalAudio call")
	out, err := exec.Command(exePath, args...).CombinedOutput()
	if err != nil {
		log.Error().Err(err).Str("output", string(out)).Msg("ExternalAudio error")
	} else {
		log.Debug().Str("output", string(out)).Msg("ExternalAudio ok")
	}
}

var _ = orb.Point{}

// ── Weather UDP listener ──────────────────────────────────────────────────────

type weatherPacket struct {
	ICAO    string
	WindDir float64
	WindSpd float64
	QNH     float64
}


// ── Tacview telemetry ─────────────────────────────────────────────────────────
// Connects to Tacview real-time telemetry (ACMI over TCP) and feeds aircraft
// positions and speeds into the ATC state machine.
//
// Tacview ACMI format (simplified):
//   #timestamp
//   objectID,T=lon|lat|alt,Name=callsign,IAS=speed,...
//
// We parse position, altitude and IAS for each aircraft and call UpdatePosition.

func tacviewLoop(ctx context.Context, addr string, atcCtrl *controller.ATCController, connected *int32, lastData *int64) {
	// Object registries persist across reconnects — Tacview resends full state on reconnect
	objects := make(map[string]string)
	objectTypes := make(map[string]string)
	type objData struct {
		lon, lat, altFt, speedKts float64
		headingDeg                float64
		vertSpeedFpm              float64
		prevAltFt                 float64
		prevTime                  time.Time
		hasPos                    bool
	}
	positions := make(map[string]*objData)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Debug().Str("addr", addr).Msg("connecting to Tacview telemetry")
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			log.Debug().Msg("Tacview not available — retrying in 30s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}

		log.Info().Str("addr", addr).Msg("Tacview telemetry connected")
		atomic.StoreInt32(connected, 1)
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		// Tacview real-time telemetry handshake
		// Send XtraLib handshake — required or Tacview drops the connection
		conn.Write([]byte("XtraLib.Stream.0\nTacview.RealTimeTelemetry.0\nvSFG7-ATC\n0\x00"))

		// Object registries declared outside loop — persist across reconnects.
		// Reference lat/lon: ACMI reports T= positions as DELTAS from these
		// globals (set once on the special object id "0"). Without them, every
		// position is interpreted as a tiny absolute coord near (0,0) — making
		// haversine to any real airfield return ~3000 nm. DCS exports both for
		// the mission map; we apply them to T= coords[0..1].
		var refLat, refLon float64
		var refSet bool

		// ReferenceTime is the mission start (UTC) reported on object "0".
		// Lines starting with "#" carry the seconds-since-ReferenceTime offset.
		// Together they let us compute in-sim wall-clock time, used by the
		// dashboard to decide day vs night without depending on real-world UTC.
		var refTime time.Time

		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 65536), 65536)

		for scanner.Scan() {
			conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
			select {
			case <-ctx.Done():
				conn.Close()
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			// "#<seconds>" lines are Tacview time markers. Combine with the
			// ReferenceTime captured on object "0" to get mission UTC, and
			// push it to the controller so the dashboard can render day/night.
			if strings.HasPrefix(line, "#") {
				if !refTime.IsZero() {
					var offset float64
					if _, err := fmt.Sscanf(line[1:], "%f", &offset); err == nil {
						atcCtrl.SetMissionTime(refTime.Add(time.Duration(offset * float64(time.Second))))
					}
				}
				continue
			}

			// Parse object update: ID,key=val,key=val,...
			parts := strings.SplitN(line, ",", 2)
			if len(parts) < 2 {
				continue
			}
			id := parts[0]
			props := parts[1]

			// Object id "0" is the global / world object — carries the
			// ReferenceLatitude / ReferenceLongitude that subsequent T=
			// position deltas are measured from, plus ReferenceTime (mission
			// start UTC). Capture and skip.
			if id == "0" {
				if v := extractACMIProp(props, "ReferenceLatitude"); v != "" {
					if _, err := fmt.Sscanf(v, "%f", &refLat); err == nil {
						refSet = true
					}
				}
				if v := extractACMIProp(props, "ReferenceLongitude"); v != "" {
					fmt.Sscanf(v, "%f", &refLon)
					refSet = true
				}
				if v := extractACMIProp(props, "ReferenceTime"); v != "" {
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						refTime = t
						// Seed mission time at offset 0 so the dashboard has
						// something to render before the first #tick arrives.
						atcCtrl.SetMissionTime(t)
					}
				}
				continue
			}

			// Priority for the callsign key:
			//   1. Pilot — for AI units this is the modex/unit callsign
			//      (e.g. "Pontiac 1-1 Rescue"); for humans it may be the
			//      player handle. Confirmed via first-seen logs 2026-05-14.
			//   2. Group — usually the formation name (e.g. "Carrier strike
			//      group-10"), only useful as a fallback.
			//   3. Name  — aircraft type (e.g. "F-14B") as last resort.
			// Human players: DCS reports Pilot as "<modex> | <player_name>"
			// (e.g. "Venom 020 | BARNEY"). The pilot says only the modex on
			// the radio, so we strip everything from " | " onward. AI units
			// don't use the " | " separator, so this is a no-op for them.
			_, wasKnown := objects[id]
			switch {
			case extractACMIProp(props, "Pilot") != "":
				p := extractACMIProp(props, "Pilot")
				// Strip on the bare "|" — DCS uses inconsistent spacing
				// around it ("Venom 020 | BARNEY" vs "Raider 032 |Jedi").
				// Take everything before the first pipe and trim whitespace.
				if i := strings.Index(p, "|"); i >= 0 {
					p = strings.TrimSpace(p[:i])
				}
				objects[id] = p
				if positions[id] == nil {
					positions[id] = &objData{}
				}
			case extractACMIProp(props, "Group") != "":
				objects[id] = extractACMIProp(props, "Group")
				if positions[id] == nil {
					positions[id] = &objData{}
				}
			case extractACMIProp(props, "Name") != "":
				if !wasKnown {
					objects[id] = extractACMIProp(props, "Name")
				}
				if positions[id] == nil {
					positions[id] = &objData{}
				}
			}
			// First-seen diagnostic: dump the full ACMI prop line so we can
			// confirm which fields DCS is actually exporting (modex vs player).
			if !wasKnown && objects[id] != "" {
				// Air-only: ATC only ever does callsign lookups for aircraft.
				// Without this filter the log is drowned by every ground
				// vehicle, ship, and static in the mission (200+ entries).
				if t := extractACMIProp(props, "Type"); strings.HasPrefix(t, "Air") {
					log.Info().
						Str("id", id).
						Str("chosenKey", objects[id]).
						Str("group", extractACMIProp(props, "Group")).
						Str("pilot", extractACMIProp(props, "Pilot")).
						Str("name", extractACMIProp(props, "Name")).
						Str("type", t).
						Msg("Tacview contact first-seen")
				}
			}
			// Extract Type — filter to air objects only
			if objType := extractACMIProp(props, "Type"); objType != "" {
				objectTypes[id] = objType
			}

			// Extract T= (transform: lon|lat|alt[|roll|pitch|heading]).
			// lon and lat are DELTAS in degrees from refLon/refLat. Add the
			// reference to recover absolute coordinates.
			//
			// ACMI delta updates: after the initial full frame for an object,
			// subsequent T= records may contain ONLY the changed fields (e.g.
			// `T=||5400|||92` for an alt+heading update with lon/lat empty).
			// An empty subfield must NOT overwrite the previously stored value
			// — earlier code parsed "" as 0 and zeroed altitude/position
			// (observed: pilot at 13,560 ft reading as angels 0).
			if t := extractACMIProp(props, "T"); t != "" {
				coords := strings.Split(t, "|")
				if len(coords) >= 3 {
					if positions[id] == nil {
						positions[id] = &objData{}
					}
					if coords[0] != "" {
						var dLon float64
						if _, err := fmt.Sscanf(coords[0], "%f", &dLon); err == nil {
							if refSet {
								positions[id].lon = refLon + dLon
							} else {
								positions[id].lon = dLon
							}
						}
					}
					if coords[1] != "" {
						var dLat float64
						if _, err := fmt.Sscanf(coords[1], "%f", &dLat); err == nil {
							if refSet {
								positions[id].lat = refLat + dLat
							} else {
								positions[id].lat = dLat
							}
						}
					}
					if coords[2] != "" {
						var altM float64
						if _, err := fmt.Sscanf(coords[2], "%f", &altM); err == nil {
							newAlt := altM * 3.28084
							// Calculate vertical speed against previous alt
							now := time.Now()
							if positions[id].hasPos && !positions[id].prevTime.IsZero() {
								dt := now.Sub(positions[id].prevTime).Minutes()
								if dt > 0 {
									positions[id].vertSpeedFpm = (newAlt - positions[id].prevAltFt) / dt
								}
							}
							positions[id].prevAltFt = newAlt
							positions[id].prevTime = now
							positions[id].altFt = newAlt
						}
					}
					positions[id].hasPos = true
					// Heading is 6th field (index 5) in full T= record
					if len(coords) >= 6 && coords[5] != "" {
						fmt.Sscanf(coords[5], "%f", &positions[id].headingDeg)
					}
				}
			}

			// Extract IAS (indicated airspeed in m/s → knots)
			if ias := extractACMIProp(props, "IAS"); ias != "" {
				if positions[id] == nil {
					positions[id] = &objData{}
				}
				var iasMs float64
				fmt.Sscanf(ias, "%f", &iasMs)
				positions[id].speedKts = iasMs * 1.94384
			}

			// Feed into ATC state — air objects only (filter ground/sea/static)
			if callsign, ok := objects[id]; ok {
				objType := objectTypes[id]
				isAir := objType == "" || strings.Contains(objType, "Air")
				if isAir {
					if pos, ok := positions[id]; ok && pos.hasPos {
						atcCtrl.UpdateAnyPosition(callsign, pos.lon, pos.lat, pos.altFt, pos.speedKts, pos.headingDeg, pos.vertSpeedFpm)
						atomic.StoreInt64(lastData, time.Now().UnixNano())
					}
				}
			}
		}

		conn.Close()
		atomic.StoreInt32(connected, 0)
		log.Debug().Msg("Tacview disconnected — reconnecting in 10s")
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}