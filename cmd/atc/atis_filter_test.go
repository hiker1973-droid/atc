package main

import "testing"

// TestFilterATISStationsEmptyKeepsTheatreSet pins the default: no
// --atis-stations means every station in the theatre set, so adding the flag
// cannot change PG, Caucasus, Germany or Iraq behaviour.
func TestFilterATISStationsEmptyKeepsTheatreSet(t *testing.T) {
	all := atisStationsForMap("syria")
	for _, list := range []string{"", "   "} {
		got := filterATISStations(all, list)
		if len(got) != len(all) {
			t.Fatalf("filterATISStations(all, %q) = %d stations, want %d", list, len(got), len(all))
		}
	}
}

// TestFilterATISStationsSelectsSyriaFive covers the 2026-09-19 operator roster:
// three primaries plus the two divert fields, and nothing else on the air.
func TestFilterATISStationsSelectsSyriaFive(t *testing.T) {
	got := filterATISStations(atisStationsForMap("syria"), "LCRA,LTAG,LLRD,OSLK,OLBA")

	want := map[string]bool{"LCRA": true, "LTAG": true, "LLRD": true, "OSLK": true, "OLBA": true}
	if len(got) != len(want) {
		t.Fatalf("got %d stations, want %d", len(got), len(want))
	}
	for _, st := range got {
		if !want[st.ICAO] {
			t.Errorf("station %q (%s) should have been filtered out", st.Name, st.ICAO)
		}
		delete(want, st.ICAO)
	}
	for icao := range want {
		t.Errorf("station %s missing from filtered set", icao)
	}
}

// TestFilterATISStationsIgnoresCaseAndSpacing — start scripts are hand-edited,
// so a stray space or a lowercase ICAO must not silently drop a station.
func TestFilterATISStationsIgnoresCaseAndSpacing(t *testing.T) {
	got := filterATISStations(atisStationsForMap("syria"), " lcra , OsLk ")
	if len(got) != 2 {
		t.Fatalf("got %d stations, want 2", len(got))
	}
}

// TestFilterATISStationsSkipsUnknownICAO — an ICAO with no station in this
// theatre is warned about and ignored; the valid entries still come through.
// (A filter matching NOTHING is fatal by design and is not exercised here.)
func TestFilterATISStationsSkipsUnknownICAO(t *testing.T) {
	got := filterATISStations(atisStationsForMap("syria"), "LCRA,OMDM")
	if len(got) != 1 || got[0].ICAO != "LCRA" {
		t.Fatalf("got %v, want just LCRA", got)
	}
}

// TestDivertFieldsAreBilingual guards the broadcast cadence: both divert
// stations read a second language, so they must land on the 90s interval and
// not the 45s tick their audio would overrun.
func TestDivertFieldsAreBilingual(t *testing.T) {
	for _, st := range atisStationsForMap("syria") {
		if st.ICAO != "OSLK" && st.ICAO != "OLBA" {
			continue
		}
		if lang, on := atisSecondLang(st, "syria"); !on || lang != "Arabic" {
			t.Errorf("%s second language = (%q, %v), want (\"Arabic\", true)", st.ICAO, lang, on)
		}
		if got := atisBroadcastIntervalSec(st, "syria"); got != atisIntervalBilingualSec {
			t.Errorf("%s interval = %ds, want %ds", st.ICAO, got, atisIntervalBilingualSec)
		}
	}
}
