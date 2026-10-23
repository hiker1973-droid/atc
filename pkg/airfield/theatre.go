package airfield

import "strings"

// TheatreKey is the canonical name for a map name, the same keys the
// launcher uses for its regions: caucasus, germany, iraq, syria, kola, pg.
// Like FieldsForMap, an unknown name is the Persian Gulf.
func TheatreKey(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ca", "caucasus", "blacksea", "black-sea", "black sea":
		return "caucasus"
	case "germany", "de", "coldwargermany", "cold-war-germany", "cwg":
		return "germany"
	case "iraq", "iq", "or":
		return "iraq"
	case "syria", "sy", "easternmed", "eastern-med", "eastern med", "emed":
		return "syria"
	case "kola", "ko", "kolapeninsula", "kola-peninsula", "arctic":
		return "kola"
	}
	return "pg"
}

// TheatreOfICAO is the theatre whose tower list holds a field; Marshal,
// Deckboss and Paddles name only an --airfield. A field in no list is "pg".
func TheatreOfICAO(icao string) string {
	icao = strings.ToUpper(strings.TrimSpace(icao))
	for _, t := range []string{"caucasus", "germany", "iraq", "syria", "kola"} {
		for _, af := range FieldsForMap(t) {
			if af.ICAO == icao {
				return t
			}
		}
	}
	return "pg"
}
