package composer

import (
	"github.com/vsfg7/atc/pkg/gci/brevity"
)

// ComposeSpikedResponse constructs natural language brevity for responding to a SPIKED call.
func (c *Composer) ComposeSpikedResponse(response brevity.SpikedResponseV2) NaturalLanguageResponse {
	return c.composeCorrelation("spike", response.Callsign, response.Status, response.Bearing, response.Group)
}
