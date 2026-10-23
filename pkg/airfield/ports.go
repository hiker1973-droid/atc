package airfield

// DashboardPorts maps an airfield ICAO to the dashboard port of the tower
// process that owns that airfield's authoritative state. ATIS polls it to
// keep the broadcast runway aligned with the tower's (the two processes
// do not share a controller), the launcher's dashboard shows it, and the
// launcher's generated start files give each tower its port from here.
var DashboardPorts = map[string]int{
	// Persian Gulf towers
	"OMDM": 6001,
	"OMAM": 6002,
	"OMAL": 6003,
	// Caucasus / Black Sea towers (own range — 6004/6005 are Marshal/Deckboss)
	"UGSB": 6011,
	"UG5X": 6012,
	"UGKS": 6013,
	"UGKO": 6014,
	"UGTB": 6015,
	"XRMF": 6016, // Mozdok, blue for Training (2026-10-23)
	// Cold War Germany towers (8 recovery bases, own range 6021-6028)
	"ETAR": 6021,
	"ETAD": 6022,
	"EDFH": 6023,
	"EDDF": 6024,
	"EDDK": 6025,
	"EDDL": 6026,
	"EDDV": 6027,
	"EDDH": 6028,
	// Iraq towers (9 recovery bases, own range 6031-6039). Only the four with
	// a dedicated ATIS station (ORAA/ORSH/ORBR/ORBD) are actually polled by an
	// ATIS loop; the rest are tower-only but keep a port for the dashboard.
	"ORAA": 6031,
	"ORSH": 6032,
	"ORBR": 6033,
	"ORBI": 6034,
	"ORBD": 6035,
	"ORBB": 6036,
	"ORER": 6037,
	"ORKK": 6038,
	"ORSU": 6039,
	// Syria / Eastern Med towers (8 recovery bases, own range 6041-6048).
	// All eight carry an ATIS: five from the squadron card, three assigned
	// (Hatay/Gaziantep/Akrotiri, which the card gives tower-only).
	"LTAG": 6041,
	"LLRD": 6042,
	"OJMF": 6043,
	"LTDA": 6044,
	"LTAJ": 6045,
	"LCRA": 6046,
	"LCPH": 6047,
	"OJHR": 6048,
	// Divert fields (not on the card), added 2026-09-19.
	"OSLK": 6049,
	"OLBA": 6050,
	// Kola towers (Operation WHITE NIGHT, own range 6051-6054). Only the field in
	// play is started -- see start_towers_kola.bat.
	"ENBO": 6051,
	"ENDU": 6052,
	"ENNA": 6053,
	"EFRO": 6054,
}
