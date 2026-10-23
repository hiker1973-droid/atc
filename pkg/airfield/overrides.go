package airfield

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Frequency overrides: the launcher's Settings page (operator 2026-10-23:
// "where to enter keys for open ai and eleven labs. setting your frequncys")
// writes frequencies.json next to atc.exe. Only the values an operator changed
// are stored; a field not listed keeps the frequencies compiled into its
// definition. atc.exe applies the file once at startup, so a change takes
// effect when the roles restart.

// FreqOverride is one field's changed frequencies in MHz; zero means "keep
// the built-in value".
type FreqOverride struct {
	Tower   float64 `json:"tower,omitempty"`
	ATIS    float64 `json:"atis,omitempty"`
	Handoff float64 `json:"handoff,omitempty"`
}

// FreqOverrides is frequencies.json: ICAO -> changed frequencies.
type FreqOverrides struct {
	Fields map[string]FreqOverride `json:"fields"`
}

// ValidFreqMHz accepts the SRS radio range: 30-88 FM, 118-137 VHF AM and
// 225-400 UHF all fall inside it.
func ValidFreqMHz(f float64) bool { return f >= 30 && f <= 400 }

// ReadFreqOverrides reads frequencies.json. A missing file is no overrides.
func ReadFreqOverrides(path string) (FreqOverrides, error) {
	o := FreqOverrides{Fields: map[string]FreqOverride{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return FreqOverrides{Fields: map[string]FreqOverride{}}, err
	}
	upper := make(map[string]FreqOverride, len(o.Fields))
	for icao, ov := range o.Fields {
		upper[strings.ToUpper(strings.TrimSpace(icao))] = ov
	}
	o.Fields = upper
	return o, nil
}

// Apply sets the overridden frequencies on the registry entries, so every
// mode sees them: a tower's own frequency, the ATIS it names, and Command's
// recoveries. An approach frequency that was the tower's follows it. Unknown
// ICAOs and out-of-range values are skipped. Returns the ICAOs changed.
func (o FreqOverrides) Apply() []string {
	var changed []string
	for icao, ov := range o.Fields {
		af := ByICAO(icao)
		if af == nil {
			continue
		}
		did := false
		if ValidFreqMHz(ov.Tower) {
			if af.ApproachFreqMHz == af.TowerFreqMHz {
				af.ApproachFreqMHz = ov.Tower
			}
			af.TowerFreqMHz = ov.Tower
			did = true
		}
		if ValidFreqMHz(ov.ATIS) {
			af.ATISFreqMHz = ov.ATIS
			did = true
		}
		if ValidFreqMHz(ov.Handoff) {
			af.HandoffFreqMHz = ov.Handoff
			did = true
		}
		if did {
			changed = append(changed, strings.ToUpper(icao))
		}
	}
	sort.Strings(changed)
	return changed
}
