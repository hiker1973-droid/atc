// Package composer generates ATC phraseology responses.
// All output follows ICAO phraseology standards with DCS/NATO military adaptations.
// Each response type has 3 unique variations selected randomly for realism.
package composer

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

// ATCComposer generates ATC clearance text for a specific tower callsign.
type ATCComposer struct {
	towerCallsign string
}

// NewATCComposer creates a composer for the given tower callsign.
func NewATCComposer(towerCallsign string) *ATCComposer {
	return &ATCComposer{towerCallsign: towerCallsign}
}

// pick selects one string from a slice at random.
func pick(opts []string) string {
	return opts[rand.Intn(len(opts))]
}

// StartupApproval approves engine start — 3 variations. Uses Ground callsign
// (derived from tower) since startup is conventionally a ground-control task.
func (c *ATCComposer) StartupApproval(callsign string, altimeterInHg float64) string {
	ground := strings.Replace(c.towerCallsign, "Tower", "Ground", 1)
	alt := formatAltimeter(altimeterInHg)
	return pick([]string{
		fmt.Sprintf("%s, %s, startup approved.", callsign, ground),
		fmt.Sprintf("%s, %s, startup approved, altimeter %s, advise ready to taxi.", callsign, ground, alt),
		fmt.Sprintf("%s, %s, startup at your discretion, advise when ready to taxi.", callsign, ground),
	})
}

// RadioCheck responds to a radio check — 3 variations.
func (c *ATCComposer) RadioCheck(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, loud and clear.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, five by five, go ahead.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, reading you loud and clear.", callsign, c.towerCallsign),
	})
}

// TaxiClearance issues taxi instructions — 3 variations.
func (c *ATCComposer) TaxiClearance(callsign, activeRunway string, altimeterInHg float64) string {
	alt := formatAltimeter(altimeterInHg)
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, %s, taxi to runway %s, altimeter %s, hold short, advise ready.", callsign, c.towerCallsign, rwy, alt),
		fmt.Sprintf("%s, %s, altimeter %s, cleared to taxi runway %s, hold short and call ready.", callsign, c.towerCallsign, alt, rwy),
		fmt.Sprintf("%s, %s, roger, altimeter %s, taxi runway %s, hold short of the runway, advise when ready for takeoff.", callsign, c.towerCallsign, alt, rwy),
	})
}

// HoldShort tells a pilot to hold short — 3 variations.
func (c *ATCComposer) HoldShort(callsign, activeRunway string) string {
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, %s, hold short runway %s, number one, advise ready.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, hold short of runway %s, standby.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, hold your position short of runway %s, you are next for departure.", callsign, c.towerCallsign, rwy),
	})
}

// HoldShortTraffic holds departure due to inbound — 3 variations.
func (c *ATCComposer) HoldShortTraffic(callsign, activeRunway, trafficCallsign string, trafficDistNm float64) string {
	rwy := spellRunway(activeRunway)
	dist := int(math.Round(trafficDistNm))
	return pick([]string{
		fmt.Sprintf("%s, %s, hold short runway %s, %s is %d miles on final.", callsign, c.towerCallsign, rwy, trafficCallsign, dist),
		fmt.Sprintf("%s, %s, hold position runway %s, traffic on %d mile final.", callsign, c.towerCallsign, rwy, dist),
		fmt.Sprintf("%s, %s, hold short, %d mile final traffic, %s, will advise.", callsign, c.towerCallsign, dist, trafficCallsign),
	})
}

// LineUpAndWait enters aircraft onto runway without takeoff clearance — 3 variations.
func (c *ATCComposer) LineUpAndWait(callsign, activeRunway, trafficCallsign string, trafficDistNm float64) string {
	rwy := spellRunway(activeRunway)
	dist := int(math.Round(trafficDistNm))
	return pick([]string{
		fmt.Sprintf("%s, %s, runway %s, line up and wait, %s is %d miles final.", callsign, c.towerCallsign, rwy, trafficCallsign, dist),
		fmt.Sprintf("%s, %s, line up and wait runway %s, traffic %d miles on final.", callsign, c.towerCallsign, rwy, dist),
		fmt.Sprintf("%s, %s, pull forward, runway %s, line up and wait, %d mile final %s.", callsign, c.towerCallsign, rwy, dist, trafficCallsign),
	})
}

// ClearedForTakeoff issues takeoff clearance — 3 variations.
func (c *ATCComposer) ClearedForTakeoff(callsign, activeRunway string, windFromMag, windKts float64, trafficOnFinal int) string {
	rwy := spellRunway(activeRunway)
	wind := formatWind(windFromMag, windKts)
	trafficNote := ""
	if trafficOnFinal > 0 {
		trafficNote = fmt.Sprintf(", traffic on final")
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, wind %s, runway %s%s, cleared for takeoff.", callsign, c.towerCallsign, wind, rwy, trafficNote),
		fmt.Sprintf("%s, %s, runway %s, wind %s%s, you are cleared for takeoff.", callsign, c.towerCallsign, rwy, wind, trafficNote),
		fmt.Sprintf("%s, %s, cleared for takeoff runway %s, wind %s%s, have a good flight.", callsign, c.towerCallsign, rwy, wind, trafficNote),
	})
}

// DepartureRelease confirms post-takeoff departure — 3 variations.
// The angels arg is ignored; we randomize 5–7 per call so successive
// departures don't all get assigned the same altitude. Pilot is told to
// call tower back at distNm DME for the Command handoff.
func (c *ATCComposer) DepartureRelease(callsign string, distNm, angels int) string {
	dist := numberWord(distNm)
	ang := numberWord(5 + rand.Intn(3))
	return pick([]string{
		fmt.Sprintf("%s, %s, proceed to angels %s, contact tower at %s DME.", callsign, c.towerCallsign, ang, dist),
		fmt.Sprintf("%s, %s, climb to angels %s, contact tower at %s DME.", callsign, c.towerCallsign, ang, dist),
		fmt.Sprintf("%s, %s, angels %s, contact tower at %s DME.", callsign, c.towerCallsign, ang, dist),
	})
}

