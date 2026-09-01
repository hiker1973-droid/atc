package main

import "testing"

// Syria is the first theatre whose fields do not share a country, so the second
// ATIS language is resolved per station rather than per map. A station that
// silently falls through to the theatre default would broadcast Arabic over
// Incirlik or Ramat David, which is the failure this guards.
func TestATISSecondLang(t *testing.T) {
	cases := []struct {
		name     string
		station  atisStation
		mapName  string
		wantLang string
		wantOn   bool
	}{
		{"station override beats the theatre default", atisStation{Lang: "Turkish"}, "syria", "Turkish", true},
		{"English means English only", atisStation{Lang: "English"}, "syria", "", false},
		{"English is matched case-insensitively", atisStation{Lang: "english"}, "syria", "", false},
		{"empty falls back to the theatre", atisStation{}, "ca", "Russian", true},
		{"empty falls back on the default theatre too", atisStation{}, "pg", "Arabic", true},
		{"Germany is unaffected by the per-station path", atisStation{}, "germany", "German", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, on := atisSecondLang(&tc.station, tc.mapName)
			if got != tc.wantLang || on != tc.wantOn {
				t.Errorf("atisSecondLang(%+v, %q) = (%q, %v), want (%q, %v)",
					tc.station, tc.mapName, got, on, tc.wantLang, tc.wantOn)
			}
		})
	}
}

// The eight Syria fields sit in four countries plus a UK Sovereign Base Area.
// Pinning each one means a new station cannot quietly inherit the Arabic
// fallback that only fits the two Jordanian fields.
func TestSyriaATISLanguages(t *testing.T) {
	want := map[string]string{
		"LTAG": "Turkish", // Incirlik, Turkey
		"LTDA": "Turkish", // Hatay, Turkey
		"LTAJ": "Turkish", // Gaziantep, Turkey
		"LLRD": "Hebrew",  // Ramat David, Israel
		"LCPH": "Greek",   // Paphos, Cyprus
		"LCRA": "English", // Akrotiri, UK SBA -- English only
		"OJMF": "Arabic",  // King Hussein, Jordan
		"OJHR": "Arabic",  // H4, Jordan
	}

	stations := atisStationsForMap("syria")
	if len(stations) != len(want) {
		t.Fatalf("atisStationsForMap(\"syria\") returned %d stations, want %d", len(stations), len(want))
	}
	for _, st := range stations {
		w, ok := want[st.ICAO]
		if !ok {
			t.Errorf("unexpected Syria ATIS station %q (%s)", st.Name, st.ICAO)
			continue
		}
		if st.Lang != w {
			t.Errorf("%s Lang = %q, want %q", st.ICAO, st.Lang, w)
		}
		// Akrotiri is the only one that must not get a second pass.
		lang, on := atisSecondLang(st, "syria")
		if st.ICAO == "LCRA" {
			if on {
				t.Errorf("LCRA wants English only, got second language %q", lang)
			}
		} else if !on || lang != w {
			t.Errorf("%s second language = (%q, %v), want (%q, true)", st.ICAO, lang, on, w)
		}
	}
}
