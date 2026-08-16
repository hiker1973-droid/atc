# Deckboss Responses (128.6 MHz — DCS carrier UHF, OMDM carrier deck operations)

Edit response text and send back. Ports to `pkg/composer/composer.go` (Deckboss section) and `cmd/atc/deckboss.go` (intent handler). `v1.0.2` for text edits, `v1.1.0` for new intents.

## Format

Triggers from `cmd/atc/deckboss.go` `handleDeckbossCall`. Responses from `pkg/composer/composer.go` (Deckboss* methods). 3 variants random pick where available.

**Placeholders:**
- `{CALLSIGN}` — pilot callsign (e.g. `Raider 032`)
- `{CAT}` — catapult number, spelled (`one`, `two`, `three`, `four`)
- `{POS}` — position in conga line, spelled

Deckboss handles **on-deck** aircraft only — cat assignment, conga-line sequencing, launch detection. Inbound recovery is Marshal's job (306.3).

**Address rule:** §1 (check-in), §2 (under tension), §6 (radio check), §7 (BRC), and §8 (bolter pattern) require the pilot to lead with `Deckboss, ...`. Without the address prefix the call is treated as a self-echo of Deckboss's own TX and dropped. §3 (shooter / tension-only) and §4 (airborne / clear traffic) skip the guard — pilots typically don't address Deckboss on those quick calls. §4 instead uses a mid-string check (see §4) since its ack now repeats the trigger word.

---

## 1. Request Taxi / check-in for cat assignment

**Triggers:** `Request Taxi` · `Ready for Taxi` · `Green Jet`

This is the pilot's call when they're up and ready for cat. Deckboss assigns a free cat or queues them in the conga. `Green Jet` is the legacy DCS phrase (yellow-shirt → green-shirt handoff on the deck) and stays accepted for backward compatibility.

**Which cat gets assigned.** By default Deckboss hands out the lowest free cat (1 → 2 → 3 → 4), blind to where the aircraft actually is. With `--deck-position-check` on, it instead reads the caller's position from Tacview, projects it onto the carrier's BRC axis, and prefers the pair of cats nearest that spot — **bow cats 1/2** for aircraft forward of the carrier point, **waist cats 3/4** for aircraft aft. If the preferred pair is full it falls through to the other pair rather than pushing the pilot into the conga.

The check **fails open**: no carrier on Tacview, no contact for the callsign, a stale (>30s) contact, or an aircraft that reads airborne or off the boat all fall back to plain first-free assignment and log a `warn`. A pilot never loses a cat to a telemetry gap. On-deck means within `DeckRadiusNm` (0.20 nm) of the carrier contact and below `DeckAltFt` (200 ft MSL) — both in `pkg/controller/controller.go`.

The Tacview path is also idempotent per callsign: a pilot who re-checks in gets their existing cat back instead of a second slot. (The default first-free path still has this gap — see Notes.)

### 1a. Cat assigned (free cat available) — `DeckbossCatAssignment`
1. `{CALLSIGN}, Deckboss, cleared to cat {CAT}.`
2. `{CALLSIGN}, Deckboss, cat {CAT} is yours, taxi forward.`
3. `{CALLSIGN}, Deckboss, proceed to cat {CAT}.`

### 1b. All cats busy → join conga — `DeckbossCongaLine`
1. `{CALLSIGN}, Deckboss, all cats engaged, proceed to conga line, standby for assignment.`
2. `{CALLSIGN}, Deckboss, cats are full, join the conga line, we'll get you up.`
3. `{CALLSIGN}, Deckboss, no cats available, conga line, standby.`

### 1c. Already in conga (re-checking in) — `DeckbossStandby`
*(Live since 2026-08-10. Previously unreachable — a re-check-in from the line got §1b again.)*
1. `{CALLSIGN}, Deckboss, you are number {POS} in the conga, standby.`
2. `{CALLSIGN}, Deckboss, hold position, number {POS} in line.`

(Note: `DeckbossStandby` only has 2 variants in source — could add a 3rd to match the convention.)

### 1d. Conga full — `DeckbossDeckFull`
*(Live since 2026-08-10 at `CongaCapacity` = 6. Previously unreachable — the line had no cap.)*
1. `{CALLSIGN}, Deckboss, deck is full, hold clear of the bow.`
2. `{CALLSIGN}, Deckboss, no room on deck, hold your position.`
3. `{CALLSIGN}, Deckboss, deck is saturated, hold clear, standby.`

---

## 2. Ready on cat (under tension)

**Triggers:** (`ready` OR `tension`) AND `cat`  *(must appear together)*  ·  OR `shoot` (shortcut)

Pilot reports they're spotted and ready. Deckboss confirms tension. Accepts both `ready cat X` (standard carrier pre-tension call) and `tension cat X` (shooter-side phrasing). The `shoot` shortcut collapses the under-tension call into one word — pilot just says `Deckboss, Raider XX, shoot` and Deckboss fires the §2 ack, then the §2a cat-clear timer. Cat number sourced from §1 assignment if present, otherwise parsed from the transmission, otherwise generic ack.

