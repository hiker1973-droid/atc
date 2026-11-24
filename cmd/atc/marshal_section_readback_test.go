package main

import (
	"testing"

	"github.com/vsfg7/atc/pkg/state"
)

func TestIsSectionWingman(t *testing.T) {
	st := state.NewMarshalStack()
	st.Enqueue("Raider 302", 6.0)
	joinSection(st, "Raider 302", []string{"Raider 303", "Raider 304"})
	for _, c := range []struct {
		cs, lower string
		want      bool
	}{
		{"Raider 303", "raider 303 brc 096", true},
		{"Raider 304", "", true},
		{"Raider 302", "raider 302 brc 096", false},
		{"", "030 brc 096", false},
		{"", "303 brc 096", true},
		{"Venom 212", "venom 212 brc 096", false},
	} {
		if got := isSectionWingman(st, c.cs, c.lower); got != c.want {
			t.Errorf("%q/%q: got %v want %v", c.cs, c.lower, got, c.want)
		}
	}
}
