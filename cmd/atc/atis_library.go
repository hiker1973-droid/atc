package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// The ATIS recording library (operator 2026-10-27: "have ATIS not change that
// much as its just a replay. can you prerecord?"). ATIS was the OpenAI bill:
// 386 recordings a day across the three rigs, every one a whole read in two
// languages, re-made on each runway change, weather change and restart. The
// words only depend on the station, weather and runway, which come round
// again - the runway rotation flips between two ends, a mission restart brings
// back its weather - so each read is kept, under a hash of its words with the
// information letter left out, and replayed (with the letter it was recorded
// with) whenever the same words come up. Only words never heard before are
// recorded.

const (
	atisLibraryVersion = "1" // bump to re-record everything (a new voice or read)
	atisLibraryKeep    = 80  // recordings kept per station, the least recently used go
)

func atisLibraryDir() string { return filepath.Join(installDir(), "atis_cache", "library") }

// atisLibraryKey is a hash of what the station would say, less its letter,
// and how it would sound: voices, styles and the second language.
func atisLibraryKey(station *atisStation, state *atisState, secondLang string, wantSecond bool) string {
	if !wantSecond {
		secondLang = ""
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n%s\n%+v\n%+v\n", atisLibraryVersion, buildATISTextIdent(station, state, "{ident}"),
		secondLang, atisEnglishVoice(station), atisVoiceFor(station))
	return hex.EncodeToString(h.Sum(nil))[:20]
}

// atisLibraryGet: the recording for key, and the letter (0-25) it says.
func atisLibraryGet(icao, key string) ([]byte, int, bool) {
	if key == "" {
		return nil, 0, false
	}
	matches, _ := filepath.Glob(filepath.Join(atisLibraryDir(), icao+"_"+key+"_*.mp3"))
	for _, p := range matches {
		base := strings.TrimSuffix(filepath.Base(p), ".mp3")
		ident, err := strconv.Atoi(base[strings.LastIndexByte(base, '_')+1:])
		if err != nil || ident < 0 || ident > 25 {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			continue
		}
		now := time.Now()
		_ = os.Chtimes(p, now, now) // recently used: kept by the trim
		return b, ident, true
	}
	return nil, 0, false
}

// atisLibraryPut keeps a new recording and trims the station's shelf.
func atisLibraryPut(icao, key string, ident int, mp3 []byte) {
	if key == "" || len(mp3) == 0 {
		return
	}
	dir := atisLibraryDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	p := filepath.Join(dir, fmt.Sprintf("%s_%s_%d.mp3", icao, key, ident))
	if err := os.WriteFile(p, mp3, 0644); err != nil {
		log.Warn().Err(err).Str("icao", icao).Msg("ATIS library: could not keep the recording")
		return
	}
	all, _ := filepath.Glob(filepath.Join(dir, icao+"_*.mp3"))
	if len(all) <= atisLibraryKeep {
		return
	}
	type f struct {
		p string
		t time.Time
	}
	var fs []f
	for _, q := range all {
		if st, err := os.Stat(q); err == nil {
			fs = append(fs, f{q, st.ModTime()})
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].t.After(fs[j].t) })
	for _, old := range fs[min(len(fs), atisLibraryKeep):] {
		os.Remove(old.p)
	}
}
