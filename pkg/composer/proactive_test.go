package composer

import (
	"strings"
	"testing"
)

func TestCardinalWord(t *testing.T) {
	cases := map[float64]string{
		0: "north", 22: "north", 23: "northeast", 45: "northeast", 90: "east",
		135: "southeast", 180: "south", 225: "southwest", 270: "west",
		315: "northwest", 338: "north", 359.9: "north",
	}
	for deg, want := range cases {
		if got := CardinalWord(deg); got != want {
			t.Errorf("CardinalWord(%v) = %q, want %q", deg, got, want)
		}
	}
}

func TestUnknownTrafficSayIntentions(t *testing.T) {
	c := NewATCComposer("Senaki Tower")
	for i := 0; i < 20; i++ {
		got := c.UnknownTrafficSayIntentions("Raider 11", 6, 45)
		for _, want := range []string{"Raider 11", "six miles northeast", "say intentions"} {
			if !strings.Contains(got, want) {
				t.Fatalf("got %q, missing %q", got, want)
			}
		}
	}
}
