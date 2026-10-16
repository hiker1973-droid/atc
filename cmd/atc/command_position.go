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

// isRadarCheck: Marshal and the towers answer it; Command did not (DCS3
// 2026-10-15 14:51:49, open since 2026-10-14).
func isRadarCheck(lower string) bool {
	return containsAny(lower, "radar check", "request radar", "radar contact check")
}

func isTankerRequest(lower string) bool {
	return containsAny(lower, "request tanker", "say tanker", "where's the tanker", "where is the tanker", "wheres the tanker",
		"nearest tanker", "tanker location", "tanker bearing",
		"say texaco", "request texaco", "say arco", "request arco", "say shell", "request shell")
}

// commandReply is the full Command answer to a transcript: the Tacview-backed
// bullseye / tanker replies first, then the tasking exchange, then the
// text-only intents.
func commandReply(text, callsign, channelName string, store *tacviewPositions) string {
	if resp := wrongRadioMarshal(text, callsign, channelName); resp != "" {
		return resp
	}
	if resp := commandPositionResponse(text, callsign, channelName, store, nearestFieldMagVar); resp != "" {
		return resp
	}
	if fb := footholdTaskingBoard(); fb != nil {
		if resp := fb.Response(text, callsign, channelName, store); resp != "" {
			return resp
		}
	}
	if resp := commandTaskingBoard().Response(text, callsign, channelName); resp != "" {
		return resp
	}
	return commandResponse(text, callsign, channelName)
}

// commandPositionResponse answers "say bullseye" and "request tanker" from the
// Tacview store. Returns "" for any other call. When the picture can't answer
// it says so rather than going silent, because the pilot asked a question.
func commandPositionResponse(text, callsign, channelName string, store *tacviewPositions, magVar magVarFn) string {
	lower := strings.ToLower(text)
	wantBull, wantTanker, wantRadar := isBullseyeRequest(lower), isTankerRequest(lower), isRadarCheck(lower)
	if !wantBull && !wantTanker && !wantRadar {
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
	if wantRadar {
		angels := int(store.Info(callsign).AltFt/1000 + 0.5)
		if !ok {
			return fmt.Sprintf("%s, %s, radar contact, angels %d.", callsign, channelName, angels)
		}
		brg, nm := bearingRangeFromRef(bull, pos[0], pos[1])
		return fmt.Sprintf("%s, %s, radar contact, bullseye %s for %d, angels %d.",
			callsign, channelName, spokenBearing(toMagnetic(brg, magVar(bull))), int(nm+0.5), angels)
	}
	if !ok {
		return fmt.Sprintf("%s, %s, unable, no bullseye on scope.", callsign, channelName)
	}
	brg, nm := bearingRangeFromRef(bull, pos[0], pos[1])
	return fmt.Sprintf("%s, %s, bullseye %s for %d.",
		callsign, channelName, spokenBearing(toMagnetic(brg, magVar(bull))), int(nm+0.5))
}

// marshalAddresses open a call meant for Marshal (Whisper's spellings too).
var marshalAddresses = []string{"union marshall", "union marshal", "unit marshall", "marshall", "marshal"}

// wrongRadioMarshal answers a call that opens with Marshal's address on
// Command's frequency with where Marshal is (operator ruling 2026-10-15, after
// Raider 302 called "Union Marshall" on 282 at 14:56). "" when the call is
// not addressed to Marshal, or Command has no Marshal frequency.
func wrongRadioMarshal(text, callsign, channelName string) string {
	if flagHandoffMarshalFreq <= 0 {
		return ""
	}
	lower := strings.TrimLeft(strings.ToLower(text), " ,.")
	for _, a := range marshalAddresses {
		if strings.HasPrefix(lower, a) && (len(lower) == len(a) || strings.ContainsRune(" ,.", rune(lower[len(a)]))) {
			who := callsign
			if who == "" {
				who = "Station calling"
			}
			return fmt.Sprintf("%s, %s, you are on Command. You need to tune to %s, %s.", who, channelName,
				flagHandoffMarshalName, formatFreqMHz(flagHandoffMarshalFreq))
		}
	}
	return ""
}
