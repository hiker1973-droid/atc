package controller

import (
	"context"

	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/traces"
	"github.com/lithammer/shortuuid/v3"
	"github.com/martinlindhe/unit"
)

func (c *Controller) broadcastSunrise(ctx context.Context) {
	radioFrequencies := c.srsClient.Frequencies()
	frequencies := make([]unit.Frequency, 0, len(radioFrequencies))
	for _, rf := range radioFrequencies {
		frequencies = append(frequencies, rf.Frequency)
	}
	c.calls <- NewCall(traces.WithTraceID(ctx, shortuuid.New()), brevity.SunriseCall{Frequencies: frequencies})
}