// HandoffToCommand issues frequency change to command net — 3 variations.
func (c *ATCComposer) HandoffToCommand(callsign, handoffCallsign string, freqMHz float64, preset string) string {
	freq := spellFrequency(freqMHz)
	return pick([]string{
		fmt.Sprintf("%s, %s, contact %s, %s, %s. Good day.", callsign, c.towerCallsign, handoffCallsign, freq, preset),
		fmt.Sprintf("%s, %s, switch to %s, %s, %s. Safe skies.", callsign, c.towerCallsign, handoffCallsign, freq, preset),
		fmt.Sprintf("%s, %s, frequency change approved, %s on %s, %s.", callsign, c.towerCallsign, handoffCallsign, freq, preset),
	})
}

// PushingCommandAck answers a pilot-initiated "pushing command" call — short
// courtesy clearance, no need to re-issue freq/preset since the pilot is
// already switching. 3 variations.
func (c *ATCComposer) PushingCommandAck(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, cleared handoff to command, good day.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, roger pushing command, good day.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, copy switch to command, good day.", callsign, c.towerCallsign),
	})
}

// DistanceInitialAck acknowledges inbound at distance — 3 variations.
func (c *ATCComposer) DistanceInitialAck(callsign string, distNm int, activeRunway string, patternAltFt int, altimeterInHg float64, trafficAhead int) string {
	rwy := spellRunway(activeRunway)
	seq := numberWord(trafficAhead + 1)
	var traffic string
	switch trafficAhead {
	case 0:
		traffic = "Number one."
	default:
		traffic = fmt.Sprintf("Number %s.", seq)
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, runway %s, %s", callsign, c.towerCallsign, rwy, traffic),
		fmt.Sprintf("%s, %s, runway %s in use, %s", callsign, c.towerCallsign, rwy, traffic),
		fmt.Sprintf("%s, %s, roger, runway %s, %s", callsign, c.towerCallsign, rwy, traffic),
	})
}

// OverheadAck sequences a pilot overhead — 3 variations.
func (c *ATCComposer) OverheadAck(callsign, activeRunway, breakDirection string, trafficAhead int) string {
	rwy := spellRunway(activeRunway)
	seq := numberWord(trafficAhead + 1)
	brk := breakDirection
	if brk != "left" && brk != "right" {
		brk = "left"
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, approved %s break runway %s, number %s, report base, final.", callsign, c.towerCallsign, brk, rwy, seq),
		fmt.Sprintf("%s, %s, number %s, runway %s, %s break approved, report base, final.", callsign, c.towerCallsign, seq, rwy, brk),
		fmt.Sprintf("%s, %s, roger initial, %s break approved runway %s, number %s, report base, final.", callsign, c.towerCallsign, brk, rwy, seq),
	})
}

// OverheadBreakAck acknowledges pilot overhead for VFR break — 3 variations.
func (c *ATCComposer) OverheadBreakAck(callsign, activeRunway string, trafficAhead int) string {
	seq := numberWord(trafficAhead + 1)
	if trafficAhead == 0 {
		return pick([]string{
			fmt.Sprintf("%s, %s, number %s, report break.", callsign, c.towerCallsign, seq),
			fmt.Sprintf("%s, %s, you are number %s, break when ready, report break.", callsign, c.towerCallsign, seq),
			fmt.Sprintf("%s, %s, cleared break runway %s, report break.", callsign, c.towerCallsign, spellRunway(activeRunway)),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, number %s, follow traffic in the break, report break.", callsign, c.towerCallsign, seq),
		fmt.Sprintf("%s, %s, number %s, traffic in the break ahead of you, report break.", callsign, c.towerCallsign, seq),
		fmt.Sprintf("%s, %s, you are number %s, follow the break, report when you break.", callsign, c.towerCallsign, seq),
	})
}

// BreakAck acknowledges the overhead break — 3 variations.
func (c *ATCComposer) BreakAck(callsign, activeRunway string, trafficAhead int) string {
	rwy := spellRunway(activeRunway)
	seq := numberWord(trafficAhead + 1)
	if trafficAhead == 0 {
		return pick([]string{
			fmt.Sprintf("%s, %s, number %s, report downwind.", callsign, c.towerCallsign, seq),
			fmt.Sprintf("%s, %s, roger break, number %s, report downwind.", callsign, c.towerCallsign, seq),
			fmt.Sprintf("%s, %s, break acknowledged, report downwind runway %s.", callsign, c.towerCallsign, rwy),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, number %s for runway %s, report downwind.", callsign, c.towerCallsign, seq, rwy),
		fmt.Sprintf("%s, %s, roger, number %s, follow traffic, report downwind.", callsign, c.towerCallsign, seq),
		fmt.Sprintf("%s, %s, break acknowledged, you are number %s, report downwind.", callsign, c.towerCallsign, seq),
	})
}

// DownwindAck acknowledges downwind call — 3 variations.
func (c *ATCComposer) DownwindAck(callsign, activeRunway string, trafficAhead int) string {
	seq := numberWord(trafficAhead + 1)
	return pick([]string{
		fmt.Sprintf("%s, %s, number %s, report base.", callsign, c.towerCallsign, seq),
		fmt.Sprintf("%s, number %s, report base.", callsign, seq),
		fmt.Sprintf("%s, %s, number %s, call base.", callsign, c.towerCallsign, seq),
	})
}

// ExtendDownwind extends downwind for spacing — 3 variations.
func (c *ATCComposer) ExtendDownwind(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, extend downwind, maintain altitude, I'll call your base.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, hold your downwind, traffic ahead on final, stand by for base.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, continue downwind and hold altitude, spacing with traffic ahead, I'll call your base.", callsign, c.towerCallsign),
	})
}

// BaseAck acknowledges base call — 3 variations.
func (c *ATCComposer) BaseAck(callsign, activeRunway string, seqNum int) string {
	// Extract modex from callsign (last word, e.g. "Raider 032" → "032")
	modex := callsign
	if parts := strings.Fields(callsign); len(parts) > 1 {
		modex = parts[len(parts)-1]
	}
	return pick([]string{
		fmt.Sprintf("%s, affirmative.", modex),
		fmt.Sprintf("%s, %s, affirmative.", modex, c.towerCallsign),
		fmt.Sprintf("Affirmative, %s.", modex),
	})
}

// ClearedToLand issues landing clearance — 3 variations.
func (c *ATCComposer) ClearedToLand(callsign, activeRunway string, windFromMag, windKts float64) string {
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, %s, runway %s, cleared to land.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, cleared to land runway %s.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, runway %s, cleared to land.", callsign, rwy),
	})
}

