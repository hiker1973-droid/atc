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
- `{WX}` — weather phrase derived from live ceiling + visibility (see below)
- `{R_ANG}` — caller's altitude in thousands as seen on Tacview, spelled
- `{R_DIST}` — caller's range from carrier in nm, spelled
- `{R_BRG}` — true bearing FROM the carrier TO the caller (cardinal/intercardinal word)

**Recovery case** auto-derived from ceiling: `Case One` (≥3000 ft or unknown), `Case Two` (≥1000 ft), `Case Three` (<1000 ft).

**`{WX}` (mother's weather)** — built from live `ceilingFt` + `visNm`:
- Ceiling unknown or ≥10 000 ft → `ceiling unrestricted, visibility ten plus`
- Ceiling ≥3 000 ft → `ceiling X thousand scattered, visibility ...`
- Ceiling ≥1 000 ft → `ceiling X thousand broken, visibility ...`
- Ceiling <1 000 ft → `ceiling X hundred overcast, visibility ...`
- Visibility: `ten plus` (≥10 nm), spelled int (3–9), digit (<3), `less than one` (<1)

---

## 1. Marking mom (initial check-in)

**Triggers:** `marking mom` · `marking moms`

Pilot's first call to Marshal. Composer also appends a stack-summary line (`Stack has N aircraft.`) when other aircraft are present. Response always carries: weather, recovery case, BRC, altimeter, stack assignment, and the "see me at ten" instruction. When the caller is visible on Tacview a `{RADAR}` clause is inserted right after `Marshal,` giving live angels/range/bearing from mother.

**`{RADAR}` clause** — randomly picks one of:
1. ` radar contact, angels {R_ANG}, range {R_DIST}, bearing {R_BRG} from mother,`
2. ` I have you on radar, angels {R_ANG}, {R_DIST} from mother, bearing {R_BRG},`
3. ` radar contact angels {R_ANG}, {R_DIST} on the {R_BRG},`

If Tacview has no fresh contact for the caller (or carrier position unknown), `{RADAR}` is empty.

**Responses (`MarshalMarkingMom`):**
1. `{CALLSIGN}, Marshal,{RADAR} mother's weather {WX}, expect {CASE} recovery, BRC {BRC}, altimeter {ALTIMETER}, Marshal angels {ANGELS}, report see me at ten.`
2. `{CALLSIGN}, Marshal,{RADAR} {CASE} recovery, mother {WX}, BRC {BRC}, altimeter {ALTIMETER}, stack angels {ANGELS}, report see me at ten.`
3. `{CALLSIGN}, Marshal,{RADAR} {CASE}, {WX}, BRC {BRC}, altimeter {ALTIMETER}, your angels are {ANGELS}, report see me at ten.`
4. `{CALLSIGN}, Marshal,{RADAR} mother {WX}, {CASE} recovery, BRC {BRC}, altimeter {ALTIMETER}, marshal angels {ANGELS}, report see me at ten.`

When BRC unknown, the `, BRC {BRC}` clause is omitted.

---

## 1a. Radio check

**Triggers:** `radio check` · `comm check` · `comms check` · `comp check` · `comcheck` · `how copy`

Same composer method as the towers (`RadioCheck`) — composer is constructed with `marshalCallsign="Marshal"` so the responses come out Marshal-flavored automatically.

**Responses (`RadioCheck`):**
1. `{CALLSIGN}, Marshal, loud and clear.`
2. `{CALLSIGN}, Marshal, five by five, go ahead.`
3. `{CALLSIGN}, Marshal, reading you loud and clear.`

---

## 1b. DME position report

**Triggers:** `N DME` · `N mile` · `N miles` · `N mile DME` (where `N` is a spelled or digit number — e.g. `7 DME`, `seven DME`, `7 mile`, `seven mile DME`)

Pilot reports current distance from mother on inbound. Typically called between `marking moms` and `see you at ten` (e.g. at 20 / 15 / 7 DME). Pure ack — no new clearance, just confirmation Marshal sees the call. If Tacview has the caller, the response includes a brief radar confirm (`radar contact` / `paint you`).

The reported distance `{DIST}` is parsed from the pilot's transmission and echoed back, spelled as digits (`7`, `15`, etc.).

**Responses (`MarshalAckDME`) — no radar:**
1. `{CALLSIGN}, Marshal, roger, {DIST} DME, continue.`
2. `{CALLSIGN}, Marshal, copy {DIST} DME.`
3. `{CALLSIGN}, Marshal, {DIST} DME, continue inbound.`

**Responses (`MarshalAckDME`) — with radar:**
1. `{CALLSIGN}, Marshal, radar contact, {DIST} DME, continue.`
2. `{CALLSIGN}, Marshal, paint you at {DIST} DME, continue inbound.`
3. `{CALLSIGN}, Marshal, contact {DIST} DME, continue.`

Pilot must still lead the transmission with the address word ("Marshal, …") — the self-echo guard in `handleMarshalCall` drops anything that doesn't, so `Raider 39, 7 DME, switching channel 4` alone gets filtered. With the prefix (`Marshal, Raider 39, 7 DME`) the trigger fires.

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

## 8. Internal stack collapse (no radio call)

Not pilot-triggered, **not transmitted**. Per the Case 1 Recovery comms (07.png) Marshal does not call step-downs on the radio — it just keeps an accurate internal model so the next inbound's altitude assignment is correct.

When an aircraft commences and `MarshalStack.Remove` fires, the handler calls `MarshalStack.CollapseStack(marshalMinAngels)`. Remaining aircraft are sorted by their current angels and reassigned to consecutive altitudes starting at `marshalMinAngels`, preserving relative order. The reassignment is logged at info level (`Marshal: stack step-down (internal, no TX)`) but no audio is generated.

**Example:** F1 at angels 2, F2 at angels 3, F3 at angels 4. F1 commences →
- copy commencing transmitted to F1.
- F2 reassigned 3 → 2 in state (no radio call).
- F3 reassigned 4 → 3 in state (no radio call).
- Next aircraft to call `marking moms` gets angels 4 from `AssignMarshalAngels` (slots 2 and 3 are now reserved).

---

## Notes

- Marshal handles 6 pilot-triggered intents (Marking mom, See you at 10, State, Established, Commencing, 3NM Initial). The internal stack collapse on a peer commencing (Section 8) updates state silently — no transmission. `MarshalContact` is the one composer method intentionally left unwired; it belongs to the LSO/Paddles role on a separate SRS button (Section 7).

### Stack altitude assignment (Tacview-aware)

On `marking moms`, the marshal handler calls `atcCtrl.AssignMarshalAngels(min, max, reserved)` which returns the **lowest unoccupied angel in `[min, max]`**. A slot is "occupied" if either:
1. It's reserved by another aircraft already in the stack (`MarshalStack.ReservedAngels` excluding self), or
2. Tacview shows any contact within 50nm of the carrier at that rounded altitude (1000-ft buckets).

Range is configured by `marshalMinAngels` / `marshalMaxAngels` in `cmd/atc/marshal.go` — currently **2k–9k**. If every slot is taken the function falls back to `marshalMaxAngels` (9). Tweak the constants there to widen or shift the band.

When an aircraft commences, its slot frees and the rest of the stack **collapses down internally** (see Section 8): aircraft are sorted by their current angels and reassigned to `marshalMinAngels`, `marshalMinAngels+1`, … in order. The reassignment is state-only — no radio call goes out. The next inbound to call `marking moms` lands on the lowest free slot above the collapsed stack.

## Case 1 Recovery comms coverage (vSFG-7 07.png)

| Stage | Pilot call | Marshal response | Wired? |
|---|---|---|---|
| Any | `radio check` / `comm check` / `comms check` / `how copy` | loud and clear / five by five | ✅ §1a |
| Before 50nm | `marking moms, [DIST], angels [XX], state [XX]` | radar contact (if Tacview), mother's weather, expect Case 1, BRC, altimeter, stack angels, report see me at 10 | ✅ §1 |
| Inbound | `[DIST] DME` / `[DIST] mile(s)` (with `Marshal,` prefix) | roger DME, continue (radar-flavored if Tacview has caller) | ✅ §1b |
| 10nm | `see you at 10` | radar contact, [DIST] miles, say state | ✅ §2 |
| 10nm | `state [XX]` | copy state | ✅ §3 |
| Stack | `established angels [XX], position [X]` | signal Charlie / hold | ✅ §4 |
| Commencing | `commencing, state [XX]` | copy commencing (state collapses silently — see §8) | ✅ §5, §8 |
| 3NM Initial | `initial` | push button [XX], check in | ✅ §6 |
| Push freq | `pushing button [XX]` / `checking in` | LSO `contact` (separate role/freq) | ⏳ LSO not implemented |
