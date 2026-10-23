// Package ffmpegpath finds ffmpeg.exe. atc.exe needs it to turn the pilots'
// SRS audio into the WAV that speech recognition reads (and for the radio
// effect); without it no call is understood. The vSFG-7 rigs keep it at
// Default; a customer PC may have it from winget, on PATH or beside
// Readback.
package ffmpegpath

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// Default is atc.exe's --ffmpeg default: where the rigs unzipped it.
const Default = `C:\ffmpeg-master-latest-win64-gpl\bin\ffmpeg.exe`

// WingetID is the winget package the installer offers ("winget install
// --id Gyan.FFmpeg -e").
const WingetID = "Gyan.FFmpeg"

// Find returns the first ffmpeg.exe found, or "": Default, beside the
// program in dir (ffmpeg.exe or ffmpeg\bin\ffmpeg.exe), winget's links and
// package folders (found even before a new PATH reaches this process), then
// PATH.
func Find(dir string) string {
	try := []string{Default}
	if dir != "" {
		try = append(try, filepath.Join(dir, "ffmpeg.exe"), filepath.Join(dir, "ffmpeg", "bin", "ffmpeg.exe"))
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		try = append(try, filepath.Join(la, "Microsoft", "WinGet", "Links", "ffmpeg.exe"))
		m, _ := filepath.Glob(filepath.Join(la, "Microsoft", "WinGet", "Packages", WingetID+"*", "ffmpeg-*", "bin", "ffmpeg.exe"))
		sort.Sort(sort.Reverse(sort.StringSlice(m))) // newest version first
		try = append(try, m...)
	}
	for _, p := range try {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	return ""
}
