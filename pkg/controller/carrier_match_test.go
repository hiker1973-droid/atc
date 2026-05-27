package controller

import (
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// logCarrierChoice's observable state is lastCarrierChoice — verify it tracks
// transitions correctly so the log-on-change behavior holds. Actual log-level
// switching is not asserted (would require capturing zerolog output) but is
// driven directly by the same predicate this test exercises.
func TestCarrierChoiceTransition(t *testing.T) {
	c := NewATCController("Test", &airfield.Airfield{})

	c.logCarrierChoice("CVN-72 ABE", []string{"CVN-72 ABE"}, "carrier match")
	if c.lastCarrierChoice != "CVN-72 ABE" {
		t.Errorf("after first match lastCarrierChoice = %q, want %q", c.lastCarrierChoice, "CVN-72 ABE")
	}

	c.logCarrierChoice("CVN-72 ABE", []string{"CVN-72 ABE"}, "carrier match")
	if c.lastCarrierChoice != "CVN-72 ABE" {
		t.Errorf("after repeat match lastCarrierChoice = %q, want %q", c.lastCarrierChoice, "CVN-72 ABE")
	}

	c.logCarrierChoice("Carrier strike group-5", []string{"Carrier strike group-5 [group]"}, "carrier match (group-label fallback)")
	if c.lastCarrierChoice != "Carrier strike group-5" {
		t.Errorf("after transition lastCarrierChoice = %q, want %q", c.lastCarrierChoice, "Carrier strike group-5")
	}
}
