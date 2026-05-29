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

## 3. Hold short (no traffic) — two-stage LUAW + auto-release

**Triggers:** `holding short` · `hold short` · `short of runway` · `at the hold`  *(when no inbound traffic conflict)*

Replaces the legacy single-TX "advise ready" ack: pilot no longer has to make a second "ready for takeoff" call. The controller acks with TX1 (line up and wait) and fires TX2 (cleared for takeoff) automatically after `AutoReleaseDelay` (5s, `pkg/controller/controller.go`). At fire time the auto-release re-checks: no new inbound within `HoldShortRadiusNm`, `DepartureSpacingSec` window honored, this aircraft still at the head of the departure queue, not already cleared by another path. If any re-check fails, the goroutine silently skips and the existing proactive departure-release monitor takes over.

**TX1 (`HoldShortLineUpAndWait`) variants:**
1. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, line up and wait.`
2. `{CALLSIGN}, {TOWER}, line up and wait runway {RUNWAY}.`
3. `{CALLSIGN}, {TOWER}, runway {RUNWAY} line up and wait, will advise.`

**TX2 (`ClearedForTakeoff` at T+5s)** — same composer used by §6.

**Vestigial:** the prior single-TX `HoldShort` composer method (`hold short runway X, number one, advise ready` / `standby` / `you are next for departure`) is left in place at `pkg/composer/composer.go` for a possible future `--no-auto-release` opt-out flag, but no controller path calls it.

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
1. `{CALLSIGN}, {TOWER}, wind {WIND}, runway {RUNWAY}[, traffic on final], cleared for takeoff. Report airborne when clear.`
2. `{CALLSIGN}, {TOWER}, runway {RUNWAY}, wind {WIND}[, traffic on final], you are cleared for takeoff. Report airborne when clear.`
3. `{CALLSIGN}, {TOWER}, cleared for takeoff runway {RUNWAY}, wind {WIND}[, traffic on final], report airborne when clear, have a good flight.`
4. `{CALLSIGN}, {TOWER}, you are number {POSITION} for takeoff, runway {RUNWAY}, wind {WIND}[, traffic on final]. Report airborne when clear.`

---

## 7. Proceed to runway (cleared from hold-short directly to takeoff)

Used when pilot has been holding and tower issues combined "proceed and cleared".

**Responses:**
1. `{CALLSIGN}, {TOWER}, proceed to runway {RUNWAY}, wind {WIND}[, traffic on final runway {RUNWAY}], cleared for takeoff. Report airborne when clear.`
2. `{CALLSIGN}, {TOWER}, enter and line up runway {RUNWAY}, wind {WIND}[, traffic on final runway {RUNWAY}], cleared for takeoff. Report airborne when clear.`
3. `{CALLSIGN}, {TOWER}, runway {RUNWAY} is yours, wind {WIND}[, traffic on final runway {RUNWAY}], cleared for takeoff. Report airborne when clear.`
4. `{CALLSIGN}, {TOWER}, you are number {POSITION} for takeoff, runway {RUNWAY}, wind {WIND}[, traffic on final runway {RUNWAY}]. Report airborne when clear.`

---

## 6b. Verify hold-short position (Tacview position gate)

Fired when `--position-check` is on AND a pilot calls "holding short" but
Tacview shows them more than `HoldShortValidationNm` (0.5 nm) from the
active runway's threshold. Pilot is NOT enqueued — they need to verify
position and re-call. Pilots without Tacview telemetry are not penalized
(the helper fails open).

**Off by default** (`--position-check=false`). Enable for a single
session to observe behavior before relying on it — depends on runway
threshold coordinates being accurate in `pkg/airfield/`.

**Responses (`VerifyHoldShortPosition`):**
1. `{CALLSIGN}, {TOWER}, unable to confirm hold-short position runway {RUNWAY}, say position.`
2. `{CALLSIGN}, {TOWER}, your position does not appear to be at the hold short of runway {RUNWAY}, verify and report.`
3. `{CALLSIGN}, {TOWER}, do not see you at the hold short, runway {RUNWAY}, confirm position.`

A corresponding `level=warn` log line is emitted with the actual distance
so the operator can spot threshold-data mismatches.

---

## 7a. Queue-position suffix

Appended to §5 (Line up and wait), §6/7 hold variants, and §7b (Hold for
spacing) responses when the aircraft is at position 2 or 3 in the
departure queue. Capped at position 3 — positions 4+ get nothing since
they'll hear the ones ahead get cleared first. Once announced, the
`AircraftState.AnnouncedQueuePos` flag is set so subsequent retries
don't re-announce on the same airplane.

**Extra placeholders:** `{POS}` (spelled position, e.g. `two`), `{AHEAD}`
(callsign immediately ahead in queue, e.g. `Raider 032`).

**Suffix variants (appended to the primary hold response):**
1. ` You're number {POS} for departure behind {AHEAD}.`
2. ` Number {POS} in sequence behind {AHEAD}.`
3. ` Sequence number {POS}, behind {AHEAD}.`