// InboundAck acknowledges inbound and provides field info — 3 variations.
func (c *ATCComposer) InboundAck(callsign, activeRunway string, windFromMag, windKts, altimeterInHg float64, trafficAhead int) string {
	rwy := spellRunway(activeRunway)
	wind := formatWind(windFromMag, windKts)
	alt := formatAltimeter(altimeterInHg)
	traffic := ""
	switch trafficAhead {
	case 0:
		traffic = "No traffic ahead."
	case 1:
		traffic = "One aircraft in the pattern ahead of you."
	default:
		traffic = fmt.Sprintf("%s aircraft ahead of you.", numberWord(trafficAhead))
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, runway %s, wind %s, altimeter %s. %s Report final.", callsign, c.towerCallsign, rwy, wind, alt, traffic),
		fmt.Sprintf("%s, %s, altimeter %s, active runway %s, wind %s. %s Report final.", callsign, c.towerCallsign, alt, rwy, wind, traffic),
		fmt.Sprintf("%s, %s, field information: runway %s, wind %s, altimeter %s. %s Call final.", callsign, c.towerCallsign, rwy, wind, alt, traffic),
	})
}

// HoldForSequence sequences traffic — 3 variations.
func (c *ATCComposer) HoldForSequence(callsign string, sequenceNum int, activeRunway string) string {
	rwy := spellRunway(activeRunway)
	if sequenceNum <= 0 {
		return pick([]string{
			fmt.Sprintf("%s, %s, hold outside the pattern at current altitude, runway %s is busy, stand by for sequence.", callsign, c.towerCallsign, rwy),
			fmt.Sprintf("%s, %s, hold clear of the pattern, maintain altitude, expect a five minute delay.", callsign, c.towerCallsign),
			fmt.Sprintf("%s, %s, unable to sequence at this time, hold outside, runway %s is occupied.", callsign, c.towerCallsign, rwy),
		})
	}
	seq := numberWord(sequenceNum)
	return pick([]string{
		fmt.Sprintf("%s, %s, you are number %s for runway %s, extend downwind and maintain altitude, I'll call your base.", callsign, c.towerCallsign, seq, rwy),
		fmt.Sprintf("%s, %s, number %s for runway %s, hold your downwind, stand by for base call.", callsign, c.towerCallsign, seq, rwy),
		fmt.Sprintf("%s, %s, number %s in sequence, runway %s, extend downwind and hold altitude.", callsign, c.towerCallsign, seq, rwy),
	})
}

// TrafficInSight approves visual separation — 3 variations.
func (c *ATCComposer) TrafficInSight(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, maintain visual separation, report final.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, roger, maintain visual, call final.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, maintain visual separation with traffic, report final.", callsign, c.towerCallsign),
	})
}

// NegativeContact re-issues traffic advisory — 3 variations.
func (c *ATCComposer) NegativeContact(callsign, trafficDescription string) string {
	if trafficDescription == "" {
		return pick([]string{
			fmt.Sprintf("%s, %s, traffic no longer a factor, continue approach.", callsign, c.towerCallsign),
			fmt.Sprintf("%s, %s, disregard traffic, continue approach.", callsign, c.towerCallsign),
			fmt.Sprintf("%s, %s, traffic is no longer a factor, continue.", callsign, c.towerCallsign),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, traffic is %s, continue search, report traffic in sight or final.", callsign, c.towerCallsign, trafficDescription),
		fmt.Sprintf("%s, %s, traffic %s, look again, advise visual contact or call final.", callsign, c.towerCallsign, trafficDescription),
		fmt.Sprintf("%s, %s, unable to confirm traffic contact, %s, continue approach and advise.", callsign, c.towerCallsign, trafficDescription),
	})
}

// GoAround instructs go-around — 3 variations.
func (c *ATCComposer) GoAround(callsign, activeRunway string) string {
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, go around, %s, climb runway heading, report downwind runway %s.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, go around, climb and maintain pattern altitude, report downwind.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, go around immediately, %s, fly runway heading, climb to pattern altitude, report downwind runway %s.", callsign, c.towerCallsign, rwy),
	})
}

// GoAroundConflict proactive go-around due to departure — 3 variations.
func (c *ATCComposer) GoAroundConflict(callsign, activeRunway, departureCallsign string) string {
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, go around, %s, %s is rolling runway %s, climb and maintain pattern altitude, report downwind.", callsign, c.towerCallsign, departureCallsign, rwy),
		fmt.Sprintf("%s, go around immediately, %s, traffic departing runway %s, %s, fly runway heading, report downwind.", callsign, c.towerCallsign, rwy, departureCallsign),
		fmt.Sprintf("%s, %s, go around, %s departing below you runway %s, climb to pattern altitude, report downwind.", callsign, c.towerCallsign, departureCallsign, rwy),
	})
}

// StraightInApproved clears IFR straight-in — 3 variations.
func (c *ATCComposer) StraightInApproved(callsign, activeRunway string, altimeterInHg float64) string {
	rwy := spellRunway(activeRunway)
	alt := formatAltimeter(altimeterInHg)
	return pick([]string{
		fmt.Sprintf("%s, %s, straight-in runway %s approved, altimeter %s, report final.", callsign, c.towerCallsign, rwy, alt),
		fmt.Sprintf("%s, %s, cleared straight-in approach runway %s, altimeter %s, call final.", callsign, c.towerCallsign, rwy, alt),
		fmt.Sprintf("%s, %s, roger, straight-in runway %s, altimeter %s, advise final.", callsign, c.towerCallsign, rwy, alt),
	})
}

// StraightInSequenced approves straight-in with traffic — 3 variations.
func (c *ATCComposer) StraightInSequenced(callsign, activeRunway string, altimeterInHg float64, trafficAhead int) string {
	rwy := spellRunway(activeRunway)
	alt := formatAltimeter(altimeterInHg)
	seq := numberWord(trafficAhead + 1)
	return pick([]string{
		fmt.Sprintf("%s, %s, straight-in runway %s approved, altimeter %s, number %s, reduce to slowest practical speed, report final.", callsign, c.towerCallsign, rwy, alt, seq),
		fmt.Sprintf("%s, %s, cleared straight-in runway %s, number %s, altimeter %s, reduce speed, call final.", callsign, c.towerCallsign, rwy, seq, alt),
		fmt.Sprintf("%s, %s, number %s straight-in runway %s, altimeter %s, slow to approach speed, report final.", callsign, c.towerCallsign, seq, rwy, alt),
	})
}

