package main

import "testing"

func TestDeckbossBareModexImpliesRaider(t *testing.T) {
	for in, want := range map[string]string{
		"Deckboss, 302, ready for cat": "Raider 302",
		"Deck boss, 3 0 2, airborne":   "Raider 302",
		"Deckboss, Raider 308, ready":  "Raider 308",
	} {
		if got := deckbossCallsign(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
