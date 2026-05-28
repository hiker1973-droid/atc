# Deckboss Responses (128.6 MHz — DCS carrier UHF, OMDM carrier deck operations)

Edit response text and send back. Ports to `pkg/composer/composer.go` (Deckboss section) and `cmd/atc/deckboss.go` (intent handler). `v1.0.2` for text edits, `v1.1.0` for new intents.

## Format

Triggers from `cmd/atc/deckboss.go` `handleDeckbossCall`. Responses from `pkg/composer/composer.go` (Deckboss* methods). 3 variants random pick where available.

**Placeholders:**
- `{CALLSIGN}` — pilot callsign (e.g. `Raider 032`)
- `{CAT}` — catapult number, spelled (`one`, `two`, `three`, `four`)
- `{POS}` — position in conga line, spelled

Deckboss handles **on-deck** aircraft only — cat assignment, conga-line sequencing, launch detection. Inbound recovery is Marshal's job (306.3).

**Address rule:** §1 (check-in), §2 (under tension), §6 (radio check), §7 (BRC), and §8 (bolter pattern) require the pilot to lead with `Deckboss, ...`. Without the address prefix the call is treated as a self-echo of Deckboss's own TX and dropped. §3 (shooter / tension-only) and §4 (airborne / clear traffic) skip the guard — pilots typically don't address Deckboss on those quick calls and the response shapes can't false-fire §4.

---

## 1. Request Taxi / check-in for cat assignment

**Triggers:** `Request Taxi` · `Ready for Taxi` · `Green Jet`

This is the pilot's call when they're up and ready for cat. Deckboss assigns a free cat or queues them in the conga. `Green Jet` is the legacy DCS phrase (yellow-shirt → green-shirt handoff on the deck) and stays accepted for backward compatibility.

### 1a. Cat assigned (free cat available) — `DeckbossCatAssignment`
1. `{CALLSIGN}, Deckboss, cleared to cat {CAT}.`
2. `{CALLSIGN}, Deckboss, cat {CAT} is yours, taxi forward.`
3. `{CALLSIGN}, Deckboss, proceed to cat {CAT}.`

### 1b. All cats busy → join conga — `DeckbossCongaLine`
1. `{CALLSIGN}, Deckboss, all cats engaged, proceed to conga line, standby for assignment.`
2. `{CALLSIGN}, Deckboss, cats are full, join the conga line, we'll get you up.`
3. `{CALLSIGN}, Deckboss, no cats available, conga line, standby.`

### 1c. Already in conga (re-checking in) — `DeckbossStandby`
1. `{CALLSIGN}, Deckboss, you are number {POS} in the conga, standby.`
2. `{CALLSIGN}, Deckboss, hold position, number {POS} in line.`

(Note: `DeckbossStandby` only has 2 variants in source — could add a 3rd to match the convention.)

### 1d. Conga full — `DeckbossDeckFull`
1. `{CALLSIGN}, Deckboss, deck is full, hold clear of the bow.`
2. `{CALLSIGN}, Deckboss, no room on deck, hold your position.`
3. `{CALLSIGN}, Deckboss, deck is saturated, hold clear, standby.`

---

## 2. Ready on cat (under tension)

**Triggers:** (`ready` OR `tension`) AND `cat`  *(must appear together)*  ·  OR `shoot` (shortcut)

Pilot reports they're spotted and ready. Deckboss confirms tension. Accepts both `ready cat X` (standard carrier pre-tension call) and `tension cat X` (shooter-side phrasing). The `shoot` shortcut collapses the under-tension call into one word — pilot just says `Deckboss, Raider XX, shoot` and Deckboss fires §2 ack + §2a auto-shoot. Cat number sourced from §1 assignment if present, otherwise parsed from the transmission, otherwise generic ack.

**Responses (`DeckbossUnderTension`):**
1. `{CALLSIGN}, Deckboss, under tension, cat {CAT}, clear to launch.`
2. `{CALLSIGN}, Deckboss, tension cat {CAT}, hold.`
3. `{CALLSIGN}, Deckboss, cat {CAT} under tension, stand by.`

---

## 2a. Auto-shoot (5s after §2) + cat clear + next-conga pull

**Triggers:** automatic — fires 5 seconds after a successful §2 `DeckbossUnderTension` response. Not pilot-initiated.

Shooter's launch signal. Not callsigned (addresses the cat, deck-wide). **After the shoot call, Deckboss frees the cat slot and pulls the next pilot from the conga onto it** (or, if conga is empty, leaves the slot open for the next §1 `Request Taxi` caller). The cat-clear ack to next-up TXes **10 seconds after the shoot call** (T+15 from the under-tension ack) — gives the launching aircraft time to taxi off the cat / clear the deck so the slot is realistically open when next-up hears it. Next-up gets their slot assignment without waiting for the launching pilot's airborne call.

