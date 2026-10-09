package main

import (
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Kola ATIS is English plus the field's own language (operator 2026-10-09): Norwegian
// at the Norwegian fields, Finnish at Rovaniemi. A station that fell through to the
// theatre default would read Norwegian over a Finnish field.
func TestKolaATISLanguages(t *testing.T) {
	want := map[string]string{
		"ENBO": "Norwegian", // Bodo, Norway
		"ENDU": "Norwegian", // Bardufoss, Norway
		"ENNA": "Norwegian", // Banak, Norway
		"EFRO": "Finnish",   // Rovaniemi, Finland
	}
	stations := atisStationsForMap("kola")
	if len(stations) != len(want) {
		t.Fatalf("atisStationsForMap(\"kola\") returned %d stations, want %d", len(stations), len(want))
	}
	for _, st := range stations {
		w, ok := want[st.ICAO]
		if !ok {
			t.Errorf("unexpected Kola ATIS station %q (%s)", st.Name, st.ICAO)
			continue
		}
		if lang, on := atisSecondLang(st, "kola"); !on || lang != w {
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
	if lang, on := atisSecondLangForMap("kola"); !on || lang != "Norwegian" {
		t.Errorf("kola theatre fallback = (%q, %v), want Norwegian", lang, on)
	}
}

// start_atis_kola.bat runs only the field in play. The filter must leave exactly the
// live tower set on the air, and that set must be what Command hands pilots to.
func TestKolaATISFilterMatchesLiveTowers(t *testing.T) {
	kept := filterATISStations(atisStationsForMap("kola"), "ENBO")
	live := airfield.FieldsForMap("kola")
	if len(kept) != len(live) {
		t.Fatalf("filter kept %d stations, live tower set has %d", len(kept), len(live))
	}
	for i, st := range kept {
		if st.ICAO != live[i].ICAO {
			t.Errorf("station %d = %s, live tower %d = %s", i, st.ICAO, i, live[i].ICAO)
		}
	}
}
