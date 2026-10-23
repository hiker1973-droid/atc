package main

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/license"
)

// checkLicence stops a role from starting once the trial has ended and there
// is no valid key (pkg/license). Checked here as well as in the launcher, so
// a start file run by hand stops too. A git checkout - the vSFG-7 rigs - is
// never checked.
func checkLicence() error {
	st := license.NewChecker(installDir()).Check()
	switch st.State {
	case license.Expired:
		log.Error().Msg(st.Message)
		return fmt.Errorf("%s", st.Message)
	case license.Trial:
		log.Info().Int("daysLeft", st.DaysLeft).Msg("trial")
	case license.Grace:
		log.Warn().Int("graceDaysLeft", st.DaysLeft).Msg(st.Message)
	case license.Licensed:
		log.Info().Str("squadron", st.Licence.Squadron).Str("id", st.Licence.ID).Msg("licensed")
	}
	return nil
}
