package main

import (
	"math"
	"testing"
)

func TestLSODeckWind(t *testing.T) {
	// Ship 30 kt on 090, 15 kt wind from the ship's heading axis (081): down the
	// angle = 15 + 30*cos9 = ~44.6, cross = 30*sin9 = ~4.7 from starboard.
	d, c := lsoDeckWind(30, 90, 81, 15)
	if math.Abs(d-44.6) > 0.2 || math.Abs(c-4.7) > 0.2 {
		t.Errorf("got %.1f down, %.1f cross", d, c)
	}
	// Dead calm: only the ship's motion.
	d, c = lsoDeckWind(25, 180, 0, 0)
	if math.Abs(d-24.7) > 0.2 || math.Abs(c-3.9) > 0.2 {
		t.Errorf("calm: got %.1f, %.1f", d, c)
	}
	got := lsoAbeamWindReply("302", 22.4, -3.6)
	if got != "302, Paddles contact, you've got 22 knots down the angle, 4 from port." {
		t.Errorf("reply %q", got)
	}
}
