package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/martinlindhe/unit"
	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"

	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/coalitions"
	"github.com/vsfg7/atc/pkg/gci/composer"
	"github.com/vsfg7/atc/pkg/gci/controller"
	"github.com/vsfg7/atc/pkg/gci/parser"
	"github.com/vsfg7/atc/pkg/gci/radar"
	"github.com/vsfg7/atc/pkg/gci/sim"
	"github.com/vsfg7/atc/pkg/gci/srsinfo"
	"github.com/vsfg7/atc/pkg/gci/telemetry"
	"github.com/vsfg7/atc/pkg/gci/traces"
)

// GCI: Darkstar (Day, Night 264.000) and Overlord (Foothold 266.000), the
// AWACS controller the rigs ran as SkyEye's skyeye.exe (operator 2026-10-21:
// "We use this but I would like to rebuild with our own voice ... build into
// our ATC").
//
// The brain is SkyEye's own (pkg/gci, vendored, MIT): Tacview telemetry,
// radar and trackfiles, the brevity parser, the controller (PICTURE, BOGEY
// DOPE, DECLARE, SNAPLOCK, SPIKED, STROBE, VECTOR, ALPHA CHECK, TRIPWIRE,
// SHOPPING, automatic PICTURE, THREAT, MERGED, FADED) and the composer. The
// ears and mouth are ours: voiceNetLoop hears the frequency and has Whisper
// transcribe it (with the SRS-name callsign fix and the prompt-echo retry),
// and every call is spoken with our ElevenLabs / OpenAI voices, radio chain
// and ExternalAudio. SkyEye's whisper.cpp, Piper and SRS client need cgo, so
// atc.exe would no longer build on the rigs with them.

var (
	flagGCIOnly            bool
	flagGCIFreq            float64
	flagGCICallsign        string
	flagGCIVoice           string
	flagElevenGCIVoice     string
	flagGCICoalition       string
	flagGCIAutoPicture     bool
	flagGCIPictureInterval time.Duration
	flagGCIThreats         bool
	flagGCIThreatInterval  time.Duration
	flagGCIThreatRadiusNM  float64
	flagGCITacviewPassword string
	flagGCISampleDir       string
	flagGCISampleOnly      string
)

// registerGCIFlags is called from the flag block in main.go.
func registerGCIFlags(f *flag.FlagSet) {
	f.BoolVar(&flagGCIOnly, "gci-only", false, "Run as the GCI (AWACS) controller only - SkyEye's brain, our voice")
	f.Float64Var(&flagGCIFreq, "gci-freq", 264.0, "GCI frequency in MHz (AM)")
	f.StringVar(&flagGCICallsign, "gci-callsign", "Darkstar", "GCI callsign, spoken and answered (Darkstar on Day/Night, Overlord on Foothold)")
	f.StringVar(&flagGCIVoice, "gci-voice", "shimmer", "GCI OpenAI TTS voice (the fallback when ElevenLabs is off)")
	// Operator 2026-10-21: "change to a British female" (was Spartan, calm American baritone).
	f.StringVar(&flagElevenGCIVoice, "elevenlabs-gci-voice", elLily,
		`ElevenLabs voice ID for the GCI (default "Lily", confident British woman; "" = OpenAI only)`)
	f.StringVar(&flagGCICoalition, "gci-coalition", "blue", "Coalition the GCI serves: blue or red")
	f.BoolVar(&flagGCIAutoPicture, "gci-auto-picture", true, "Broadcast a PICTURE on a timer while pilots are on frequency")
	f.DurationVar(&flagGCIPictureInterval, "gci-picture-interval", 5*time.Minute,
		"Automatic PICTURE interval (SkyEye's default is 2m; 5m keeps ElevenLabs credits down)")
	f.BoolVar(&flagGCIThreats, "gci-threats", true, "Call THREATs to friendlies on frequency")
	f.DurationVar(&flagGCIThreatInterval, "gci-threat-interval", 3*time.Minute, "Cooldown between THREAT calls on the same group")
	f.Float64Var(&flagGCIThreatRadiusNM, "gci-threat-radius", 25, "Briefed radius for mandatory THREAT calls, nm")
	f.StringVar(&flagGCITacviewPassword, "gci-tacview-password", "", "Tacview real-time telemetry password, if the server sets one")
	f.StringVar(&flagGCISampleDir, "gci-sample", "", "Render sample GCI calls in the GCI voice to this folder and exit (gci_sample.go)")
	f.StringVar(&flagGCISampleOnly, "gci-sample-only", "", `With --gci-sample: render just this call ("picture", "threat", ...)`)
}

