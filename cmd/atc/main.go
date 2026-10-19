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
	"math/rand"
	"strconv"
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
	"regexp"
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
	"github.com/vsfg7/atc/pkg/miz"
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
	flagMap           string
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
	// Handoff destinations — see the --handoff-* flag block in init.
	flagHandoffCommandFreq   float64
	flagHandoffCommandName   string
	flagHandoffCommandPreset string
	flagHandoffMarshalFreq   float64
	flagHandoffMarshalName   string
	// Per-role TTS delivery instructions — see the --voice-style-* flag block.
	flagVoiceStyleTower    string
	flagVoiceStyleMarshal  string
	flagVoiceStyleDeckboss string
	flagVoiceStyleCommand  string
	flagVoiceStyleATIS     string
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
	flagMizPath         string
	flagMizDir          string
	flagMizWatchFile    string
	flagFootholdSave    string
	flagFootholdTasking bool
	flagTacviewAddr     string
	flagATISFreq        string
	flagATISStations    string
	flagATISBroadcast   bool
	flagCommandFreq     string
	flagCommandName     string
	flagCommandVoice    string
	flagJTACFreq        float64
	flagJTACName        string
	flagJTACSpoken      string
	flagJTACVoice       string
	flagVoiceStyleJTAC  string
	flagJTACRadioIntensity string
	flagJTACSector         string
	flagJTAC2Freq          float64
	flagJTAC2Name          string
	flagJTAC2Spoken        string
	flagJTAC2Voice         string
	flagJTAC2Sector        string
	flagVoiceStyleJTAC2    string
	flagElevenJTAC2Voice   string
	flagOverlordName       string
	flagOverlordVoice      string
	flagElevenOverlord     string
	flagElevenLabs         bool
	flagElevenLabsKey      string
	flagElevenLabsModel    string
	flagElevenStopAt       int
	flagElevenCommandVoice string
	flagElevenJTACVoice    string
	flagElevenMarshalVoice string
	flagElevenDepartVoice  string
	flagElevenGroundVoice  string
	flagElevenTowerAltVoice string
	flagElevenTowerVoice   string
	flagTowerShift         time.Duration
	flagElevenDeckbossVoice string
	flagDeckbossVoice   string
	flagMarshalVoice    string
	flagRadioEffect          bool
	flagRadioIntensity       string
	flagOutputGainDB         float64
	flagTowerRadioIntensity  string
	flagCarrierRadioIntensity string
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
	flagRunwaySlot          time.Duration
	flagRunwayCalmKts       float64
	flagPositionCheck       bool
	flagDeckPositionCheck   bool
	flagUnknownTraffic      bool
	flagRunwayVacateChase   bool
	flagDepartureCall       bool
	flagTrafficAlerts       bool
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
	f.StringVar(&flagAirfield, "airfield", "OMDM", "ICAO: OMDM/OMAM/OMAL (PG) · UGSB/UG5X/UGKS/UGKO/UGTB (Caucasus) · ETAR/ETAD/EDFH/EDDF/EDDK/EDDL/EDDV/EDDH (Germany) · ORAA/ORSH/ORBR/ORBI/ORBD/ORBB/ORER/ORKK/ORSU (Iraq) · ENBO/ENDU/ENNA/EFRO (Kola)")
	f.StringVar(&flagMap, "map", "pg", "Theatre for ATIS station set + Command handoff scan: pg | caucasus | germany | iraq | syria | kola")
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
	f.BoolVar(&flagFootholdTasking, "foothold-tasking", true,
		"Command on --map syria tasks flights against enemy zones read from the Foothold campaign save file")
	f.StringVar(&flagFootholdSave, "foothold-save", "",
		"Foothold save file (default: the path in Missions/Saves/foothold.status, else the newest Saves/foothold*.lua, under the mission folders SkyEye knows)")
	f.StringVar(&flagMizPath, "miz-path", "", "Path to a specific .miz to read weather from at boot (overrides --miz-dir)")
	f.StringVar(&flagMizWatchFile, "miz-watch-file", `C:\SkyeyeATC\current_mission.txt`,
		"File the DCS server hook (tools/dcs-hooks/vsfg7-current-mission.lua) writes the loaded mission's path to; polled every 30s and the new mission's weather applied live. Empty disables. Missing file = keep boot weather.")
	f.StringVar(&flagMizDir,  "miz-dir",  `C:\Users\Administrator\Saved Games\DCS.dcs_serverrelease\Missions`, "Dir scanned for the newest .miz when --miz-path is empty. Empty disables .miz auto-load.")
	f.BoolVar(&flagCommandOnly, "command-only", false,
		"Run as command channel only — no tower, no ATIS, no marshal")
	f.StringVar(&flagCommandVoice, "command-voice", "onyx",
		"OpenAI TTS voice for command channel: onyx recommended for male controller")
	f.Float64Var(&flagJTACFreq, "jtac-freq", 0,
		"With --command-only: run the JTAC (jtac.go) on this frequency in MHz, e.g. 61.0 (30-88 MHz is FM). Command then hands tasked flights to it instead of clearing them hot. 0=disabled")
	f.StringVar(&flagJTACName, "jtac-name", "Hitman-1-1", "JTAC SRS client name (no spaces)")
	f.StringVar(&flagJTACSpoken, "jtac-spoken", "Hitman one one", "JTAC callsign as spoken on the radio")
	f.StringVar(&flagJTACVoice, "jtac-voice", "ash", "OpenAI TTS voice for the JTAC")
	f.StringVar(&flagVoiceStyleJTAC, "voice-style-jtac", styleJTAC, "TTS delivery instructions for the JTAC (empty=none)")
	// Hitman 2-1 (operator 2026-10-16: "Lets do Hitman on 62.00"), split by
	// sector: with both JTACs up, each runs one range_north sector.
	f.StringVar(&flagJTACSector, "jtac-sector", "Kuban Valley", "With --jtac2-freq: the range sector Hitman 1-1 runs (targets 1-3 on Caucasus)")
	f.Float64Var(&flagJTAC2Freq, "jtac2-freq", 0,
		"With --jtac-freq: run a second JTAC on this frequency in MHz, e.g. 62.0; each JTAC then runs one sector and Command picks the one with a free target. 0=disabled")
	f.StringVar(&flagJTAC2Name, "jtac2-name", "Hitman-2-1", "Second JTAC SRS client name (no spaces)")
	f.StringVar(&flagJTAC2Spoken, "jtac2-spoken", "Hitman two one", "Second JTAC callsign as spoken on the radio")
	f.StringVar(&flagJTAC2Voice, "jtac2-voice", "echo", "OpenAI TTS voice for the second JTAC (the ElevenLabs fallback)")
	f.StringVar(&flagJTAC2Sector, "jtac2-sector", "Cherkessk", "The range sector the second JTAC runs (targets 4-5 on Caucasus)")
	f.StringVar(&flagVoiceStyleJTAC2, "voice-style-jtac2", styleJTAC2, "TTS delivery instructions for the second JTAC (empty=none)")
	f.StringVar(&flagElevenJTAC2Voice, "elevenlabs-jtac2-voice", elHitman21, "ElevenLabs voice ID for the second JTAC (default round 18 \"George\", library)")
	// Overlord Actual (operator 2026-10-16): answers "permission to proceed to
	// <area>" on Command's frequency ~10 s after Command's stand by.
	f.StringVar(&flagOverlordName, "overlord-name", "Overlord-Actual", "SRS client name Overlord Actual transmits under on the Command frequency (\"\" = no Overlord: permission requests get Command's brief)")
	f.StringVar(&flagOverlordVoice, "overlord-voice", "ash", "OpenAI TTS voice for Overlord Actual (the ElevenLabs fallback)")
	f.StringVar(&flagElevenOverlord, "elevenlabs-overlord-voice", elBrad, "ElevenLabs voice ID for Overlord Actual (default round 10 \"Brad\", library)")
	f.BoolVar(&flagElevenLabs, "elevenlabs", true,
		"Speak Command, Hitman, the towers, Marshal and Deckboss with ElevenLabs when a key is set (ELEVENLABS_API_KEY or --elevenlabs-key); OpenAI is the fallback. ATIS stays on OpenAI.")
	f.StringVar(&flagElevenLabsKey, "elevenlabs-key", "", "ElevenLabs API key (default: ELEVENLABS_API_KEY)")
	// Flash for every role since 2026-10-15: half the credits a character
	// (measured: Lisa's 194-char tower line 85 -> 43), and the operator could
	// not hear the difference on the round 15 A/B ("Do flash I cannot tell the
	// difference"). eleven_multilingual_v2 is the richer, 2x-cost model.
	f.IntVar(&flagElevenStopAt, "elevenlabs-stop-at", 127000, "ElevenLabs credits used this billing period (the account's count) at which every role switches to OpenAI until the period resets; checked every 10 min (operator 2026-10-16: \"flip to open AI once we hit 127k\"; 0 = never)")
	f.StringVar(&flagElevenLabsModel, "elevenlabs-model", elFlash, "ElevenLabs model (default Flash, half the credits of eleven_multilingual_v2)")
	f.StringVar(&flagElevenCommandVoice, "elevenlabs-command-voice", elDeparture11, "ElevenLabs voice ID for Command (default round 11 voice 4 \"Jodi\", library - the Departure voice too)")
	f.StringVar(&flagElevenJTACVoice, "elevenlabs-jtac-voice", elScotty, "ElevenLabs voice ID for Hitman (default Scotty, library)")
	f.StringVar(&flagElevenTowerVoice, "elevenlabs-tower-voice", "",
		"ElevenLabs voice ID for this tower, over the accent default (Lisa / Alice / George); Senaki uses round 13 senaki_6 for Tower, Ground and Traffic")
	f.StringVar(&flagElevenTowerAltVoice, "elevenlabs-tower-alt-voice", "",
		"ElevenLabs voice ID of this tower's alternate controller; the two swap every --tower-shift once the frequency has been quiet 5 min (\"\" = one controller)")
	f.DurationVar(&flagTowerShift, "tower-shift", 2*time.Hour,
		"Shift length for --elevenlabs-tower-alt-voice, counted from the Unix epoch so every rig changes together")
	f.StringVar(&flagElevenDepartVoice, "elevenlabs-departure-voice", elDeparture11,
		"ElevenLabs voice ID for the tower's Departure position - the 7-mile handoff call (default round 11 voice 4, library)")
	f.StringVar(&flagElevenGroundVoice, "elevenlabs-ground-voice", "",
		"ElevenLabs voice ID for the tower's Ground position (parking spots, taxi routes, at Senaki); \"\" = the tower's own voice (operator 2026-10-16: Ground, Tower and Traffic one voice)")
	f.StringVar(&flagElevenMarshalVoice, "elevenlabs-marshal-voice", elRolando, "ElevenLabs voice ID for Marshal (default Rolando, library)")
	f.StringVar(&flagElevenDeckbossVoice, "elevenlabs-deckboss-voice", elDeckboss11,
		"ElevenLabs voice ID for Deckboss (default round 11 voice 5, library; every theatre)")
	f.StringVar(&flagJTACRadioIntensity, "jtac-radio-intensity", "field",
		"Radio effect for the JTAC: light, medium, heavy, extreme, or field (a manpack FM set on the ground: narrow band, heavy hiss)")
	f.BoolVar(&flagATISBroadcast, "atis-broadcast", false,
		"Broadcast ATIS on airfield ATIS frequency (replaces MOOSE ATIS)")
	f.StringVar(&flagATISFreq, "atis-freq", "0",
		"ATIS frequency in MHz to monitor for weather (0=auto from airfield)")
	f.StringVar(&flagATISStations, "atis-stations", "",
		"Comma-separated ICAOs to limit --atis-only to (empty=every station in the theatre set)")
	f.StringVar(&flagTacviewAddr, "tacview-addr", "192.168.1.221:42676",
		"Tacview real-time telemetry address:port")
	f.IntVar(&flagDashboardPort, "dashboard-port", 0,
		"HTTP dashboard port (0=disabled). OMDM=6001, OMAM=6002, OMAL=6003")
	f.StringVar(&flagDeckbossFreq, "deckboss-freq", "0",
		"Deckboss frequency MHz (OMDM only, e.g. 128.6 — DCS carrier UHF)")
	f.StringVar(&flagDeckbossVoice, "deckboss-voice", "shimmer",
		"OpenAI TTS voice for Deckboss: shimmer (default, female, bright and cutting), nova/coral/sage (other female), ash/echo/onyx (male)")
	f.StringVar(&flagMarshalVoice, "marshal-voice", "ballad",
		"OpenAI TTS voice for Marshal: ballad (default, naval-controller feel), verse, sage, onyx, etc.")
	f.StringVar(&flagMarshalFreq, "marshal-freq", "0",
		"Marshal frequency MHz (OMDM only, e.g. 306.3)")

	// Handoff directory — where each role sends pilots when it's done with
	// them. A role process only knows its OWN frequency from the flags above
	// (see start_*.bat), so the destinations it hands off TO are configured
	// separately. Defaults match the vSFG-7 rig, so the handoffs work with no
	// extra flags on the existing .bat files.
	// Departing aircraft follow a three-leg chain (2026-08-16): Deckboss acks
	// airborne and pushes to Marshal on -marshal-freq; Marshal releases them
	// from the control zone and pushes to Command on -command-freq. Deckboss
	// went via Command directly until 2026-08-06, then handed off to nobody at
	// all until the Marshal leg was added.
	f.Float64Var(&flagHandoffCommandFreq, "handoff-command-freq", 282.0,
		"Frequency MHz that Marshal sends departing pilots to (0=omit the frequency from that call)")
	f.StringVar(&flagHandoffCommandName, "handoff-command-name", "vSFG-7-Command",
		"Controller name spoken in the departure handoff")
	f.StringVar(&flagHandoffCommandPreset, "handoff-command-preset", "channel four",
		"Spoken preset for the departure handoff (empty to omit)")
	f.Float64Var(&flagHandoffMarshalFreq, "handoff-marshal-freq", 306.3,
		"Frequency MHz for Marshal — Command sends carrier-inbound pilots here, and Deckboss sends departing pilots here (0=disable both Marshal handoffs)")
	f.StringVar(&flagHandoffMarshalName, "handoff-marshal-name", "Marshal",
		"Controller name spoken in the carrier-recovery handoff")

	// Delivery style per role — the gpt-4o-mini-tts `instructions` field.
	// This is what makes a role sound like that role rather than like an
	// audiobook; empty disables it for that role.
	f.StringVar(&flagVoiceStyleTower, "voice-style-tower", styleTower,
		"TTS delivery instructions for tower (empty=none)")
	f.StringVar(&flagVoiceStyleMarshal, "voice-style-marshal", styleMarshal,
		"TTS delivery instructions for Marshal (empty=none)")
	f.StringVar(&flagVoiceStyleDeckboss, "voice-style-deckboss", styleDeckboss,
		"TTS delivery instructions for Deckboss (empty=none)")
	f.StringVar(&flagVoiceStyleCommand, "voice-style-command", styleCommand,
		"TTS delivery instructions for Command (empty=none)")
	f.StringVar(&flagVoiceStyleATIS, "voice-style-atis", styleATIS,
		"TTS delivery instructions for ATIS (empty=none)")
	f.StringVar(&flagTTSVoice, "tts-voice", "nova",
		"OpenAI TTS voice (female slot): alloy, ash, coral, echo, fable, nova, onyx, sage, shimmer")
	f.StringVar(&flagTTSVoiceMale, "tts-voice-male", "onyx",
		"OpenAI TTS voice (male slot): alloy, ash, coral, echo, fable, nova, onyx, sage, shimmer")
	f.IntVar(&flagVoiceRotateHrs, "voice-rotate-hours", 0,
		"Hours between female/male voice rotation on the tower channel (0=disabled: the field keeps its --tts-voice)")
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
	f.StringVar(&flagCarrierRadioIntensity, "carrier-radio-intensity", "carrier",
		"Radio effect intensity for Marshal and Deckboss: light, medium, carrier, heavy, extreme")
	f.StringVar(&flagTowerRadioIntensity, "tower-radio-intensity", "heavy",
		"Radio effect intensity for tower TX: light, medium, heavy, extreme")
	f.StringVar(&flagATISRadioIntensity, "atis-radio-intensity", "light",
		"Radio effect intensity for ATIS TX (kept clean since it's a recorded loop): light, medium, heavy, extreme")
	f.Float64Var(&flagOutputGainDB, "output-gain-db", 14,
		"Gain in dB applied to every radio-effect TX after the noise mix — raises voice and hiss together (0 = the level before 2026-10-10)")
	f.Float64Var(&flagTTSSpeed, "tts-speed", speedDeckboss,
		"TTS playback speed for tower/command — defaults to the Deckboss rate, which every other role also runs at")
	f.BoolVar(&flagScudwatchOnly, "scudwatch-only", false,
		"Run as Scud / launch-warning monitor only — no tower, ATIS, deckboss, or marshal")
	f.StringVar(&flagScudwatchFreq, "scudwatch-freq", "282.00",
		"Scudwatch broadcast frequency in MHz")
	f.StringVar(&flagScudwatchCallsign, "scudwatch-callsign", "SENTINEL",
		"Scudwatch SRS display callsign (dashes between syllables, no spaces — e.g. Darkstar-1-1)")
	f.StringVar(&flagScudwatchBullseye, "scudwatch-bullseye", "",
		"Bullseye reference for Scudwatch announcements as 'lat,lon' (default: airfield center)")
	registerLSOFlags(f)
	registerApproachGradeFlags(f)
	registerTankerFlags(f)
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
	f.DurationVar(&flagRunwaySlot, "runway-slot", 4*time.Hour,
		"Length of one runway-rotation slot. Default 4h matches production cadence; shorten (e.g. 60s) for live verification of slot rolls.")
	f.Float64Var(&flagRunwayCalmKts, "runway-calm-kts", 5,
		"With --runway-rotation=false: below this wind (kts) the runway rotates by wall clock, one per --runway-slot, instead of following the wind. 0 = always by wind.")
	f.BoolVar(&flagPositionCheck, "position-check", false,
		"Validate via Tacview that a pilot calling 'holding short' is actually within HoldShortValidationNm of the runway threshold. Off by default — enable for a session to observe before relying on it.")
	f.BoolVar(&flagDeckPositionCheck, "deck-position-check", false,
		"Deckboss assigns catapults from the caller's actual Tacview position on deck — bow cats 1/2 forward, waist cats 3/4 aft — instead of always taking the lowest free cat. Fails open to first-free when Tacview can't place the caller. Off by default.")
	f.BoolVar(&flagUnknownTraffic, "unknown-traffic-calls", false,
		"Tower calls a human-flown aircraft that closes inside 8 nm / 5,000 ft above the field without having talked to it: \"say intentions\". Once per aircraft per 10 min, one call per 10 s monitor tick. Off by default — trial on the dev rig first.")
	f.BoolVar(&flagRunwayVacateChase, "runway-vacate-chase", false,
		"Tower asks a landed aircraft to \"report clear of the runway\" when it has sat on the field below 40 kt for 90 s after its landing clearance without calling vacated. Once per landing. Off by default.")
	f.BoolVar(&flagDepartureCall, "departure-handoff-call", true,
		"Tower calls a departure itself at the field's departure distance (7 nm): \"<callsign>, Senaki Departure, seven miles, angels five, leaving my airspace, contact Command ...\". A pilot \"seven DME\" report after that gets \"copy\". --departure-handoff-call=false = the pilot's report only.")
	f.BoolVar(&flagTrafficAlerts, "traffic-alerts", true,
		"Tower warns two airborne aircraft within 10 nm of the field that are converging to a close pass (inside 0.25 nm and 500 ft within 30 s, 80+ kt closure, headings 45+ degrees apart): \"traffic alert, two o'clock, one mile, same altitude\". Formations and rejoins stay quiet. Each alert also goes to the comms report. --traffic-alerts=false turns them off.")
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
	resolveStylePresets()
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
	case flagLSOOnly:
		logSlug = "lso"
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

	af := airfield.ByICAO(flagAirfield)
	if af == nil {
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
	if flagLSOOnly       { mode = "lso-only" }
	if flagDeckbossFreq != "0" && flagDeckbossFreq != "" { mode = "deckboss" }
	log.Info().Str("mode", mode).Str("srs", flagSRSAddr).Msg("vsfg7-atc starting")
	if !flagCommandOnly && !flagATISOnly && !flagScudwatchOnly && !flagMarshalOnly && !flagLSOOnly {
		log.Info().
			Str("airfield", af.ICAO).
			Str("callsign", callsign).
			Float64("freqMHz", freqMHz).
			Str("srs", flagSRSAddr).
			Msg("starting vsfg7-atc")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runElevenCapWatch(ctx)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Info().Msg("shutting down")
		cancel()
	}()

	var wg sync.WaitGroup

	// Paddles (lso.go) is self-contained: its own Tacview reader and radio.
	if flagLSOOnly {
		runLSO(ctx, apiKey)
		log.Info().Msg("Paddles offline")
		return nil
	}

	// Apply runway-slot override before any AirfieldState is constructed so
	// the rotation anchor uses the requested cadence from t=0. Logged so any
	// non-default duration is loud in the operator console.
	if flagRunwaySlot != 4*time.Hour {
		airfield.RotationSlotDuration = flagRunwaySlot
		log.Warn().
			Dur("slot", flagRunwaySlot).
			Msg("DEBUG: runway-slot overridden — non-production cadence")
	}

	// ── ATC controller ────────────────────────────────────────────────────────
	atcCtrl := controller.NewATCController(callsign, af)
	if !flagRunwayRotation {
		atcCtrl.DisableRunwayRotation()
		if flagRunwayCalmKts > 0 {
			atcCtrl.SetCalmRotation(flagRunwayCalmKts)
		}
	}
	if flagPositionCheck {
		atcCtrl.SetPositionCheck(true)
		log.Info().Msg("Tacview hold-short position check enabled (--position-check)")
	}
	if flagUnknownTraffic {
		atcCtrl.SetUnknownTrafficCalls(true)
		log.Info().Msg("unknown-traffic \"say intentions\" calls enabled (--unknown-traffic-calls)")
	}
	if flagRunwayVacateChase {
		atcCtrl.SetRunwayVacateChase(true)
		log.Info().Msg("runway-vacate chase enabled (--runway-vacate-chase)")
	}
	if flagDepartureCall {
		atcCtrl.SetDepartureCalls(true, departureSignOff(flagAirfield))
		log.Info().Msg("departure handoff call enabled - tower calls departures at the departure distance")
	}
	// Towers only: Marshal and Deckboss run this controller's monitor too.
	towerMode := !flagCommandOnly && !flagATISOnly && !flagScudwatchOnly && !flagMarshalOnly &&
		(flagDeckbossFreq == "0" || flagDeckbossFreq == "")
	if flagTrafficAlerts && towerMode {
		atcCtrl.SetTrafficAlerts(true)
		log.Info().Msg("traffic alerts enabled - tower warns aircraft converging near the field")
	}
	atcCtrl.SetReportFn(func(ev controller.CommsEvent) { recordCommsEvent(callsign, ev) })
	// Seed weather. Priority: the mission the DCS server has loaded (the hook's
	// --miz-watch-file) > explicit --miz-path > newest .miz in --miz-dir >
	// --static-* flags. The loaded mission comes first because SKYEYE_MIZ and
	// "newest" both go stale on a rotation: on 2026-10-19 every role on all
	// three rigs booted on another mission's weather (Day "VFR 7 Isolated
	// Thunderstorms" for VFR 6, Night V1.30, Foothold 15 for 11) until mizwatch
	// caught up. .miz wind is true degrees (controller applies MagVar
	// internally). IsNight always comes from --static-night until Tacview's
	// mission time overrides it on the dashboard side.
	bootMiz := "" // the .miz the boot weather came from, for the live-weather watcher
	// A tower reports its ceiling above the field; the .miz gives it above sea
	// level. ATIS reads each tower's ceiling, so it follows. Carrier, Command
	// and ATIS-only roles keep MSL (the carrier is at sea level).
	ceilElevFt := 0
	if mode == "tower" {
		ceilElevFt = af.ElevationFt
	}
	seedFromMiz := func() bool {
		var mizPath string
		switch {
		case readMissionPointer(flagMizWatchFile) != "":
			mizPath = readMissionPointer(flagMizWatchFile)
		case flagMizPath != "":
			mizPath = flagMizPath
		case flagMizDir != "":
			if p, err := miz.FindNewestMiz(flagMizDir); err == nil {
				mizPath = p
			} else {
				log.Warn().Err(err).Str("dir", flagMizDir).Msg("no .miz found — using --static-* flags")
				return false
			}
		default:
			return false
		}
		w, err := miz.ReadMizWeather(mizPath)
		if err != nil {
			log.Warn().Err(err).Str("miz", mizPath).Msg(".miz weather read failed — using --static-* flags")
			return false
		}
		atcCtrl.SetFullWeather(w.WindDirTrue, w.WindKts, fieldCeilFt(w.CeilFt, ceilElevFt), w.VisNm, w.AltInHg, flagStaticNight)
		log.Info().
			Str("miz", filepath.Base(mizPath)).
			Float64("windDirTrue", w.WindDirTrue).
			Float64("windKts", w.WindKts).
			Float64("ceilFt", w.CeilFt).
			Float64("fieldCeilFt", fieldCeilFt(w.CeilFt, ceilElevFt)).
			Float64("visNm", w.VisNm).
			Float64("altInHg", w.AltInHg).
			Float64("tempC", w.TempC).
			Msg("Weather loaded from .miz")
		bootMiz = mizPath
		return true
	}
	if !seedFromMiz() {
		atcCtrl.SetFullWeather(flagStaticWindDir, flagStaticWindKts, flagStaticCeilFt, flagStaticVisSm, flagStaticAltInHg, flagStaticNight)
	}
	// Follow the mission the DCS server actually has loaded (weather rotation).
	go runMizWatch(ctx, &mizWatcher{
		pointerFile: flagMizWatchFile,
		read:        miz.ReadMizWeather,
		apply: func(w miz.Weather) {
			atcCtrl.SetFullWeather(w.WindDirTrue, w.WindKts, fieldCeilFt(w.CeilFt, ceilElevFt), w.VisNm, w.AltInHg, flagStaticNight)
		},
	}, bootMiz)
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
		var store *tacviewPositions
		if flagTacviewAddr != "" {
			tracker = newPilotTracker()
			store = newTacviewPositions()
			go runMiniTacview(ctx, flagTacviewAddr, store)
			srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)
			handoffTX := func(text, _ string) {
				mp3, err := synthesizeSpeech(ctx, apiKey, text, commandVoice(flagCommandVoice))
				if err != nil {
					log.Error().Err(err).Msg("Command handoff TTS failed")
					return
				}
				noteOwnTX(cmdFreq, text)
				transmitExternalAudioFile(ctx, mp3, cmdFreq, flagCommandName, srsHost, srsPort, flagExternalAudio)
			}
			go runCommandHandoffWatch(ctx, tracker, store, handoffTX)
			log.Info().Str("tacview", flagTacviewAddr).Msg("Command: tower-handoff watcher armed")
		}

		if flagJTACFreq > 0 {
			startJTACs(ctx, apiKey, store)
		}
		startTankers(ctx, apiKey, store)

		commandLoop(ctx, flagSRSAddr, cmdFreq, flagCommandName, apiKey, flagEAMPassword, flagCommandVoice, flagExternalAudio, tracker, store)
		return nil
	}

	// ── TX: ExternalAudio ─────────────────────────────────────────────────────
	txChan := make(chan towerTX, 16)
	atcCtrl.SetTransmitFn(func(ctx context.Context, text string) {
		select {
		case txChan <- towerTX{text: text}:
		default:
			log.Warn().Str("text", text).Msg("TX queue full")
		}
	})
	atcCtrl.SetDepartureTransmitFn(func(ctx context.Context, text string) {
		select {
		case txChan <- towerTX{text: text, departure: true}:
		default:
			log.Warn().Str("text", text).Msg("TX queue full")
		}
	})

	if af.ParkingFirst > 0 {
		atcCtrl.SetGroundTransmitFn(af.Name+" Ground", func(ctx context.Context, text string) {
			select {
			case txChan <- towerTX{text: text, ground: true}:
			default:
				log.Warn().Str("text", text).Msg("TX queue full")
			}
		})
	}

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
			case tx := <-txChan:
				text := tx.text
				voice := currentTowerVoice()
				profile := towerShiftProfile(voice, time.Now())
				if tx.departure {
					profile = departureProfile(voice)
				}
				if tx.ground {
					profile = groundProfile(voice)
				}
				log.Info().Str("text", text).Str("voice", voice).Bool("departure", tx.departure).Int("queued", len(txChan)).Msg("TX via OpenAI TTS + ExternalAudio")
				// 10s ceiling on TTS: OpenAI is either fast (~2s) or hung. No
				// SAPI fallback — operator preference is one consistent voice;
				// pilot will repeat if TX drops.
				ttsCtx, ttsCancel := context.WithTimeout(ctx, 10*time.Second)
				ttsStart := time.Now()
				mp3, err := synthesizeSpeech(ttsCtx, apiKey, text, profile)
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
	if flagApproachGrade {
		go runFieldGrade(ctx, flagTacviewAddr, af)
	}

	log.Info().
		Str("callsign", callsign).
		Float64("freqMHz", freqMHz).
		Msg("ATC online — ready for traffic")

	// Runway rotation ticker — checks every 60s (or twice per slot, whichever
	// is shorter) whether the slot has rolled and updates the active runway.
	// UpdateWeather also reads rotation but Tacview weather updates can be
	// sparse in stable conditions, so the ticker is the reliable trigger.
	if flagRunwayRotation || flagRunwayCalmKts > 0 {
		tickInterval := 60 * time.Second
		if airfield.RotationSlotDuration < 2*tickInterval {
			tickInterval = airfield.RotationSlotDuration / 2
		}
		if tickInterval < time.Second {
			tickInterval = time.Second
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(tickInterval)
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
			Dur("slot", airfield.RotationSlotDuration).
			Dur("tick", tickInterval).
			Bool("calmOnly", !flagRunwayRotation).
			Msg("runway rotation enabled — slot-based (calmOnly: only below --runway-calm-kts)")
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
				Name: "Kish ATIS", FreqMHz: 248.500, Voice: "fable",
				ICAO: "OIBK", TACAN: "", ILS: "",
				Advisory: "vSFG-7 traffic in area. Advise information on initial contact.",
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
				atisLoop(ctx, st, apiKey, flagEAMPassword, flagSRSAddr, atcCtrl, atisBroadcastIntervalSec(st, flagMap))
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
				flagDeckbossVoice, &deckbossCooldown, atcCtrl, deckState, flagDeckPositionCheck)
		}()
		log.Info().Float64("freq", deckFreqMHz).Bool("deckPositionCheck", flagDeckPositionCheck).Msg("Deckboss online")
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

