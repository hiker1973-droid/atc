package main

import (
	"regexp"
	"strings"

	flag "github.com/spf13/pflag"
)

// Stations added on the launcher's Settings page (operator 2026-10-25: add
// "JTAC, COMMAND or other"): a second AWACS or a second Command runs as one
// more atc.exe beside the theatre's own, from start_added_<theatre>.bat, which
// the launcher writes. These flags keep the two apart:
//
//	--added-station gci2     the Settings id: its own single-instance key, and
//	                         roles.json's changes for the theatre's own
//	                         Command or AWACS are not applied to it
//	--log-name gci-gci2      logs\atc-gci-gci2.log, not the theatre's
//	--handoff-watch=false    a second Command leaves the tower handoffs to the
//	                         first, so pilots do not hear them twice
var (
	flagAddedStation string
	flagLogName      string
	flagHandoffWatch bool
)

func registerAddedFlags(f *flag.FlagSet) {
	f.StringVar(&flagAddedStation, "added-station", "", "Settings page id of a station added beside the theatre's own (its own instance, no roles.json changes)")
	f.StringVar(&flagLogName, "log-name", "", "Log file logs\\atc-<name>.log instead of the mode's own (letters, digits, dashes)")
	f.BoolVar(&flagHandoffWatch, "handoff-watch", true, "With --command-only and Tacview: call inbound jets over to the towers (off for a second Command)")
}

var reLogName = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

// logNameOr is --log-name when it is a safe file name part, else slug.
func logNameOr(slug string) string {
	if n := strings.ToLower(strings.TrimSpace(flagLogName)); reLogName.MatchString(n) {
		return n
	}
	return slug
}