// styleGCI is the OpenAI fallback's delivery brief.
const styleGCI = "Speak as an AWACS weapons controller on a tactical net, a British woman with a clear Received Pronunciation accent. " +
	"Calm, clear and precise, unhurried even when the picture is busy. " +
	"Read numbers crisply and run grouped numbers together as one phrase."

// gciVoice: Lily on Flash (GCI talks far more than ATC - automatic
// pictures and threat calls - so the cheaper model).
func gciVoice() voiceProfile {
	return voiceProfile{voice: flagGCIVoice, speed: flagTTSSpeed * gciSpeedUp, instructions: styleGCI,
		eleven: elevenVoice{ID: flagElevenGCIVoice, Name: flagGCICallsign, Stability: 0.55, Style: 0.25,
			Speed: gciBaseSpeed, Tempo: gciSpeedUp, Model: elFlash}}
}

// gciSpeedUp: operator 2026-10-21 "speed up 30%" on the Lily sample, which
// was ElevenLabs speed gciBaseSpeed. ElevenLabs stops at 1.2 and its speed is
// not linear (1.05 -> 1.2 plus a 1.14 stretch came out 1.7x faster), so the
// delivery stays the one heard at gciBaseSpeed and is time-stretched exactly
// gciSpeedUp (Tempo, pitch kept).
const (
	gciBaseSpeed = 1.05
	gciSpeedUp   = 1.20 // 1.30, then "back it off 5-10%" (operator 2026-10-21)
)

// gciSRSBook is the SRS client list the GCI's controller and radar read
// (who is on frequency). voiceNetLoop feeds it from its SRS connection.
var gciSRSBook *srsinfo.Book

// gciWhisperPrompt is SkyEye's own transcription prompt (pkg/recognizer).
func gciWhisperPrompt(callsign string) string {
	return "Either ANYFACE or " + callsign + ", PILOT CALLSIGN, DIGITS, one of 'RADIO' 'ALPHA' 'BOGEY' " +
		"'PICTURE' 'DECLARE' 'SNAPLOCK' 'SPIKED', ARGUMENTS such as BULLSEYE, BRAA, numbers or digits."
}

