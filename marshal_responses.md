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

The Case 3 variant (`MarshalMarkingMomCase3`) substitutes "from the Abe" (CVN-72 callname) for "from mother" in all three radar variants. Case 1/2 marking moms still uses the generic "from mother" wording.

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

**If deck is busy → ack and hold (now confirms `{POS}` so pilot can verify slot):**
1. `{CALLSIGN}, Marshal, entering stack at angels {ANGELS}, position {POS}, hold for Charlie.`
2. `{CALLSIGN}, Marshal, roger, in the stack angels {ANGELS} at position {POS}, stand by for Charlie.`
3. `{CALLSIGN}, Marshal, copy established, angels {ANGELS}, position {POS} in the stack.`

---

## 5. Commencing approach → call 3-mile initial for paddles

**Triggers:** `commencing`

Commencing is the ack that the pilot is leaving the stack and starting
the approach. Marshal acks, instructs the pilot to check in at the
3-mile initial, and flags that the §6 `initial` call is the actual
paddles handoff. The pilot is still on Marshal freq until they call
`initial` at 3nm — see §6.

**If pilot also reports state:**
1. `{CALLSIGN}, Marshal, copy commencing, state {STATE}, check in three mile initial, paddles handoff.`
2. `{CALLSIGN}, Marshal, commencing, state {STATE}, report three mile initial for paddles.`
3. `{CALLSIGN}, Marshal, roger commencing, state {STATE}, call three mile initial, paddles will have you.`

**Without state:**
1. `{CALLSIGN}, Marshal, copy commencing, check in three mile initial, paddles handoff.`
2. `{CALLSIGN}, Marshal, commencing, report three mile initial for paddles.`
3. `{CALLSIGN}, Marshal, roger commencing, call three mile initial, paddles will have you.`

---

## 6. 3NM Initial → LSO handoff (no TACAN)

**Triggers:** `initial`

The 3-mile initial call is the actual paddles handoff. After the §5
`commencing` ack, the pilot continues the approach and calls `initial`
at 3nm — Marshal hands off to paddles here. No TACAN button cited
(prior version had `push button 72`; dropped since pilots know to
switch and the explicit number was unnecessary friction).

**Responses (`MarshalPushButton`):**
1. `{CALLSIGN}, Marshal, contact paddles, good luck.`
2. `{CALLSIGN}, Marshal, switch to paddles, good luck.`
3. `{CALLSIGN}, Marshal, paddles has you, good luck.`

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

## Case 3 Recovery — phraseology overrides (vSFG-7 10vSFG7_Case3_Kneeboard.jpg)

Case 3 (night or IMC — ceiling <1000 ft, visibility <5 miles, or any night
recovery) replaces the wording of §1 marking moms and §4 established angels.
Other intents (radio check, DME, state, commencing, initial) still use the
Case 1 composer methods for now — those will be revisited in Phase 2.

**Trigger detection is unchanged.** The handler branches on
`atcCtrl.GetRecoveryCase()` at TX time; the recovery-case watcher already
broadcasts on Case 1↔3 transitions so an in-flight pilot will hear the
change announcement before their next call.

### §1 Case 3 Marking mom (`MarshalMarkingMomCase3`)

Issues a holding clearance instead of "Marshal at angels N, see me at ten":
- **assigned radial** = `(BRC + 180) mod 360`, spelled 3-digit (e.g. `two eight seven`)
- **assigned angels** = lowest unoccupied slot in `[marshalMinAngels, marshalMaxAngels]` (same logic as Case 1)
- **EAT** = `now + leadMin + (pos-1)*intervalMin`, minute-of-hour spelled 2-digit. Defaults: lead = 10 min, interval = 1 min. Tunable in `cmd/atc/marshal.go`.

State side-effects: `MarshalStack` now stores `AssignedRadial` and `EAT` per aircraft so the §4 established ack can recite the same values.

**Responses:**
1. `{CALLSIGN}, Marshal,{RADAR} mother's weather {WX}, Case Three recovery, BRC {BRC}, altimeter {ALTIMETER}, hold on the {RADIAL} at angels {ANGELS}, commence at {EAT}, report established.`
2. `{CALLSIGN}, Marshal,{RADAR} Case Three, mother {WX}, BRC {BRC}, altimeter {ALTIMETER}, holding radial {RADIAL}, angels {ANGELS}, commence at {EAT}, report established.`
3. `{CALLSIGN}, Marshal,{RADAR} Case Three recovery, {WX}, BRC {BRC}, altimeter {ALTIMETER}, your hold is the {RADIAL} at angels {ANGELS}, commence at {EAT}, report established.`

`{RADAR}` uses the Case 3 "from the Abe" variant (same three shapes, mother → Abe). BRC clause is omitted when carrier is off scope; radial falls back to spoken `zero zero zero` in that case. Case 3 uses "commence at {EAT}" phrasing rather than "expected approach time {EAT}" so §1 and §4 stay consistent ("commence at" appears in both).

### §4 Case 3 Established (`MarshalEstablishedAckCase3`)

No Charlie / hold-for-Charlie in Case 3 — the pilot commences on the EAT clock, regardless of deck status.

