package controller

import "testing"

// Calls from the Day (Senaki) and Foothold (Bassel) logs of 2026-10-27 - 28.
func TestFlown20261027(t *testing.T) {
	cases := []struct {
		name, tower, text string
		addressed         bool
		callsign          string
	}{
		{"scooby is senaki", "Senaki Tower", "Scooby Tower, Venom 201 reporting 14 miles.", true, "Venom 201"},
		{"sukhoi is senaki", "Senaki Tower", "Sukhoi Tower, Venom 201, clear the active.", true, "Venom 201"},
		{"callsign before tower", "Bassel Al-Assad Tower", "Venom 212, tower, Venom 212 airborne.", true, "Venom 212"},
		{"flight size is not a callsign", "Senaki Tower", "Senaki Tower, flight of two F-16s inbound at 36 nautical miles out.", true, ""},
		{"two-ship digit split off", "Senaki Tower", "Senaki Tower, Venom 0-2-0-2, ship F-16 inbound at 30 miles.", true, "Venom 020"},
		{"four-digit modex untouched", "Senaki Tower", "Senaki Tower, Venom 0202, request taxi.", true, "Venom 0202"},
		{"another field's tower still dropped", "Senaki Tower", "Kobuleti Tower, Venom 201, request taxi.", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := ParseIntent(tc.text, tc.tower)
			if (req != nil) != tc.addressed {
				t.Fatalf("ParseIntent(%q) addressed = %v, want %v", tc.text, req != nil, tc.addressed)
			}
			if req != nil && req.Callsign != tc.callsign {
				t.Errorf("callsign = %q, want %q", req.Callsign, tc.callsign)
			}
		})
	}
	if req := ParseIntent("Senaki Tower, Venom 0-2-0-2, ship F-16 inbound at 30 miles.", "Senaki Tower"); req != nil && req.FlightSize != 2 {
		t.Errorf("2-ship flight size = %d, want 2", req.FlightSize)
	}
}
