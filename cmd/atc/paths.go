package main

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/vsfg7/atc/pkg/ffmpegpath"
)

// installDir is the folder atc.exe runs from: C:\SkyeyeATC on the vSFG-7
// rigs, wherever SkyEye-Setup put it on another PC. Logs, the ATIS cache and
// the mission-watch file live beside it, as tts_cache and the settings files
// already did.
func installDir() string {
	exe, err := os.Executable()
	if err != nil {
		return `C:\SkyeyeATC`
	}
	return filepath.Dir(exe)
}

// flagSquadronRange (--squadron-range): Command knows the vSFG-7 Caucasus
// training range's targets and areas (command_tasking.go, command_areas.go).
// On unless a generated start file says otherwise; true here too, so the
// tests (which never register flags) see the range as the rigs do.
var flagSquadronRange = true

// resolveFFmpeg: with --ffmpeg left at its default and nothing there (a
// customer PC), use the ffmpeg found elsewhere (winget, PATH, beside
// atc.exe). ffmpeg turns the pilots' audio into what speech recognition
// reads, so without one no call is understood: say so loudly.
func resolveFFmpeg(fs *pflag.FlagSet) {
	if fs.Changed("ffmpeg") {
		return
	}
	if _, err := os.Stat(flagFFmpeg); err == nil {
		return
	}
	if p := ffmpegpath.Find(installDir()); p != "" {
		flagFFmpeg = p
		log.Info().Str("ffmpeg", p).Msg("using ffmpeg")
		return
	}
	log.Error().Str("looked", ffmpegpath.Default).
		Msg("ffmpeg NOT FOUND: pilots' calls cannot be understood until it is installed (winget install --id Gyan.FFmpeg -e)")
}