**Responses:**
1. `{CALLSIGN}, Marshal, roger established angels {ANGELS} on the {RADIAL}, commence at {EAT}.`
2. `{CALLSIGN}, Marshal, copy established radial {RADIAL} angels {ANGELS}, your push time is {EAT}.`
3. `{CALLSIGN}, Marshal, in the hold angels {ANGELS} on the {RADIAL}, expect commence at {EAT}.`

If the aircraft was originally checked in under Case 1/2 and the case flipped mid-flight (so `EAT` was never assigned), the handler computes one at established-time using the current stack position.

### §5 Case 3 Commencing (`MarshalCopyCommencingCase3`)

Pilot reports `commencing` at their EAT. Marshal removes them from the stack
(triggering the silent collapse for remaining aircraft) and directs them to
the CV-1 descent profile: platform 5000 ft on the **final bearing** (BRC −
`marshalFinalBearingOffset`, default 9°). No 3-mile-initial / paddles tail
— in Case 3 the next pilot call is `platform`, not `initial`.

**Final bearing** = `(BRC - 9 + 360) % 360`, spelled 3-digit. When carrier is
off scope (`brc < 0`) the final bearing falls back to `000`. Tune the offset
via the `marshalFinalBearingOffset` const in `cmd/atc/marshal.go`.

A "confirm BRC {BRC}" clause is appended after final bearing so the pilot reads back the current carrier heading along with the platform-descent setup. BRC clause is omitted when carrier is off scope (`brcDeg < 0`).

**With state:**
1. `{CALLSIGN}, Marshal, copy commencing, state {STATE}, descend to platform five thousand, final bearing {FB}, confirm BRC {BRC}, report platform.`
2. `{CALLSIGN}, Marshal, commencing, state {STATE}, platform five thousand, final bearing {FB}, confirm BRC {BRC}, report platform.`
3. `{CALLSIGN}, Marshal, roger commencing, state {STATE}, descend platform, final bearing {FB}, confirm BRC {BRC}, call platform.`

**Without state:**
1. `{CALLSIGN}, Marshal, copy commencing, descend to platform five thousand, final bearing {FB}, confirm BRC {BRC}, report platform.`
2. `{CALLSIGN}, Marshal, commencing, platform five thousand, final bearing {FB}, confirm BRC {BRC}, report platform.`
3. `{CALLSIGN}, Marshal, roger commencing, descend platform, final bearing {FB}, confirm BRC {BRC}, call platform.`

### §5a Platform (`MarshalAtPlatform`)

**Triggers:** `platform` · `passing five thousand` · `passing 5000` · `at platform`

Pilot reports passing 5000 ft on descent, typically inside 20 nm of mother.
Marshal acks and re-recites the final bearing — that's it. The pilot
continues to glideslope intercept (1200 ft at 12 DME per the kneeboard)
and hands off to LSO at ball call; Marshal's job effectively ends here in
Case 3. (Dirty-up and 12-mile-gate hints were dropped — pilots know the
profile from the kneeboard; brevity wins on the radio.)

The trigger is wired regardless of recovery case (rare false-fire in Case 1
is harmless — the pilot still gets a sensible ack). Phase is set to
`platform` on the stack entry, but by this point `commencing` has already
removed the aircraft so the SetPhase is a noop — kept for forward
compatibility if we re-track post-commence aircraft later.

**Responses (`MarshalAtPlatform`):**
1. `{CALLSIGN}, Marshal, copy platform, final bearing {FB}.`
2. `{CALLSIGN}, Marshal, roger platform, final bearing {FB}.`
3. `{CALLSIGN}, Marshal, platform, final bearing {FB}.`

### Not yet wired (Phase 3)
- "say needles" / ball call — LSO/CATCC territory, parked for the LSO role.
- Case 3 departure flow (airborne / 10-DME arc / departure radial / popeye / on top / kilo) — not wired; Marshal does not currently own departure routing.

---

## Case 1 Recovery comms coverage (vSFG-7 07.png)

| Stage | Pilot call | Marshal response | Wired? |
|---|---|---|---|
| Any | `radio check` / `comm check` / `comms check` / `how copy` | loud and clear / five by five | ✅ §1a |
| Before 50nm | `marking moms, [DIST], angels [XX], state [XX]` | radar contact (if Tacview), mother's weather, expect Case 1, BRC, altimeter, stack angels, report see me at 10 | ✅ §1 |
| Inbound | `[DIST] DME` / `[DIST] mile(s)` (with `Marshal,` prefix) | roger DME, continue (radar-flavored if Tacview has caller) | ✅ §1b |
| 10nm | `see you at 10` | radar contact, [DIST] miles, say state | ✅ §2 |
| 10nm | `state [XX]` | copy state | ✅ §3 |
| Stack | `established angels [XX], position [X]` | signal Charlie / hold | ✅ §4 |
| Commencing | `commencing, state [XX]` | copy commencing, check in three mile initial for paddles (state collapses silently — see §8) | ✅ §5, §8 |
| 3NM Initial | `initial` | contact paddles, good luck (paddles handoff; no TACAN cited) | ✅ §6 |
| Push freq | `pushing button [XX]` / `checking in` | LSO `contact` (separate role/freq) | ⏳ LSO not implemented |
