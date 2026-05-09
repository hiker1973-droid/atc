# Tower Departure Responses

Edit any response text and send back. I'll port to `pkg/composer/composer.go` and ship as `v1.0.2` (text edits) or `v1.1.0` (new intents).

## Format

Triggers come from `pkg/controller/controller.go` (the intent classifier). Responses come from `pkg/composer/composer.go`. Each composer method has 3 variants chosen at random.

**Placeholders** filled at runtime:
- `{CALLSIGN}` — pilot callsign (e.g. `Raider 032`)
- `{TOWER}` — tower callsign (e.g. `Al Minhad Tower`, `Al Dhafra Tower`, `Al Ain Tower`)
- `{RUNWAY}` — active runway, spelled out (e.g. `two seven`)
- `{WIND}` — `[direction] at [knots]` (e.g. `two seven zero at one five knots`) or `calm` if <3 kts
- `{ALTIMETER}` — `[whole] point [hundredths]` (e.g. `two niner point niner two`)
- `{TRAFFIC}` — counted-aircraft phrase (e.g. `One aircraft in the pattern ahead of you.`)

---

## 1. Startup approval

**Triggers:** `request startup` · `ready for startup` · `ready to start` · `request start`

**Note:** Response uses the **Ground** callsign (derived from the tower callsign by replacing "Tower" with "Ground") since engine-start clearance is conventionally a ground-control task.

**Responses:**
1. `{CALLSIGN}, {GROUND}, startup approved.`
2. `{CALLSIGN}, {GROUND}, startup approved, altimeter {ALTIMETER}, advise ready to taxi.`
3. `{CALLSIGN}, {GROUND}, startup at your discretion, advise when ready to taxi.`

---

## 2. Taxi clearance

**Triggers:** `request taxi` · `request ground` · `taxi to` · `ready to taxi`

**Responses:**
1. `{CALLSIGN}, {TOWER}, taxi to runway {RUNWAY}, altimeter {ALTIMETER}, hold short, advise ready.`
2. `{CALLSIGN}, {TOWER}, altimeter {ALTIMETER}, cleared to taxi runway {RUNWAY}, hold short and call ready.`
3. `{CALLSIGN}, {TOWER}, roger, altimeter {ALTIMETER}, taxi runway {RUNWAY}, hold short of the runway, advise when ready for takeoff.`

---

## 3. Hold short (no traffic)

**Triggers:** `holding short` · `hold short` · `short of runway` · `at the hold`  *(when no inbound traffic conflict)*

**Responses:**
1. `{CALLSIGN}, {TOWER}, hold short runway {RUNWAY}, number one, advise ready.`
2. `{CALLSIGN}, {TOWER}, hold short of runway {RUNWAY}, standby.`
3. `{CALLSIGN}, {TOWER}, hold your position short of runway {RUNWAY}, you are next for departure.`

---

## 4. Hold short — traffic on final

**Triggers:** same as #3, but fires when an inbound aircraft is on final.

**Extra placeholders:** `{TRAFFIC_CALLSIGN}`, `{TRAFFIC_DIST}` (miles, integer)

**Responses:**
1. `{CALLSIGN}, {TOWER}, hold short runway {RUNWAY}, {TRAFFIC_CALLSIGN} is {TRAFFIC_DIST} miles on final.`
2. `{CALLSIGN}, {TOWER}, hold position runway {RUNWAY}, traffic on {TRAFFIC_DIST} mile final.`
3. `{CALLSIGN}, {TOWER}, hold short, {TRAFFIC_DIST} mile final traffic, {TRAFFIC_CALLSIGN}, will advise.`

---

## 5. Line up and wait (close-spacing departure with traffic on final)

**Triggers:** same as #3, fires when traffic is far enough out to allow LUAW.

**Responses:**
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, line up and wait, {TRAFFIC_CALLSIGN} is {TRAFFIC_DIST} miles final.`
2. `{CALLSIGN}, {TOWER}, line up and wait runway {RUNWAY}, traffic {TRAFFIC_DIST} miles on final.`
3. `{CALLSIGN}, {TOWER}, pull forward, runway {RUNWAY}, line up and wait, {TRAFFIC_DIST} mile final {TRAFFIC_CALLSIGN}.`

---

## 6. Cleared for takeoff

**Triggers:** `request takeoff` · `request departure` · `ready for departure` · `ready for takeoff` · `lineup`  *(or after holding-short ready call)*

**Optional `, traffic on final`** appended when traffic is in pattern.

**Responses:**
1. `{CALLSIGN}, {TOWER}, wind {WIND}, runway {RUNWAY}[, traffic on final], cleared for takeoff.`
2. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, wind {WIND}[, traffic on final], you are cleared for takeoff.`
3. `{CALLSIGN}, {TOWER}, cleared for takeoff runway {RUNWAY}, wind {WIND}[, traffic on final], have a good flight.`

---

## 7. Proceed to runway (cleared from hold-short directly to takeoff)

Used when pilot has been holding and tower issues combined "proceed and cleared".

**Responses:**
1. `{CALLSIGN}, {TOWER}, proceed to runway {RUNWAY}, wind {WIND}[, traffic on final runway {RUNWAY}], cleared for takeoff.`
2. `{CALLSIGN}, {TOWER}, enter and line up runway {RUNWAY}, wind {WIND}[, traffic on final runway {RUNWAY}], cleared for takeoff.`
3. `{CALLSIGN}, {TOWER}, runway {RUNWAY} is yours, wind {WIND}[, traffic on final runway {RUNWAY}], cleared for takeoff.`

---

## 8. Departure release / clear of pattern

**Triggers:** `clear traffic` · `clear of traffic` · `airborne` · `departing` · `seven dme` · `7 dme` · `seven miles` · `cleared airspace` · `five miles`

**Extra placeholders:** `{DIST}` (word form, e.g. `seven`), `{ANGELS}` (e.g. `five`)

**Responses:**
1. `{CALLSIGN}, {TOWER}, proceed to angels {ANGELS}, {DIST} DME, frequency change approved, good day.`
2. `{CALLSIGN}, {TOWER}, climb angels {ANGELS}, {DIST} DME, freq change approved, navigate at will.`
3. `{CALLSIGN}, {TOWER}, roger airborne, angels {ANGELS} at {DIST} DME, then on your own. Frequency change approved, safe flight.`

---

## 9. Handoff to Command (282.0)

Fires after departure release.

**Extra placeholders:** `{HANDOFF}` (`vSFG-7-Command`), `{FREQ}` (`two eight two point zero`), `{PRESET}` (e.g. `preset 5`)

**Responses:**
1. `{CALLSIGN}, {TOWER}, contact {HANDOFF}, {FREQ}, {PRESET}. Good day.`
2. `{CALLSIGN}, {TOWER}, switch to {HANDOFF}, {FREQ}, {PRESET}. Safe skies.`
3. `{CALLSIGN}, {TOWER}, frequency change approved, {HANDOFF} on {FREQ}, {PRESET}.`

---

## Common (also used on the arrivals side)

### Radio check (tower)
**Triggers:** `radio check` · `comm check` · `comms check` · `comcheck` · `comp check` · `how copy`
1. `{CALLSIGN}, {TOWER}, loud and clear.`
2. `{CALLSIGN}, {TOWER}, five by five, go ahead.`
3. `{CALLSIGN}, {TOWER}, reading you loud and clear.`

### Unable to understand (fallback)
1. `{CALLSIGN}, {TOWER}, say again your request.`
2. `{CALLSIGN}, {TOWER}, unable to copy, say again.`
3. `{CALLSIGN}, {TOWER}, you were broken, say again.`