// RunwayVacated acknowledges runway clear — 3 variations.
func (c *ATCComposer) RunwayVacated(callsign, activeRunway string) string {
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, %s, runway %s clear, taxi to parking.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, roger clear of runway %s, taxi to parking at your discretion.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, runway %s is clear, welcome back, taxi to parking.", callsign, c.towerCallsign, rwy),
	})
}

// EmergencyAck acknowledges emergency — 3 variations.
func (c *ATCComposer) EmergencyAck(callsign, activeRunway string, windFromMag, windKts, altimeterInHg float64) string {
	rwy := spellRunway(activeRunway)
	wind := formatWind(windFromMag, windKts)
	alt := formatAltimeter(altimeterInHg)
	return pick([]string{
		fmt.Sprintf("%s, %s, emergency acknowledged, runway %s is yours, wind %s, altimeter %s, cleared to land, crash crew is standing by.", callsign, c.towerCallsign, rwy, wind, alt),
		fmt.Sprintf("%s, %s, roger mayday, all traffic hold, runway %s cleared, wind %s, altimeter %s, cleared immediate landing, emergency services are alerted.", callsign, c.towerCallsign, rwy, wind, alt),
		fmt.Sprintf("%s, %s, emergency acknowledged, you have runway %s, wind %s, altimeter %s, cleared to land, say souls on board and fuel state.", callsign, c.towerCallsign, rwy, wind, alt),
	})
}

// ProceedToRunway clears from hold short with takeoff clearance — 3 variations.
func (c *ATCComposer) ProceedToRunway(callsign, activeRunway string, windFromMag, windKts float64, trafficOnFinal int) string {
	rwy := spellRunway(activeRunway)
	wind := formatWind(windFromMag, windKts)
	trafficNote := ""
	if trafficOnFinal > 0 {
		trafficNote = fmt.Sprintf(", traffic on final runway %s", rwy)
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, proceed to runway %s, wind %s%s, cleared for takeoff.", callsign, c.towerCallsign, rwy, wind, trafficNote),
		fmt.Sprintf("%s, %s, enter and line up runway %s, wind %s%s, cleared for takeoff.", callsign, c.towerCallsign, rwy, wind, trafficNote),
		fmt.Sprintf("%s, %s, runway %s is yours, wind %s%s, cleared for takeoff.", callsign, c.towerCallsign, rwy, wind, trafficNote),
	})
}

// AltimeterCheck issues a proactive altimeter reminder — 3 variations.
func (c *ATCComposer) AltimeterCheck(callsign string, altimeterInHg float64) string {
	alt := formatAltimeter(altimeterInHg)
	return pick([]string{
		fmt.Sprintf("%s, %s, check altimeter, %s.", callsign, c.towerCallsign, alt),
		fmt.Sprintf("%s, %s, altimeter %s, acknowledge.", callsign, c.towerCallsign, alt),
		fmt.Sprintf("%s, %s, set altimeter %s.", callsign, c.towerCallsign, alt),
	})
}

// RadarCheck reads back the pilot's Tacview-derived position: angels (alt/1000),
// range from tower in nm, and true bearing from tower (0..359 degrees).
func (c *ATCComposer) RadarCheck(callsign string, angels, distNm, bearingDeg int) string {
	ang := numberWord(angels)
	dist := milesToWord(distNm)
	brg := bearingWord(bearingDeg)
	return pick([]string{
		fmt.Sprintf("%s, %s, you are at angels %s and %s from tower on a bearing of %s.", callsign, c.towerCallsign, ang, dist, brg),
		fmt.Sprintf("%s, %s, radar contact, angels %s, %s, bearing %s from the field.", callsign, c.towerCallsign, ang, dist, brg),
		fmt.Sprintf("%s, %s, I have you angels %s, range %s, bearing %s.", callsign, c.towerCallsign, ang, dist, brg),
	})
}

// RadarCheckNoContact responds to a radar check when no Tacview track exists
// for the calling aircraft.
func (c *ATCComposer) RadarCheckNoContact(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, negative radar contact, say position.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, no radar contact at this time, say position and altitude.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, unable radar contact, say your position.", callsign, c.towerCallsign),
	})
}

// SequencedInitialAck — Tacview-aware initial ack with named traffic ahead.
func (c *ATCComposer) SequencedInitialAck(callsign string, distNm int, activeRunway string, patternAltFt int, altimeterInHg float64, seqNum int, leadCallsign string, leadDistNm int) string {
	rwy := spellRunway(activeRunway)
	seq := numberWord(seqNum)
	lead := numberWord(leadDistNm)
	return pick([]string{
		fmt.Sprintf("%s, %s, number %s, follow %s, %s miles in trail, runway %s.", callsign, c.towerCallsign, seq, leadCallsign, lead, rwy),
		fmt.Sprintf("%s, %s, number %s, traffic is %s, %s miles ahead, runway %s.", callsign, c.towerCallsign, seq, leadCallsign, lead, rwy),
		fmt.Sprintf("%s, number %s, follow %s, runway %s.", callsign, seq, leadCallsign, rwy),
	})
}

// ShortFinalSequenced — pilot calls short final but traffic ahead, give sequence.
func (c *ATCComposer) ShortFinalSequenced(callsign, activeRunway string, seqNum int) string {
	rwy := spellRunway(activeRunway)
	seq := numberWord(seqNum)
	return pick([]string{
		fmt.Sprintf("%s, %s, number %s, report short final.", callsign, c.towerCallsign, seq),
		fmt.Sprintf("%s, %s, number %s for runway %s, short final.", callsign, c.towerCallsign, seq, rwy),
		fmt.Sprintf("%s, number %s, short final runway %s.", callsign, seq, rwy),
	})
}

// AltitudeClearance responds to a pilot requesting altitude check.
// If tacviewAltFt > 0 we read back their actual altitude from Tacview.
func (c *ATCComposer) AltitudeClearance(callsign, activeRunway string, altimeterInHg float64, tacviewAltFt int) string {
	alt := formatAltimeter(altimeterInHg)
	if tacviewAltFt > 0 {
		actual := spellAltitudeFt(tacviewAltFt)
		return pick([]string{
			fmt.Sprintf("%s, %s, radar shows you at %s feet, altimeter %s, maintain VFR.", callsign, c.towerCallsign, actual, alt),
			fmt.Sprintf("%s, %s, indicating %s feet, altimeter %s.", callsign, c.towerCallsign, actual, alt),
			fmt.Sprintf("%s, %s, you are at %s feet, altimeter %s, maintain VFR.", callsign, c.towerCallsign, actual, alt),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, altimeter %s, maintain VFR.", callsign, c.towerCallsign, alt),
		fmt.Sprintf("%s, %s, altimeter %s, VFR altitude at pilot discretion.", callsign, c.towerCallsign, alt),
		fmt.Sprintf("%s, %s, altimeter %s, no altitude restrictions.", callsign, c.towerCallsign, alt),
	})
}

// SequencedClearedToLand is issued when the previous aircraft has just vacated — 3 var.
func (c *ATCComposer) SequencedClearedToLand(callsign, activeRunway string, windFromMag, windKts float64) string {
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, %s, runway %s clear, cleared to land.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, runway clear, cleared to land runway %s.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, runway %s clear, cleared to land.", callsign, rwy),
	})
}

