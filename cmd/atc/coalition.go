package main

import (
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"

	"github.com/vsfg7/atc/pkg/airfield"
)

// The red side (operator 2026-10-22, Day, for the BFM Venom vs Raider night:
// "a red Command 280.00 ... Red will be using mozdok airfield", "Mozdok tower
// is 266", "tonight only and maybe used in the future but not all the time").
// start_red_caucasus.bat runs Red Command and Mozdok Tower with --coalition
// red. Every SRS client and ExternalAudio transmission was registered blue
// (coalition 2), so a server with coalition security on would never let a red
// pilot hear them.
var (
	flagCoalition     string
	flagCommandFields string
)

func registerCoalitionFlags(f *pflag.FlagSet) {
	f.StringVar(&flagCoalition, "coalition", "blue",
		"SRS coalition this process registers and transmits as: blue or red. Red Command also drops blue's tasking targets and range areas")
	f.StringVar(&flagCommandFields, "command-fields", "",
		"With --command-only: comma-separated ICAOs Command recovers and diverts jets to, instead of the --map's tower fields (red: XRMF)")
}

// redSide reports --coalition red.
func redSide() bool { return strings.EqualFold(strings.TrimSpace(flagCoalition), "red") }

// srsCoalition is the SRS coalition number: 1 red, 2 blue.
func srsCoalition() int {
	if redSide() {
		return 1
	}
	return 2
}

// srsCoalitionArg is srsCoalition as ExternalAudio's -c takes it.
func srsCoalitionArg() string {
	if redSide() {
		return "1"
	}
	return "2"
}

// commandFields are the fields Command hands recovering jets to, takes its
// magnetic variation from and offers for a divert: --command-fields when set,
// else the --map's tower fields.
func commandFields() []*airfield.Airfield {
	if strings.TrimSpace(flagCommandFields) == "" {
		return airfield.FieldsForMap(flagMap)
	}
	var out []*airfield.Airfield
	for _, icao := range strings.Split(flagCommandFields, ",") {
		if icao = strings.TrimSpace(icao); icao == "" {
			continue
		}
		if f := airfield.ByICAO(icao); f != nil {
			out = append(out, f)
		} else {
			log.Warn().Str("icao", icao).Msg("--command-fields: unknown airfield, skipped")
		}
	}
	if len(out) == 0 {
		return airfield.FieldsForMap(flagMap)
	}
	return out
}
