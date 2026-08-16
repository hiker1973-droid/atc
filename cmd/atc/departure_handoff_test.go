package main

import "testing"

// The departure check-in and the inbound DME report share a distance shape, so
// the whole correctness of the Deckboss → Marshal → Command chain rests on
// isDepartureClearCall telling them apart. A false positive answers a recovering
// pilot with "clear of Union control zone, push command" while they are trying
// to get aboard, which is the failure worth guarding hardest against.
func TestIsDepartureClearCall(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		// Departures — what a pilot says after Deckboss pushes them over.
		{"canonical clear with distance", "union marshal, raider 32, clear 7 miles", true},
		{"clear with dme unit", "union marshal, raider 32, clear 7 dme", true},
		{"clear of with no distance", "marshal, raider 32, clear of mother", true},
		{"word-form distance", "union marshal, raider 32, clear seven miles", true},
		{"outbound phrasing", "marshal, raider 32, clear and outbound", true},
		{"cleared past tense", "union marshal, raider 32, cleared 10 miles", true},

		// Inbound recoveries — must keep the existing DME/stack handling.
		{"bare dme report is inbound", "marshal, raider 39, 7 dme", false},
		{"bare mile report is inbound", "marshal, raider 39, 7 miles", false},
		{"marking mom is a stack check-in", "marshal, raider 39, marking mom", false},
		{"commencing vetoes even with clear", "marshal, raider 39, commencing, 7 miles, clear", false},
		{"platform vetoes", "marshal, raider 39, platform, clear 5 miles", false},
		{"initial vetoes", "marshal, raider 39, 3 mile initial, clear", false},
		{"established vetoes", "marshal, raider 39, established angels 6, clear", false},
		{"see you at ten vetoes", "marshal, raider 39, see you at 10, clear", false},
		{"paddles handoff vetoes", "marshal, raider 39, clear, pushing paddles", false},
		{"bolter vetoes", "marshal, raider 39, clear, bolter", false},
		{"fuel state vetoes", "marshal, raider 39, 7 miles, state 5 point 2, clear", false},

		// No "clear" token at all — nothing here should match.
		{"radio check", "marshal, raider 39, radio check", false},
		{"brc request", "marshal, raider 39, say brc", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDepartureClearCall(tc.text); got != tc.want {
				t.Errorf("isDepartureClearCall(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// The zone name is spoken on every departure release, so a bad derivation is
// heard on every launch.
func TestMarshalZoneName(t *testing.T) {
	if got := marshalZoneName(); got != "Union" {
		t.Errorf("marshalZoneName() = %q, want %q", got, "Union")
	}
}