// CommandFenceIn — 3 aggressive variations per fuel state scenario.
func (c *ATCComposer) CommandFenceIn(callsign string, fuelState float64) string {
	if fuelState > 0 {
		s := fmt.Sprintf("%.1f", fuelState)
		return pick([]string{
			fmt.Sprintf("%s, Command, state %s, fence in, go kick some ass.", callsign, s),
			fmt.Sprintf("%s, Command, copy, state %s, fence in, make it hurt.", callsign, s),
			fmt.Sprintf("%s, Command, state %s, fence in, go get some.", callsign, s),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, Command, fence in, go kick some ass.", callsign),
		fmt.Sprintf("%s, Command, copy, fence in, make it hurt.", callsign),
		fmt.Sprintf("%s, Command, fence in, go get some.", callsign),
	})
}

// CommandFenceOut — 3 variations per fuel state scenario.
func (c *ATCComposer) CommandFenceOut(callsign string, fuelState float64) string {
	if fuelState > 0 {
		s := fmt.Sprintf("%.1f", fuelState)
		if fuelState < 2.0 {
			return pick([]string{
				fmt.Sprintf("%s, Command, state %s, fence out, bingo, get your ass back now.", callsign, s),
				fmt.Sprintf("%s, Command, copy, state %s, you are bingo, expedite recovery.", callsign, s),
				fmt.Sprintf("%s, Command, state %s, fence out, low state, move it.", callsign, s),
			})
		}
		return pick([]string{
			fmt.Sprintf("%s, Command, state %s, fence out, good work, proceed recovery.", callsign, s),
			fmt.Sprintf("%s, Command, copy, state %s, fence out, well done, RTB.", callsign, s),
			fmt.Sprintf("%s, Command, state %s, fence out, nice work, come on home.", callsign, s),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, Command, fence out, good work, proceed recovery.", callsign),
		fmt.Sprintf("%s, Command, copy, fence out, well done, RTB.", callsign),
		fmt.Sprintf("%s, Command, fence out, nice work, come on home.", callsign),
	})
}

// CommandFuelState — 3 variations per fuel state scenario.
func (c *ATCComposer) CommandFuelState(callsign string, fuelState float64) string {
	if fuelState <= 0 {
		return fmt.Sprintf("%s, Command, say state again.", callsign)
	}
	s := fmt.Sprintf("%.1f", fuelState)
	if fuelState < 2.0 {
		return pick([]string{
			fmt.Sprintf("%s, Command, state %s, you are bingo, get your ass back now.", callsign, s),
			fmt.Sprintf("%s, Command, copy state %s, bingo, expedite recovery.", callsign, s),
			fmt.Sprintf("%s, Command, state %s, low state, move it.", callsign, s),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, Command, copy state %s.", callsign, s),
		fmt.Sprintf("%s, Command, state %s, copy.", callsign, s),
		fmt.Sprintf("%s, Command, roger, state %s.", callsign, s),
	})
}

// ── Deckboss Composer Methods ─────────────────────────────────────────────────

// DeckbossCatAssign — assigns a cat to an incoming green jet.
// DeckbossUnderTension — aircraft on cat, under tension.
func (c *ATCComposer) DeckbossUnderTension(callsign string, catNum int) string {
	cat := numberWord(catNum)
	return pick([]string{
		fmt.Sprintf("%s, Deckboss, under tension, cat %s.", callsign, cat),
		fmt.Sprintf("%s, Deckboss, tension cat %s, hold.", callsign, cat),
		fmt.Sprintf("%s, Deckboss, cat %s under tension, stand by.", callsign, cat),
	})
}

// DeckbossShooter — shooter call, aggressive, cleared to launch.
// DeckbossCatClear — cat has cleared after launch, advance conga.
func (c *ATCComposer) DeckbossCatClear(catNum int) string {
	cat := numberWord(catNum)
	return pick([]string{
		fmt.Sprintf("Cat %s is clear.", cat),
		fmt.Sprintf("Cat %s clear, deck is moving.", cat),
		fmt.Sprintf("Cat %s off the deck.", cat),
	})
}

// DeckbossAdvanceConga — a cat opened up, pull next from conga.
// DeckbossCongaHold — cats full, hold in conga line.
// DeckbossDeckFull — conga line also full, hold clear.
func (c *ATCComposer) DeckbossDeckFull(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, Deckboss, deck is full, hold clear of the bow.", callsign),
		fmt.Sprintf("%s, Deckboss, no room on deck, hold your position.", callsign),
		fmt.Sprintf("%s, Deckboss, deck is saturated, hold clear, standby.", callsign),
	})
}

// DeckbossRadioCheck — radio check response.
// ── Deckboss Composer Methods ─────────────────────────────────────────────────

func (c *ATCComposer) DeckbossCatAssignment(callsign string, catNum int) string {
	cat := numberWord(catNum)
	return pick([]string{
		fmt.Sprintf("%s, Deckboss, cat %s, clear to taxi cat %s.", callsign, cat, cat),
		fmt.Sprintf("%s, Deckboss, cat %s is yours, taxi forward.", callsign, cat),
		fmt.Sprintf("%s, Deckboss, proceed to cat %s, cleared to spot.", callsign, cat),
	})
}


func (c *ATCComposer) DeckbossCongaLine(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, Deckboss, all cats engaged, proceed to conga line, standby for assignment.", callsign),
		fmt.Sprintf("%s, Deckboss, cats are full, join the conga line, we'll get you up.", callsign),
		fmt.Sprintf("%s, Deckboss, no cats available, conga line, standby.", callsign),
	})
}

// DeckbossDeckFull — conga line full, hold clear.