// carrierWhisperPrompt is the vocabulary prompt whisperTranscribe sends for the
// Marshal and Deckboss freqs. Whisper sometimes returns it verbatim on noise.
const carrierWhisperPrompt = "Marshal, Raider, Venom, marking mom, angels, state, established, commencing, pushing, checking in, see you at ten, signal Charlie, BRC, altimeter, radio check, five by five"

// carrierPromptFragments are the distinctive pieces of carrierWhisperPrompt —
// "marshal", "raider", "angels" and "state" are left out because every real
// Marshal call carries them.
var carrierPromptFragments = []string{
	"venom", "marking mom", "established", "commencing", "pushing", "checking in",
	"see you at ten", "signal charlie", "brc", "altimeter", "radio check", "five by five",
}

// isPromptEcho reports whether Whisper handed back the prompt it was given
// instead of speech: eight or more of the prompt's comma-separated phrases,
// and at least a third of them, in one transcript. A real call carries a few
// (a busy close-out, "Command, Raider 302, copy, off target Bravo, Winchester,
// Bingo, RTB", carries eight of Command's 37). Day 2026-10-16 16:57: a
// 0.4 s blip on 282 came back as Command's whole prompt ("Command, Raider,
// Venom, Viper, request tasking, ... Winchester, Bingo, Joker, RTB") and
// Command answered it with "copy off target, Winchester, RTB" - the fixed
// fragment lists below only know the tower and carrier prompts.
func isPromptEcho(text, prompt string) bool {
	lower := strings.ToLower(text)
	n, total := 0, 0
	for _, frag := range strings.Split(strings.ToLower(prompt), ",") {
		if frag = strings.TrimSpace(frag); len(frag) >= 3 {
			total++
			if strings.Contains(lower, frag) {
				n++
			}
		}
	}
	return n >= 8 && n*3 >= total
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
		"tower, al ain tower",
		"raider, venom, radio",
		"raider, venom, marking mom",
	}
	for _, c := range compound {
		if strings.Contains(lower, c) {
			return true
		}
	}
	// Carrier prompt echo. The fragments above are the tower prompt, so the
	// Marshal/Deckboss prompt came straight through: on 2026-09-14 Whisper
	// returned carrierWhisperPrompt verbatim on the Syria Marshal freq, and
	// Marshal answered it as a radio check from "Raider". A real call uses a
	// few of these words; the echo uses all of them.
	carrierMatches := 0
	for _, frag := range carrierPromptFragments {
		if strings.Contains(lower, frag) {
			carrierMatches++
		}
	}
	if carrierMatches >= 5 {
		return true
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

// lostCallMinFrames is about one second of 40 ms SRS Opus frames: below that a
// dropped transcription is a squelch blip, not a lost call.
const lostCallMinFrames = 25

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
// splitModexRe matches a callsign word split from its number by a comma at the
// start of a segment: "Raider, 308".
var splitModexRe = regexp.MustCompile(`^([A-Za-z][A-Za-z'-]*)\s*,\s*(\d)`)

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
		// Whisper sometimes puts a comma inside the callsign ("Deck boss,
		// Raider, 308, comcheck", DCS3 2026-10-14) — rejoin "word, digits"
		// before the comma cut below, or the callsign comes out as "Raider".
		rest = splitModexRe.ReplaceAllString(rest, "$1 $2")
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
		" copy", " copies", " wilco", " roger", " proceeding",
	}
	lower := strings.ToLower(s)
	cut := len(s)
	for _, v := range verbs {
		if i := strings.Index(lower, v); i > 0 && i < cut {
			cut = i
		}
	}
	return endAtCallsignNumber(strings.TrimSpace(s[:cut]))
}

