package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

// newRecordingController returns a controller whose transmissions land in the
// returned slice, and a helper that feeds it a transcript end to end.
func newRecordingController(t *testing.T) (*ATCController, *[]string, func(string)) {
	t.Helper()
	const tower = "Senaki Tower"
	c := NewATCController(tower, &airfield.Airfield{})
	sent := &[]string{}
	c.SetTransmitFn(func(_ context.Context, text string) { *sent = append(*sent, text) })
	handle := func(text string) {
		t.Helper()
		req := ParseIntent(text, tower)
		if req == nil {
			t.Fatalf("ParseIntent(%q) = nil", text)
		}
		c.HandleRequest(context.Background(), req)
	}
	return c, sent, handle
}

func TestSayAgainRepeatsLastTransmission(t *testing.T) {
	c, sent, handle := newRecordingController(t)

	handle("Senaki Tower, Raider 11, say again")
	if len(*sent) != 1 || !containsAny((*sent)[0], "go ahead", "say request") {
		t.Fatalf("say again with nothing on file: got %q", *sent)
	}

	handle("Senaki Tower, Raider 11, radio check")
	handle("Senaki Tower, Raider 11, say again")
	if len(*sent) != 3 || (*sent)[2] != (*sent)[1] {
		t.Fatalf("say again should repeat the radio check reply verbatim: got %q", *sent)
	}

	// Another pilot's say again must not get Raider 11's clearance.
	handle("Senaki Tower, Venom 21, say again")
	if (*sent)[3] == (*sent)[1] {
		t.Errorf("Venom 21 was read Raider 11's transmission: %q", (*sent)[3])
	}

	// A stale transmission is not repeated.
	c.lastTxMu.Lock()
	tx := c.lastTx["Raider 11"]
	tx.at = time.Now().Add(-2 * SayAgainWindow)
	c.lastTx["Raider 11"] = tx
	c.lastTxMu.Unlock()
	handle("Senaki Tower, Raider 11, say again")
	if last := (*sent)[len(*sent)-1]; last == (*sent)[1] {
		t.Errorf("stale transmission was repeated: %q", last)
	}
}

func TestOptionClearanceReplacesClearedToLand(t *testing.T) {
	cases := []struct {
		text, want, notWant string
	}{
		{"Senaki Tower, Raider 11, on final, touch and go", "cleared touch and go", "cleared to land"},
		{"Senaki Tower, Raider 11, on final, low approach", "cleared low approach", "cleared to land"},
		{"Senaki Tower, Raider 11, request the option", "cleared for the option", "cleared to land"},
		{"Senaki Tower, Raider 11, on final, full stop", "cleared to land", "touch and go"},
	}
	for _, tc := range cases {
		for i := 0; i < 10; i++ {
			_, sent, handle := newRecordingController(t)
			handle(tc.text)
			if len(*sent) != 1 {
				t.Fatalf("%q: want one transmission, got %q", tc.text, *sent)
			}
			got := (*sent)[0]
			if !strings.Contains(got, tc.want) || strings.Contains(got, tc.notWant) {
				t.Fatalf("%q: got %q, want %q and not %q", tc.text, got, tc.want, tc.notWant)
			}
		}
	}
}

func TestHungOrdnanceAndWindCheckReplies(t *testing.T) {
	for i := 0; i < 10; i++ {
		_, sent, handle := newRecordingController(t)
		handle("Senaki Tower, Raider 11, hung ordnance, 10 miles south")
		handle("Senaki Tower, Raider 12, wind check")
		if len(*sent) != 2 {
			t.Fatalf("want two transmissions, got %q", *sent)
		}
		if hung := (*sent)[0]; !strings.Contains(hung, "straight in") || !strings.Contains(hung, "dearm") {
			t.Fatalf("hung ordnance reply missing straight in / dearm: %q", hung)
		}
		if wind := (*sent)[1]; !strings.Contains(wind, "wind") || !strings.Contains(wind, "altimeter") {
			t.Fatalf("wind check reply missing wind / altimeter: %q", wind)
		}
	}
}