func (c *ATCComposer) DeckbossStandby(callsign string, position int) string {
	pos := numberWord(position)
	return pick([]string{
		fmt.Sprintf("%s, Deckboss, you are number %s in the conga, standby.", callsign, pos),
		fmt.Sprintf("%s, Deckboss, hold position, number %s in line.", callsign, pos),
	})
}

// ── Marshal Composer Methods ──────────────────────────────────────────────────

// marshalWeatherPhrase summarises ceiling + visibility into a short Marshal-style
// weather call. Falls back to "weather clear, visibility ten" if inputs are zero.
func marshalWeatherPhrase(ceilingFt, visNm float64) string {
	// Visibility wording
	visWord := "ten"
	switch {
	case visNm <= 0:
		visWord = "ten"
	case visNm < 1:
		visWord = "less than one"
	case visNm < 3:
		visWord = fmt.Sprintf("%.0f", visNm)
	case visNm >= 10:
		visWord = "ten plus"
	default:
		visWord = numberWord(int(visNm))
	}

	// Ceiling wording
	switch {
	case ceilingFt <= 0 || ceilingFt >= 10000:
		return fmt.Sprintf("ceiling unrestricted, visibility %s", visWord)
	case ceilingFt >= 3000:
		return fmt.Sprintf("ceiling %s thousand scattered, visibility %s", numberWord(int(ceilingFt/1000)), visWord)
	case ceilingFt >= 1000:
		return fmt.Sprintf("ceiling %s thousand broken, visibility %s", numberWord(int(ceilingFt/1000)), visWord)
	default:
		return fmt.Sprintf("ceiling %s hundred overcast, visibility %s", numberWord(int(ceilingFt/100)), visWord)
	}
}

// MarshalMarkingMom — initial marshal contact with weather, Case, BRC, altimeter,
// and stack altitude assignment. brc = carrier heading in degrees, -1 if unknown.
// If radarFound, prepends a "I have you on radar at angels X, range Y, bearing Z
// from mother" line built from live Tacview position relative to the carrier.
func (c *ATCComposer) MarshalMarkingMom(callsign string, position, stackAngels int, altimeterInHg, ceilingFt, visNm, brc float64,
	radarAngels, radarDistNm, radarBearingDeg int, radarFound bool) string {
	ang := numberWord(stackAngels)
	alt := formatAltimeter(altimeterInHg)
	wx := marshalWeatherPhrase(ceilingFt, visNm)

	// Recovery case derived from ceiling
	var recovery string
	switch {
	case ceilingFt <= 0, ceilingFt >= 3000:
		recovery = "Case One"
	case ceilingFt >= 1000:
		recovery = "Case Two"
	default:
		recovery = "Case Three"
	}

	// BRC phrase — omitted if carrier heading unknown
	brcStr := ""
	if brc >= 0 {
		brcStr = fmt.Sprintf(", BRC %03.0f", brc)
	}

	// Radar readback phrase from carrier — only if Tacview shows the caller
	radarStr := ""
	if radarFound {
		rAng := numberWord(radarAngels)
		rDist := milesToWord(radarDistNm)
		rBrg := bearingWord(radarBearingDeg)
		radarStr = pick([]string{
			fmt.Sprintf(" radar contact, angels %s, range %s, bearing %s from mother,", rAng, rDist, rBrg),
			fmt.Sprintf(" I have you on radar, angels %s, %s from mother, bearing %s,", rAng, rDist, rBrg),
			fmt.Sprintf(" radar contact angels %s, %s on the %s,", rAng, rDist, rBrg),
		})
	}

	return pick([]string{
		fmt.Sprintf("%s, Marshal,%s mother's weather %s, expect %s recovery%s, altimeter %s, Marshal angels %s, report see me at ten.",
			callsign, radarStr, wx, recovery, brcStr, alt, ang),
		fmt.Sprintf("%s, Marshal,%s %s recovery, mother %s%s, altimeter %s, stack angels %s, report see me at ten.",
			callsign, radarStr, recovery, wx, brcStr, alt, ang),
		fmt.Sprintf("%s, Marshal,%s %s, %s%s, altimeter %s, your angels are %s, report see me at ten.",
			callsign, radarStr, recovery, wx, brcStr, alt, ang),
		fmt.Sprintf("%s, Marshal,%s mother %s, %s recovery%s, altimeter %s, marshal angels %s, report see me at ten.",
			callsign, radarStr, wx, recovery, brcStr, alt, ang),
	})
}

// MarshalRadarContact — at 10nm contact.
func (c *ATCComposer) MarshalRadarContact(callsign string, distNm int) string {
	dist := numberWord(distNm)
	return pick([]string{
		fmt.Sprintf("%s, Marshal, radar contact, %s miles, say state.", callsign, dist),
		fmt.Sprintf("%s, Marshal, contact, %s miles, say state.", callsign, dist),
		fmt.Sprintf("%s, Marshal, got you at %s miles, say state.", callsign, dist),
	})
}

// MarshalCopyState — acknowledges fuel state.
func (c *ATCComposer) MarshalCopyState(callsign string, fuelState float64) string {
	s := fmt.Sprintf("%.1f", fuelState)
	if fuelState < 2.0 {
		return pick([]string{
			fmt.Sprintf("%s, Marshal, state %s, expedite recovery.", callsign, s),
			fmt.Sprintf("%s, Marshal, copy state %s, you are priority.", callsign, s),
			fmt.Sprintf("%s, Marshal, state %s, priority recovery.", callsign, s),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, Marshal, copy state %s.", callsign, s),
		fmt.Sprintf("%s, Marshal, state %s, copy.", callsign, s),
		fmt.Sprintf("%s, Marshal, roger, state %s.", callsign, s),
	})
}

// MarshalSignalCharlie — deck is clear, cleared to commence.
func (c *ATCComposer) MarshalSignalCharlie(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, Marshal, signal Charlie.", callsign),
		fmt.Sprintf("%s, Marshal, you have Charlie.", callsign),
		fmt.Sprintf("%s, Marshal, Charlie.", callsign),
	})
}

// MarshalCopyCommencing — pilot is commencing approach.
func (c *ATCComposer) MarshalCopyCommencing(callsign string, fuelState float64) string {
	if fuelState > 0 {
		s := fmt.Sprintf("%.1f", fuelState)
		return pick([]string{
			fmt.Sprintf("%s, Marshal, copy commencing, state %s.", callsign, s),
			fmt.Sprintf("%s, Marshal, commencing, state %s, copy.", callsign, s),
			fmt.Sprintf("%s, Marshal, roger, commencing, state %s.", callsign, s),
		})
	}
	return pick([]string{
		fmt.Sprintf("%s, Marshal, copy commencing.", callsign),
		fmt.Sprintf("%s, Marshal, commencing, copy.", callsign),
		fmt.Sprintf("%s, Marshal, roger, commencing.", callsign),
	})
}

