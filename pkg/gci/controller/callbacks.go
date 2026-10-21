package controller

import (
	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/coalitions"
	"github.com/vsfg7/atc/pkg/gci/traces"
	"github.com/vsfg7/atc/pkg/gci/trackfiles"
	"github.com/martinlindhe/unit"
	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

var fadeBroadcastRadius = 55 * unit.NauticalMile

func (c *Controller) handleStarted() {
	c.merges.reset()
	c.threatCooldowns.reset()
	c.mergeCooldowns.reset()
	c.wasLastPictureClean = false
}

func (c *Controller) handleFaded(location orb.Point, group brevity.Group, coalition coalitions.Coalition) {
	for _, id := range group.ObjectIDs() {
		c.remove(id)
	}
	isHostile := coalition == c.coalition.Opposite()
	areHumansOnFrequency := c.srsClient.HumansOnFrequency() > 0
	nearbyFriendlies := c.scope.FindNearbyGroupsWithBullseye(
		location,
		lowestAltitude,
		highestAltitude,
		fadeBroadcastRadius,
		c.coalition,
		brevity.Aircraft,
		[]uint64{},
	)
	isNearFriendly := len(nearbyFriendlies) > 0

	if isHostile && isNearFriendly && areHumansOnFrequency {
		log.Info().Stringer("group", group).Msg("broadcasting FADED call")
		group.SetDeclaration(brevity.Hostile)
		c.calls <- NewCall(traces.NewRequestContext(), brevity.FadedCall{Group: group})
	} else {
		log.Debug().
			Bool("isHostile", isHostile).
			Bool("isNearFriendly", isNearFriendly).
			Bool("areHumansOnFrequency", areHumansOnFrequency).
			Msg("skipping FADED call because broadcast criteria are not met")
	}
}

func (c *Controller) handleRemoved(trackfile *trackfiles.Trackfile) {
	c.remove(trackfile.Contact.ID)
}

func (c *Controller) remove(id uint64) {
	log.Debug().Uint64("id", id).Msg("removing ID from controller state tracking")
	c.threatCooldowns.remove(id)
	c.mergeCooldowns.remove(id)
	c.merges.remove(id)
}
