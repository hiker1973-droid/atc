# Tower Arrival Responses

Edit any response text and send back. I'll port to `pkg/composer/composer.go` and ship as `v1.0.2` (text edits) or `v1.1.0` (new intents).

## Format

Triggers from `pkg/controller/controller.go`. Responses from `pkg/composer/composer.go`. Each composer method picks 1 of 3 variants at random.

**Placeholders:**
- `{CALLSIGN}` — pilot callsign (e.g. `Raider 032`)
- `{TOWER}` — tower callsign (`Al Minhad Tower`, `Al Dhafra Tower`, `Al Ain Tower`)
- `{RUNWAY}` — active runway, spelled (e.g. `two seven`)
- `{WIND}` — `[direction] at [knots]` or `calm` if <3 kts
- `{ALTIMETER}` — `[whole] point [hundredths]`
- `{SEQ}` — sequence number spelled (`one`, `two`, `three`...)
- `{MODEX}` — last-word of callsign (e.g. `032` from `Raider 032`)

---

## 1. Distance initial (10+ mile inbound report)

**Triggers:** `[N] mile initial` · `[N] miles inbound` · `inbound`

**Day VMC variants** (`DistanceInitialAck`):
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, Number {SEQ}.`
2. `{CALLSIGN}, {TOWER}, runway {RUNWAY} in use, Number {SEQ}.`
3. `{CALLSIGN}, {TOWER}, roger, runway {RUNWAY}, Number {SEQ}.`

**Sequenced (with traffic ahead, Tacview-aware)** — extra `{LEAD_CALLSIGN}`, `{LEAD_DIST}`:
1. `{CALLSIGN}, {TOWER}, number {SEQ}, follow {LEAD_CALLSIGN}, {LEAD_DIST} miles in trail, runway {RUNWAY}.`
2. `{CALLSIGN}, {TOWER}, number {SEQ}, traffic is {LEAD_CALLSIGN}, {LEAD_DIST} miles ahead, runway {RUNWAY}.`
3. `{CALLSIGN}, number {SEQ}, follow {LEAD_CALLSIGN}, runway {RUNWAY}.`

**Night/IFR variants:**
1. `{CALLSIGN}, {TOWER}, got you at {DIST} mile initial, night ops in effect, runway lights are on, proceed angels {ANGELS}, altimeter {ALTIMETER}, enter pattern runway {RUNWAY}.{TRAFFIC}`
2. `{CALLSIGN}, {TOWER}, {DIST} mile initial, night conditions, field is lit, angels {ANGELS}, altimeter {ALTIMETER}, runway {RUNWAY} active.{TRAFFIC}`
3. `{CALLSIGN}, {TOWER}, roger {DIST} mile initial, night ops, runway {RUNWAY} lights on, altimeter {ALTIMETER}, proceed angels {ANGELS}.{TRAFFIC}`

**IMC redirect variants** (when ceiling too low for VFR pattern):
1. `{CALLSIGN}, {TOWER}, got you at {DIST} miles, IMC conditions in effect, straight-in only runway {RUNWAY}, altimeter {ALTIMETER}[, number {SEQ}], report ten mile final.`
2. `{CALLSIGN}, {TOWER}, {DIST} miles, IMC in effect, execute straight-in runway {RUNWAY}, altimeter {ALTIMETER}[, number {SEQ}], call ten mile final.`
3. `{CALLSIGN}, {TOWER}, roger {DIST} miles, no pattern available IMC, straight-in runway {RUNWAY}[, number {SEQ}], altimeter {ALTIMETER}, report ten mile final.`

---

## 2. Inbound (general inbound, no specific distance)

**Triggers:** `inbound` (without `initial` / `mile`)

**Variants:**
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, wind {WIND}, altimeter {ALTIMETER}. {TRAFFIC} Report final.`
2. `{CALLSIGN}, {TOWER}, altimeter {ALTIMETER}, active runway {RUNWAY}, wind {WIND}. {TRAFFIC} Report final.`
3. `{CALLSIGN}, {TOWER}, field information: runway {RUNWAY}, wind {WIND}, altimeter {ALTIMETER}. {TRAFFIC} Call final.`

