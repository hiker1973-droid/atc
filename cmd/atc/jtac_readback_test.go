package main

import "testing"

func TestGridReadOK(t *testing.T) {
	g := "3813184036"
	for _, d := range []string{"3813184036", "38134036", "38138403"} {
		if !gridReadOK(d, g) {
			t.Errorf("%s should pass", d)
		}
	}
	for _, d := range []string{"", "38138036", "99994036"} {
		if gridReadOK(d, g) {
			t.Errorf("%s should fail", d)
		}
	}
}

func TestStripCallsignSpacedDigits(t *testing.T) {
	got := stripCallsign("hitman 1-1, raider 3-0-2, 209, 5 armor", "Raider 302")
	if want := "hitman 1-1,  209, 5 armor"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestJtacCallsignHammer(t *testing.T) {
	if cs := jtacCallsign("Hammer 1-1, Raider 302, ready 9-line."); cs != "Raider 302" {
		t.Errorf("got %q", cs)
	}
}
