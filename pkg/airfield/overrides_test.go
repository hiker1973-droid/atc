package airfield

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreqOverrides(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frequencies.json")
	if o, err := ReadFreqOverrides(p); err != nil || len(o.Fields) != 0 {
		t.Fatalf("missing file: %v %v", o, err)
	}
	if err := os.WriteFile(p, []byte(`{"fields":{"xrmf":{"tower":251.5,"atis":999},"NOPE":{"tower":250}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := ReadFreqOverrides(p)
	if err != nil {
		t.Fatal(err)
	}
	saved := *XRMF
	t.Cleanup(func() { *XRMF = saved })
	if got := o.Apply(); len(got) != 1 || got[0] != "XRMF" {
		t.Fatalf("changed %v", got)
	}
	if XRMF.TowerFreqMHz != 251.5 || XRMF.ApproachFreqMHz != 251.5 {
		t.Fatalf("tower %v approach %v", XRMF.TowerFreqMHz, XRMF.ApproachFreqMHz)
	}
	if XRMF.ATISFreqMHz != 237 || XRMF.HandoffFreqMHz != 282 {
		t.Fatalf("out-of-range ATIS applied or handoff changed: %+v", XRMF)
	}
}
