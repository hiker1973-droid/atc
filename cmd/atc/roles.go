package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/rolecfg"
)

// Roles from the launcher's Settings page: roles.json next to atc.exe
// (pkg/rolecfg). It changes this process's flags before anything reads them,
// as if the bat line had said so.

var (
	roleCfg     = rolecfg.File{Theatres: map[string]rolecfg.Theatre{}}
	roleTheatre string
	rolesPath   = func() string {
		exe, err := os.Executable()
		if err != nil {
			return "roles.json"
		}
		return filepath.Join(filepath.Dir(exe), "roles.json")
	}
)

// pflagFlags adapts a FlagSet to rolecfg.Flags.
type pflagFlags struct{ fs *pflag.FlagSet }

func (p pflagFlags) Get(name string) (string, bool) {
	if f := p.fs.Lookup(name); f != nil {
		return f.Value.String(), true
	}
	return "", false
}
func (p pflagFlags) Set(name, value string) error { return p.fs.Set(name, value) }

// roleConfigResult is kept for the log, which starts after the flags are read.
var roleConfigResult struct {
	res rolecfg.Result
	err error
}

// applyRoleConfig reads roles.json and applies this process's changes. It
// runs first thing, before the logger exists; logRoleConfig reports it.
func applyRoleConfig(fs *pflag.FlagSet) {
	if flagSay != "" {
		return // the Setup test call is not a role
	}
	given := map[string]string{}
	fs.Visit(func(f *pflag.Flag) { given[f.Name] = f.Value.String() })
	if fs.Changed("map") {
		roleTheatre = airfield.TheatreKey(flagMap)
	} else {
		roleTheatre = airfield.TheatreOfICAO(flagAirfield)
	}
	f, err := rolecfg.Read(rolesPath())
	if err != nil {
		roleConfigResult.err = fmt.Errorf("roles.json unreadable - bat settings kept: %w", err)
		return
	}
	roleCfg = f
	if flagAddedStation != "" {
		// Added on the Settings page: its bat line is written from there whole,
		// and the theatre's own Command / AWACS changes are not its.
		return
	}
	owner := rolecfg.OwnerOf(given)
	roleConfigResult.res, roleConfigResult.err = f.Apply(roleTheatre, owner, flagAirfield, pflagFlags{fs})

	// Command's tankers beyond the two catalog slots: the bat's --tanker-net
	// lines with the Settings changes, and the tankers added there.
	if owner == rolecfg.OwnerCommand {
		nets := f.TankerNets(roleTheatre, flagTankerNets)
		if strings.Join(nets, "|") != strings.Join(flagTankerNets, "|") {
			flagTankerNets = nets
			roleConfigResult.res.Changed = append(roleConfigResult.res.Changed, "tanker-net="+strings.Join(nets, ","))
		}
		// JTACs added there, beside Hitman 1-1 and 2-1.
		if j := f.JTACNets(roleTheatre); len(j) > 0 {
			flagJTACNets = append(flagJTACNets, j...)
			roleConfigResult.res.Changed = append(roleConfigResult.res.Changed, "jtac-net="+strings.Join(j, ","))
		}
	}

	// A red tower hands off to Red Command wherever Settings put it.
	if o, ok := f.Station(roleTheatre, "red-command"); ok {
		if o.Freq > 0 {
			redCommandFreqMHz = o.Freq
		}
		if o.Name != "" {
			redCommandName = strings.ReplaceAll(o.Name, "-", " ")
		}
	}
}

// logRoleConfig reports what applyRoleConfig did; a role switched off in
// Settings stops here.
func logRoleConfig() error {
	r := roleConfigResult
	if r.err != nil {
		log.Warn().Err(r.err).Msg("roles.json")
	}
	if len(r.res.Changed) > 0 {
		log.Info().Str("theatre", roleTheatre).Strs("flags", r.res.Changed).Msg("Settings page changes applied (roles.json)")
	}
	if r.res.Off {
		log.Warn().Str("theatre", roleTheatre).Msg("this role is turned off on the Settings page - exiting")
		return fmt.Errorf("turned off in Settings (roles.json)")
	}
	return nil
}

// applyATISRoles puts the Settings page's ATIS changes on the theatre's
// stations: renamed, revoiced, taken off the air, and a field that has no ATIS
// of its own given one (its weather, runway and altimeter; no TACAN or ILS,
// which only the built-in stations carry).
func applyATISRoles(stations []*atisStation) []*atisStation {
	set := roleCfg.Theatres[roleTheatre].ATIS
	if len(set) == 0 {
		return stations
	}
	advisory := "vSFG-7 training flights in area. Advise information on initial contact."
	if len(stations) > 0 {
		advisory = stations[0].Advisory
	}
	kept := stations[:0:0]
	have := map[string]bool{}
	for _, st := range stations {
		o := set[st.ICAO]
		if o.Off {
			log.Info().Str("icao", st.ICAO).Msg("ATIS off on the Settings page")
			continue
		}
		if o.Name != "" {
			st.Name = o.Name
		}
		if o.Voice != "" {
			st.Voice = o.Voice
		}
		have[st.ICAO] = true
		kept = append(kept, st)
	}
	icaos := make([]string, 0, len(set))
	for icao := range set {
		icaos = append(icaos, icao)
	}
	sort.Strings(icaos)
	for _, icao := range icaos {
		o := set[icao]
		af := airfield.ByICAO(icao)
		if !o.Added || o.Off || have[icao] || af == nil || !airfield.ValidFreqMHz(o.Freq) || airfield.TheatreOfICAO(icao) != roleTheatre {
			continue
		}
		name, voice := o.Name, o.Voice
		if name == "" {
			name = af.Name + " ATIS"
		}
		if voice == "" {
			voice = "alloy"
		}
		af.ATISFreqMHz = o.Freq // the tower's "information" calls name it
		kept = append(kept, &atisStation{Name: name, FreqMHz: o.Freq, Voice: voice, ICAO: icao, Advisory: advisory})
		log.Info().Str("icao", icao).Float64("freq", o.Freq).Msg("ATIS added on the Settings page")
	}
	return kept
}

// commandHandoffFollows points the theatre's towers that hand off to Command
// at Command's frequency from Settings, unless the field's own handoff was set
// in the Frequencies table.
func commandHandoffFollows() {
	o, ok := roleCfg.Station(roleTheatre, "command")
	if !ok || o.Off || o.Freq <= 0 {
		return
	}
	for _, af := range airfield.FieldsForMap(roleTheatre) {
		if strings.EqualFold(af.HandoffCallsign, "command") && freqOverrides.Fields[af.ICAO].Handoff == 0 {
			af.HandoffFreqMHz = o.Freq
		}
	}
}
