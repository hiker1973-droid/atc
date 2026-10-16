package composer

import (
	"fmt"
	"math"
)

// Safety and comms-discipline calls (operator 2026-10-15: "Should we have a
// tower or command report near miss or improper comms?").

// SayAgainFullCallsign answers a call that gave only a flight name ("Venom
// flight, request departure") - the tower cannot tell which jet it is.
func (c *ATCComposer) SayAgainFullCallsign(flight string) string {
	return pick([]string{
		fmt.Sprintf("%s, %s, say again with full callsign.", flight, c.towerCallsign),
		fmt.Sprintf("%s, %s, say full callsign.", flight, c.towerCallsign),
	})
}

// TrafficAlert warns one aircraft of another converging on it: clock is the
// traffic's clock position from the aircraft's nose, altDiffFt the traffic's
// altitude minus the aircraft's.
func (c *ATCComposer) TrafficAlert(callsign string, clock int, distNm float64, altDiffFt float64) string {
	return fmt.Sprintf("%s, %s, traffic alert, %s o'clock, %s, %s.",
		callsign, c.towerCallsign, clockWord(clock), trafficRange(distNm), trafficAltitude(altDiffFt))
}

// DepartedWithoutClearance is said only when a jet calls airborne without a
// takeoff clearance while there was traffic on short final.
func (c *ATCComposer) DepartedWithoutClearance(callsign string) string {
	return fmt.Sprintf("%s, %s, no takeoff clearance was issued, traffic was on short final. Wait for the clearance next time.",
		callsign, c.towerCallsign)
}

func clockWord(clock int) string {
	words := []string{"twelve", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve"}
	if clock < 0 || clock > 12 {
		clock = 12
	}
	return words[clock]
}

func trafficRange(distNm float64) string {
	switch {
	case distNm < 0.75:
		return "less than one mile"
	case distNm < 1.5:
		return "one mile"
	default:
		return milesToWord(int(math.Round(distNm)))
	}
}

func trafficAltitude(diffFt float64) string {
	h := int(math.Round(math.Abs(diffFt)/100)) * 100
	switch {
	case h <= 200:
		return "same altitude"
	case diffFt > 0:
		return fmt.Sprintf("%s feet above", spellHundreds(h))
	default:
		return fmt.Sprintf("%s feet below", spellHundreds(h))
	}
}

// spellHundreds: 300 -> "three hundred", 1200 -> "one thousand two hundred".
func spellHundreds(ft int) string {
	th, hu := ft/1000, (ft%1000)/100
	switch {
	case th > 0 && hu > 0:
		return fmt.Sprintf("%s thousand %s hundred", numberWord(th), numberWord(hu))
	case th > 0:
		return fmt.Sprintf("%s thousand", numberWord(th))
	default:
		return fmt.Sprintf("%s hundred", numberWord(hu))
	}
}