**Timeline:** T+0 under tension → T+5 shoot → T+15 cat clear to next-up.

**Responses (`DeckbossShoot`):**
1. `Cat {CAT}, fly.`
2. `Cat {CAT}, shoot, shoot, shoot.`
3. `Cat {CAT}, cleared to launch.`

**Cat-clear ack to next-up (`DeckbossCatClear`)** — fires immediately after the shoot call if a conga pilot is waiting. Prefixed with next-up callsign:
1. `{NEXT_CALLSIGN}, Deckboss, cat {CAT} is clear.`
2. `{NEXT_CALLSIGN}, Deckboss, cat {CAT} clear, deck is moving.`
3. `{NEXT_CALLSIGN}, Deckboss, cat {CAT} off the deck.`

Only fires when §2 had a real cat number (either from `GetCatByCallsign` or parsed from the pilot's transmission). The generic "copy under tension" fallback skips the auto-shoot **and** the cat-clear — §4 airborne or §5 Tacview fallback handles those edge cases.

---

## 3. Tension-only (pilot launching)

**Triggers:** `tension` (without a "cat" word)

Currently **silent** — no transmission, just a debug log. Pilot is going. If they say "tension cat X" instead, that matches §2 instead and gets the audible under-tension ack.

---

## 4. Airborne (pilot confirmation — slot already cleared)

**Triggers:** `airborne` OR `clear traffic` (from the just-launched pilot)

Pilot's optional airborne callout. In the standard flow the cat slot was already cleared by §2a immediately after shoot, so this is just an ack — no slot management needed. The ack TXes to the launching pilot only; next-up was already pulled at §2a.

**Ack to launching pilot** (always fires):
- `{CALLSIGN}, Deckboss, copy, good hunting.`

The ack deliberately omits the word "airborne" so the SRS echo of our own TX doesn't re-trigger §4 in a loop. (§4 skips the address-led guard since pilots don't address Deckboss on quick airborne calls — the self-trigger risk is mitigated by avoiding the trigger word in our response.)

**Fallback path:** If §2 fell back to the generic "copy under tension" ack (no real cat number identified), §2a was skipped and the cat slot is still held. In that case §4 takes over: frees the cat, pulls next conga aircraft, and TXes `DeckbossCatClear` to next-up. The cat-clear variants are the same as §2a.

---

## 5. Auto-detected launch (no shoot, no airborne call)

Background Tacview timer. If §2a fired (shoot path), the cat is already clear — this fallback does nothing. Same for §4 (airborne path). This only fires when both §2a and §4 were skipped: e.g. pilot said "under tension" without a cat number AND never called airborne. After 2 minutes on cat the Tacview monitor detects the aircraft off the deck and frees the slot. Same `DeckbossCatClear` response goes to next conga aircraft.

No pilot-side trigger here — it's a background timer.

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

**Triggers:** `remain in bolter pattern` · `bolter pattern` · `remain bolter` · `staying in bolter` · `in the bolter`

Post-launch intent from pilots doing touch-and-go trap practice. Pilot announces they're not departing the area — they'll stay in the carrier bolter pattern (touch the deck on the next pass, no trap, climb out and circle back). Standard bolter pattern is **600 ft AGL, 1 mile abeam** the carrier. Deckboss acks with the pattern parameters; no state change, no slot management.

Typically called right after §4 airborne / shoot, but accepted any time the pilot transmits the trigger.

**Responses (`DeckbossBolterPattern`):**
1. `{CALLSIGN}, Deckboss, copy bolter pattern, stay six hundred feet, one mile out.`
2. `{CALLSIGN}, Deckboss, roger bolter, maintain six hundred feet, one mile abeam.`
3. `{CALLSIGN}, Deckboss, in the bolter, six hundred feet, one mile out.`

Address-led guard applies — pilot must lead with `Deckboss, ...` to avoid self-echo (Deckboss's own response contains "bolter" / "six hundred" which could otherwise re-trigger).

---

## Notes

- **No Marshal/Recovery responses here.** Deckboss only does deck operations (launch). Recovery is Marshal's role on 306.3.
- The cat number range is 1–4 (super carrier standard).
- Conga line capacity is set in `pkg/state/state.go` — currently a fixed limit. If you want to expose it as a flag, that's v1.1.0.
- Suggestions for **new intents** worth adding (just say which):
  - `bingo` / `state` — fuel report (currently no Deckboss handler)
  - `wave off` — Deckboss-side equivalent (rare; usually LSO not Deckboss)
  - `crowded deck` / `foul deck` — operator-initiated deck-status announcement
