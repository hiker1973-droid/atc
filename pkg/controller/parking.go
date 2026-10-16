package controller

import (
	"strings"
	"time"
)

// Parking areas on landing (operator 2026-10-16, Senaki: "Lets assign parking
// on landing ... Assign F, G, A B C D area via N"). The tower hands the areas
// out in turn when a jet reports clear of the runway or asks to taxi to
// parking; a flight asking again gets the area it already has.

// parkingHold is how long an assigned area stays with a callsign.
const parkingHold = 45 * time.Minute

type parkingSlot struct {
	area string
	at   time.Time
}

// wantsParking: a taxi request that is going in, not out.
func wantsParking(raw string) bool {
	return containsAny(strings.ToLower(raw), "parking", "to park", "the ramp", "to ramp", "chocks", "the line", "hangar")
}

// assignParking returns the parking area for callsign, the next one in turn
// on a first call. "" when the field has none. Caller holds c.mu.
func (c *ATCController) assignParking(callsign string, areas []string, now time.Time) string {
	if len(areas) == 0 {
		return ""
	}
	if c.parking == nil {
		c.parking = make(map[string]parkingSlot)
	}
	key := compactCallsign(callsign)
	if p, ok := c.parking[key]; ok && now.Sub(p.at) < parkingHold {
		return p.area
	}
	for k, p := range c.parking {
		if now.Sub(p.at) >= parkingHold {
			delete(c.parking, k)
		}
	}
	area := areas[c.parkingNext%len(areas)]
	c.parkingNext++
	c.parking[key] = parkingSlot{area: area, at: now}
	return area
}