---

## 3. Overhead (initial overhead, ≤3 mile initial)

**Triggers:** `overhead` · `over the field` · `initial overhead` · `[≤3] mile initial`

**Standard pattern entry** (runway-aware break direction):

`{BREAK}` is read from `Airfield.BreakDirections[activeRunway]` — `"left"` or `"right"` per the table below. Composer falls back to `"left"` if the active runway has no entry.

| ICAO | Runway | Break | Reasoning |
|---|---|---|---|
| OMDM | 27 | left | Ramp on south side; heading west, south is on the left |
| OMDM | 09 | right | Reverse heading, south is now on the right |
| OMAM | 31L / 31R | left | Ramp on south/east; heading NW, south is on the left |
| OMAM | 13L / 13R | right | Reverse heading, south is now on the right |
| OMAL | 19 | right | Terminal on west side; heading south, west is on the right |
| OMAL | 01 | left | Reverse heading, west is now on the left |

Variants:
1. `{CALLSIGN}, {TOWER}, approved {BREAK} break runway {RUNWAY}, number {SEQ}, report base, final.`
2. `{CALLSIGN}, {TOWER}, number {SEQ}, runway {RUNWAY}, {BREAK} break approved, report base, final.`
3. `{CALLSIGN}, {TOWER}, roger initial, {BREAK} break approved runway {RUNWAY}, number {SEQ}, report base, final.`

**Overhead break (no traffic ahead):**
1. `{CALLSIGN}, {TOWER}, number {SEQ}, report break.`
2. `{CALLSIGN}, {TOWER}, you are number {SEQ}, break when ready, report break.`
3. `{CALLSIGN}, {TOWER}, cleared break runway {RUNWAY}, report break.`

**Overhead break (traffic ahead):**
1. `{CALLSIGN}, {TOWER}, number {SEQ}, follow traffic in the break, report break.`
2. `{CALLSIGN}, {TOWER}, number {SEQ}, traffic in the break ahead of you, report break.`
3. `{CALLSIGN}, {TOWER}, you are number {SEQ}, follow the break, report when you break.`

---

## 4. Break

**Triggers:** `break`  *(unless transmission also contains "radio" or "check")*

**No traffic ahead:**
1. `{CALLSIGN}, {TOWER}, number {SEQ}, report downwind.`
2. `{CALLSIGN}, {TOWER}, roger break, number {SEQ}, report downwind.`
3. `{CALLSIGN}, {TOWER}, break acknowledged, report downwind runway {RUNWAY}.`

**Traffic ahead:**
1. `{CALLSIGN}, {TOWER}, number {SEQ} for runway {RUNWAY}, report downwind.`
2. `{CALLSIGN}, {TOWER}, roger, number {SEQ}, follow traffic, report downwind.`
3. `{CALLSIGN}, {TOWER}, break acknowledged, you are number {SEQ}, report downwind.`

---

## 5. Downwind

**Triggers:** `downwind`

**Variants:**
1. `{CALLSIGN}, {TOWER}, number {SEQ}, report base.`
2. `{CALLSIGN}, number {SEQ}, report base.`
3. `{CALLSIGN}, {TOWER}, number {SEQ}, call base.`

**Extend downwind (traffic spacing):**
1. `{CALLSIGN}, {TOWER}, extend downwind, maintain altitude, I'll call your base.`
2. `{CALLSIGN}, {TOWER}, hold your downwind, traffic ahead on final, stand by for base.`
3. `{CALLSIGN}, {TOWER}, continue downwind and hold altitude, spacing with traffic ahead, I'll call your base.`

**IMC continue (no separate downwind in IMC):**
1. `{CALLSIGN}, {TOWER}, roger, continue approach, report final.`
2. `{CALLSIGN}, {TOWER}, continue approach, call final.`
3. `{CALLSIGN}, {TOWER}, maintain approach, advise final.`

