package airfield

import "strings"

// registry maps ICAO → airfield for every theatre. Keyed lookups replace the
// per-call-site switch/slice literals that used to hardcode the PG fields.
var registry = map[string]*Airfield{
	"OMDM": OMDM, "OMAM": OMAM, "OMAL": OMAL, // Persian Gulf
	"UGSB": UGSB, "UG5X": UG5X, "UGKS": UGKS, "UGKO": UGKO, // Caucasus / Black Sea
}

// ByICAO returns the airfield for an ICAO (case-insensitive), or nil if unknown.
func ByICAO(icao string) *Airfield { return registry[strings.ToUpper(strings.TrimSpace(icao))] }

// Tower-controlled fields per theatre, in launch order. Used by the ATIS set,
// the Command proactive-handoff scan, and the start scripts.
var (
	PersianGulf = []*Airfield{OMDM, OMAM, OMAL}
	Caucasus    = []*Airfield{UGSB, UG5X, UGKS, UGKO}
)

// FieldsForMap returns the tower fields for a map name (default: Persian Gulf).
// Accepts "pg"/"persiangulf" and "ca"/"caucasus"/"blacksea".
func FieldsForMap(name string) []*Airfield {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ca", "caucasus", "blacksea", "black-sea", "black sea":
		return Caucasus
	default:
		return PersianGulf
	}
}
