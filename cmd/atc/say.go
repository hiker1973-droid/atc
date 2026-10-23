package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

// The launcher Setup page's test call (operator 2026-10-23: "start on the
// setup wizard"): `atc.exe --say "<line>" --freq 251.0` speaks one line on
// SRS and exits, 0 when ExternalAudio took it and 1 with the reason when not.
// It proves the keys, the voice service, ExternalAudio and the SRS server in
// one go, which is what a new install needs to know before starting roles.

var (
	flagSay     string
	flagSayName string
)

func registerSayFlags(f *pflag.FlagSet) {
	f.StringVar(&flagSay, "say", "", "Setup test call: speak this line once on --freq and exit (the launcher's Setup page)")
	f.StringVar(&flagSayName, "say-name", "Readback-Setup", "SRS client name for --say (no spaces)")
}

// sayOnce speaks flagSay on --freq in the --tts-voice / --elevenlabs-tower-voice
// voice. The success line is for the launcher, which reads this process's output.
func sayOnce(apiKey string) error {
	if flagFreqMHz < 30 || flagFreqMHz > 400 {
		return fmt.Errorf("--say needs --freq between 30 and 400 MHz")
	}
	_, port, err := net.SplitHostPort(flagSRSAddr)
	if err != nil {
		return fmt.Errorf("SRS address %q is not host:port", flagSRSAddr)
	}
	if _, err := os.Stat(flagExternalAudio); err != nil {
		return fmt.Errorf("SRS ExternalAudio not found at %s - is DCS-SRS installed on this PC?", flagExternalAudio)
	}
	v := voiceProfile{voice: flagTTSVoice, speed: flagTTSSpeed}
	if flagElevenTowerVoice != "" {
		v.eleven = elTower(flagElevenTowerVoice, "Setup")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	mp3, src, err := synthesizeSpeechFrom(ctx, apiKey, flagSay, v)
	if err != nil {
		return fmt.Errorf("text-to-speech failed (%s): %w", src.Service, err)
	}
	// The audio, or with the Windows voices the line itself (tts_windows.go).
	var speak []string
	if c, ok := readWinClip(mp3); ok {
		speak = winClipArgs(c)
	} else {
		tmp, err := os.CreateTemp("", "atc-say-*.mp3")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.Write(mp3); err != nil {
			tmp.Close()
			return err
		}
		tmp.Close()
		txPath, cleanup := prepareTX(tmp.Name(), flagSayName)
		defer cleanup()
		speak = []string{"--file", txPath}
	}
	args := append(speak,
		"--freqs", fmt.Sprintf("%.3f", flagFreqMHz),
		"--modulations", srsModulationName(flagFreqMHz),
		"-c", srsCoalitionArg(),
		"-n", strings.ReplaceAll(flagSayName, " ", "-"),
		"-p", port,
		"-v", "1",
	)
	out, err := runExternalAudio(ctx, flagExternalAudio, args)
	if err != nil {
		// SRS 2.4's ExternalAudio is a .NET 10 program; on a PC without the
		// runtime it exits 0x80008096 with only a download link (dev box,
		// 2026-10-23, which had .NET 8). The SRS installer is .NET 10 too and
		// will not even open without it, so the runtime comes first.
		if strings.Contains(string(out), "dotnet-core-applaunch") {
			return fmt.Errorf("SRS ExternalAudio needs Microsoft .NET 10 on this PC: install the .NET 10 Desktop Runtime (x64) from dotnet.microsoft.com, then the DCS-SRS installer")
		}
		return fmt.Errorf("SRS did not take the transmission (%v): %s", err, lastLine(string(out)))
	}
	log.Info().Float64("freqMHz", flagFreqMHz).Str("tts", src.Service).Str("voice", src.Voice).Msg("setup test call sent")
	fmt.Printf("SAY-OK %s %s\n", src.Service, src.Voice)
	return nil
}

// lastLine is the last non-empty line of a program's output, for an error.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return "no output"
}
