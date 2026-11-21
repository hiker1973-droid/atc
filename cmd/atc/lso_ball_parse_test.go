package main

import "testing"

func TestLSOBallCallForms(t *testing.T) {
	for _, in := range []string{"100, Hornet, Ball, 3.2", "100 Hornet ball 3.2", "100, Hornet, Clara"} {
		if !lsoBallRe.MatchString(in) && !lsoClaraRe.MatchString(in) {
			t.Errorf("%q not read as a ball/clara call", in)
		}
	}
}
