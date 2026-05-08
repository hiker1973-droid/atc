# Deckboss Responses (306.2 MHz, OMDM carrier deck operations)

Edit response text and send back. Ports to `pkg/composer/composer.go` (Deckboss section) and `cmd/atc/deckboss.go` (intent handler). `v1.0.2` for text edits, `v1.1.0` for new intents.

## Format

Triggers from `cmd/atc/deckboss.go` `handleDeckbossCall`. Responses from `pkg/composer/composer.go` (Deckboss* methods). 3 variants random pick where available.

**Placeholders:**
- `{CALLSIGN}` — pilot callsign (e.g. `Raider 032`)
- `{CAT}` — catapult number, spelled (`one`, `two`, `three`, `four`)
- `{POS}` — position in conga line, spelled

Deckboss handles **on-deck** aircraft only — cat assignment, conga-line sequencing, launch detection. Inbound recovery is Marshal's job (306.3).

---

## 1. Request Taxi (check-in for cat assignment)

**Triggers:** `Request Taxi`

This is the pilot's call when they're up and ready for cat. Deckboss assigns a free cat or queues them in the conga.

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

**Triggers:** `tension` AND `cat`  *(both must appear)*

Pilot reports they're spotted and ready. Deckboss confirms tension.

**Responses (`DeckbossUnderTension`):**
1. `{CALLSIGN}, Deckboss, under tension, cat {CAT}. Clea to launch`
2. `{CALLSIGN}, Deckboss, tension cat {CAT}, hold.`
3. `{CALLSIGN}, Deckboss, cat {CAT} under tension, stand by.`

---

## 3. Tension confirmed (pilot launching)

**Triggers:** `shooter`

Currently **silent** — no transmission, just a debug log. Pilot is going. Could add an audible response if desired (would be a v1.0.2 patch).

Suggested if you want one:
2. `Current BRC  XXX (BRC of Carrier)

(Just suggestions — leave commented out if you don't want them.)

---

(Note: response is prefixed with the next-up callsign at runtime, so player hears: `Raider 045, Cat one is clear.`)

The pilot who just launched gets **no acknowledgement** — they're already gone. If you want a "good launch" call to them too, that's a v1.0.2 addition.

---

## 5. Auto-detected launch (no airborne call)

If a pilot launches without saying "airborne," Deckboss's Tacview monitor detects them off the deck after 2 minutes on cat and frees it automatically. Same `DeckbossCatClear` response goes to next conga aircraft.

No pilot-side trigger here — it's a background timer.

---

## 6. Radio check

**Triggers:** `radio check` · `comm check` · `how copy` · `radio` · `5x5` · `five by five` · `five by`

**Responses** (defined inline in deckboss.go, NOT in composer):
1. `{CALLSIGN}, Deckboss, loud and clear.`
2. `{CALLSIGN}, Deckboss, five by five.`
3. `{CALLSIGN}, Deckboss, read you five by five.`

---

## Notes

- **No Marshal/Recovery responses here.** Deckboss only does deck operations (launch). Recovery is Marshal's role on 306.3.
- The cat number range is 1–4 (super carrier standard).
- Conga line capacity is set in `pkg/state/state.go` — currently a fixed limit. If you want to expose it as a flag, that's v1.1.0.
- Suggestions for **new intents** worth adding (just say which):
  - `bingo` / `state` — fuel report (currently no Deckboss handler)
  - `wave off` — Deckboss-side equivalent (rare; usually LSO not Deckboss)
  - `crowded deck` / `foul deck` — operator-initiated deck-status announcement