---

## 6. Base

**Triggers:** `base` · `turning base` · `right base` · `left base` · `base final`

**Day variants** (use modex only — terse, realistic):
1. `{MODEX}, affirmative.`
2. `{MODEX}, {TOWER}, affirmative.`
3. `Affirmative, {MODEX}.`

**Night variants** (with gear reminder):
1. `{MODEX}, affirmative, check gear down.`
2. `{MODEX}, {TOWER}, affirmative, gear check.`
3. `Affirmative, {MODEX}, confirm gear down.`

---

## 7. Cleared to land (final)

**Triggers:** `on final` · `final` · `request landing` · `cleared to land`

**Day variants:**
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, cleared to land.`
2. `{CALLSIGN}, {TOWER}, cleared to land runway {RUNWAY}.`
3. `{CALLSIGN}, runway {RUNWAY}, cleared to land.`

**Night variants** (with gear check):
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, check gear, cleared to land.`
2. `{CALLSIGN}, {TOWER}, gear check, runway {RUNWAY}, cleared to land.`
3. `{CALLSIGN}, gear down, runway {RUNWAY}, cleared to land.`

**Sequenced (previous aircraft just vacated):**
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY} clear, cleared to land.`
2. `{CALLSIGN}, {TOWER}, runway clear, cleared to land runway {RUNWAY}.`
3. `{CALLSIGN}, runway {RUNWAY} clear, cleared to land.`

---

## 8. Straight-in (IFR / instrument approach)

**Triggers:** `straight in` · `straight-in` · `ils` · `instrument approach` · `rnav`

**No traffic ahead:**
1. `{CALLSIGN}, {TOWER}, straight-in runway {RUNWAY} approved, altimeter {ALTIMETER}, report final.`
2. `{CALLSIGN}, {TOWER}, cleared straight-in approach runway {RUNWAY}, altimeter {ALTIMETER}, call final.`
3. `{CALLSIGN}, {TOWER}, roger, straight-in runway {RUNWAY}, altimeter {ALTIMETER}, advise final.`

**With traffic ahead:**
1. `{CALLSIGN}, {TOWER}, straight-in runway {RUNWAY} approved, altimeter {ALTIMETER}, number {SEQ}, reduce to slowest practical speed, report final.`
2. `{CALLSIGN}, {TOWER}, cleared straight-in runway {RUNWAY}, number {SEQ}, altimeter {ALTIMETER}, reduce speed, call final.`
3. `{CALLSIGN}, {TOWER}, number {SEQ} straight-in runway {RUNWAY}, altimeter {ALTIMETER}, slow to approach speed, report final.`

**Redirect to straight-in (when pilot wants overhead but IMC):**
1. `{CALLSIGN}, {TOWER}, negative overhead break, IMC conditions, straight-in only runway {RUNWAY}, altimeter {ALTIMETER}, report ten mile final.`
2. `{CALLSIGN}, {TOWER}, overhead break not authorized, IMC in effect, execute straight-in runway {RUNWAY}, altimeter {ALTIMETER}, report ten mile final.`
3. `{CALLSIGN}, {TOWER}, pattern not available, IMC conditions, straight-in runway {RUNWAY} approved, altimeter {ALTIMETER}, call ten mile final.`

---

## 9. Runway vacated

**Triggers:** `runway vacated` · `vacated` · `clear of runway` · `off the runway` · `clear runway` · `cleared runway` · `clearing runway` · `runway clear` · `runway cleared` · `cleared the runway` · `clear of` · `clear active` · `clear of active` · `clearing active` · `runway is clear` · `runway is vacated` · `off runway` · `off the active` · `exiting runway` · `exited runway` · `runway exit` · `leaving runway` · `left the runway` · `clear active runway`

**Variants:**
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY} clear, taxi to parking.`
2. `{CALLSIGN}, {TOWER}, roger clear of runway {RUNWAY}, taxi to parking at your discretion.`
3. `{CALLSIGN}, {TOWER}, runway {RUNWAY} is clear, welcome back, taxi to parking.`

