package controller

import (
	"context"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Parking spots on landing (operator 2026-10-16, Senaki: "Lets assign parking
// on landing", then off the Taxi Parking page, spots 1-58: "just assign a
// parking number"). The spots go out in turn when a jet reports clear of the
// runway or asks to taxi to parking; a flight asking again gets the spot it
// already has. With SetGroundTransmitFn the reply is spoken by "<field>
// Ground" in its own voice ("use this one", senaki_6 from round 13).

// parkingHold is how long an assigned spot stays with a callsign.
const parkingHold = 45 * time.Minute

type parkingSlot struct {
	spot int
	at   time.Time
}

// SetGroundTransmitFn wires the Ground position: parking replies are spoken
// as name ("Senaki Ground") through fn.
func (c *ATCController) SetGroundTransmitFn(name string, fn func(ctx context.Context, text string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.groundName, c.groundTransmitFn = name, fn
}

// groundSpeaker is the name a parking reply goes out under; "" = the tower.
// Caller holds c.mu.
func (c *ATCController) groundSpeaker() string {
	if c.groundTransmitFn == nil {
		return ""
	}
	return c.groundName
}

// transmitGroundTo is transmitTo in the Ground voice.
func (c *ATCController) transmitGroundTo(ctx context.Context, callsign, text string) {
	if c.groundTransmitFn == nil {
		c.transmitTo(ctx, callsign, text)
		return
	}
	text = c.flightAddress(callsign, text)
	c.lastTxMu.Lock()
	if c.lastTx == nil {
		c.lastTx = make(map[string]lastTransmission)
	}
	c.lastTx[callsign] = lastTransmission{text: text, at: time.Now()}
	c.lastTxMu.Unlock()
	c.groundTransmitFn(ctx, text)
}

// wantsParking: a taxi request that is going in, not out.
func wantsParking(raw string) bool {
	return containsAny(strings.ToLower(raw), "parking", "to park", "the ramp", "to ramp", "chocks", "the line", "hangar")
}

// assignParking returns the parking spot for callsign, the next one in turn
// on a first call; 0 when the field has none. Caller holds c.mu.
func (c *ATCController) assignParking(callsign string, af *airfield.Airfield, now time.Time) int {
	if af == nil {
		return 0
	}
	if c.parking == nil {
		c.parking = make(map[string]parkingSlot)
	}
	key := compactCallsign(callsign)
	if p, ok := c.parking[key]; ok && now.Sub(p.at) < parkingHold {
		return p.spot
	}
	spot, ok := af.ParkingSpot(c.parkingNext)
	if !ok {
		return 0
	}
	for k, p := range c.parking {
		if now.Sub(p.at) >= parkingHold {
			delete(c.parking, k)
		}
	}
	c.parkingNext++
	c.parking[key] = parkingSlot{spot: spot, at: now}
	return spot
}
