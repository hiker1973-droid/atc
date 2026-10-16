package controller

import (
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Parking spots on landing (operator 2026-10-16, Senaki: "Lets assign parking
// on landing", then off the Taxi Parking page "Assign 1 58 via Charlie or
// Bravo via N[ovember]"). The tower hands the spots out in turn when a jet
// reports clear of the runway or asks to taxi to parking; a flight asking
// again gets the spot it already has.

// parkingHold is how long an assigned spot stays with a callsign.
const parkingHold = 45 * time.Minute

type parkingSlot struct {
	spot int
	via  string
	at   time.Time
}

// wantsParking: a taxi request that is going in, not out.
func wantsParking(raw string) bool {
	return containsAny(strings.ToLower(raw), "parking", "to park", "the ramp", "to ramp", "chocks", "the line", "hangar")
}

// assignParking returns the parking spot and taxi route for callsign, the
// next spot in turn on a first call. spot == 0 when the field has none.
// Caller holds c.mu.
func (c *ATCController) assignParking(callsign string, af *airfield.Airfield, now time.Time) (int, string) {
	if af == nil || len(af.ParkingRoutes) == 0 {
		return 0, ""
	}
	if c.parking == nil {
		c.parking = make(map[string]parkingSlot)
	}
	key := compactCallsign(callsign)
	if p, ok := c.parking[key]; ok && now.Sub(p.at) < parkingHold {
		return p.spot, p.via
	}
	for k, p := range c.parking {
		if now.Sub(p.at) >= parkingHold {
			delete(c.parking, k)
		}
	}
	spot, via, ok := af.ParkingSpot(c.parkingNext)
	if !ok {
		return 0, ""
	}
	c.parkingNext++
	c.parking[key] = parkingSlot{spot: spot, via: via, at: now}
	return spot, via
}
