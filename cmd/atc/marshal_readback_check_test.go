package main

import "testing"

func TestBRCReadbackValueRe(t *testing.T) {
	for in, want := range map[string]string{
		"raider 302 brc 096 marshalling at angels 2": "096",
		"understands brc is 270, altimeter 2992":     "270",
		"copy brc 09 stick":                          "",
		"marshal on the 263 angels 6":                "",
	} {
		m := brcReadbackValueRe.FindStringSubmatch(in)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
