package main

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Frequency overrides from the launcher's Settings page: frequencies.json
// next to atc.exe (pkg/airfield/overrides.go). Read once at startup.

var (
	freqOverrides = airfield.FreqOverrides{Fields: map[string]airfield.FreqOverride{}}
	freqsPath     = func() string {
		exe, err := os.Executable()
		if err != nil {
			return "frequencies.json"
		}
		return filepath.Join(filepath.Dir(exe), "frequencies.json")
	}
)

// applyFreqOverrides puts frequencies.json onto the airfield registry. An
// unreadable file is logged and ignored: the built-in frequencies stay.
func applyFreqOverrides() {
	p := freqsPath()
	o, err := airfield.ReadFreqOverrides(p)
	if err != nil {
		log.Warn().Err(err).Str("file", p).Msg("frequencies.json unreadable - built-in frequencies kept")
		return
	}
	freqOverrides = o
	if changed := o.Apply(); len(changed) > 0 {
		log.Info().Strs("fields", changed).Str("file", p).Msg("frequency overrides applied")
	}
}

// applyATISFreqOverrides moves the ATIS stations whose field has an ATIS
// override; the station list keeps its own copy of each frequency.
func applyATISFreqOverrides(stations []*atisStation) {
	for _, st := range stations {
		if v := freqOverrides.Fields[st.ICAO].ATIS; airfield.ValidFreqMHz(v) {
			st.FreqMHz = v
		}
	}
}