var callsignNumberRe = regexp.MustCompile(`^\d[\d-]*$`)

// endAtCallsignNumber ends a callsign at its number: "Raider 302 com" ->
// "Raider 302". Live 2026-10-15 09:50, Day: "Command, Raider 302 com check"
// came back "Raider 302 com, vSFG-7-Command" - the verb cut stopped at
// "check" and kept "com". Spaced digits stay together ("Raider 3 0 2"); a
// segment with no number, or one that starts with it, is left alone.
func endAtCallsignNumber(s string) string {
	f := strings.Fields(s)
	for i := 1; i < len(f); i++ {
		if !callsignNumberRe.MatchString(f[i]) {
			continue
		}
		j := i
		for j+1 < len(f) && callsignNumberRe.MatchString(f[j+1]) {
			j++
		}
		return strings.Join(f[:j+1], " ")
	}
	return s
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
// Whisper often writes the spoken decimal as "5 point 6", which Sscanf would
// read as a bare 5, so "point" is folded back into a decimal point first.
func extractFuelStateMarshal(lower string) float64 {
	idx := strings.Index(lower, "state")
	if idx < 0 { return 0 }
	after := strings.TrimSpace(lower[idx+5:])
	after = strings.Replace(after, " point ", ".", 1)
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

// dialSRSKeepAlive dials the SRS TCP endpoint with OS-level TCP keepalive enabled.
// SRS audio roles otherwise only notice a dropped server via the 90s read deadline,
// so a peer that vanishes without a clean FIN (DCS crash, VM pause, network blip)
// leaves the role silently deaf for up to 90s before it reconnects. Kernel keepalive
// probes (idle 15s, then 3 probes 5s apart) tear the socket down in ~30s, which
// makes the blocked Read return an error and triggers the reconnect. Keepalive ACKs
// are handled by the kernel and never delivered to the app, so they don't reset the
// application read deadline — this adds no false-disconnect risk on idle links.
func dialSRSKeepAlive(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{
		Timeout: 10 * time.Second,
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Idle:     15 * time.Second,
			Interval: 5 * time.Second,
			Count:    3,
		},
	}
	return d.DialContext(ctx, "tcp", addr)
}

// srsClientGUID builds the SRS client GUID for a role.
//
// This MUST be unique per process. SRS keys its client table on this string, so
// two roles that generate the same GUID collapse into a single slot: the later
// registration overwrites the earlier one's radio, and the loser goes silent in
// both directions while still looking connected.
//
// That is not hypothetical. The previous scheme was time.Now().UnixNano() alone,
// which is NOT unique on Windows -- the clock there is far coarser than a
// nanosecond, so processes launched in the same tick got identical values. On
// 2026-09-02 eight Syria towers launched within ~81ms of each other; only six
// reached SRS, and two of those carried another tower's callsign and frequency.
// PG (3 towers) and Caucasus (4) had been lucky, not correct.
//
// PID is unique among live processes, so it alone settles the simultaneous-launch
// case; the random tail covers PID reuse and the %1e6 wrap. digits is the total
// width of the numeric tail, kept per-role so GUID length does not change.
func srsClientGUID(prefix string, digits int) string {
	const pidDigits = 6
	randDigits := digits - pidDigits
	bound := 1
	for i := 0; i < randDigits; i++ {
		bound *= 10
	}
	return fmt.Sprintf("%s%0*d%0*d", prefix, pidDigits, os.Getpid()%1000000, randDigits, rand.Intn(bound))
}

func srsLoop(ctx context.Context, addr string, freqMHz float64, callsign, apiKey, eamPassword string, txCooldown *int64, atcCtrl *controller.ATCController) {
	freqHz := freqMHz * 1e6
	guid := srsClientGUID("vsfg7atc", 14)
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

		// TCP connection (OS keepalive — see dialSRSKeepAlive)
		tcpConn, err := dialSRSKeepAlive(ctx, addr)
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
						if (silent || tooLong) && len(tx.opusFrames) > 9 {
							frames := tx.opusFrames
							log.Info().Str("origin", origin).Int("frames", len(frames)).Msg("flushing transmission to Whisper")
							delete(transmissions, origin)
							txMu.Unlock()
							go transcribeAndHandle(ctx, apiKey, flagFFmpeg, frames, callsign, origin, atcCtrl)
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
				if rxMuted(txCooldown, freqMHz) {
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

// srsModulation is the SRS modulation code for a frequency: 1 (FM) in the
// 30-88 MHz military VHF-FM band, where the Hornet's ARC-210 and the Viper's
// ARC-222 only do FM, else 0 (AM). SRS routes voice by frequency AND
// modulation, so a 61.000 net registered as AM hears nobody.
func srsModulation(freqMHz float64) int {
	if freqMHz >= 30 && freqMHz < 88 {
		return 1
	}
	return 0
}

// srsModulationName is srsModulation as ExternalAudio's --modulations takes it.
func srsModulationName(freqMHz float64) string {
	if srsModulation(freqMHz) == 1 {
		return "FM"
	}
	return "AM"
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
						"modulation": srsModulation(freqHz / 1e6),
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
						"modulation": srsModulation(freqHz / 1e6),
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
func transcribeAndHandle(ctx context.Context, apiKey, ffmpegPath string, frames [][]byte, callsign, origin string, atcCtrl *controller.ATCController) {
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
	if isWhisperHallucination(text) || isPromptEcho(text, prompt) {
		log.Warn().Str("text", text).Msg("Whisper hallucination — dropping")
		if len(frames) >= lostCallMinFrames {
			// A real call was keyed and lost; short blips are not worth a line.
			atcCtrl.ReportLostCall(text, fmt.Sprintf("a %.1f s call came back from Whisper as its prompt or noise - call lost, nothing said", float64(len(frames))*0.04))
		}
		return
	}
	log.Info().Str("text", text).Msg("recognized")
	req := controller.ParseIntent(text, callsign)
	if req == nil {
		// Logged so a call to another field on this frequency, or a field name
		// Whisper mangled past matching, shows up instead of silence.
		log.Info().Str("text", text).Str("tower", callsign).Msg("not addressed to this tower — ignored")
		return
	}
	req.Origin = origin
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

// ttsKey includes the delivery instructions: two roles can share a voice and
// speed but sound different, so leaving instructions out of the key would let
// one role serve the other a cached MP3 in the wrong style.
func ttsKey(v voiceProfile, text string) string {
	return fmt.Sprintf("%s:%.2f:%s:%s:%s:%s", v.voice, v.speed, v.instructions, v.eleven.ID, v.eleven.model(), text)
}

func (c *ttsCache) get(v voiceProfile, text string) ([]byte, bool) {
	key := ttsKey(v, text)
	c.mu.RLock()
	got, ok := c.items[key]
	c.mu.RUnlock()
	if ok {
		atomic.AddInt64(&c.hits, 1)
	}
	return got, ok
}

func (c *ttsCache) set(v voiceProfile, text string, mp3 []byte) {
	key := ttsKey(v, text)
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

// TTS cache prewarm was removed. It synthesized ~30 bare response bodies at
// startup (twice, when voice rotation was on), but globalTTSCache is keyed by
// exact text and every tower TX is callsign-led — "{callsign}, {tower}, {body}."
// — so no prewarmed entry could ever be read. It cost ~60 TTS calls and ~12s of
// rate-limit sleep per tower start for a guaranteed zero hit rate.
//
// To make prewarming actually work the composer would have to stitch a cached
// body MP3 onto a per-callsign+tower prefix MP3, which is the follow-up the old
// implementation flagged and never did. Recover the phrase list from git history
// if that gets built.

// currentTowerVoice is the tower TTS voice: the one the start script gives the
// field (--tts-voice). Each field's voice picks its ElevenLabs cast (Senaki
// alloy -> Lisa, Tbilisi onyx -> George), so it must not change under the
// pilot. --voice-rotate-hours > 0 brings back the old female/male swap on a
// wall-clock bucket; it was on by default (4h) until 2026-10-14, when it
// turned Senaki male mid-session ("Senaki changed to a male voice").
func currentTowerVoice() string {
	if flagVoiceRotateHrs <= 0 || flagTTSVoiceMale == "" {
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
// 14 chars/sec rate is empirical at our old 0.88 speed; every role now runs at
// 1.20, where actual playback is faster still, so 14 keeps the estimate
// conservative (over-cools rather than under-cools) — deliberately left alone
// when the rates went up, since under-cooling is the failure that bites.
// The 5s margin covers ExternalAudio.exe
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

// voiceProfile is everything that decides how a role sounds: which OpenAI voice
// speaks, how fast, and — the part that actually separates one controller from
// another — how they deliver it.
//
// gpt-4o-mini-tts accepts an `instructions` string that steers tone, cadence,
// and affect. It is the whole reason to use that model over tts-1, which
// ignores the field. Without it every role reads in the same neutral
// audiobook register regardless of which voice is assigned.
type voiceProfile struct {
	voice        string
	speed        float64
	instructions string
	// eleven, when set and the rig has an ElevenLabs key, speaks the line
	// instead; the OpenAI voice above is the fallback (tts_elevenlabs.go).
	eleven elevenVoice
}

// Delivery briefs per role. A tower controller, a marshal controller, and a
// flight-deck boss do not sound alike, and the difference is cadence and affect
// far more than it is timbre.
const (
	styleTower = "Speak as a US military air traffic control tower controller. " +
		"Clipped, matter-of-fact, brisk. Flat affect, no warmth, no rising intonation. " +
		"Run grouped numbers together as a single phrase rather than reading them one by one."

	styleMarshal = "Speak as a US Navy carrier Marshal controller. " +
		"Calm, measured, deliberate pacing. Unhurried and steady even in bad weather. " +
		"Authoritative but never rushed."

	styleDeckboss = "Speak as a US Navy carrier flight deck boss on the deck-edge handset, " +
		"raising your voice over jet noise. Loud, urgent, punchy, commanding. Very short clauses."

	// Operator 2026-10-14: "Command need to be stern English male voice", then
	// the same day "The command voice is not good" / "need american".
	styleCommand = "Speak as a US military strike command controller on a tactical net: " +
		"a man with a General American accent and a low, grounded voice. " +
		"Stern, firm and authoritative. Clipped, steady, businesslike. No warmth, no emotion."

	// styleJTAC: Hitman 1-1 is a ground party near the target on a manpack
	// radio, not a controller at a console (operator 2026-10-14: "realistic
	// ... radio static type feel since it is a JTAC from ground").
	// Australian man, operator 2026-10-14.
	styleJTAC = "Speak as a JTAC on the ground, an Australian man with a clear, natural Australian accent, " +
		"talking on a manpack radio from a covered position near the target. " +
		"Low, calm, clipped and deliberate, slightly hushed as if keeping your voice down. Crisp on every number. No emotion."
	// styleJTAC2: Hitman 2-1, the OpenAI fallback for its American ElevenLabs voice.
	// styleOverlord: the OpenAI fallback for Overlord Actual's American ElevenLabs voice.
	styleOverlord = "Speak as a senior American commander on a military radio net, General American accent. " +
		"Calm, measured and commanding, unhurried, every word deliberate. No emotion."
	styleJTAC2 = "Speak as a JTAC on the ground, a young American man with a clear, neutral General American accent, " +
		"talking on a manpack radio from a covered position near the target. " +
		"Calm, relaxed and confident, clipped on the radio. Crisp on every number. No emotion."

	styleATIS = "Read as a recorded automated terminal information broadcast. " +
		"Even, neutral, unhurried, identical inflection on every phrase. No emphasis anywhere."

	// Foothold voice casting, operator 2026-09-19. Passed per role (tower via
	// --voice-style-tower in start_towers_syria.bat, Marshal via
	// --voice-style-marshal in start_marshal_syria.bat) or per ATIS station.
	styleTowerBritish = "Speak as a Royal Air Force air traffic controller at RAF Akrotiri: " +
		"a woman with a crisp southern British (Received Pronunciation) accent. " +
		"Clipped, precise, brisk, matter-of-fact. Flat affect, no warmth. " +
		"Run grouped numbers together as a single phrase rather than reading them one by one."

	styleATISBritish = "Read as a recorded RAF aerodrome information broadcast by a woman " +
		"with a British (Received Pronunciation) accent. Even, neutral, unhurried, " +
		"identical inflection on every phrase. No emphasis anywhere."

	// Syria ATIS: English read in the local accent (operator 2026-09-19: "make sure
	// the other ATIS airfields reflect English and local dialect"). The second-language
	// pass uses the same instructions, so it sounds native too.
	styleMarshalStern = "Speak as a US Navy carrier Marshal controller, a woman: stern, firm, " +
		"no-nonsense and exacting. Calm, measured, deliberate pacing. Authoritative and " +
		"clipped, never warm, never rushed."
)

// stylePresets are short names a start script can pass to --voice-style-* in
// place of a whole sentence, which is fragile inside the bats' nested cmd /c
// quoting. Anything that is not a preset name is used as literal instructions.
var stylePresets = map[string]string{
	"raf-british":   styleTowerBritish,
	"marshal-stern": styleMarshalStern,
}

func resolveStylePresets() {
	for _, p := range []*string{&flagVoiceStyleTower, &flagVoiceStyleMarshal,
		&flagVoiceStyleDeckboss, &flagVoiceStyleCommand, &flagVoiceStyleATIS, &flagVoiceStyleJTAC} {
		if full, ok := stylePresets[*p]; ok {
			*p = full
		}
	}
}

// styleATISAccent is the shared ATIS delivery read in a given local accent.
func styleATISAccent(accent string) string {
	return "Read as a recorded automated terminal information broadcast, spoken by a local " +
		"controller with a " + accent + " accent. Even, neutral, unhurried, identical " +
		"inflection on every phrase. No emphasis anywhere."
}

// Per-role speeds. All roles run at 1.20 since 2026-09-25 (operator request);
// 1.15 from 2026-09-19, and 1.10 from 2026-08-16, the Deckboss rate (operator
// preference): the deck boss's clip was the one that sounded right, so every
// channel — live controller and recorded loop alike — reads at one pace.
// Marshal was 1.00 (a Case III approach is read to a pilot who is writing it
// down) and ATIS was 0.97 (clarity on a loop); both were raised to match.
//
// Tower/Command speed is the --tts-speed flag, whose default is speedDeckboss.
// The constants stay separate so a single role can be pulled back off 1.20
// without disturbing the others, and overriding --tts-speed on one process
// still works if a field needs to differ.
const (
	speedMarshal  = 1.20
	speedDeckboss = 1.20
	speedATIS     = 1.20
)

// towerVoice / marshalVoice / deckbossVoice / commandVoice / atisVoice build the
// profile for each role from the flags. Kept as functions rather than vars so
// they pick up the rotating tower voice at call time.
func towerVoice() voiceProfile {
	return towerProfile(currentTowerVoice())
}

func marshalVoice(v string) voiceProfile {
	return voiceProfile{voice: v, speed: speedMarshal, instructions: flagVoiceStyleMarshal,
		eleven: elMarshal(flagElevenMarshalVoice)}
}

func deckbossVoice(v string) voiceProfile {
	return voiceProfile{voice: v, speed: speedDeckboss, instructions: flagVoiceStyleDeckboss,
		eleven: elDeckboss(flagElevenDeckbossVoice)}
}

// towerTX is one tower transmission; departure = spoken as "<field>
// Departure" in the Departure voice (the 7-mile handoff and its "copy").
type towerTX struct {
	text      string
	departure bool
	ground    bool // "<field> Ground": parking spots (pkg/controller/parking.go)
}

// departureProfile is the tower's Departure position: the tower's delivery and
// OpenAI fallback, its own ElevenLabs voice (operator 2026-10-15: "maybe have
// departure a different voice").
// groundProfile is the tower's Ground position (parking spots, taxi routes):
// the tower's own voice unless --elevenlabs-ground-voice gives it another
// (operator 2026-10-16: "Have Senaki Ground, tower and Traffic all use the
// same voice").
func groundProfile(towerVoice string) voiceProfile {
	p := towerShiftProfile(towerVoice, time.Now())
	if flagElevenGroundVoice != "" {
		p.eleven = elTower(flagElevenGroundVoice, "Ground")
	}
	return p
}

func departureProfile(towerVoice string) voiceProfile {
	p := towerProfile(towerVoice)
	p.eleven = elTower(flagElevenDepartVoice, "Departure")
	return p
}

// commandSpeedUp is Command's rate over the shared --tts-speed (operator
// 2026-10-15: "speed up command by 10%"); the ElevenLabs read is in elCommand.
const commandSpeedUp = 1.10

func commandVoice(v string) voiceProfile {
	return voiceProfile{voice: v, speed: flagTTSSpeed * commandSpeedUp, instructions: flagVoiceStyleCommand,
		eleven: elCommand(flagElevenCommandVoice)}
}

func atisVoice(v string) voiceProfile {
	return voiceProfile{voice: v, speed: speedATIS, instructions: flagVoiceStyleATIS}
}

// atisVoiceFor is atisVoice with the station's own Style, when it has one.
func atisVoiceFor(st *atisStation) voiceProfile {
	p := atisVoice(st.Voice)
	if st.Style != "" {
		p.instructions = st.Style
	}
	return p
}

// synthesizeSpeech checks cache first, falls back to API on miss.
func synthesizeSpeech(ctx context.Context, apiKey, text string, v voiceProfile) ([]byte, error) {
	if elevenCapped() {
		v.eleven = elevenVoice{} // past the credit cap: OpenAI only, cached ElevenLabs clips too (eleven_cap.go)
	}
	if mp3, ok := globalTTSCache.get(v, text); ok {
		log.Debug().Str("text", text).Msg("TTS cache hit")
		return mp3, nil
	}
	if mp3, ok := speakElevenLabs(ctx, text, v); ok {
		globalTTSCache.set(v, text, mp3)
		return mp3, nil
	}
	// OpenAI: no ElevenLabs voice for this role, or it failed. Cached under
	// the profile without the ElevenLabs voice, so a recovered ElevenLabs is
	// not shadowed by the fallback clip.
	v.eleven = elevenVoice{}
	if mp3, ok := globalTTSCache.get(v, text); ok {
		return mp3, nil
	}
	mp3, err := synthesizeSpeechAPI(ctx, apiKey, text, v)
	if err != nil {
		return nil, err
	}
	globalTTSCache.set(v, text, mp3)
	return mp3, nil
}

// synthesizeSpeechAPI calls OpenAI TTS API directly — bypasses cache.
// Uses gpt-4o-mini-tts for more natural cadence than the legacy tts-1 model.
// Voice list is unchanged: alloy, ash, ballad, coral, echo, fable, onyx, nova,
// sage, shimmer, verse.
// translateATIS asks OpenAI to translate aviation ATIS text into the named
// language (e.g. "Arabic", "Russian"). Returns the translated string, or an
// error if the API call fails. Caller is responsible for falling back
// gracefully (e.g. broadcast English only). The second-language choice is
// theatre-driven — see atisSecondLangForMap.
func translateATIS(ctx context.Context, apiKey, text, language string) (string, error) {
	sysPrompt := fmt.Sprintf("Translate the following aviation ATIS broadcast to %s. Use ICAO phraseology conventions where applicable. Numbers should be spelled out as %s words for clear TTS pronunciation. Return ONLY the %s translation, no preamble or explanation.", language, language, language)
	body := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
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
	resp, err := openAIClient.Do(req)
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

// openAIClient bounds every OpenAI call. http.DefaultClient has no timeout and
// the callers pass the process-lifetime ctx, so one stalled request blocked its
// caller for good: on 2026-10-14 Kobuleti and then Batumi ATIS each hung in TTS
// for minutes while holding the station lock, silent on air and in the log.
// A timeout errors out instead, and the ATIS loop retries on its next tick.
var openAIClient = &http.Client{Timeout: 60 * time.Second}

func synthesizeSpeechAPI(ctx context.Context, apiKey, text string, v voiceProfile) ([]byte, error) {
	speed := v.speed
	if speed <= 0 {
		speed = speedDeckboss
	}
	body := map[string]interface{}{
		"model": "gpt-4o-mini-tts",
		"input": text,
		"voice": v.voice,
		"speed": speed,
	}
	// Only gpt-4o-mini-tts honours instructions; tts-1/tts-1-hd reject it.
	if v.instructions != "" {
		body["instructions"] = v.instructions
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
	resp, err := openAIClient.Do(req)
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
	resp, err := openAIClient.Do(req)
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
	w.WriteField("prompt", carrierWhisperPrompt)
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := openAIClient.Do(req)
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


// radioTier returns the noise amplitude and bandpass corners for a radio
// effect intensity.
func radioTier(intensity string) (noiseAmp float64, hpf, lpf int) {
	// Retuned 2026-10-14 (operator, on the ElevenLabs A/B clips: "lessen the
	// radio harshness ... voice seem low volume but loud on static, need more
	// bass"): roughly half the hiss of the 2026-08-16 values, and the low cut
	// moved from 350-600 Hz down to 150-300 Hz so the voice keeps its chest.
	// The tiers keep their order, light to extreme.
	switch strings.ToLower(intensity) {
	case "light":
		return 0.016, 150, 3600
	case "heavy":
		return 0.060, 220, 3200
	case "extreme":
		return 0.100, 300, 2800
	case "carrier":
		// Marshal and Deckboss (operator 2026-10-15: "add a little radio hiss
		// to deckboss and marshall"): half again medium's hiss, a shade
		// narrower, still well short of the towers' heavy.
		return 0.045, 200, 3300
	case "field":
		// The JTAC's manpack FM set: narrower than a console's radio and
		// noisier than "heavy", short of "extreme" so numbers stay clear.
		return 0.075, 250, 3000
	default: // medium
		return 0.030, 180, 3400
	}
}

// radioFilter builds the ffmpeg graph that turns clean TTS into a radio
// transmission in one pass: level the voice, bandpass it to ~radio bandwidth,
// compress it, mix in pink-noise carrier hiss, bring the result up to
// transmission level, then splice a PTT key-up click (~70ms white noise,
// 1500–4500 Hz) onto the front and a squelch tail (~180ms pink noise, faded)
// onto the back. The clicks come from anoisesrc, so no audio assets are
// bundled.
//
// levelDB levels the voice before anything else touches it. gpt-4o-mini-tts
// does not return every voice at the same level: measured 2026-10-10, sage
// (Command, Syria Marshal) came back at about -34 dB mean against -20 dB for
// alloy/nova/shimmer. Nothing downstream makes that up — a -34 dB voice sits
// under the compressor's threshold and the noise is mixed in at a fixed
// amplitude, so Command went out 14 dB down and buried in its own hiss while
// the towers sounded fine.
//
// outGainDB is applied after the mix, so it raises voice and hiss together
// and leaves the character of each tier alone. Without it the compressor (no
// makeup gain) and amix (which halves both inputs) leave the voice peaking
// near -20 dB — 11 dB under the key-up click in front of it, and well short
// of what the limiter was put there to catch.
//
// alimiter stays last in the voice path: digital clipping sounds like crackle
// rather than like a driven radio, and the limiter gives the hard-driven UHF
// character the compressor alone doesn't.
// radioClicks is the PTT key-up click and the squelch tail for a tier:
// amplitude and length of each. The carrier tier (Marshal, Deckboss) keys up
// harder and drops a longer squelch (operator 2026-10-15: "more radio click");
// every other tier keeps the original clicks.
func radioClicks(intensity string) (upAmp, upSec, dnAmp, dnSec float64) {
	if strings.ToLower(intensity) == "carrier" {
		return 0.85, 0.10, 0.65, 0.30
	}
	return 0.55, 0.07, 0.4, 0.18
}

func radioFilter(intensity string, levelDB, outGainDB float64) string {
	noiseAmp, hpf, lpf := radioTier(intensity)
	upAmp, upSec, dnAmp, dnSec := radioClicks(intensity)
	return fmt.Sprintf(
		// bass: +5 dB shelf at 150 Hz puts back the body the radio band cuts.
		// Compression 4:1 (was 8:1) keeps the voice punchy without the squash.
		// amix weights 1:0.5 sit the hiss under the voice instead of level with it.
		"[0:a]volume=%.1fdB,highpass=f=%d,lowpass=f=%d,"+
			"bass=g=5:f=150:w=0.7,"+
			"acompressor=threshold=-22dB:ratio=4:attack=8:release=80,"+
			"volume=1.15[voice];"+
			"anoisesrc=color=pink:amplitude=%.3f:duration=0[noise];"+
			"[voice][noise]amix=inputs=2:duration=first:dropout_transition=0:weights=1 0.5,"+
			"volume=%.1fdB,"+
			"alimiter=limit=0.95:attack=1:release=40,"+
			// 44100/mono so concat can match the synthetic clicks.
			"aresample=44100,aformat=channel_layouts=mono[body];"+
			"anoisesrc=color=white:amplitude=%.2f:duration=%.2f:sample_rate=44100,"+
			"highpass=f=1500,lowpass=f=4500,"+
			"afade=t=in:st=0:d=0.005,afade=t=out:st=%.3f:d=0.02[click_up];"+
			"anoisesrc=color=pink:amplitude=%.2f:duration=%.2f:sample_rate=44100,"+
			"highpass=f=300,lowpass=f=4000,"+
			"afade=t=out:st=0.02:d=%.2f[click_dn];"+
			"[click_up][body][click_dn]concat=n=3:v=0:a=1[out]",
		levelDB, hpf, lpf, noiseAmp, outGainDB,
		upAmp, upSec, upSec-0.02, dnAmp, dnSec, dnSec-0.02,
	)
}

// levelFilter is the whole graph when the radio effect is off: the voice is
// still levelled, so a quiet voice is not quiet just because it is clean.
func levelFilter(levelDB float64) string {
	return fmt.Sprintf("[0:a]volume=%.1fdB,alimiter=limit=0.95:attack=1:release=40[out]", levelDB)
}

// runAudioFilter renders inPath through an ffmpeg filter graph ending in
// [out] to an MP3 at outPath, at the rate ExternalAudio wants.
func runAudioFilter(ffmpegPath, inPath, outPath, filter string) error {
	cmd := exec.Command(ffmpegPath,
		"-y",
		"-i", inPath,
		"-filter_complex", filter,
		"-map", "[out]",
		"-c:a", "libmp3lame",
		"-ar", strconv.Itoa(srsAudioRateHz),
		"-b:a", "64k",
		outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, string(out))
	}
	if fi, err := os.Stat(outPath); err != nil || fi.Size() == 0 {
		return fmt.Errorf("ffmpeg produced empty output")
	}
	return nil
}

// voiceTargetMeanDB is the mean level every TTS voice is brought to ahead of
// the radio effect. It is where the tower voices already land on their own, so
// levelling leaves them as they were and only lifts the quiet ones.
const voiceTargetMeanDB = -20.0

// levelGainNotableDB is the correction past which a transmission's levelling
// is logged at info rather than debug: a voice that far off the others is the
// "why is Command so quiet" report waiting to happen.
const levelGainNotableDB = 6.0

// levelGainDB returns the gain that takes a clip measured at meanDB to
// voiceTargetMeanDB. Clamped so a near-silent or mis-measured clip can't be
// driven into the limiter, and so a hot one is never pulled down far.
func levelGainDB(meanDB float64) float64 {
	return math.Max(-6, math.Min(24, voiceTargetMeanDB-meanDB))
}

var meanVolumeRe = regexp.MustCompile(`mean_volume:\s*(-?[0-9.]+) dB`)

// measureMeanVolumeDB reads a clip's mean level with ffmpeg's volumedetect.
// ok is false on any failure, and the caller then applies no gain.
func measureMeanVolumeDB(path, ffmpegPath string) (float64, bool) {
	out, err := exec.Command(ffmpegPath,
		"-hide_banner", "-nostats",
		"-i", path,
		"-af", "volumedetect",
		"-f", "null", "-",
	).CombinedOutput()
	if err != nil {
		return 0, false
	}
	return parseMeanVolumeDB(string(out))
}

func parseMeanVolumeDB(ffmpegOutput string) (float64, bool) {
	m := meanVolumeRe.FindStringSubmatch(ffmpegOutput)
	if m == nil {
		return 0, false
	}
	mean, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return mean, true
}

// txIntensity picks the radio effect tier for a transmitting callsign:
//   - Tower callsigns end in "-ATC" (e.g. OMAM-ATC) → heavier static for that
//     punchy ATC sound.
//   - ATIS station callsigns end with " ATIS" (e.g. "Liwa ATIS", "Kish ATIS")
//     → kept clean since it's a recorded loop.
//   - Marshal (OMDM-MSH) and Deckboss (OMDM-DKB) → the carrier tier, a
//     little more hiss than Command.
//   - Everything else (vSFG-7-Command) gets the general operational intensity.
func txIntensity(callsign string) string {
	switch {
	case callsign == "OMDM-MSH" || callsign == "OMDM-DKB":
		return flagCarrierRadioIntensity
	case strings.HasSuffix(callsign, "-ATC"):
		return flagTowerRadioIntensity
	case strings.HasSuffix(callsign, " ATIS"):
		return flagATISRadioIntensity
	case flagJTACName != "" && callsign == flagJTACName,
		flagJTAC2Freq > 0 && flagJTAC2Name != "" && callsign == flagJTAC2Name:
		return flagJTACRadioIntensity
	}
	return flagRadioIntensity
}

// prepareTX levels the clean TTS clip at inPath and, with the radio effect on,
// runs it through the radio chain. It returns the path to transmit and a
// cleanup func. On any ffmpeg failure it returns inPath untouched, so a
// broadcast never drops to silence.
func prepareTX(inPath, callsign string) (string, func()) {
	levelDB := 0.0
	mean, ok := measureMeanVolumeDB(inPath, flagFFmpeg)
	if !ok {
		log.Warn().Str("callsign", callsign).Msg("voice level measurement failed — transmitting unlevelled")
	} else {
		levelDB = levelGainDB(mean)
		ev := log.Debug()
		if math.Abs(levelDB) >= levelGainNotableDB {
			ev = log.Info()
		}
		ev.Str("callsign", callsign).
			Float64("meanDB", mean).
			Float64("gainDB", math.Round(levelDB*10)/10).
			Msg("voice levelled")
	}

	var filter string
	switch {
	case flagRadioEffect:
		filter = radioFilter(txIntensity(callsign), levelDB, flagOutputGainDB)
	case math.Abs(levelDB) >= 0.5:
		filter = levelFilter(levelDB)
	default:
		return inPath, func() {}
	}
	outPath := inPath + ".tx.mp3"
	if err := runAudioFilter(flagFFmpeg, inPath, outPath, filter); err != nil {
		os.Remove(outPath)
		log.Warn().Err(err).Str("callsign", callsign).Msg("TX audio processing failed — using clean audio")
		return inPath, func() {}
	}
	return outPath, func() { os.Remove(outPath) }
}

// concatMP3WithSilence re-encodes two MP3 byte slices into a single MP3 with
// `silenceSec` seconds of silence between them. Used by ATIS to merge EN+AR
// into one clean stream — byte-level MP3 concat is fragile and can cause some
// decoders (DCS-SR-ExternalAudio) to play the two halves simultaneously.
// srsAudioRateHz is the sample rate we hand to DCS-SR-ExternalAudio.
//
// ExternalAudio always converts its input to mono 16-bit PCM at 16kHz before
// Opus-encoding it, so anything else buys nothing and costs a resample. This
// used to be 44100, which made every bilingual ATIS the most expensive file we
// produce: ~70s of audio resampled 44100 -> 16000 on the fly.
//
// That mattered because ExternalAudio 2.4.0.0 crashes partway through long
// transmissions -- "A callback was made on a garbage collected delegate of type
// MMTimerProc::Invoke", exit 0x80131623 -- killing the process mid-broadcast.
// On 2026-09-02 the logs held 827 such failures, 798 of them ATIS, dying
// anywhere from 3% to 79% through the file. Pilots heard a few seconds of a
// station and then nothing, on every field, which read as "SRS not
// transmitting". Crashes broke down 1586 log lines at 44100Hz source against 4
// at 24000Hz -- the long resampled files, not the short ones.
//
// Matching ExternalAudio's own target rate removes the resample entirely and
// shrinks the file, which shortens the window the crash has to land in. It is a
// mitigation, not a cure: the bug is in ExternalAudio, not here.
const srsAudioRateHz = 16000

// atisConcatFilter builds the ffmpeg filter_complex that joins the English and
// local-language ATIS passes with a gap between them.
//
// Split out so it can be tested. A malformed filter here does NOT surface as an
// outage: concatMP3WithSilence just returns an error and the caller falls back
// to broadcasting English only, so bilingual ATIS quietly disappears with a
// single warn line. That happened on 2026-09-02 with "%[1].3f" -- Go wants the
// argument index AFTER the precision ("%.3[1]f"), so it rendered
// "%!f(BADINDEX)" and ffmpeg rejected the atrim duration. Positional args now.
func atisConcatFilter(silenceSec float64) string {
	return fmt.Sprintf(
		"[0:a]aresample=%d,aformat=channel_layouts=mono[en];"+
			"[1:a]aresample=%d,aformat=channel_layouts=mono[ar];"+
			"anullsrc=r=%d:cl=mono,atrim=duration=%.3f[sil];"+
			"[en][sil][ar]concat=n=3:v=0:a=1[out]",
		srsAudioRateHz, srsAudioRateHz, srsAudioRateHz, silenceSec,
	)
}

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

	filter := atisConcatFilter(silenceSec)

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-i", tmpEn.Name(),
		"-i", tmpAr.Name(),
		"-filter_complex", filter,
		"-map", "[out]",
		"-c:a", "libmp3lame",
		"-ar", strconv.Itoa(srsAudioRateHz),
		"-b:a", "64k",
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
	// Our receivers on this frequency ignore what they hear until playback
	// has ended (tx_guard.go).
	defer beginTX(freqMHz)()
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

	txPath, cleanup := prepareTX(tmpPath, callsign)
	defer cleanup()

	safeName := strings.ReplaceAll(callsign, " ", "-")
	file := txPath
	for resume := 0; ; resume++ {
		args := []string{
			"--file", file,
			"--freqs", fmt.Sprintf("%.3f", freqMHz),
			"--modulations", srsModulationName(freqMHz),
			"-c", "2",
			"-n", safeName,
			"-p", srsPort,
			"-v", "1",
		}
		log.Debug().Strs("args", args).Msg("ExternalAudio file TX")
		out, err := runExternalAudio(ctx, exePath, args)
		if err == nil {
			log.Debug().Int("resumes", resume).Msg("ExternalAudio file TX ok")
			return
		}
		if ctx.Err() != nil {
			log.Warn().Err(ctx.Err()).Msg("ExternalAudio file TX timed out — killed")
			return
		}
		sentMs, midPlay := externalAudioProgress(string(out))
		if !midPlay && externalAudioGCOn.Load() && resume == 0 {
			// Failed before sending a sample: rule out the GC setting (it is
			// untested against every ExternalAudio build) by trying once
			// without it; if that works it stays off for this process.
			if out2, err2 := runExternalAudioPlain(ctx, exePath, args); err2 == nil {
				externalAudioGCOn.Store(false)
				log.Warn().Str("output", string(out)).Msg("ExternalAudio failed with the GC setting and worked without it - GC setting off for this session")
				return
			} else {
				out, err = out2, err2
				sentMs, midPlay = externalAudioProgress(string(out))
			}
		}
		if !midPlay || resume == externalAudioResumes {
			log.Error().Err(err).Int("resumes", resume).Str("output", string(out)).Msg("ExternalAudio file error")
			return
		}
		rest, rerr := cutMP3From(flagFFmpeg, file, sentMs)
		if rerr != nil {
			log.Error().Err(err).AnErr("cutErr", rerr).Str("output", string(out)).Msg("ExternalAudio file error — could not resend the rest")
			return
		}
		if rest == "" {
			log.Info().Str("callsign", callsign).Int("sentMs", sentMs).Msg("ExternalAudio crashed at the very end - nothing left to resend")
			return
		}
		defer os.Remove(rest)
		log.Warn().Err(err).Str("callsign", callsign).Float64("freq", freqMHz).Int("sentMs", sentMs).Int("resume", resume+1).
			Msg("ExternalAudio crashed mid-transmission - resending the rest")
		file = rest
	}
}

// ExternalAudio (SRS's .NET sender) dies mid-play with "A callback was made on
// a garbage collected delegate of type ...MMTimerProc::Invoke": its playback
// timer's callback is collected by a GC while the clip is still playing. The
// longer the clip the likelier a GC lands in it - ATIS (~50 s, EN + local)
// was cut off 6-14 times a morning on every rig (2026-10-15), anywhere 2-40 s
// in; tower calls (3-10 s) almost never. Two layers:
//   - a gen0 budget large enough that a broadcast never triggers a GC, so the
//     timer callback is never collected (both env spellings: .NET Framework
//     reads COMPlus_, .NET 5+ reads DOTNET_ too);
//   - if it dies anyway, the rest of the clip is resent from the point its
//     own log says it had reached ("sent 14000ms"), up to externalAudioResumes
//     times. Progress is logged every 2 s, so the resend repeats up to 2 s
//     rather than skipping any.

const externalAudioResumes = 3

var externalAudioEnv = []string{"DOTNET_GCgen0size=0x20000000", "COMPlus_GCgen0size=0x20000000"}

// externalAudioGCOn turns the GC setting off for the session if ExternalAudio
// is seen to fail with it and work without it.
var externalAudioGCOn atomic.Bool

func init() { externalAudioGCOn.Store(true) }

func runExternalAudio(ctx context.Context, exePath string, args []string) ([]byte, error) {
	if !externalAudioGCOn.Load() {
		return runExternalAudioPlain(ctx, exePath, args)
	}
	cmd := exec.CommandContext(ctx, exePath, args...)
	cmd.Env = append(os.Environ(), externalAudioEnv...)
	return cmd.CombinedOutput()
}

func runExternalAudioPlain(ctx context.Context, exePath string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, exePath, args...).CombinedOutput()
}

var externalAudioSentRe = regexp.MustCompile(`sent (\d+)ms`)

// externalAudioProgress reads a failed ExternalAudio run's output: whether it
// had started sending audio (a crash mid-play, worth resending the rest) and
// how many ms it reported sent. A failure before "Sending Audio" (no SRS, bad
// file) is not resent.
func externalAudioProgress(out string) (sentMs int, midPlay bool) {
	if !strings.Contains(out, "Sending Audio") {
		return 0, false
	}
	if m := externalAudioSentRe.FindAllStringSubmatch(out, -1); len(m) > 0 {
		sentMs, _ = strconv.Atoi(m[len(m)-1][1])
	}
	return sentMs, true
}

// cutMP3From writes the part of the MP3 at path from fromMs on to a new temp
// file. "" and no error when less than half a second is left.
func cutMP3From(ffmpegPath, path string, fromMs int) (string, error) {
	tmp, err := os.CreateTemp("", "atc-tx-rest-*.mp3")
	if err != nil {
		return "", err
	}
	tmp.Close()
	out := tmp.Name()
	cmd := exec.Command(ffmpegPath, "-y", "-ss", fmt.Sprintf("%.3f", float64(fromMs)/1000), "-i", path,
		"-c:a", "libmp3lame", "-ar", strconv.Itoa(srsAudioRateHz), "-b:a", "64k", out)
	if o, err := cmd.CombinedOutput(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("ffmpeg: %w: %s", err, string(o))
	}
	if b, err := os.ReadFile(out); err == nil {
		if d, err := mp3DurationSec(ffmpegPath, b); err == nil && d < 0.5 {
			os.Remove(out)
			return "", nil
		}
	}
	return out, nil
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
		"--modulations", srsModulationName(freqMHz),
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
	// Hull/airframe name (ACMI Name=), kept separately from the objects map:
	// that one prefers Group=, which the whole strike group shares, so it can't
	// tell CVN_72 from LHA_Tarawa. findCarrierContact needs the real name.
	objectNames := make(map[string]string)
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
				// The pipe is also the only human-vs-AI signal Tacview gives,
				// which the unknown-traffic call relies on.
				if i := strings.Index(p, "|"); i >= 0 {
					p = strings.TrimSpace(p[:i])
					atcCtrl.MarkPlayer(p)
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
			if n := extractACMIProp(props, "Name"); n != "" {
				objectNames[id] = n
			}
			// First-seen diagnostic: dump the full ACMI prop line so we can
			// confirm which fields DCS is actually exporting (modex vs player).
			if !wasKnown && objects[id] != "" {
				// Air-only: ATC only ever does callsign lookups for aircraft.
				// Without this filter the log is drowned by every ground
				// vehicle, ship, and static in the mission (200+ entries).
				// Flat-tops are logged too — when Marshal calls a BRC from the
				// wrong hull, this line is what identifies the right one.
				if t := extractACMIProp(props, "Type"); strings.HasPrefix(t, "Air") ||
					strings.Contains(t, "AircraftCarrier") {
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

			// Feed into ATC state — aircraft plus flat-tops; everything else
			// (escorts, ground units, statics) is filtered out. The carrier has
			// to be let through for Marshal/Deckboss BRC and range calls.
			// NOTE: this used to read strings.Contains(objType, "Air"), which
			// admitted carriers only by accident — "Sea+Watercraft+Aircraft-
			// Carrier" happens to contain "Air". Spelled out so the carrier
			// path can't be lost to a well-meaning tightening of the filter.
			if callsign, ok := objects[id]; ok {
				objType := objectTypes[id]
				wanted := objType == "" || strings.HasPrefix(objType, "Air") ||
					strings.Contains(objType, "AircraftCarrier")
				if wanted {
					if pos, ok := positions[id]; ok && pos.hasPos {
						atcCtrl.UpdateAnyPosition(callsign, objectNames[id], objType, pos.lon, pos.lat, pos.altFt, pos.speedKts, pos.headingDeg, pos.vertSpeedFpm)
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