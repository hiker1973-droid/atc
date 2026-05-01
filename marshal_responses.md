# Marshal Responses (306.3 MHz, OMDM carrier marshal stack)

Edit response text and send back. Ports to `pkg/composer/composer.go` (Marshal section) and `cmd/atc/marshal.go` (intent handler). `v1.0.2` for text edits, `v1.1.0` for new intents.

## Format

Triggers from `cmd/atc/marshal.go` `handleMarshalCall`. Responses from `pkg/composer/composer.go` (Marshal* methods). 3 variants random pick.

**Placeholders:**
- `{CALLSIGN}` — pilot callsign
- `{ANGELS}` — assigned stack altitude in thousands, spelled (e.g. `seven`)
- `{ALTIMETER}` — altimeter setting (e.g. `two niner point niner two`)
- `{BRC}` — base recovery course (carrier heading), 3 digits (e.g. `135`); omitted if unknown
- `{POS}` — position in stack
- `{DIST}` — distance in nautical miles, spelled
- `{STATE}` — fuel state (e.g. `4.2`)

**Recovery case** auto-derived from ceiling: `Case One` (≥3000 ft), `Case Two` (≥1000 ft), `Case Three` (<1000 ft).

---

## 1. Marking mom (initial check-in)

**Triggers:** `marking mom` · `marking moms`

This is the pilot's first call to Marshal. Composer also reports a stack-summary line (`Stack has N aircraft.`) appended when other aircraft are present.

**Responses (`MarshalMarkingMom`):**
1. `{CALLSIGN}, Marshal, mother's weather is clear, visibility ten, expect {CASE} recovery, BRC {BRC}, altimeter {ALTIMETER}, Marshal angels {ANGELS}, report see me at ten.`
2. `{CALLSIGN}, Marshal, {CASE} recovery, BRC {BRC}, altimeter {ALTIMETER}, stack angels {ANGELS}, report see me at ten.`
3. `{CALLSIGN}, Marshal, {CASE}, BRC {BRC}, altimeter {ALTIMETER}, your angels are {ANGELS}, report see me at ten.`

When BRC unknown, the `, BRC {BRC}` clause is omitted.

---

## 2. See me at 10 / radar contact

**Triggers:** `see you at 10` · `see you at ten`

**Responses (`MarshalRadarContact`):**
1. `{CALLSIGN}, Marshal, radar contact, {DIST} miles, say state.`
2. `{CALLSIGN}, Marshal, contact, {DIST} miles, say state.`
3. `{CALLSIGN}, Marshal, got you at {DIST} miles, say state.`

(Hardcoded to 10 nm in current handler — `MarshalRadarContact(callsign, 10)`.)

---

## 3. Fuel state report

**Triggers:** `state` (containing a numeric fuel value)

**Bingo / low state (<2.0):**
1. `{CALLSIGN}, Marshal, state {STATE}, expedite recovery.`
2. `{CALLSIGN}, Marshal, copy state {STATE}, you are priority.`
3. `{CALLSIGN}, Marshal, state {STATE}, priority recovery.`

**Normal state (≥2.0):**
1. `{CALLSIGN}, Marshal, copy state {STATE}.`
2. `{CALLSIGN}, Marshal, state {STATE}, copy.`
3. `{CALLSIGN}, Marshal, roger, state {STATE}.`

---

## 4. Established in stack

**Triggers:** `established angels` · `established at angels`

**If deck is clear → Signal Charlie (cleared to commence):**
1. `{CALLSIGN}, Marshal, signal Charlie.`
2. `{CALLSIGN}, Marshal, you have Charlie.`
3. `{CALLSIGN}, Marshal, Charlie.`

**If deck is busy → ack and hold:**
1. `{CALLSIGN}, Marshal, roger, hold angels {ANGELS}.`
2. `{CALLSIGN}, Marshal, established angels {ANGELS}, copy.`
3. `{CALLSIGN}, Marshal, angels {ANGELS}, stand by for Charlie.`

---

## 5. Commencing approach

**Triggers:** `commencing`

**If pilot also reports state:**
1. `{CALLSIGN}, Marshal, copy commencing, state {STATE}.`
2. `{CALLSIGN}, Marshal, commencing, state {STATE}, copy.`
3. `{CALLSIGN}, Marshal, roger, commencing, state {STATE}.`

**Without state:**
1. `{CALLSIGN}, Marshal, copy commencing.`
2. `{CALLSIGN}, Marshal, commencing, copy.`
3. `{CALLSIGN}, Marshal, roger, commencing.`

---

## 6. 3NM Initial → push button (TACAN handoff)

**Triggers:** `initial`

Per Case 1 Recovery comms (vSFG-7 07.png): pilot calls `[MODEX], INITIAL` at 3nm; Marshal hands off to LSO/Paddles. This is Marshal's final transmission to the pilot — the subsequent "pushing button XX" / "checking in" / "contact" exchange happens on the LSO freq (not yet implemented).

**Responses (`MarshalPushButton`):**
1. `{CALLSIGN}, Marshal, push button {TACAN_CH}, check in.`
2. `{CALLSIGN}, Marshal, button {TACAN_CH}, check in.`
3. `{CALLSIGN}, Marshal, push button {TACAN_CH} and check in.`

(`marshalTacanChannel` constant is currently 72 in marshal.go — let me know if you want a different default or a flag.)

---

## 7. Marshal contact (LSO confirmation — UNWIRED)

Composed but not wired. Per the Case 1 Recovery comms (07.png) the "contact" call comes from the LSO/Paddles role on a separate SRS button after the pilot pushes off Marshal — it is not a Marshal transmission. Keep this composer method around for the eventual LSO handler; do not wire it into Marshal.

**`MarshalContact`:**
1. `{CALLSIGN}, Marshal, contact.`
2. `{CALLSIGN}, Marshal, you have contact.`
3. `{CALLSIGN}, Marshal, contact, good luck.`

---

## Notes

- Marshal handles 6 intents now: Marking mom, See you at 10, State, Established, Commencing, 3NM Initial. The composer has 8 methods total — `MarshalContact` is intentionally unwired (it belongs to the LSO/Paddles role on a separate SRS button, see Section 7).
- Marshal stack altitude assignment is auto-deconflicted via `atcCtrl.GetDeconflictedAngels(baseAngels)` — base angels = 5 + position in stack, then atcCtrl checks Tacview for collisions and bumps if needed. Stack altitude logic is *not* in this responses file; that lives in `pkg/state/state.go` and `pkg/controller/controller.go`.

## Case 1 Recovery comms coverage (vSFG-7 07.png)

| Stage | Pilot call | Marshal response | Wired? |
|---|---|---|---|
| Before 50nm | `marking moms, [DIST], angels [XX], state [XX]` | mother's weather, expect Case 1, altimeter, stack angels, report see me at 10 | ✅ §1 |
| 10nm | `see you at 10` | radar contact, [DIST] miles, say state | ✅ §2 |
| 10nm | `state [XX]` | copy state | ✅ §3 |
| Stack | `established angels [XX], position [X]` | signal Charlie / hold | ✅ §4 |
| Commencing | `commencing, state [XX]` | copy commencing | ✅ §5 |
| 3NM Initial | `initial` | push button [XX], check in | ✅ §6 |
| Push freq | `pushing button [XX]` / `checking in` | LSO `contact` (separate role/freq) | ⏳ LSO not implemented |
