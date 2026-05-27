package controller

import "testing"

func TestExtractCallsign(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		tower string
		want  string
	}{
		// Yesterday's three production failures (2026-05-26 OMDM window).
		{
			name:  "is_airborne_tail",
			text:  "Al Minhad Tower, Venom 2-0-0 is airborne.",
			tower: "Al Minhad Tower",
			want:  "Venom 2-0-0",
		},
		{
			name:  "declaring_emergency_with_this_is_filler",
			text:  "Al Minhad Tower, this is Viper 27 declaring in-flight emergency. Request immediate landing.",
			tower: "Al Minhad Tower",
			want:  "Viper 27",
		},
		{
			name:  "is_clear_of_the_active_runway",
			text:  "Minhad Tower, Venom 200 is clear of the active runway.",
			tower: "Al Minhad Tower",
			want:  "Venom 200",
		},
		// Regression — happy paths that must still work.
		{
			name:  "request_taxi",
			text:  "Al Minhad Tower, Raider 032 request taxi.",
			tower: "Al Minhad Tower",
			want:  "Raider 032",
		},
		{
			name:  "short_tower_form",
			text:  "Minhad Tower, Venom 211 holding short runway 09.",
			tower: "Al Minhad Tower",
			want:  "Venom 211",
		},
		{
			name:  "callsign_before_tower",
			text:  "Raider 39, Al Minhad Tower request landing.",
			tower: "Al Minhad Tower",
			want:  "Raider 39",
		},
		{
			name:  "no_tower_comma_fallback",
			text:  "Tower, Venom 211, downwind.",
			tower: "Al Minhad Tower",
			want:  "Venom 211",
		},
		{
			name:  "squadron_regex_fallback",
			text:  "Tower this is Raider 032 inbound seven miles.",
			tower: "Al Minhad Tower",
			want:  "Raider 032",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCallsign(tc.text, tc.tower)
			if got != tc.want {
				t.Errorf("extractCallsign(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestStripLeadingFiller(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"this is Viper 27 declaring", "Viper 27 declaring"},
		{"Al Minhad Tower, this is Viper 27", "Al Minhad Tower, Viper 27"},
		{"i am Raider 032 holding short", "Raider 032 holding short"},
		{"we're inbound seven miles", "inbound seven miles"},
		{"Raider 032 no filler here", "Raider 032 no filler here"},
	}
	for _, tc := range cases {
		got := stripLeadingFiller(tc.in)
		if got != tc.want {
			t.Errorf("stripLeadingFiller(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripTrailingConnectors(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Venom 2-0-0 is", "Venom 2-0-0"},
		{"Raider 032 is now", "Raider 032"},
		{"Venom 200 is the", "Venom 200"},
		{"Raider 039", "Raider 039"},
		{"", ""},
	}
	for _, tc := range cases {
		got := stripTrailingConnectors(tc.in)
		if got != tc.want {
			t.Errorf("stripTrailingConnectors(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
