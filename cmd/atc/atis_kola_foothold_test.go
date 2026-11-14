package main

import (
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Foothold Kola (operator 2026-11-14): five ATIS stations, Andoya first, each paired to a tower dashboard
// port and speaking its own language. The White Night Kola set (4 stations) must stay exactly as it was.
func TestKolaFootholdATIS(t *testing.T) {
	want := map[string]string{
		"ENAN": "Norwegian", // Andoya
		"ENBO": "Norwegian", // Bodo
		"ENDU": "Norwegian", // Bardufoss
		"ENNA": "Norwegian", // Banak
		"EFRO": "Finnish",   // Rovaniemi
	}
	for _, m := range []string{"kola-foothold", "kolafh", " Kola-FH "} {
		stations := atisStationsForMap(m)
		if len(stations) != len(want) {
			t.Fatalf("atisStationsForMap(%q) returned %d stations, want %d", m, len(stations), len(want))
		}
		for _, st := range stations {
			w, ok := want[st.ICAO]
			if !ok {
				t.Errorf("unexpected Foothold Kola ATIS station %q (%s)", st.Name, st.ICAO)
				continue
			}
			if lang, on := atisSecondLang(st, m); !on || lang != w {
				t.Errorf("%s second language = (%q, %v), want (%q, true)", st.ICAO, lang, on, w)
			}
			af := airfield.ByICAO(st.ICAO)
			if af == nil || af.ATISFreqMHz != st.FreqMHz {
				t.Errorf("%s ATIS station is on %.3f but the airfield says %v", st.ICAO, st.FreqMHz, af)
			}
			if _, paired := towerDashboardPortByICAO[st.ICAO]; !paired {
				t.Errorf("%s has no tower dashboard port, so its ATIS cannot mirror the tower's runway", st.ICAO)
			}
		}
	}
	if n := len(atisStationsForMap("kola")); n != 4 {
		t.Errorf("the White Night Kola ATIS set changed to %d stations; it must stay 4", n)
	}
}

// start_atis_kola_foothold.bat names all five ICAOs; the filter must keep exactly the Foothold tower set,
// in the order Command hands pilots to them.
func TestKolaFootholdATISFilterMatchesTowers(t *testing.T) {
	kept := filterATISStations(atisStationsForMap("kola-foothold"), "ENAN,ENBO,ENDU,ENNA,EFRO")
	live := airfield.FieldsForMap("kola-foothold")
	if len(kept) != len(live) {
		t.Fatalf("filter kept %d stations, tower set has %d", len(kept), len(live))
	}
	for i, st := range kept {
		if st.ICAO != live[i].ICAO {
			t.Errorf("station %d = %s, tower %d = %s", i, st.ICAO, i, live[i].ICAO)
		}
	}
}
