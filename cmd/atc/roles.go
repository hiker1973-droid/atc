package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	roleConfigResult.res, roleConfigResult.err = f.Apply(roleTheatre, rolecfg.OwnerOf(given), flagAirfield, pflagFlags{fs})

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