**Example combined TX:** `Raider 39, Al Minhad Tower, hold short runway zero niner, departure spacing in forty five seconds. You're number two for departure behind Raider 032.`

---

## 7b. Hold for departure spacing

Fired when a pilot calls `request takeoff` or `holding short` within
`DepartureSpacingSec` (default 60s) of the most recent successful
takeoff clearance on the same airfield. Prevents wake-turbulence /
conflict cascades like the 2026-05-26 Raider 032 + Raider 39
double-clear 23s apart.

The pilot stays in the departure queue — the controller's proactive
monitor will re-attempt the clearance on its next tick once the
spacing window elapses, so the pilot does not need to re-request.

**`{SECS_LEFT}`** — remaining cooldown in whole seconds, spelled
(e.g. `forty five`).

**Responses (`HoldForSpacing`):**
1. `{CALLSIGN}, {TOWER}, hold short runway {RUNWAY}, departure spacing in {SECS_LEFT} seconds.`
2. `{CALLSIGN}, {TOWER}, hold short {RUNWAY}, traffic spacing, will advise in {SECS_LEFT} seconds.`
3. `{CALLSIGN}, {TOWER}, hold short, departure spacing {SECS_LEFT} seconds.`

---

## 8. Departure release / clear of pattern

**Triggers:** `clear traffic` · `clear of traffic` · `airborne` · `departing` · `seven dme` · `7 dme` · `seven miles` · `cleared airspace` · `five miles`

**Extra placeholders:** `{DIST}` (word form, e.g. `seven` — pulled from airfield config, currently `7` for all towers), `{ANGELS}` (random 3/4/5/6 per call — angels 7 was too high in live ops vs UAE MOA/CAS floors, range narrowed 2026-05-27)

**Responses:**
1. `{CALLSIGN}, {TOWER}, proceed to angels {ANGELS}, contact tower at {DIST} DME.`
2. `{CALLSIGN}, {TOWER}, climb to angels {ANGELS}, contact tower at {DIST} DME.`
3. `{CALLSIGN}, {TOWER}, angels {ANGELS}, contact tower at {DIST} DME.`

---

## 9. Handoff to Command (282.0)

Fires after departure release (`7 DME` / `cleared airspace` etc).

**Extra placeholders:** `{HANDOFF}` (`vSFG-7-Command`), `{FREQ}` (`two eight two point zero`), `{PRESET}` (e.g. `preset 5`)

**Responses:**
1. `{CALLSIGN}, {TOWER}, contact {HANDOFF}, {FREQ}, {PRESET}. Good day.`
2. `{CALLSIGN}, {TOWER}, switch to {HANDOFF}, {FREQ}, {PRESET}. Safe skies.`
3. `{CALLSIGN}, {TOWER}, frequency change approved, {HANDOFF} on {FREQ}, {PRESET}.`

---

## 10. Pushing Command Acknowledge

Pilot-initiated courtesy call announcing they're already switching to Command. Short ack only — freq/preset not reissued (already given in #9).

**Triggers:** `pushing command` · `pushing to command` · `switching command` · `switching to command` · `push command`

**Responses:**
1. `{CALLSIGN}, {TOWER}, cleared handoff to command, good day.`
2. `{CALLSIGN}, {TOWER}, roger pushing command, good day.`
3. `{CALLSIGN}, {TOWER}, copy switch to command, good day.`

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