// MarshalPushButton — at 3nm initial, tells pilot to push to TACAN.
func (c *ATCComposer) MarshalPushButton(callsign string, tacanChannel int) string {
	ch := numberWord(tacanChannel)
	return pick([]string{
		fmt.Sprintf("%s, Marshal, push button %s, check in.", callsign, ch),
		fmt.Sprintf("%s, Marshal, button %s, check in.", callsign, ch),
		fmt.Sprintf("%s, Marshal, push button %s and check in.", callsign, ch),
	})
}

// MarshalContact — LSO contact, marshal done.
func (c *ATCComposer) MarshalContact(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, Marshal, contact.", callsign),
		fmt.Sprintf("%s, Marshal, you have contact.", callsign),
		fmt.Sprintf("%s, Marshal, contact, good luck.", callsign),
	})
}

// MarshalEstablishedAck — pilot established in stack.
func (c *ATCComposer) MarshalEstablishedAck(callsign string, angels int) string {
	ang := numberWord(angels)
	return pick([]string{
		fmt.Sprintf("%s, Marshal, roger, hold angels %s.", callsign, ang),
		fmt.Sprintf("%s, Marshal, established angels %s, copy.", callsign, ang),
		fmt.Sprintf("%s, Marshal, angels %s, stand by for Charlie.", callsign, ang),
	})
}

// SpeedWarning warns an aircraft exceeding 350kt within 10nm — 3 variations.
func (c *ATCComposer) SpeedWarning(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, speed check, you are fast inside ten miles, reduce to three five zero or below.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, caution airspeed, reduce speed, three five zero knots max inside ten miles.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, you are showing fast, reduce to three five zero immediately, inside ten miles.", callsign, c.towerCallsign),
	})
}

// WindShift announces a runway change to all aircraft on tower freq when the
// wind shifts enough to make a different runway most-into-wind. Proactive
// broadcast — no addressed callsign.
func (c *ATCComposer) WindShift(activeRunway string, windFromMag, windKts float64) string {
	rwy := spellRunway(activeRunway)
	wind := formatWind(windFromMag, windKts)
	return pick([]string{
		fmt.Sprintf("All aircraft, %s, wind shift, runway in use is now %s, wind %s.", c.towerCallsign, rwy, wind),
		fmt.Sprintf("Notice all stations, %s advises runway change, runway in use is %s, wind %s.", c.towerCallsign, rwy, wind),
		fmt.Sprintf("Attention all aircraft, %s, runway in use is now %s, wind %s.", c.towerCallsign, rwy, wind),
	})
}

// ── Mode-aware responses ──────────────────────────────────────────────────────

// RedirectToStraightIn tells a VFR pilot to execute straight-in instead (IMC) — 3 var.
func (c *ATCComposer) RedirectToStraightIn(callsign, activeRunway string, altimeterInHg float64) string {
	rwy := spellRunway(activeRunway)
	alt := formatAltimeter(altimeterInHg)
	return pick([]string{
		fmt.Sprintf("%s, %s, negative overhead break, IMC conditions, straight-in only runway %s, altimeter %s, report ten mile final.", callsign, c.towerCallsign, rwy, alt),
		fmt.Sprintf("%s, %s, overhead break not authorized, IMC in effect, execute straight-in runway %s, altimeter %s, report ten mile final.", callsign, c.towerCallsign, rwy, alt),
		fmt.Sprintf("%s, %s, pattern not available, IMC conditions, straight-in runway %s approved, altimeter %s, call ten mile final.", callsign, c.towerCallsign, rwy, alt),
	})
}

// IMCContinueApproach tells a pilot to continue and report final (replaces downwind/base in IMC) — 3 var.
func (c *ATCComposer) IMCContinueApproach(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, roger, continue approach, report final.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, continue approach, call final.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, maintain approach, advise final.", callsign, c.towerCallsign),
	})
}

// NightDistanceInitialAck acknowledges inbound at distance for night ops — 3 var.
func (c *ATCComposer) IFRDistanceInitialAck(callsign string, distNm int, activeRunway string, patternAltFt int, altimeterInHg float64, trafficAhead int) string {
	rwy := spellRunway(activeRunway)
	ang := numberWord(approachAngels(distNm, patternAltFt))
	alt := formatAltimeter(altimeterInHg)
	dist := numberWord(distNm)
	traffic := ""
	switch trafficAhead {
	case 0:
		traffic = " No traffic ahead."
	case 1:
		traffic = " One aircraft in the pattern ahead of you."
	default:
		traffic = fmt.Sprintf(" %s aircraft in the pattern ahead of you.", numberWord(trafficAhead))
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, got you at %s mile initial, night ops in effect, runway lights are on, proceed angels %s, altimeter %s, enter pattern runway %s.%s", callsign, c.towerCallsign, dist, ang, alt, rwy, traffic),
		fmt.Sprintf("%s, %s, %s mile initial, night conditions, field is lit, angels %s, altimeter %s, runway %s active.%s", callsign, c.towerCallsign, dist, ang, alt, rwy, traffic),
		fmt.Sprintf("%s, %s, roger %s mile initial, night ops, runway %s lights on, altimeter %s, proceed angels %s.%s", callsign, c.towerCallsign, dist, rwy, alt, ang, traffic),
	})
}

// IMCDistanceInitialAck redirects inbound to straight-in from initial call — 3 var.
func (c *ATCComposer) IMCDistanceInitialAck(callsign string, distNm int, activeRunway string, altimeterInHg float64, trafficAhead int) string {
	rwy := spellRunway(activeRunway)
	alt := formatAltimeter(altimeterInHg)
	dist := numberWord(distNm)
	seq := ""
	if trafficAhead > 0 {
		seq = fmt.Sprintf(", number %s", numberWord(trafficAhead+1))
	}
	return pick([]string{
		fmt.Sprintf("%s, %s, got you at %s miles, IMC conditions in effect, straight-in only runway %s, altimeter %s%s, report ten mile final.", callsign, c.towerCallsign, dist, rwy, alt, seq),
		fmt.Sprintf("%s, %s, %s miles, IMC in effect, execute straight-in runway %s, altimeter %s%s, call ten mile final.", callsign, c.towerCallsign, dist, rwy, alt, seq),
		fmt.Sprintf("%s, %s, roger %s miles, no pattern available IMC, straight-in runway %s%s, altimeter %s, report ten mile final.", callsign, c.towerCallsign, dist, rwy, seq, alt),
	})
}

