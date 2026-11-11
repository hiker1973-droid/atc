package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// The CAS-LGB TIC training mission has its own nine-site target set with two
// 9-lines per site (cas_tic_targets.go). Every other Caucasus Training mission
// uses the five-site range in command_tasking.go. Which one applies follows the
// mission the DCS server has loaded (the hook's --miz-watch-file), re-read while
// the role runs because the mission rotates without the role restarting.

var casTICOn atomic.Bool

// casTICMissionName reports whether a .miz path is the CAS-LGB TIC mission.
func casTICMissionName(path string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(path)), "cas-lgb_tic")
}

// currentMissionIsCASTIC reads the loaded mission: the watch file first, then --miz-path.
func currentMissionIsCASTIC() bool {
	if p := readMissionPointer(flagMizWatchFile); p != "" {
		return casTICMissionName(p)
	}
	return flagMizPath != "" && casTICMissionName(flagMizPath)
}

// casTICLoaded is read when the board is first built.
func casTICLoaded() bool {
	on := currentMissionIsCASTIC()
	casTICOn.Store(on)
	return on
}

// watchTaskingSet swaps the Command board and each JTAC's sector board when the
// loaded mission changes between the CAS-LGB TIC set and the standard range.
func watchTaskingSet(ctx context.Context, board *taskingBoard) {
	tk := time.NewTicker(20 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}
		on := currentMissionIsCASTIC()
		if on == casTICOn.Load() {
			continue
		}
		casTICOn.Store(on)
		all := taskingTargetsForMap(flagMap)
		board.setTargets(all)
		for _, n := range board.jtacs {
			if n.board == nil {
				continue
			}
			ts := sectorTargets(all, n.Sector)
			if len(ts) == 0 {
				ts = all
			}
			n.board.setTargets(ts)
		}
		log.Info().Bool("casTIC", on).Int("targets", len(all)).Msg("JTAC: mission changed, target set swapped")
	}
}

// allTaskingIPs lists every IP name either set uses, for the Whisper prompt.
func allTaskingIPs() []string {
	seen := map[string]bool{}
	var out []string
	for _, set := range [][]taskingTarget{caucasusTaskingTargets, casTICTargets} {
		for _, t := range set {
			if !seen[t.IP] {
				seen[t.IP] = true
				out = append(out, t.IP)
			}
		}
	}
	return out
}

// egressLine ends a good BDA. After a phase 1 (RED force) pass the next call is
// the site's static target.
func (t taskingTarget) egressLine() string {
	if t.Phase == 1 {
		return "Egress " + t.Egress + ", report ready for the next 9-line."
	}
	return "Egress " + t.Egress + ", report ready for further tasking."
}

// otherTaskingSet is the set the loaded mission is not using; its IP names go in
// the Whisper prompt too, so a mission change needs no net restart.
func otherTaskingSet() []taskingTarget {
	if casTICOn.Load() {
		return caucasusTaskingTargets
	}
	return casTICTargets
}
