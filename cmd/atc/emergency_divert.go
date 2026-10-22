package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Damaged-jet divert (operator 2026-10-21): "If you report to AWACS or
// Command you are hit / damaged can it have them tell the player which
// emergency landing spot is available (miles, heading etc.)". The nearest
// field on the map is given as a magnetic bearing and range from the jet,
// with its tower frequency.

// damagedRe: "I'm hit", "took a hit", "battle damage",
// "damaged", "mayday", "pan pan", "emergency", "engine fire", "single
// engine", "lost an engine", "hydraulic failure", "fuel leak", "request
// nearest field" / "divert". "Hitman" is not "hit" (\b). A BDA request
// ("battle damage assessment") and "target is hit / damaged" are effects on
// the target, not on the jet.
var damagedRe = regexp.MustCompile(`(?i)\b(?:(?:i'?m|i am|we'?re|we are|been|took|taken|taking)\s+(?:a\s+|some\s+)?hits?|hit\s+hard|battle\s+damage|damaged|taking\s+damage|mayday|pan[\s-]pan|emergency|engine\s+fire|fire\s+light|single[\s-]engine|lost\s+(?:\w+\s+){0,2}engines?|engine\s+(?:out|failure|failed)|hydraulic|fuel\s+leak|leaking\s+fuel|nearest\s+(?:air)?field|nearest\s+(?:base|runway|airport)|request\s+divert|divert\s+field)\b`)

// isDamagedCall: a pilot reporting damage or asking for the nearest field.
func isDamagedCall(lower string) bool {
	if containsAny(lower, "bda", "b.d.a", "b d a", "assessment", "say effects", "request effects",
		"target is hit", "target hit", "target damaged", "target is damaged") {
		return false
	}
	return damagedRe.MatchString(lower)
}

// divertField is the nearest field to pos and its true bearing and range.
func divertField(pos orb.Point, fields []*airfield.Airfield) (fld *airfield.Airfield, brgTrue, nm float64) {
	nm = math.MaxFloat64
	for _, f := range fields {
		if d := haversineNm(pos, f.Center); d < nm {
			fld, nm = f, d
		}
	}
	if fld != nil {
		brgTrue = initialBearing(pos, fld.Center)
	}
	return fld, brgTrue, nm
}

// initialBearing is the true great-circle bearing from a to b, degrees.
func initialBearing(a, b orb.Point) float64 {
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	lat1, lat2 := toRad(a[1]), toRad(b[1])
	dLon := toRad(b[0] - a[0])
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

// divertReply answers a damaged jet: "Raider 302, Darkstar, copy. Nearest
// field Kobuleti, bearing 0 9 5, 42 miles, elevation 59. Kobuleti tower two
// six two point zero, declare your emergency." Without a position it asks
// for one. who is the callsign as this station says it; station its name.
func divertReply(who, station string, pos orb.Point, havePos bool, fields []*airfield.Airfield) string {
	head := fmt.Sprintf("%s, %s, copy.", who, station)
	if !havePos {
		return head + " No radar contact, say position, and the nearest field."
	}
	fld, brg, nm := divertField(pos, fields)
	if fld == nil {
		return head + " No friendly field available, say intentions."
	}
	resp := fmt.Sprintf("%s Nearest field %s, bearing %s, %d miles, elevation %d.", head, fld.Name,
		spokenBearing(toMagnetic(brg, fld.MagVar)), int(nm+0.5), fld.ElevationFt)
	if fld.TowerFreqMHz > 0 {
		resp += fmt.Sprintf(" %s tower %s, declare your emergency.", fld.Name, spokenFreq1(fld.TowerFreqMHz))
	}
	return resp
}

// divertSave reads the Foothold campaign save for who holds each field.
var divertSave = &footholdSave{paths: footholdSavePaths}

// divertFields is the map's fields a damaged jet may be sent to: on Foothold
// (Syria) those the red side holds are left out (operator 2026-10-21: "yes
// for sure"). Without a readable save every field is offered.
func divertFields() []*airfield.Airfield {
	fields := airfield.FieldsForMap(flagMap)
	if !strings.EqualFold(strings.TrimSpace(flagMap), "syria") {
		return fields
	}
	zones, err := divertSave.Zones()
	if len(zones) == 0 {
		if err != nil {
			log.Debug().Err(err).Msg("divert: no Foothold save - every field offered")
		}
		return fields
	}
	return withoutRedFields(fields, zones)
}

// withoutRedFields drops each field whose Foothold zone is red.
func withoutRedFields(fields []*airfield.Airfield, zones []footholdZone) []*airfield.Airfield {
	var out []*airfield.Airfield
	for _, f := range fields {
		if z := fieldZone(f, zones); z != nil && z.Side == footholdSideRed {
			continue
		}
		out = append(out, f)
	}
	return out
}

// fieldZone is the Foothold zone for a field: the one named after it (zones
// carry the field's name, "Bassel Al-Assad"), else one within 5 nm of it.
func fieldZone(f *airfield.Airfield, zones []footholdZone) *footholdZone {
	for i := range zones {
		if strings.EqualFold(zones[i].Name, f.Name) || (f.DCSName != "" && strings.EqualFold(zones[i].Name, f.DCSName)) {
			return &zones[i]
		}
	}
	for i := range zones {
		if zones[i].Pos != (orb.Point{}) && haversineNm(zones[i].Pos, f.Center) <= 5 {
			return &zones[i]
		}
	}
	return nil
}
