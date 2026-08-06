# Inter-Role Handoffs

How each controller passes a pilot to the next one. Edit response text and send
back — same convention as the other `*_responses.md` files (`v1.0.2` for text
edits, `v1.1.0` for new intents).

## The phraseology

FAA JO 7110.65 2-1-17 (Radio Communications Transfer):

> **CONTACT** (facility name or location name and terminal function), (frequency).

The facility is named **before** the frequency, so the pilot hears the number
last — immediately before they reach for the radio. Every handoff below follows
that order. The trailing courtesy ("Good day", "Good hunting") and the spoken
preset are the DCS/SRS adaptation: pilots tune presets, not raw frequencies.

Generic composer: `ATCComposer.Handoff(callsign, toName, freqMHz, preset, courtesy)`.
Both `freqMHz` and `preset` are optional — a destination with neither (Paddles)
still reads cleanly. Pilot-side courtesy acks use `ATCComposer.HandoffAck`.

## Who hands off to whom

| From | To | Trigger | Wired |
|---|---|---|---|
| Tower | Command | Departure release (`7 DME` / `airborne` / `cleared airspace`) | ✅ `departure_responses.md` §9 |
| Tower | Command | Pilot-initiated `pushing command` | ✅ `departure_responses.md` §10 |
| Command | Tower | Tacview: pilot closes through 30 NM of a land field | ✅ §1 below |
| Command | Marshal | Tacview: pilot closes through 30 NM and the **carrier** is nearer than any field | ✅ §1 below |
| Command | — | Pilot-initiated `RTB` / `request recovery` | ✅ §2 below |
| Deckboss | Command | Pilot's `airborne` / `clear traffic` call after the shot | ❌ removed 2026-08-06 — see §3 |
| Deckboss | Command | Pilot-initiated `pushing command` | ✅ §4 below |
| Deckboss | LSO | `remain in bolter pattern` | ✅ `deckboss_responses.md` §8 |
| Marshal | Paddles | 3 NM `initial` call | ✅ `marshal_responses.md` §6 |
| Marshal | Paddles | Pilot-initiated `pushing paddles` | ✅ §5 below |

Not modelled: Marshal → Tower for a beach divert, and the real Case I
Marshal → carrier Tower handoff at 10 NM (see `marshal_responses.md` — Marshal
issues Charlie itself here because there is no carrier-Tower role yet).

## Configuration

A role process only knows its **own** frequency (`--command-freq`,
`--marshal-freq`, `--deckboss-freq` — see the `start_*.bat` files), so the
destinations it hands off **to** are configured separately. Defaults match the
vSFG-7 rig, so the handoffs work with no changes to the existing `.bat` files.

| Flag | Default | Used by |
|---|---|---|
| `--handoff-command-freq` | `282.0` | Marshal departure handoff (`0` disables). No longer read by Deckboss — see §3 |
| `--handoff-command-name` | `vSFG-7-Command` | Marshal departure handoff; also Deckboss's `pushing command` ack (§4) |
| `--handoff-command-preset` | `channel four` | Marshal departure handoff (empty omits). No longer read by Deckboss |
| `--handoff-marshal-freq` | `306.3` | Command carrier-recovery handoff (`0` disables) |
| `--handoff-marshal-name` | `Marshal` | Command carrier-recovery handoff |

---

## 1. Command → Tower / Marshal (Tacview-driven, no trigger phrase)

`cmd/atc/command_handoff.go`. Every 30 s the watcher checks each pilot who has
transmitted to Command. It computes the distance to the nearest land field
**and** to the carrier, takes whichever is closer, and TXes once when the pilot
crosses from outside 30 NM to inside — so it fires on a genuine recovery, not on
a sortie that starts inside the ring or an orbit at the edge.

Carrier detection mirrors `controller.findCarrierContact`: prefer a named CVN
(`CVN`, `Lincoln`, `Stennis`, `Roosevelt`, `Washington`, `Vinson`), fall back to
a generic `carrier` group label, since missions export the ship either way.

