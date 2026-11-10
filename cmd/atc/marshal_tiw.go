package main

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/controller"
)

// Turn Into Wind BRC call (operator 2026-11-08). The mission's CarrierGUI Bridge
// writes "[CarrierGUI Bridge] TIW flag 320=N" to dcs.log when an into-wind period
// starts (N = 30/60/90 minutes) and again with 320=0 when it ends or is stopped.
// A while after a start - the ship is still swinging at first - Marshal says the
// BRC once to all aircraft, from Tacview's carrier heading like every other BRC.
const (
	tiwTick       = 5 * time.Second
	tiwSteadyDeg  = 2.0 // heading must move less than this between two samples
	tiwSteadyWait = 30 * time.Second
	tiwMaxExtra   = 6 // extra steady checks before the call is made anyway
)

var tiwLineRe = regexp.MustCompile(`\[CarrierGUI Bridge\] TIW flag 320=(\d+)`)

// tiwParse returns the minutes from a dcs.log line carrying the TIW flag.
func tiwParse(line string) (int, bool) {
	m := tiwLineRe.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// tiwLogPath is --tiw-log, else the newest dcs.log under a Saved Games\DCS*\Logs
// of this user or the Administrator account. "" when there is none.
func tiwLogPath() string {
	if flagTiwLog != "" {
		return flagTiwLog
	}
	var homes []string
	if h, err := os.UserHomeDir(); err == nil {
		homes = append(homes, h)
	}
	homes = append(homes, `C:\Users\Administrator`)
	var best string
	var bestT time.Time
	for _, h := range homes {
		m, _ := filepath.Glob(filepath.Join(h, "Saved Games", "DCS*", "Logs", "dcs.log"))
		sort.Strings(m)
		for _, p := range m {
			if st, err := os.Stat(p); err == nil && st.ModTime().After(bestT) {
				best, bestT = p, st.ModTime()
			}
		}
	}
	return best
}

// tiwTail returns the TIW minutes found in new lines since off, and the new offset.
// A file that has shrunk (DCS restarted) is read from its start.
func tiwTail(path string, off int64) ([]int, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, off
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, off
	}
	if st.Size() < off {
		off = 0
	}
	if st.Size() == off {
		return nil, off
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, off
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, off
	}
	// only whole lines: leave a half-written last line for the next tick
	end := len(buf)
	for end > 0 && buf[end-1] != '\n' {
		end--
	}
	var out []int
	start := 0
	for i := 0; i < end; i++ {
		if buf[i] == '\n' {
			if n, ok := tiwParse(string(buf[start:i])); ok {
				out = append(out, n)
			}
			start = i + 1
		}
	}
	return out, off + int64(end)
}

func angDiff(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// watchTurnIntoWind tails dcs.log and calls the BRC once per into-wind start.
func watchTurnIntoWind(ctx context.Context, comp *composer.ATCComposer, transmit func(string), atcCtrl *controller.ATCController) {
	if flagTiwDelay <= 0 {
		return
	}
	path := tiwLogPath()
	if path == "" {
		log.Info().Msg("Marshal: no dcs.log found, Turn Into Wind BRC call off")
		return
	}
	var off int64
	if st, err := os.Stat(path); err == nil {
		off = st.Size() // history is not replayed
	}
	log.Info().Str("log", path).Dur("delay", flagTiwDelay).Msg("Marshal: watching dcs.log for Turn Into Wind")
	var due time.Time // zero = no call pending
	extra := 0
	tk := time.NewTicker(tiwTick)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}
		var mins []int
		mins, off = tiwTail(path, off)
		for _, n := range mins {
			if n > 0 && due.IsZero() {
				due, extra = time.Now().Add(flagTiwDelay), 0
				log.Info().Int("minutes", n).Time("callAt", due).Msg("Marshal: Turn Into Wind started, BRC call scheduled")
				transmit(comp.MarshalTurningIntoWind(n))
			} else if n == 0 && !due.IsZero() {
				due = time.Time{}
				log.Info().Msg("Marshal: Turn Into Wind ended before the BRC call, cancelled")
			}
		}
		if due.IsZero() || time.Now().Before(due) {
			continue
		}
		a := atcCtrl.GetCarrierBRC()
		if a < 0 {
			if extra++; extra > tiwMaxExtra {
				log.Warn().Msg("Marshal: Turn Into Wind BRC call dropped, carrier not on Tacview")
				due = time.Time{}
			} else {
				due = time.Now().Add(tiwSteadyWait)
			}
			continue
		}
		// still turning: look again in 30 s, then call anyway after a few looks
		select {
		case <-ctx.Done():
			return
		case <-time.After(tiwSteadyWait):
		}
		b := atcCtrl.GetCarrierBRC()
		if b < 0 || (angDiff(a, b) > tiwSteadyDeg && extra < tiwMaxExtra) {
			extra++
			due = time.Now()
			continue
		}
		due = time.Time{}
		brc := atcCtrl.ToMagnetic(b)
		log.Info().Float64("brcMag", brc).Msg("Marshal: Turn Into Wind BRC call")
		transmit(comp.MarshalRecoveryBRC(brc))
	}
}
