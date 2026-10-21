package controller

import (
	"context"

	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/rs/zerolog/log"
)

// HandleSpiked handles a SPIKED request by reporting any enemy groups in the direction of the radar spike.
func (c *Controller) HandleSpiked(ctx context.Context, request *brevity.SpikedRequest) {
	logger := log.With().Str("callsign", request.Callsign).Type("type", request).Float64("bearing", request.Bearing.Degrees()).Logger()
	correlation := c.correlate(logger, request.Callsign, request.Bearing)
	if correlation.Callsign == "" {
		c.calls <- NewCall(ctx, brevity.NegativeRadarContactResponse{Callsign: request.Callsign})
	} else {
		response := brevity.SpikedResponseV2{
			Callsign: correlation.Callsign,
			Status:   correlation.Group != nil,
			Bearing:  correlation.Bearing,
			Group:    correlation.Group,
		}
		c.calls <- NewCall(ctx, response)
	}
}
