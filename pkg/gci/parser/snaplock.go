package parser

import (
	"github.com/vsfg7/atc/pkg/gci/internal/parser/token"
	"github.com/vsfg7/atc/pkg/gci/brevity"
)

func parseSnaplock(callsign string, stream *token.Stream) (*brevity.SnaplockRequest, bool) {
	bra, ok := parseBRA(stream)
	if !ok {
		return nil, false
	}
	return &brevity.SnaplockRequest{
		Callsign: callsign,
		BRA:      bra,
	}, true
}
