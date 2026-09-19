package controller

import "testing"

// Addressing is the gate in front of every tower reply: a false negative is a
// pilot hearing silence, a false positive is a tower answering a call meant
// for another field. The first block is the Foothold 2026-09-13 session.
func TestParseIntentAddressing(t *testing.T) {
	cases := []struct {
		name, tower, text string
		want              bool
	}{
		// Live misses, Akrotiri / H4 on Foothold.
		{"field name without tower", "Akrotiri Tower", "Akrotiri, Raider 331, are ready to taxi.", true},
		{"field name without tower or comma", "Akrotiri Tower", "Akrotiri Raider 331 checking in.", true},
		{"misspelt field with tower", "Akrotiri Tower", "Acrotari Tower, Raider 331, radar check.", true},
		{"badly misspelt field with tower", "Akrotiri Tower", "Akatari Tower, Raider 331, radar check.", true},
		{"another field's call on this frequency", "H4 Tower", "Acrotari Tower, Raider 331, radar check.", false},

		// Spoken names that don't derive from the callsign.
		{"king hussein tower", "Mafraq Tower", "King Hussein Tower, Raider 11, request taxi", true},
		{"king hussein without tower", "Mafraq Tower", "King Hussein, Raider 11, inbound", true},
		{"h4 hyphenated", "H4 Tower", "H-4 Tower, Raider 11, radio check", true},
		{"h4 without tower", "H4 Tower", "H4, Raider 11, radio check", true},
		{"bassel tower", "Bassel Al-Assad Tower", "Bassel Tower, Raider 11, request taxi", true},
		{"bassel without tower", "Bassel Al-Assad Tower", "Bassel, Raider 11, inbound", true},
		{"basel misspelt", "Bassel Al-Assad Tower", "Basel Tower, Raider 11, radio check", true},
		{"bassel full name", "Bassel Al-Assad Tower", "Bassel al-Assad Tower, Raider 11, ready to taxi", true},
		{"latakia is bassel", "Bassel Al-Assad Tower", "Latakia Tower, Raider 11, radio check", true},
		{"beirut", "Beirut Tower", "Beirut, Raider 11, request taxi", true},
		{"beirut is not bassel", "Bassel Al-Assad Tower", "Beirut Tower, Raider 11, request taxi", false},

		// Other shapes of the field-name address.
		{"misspelt without tower", "Incirlik Tower", "Injirlik, Raider 11, request taxi", true},
		{"pg short name", "Al Minhad Tower", "Minhad, Raider 11, request taxi", true},
		{"two-word field", "Ramat David Tower", "Ramat David, Raider 11, request taxi", true},
		{"split one-word field", "Gaziantep Tower", "Gazi Antep Tower, Raider 11, radio check", true},

		// Must stay unaddressed.
		{"field name mid-call", "Akrotiri Tower", "Raider 331, inbound from Akrotiri", false},
		{"too far from the name", "Akrotiri Tower", "Acapulco, Raider 331, radio check", false},
		{"short name is not fuzzy", "H4 Tower", "H5, Raider 11, radio check", false},
		{"different field", "Paphos Tower", "Akrotiri, Raider 331, ready to taxi", false},

		// Existing address forms still work.
		{"field tower", "Akrotiri Tower", "Akrotiri Tower, Raider 331, request taxi", true},
		{"ctaf", "Akrotiri Tower", "Akrotiri traffic, Raider 331, departing runway 28", true},
		{"bare tower", "Akrotiri Tower", "Tower, Raider 331, radio check", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseIntent(tc.text, tc.tower) != nil; got != tc.want {
				t.Errorf("ParseIntent(%q, %q) addressed = %v, want %v", tc.text, tc.tower, got, tc.want)
			}
		})
	}
}

// Once addressed, the call must still classify and yield the callsign.
func TestFieldNameAddressClassifies(t *testing.T) {
	req := ParseIntent("Akrotiri, Raider 331, are ready to taxi.", "Akrotiri Tower")
	if req == nil {
		t.Fatal("not addressed")
	}
	if req.Type != RequestTaxiClear || req.Callsign != "Raider 331" {
		t.Errorf("got type %d callsign %q, want taxi / Raider 331", req.Type, req.Callsign)
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"akrotiri", "akrotiri", 0},
		{"acrotari", "akrotiri", 2},
		{"akatari", "akrotiri", 3},
		{"injirlik", "incirlik", 1},
		{"", "abc", 3},
	}
	for _, tc := range cases {
		if got := levenshtein(tc.a, tc.b); got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
