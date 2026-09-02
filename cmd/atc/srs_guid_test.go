package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The GUID length is what SRS sees on the wire, and each role had its own
// width before this helper existed. Changing a role's width is a protocol
// change, not a refactor, so pin the four live call sites.
func TestSRSClientGUIDLength(t *testing.T) {
	cases := []struct {
		prefix string
		digits int
		want   int
	}{
		{"vsfg7atc", 14, 22}, // towers
		{"vsfg7msh", 14, 22}, // marshal
		{"vsfg7dkb", 14, 22}, // deckboss
		// Command asks for 15 and then truncates to 22 at its call site, so the
		// last random digit never reaches the wire. Harmless -- the PID segment
		// sits at chars 8-14 and survives the cut -- but do not "tidy" this to 14
		// expecting the on-wire value to change; it already is 22.
		{"vsfg7cmd", 15, 23},
	}
	for _, c := range cases {
		got := srsClientGUID(c.prefix, c.digits)
		if len(got) != c.want {
			t.Errorf("srsClientGUID(%q, %d) = %q: length %d, want %d",
				c.prefix, c.digits, got, len(got), c.want)
		}
		if !strings.HasPrefix(got, c.prefix) {
			t.Errorf("srsClientGUID(%q, %d) = %q: lost its prefix", c.prefix, c.digits, got)
		}
		for _, r := range got[len(c.prefix):] {
			if r < '0' || r > '9' {
				t.Errorf("srsClientGUID(%q, %d) = %q: non-digit %q in tail",
					c.prefix, c.digits, got, r)
			}
		}
	}
}

// The bug this replaced: eight towers launched in the same Windows clock tick
// drew identical GUIDs, so SRS collapsed them into shared client slots and the
// losers went silent while still appearing connected.
//
// Cross-process uniqueness -- the property that actually matters -- rests on the
// PID, which is unique among live processes and is exactly what the old
// timestamp scheme lacked. A test in one process cannot observe that, so pin the
// PID segment instead.
func TestSRSClientGUIDEmbedsPID(t *testing.T) {
	got := srsClientGUID("vsfg7atc", 14)
	want := fmt.Sprintf("%06d", os.Getpid()%1000000)
	if pid := got[len("vsfg7atc") : len("vsfg7atc")+6]; pid != want {
		t.Errorf("srsClientGUID = %q: PID segment %q, want %q", got, pid, want)
	}
}

// The random tail is the secondary guard, covering PID reuse and the %1e6 wrap.
// It is only 10^8 wide, so draws are not guaranteed distinct -- birthday odds
// over 10k draws are ~39%. This asserts only that the tail actually varies, i.e.
// that rand is not returning a constant, which is what would silently reduce the
// GUID back to one value per PID.
func TestSRSClientGUIDTailVaries(t *testing.T) {
	const n = 100
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		seen[srsClientGUID("vsfg7atc", 14)] = true
	}
	if len(seen) < n*9/10 {
		t.Errorf("%d distinct GUIDs from %d draws -- random tail is not varying", len(seen), n)
	}
}
