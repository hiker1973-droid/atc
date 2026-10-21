package parser

import (
	"github.com/vsfg7/atc/pkg/gci/internal/parser/token"
	"github.com/vsfg7/atc/pkg/gci/brevity"
)

func parseStrobe(callsign string, stream *token.Stream) (*brevity.StrobeRequest, bool) {
	bearing, ok := parseBearingOnly(stream)
	if !ok {
		return nil, false
	}
	return &brevity.StrobeRequest{
		Callsign: callsign,
		Bearing:  bearing,
	}, true
}
