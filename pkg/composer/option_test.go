package composer

import (
	"strings"
	"testing"
)

func TestOptionClearance(t *testing.T) {
	c := NewATCComposer("Senaki Tower")
	cases := []struct {
		option, want string
	}{
		{OptionTouchAndGo, "cleared touch and go."},
		{OptionLowApproach, "cleared low approach."},
		{OptionTheOption, "cleared for the option."},
	}
	for _, tc := range cases {
		for i := 0; i < 20; i++ {
			got := c.OptionClearance("Raider 11", "09", 90, 12, tc.option, true)
			// The clearance is the last thing the pilot hears (7110.65 3-10-5).
			if !strings.HasSuffix(got, tc.want) {
				t.Fatalf("OptionClearance(%q) = %q, want suffix %q", tc.option, got, tc.want)
			}
			if !strings.Contains(got, "check wheels down") {
				t.Fatalf("OptionClearance(%q) dropped the wheels check: %q", tc.option, got)
			}
			if strings.Contains(got, "cleared to land") {
				t.Fatalf("OptionClearance(%q) said cleared to land: %q", tc.option, got)
			}
		}
	}
	if got := c.OptionClearance("Raider 11", "09", 90, 12, OptionTouchAndGo, false); strings.Contains(got, "wheels") {
		t.Errorf("wheels check issued when not owed: %q", got)
	}
}
