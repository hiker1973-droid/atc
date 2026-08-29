package airfield

import "strings"

// registry maps ICAO → airfield for every theatre. Keyed lookups replace the
// per-call-site switch/slice literals that used to hardcode the PG fields.
var registry = map[string]*Airfield{
	"OMDM": OMDM, "OMAM": OMAM, "OMAL": OMAL, // Persian Gulf
	"UGSB": UGSB, "UG5X": UG5X, "UGKS": UGKS, "UGKO": UGKO, // Caucasus / Black Sea
	"ETAR": ETAR, "ETAD": ETAD, "EDFH": EDFH, "EDDF": EDDF, // Cold War Germany
	"EDDK": EDDK, "EDDL": EDDL, "EDDV": EDDV, "EDDH": EDDH, // Cold War Germany
	"ORAA": ORAA, "ORSH": ORSH, "ORBR": ORBR, "ORBI": ORBI, // Iraq
	"ORBD": ORBD, "ORBB": ORBB, "ORER": ORER, "ORKK": ORKK, "ORSU": ORSU, // Iraq
	"LTAG": LTAG, "LLRD": LLRD, "OJMF": OJMF, "LTDA": LTDA, // Syria / Eastern Med
	"LTAJ": LTAJ, "LCRA": LCRA, "LCPH": LCPH, "OJHR": OJHR, // Syria / Eastern Med
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
	// Iraq is the 9 COMM1 recovery bases from the vSFG-7 Iraq presets card
	// (tower on UHF; only Al Asad/Al Sahra/Al Salam/Balad carry ATIS), in
	// launch order. Land-based — no carrier (Marshal/Deckboss).
	Iraq = []*Airfield{ORAA, ORSH, ORBR, ORBI, ORBD, ORBB, ORER, ORKK, ORSU}
	// Syria is the 8 recovery bases from the vSFG-7 "Hornet Radio Presets —
	// Eastern Med" card (tower on UHF), in launch order. Carrier ops ARE run on
	// this map: CVN-72 ABE, with Marshal/Deckboss/LSO on 306.100/306.200/128.100.
	// ⚠ H4 (OJHR) has no ILS/TACAN/VOR/NDB at all — its ATIS reports no aids.
	Syria = []*Airfield{LTAG, LLRD, OJMF, LTDA, LTAJ, LCRA, LCPH, OJHR}
)

// FieldsForMap returns the tower fields for a map name (default: Persian Gulf).
// Accepts "pg"/"persiangulf", "ca"/"caucasus"/"blacksea", "germany"/"de",
// "iraq"/"iq"/"or", and "syria"/"sy"/"easternmed".
//
// ⚠ THIS FAILS OPEN: an unrecognised name silently returns the Persian Gulf
// fields rather than erroring, so a typo in a start script gives you PG towers
// on PG frequencies with nothing in the log. Check the name reaches a case.
func FieldsForMap(name string) []*Airfield {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ca", "caucasus", "blacksea", "black-sea", "black sea":
		return Caucasus
	case "germany", "de", "coldwargermany", "cold-war-germany", "cwg":
		return Germany
	case "iraq", "iq", "or":
		return Iraq
	case "syria", "sy", "easternmed", "eastern-med", "eastern med", "emed":
		return Syria
	default:
		return PersianGulf
	}
}
