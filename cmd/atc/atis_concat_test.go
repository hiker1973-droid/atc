package main

import (
	"strings"
	"testing"
)

// A bad format verb here does not fail loudly -- ffmpeg rejects the filter,
// concatMP3WithSilence returns an error, and the station drops to English-only
// with one warn line. So assert the rendered filter is actually well-formed.
func TestATISConcatFilterRenders(t *testing.T) {
	got := atisConcatFilter(1.2)

	if strings.Contains(got, "%!") {
		t.Fatalf("filter has an unrendered verb: %s", got)
	}
	if strings.Contains(got, "BADINDEX") {
		t.Fatalf("filter hit the indexed-verb trap: %s", got)
	}
	if !strings.Contains(got, "atrim=duration=1.200") {
		t.Errorf("silence duration not rendered: %s", got)
	}
	// Every stage must sit at the rate ExternalAudio wants, or it resamples.
	for _, want := range []string{
		"[0:a]aresample=16000",
		"[1:a]aresample=16000",
		"anullsrc=r=16000:cl=mono",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in filter: %s", want, got)
		}
	}
	if !strings.Contains(got, "concat=n=3:v=0:a=1[out]") {
		t.Errorf("concat stage missing: %s", got)
	}
}
