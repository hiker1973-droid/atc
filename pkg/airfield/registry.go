package airfield

import "strings"

// registry maps ICAO → airfield for every theatre. Keyed lookups replace the
// per-call-site switch/slice literals that used to hardcode the PG fields.
var registry = map[string]*Airfield{
	"OMDM": OMDM, "OMAM": OMAM, "OMAL": OMAL, // Persian Gulf
	"UGSB": UGSB, "UG5X": UG5X, "UGKS": UGKS, "UGKO": UGKO, // Caucasus / Black Sea
	"ETAR": ETAR, "ETAD": ETAD, "EDFH": EDFH, "EDDF": EDDF, // Cold War Germany
	"EDDK": EDDK, "EDDL": EDDL, "EDDV": EDDV, "EDDH": EDDH, // Cold War Germany
}

// ByICAO returns the airfield for an ICAO (case-insensitive), or nil if unknown.
func ByICAO(icao string) *Airfield { return registry[strings.ToUpper(strings.TrimSpace(icao))] }

// Tower-controlled fields per theatre, in launch order. Used by the ATIS set,
// the Command proactive-handoff scan, and the start scripts.
var (
	PersianGulf = []*Airfield{OMDM, OMAM, OMAL}
	Caucasus    = []*Airfield{UGSB, UG5X, UGKS, UGKO}
	// Germany is the 8 F/A-18 recovery bases from the Cold War Germany COMM1
	// preset card (tower on VHF, ATIS on UHF), in launch order.
	Germany = []*Airfield{ETAR, ETAD, EDFH, EDDF, EDDK, EDDL, EDDV, EDDH}
)

// FieldsForMap returns the tower fields for a map name (default: Persian Gulf).
// Accepts "pg"/"persiangulf", "ca"/"caucasus"/"blacksea", and "germany"/"de".
func FieldsForMap(name string) []*Airfield {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ca", "caucasus", "blacksea", "black-sea", "black sea":
		return Caucasus
	case "germany", "de", "coldwargermany", "cold-war-germany", "cwg":
		return Germany
	default:
		return PersianGulf
	}
}
