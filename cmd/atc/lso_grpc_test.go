package main

import (
	"testing"
	"time"
)

func TestParseDCSGrade(t *testing.T) {
	for _, c := range []struct {
		in    string
		grade string
		wire  int
	}{
		{"LSO: GRADE:OK  : WIRE# 3", "OK", 3},
		{"LSO: GRADE:--- (LUL)X _LUL_IM  LOIC : WIRE# 1", "---", 1},
		{"LSO: GRADE:_OK_  WIRE# 2", "_OK_", 2},
		{"LSO: GRADE:B", "B", 0},
		{"LSO: GRADE:WO  (H)X", "WO", 0},
	} {
		g, w := parseDCSGrade(c.in)
		if g != c.grade || w != c.wire {
			t.Errorf("%q: got %q %d, want %q %d", c.in, g, w, c.grade, c.wire)
		}
	}
}

// A Paddles pass and DCS's grade for the same jet pair up whichever comes
// first; a stale one does not.
func TestLSOTruthPairs(t *testing.T) {
	tr := newLSOTruth()
	now := time.Now()
	tr.grade("Raider 302", "LSO: GRADE:OK  : WIRE# 3", now)
	tr.pass("Raider 302 (FA-18C_hornet): trap, grade (OK): 3 wire", now.Add(5*time.Second))
	if len(tr.dcs)+len(tr.passes) != 0 {
		t.Errorf("DCS then Paddles: not paired (%v %v)", tr.dcs, tr.passes)
	}
	tr.pass("Raider 312 (FA-18C_hornet): bolter, grade B", now)
	tr.grade("Raider 312", "LSO: GRADE:B", now.Add(2*time.Second))
	if len(tr.dcs)+len(tr.passes) != 0 {
		t.Errorf("Paddles then DCS: not paired (%v %v)", tr.dcs, tr.passes)
	}
	tr.pass("Raider 319 (FA-18C_hornet): trap", now)
	tr.grade("Raider 319", "LSO: GRADE:OK  : WIRE# 2", now.Add(5*time.Minute))
	if len(tr.dcs) != 1 {
		t.Errorf("5 min apart: want the DCS grade kept unpaired")
	}
}
