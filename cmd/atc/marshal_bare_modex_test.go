package main

import "testing"

func TestMarshalBareModexImpliesRaider(t *testing.T) {
	cases := map[string]string{
		"Union Marshal, 302, established, angels 2": "Raider 302",
		"Marshal, 3 0 2, state 6.5":                 "Raider 302",
		"Union Marshal, Venom 212, established":     "Venom 212",
		"Union Marshal, 350.5 state":                "",
	}
	for in, want := range cases {
		if got := marshalCallsignFromText(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestMarshalOverheadIsEstablished(t *testing.T) {
	for _, in := range []string{
		"union marshal, raider 302, overhead",
		"union marshal, 302 overhead, state 5.5",
		"union marshal, raider 303 overhead, angels 2",
	} {
		if got := classifyMarshalCall(in, extractFuelStateMarshal(in)); got != marshalEstablished {
			t.Errorf("%q: intent %d, want established", in, got)
		}
	}
}
