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

// Senaki, 2026-10-14. The first two became the callsign whole: a phantom
// "Venom 203 requests taxi to the active" sat at the head of the departure
// queue and blocked the real Venom 203's auto-release, and "Venom 201
// reporting 9 miles" counted as a second aircraft in the landing sequence.
func TestExtractCallsignSenaki20261014(t *testing.T) {
	cases := []struct{ text, want string }{
		{"Tower, Venom 203 requests taxi to the active.", "Venom 203"},
		{"Senaki Tower, Venom 201 reporting 9 miles.", "Venom 201"},
		{"Senaki Tower, Venom 201 reporting initial.", "Venom 201"},
		{"Tower, Venom 201 reporting downwind.", "Venom 201"},
		{"Tower, Venom 201, request permission to the active.", "Venom 201"},
		{"Senaki traffic, Raider 3-0-2, request taxi.", "Raider 3-0-2"},
		{"Senaki Tower, Raider 302 checking in.", "Raider 302"},
		{"Tower, Venom 201 airborne.", "Venom 201"},
	}
	for _, tc := range cases {
		if got := extractCallsign(tc.text, "Senaki Tower"); got != tc.want {
			t.Errorf("extractCallsign(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestPermissionToTheActiveIsTaxi(t *testing.T) {
	for _, text := range []string{
		"Tower, Venom 201, request permission to the active.",
		"Tower, Venom 201, request permission to taxi to the active.",
	} {
		if req := ParseIntent(text, "Senaki Tower"); req == nil || req.Type != RequestTaxiClear {
			t.Errorf("ParseIntent(%q) = %+v, want RequestTaxiClear", text, req)
		}
	}
}

// Senaki, 2026-10-15: "Veta 201" was Venom 201 (18:34), and "Venom 2-0-3"
// (19:43) was answered and tracked apart from "Venom 203".
func TestParseIntentCallsignRepairs20261015(t *testing.T) {
	cases := []struct{ text, want string }{
		{"Tower, Veta 201, permission to taxi the active.", "Venom 201"},
		{"Senaki Tower, Vetta 2-0-1, request taxi.", "Venom 201"},
		{"Senaki Tower, Venom 2-0-3, holding short runway 09.", "Venom 203"},
		{"Senaki traffic, Raider 3-0-2, request taxi.", "Raider 302"},
		{"Senaki Tower, Raider 3 0 2, inbound.", "Raider 302"},
		{"Senaki Tower, Viper 1-1, request taxi.", "Viper 1-1"},
		{"Senaki Tower, Raider 302, request taxi.", "Raider 302"},
	}
	for _, tc := range cases {
		req := ParseIntent(tc.text, "Senaki Tower")
		if req == nil {
			t.Fatalf("ParseIntent(%q) = nil", tc.text)
		}
		if req.Callsign != tc.want {
			t.Errorf("ParseIntent(%q).Callsign = %q, want %q", tc.text, req.Callsign, tc.want)
		}
	}
	for in, want := range map[string]string{
		"the veteran pilot": "the veteran pilot",
		"Veta flight":       "Veta flight",
		"Venom 2-0-3":       "Venom 203",
		"Viper 1 1":         "Viper 1 1",
		"Raider 302":        "Raider 302",
	} {
		got := JoinModexDigits(normalizeCallsign(in))
		if got != want {
			t.Errorf("repair(%q) = %q, want %q", in, got, want)
		}
	}
}