**To a land field:**

> `{CALLSIGN}, vSFG-7-Command, contact {FIELD} tower on {FREQ}, switching now approved, good landing.`

**To the boat:**

> `{CALLSIGN}, vSFG-7-Command, contact Marshal on {FREQ}, switching now approved, call marking mom.`

The carrier tail names the pilot's next call (`marking mom`) rather than wishing
them a landing — Marshal's flow starts with that check-in.

Each pilot is handed off at most once per Command session; the watcher forgets
pilots after 60 min of radio silence.

---

## 2. Command — RTB / recovery request

**Triggers:** `rtb` · `returning to base` · `request recovery` · `request handoff` · `request tower` · `request marshal` · `inbound home plate` · `bingo home plate`

Checked **before** `fence out`, so an "RTB, fence out" call reads as the recovery
request. Command can't name the destination here — `commandResponse` is a pure
text function with no position feed — so the ack states what the watcher in §1
will do rather than guessing a field.

1. `{CALLSIGN}, {CHANNEL}, copy RTB, expect recovery handoff at three zero miles, continue.`
2. `{CALLSIGN}, {CHANNEL}, roger returning to base, stand by for handoff inside three zero miles.`
3. `{CALLSIGN}, {CHANNEL}, copy RTB, remain this frequency, I'll pass you to recovery at three zero miles.`

---

## 3. Deckboss → Command (on the airborne call) — **REMOVED 2026-08-06**

Deckboss no longer hands departing aircraft to Command on the airborne call.
The `deckboss_responses.md` §4 ack is now a bare `{CALLSIGN}, Deckboss, copy
airborne.` and pilots switch to Command on their own. Cat/conga slot management
on that call is unchanged.

The removed variants, for reference if this is ever put back:

1. `{CALLSIGN}, Deckboss, contact {HANDOFF}, {FREQ}, {PRESET}. Good hunting.`
2. `{CALLSIGN}, Deckboss, switch to {HANDOFF}, {FREQ}, {PRESET}. Good hunting.`
3. `{CALLSIGN}, Deckboss, frequency change approved, contact {HANDOFF}, {FREQ}, {PRESET}. Good hunting.`

`--handoff-command-freq` / `--handoff-command-preset` now have no effect on
Deckboss; `--handoff-command-name` is still read by §4 below (the
pilot-initiated `pushing command` ack). Both flags still drive the Tower and
Marshal handoffs above.

---

## 4. Deckboss — pilot-initiated handoff ack

**Triggers:** `pushing command` · `switching command` · `push command` · `switch command` · `pushing strike` · `switching strike` · `pushing departure` · `switching departure`

Address-led (`Deckboss, ...`) — required here because Deckboss's own §3 TX
contains "switch to {HANDOFF}" and would otherwise re-trigger on the SRS echo.

1. `{CALLSIGN}, Deckboss, cleared handoff to {HANDOFF}, good day.`
2. `{CALLSIGN}, Deckboss, roger pushing {HANDOFF}, good day.`
3. `{CALLSIGN}, Deckboss, copy switch to {HANDOFF}, good day.`

---

## 5. Marshal — pilot-initiated paddles handoff ack

**Triggers:** `pushing paddles` · `switching paddles` · `push paddles` · `switch paddles` · `pushing lso` · `switching lso` · `pushing button` · `pushing channel`

The other half of the `marshal_responses.md` §6 handoff. Short ack only — the
destination was already issued, and re-reciting it burns radio time.

Deliberately ordered **after** the §1b DME case so
`Marshal, Raider 39, 7 DME, switching channel 4` still reads as a DME position
report, not a handoff ack.

1. `{CALLSIGN}, Marshal, cleared handoff to paddles, good day.`
2. `{CALLSIGN}, Marshal, roger pushing paddles, good day.`
3. `{CALLSIGN}, Marshal, copy switch to paddles, good day.`