**Responses (`DeckbossUnderTension`):**
1. `{CALLSIGN}, Deckboss, under tension cat {CAT}, spin it up, shooter's discretion.`
2. `{CALLSIGN}, Deckboss, copy under tension, go gates, shooter's discretion.`

Deckboss does **not** call the shot (changed 2026-08-06). The launch belongs to the shooter, so the ack tells the pilot to run the engines up and go on the shooter's signal — no "clear to launch" from the radio. Variant 2 carries no cat number, which is fine: the cat was already named in the §1 assignment.

---

## 2a. Cat clear + next-conga pull (10s after §2)

**Triggers:** automatic — fires 10 seconds after a successful §2 `DeckbossUnderTension` response. Not pilot-initiated.

There is no longer a Deckboss shoot call (`DeckbossShoot` removed 2026-08-06) — this is now a silent timer that only does slot management. By T+10 the jet is off the cat, so Deckboss frees the slot and pulls the next pilot from the conga onto it (or, if the conga is empty, leaves the slot open for the next §1 `Request Taxi` caller). Next-up gets their slot assignment without waiting for the launching pilot's airborne call.

**Timeline:** T+0 under tension → T+10 cat clear to next-up.

**Cat-clear ack to next-up (`DeckbossCatClear`)** — fires at T+10 if a conga pilot is waiting. Prefixed with next-up callsign:
1. `{NEXT_CALLSIGN}, Deckboss, cat {CAT} is clear.`
2. `{NEXT_CALLSIGN}, Deckboss, cat {CAT} clear, deck is moving.`
3. `{NEXT_CALLSIGN}, Deckboss, cat {CAT} off the deck.`

