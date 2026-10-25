package composer

import (
	"strings"
	"testing"
)

// Night 2026-10-25: Raider 319 was given "angels two" for Case Three. The
// hold is on the radial at angels + 15 DME, in every wording.
func TestCase3HoldHasDME(t *testing.T) {
	c := NewATCComposer("Union Marshal")
	for i := 0; i < 40; i++ {
		got := c.MarshalMarkingMomCase3("Raider 319", 6, 273, 23, 30.16, 14468, 43, 92, 27, 81, 90, true)
		if !strings.Contains(got, "two seven three") || !strings.Contains(got, "two one DME") || !strings.Contains(got, "angels six") {
			t.Fatalf("marking moms: %s", got)
		}
		ack := c.MarshalEstablishedAckCase3("Raider 319", 7, 273, 23)
		if !strings.Contains(ack, "two two DME") || !strings.Contains(ack, "angels seven") {
			t.Fatalf("established: %s", ack)
		}
	}
}
