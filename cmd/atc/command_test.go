package main

import (
	"math"
	"strings"
	"testing"
)

// Whisper writes a spoken decimal as "5 point 2"; before the fix Sscanf read
// that as a bare 5, so Marshal and Command both misreported the state.
func TestExtractFuelStateMarshal(t *testing.T) {
	cases := []struct {
		text string
		want float64
	}{
		{"marshal, raider 39, state 5.2", 5.2},
		{"marshal, raider 39, state 5 point 2", 5.2},
		{"command, raider 11, fence out, state 1 point 8", 1.8},
		{"marshal, raider 39, state 6", 6},
		{"marshal, raider 39, marking mom", 0},
	}
	for _, tc := range cases {
		if got := extractFuelStateMarshal(tc.text); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("extractFuelStateMarshal(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

// commandResponse picks one of three variants at random, so each case runs
// enough times to see every variant and asserts on what all of them share.
func TestCommandResponseFuelState(t *testing.T) {
	const channel = "vSFG-7-Command"
	cases := []struct {
		name    string
		text    string
		wantAll []string // every variant must contain all of these
		wantAny []string // and at least one of these (nil = no check)
	}{
		{"fence out bingo", "Command, Raider 11, fence out, state 1.8", []string{channel, "state 1.8"}, []string{"bingo", "low state"}},
		{"fence out spoken decimal", "Command, Raider 11, fence out, state 1 point 8", []string{"state 1.8"}, []string{"bingo", "low state"}},
		{"fence out healthy state", "Command, Raider 11, fence out, state 6.0", []string{channel, "state 6.0", "fence out"}, nil},
		{"fence out no state unchanged", "Command, Raider 11, fence out", []string{channel, "fence out"}, nil},
		{"fence in with state", "Command, Raider 11, fence in, state 7.5", []string{channel, "state 7.5", "fence in"}, nil},
		{"fence in no state unchanged", "Command, Raider 11, fence in", []string{channel}, []string{"fence in", "fence check"}},
		{"state report", "Command, Raider 11, state 5.2", []string{channel, "state 5.2"}, nil},
		{"low state report", "Command, Raider 11, state 1.5", []string{"state 1.5"}, []string{"bingo", "low state"}},
		{"on station keeps its reply", "Command, Raider 11, on station, state 8.0", []string{channel, "good hunting"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < 30; i++ {
				got := commandResponse(tc.text, "Raider 11", channel)
				for _, w := range tc.wantAll {
					if !strings.Contains(got, w) {
						t.Fatalf("commandResponse(%q) = %q, missing %q", tc.text, got, w)
					}
				}
				if tc.wantAny != nil && !containsAny(got, tc.wantAny...) {
					t.Fatalf("commandResponse(%q) = %q, want one of %q", tc.text, got, tc.wantAny)
				}
			}
		})
	}

	if got := commandResponse("Command, Raider 11, say again", "Raider 11", channel); got != "" {
		t.Errorf("unrelated call should stay silent, got %q", got)
	}
}
