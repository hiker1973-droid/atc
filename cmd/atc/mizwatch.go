package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/miz"
)

// Live weather from whichever mission the DCS server has loaded.
//
// Operator ask 2026-09-19: Foothold rotates through ten weather versions, and a
// role reads its .miz weather once at boot (SKYEYE_MIZ / --miz-path), so after a
// rotation SkyEye kept announcing the first mission's wind, ceiling and
// altimeter. tools/dcs-hooks/vsfg7-current-mission.lua, installed in the DCS
// server's Scripts\Hooks, writes the loaded mission's path to --miz-watch-file on
// every mission load. Each role polls that file and, when it names a different
// mission, re-reads that .miz and applies its weather live -- no restart.
//
// ATIS follows on its own: it syncs weather from its paired tower before every
// broadcast and regenerates its audio when the weather changes.
//
// No file (hook not installed, or a rig that doesn't host the server) is not an
// error: the boot weather stays, exactly as before.

const mizWatchInterval = 30 * time.Second

// readMissionPointer returns the mission path the hook wrote, or "" when the
// file is missing, empty, or does not name an existing .miz. DCS may write a
// UTF-8 BOM and a trailing newline; both are tolerated.
func readMissionPointer(pointerFile string) string {
	b, err := os.ReadFile(pointerFile)
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(strings.TrimPrefix(string(b), "\uFEFF"))
	if p == "" || !strings.EqualFold(filepath.Ext(p), ".miz") {
		return ""
	}
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// mizWatcher applies a mission's weather whenever the pointer file names a new one.
type mizWatcher struct {
	pointerFile string
	current     string // mission whose weather is applied now ("" = boot weather)
	read        func(string) (miz.Weather, error)
	apply       func(miz.Weather)
}

// step checks the pointer once and reports whether new weather was applied.
func (w *mizWatcher) step() bool {
	p := readMissionPointer(w.pointerFile)
	if p == "" || strings.EqualFold(p, w.current) {
		return false
	}
	wx, err := w.read(p)
	if err != nil {
		log.Warn().Err(err).Str("miz", p).Msg("mission changed but its weather could not be read — keeping current weather")
		w.current = p // don't retry a broken file every poll
		return false
	}
	w.apply(wx)
	log.Info().
		Str("miz", filepath.Base(p)).
		Float64("windDirTrue", wx.WindDirTrue).
		Float64("windKts", wx.WindKts).
		Float64("ceilFt", wx.CeilFt).
		Float64("visNm", wx.VisNm).
		Float64("altInHg", wx.AltInHg).
		Msg("Weather reloaded — DCS server loaded a different mission")
	w.current = p
	return true
}

// runMizWatch polls until ctx ends. bootMiz is the mission the boot weather came
// from, so a pointer that already names it does not trigger a redundant reload.
func runMizWatch(ctx context.Context, w *mizWatcher, bootMiz string) {
	if w.pointerFile == "" {
		return
	}
	w.current = bootMiz
	w.step() // pick up the loaded mission immediately, not 30 s after boot
	t := time.NewTicker(mizWatchInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.step()
		}
	}
}