Only fires when §2 had a real cat number (either from `GetCatByCallsign` or parsed from the pilot's transmission). The generic "copy under tension" fallback skips the cat-clear — §4 airborne or §5 Tacview fallback handles those edge cases.

---

## 3. Tension-only (pilot launching)

**Triggers:** `tension` (without a "cat" word)

Currently **silent** — no transmission, just a debug log. Pilot is going. If they say "tension cat X" instead, that matches §2 instead and gets the audible under-tension ack.

---

## 4. Airborne (pilot confirmation — slot already cleared)

**Triggers:** `airborne` OR `clear traffic` (from the just-launched pilot)

Pilot's optional airborne callout. In the standard flow the cat slot was already cleared by §2a on the T+10 timer, so this is just an ack — no slot management needed. The ack TXes to the launching pilot only; next-up was already pulled at §2a.

**Ack to launching pilot** (always fires) — 3 variants, all carrying the Marshal handoff:
- `{CALLSIGN}, Deckboss, copy airborne, push Marshal, three zero six point three.`
- `{CALLSIGN}, Deckboss, copy airborne, switch Marshal, {FREQ}.`
- `{CALLSIGN}, Deckboss, copy airborne, contact Marshal, {FREQ}.`

**Departure chain (restored 2026-08-16).** Deckboss handed departing aircraft to Command directly until 2026-08-06, then to nobody at all — the ack was a bare `copy airborne` and pilots switched on their own. The handoff is back, but aimed at **Marshal** rather than Command, so a departure now leaves the boat on a defined three-leg chain:

| Leg | Who | Call |
|---|---|---|
| 1 | Deckboss → pilot | `copy airborne, push Marshal, three zero six point three` |
| 2 | pilot → Marshal | `Union Marshal, {CALLSIGN}, clear seven miles` |
| 3 | Marshal → pilot | `clear of Union control zone, push {COMMAND}, {FREQ}, for tasking` |

Name and frequency come from `--handoff-marshal-name` / `--handoff-marshal-freq` (default `Marshal` / `306.3`). Setting `--handoff-marshal-freq=0` disables the handoff and restores the exact pre-2026-08-16 bare ack, which is pinned by a test. Leg 3 is documented in `marshal_responses.md`.

**Self-echo guard.** The ack now repeats the trigger word "airborne", so the old "omit the trigger word" protection is gone. In its place §4 drops any transmission that is *not* address-led but *does* contain a Deckboss token somewhere in the middle — which is exactly the shape of our own TX echoing back (`Raider 032, Deckboss, copy airborne.`) and never the shape of a pilot call. Both pilot phrasings still land: `Deckboss, Raider 032, airborne` (address-led) and a bare `Raider 032 airborne` (no Deckboss token).

**Fallback path:** If §2 fell back to the generic "copy under tension" ack (no real cat number identified), §2a was skipped and the cat slot is still held. In that case §4 takes over: frees the cat, pulls next conga aircraft, and TXes `DeckbossCatClear` to next-up. The cat-clear variants are the same as §2a.

---

## 5. Auto-detected launch + stale-slot reclaim

Background Tacview monitor, 15s tick. No pilot-side trigger. Two independent reclaim paths, both gated on a slot having been held at least **2 minutes** (a pilot who just got the cat is still taxiing to it):

**5a. Launch detected.** If §2a fired (T+10 timer) the cat is already clear and this does nothing; same for §4. It only fires when both were skipped — e.g. the pilot said "under tension" without a cat number AND never called airborne. `IsAircraftAirborne` sees the climb-out and the slot is freed, `DeckbossCatClear` to next-up.

**5b. Stale slot reclaim (added 2026-08-10).** A pilot who disconnects, respawns, or dies on the deck never produces the climb-out 5a looks for, so their slot used to be held for the life of the process — a second route to "all cats engaged" with an empty deck. Now: slot held **> 5 minutes** with **no Tacview contact for the callsign in the last 60s** → reclaim, `warn` log, pull next-up. The same rule evicts pilots who left while queued in the conga (they otherwise keep their place and get handed a cat they'll never taxi to).

Both stale checks are gated on `IsTacviewActive()`. With the feed down every callsign reads as absent, which would otherwise clear the whole deck at once. No feed = no reclaim, slots stay held.

Tuning constants are at the top of the monitor goroutine in `cmd/atc/deckboss.go`.

---

## 6. Radio check

**Triggers:** `radio check` · `comm check` · `comms check` · `how copy` · `five by five` · `five by`

(Dropped the bare `radio` and `5x5` triggers — `radio` was too loose and false-fired on any TX containing "radio"; `5x5` was a STT-unreliable variant of `five by five`.)

**Responses** (defined inline in deckboss.go, NOT in composer):
1. `{CALLSIGN}, Deckboss, loud and clear.`
2. `{CALLSIGN}, Deckboss, five by five.`
3. `{CALLSIGN}, Deckboss, read you five by five.`

---

## 7. BRC request

**Triggers:** `say brc` · `request brc` · `brc check` · `check brc` · `what's brc` · `what is brc` · `say bearing` · `current brc` · `current bearing`

Pilot asks for mother's bow heading. Deckboss reads the live carrier BRC from Tacview. Reuses `MarshalSayBRC` — composer is constructed with `towerCallsign="Deckboss"` so the response comes out Deckboss-flavored automatically.

**Responses (`MarshalSayBRC`, Deckboss flavor):**
1. `{CALLSIGN}, Deckboss, mother's BRC is {BRC}.`
2. (additional variants per composer)

If BRC is unknown (no carrier on Tacview), the composer returns "BRC unknown" phrasing.

Address-led guard applies — pilot must lead with `Deckboss, ...`.

---

## 8. Remain in bolter pattern (trap practice)

**Triggers:** `remain in bolter pattern` · `remain in bolter` · `bolter pattern` · `remain bolter` · `staying in bolter` · `in the bolter`

Post-launch intent from pilots doing touch-and-go trap practice. Pilot announces they're not departing the area — they'll stay in the carrier bolter pattern (touch the deck on the next pass, no trap, climb out and circle back). Deckboss owns the deck, not the recovery pattern, so it acks and **hands the pilot off to the LSO**, who works and grades the passes; no deck state change, no slot management.

Typically called right after §4 airborne / shoot, but accepted any time the pilot transmits the trigger.

**Responses (`DeckbossBolterPattern`):**
1. `{CALLSIGN}, Deckboss, copy remain in bolter, contact LSO.`
2. `{CALLSIGN}, Deckboss, roger bolter pattern, hand off to LSO.`
3. `{CALLSIGN}, Deckboss, copy bolter, switch to LSO.`

Address-led guard applies — pilot must lead with `Deckboss, ...` to avoid self-echo (Deckboss's own response contains "bolter" which could otherwise re-trigger).

---

## Notes

- **No Marshal/Recovery responses here.** Deckboss only does deck operations (launch). Recovery is Marshal's role on 306.3.
- The cat number range is 1–4 (super carrier standard). Bow = 1/2, waist = 3/4.
- **Fixed 2026-08-10 — "cats full with nothing on deck".** `state.AssignCat` (the default path) did not check whether the callsign already held a cat, so every repeated §1 check-in took a *second* slot and orphaned the first — and `FreeCat` only ever released one, which made the leak permanent. Four repeats from one aircraft filled the deck. `AssignCat` is now idempotent per callsign like `AssignCatPreferred`, and `FreeCat` releases every slot a callsign holds.
- Conga line capacity is `state.CongaCapacity` in `pkg/state/state.go` — currently **6**. Before 2026-08-10 no limit was enforced at all: the handler tested `EnqueueConga` for result codes it never returned, so §1c and §1d were unreachable and an over-full line just kept growing. If you want the cap exposed as a flag, that's v1.1.0.
- Suggestions for **new intents** worth adding (just say which):
  - `bingo` / `state` — fuel report (currently no Deckboss handler)
  - `wave off` — Deckboss-side equivalent (rare; usually LSO not Deckboss)
  - `crowded deck` / `foul deck` — operator-initiated deck-status announcement
