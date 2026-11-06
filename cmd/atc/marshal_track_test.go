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

func TestFollowAfterCommence(t *testing.T) {
	now := time.Unix(1762400000, 0)
	followAfterCommence("Raider 302", now)
	if got := trackedCallsigns([]string{"Raider 301"}, now.Add(5*time.Minute)); !got["Raider 302"] || !got["Raider 301"] {
		t.Fatalf("a commenced jet should be followed with the stack, got %v", got)
	}
	if got := trackedCallsigns(nil, now.Add(marshalTrackAfter+time.Second)); got["Raider 302"] {
		t.Fatalf("a commenced jet should stop being followed after %s, got %v", marshalTrackAfter, got)
	}
}