func runGCI(ctx context.Context, apiKey string) {
	if flagTacviewAddr == "" {
		log.Error().Msg("GCI needs --tacview-addr: the radar picture comes from Tacview real-time telemetry")
		return
	}
	var coalition coalitions.Coalition = coalitions.Blue
	if flagGCICoalition == "red" {
		coalition = coalitions.Red
	}
	cs := flagGCICallsign
	log.Info().Str("callsign", cs).Float64("freq", flagGCIFreq).Str("tacview", flagTacviewAddr).
		Dur("autoPicture", flagGCIPictureInterval).Bool("threats", flagGCIThreats).Msg("GCI starting")

	starts, updates, fades := make(chan sim.Started), make(chan sim.Updated), make(chan sim.Faded)
	tel := telemetry.NewRealTimeClient(flagTacviewAddr, cs, flagGCITacviewPassword, 10*time.Second, 2*time.Second)
	gciSRSBook = srsinfo.NewBook(cs, flagGCIFreq*1e6)
	rdr := radar.New(coalition, starts, updates, fades,
		unit.Length(flagGCIThreatRadiusNM)*unit.NauticalMile, 5*unit.Degree, 1*unit.NauticalMile, false, gciSRSBook)
	ctrl := controller.New(rdr, gciSRSBook, coalition, flagGCIAutoPicture, flagGCIPictureInterval,
		flagGCIThreats, flagGCIThreatInterval, true, nil)
	prs := parser.New(cs, nil, true)
	comp := composer.Composer{Callsign: cs}

	var wg sync.WaitGroup
	wg.Go(func() {
		if err := tel.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error().Err(err).Msg("GCI: telemetry client stopped")
		}
	})
	wg.Go(func() { tel.Stream(ctx, &wg, starts, updates, fades) })
	wg.Go(func() {
		tk := time.NewTicker(2*time.Second + 100*time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				rdr.SetMissionTime(tel.Time())
				for _, c := range []coalitions.Coalition{coalitions.Red, coalitions.Blue} {
					if b, err := tel.Bullseye(c); err == nil {
						rdr.SetBullseye(b, c)
					}
				}
			}
		}
	})
	wg.Go(func() { rdr.Run(ctx, &wg) })

	calls := make(chan controller.Call)
	wg.Go(func() { ctrl.Run(ctx, calls) })
	wg.Go(func() { gciSpeak(ctx, apiKey, comp, calls) })

	requests := make(chan any, 8)
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case r := <-requests:
				gciRoute(ctrl, r)
			}
		}
	})

	voiceNetLoop(ctx, flagSRSAddr, flagGCIFreq, cs, apiKey, flagEAMPassword, gciVoice(), flagExternalAudio,
		gciWhisperPrompt(cs), nil,
		func(text string) (string, string) {
			req := prs.Parse(text)
			if req == nil {
				log.Info().Str("text", text).Msg("GCI: not addressed to us or not understood - silent")
				return "", ""
			}
			log.Info().Str("text", text).Any("request", req).Msg("GCI request")
			select {
			case requests <- req:
			default:
				log.Warn().Str("text", text).Msg("GCI: request queue full - dropped")
			}
			return "", "" // the controller answers on its own, through gciSpeak
		})
	wg.Wait()
	log.Info().Msg("GCI offline")
}

// gciRoute hands a parsed request to its controller handler (SkyEye's
// application.handleRequest).
func gciRoute(c *controller.Controller, r any) {
	ctx := traces.NewRequestContext()
	switch req := r.(type) {
	case *brevity.AlphaCheckRequest:
		c.HandleAlphaCheck(ctx, req)
	case *brevity.BogeyDopeRequest:
		c.HandleBogeyDope(ctx, req)
	case *brevity.CheckInRequest:
		c.HandleCheckIn(ctx, req)
	case *brevity.DeclareRequest:
		c.HandleDeclare(ctx, req)
	case *brevity.PictureRequest:
		c.HandlePicture(ctx, req)
	case *brevity.RadioCheckRequest:
		c.HandleRadioCheck(ctx, req)
	case *brevity.ShoppingRequest:
		c.HandleShopping(ctx, req)
	case *brevity.SnaplockRequest:
		c.HandleSnaplock(ctx, req)
	case *brevity.SpikedRequest:
		c.HandleSpiked(ctx, req)
	case *brevity.StrobeRequest:
		c.HandleStrobe(ctx, req)
	case *brevity.TripwireRequest:
		c.HandleTripwire(ctx, req)
	case *brevity.VectorRequest:
		c.HandleVector(ctx, req)
	case *brevity.UnableToUnderstandRequest:
		c.HandleUnableToUnderstand(ctx, req)
	default:
		log.Error().Any("request", r).Msg("GCI: no handler for request")
	}
}