---

## 10. Go-around

**Triggers:** `going around` · `go around` · `missed approach`

**Pilot-initiated:**
1. `{CALLSIGN}, go around, {TOWER}, climb runway heading, report downwind runway {RUNWAY}.`
2. `{CALLSIGN}, {TOWER}, go around, climb and maintain pattern altitude, report downwind.`
3. `{CALLSIGN}, go around immediately, {TOWER}, fly runway heading, climb to pattern altitude, report downwind runway {RUNWAY}.`

**Tower-initiated (conflict with departure)** — extra `{DEP_CALLSIGN}`:
1. `{CALLSIGN}, go around, {TOWER}, {DEP_CALLSIGN} is rolling runway {RUNWAY}, climb and maintain pattern altitude, report downwind.`
2. `{CALLSIGN}, go around immediately, {TOWER}, traffic departing runway {RUNWAY}, {DEP_CALLSIGN}, fly runway heading, report downwind.`
3. `{CALLSIGN}, {TOWER}, go around, {DEP_CALLSIGN} departing below you runway {RUNWAY}, climb to pattern altitude, report downwind.`

---

## 11. Emergency

**Triggers:** `mayday` · `pan pan` · `emergency` · `declaring emergency`

**Variants:**
1. `{CALLSIGN}, {TOWER}, emergency acknowledged, runway {RUNWAY} is yours, wind {WIND}, altimeter {ALTIMETER}, cleared to land, crash crew is standing by.`
2. `{CALLSIGN}, {TOWER}, roger mayday, all traffic hold, runway {RUNWAY} cleared, wind {WIND}, altimeter {ALTIMETER}, cleared immediate landing, emergency services are alerted.`
3. `{CALLSIGN}, {TOWER}, emergency acknowledged, you have runway {RUNWAY}, wind {WIND}, altimeter {ALTIMETER}, cleared to land, say souls on board and fuel state.`

---

## 12. Traffic in sight / Negative contact

### Traffic in sight
**Triggers:** `traffic in sight` · `visual` · `tally`
1. `{CALLSIGN}, {TOWER}, maintain visual separation, report final.`
2. `{CALLSIGN}, {TOWER}, roger, maintain visual, call final.`
3. `{CALLSIGN}, {TOWER}, maintain visual separation with traffic, report final.`

### Negative contact (traffic still a factor)
**Triggers:** `negative contact` · `no contact` · `no joy`
1. `{CALLSIGN}, {TOWER}, traffic is {TRAFFIC_DESC}, continue search, report traffic in sight or final.`
2. `{CALLSIGN}, {TOWER}, traffic {TRAFFIC_DESC}, look again, advise visual contact or call final.`
3. `{CALLSIGN}, {TOWER}, unable to confirm traffic contact, {TRAFFIC_DESC}, continue approach and advise.`

### Negative contact (traffic no longer a factor)
1. `{CALLSIGN}, {TOWER}, traffic no longer a factor, continue approach.`
2. `{CALLSIGN}, {TOWER}, disregard traffic, continue approach.`
3. `{CALLSIGN}, {TOWER}, traffic is no longer a factor, continue.`

---

## 13. Altitude check / altimeter check

**Triggers:** `request altitude` · `altitude check` · `altitude request` · `what altitude` · `assigned altitude`

**With Tacview altitude data:**
1. `{CALLSIGN}, {TOWER}, radar shows you at {ACTUAL_FT} feet, altimeter {ALTIMETER}, maintain VFR.`
2. `{CALLSIGN}, {TOWER}, indicating {ACTUAL_FT} feet, altimeter {ALTIMETER}.`
3. `{CALLSIGN}, {TOWER}, you are at {ACTUAL_FT} feet, altimeter {ALTIMETER}, maintain VFR.`

