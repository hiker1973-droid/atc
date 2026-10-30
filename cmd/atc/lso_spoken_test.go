package main

import "testing"

func TestLSOSpokenModex(t *testing.T) {
	for in, want := range map[string]string{
		"Three-one-nine, abeam.":        "319, abeam.",
		"4 of 5 Hornet ball, 2.5.":      "405 Hornet ball, 2.5.",
		"Raider three one six abeam":    "Raider 316 abeam",
		"three-quarter mile, 5.4":       "three-quarter mile, 5.4",
		"319 Hornet ball, 5.":           "319 Hornet ball, 5.",
		"Four zero niner, in the break": "409, in the break",
	} {
		if got := lsoSpokenModex(in); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}