// gciCompose turns a controller call into speech (SkyEye's application.composeCall).
func gciCompose(comp composer.Composer, call any) (composer.NaturalLanguageResponse, bool) {
	switch c := call.(type) {
	case brevity.AlphaCheckResponse:
		return comp.ComposeAlphaCheckResponse(c), true
	case brevity.BogeyDopeResponse:
		return comp.ComposeBogeyDopeResponse(c), true
	case brevity.CheckInResponse:
		return gciCheckIn(comp, c), true
	case brevity.DeclareResponse:
		return comp.ComposeDeclareResponse(c), true
	case brevity.FadedCall:
		return comp.ComposeFadedCall(c), true
	case brevity.NegativeRadarContactResponse:
		return comp.ComposeNegativeRadarContactResponse(c), true
	case brevity.PictureResponse:
		return comp.ComposePictureResponse(c), true
	case brevity.RadioCheckResponse:
		return gciRadioCheck(comp, c), true
	case brevity.ShoppingResponse:
		return comp.ComposeShoppingResponse(c), true
	case brevity.SnaplockResponse:
		return comp.ComposeSnaplockResponse(c), true
	case brevity.SpikedResponseV2:
		return comp.ComposeSpikedResponse(c), true
	case brevity.StrobeResponse:
		return comp.ComposeStrobeResponse(c), true
	case brevity.TripwireResponse:
		return comp.ComposeTripwireResponse(c), true
	case brevity.VectorResponse:
		return comp.ComposeVectorResponse(c), true
	case brevity.SunriseCall:
		return comp.ComposeSunriseCall(c), true
	case brevity.ThreatCall:
		return comp.ComposeThreatCall(c), true
	case brevity.MergedCall:
		return comp.ComposeMergedCall(c), true
	case brevity.SayAgainResponse:
		return comp.ComposeSayAgainResponse(c), true
	}
	return composer.NaturalLanguageResponse{}, false
}

// gciRadioCheck answers a radio / comm check with "good copy" (operator
// 2026-10-21: "For Comms check response instead of Lima Charlie say good
// copy"). SkyEye's own picks at random from "5 by 5", "loud and clear",
// "Lima Charlie" and the like. Not on scope is still said - it tells the
// pilot the GCI cannot see them (wrong DCS name, or still on the ramp).
func gciRadioCheck(comp composer.Composer, c brevity.RadioCheckResponse) composer.NaturalLanguageResponse {
	reply := fmt.Sprintf("%s, %s, good copy.", strings.ToUpper(c.Callsign), strings.ToUpper(comp.Callsign))
	if !c.RadarContact {
		reply = fmt.Sprintf("%s, %s, good copy, but I don't see you on scope.", strings.ToUpper(c.Callsign), strings.ToUpper(comp.Callsign))
	}
	return composer.NaturalLanguageResponse{Subtitle: reply, Speech: reply}
}

// gciCheckIn answers a plain "checking in". SkyEye's own answer is a joke
// ("did you want a radio check or an alpha check? Or are you just trying to
// get my attention? In that case, I'm flattered."); operator 2026-10-21:
// replace it. The controller only sends this once the callsign is on the
// scope, so it is radar contact.
func gciCheckIn(comp composer.Composer, c brevity.CheckInResponse) composer.NaturalLanguageResponse {
	reply := fmt.Sprintf("%s, %s, radar contact.", strings.ToUpper(c.Callsign), strings.ToUpper(comp.Callsign))
	return composer.NaturalLanguageResponse{Subtitle: reply, Speech: reply}
}

// gciSpeak speaks the controller's calls one at a time, in order.
func gciSpeak(ctx context.Context, apiKey string, comp composer.Composer, calls <-chan controller.Call) {
	srsHost, srsPort, _ := net.SplitHostPort(flagSRSAddr)
	vp := gciVoice()
	for {
		select {
		case <-ctx.Done():
			return
		case call := <-calls:
			resp, ok := gciCompose(comp, call.Call)
			if !ok || resp.Speech == "" {
				log.Debug().Type("call", call.Call).Msg("GCI: nothing to say for call")
				continue
			}
			mp3, err := synthesizeSpeech(ctx, apiKey, resp.Speech, vp)
			if err != nil {
				log.Error().Err(err).Str("text", resp.Speech).Msg("GCI TTS failed")
				continue
			}
			log.Info().Type("call", call.Call).Str("text", resp.Subtitle).Float64("freq", flagGCIFreq).Msg("GCI TX")
			broadcastLog("tx", flagGCICallsign+" TX → "+resp.Subtitle)
			noteOwnTX(flagGCIFreq, resp.Speech)
			transmitExternalAudioFile(ctx, mp3, flagGCIFreq, flagGCICallsign, srsHost, srsPort, flagExternalAudio)
		}
	}
}
