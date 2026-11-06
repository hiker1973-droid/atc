package main

import (
	"testing"
	"time"
)

func TestMarshalTrackSample(t *testing.T) {
	got := marshalTrackSample(time.Unix(1762400000, 0), 7.456, 262.4, 4012.7)
	if want := "1762400000:7.46:262:4013"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
