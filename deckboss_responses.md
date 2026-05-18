# Deckboss Responses (128.6 MHz — DCS carrier UHF, OMDM carrier deck operations)

Edit response text and send back. Ports to `pkg/composer/composer.go` (Deckboss section) and `cmd/atc/deckboss.go` (intent handler). `v1.0.2` for text edits, `v1.1.0` for new intents.

## Format

Triggers from `cmd/atc/deckboss.go` `handleDeckbossCall`. Responses from `pkg/composer/composer.go` (Deckboss* methods). 3 variants random pick where available.

**Placeholders:**
- `{CALLSIGN}` — pilot callsign (e.g. `Raider 032`)
- `{CAT}` — catapult number, spelled (`one`, `two`, `three`, `four`)
- `{POS}` — position in conga line, spelled

Deckboss handles **on-deck** aircraft only — cat assignment, conga-line sequencing, launch detection. Inbound recovery is Marshal's job (306.3).

**Address rule:** §1 (check-in), §2 (under tension), and §6 (radio check) require the pilot to lead with `Deckboss, ...`. Without the address prefix the call is treated as a self-echo of Deckboss's own TX and dropped. §3 (shooter / tension-only) and §4 (airborne / clear traffic) skip the guard — pilots typically don't address Deckboss on those quick calls and the response shapes can't false-fire §4.

---

## 1. Request Taxi / check-in for cat assignment

**Triggers:** `Request Taxi` · `Ready for Taxi` · `Green Jet`

This is the pilot's call when they're up and ready for cat. Deckboss assigns a free cat or queues them in the conga. `Green Jet` is the legacy DCS phrase (yellow-shirt → green-shirt handoff on the deck) and stays accepted for backward compatibility.

### 1a. Cat assigned (free cat available) — `DeckbossCatAssignment`
1. `{CALLSIGN}, Deckboss, cat {CAT}, clear to taxi cat {CAT}.`
2. `{CALLSIGN}, Deckboss, cat {CAT} is yours, taxi forward.`
3. `{CALLSIGN}, Deckboss, proceed to cat {CAT}, cleared to spot.`

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

**Triggers:** (`ready` OR `tension`) AND `cat`  *(must appear together)*

Pilot reports they're spotted and ready. Deckboss confirms tension. Accepts both `ready cat X` (standard carrier pre-tension call) and `tension cat X` (shooter-side phrasing).

**Responses (`DeckbossUnderTension`):**
1. `{CALLSIGN}, Deckboss, under tension, cat {CAT}, clear to launch.`
2. `{CALLSIGN}, Deckboss, tension cat {CAT}, hold.`
3. `{CALLSIGN}, Deckboss, cat {CAT} under tension, stand by.`

---

## 3. Tension-only (pilot launching)

**Triggers:** `tension` (without a "cat" word)

Currently **silent** — no transmission, just a debug log. Pilot is going. If they say "tension cat X" instead, that matches §2 instead and gets the audible under-tension ack.

---

## 4. Cat clear (post-launch handoff to next in conga)

**Triggers:** `airborne` OR `clear traffic` (from the just-launched pilot)

Pilot calls airborne off the deck; Deckboss frees their cat and pulls the next conga aircraft onto it. Response is prefixed with the **next-up** callsign so the player hears `Raider 045, Cat one is clear.` The pilot who just launched gets no ack — they're already gone.

**Responses (`DeckbossCatClear`):**
1. `Cat {CAT} is clear.`
2. `Cat {CAT} clear, deck is moving.`
3. `Cat {CAT} off the deck.`

If no pilot calls "airborne", the Tacview monitor auto-frees the cat 2 min after assignment when the aircraft is detected airborne (see §5).

---

## 5. Auto-detected launch (no airborne call)

If a pilot launches without saying "airborne," Deckboss's Tacview monitor detects them off the deck after 2 minutes on cat and frees it automatically. Same `DeckbossCatClear` response goes to next conga aircraft.

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

## Notes

- **No Marshal/Recovery responses here.** Deckboss only does deck operations (launch). Recovery is Marshal's role on 306.3.
- The cat number range is 1–4 (super carrier standard).
- Conga line capacity is set in `pkg/state/state.go` — currently a fixed limit. If you want to expose it as a flag, that's v1.1.0.
- Suggestions for **new intents** worth adding (just say which):
  - `bingo` / `state` — fuel report (currently no Deckboss handler)
  - `wave off` — Deckboss-side equivalent (rare; usually LSO not Deckboss)
  - `crowded deck` / `foul deck` — operator-initiated deck-status announcement
