package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// The Settings page moves Mozdok's tower and ATIS: the registry and the
// ATIS station list both follow frequencies.json.
func TestFreqOverridesReachTowerAndATIS(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frequencies.json")
	if err := os.WriteFile(p, []byte(`{"fields":{"XRMF":{"tower":251.5,"atis":238.5}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	oldPath, oldOv, saved := freqsPath, freqOverrides, *airfield.XRMF
	freqsPath = func() string { return p }
	t.Cleanup(func() { freqsPath, freqOverrides, *airfield.XRMF = oldPath, oldOv, saved })

	applyFreqOverrides()
	if airfield.XRMF.TowerFreqMHz != 251.5 || airfield.XRMF.ATISFreqMHz != 238.5 {
		t.Fatalf("registry %+v", airfield.XRMF)
	}
	stations := atisStationsForMap("caucasus")
	applyATISFreqOverrides(stations)
	for _, st := range stations {
		switch st.ICAO {
		case "XRMF":
			if st.FreqMHz != 238.5 {
				t.Fatalf("Mozdok ATIS %v", st.FreqMHz)
			}
		case "UGSB":
			if st.FreqMHz != 230 {
				t.Fatalf("Batumi ATIS moved to %v", st.FreqMHz)
			}
		}
	}
}
