package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/paulmach/orb"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Command replies that need the Tacview picture: the pilot's bullseye and the
// nearest tanker. Kept apart from commandResponse, which is pure text.

// magVarFn returns magnetic variation in degrees (east positive) at a point.
// Bearings on the radio are magnetic; Tacview positions give true.
type magVarFn func(orb.Point) float64

// nearestFieldMagVar uses the closest airfield on the current map as the local
// variation. Good to a degree or two across a theatre, which is inside what a
// spoken three-digit bearing resolves anyway.
func nearestFieldMagVar(p orb.Point) float64 {
	best, bestNm := 0.0, math.MaxFloat64
	for _, fld := range airfield.FieldsForMap(flagMap) {
		if d := haversineNm(p, fld.Center); d < bestNm {
			best, bestNm = fld.MagVar, d
		}
	}
	return best
}

func toMagnetic(trueDeg, magVar float64) float64 {
	return math.Mod(trueDeg-magVar+720, 360)
}

func isBullseyeRequest(lower string) bool {
	return containsAny(lower, "say bullseye", "request bullseye", "bullseye check", "say my bullseye", "request my bullseye", "bullseye position")
}

func isTankerRequest(lower string) bool {
	return containsAny(lower, "request tanker", "say tanker", "where's the tanker", "where is the tanker", "wheres the tanker",
		"nearest tanker", "tanker location", "tanker bearing",
		"say texaco", "request texaco", "say arco", "request arco", "say shell", "request shell")
}

// commandReply is the full Command answer to a transcript: the Tacview-backed
// bullseye / tanker replies first, then the text-only intents.
func commandReply(text, callsign, channelName string, store *tacviewPositions) string {
	if resp := commandPositionResponse(text, callsign, channelName, store, nearestFieldMagVar); resp != "" {
		return resp
	}
	return commandResponse(text, callsign, channelName)
}

// commandPositionResponse answers "say bullseye" and "request tanker" from the
// Tacview store. Returns "" for any other call. When the picture can't answer
// it says so rather than going silent, because the pilot asked a question.
func commandPositionResponse(text, callsign, channelName string, store *tacviewPositions, magVar magVarFn) string {
	lower := strings.ToLower(text)
	wantBull, wantTanker := isBullseyeRequest(lower), isTankerRequest(lower)
	if !wantBull && !wantTanker {
		return ""
	}
	if store == nil {
		return fmt.Sprintf("%s, %s, unable, no radar picture.", callsign, channelName)
	}
	pos, ok := store.Get(callsign)
	if !ok {
		return fmt.Sprintf("%s, %s, unable, no radar contact, say position.", callsign, channelName)
	}
	coalition := store.Info(callsign).Coalition

	if wantTanker {
		tk, ok := store.NearestTanker(pos, coalition)
		if !ok {
			return fmt.Sprintf("%s, %s, negative tanker on scope.", callsign, channelName)
		}
		brg, nm := bearingRangeFromRef(pos, tk.pos[0], tk.pos[1])
		return fmt.Sprintf("%s, %s, %s bears %s for %d, angels %d.",
			callsign, channelName, tk.callsign,
			spokenBearing(toMagnetic(brg, magVar(pos))), int(nm+0.5), int(tk.info.AltFt/1000+0.5))
	}

	bull, ok := store.Bullseye(coalition)
	if !ok {
		return fmt.Sprintf("%s, %s, unable, no bullseye on scope.", callsign, channelName)
	}
	brg, nm := bearingRangeFromRef(bull, pos[0], pos[1])
	return fmt.Sprintf("%s, %s, bullseye %s for %d.",
		callsign, channelName, spokenBearing(toMagnetic(brg, magVar(bull))), int(nm+0.5))
}