// NightClearedToLand adds gear check for night landings — 3 var.
func (c *ATCComposer) NightClearedToLand(callsign, activeRunway string, windFromMag, windKts float64) string {
	rwy := spellRunway(activeRunway)
	return pick([]string{
		fmt.Sprintf("%s, %s, runway %s, check gear, cleared to land.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, %s, gear check, runway %s, cleared to land.", callsign, c.towerCallsign, rwy),
		fmt.Sprintf("%s, gear down, runway %s, cleared to land.", callsign, rwy),
	})
}

// NightBaseAck acknowledges base with gear reminder — 3 var.
func (c *ATCComposer) NightBaseAck(callsign, activeRunway string, seqNum int) string {
	modex := callsign
	if parts := strings.Fields(callsign); len(parts) > 1 {
		modex = parts[len(parts)-1]
	}
	return pick([]string{
		fmt.Sprintf("%s, affirmative, check gear down.", modex),
		fmt.Sprintf("%s, %s, affirmative, gear check.", modex, c.towerCallsign),
		fmt.Sprintf("Affirmative, %s, confirm gear down.", modex),
	})
}

// UnableToUnderstand — 3 variations.
func (c *ATCComposer) UnableToUnderstand(callsign string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, say again your request.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, unable to copy, say again.", callsign, c.towerCallsign),
		fmt.Sprintf("%s, %s, you were broken, say again.", callsign, c.towerCallsign),
	})
}

// ── Formatting helpers ─────────────────────────────────────────────────────────

func formatWind(fromMag, kts float64) string {
	if kts < 3 {
		return "calm"
	}
	dir := int(math.Round(fromMag/10) * 10)
	if dir == 0 {
		dir = 360
	}
	return fmt.Sprintf("%03d at %d knots", dir, int(math.Round(kts)))
}

func formatAltimeter(inHg float64) string {
	hundredths := int(math.Round(inHg * 100))
	return spellAltimeter(hundredths/100, hundredths%100)
}

func spellAltimeter(whole, frac int) string {
	return fmt.Sprintf("%s point %s", spellDigits(whole), spellDigits(frac))
}

func spellDigits(n int) string {
	digits := fmt.Sprintf("%02d", n)
	words := make([]string, len(digits))
	dw := map[rune]string{
		'0': "zero", '1': "one", '2': "two", '3': "three",
		'4': "four", '5': "five", '6': "six", '7': "seven",
		'8': "eight", '9': "niner",
	}
	for i, d := range digits {
		words[i] = dw[d]
	}
	return strings.Join(words, " ")
}

func spellRunway(designator string) string {
	suffix := ""
	digits := designator
	if strings.HasSuffix(designator, "L") {
		suffix = " left"
		digits = designator[:len(designator)-1]
	} else if strings.HasSuffix(designator, "R") {
		suffix = " right"
		digits = designator[:len(designator)-1]
	} else if strings.HasSuffix(designator, "C") {
		suffix = " center"
		digits = designator[:len(designator)-1]
	}
	spoken := make([]string, len(digits))
	dw := map[rune]string{
		'0': "zero", '1': "one", '2': "two", '3': "three",
		'4': "four", '5': "five", '6': "six", '7': "seven",
		'8': "eight", '9': "niner",
	}
	for i, d := range digits {
		spoken[i] = dw[d]
	}
	return strings.Join(spoken, " ") + suffix
}

func numberWord(n int) string {
	words := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	if n >= 0 && n < len(words) {
		return words[n]
	}
	return fmt.Sprintf("%d", n)
}

func milesToWord(nm int) string {
	if nm == 1 {
		return "1 mile"
	}
	return fmt.Sprintf("%s miles", numberWord(nm))
}

// bearingWord spells a 0..359 bearing as three digits (e.g. 270 → "two seven zero").
func bearingWord(deg int) string {
	deg = ((deg % 360) + 360) % 360
	digits := fmt.Sprintf("%03d", deg)
	parts := make([]string, 0, 3)
	for _, d := range digits {
		parts = append(parts, digitWords[d])
	}
	return strings.Join(parts, " ")
}

// spellAltitudeFt converts an altitude in feet to spoken form.
// e.g. 15000 → "fifteen thousand", 2500 → "two thousand five hundred"
func spellAltitudeFt(ft int) string {
	if ft <= 0 {
		return "unknown"
	}
	thousands := ft / 1000
	hundreds := (ft % 1000) / 100
	result := ""
	if thousands > 0 {
		result = numberWord(thousands) + " thousand"
	}
	if hundreds > 0 {
		if result != "" {
			result += " "
		}
		result += numberWord(hundreds*100)
	}
	if result == "" {
		result = numberWord(ft)
	}
	return result
}

func approachAngels(distNm, patternAltFt int) int {
	patternAngels := (patternAltFt / 1000) + 1
	switch {
	case distNm >= 25:
		return 5
	case distNm >= 15:
		return 4
	case distNm >= 10:
		return 3
	case distNm >= 5:
		return patternAngels + 1
	default:
		return patternAngels
	}
}

var digitWords = map[rune]string{
	'0': "zero", '1': "one", '2': "two", '3': "three",
	'4': "four", '5': "five", '6': "six", '7': "seven",
	'8': "eight", '9': "niner",
}

func spellFrequency(mhz float64) string {
	whole := int(mhz)
	frac := int(math.Round((mhz - float64(whole)) * 1000))
	wholeSpoken := ""
	for i, d := range fmt.Sprintf("%d", whole) {
		if i > 0 {
			wholeSpoken += " "
		}
		wholeSpoken += digitWords[d]
	}
	fracSpoken := "zero"
	if frac > 0 {
		fracSpoken = ""
		for i, d := range fmt.Sprintf("%03d", frac) {
			if i > 0 {
				fracSpoken += " "
			}
			fracSpoken += digitWords[d]
		}
		for strings.HasSuffix(fracSpoken, " zero") {
			fracSpoken = fracSpoken[:len(fracSpoken)-5]
		}
	}
	return wholeSpoken + " decimal " + fracSpoken
}
