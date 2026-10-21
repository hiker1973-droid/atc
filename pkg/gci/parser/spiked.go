package parser

import (
	"github.com/vsfg7/atc/pkg/gci/internal/parser/token"
	"github.com/vsfg7/atc/pkg/gci/brevity"
)

func parseSpiked(callsign string, stream *token.Stream) (*brevity.SpikedRequest, bool) {
	bearing, ok := parseBearingOnly(stream)
	if !ok {
		return nil, false
	}
	return &brevity.SpikedRequest{
		Callsign: callsign,
		Bearing:  bearing,
	}, true
}
