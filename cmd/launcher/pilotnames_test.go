package main

import "testing"

func TestStandardPilotName(t *testing.T) {
	for in, want := range map[string]string{
		"Raider 306 | Turbo":     "Raider 306 | Turbo",
		"Raider 302| Jedi":      "Raider 302 | Jedi",
		"RAIDER 031 | BACON":     "Raider 031 | Bacon",
		"leeroy-venom 024":       "Venom 024 | Leeroy",
		"vSFG-7 | Leaker | 029":  "vSFG-7 | Leaker | 029", // no flight word: only tidied
		"RAIDER 2-1 | FERGI":     "Raider 2-1 | Fergi",
		"Venom 023 | KC-DC":      "Venom 023 | KC-DC",
		"Raider 312 | longbranch": "Raider 312 | Longbranch",
		"NERF":                   "Nerf",
	} {
		if got := standardPilotName(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestIsStandardPilotName(t *testing.T) {
	for n, want := range map[string]bool{"Raider 302 | Jedi": true, "Venom 024 | Leeroy": true, "Raider 2-1 | Fergi": true,
		"Nerf": false, "vSFG-7 | Leaker | 029": false, "Raider 302": false, "Viper 1 | X": false} {
		if isStandardPilotName(n) != want {
			t.Errorf("%q = %v, want %v", n, !want, want)
		}
	}
}
