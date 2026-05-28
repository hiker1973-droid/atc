// Package miz reads DCS .miz files (zip archives containing a Lua-table
// 'mission' entry) and extracts the surface-level weather block. Targeted
// regex parsing, not a full Lua interpreter — the DCS weather block is small
// and stable. Falls back gracefully when fields are missing.
package miz

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Weather is the subset of .miz weather relevant to ATC: surface wind, QNH,
// visibility, ceiling, and temperature. Wind direction is FROM, in true
// degrees — the controller applies MagVar internally.
type Weather struct {
	WindDirTrue float64 // wind FROM, degrees true
	WindKts     float64
	CeilFt      float64 // 25000 sentinel when density==0 (no significant cloud)
	VisNm       float64
	AltInHg     float64
	TempC       float64
	SourcePath  string
}

var (
	qnhRE        = regexp.MustCompile(`\["qnh"\]\s*=\s*([\d.]+)`)
	groundWindRE = regexp.MustCompile(`(?s)\["atGround"\]\s*=\s*\{[^{}]*?\["speed"\]\s*=\s*([\d.]+)[^{}]*?\["dir"\]\s*=\s*(\d+)`)
	visRE        = regexp.MustCompile(`(?s)\["visibility"\]\s*=\s*\{[^{}]*?\["distance"\]\s*=\s*(\d+)`)
	cloudsBaseRE = regexp.MustCompile(`(?s)\["clouds"\]\s*=\s*\{[^{}]*?\["base"\]\s*=\s*(\d+)`)
	cloudsDenRE  = regexp.MustCompile(`(?s)\["clouds"\]\s*=\s*\{[^{}]*?\["density"\]\s*=\s*(\d+)`)
	tempRE       = regexp.MustCompile(`(?s)\["season"\]\s*=\s*\{[^{}]*?\["temperature"\]\s*=\s*([-\d.]+)`)
)

// FindNewestMiz returns the most-recently-modified .miz in dir, or an error
// if none exist or the directory is unreadable.
func FindNewestMiz(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var newestMtime int64
	var newestPath string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".miz" {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		if newestPath == "" || info.ModTime().UnixNano() > newestMtime {
			newestMtime = info.ModTime().UnixNano()
			newestPath = filepath.Join(dir, e.Name())
		}
	}
	if newestPath == "" {
		return "", fmt.Errorf("no .miz files in %s", dir)
	}
	return newestPath, nil
}

// ReadMizWeather opens the .miz at mizPath, extracts the embedded 'mission'
// file, and parses the weather block. Missing fields leave the corresponding
// Weather field at its zero value, except ceiling which gets the 25000ft
// sentinel meaning "no significant cloud".
func ReadMizWeather(mizPath string) (Weather, error) {
	w := Weather{SourcePath: mizPath, CeilFt: 25000}
	zr, err := zip.OpenReader(mizPath)
	if err != nil {
		return w, err
	}
	defer zr.Close()

	var missionFile *zip.File
	for _, f := range zr.File {
		if f.Name == "mission" {
			missionFile = f
			break
		}
	}
	if missionFile == nil {
		return w, fmt.Errorf("no 'mission' entry in %s", filepath.Base(mizPath))
	}
	rc, err := missionFile.Open()
	if err != nil {
		return w, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return w, err
	}
	s := string(data)

	if m := qnhRE.FindStringSubmatch(s); len(m) == 2 {
		qnh, _ := strconv.ParseFloat(m[1], 64)
		w.AltInHg = qnh * 0.0393701 // mmHg → inHg
	}
	if m := groundWindRE.FindStringSubmatch(s); len(m) == 3 {
		spd, _ := strconv.ParseFloat(m[1], 64)
		dirGoing, _ := strconv.ParseFloat(m[2], 64)
		w.WindKts = spd * 1.94384 // m/s → kt
		// DCS dir is the direction wind is BLOWING TOWARDS; we want FROM.
		w.WindDirTrue = float64((int(dirGoing)+180)%360)
	}
	if m := visRE.FindStringSubmatch(s); len(m) == 2 {
		dist, _ := strconv.ParseFloat(m[1], 64)
		w.VisNm = dist / 1852.0
		if w.VisNm > 50 {
			w.VisNm = 50
		}
	}
	// Cloud base is only a meaningful ceiling when density > 0. With
	// density=0 the layer is "preset only" — treat as no significant cloud
	// and leave the sentinel 25000ft.
	if m := cloudsDenRE.FindStringSubmatch(s); len(m) == 2 {
		den, _ := strconv.Atoi(m[1])
		if den > 0 {
			if mb := cloudsBaseRE.FindStringSubmatch(s); len(mb) == 2 {
				base, _ := strconv.ParseFloat(mb[1], 64)
				w.CeilFt = base * 3.28084 // m → ft
			}
		}
	}
	if m := tempRE.FindStringSubmatch(s); len(m) == 2 {
		t, _ := strconv.ParseFloat(m[1], 64)
		w.TempC = t
	}
	return w, nil
}