**Without Tacview data:**
1. `{CALLSIGN}, {TOWER}, altimeter {ALTIMETER}, maintain VFR.`
2. `{CALLSIGN}, {TOWER}, altimeter {ALTIMETER}, VFR altitude at pilot discretion.`
3. `{CALLSIGN}, {TOWER}, altimeter {ALTIMETER}, no altitude restrictions.`

**Proactive altimeter (tower-initiated):**
1. `{CALLSIGN}, {TOWER}, check altimeter, {ALTIMETER}.`
2. `{CALLSIGN}, {TOWER}, altimeter {ALTIMETER}, acknowledge.`
3. `{CALLSIGN}, {TOWER}, set altimeter {ALTIMETER}.`

---

## 14. Speed warning (proactive, ≥350 kt inside 10 nm)

Tower fires this autonomously when Tacview shows speed violation.

**Variants:**
1. `{CALLSIGN}, {TOWER}, speed check, you are fast inside ten miles, reduce to three five zero or below.`
2. `{CALLSIGN}, {TOWER}, caution airspeed, reduce speed, three five zero knots max inside ten miles.`
3. `{CALLSIGN}, {TOWER}, you are showing fast, reduce to three five zero immediately, inside ten miles.`

---

## 15. Hold for sequence (no spot in pattern)

Used when pattern is saturated.

**Sequence position 0 (no slot at all):**
1. `{CALLSIGN}, {TOWER}, hold outside the pattern at current altitude, runway {RUNWAY} is busy, stand by for sequence.`
2. `{CALLSIGN}, {TOWER}, hold clear of the pattern, maintain altitude, expect a five minute delay.`
3. `{CALLSIGN}, {TOWER}, unable to sequence at this time, hold outside, runway {RUNWAY} is occupied.`

**Sequence position N>0:**
1. `{CALLSIGN}, {TOWER}, you are number {SEQ} for runway {RUNWAY}, extend downwind and maintain altitude, I'll call your base.`
2. `{CALLSIGN}, {TOWER}, number {SEQ} for runway {RUNWAY}, hold your downwind, stand by for base call.`
3. `{CALLSIGN}, {TOWER}, number {SEQ} in sequence, runway {RUNWAY}, extend downwind and hold altitude.`

---

## 16. Short final sequenced

When pilot calls short final but there's traffic ahead.

1. `{CALLSIGN}, {TOWER}, number {SEQ}, report short final.`
2. `{CALLSIGN}, {TOWER}, number {SEQ} for runway {RUNWAY}, short final.`
3. `{CALLSIGN}, number {SEQ}, short final runway {RUNWAY}.`

---

## 17. Radar check (Tacview-aware)

Pilot asks tower for a position read-back. Tower pulls live state from Tacview (`pkg/controller/controller.go` — `TacviewContact` map) and replies with angels, range, and true bearing from the tower.

**Triggers:** `radar check` · `radar contact` · `request radar` · `radar service`

**Placeholders:**
- `{ANGELS}` — altitude in thousands of feet, spelled (e.g. `three` for 3000 ft)
- `{DIST}` — range from tower in nm, spelled (`twelve miles`, uses `milesToWord`)
- `{BEARING}` — true bearing FROM tower TO aircraft, spelled three-digit (`two seven zero`)

**With Tacview contact** (`RadarCheck`):
1. `{CALLSIGN}, {TOWER}, you are at angels {ANGELS} and {DIST} from tower on a bearing of {BEARING}.`
2. `{CALLSIGN}, {TOWER}, radar contact, angels {ANGELS}, {DIST}, bearing {BEARING} from the field.`
3. `{CALLSIGN}, {TOWER}, I have you angels {ANGELS}, range {DIST}, bearing {BEARING}.`

**No Tacview contact** (`RadarCheckNoContact` — aircraft not yet seen by Tacview):
1. `{CALLSIGN}, {TOWER}, negative radar contact, say position.`
2. `{CALLSIGN}, {TOWER}, no radar contact at this time, say position and altitude.`
3. `{CALLSIGN}, {TOWER}, unable radar contact, say your position.`
