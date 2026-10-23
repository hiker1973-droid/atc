package main

import (
	"os"
	"path/filepath"
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
