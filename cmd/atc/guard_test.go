package main

import (
	"strings"
	"testing"

	"github.com/paulmach/orb"
)

// Guard Watch: damage and lost calls get the nearest Kola field; chatter and
// calls for other roles are left alone.
func TestGuardReply(t *testing.T) {
	saveMap := flagMap
	defer func() { flagMap = saveMap }()
	flagMap = "kola"
	store := newTacviewPositions()
	// A jet 25 nm or so off Bodo.
	store.Set("Raider 302", orb.Point{14.0, 67.0})

	for _, c := range []struct {
		text, want, not string
		silent          bool
	}{
		{text: "Guard, Raider 302, mayday, engine fire", want: "Nearest field"},
		{text: "Guard, Raider 302, I'm hit", want: "Nearest field"},
		{text: "Guard, Raider 302, lost, request vectors", want: "say intentions", not: "declare your emergency"},
		{text: "Guard, Raider 302, radio check", want: "loud and clear"},
		{text: "Raider 302, mayday", want: "Nearest field"},
		{text: "Tower, Raider 302, request taxi", silent: true},
		{text: "Command, Raider 302, fence in", silent: true},
		{text: "Guard, Raider 302, target is hit", silent: true},
	} {
		_, got := guardReply(c.text, "Guard", store)
		if c.silent {
			if got != "" {
				t.Errorf("%q: should be silent, got %q", c.text, got)
			}
			continue
		}
		if got == "" || (c.want != "" && !strings.Contains(got, c.want) && !strings.Contains(got, strings.ReplaceAll(c.want, " ", ""))) {
			t.Errorf("%q: reply %q lacks %q", c.text, got, c.want)
		}
		if c.not != "" && strings.Contains(got, c.not) {
			t.Errorf("%q: reply %q has %q", c.text, got, c.not)
		}
	}
}
