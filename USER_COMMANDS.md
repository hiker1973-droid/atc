# SkyeyeATC — Pilot Phraseology Reference

What to say on each channel to get a useful response from the AI ATC. Speak naturally — Whisper handles real ICAO phraseology fine. The bot looks for keywords, not exact strings, so most variations work. Examples below are tested phrases; anything similar will hit the same intent.

---

## Frequency map

| Channel | Freq MHz | Type |
|---|---|---|
| Al Minhad Tower (OMDM) | 250.1 | Two-way |
| Al Dhafra Tower (OMAM) | 251.1 | Two-way |
| Al Ain Tower (OMAL) | 250.7 | Two-way |
| Command (vSFG-7-Command) | 282.0 | Two-way |
| Deckboss (carrier deck) | 128.6 | Two-way |
| Marshal (carrier stack) | 306.3 | Two-way |
| ATIS Dhafra | 248.20 | Listen only |
| ATIS Minhad | 248.30 | Listen only |
| ATIS Khasab | 248.50 | Listen only |
| ATIS Liwa | 248.55 | Listen only |
| ATIS Al Ain | 248.85 | Listen only |

---

## How to address the Tower

The tower will only respond when it's addressed. Any of these forms work:

- `"Minhad Tower, Raider 1-1, ..."`
- `"Dhafra Traffic, Venom 2-3, ..."`
- `"Tower, Raider 1-1, ..."` (bare)
- `"Traffic, Raider 1-1, ..."` (CTAF style)

The system is fuzzy-matched on the field name (`minhad`, `dhafra`, `al ain`) so common Whisper mishears are forgiven. Squadron callsigns the bot auto-corrects: **Raider** (mishears: reader/radar/rater), **Venom** (mishears: vino/venue/demon).

---

## Tower — request types it understands

### Ground / Departure

| Intent | Say something like | Notes |
|---|---|---|
| Request taxi | `"...request taxi"`, `"ready to taxi"`, `"taxi to active"` | |
| Holding short | `"...holding short runway 27"`, `"at the hold"`, `"short of runway"` | Tower will sequence with inbounds within 15 nm |
| Ready for takeoff | `"...ready for takeoff"`, `"ready for departure"`, `"request takeoff"`, `"line up"` | |
| Distance check (post-departure) | `"...7 DME"`, `"5 miles"`, `"cleared airspace"` | Releases the runway from your slot |
| Clear of traffic / pattern | `"...clear of traffic"`, `"airborne"`, `"departing"` | CTAF-style departure call |

### Pattern

| Intent | Say | Notes |
|---|---|---|
| Inbound (initial) | `"...10 mile initial"`, `"5 miles inbound"`, `"inbound for the overhead"` | <=3 mile initial = overhead break |
| Overhead | `"...overhead"`, `"3 mile initial"`, `"over the field"` | |
| Break | `"...breaking left"`, `"break"` | Don't say "radio break" |
| Downwind | `"...downwind"` | |
| Base | `"...turning base"`, `"left base"`, `"right base"`, `"base final"` | |
| Straight-in | `"...straight in"`, `"ILS"`, `"RNAV approach"` | |
| Final / landing | `"...on final"`, `"request landing"`, `"final, gear down"` | |
| Going around | `"...going around"`, `"go around"`, `"missed approach"` | |
| Runway vacated | `"...runway vacated"`, `"clear of the active"`, `"off the runway"`, `"exiting runway"` | Releases your slot |

### Traffic / situational

| Intent | Say |
|---|---|
| Traffic in sight | `"...traffic in sight"`, `"visual"`, `"tally"` |
| No contact | `"...negative contact"`, `"no joy"` |
| Altitude check | `"...altitude check"`, `"request altitude"`, `"what altitude"` |
| Radio check | `"...radio check"`, `"comm check"`, `"how copy"` |
| Readback | `"...wilco"`, `"roger"`, `"copy"`, `"affirm"` |
| Emergency | `"mayday mayday mayday"`, `"declaring emergency"`, `"pan pan"` |

### Sample exchange

> Pilot: *"Minhad Tower, Raider 1-1, 10 mile initial, request the overhead."*
> Tower: *"Raider 1-1, Minhad Tower, cleared the overhead, runway 27, altimeter 29.88, report initial."*

---

## Command channel (282.0)

Mission-wide ops channel. Six intents, each with three randomised responses.

| Intent | Say | Bot replies with |
|---|---|---|
| Radio check | `"...radio check"`, `"comm check"`, `"how copy"` | "Loud and clear" / "Five by five" |
| Check-in | `"...checking in"`, `"check in"` | Cleared to proceed |
| On station | `"...on station"` | Cleared hot, good hunting |
| Off station | `"...off station"`, `"departing station"` | Return to assigned pattern |
| Fence in | `"...fence in"`, `"fence check"` | Cleared hot, master arm on |
| Fence out | `"...fence out"` | Squawk standard, switch to departure |

If your transmission doesn't hit one of these, Command stays silent (logged as "Command intent miss"). The fence-out match is checked **before** fence-in, so "fence out" never gets misclassified.

### Sample exchange

> Pilot: *"Command, Raider 1-1, fence in."*
> Command: *"Raider 1-1, Command, copy fence in, you are cleared hot."*

---

## Deckboss (128.6) — carrier launch director

Triggered by what you say, not by where you are on the boat. Lead with
`Deckboss, ...` on every call except the airborne one — without the address
most intents are treated as a radio echo and dropped.

| Intent | Say | What happens |
|---|---|---|
| Check in for launch | `"...request taxi"`, `"green jet"` | Assigned a cat (1–4) or queued in the conga line |
| Tensioned, ready | `"...under tension cat 3"`, `"ready cat 3"`, `"shoot"` | Under-tension ack — spin it up, the shooter fires you |
| Tension confirmed | `"...tension"` | Silent — you launch |
| Airborne | `"...airborne"`, `"clear of traffic"` | `"copy airborne"` — optional, your cat was already freed |
| Bolter pattern | `"...remain in bolter pattern"` | Ack + handoff to LSO |
| BRC | `"...say BRC"`, `"request BRC"` | Mother's current bow heading |
| Radio check | `"...radio check"`, `"five by five"` | Standard reply |

The deck has 4 cats. If all are taken, you're queued. If the conga line is also full you'll be told the deck is full — try again later.

**Deckboss does not call the shot.** The under-tension ack ends with "shooter's
discretion" — the shooter on the deck fires you, so don't wait for a "shoot"
call on the radio; there isn't one. Ten seconds after that ack your cat is
freed and the next pilot in the conga is called up, so your airborne call is a
courtesy, not something the queue is waiting on.

### Sample exchange

> Pilot: *"Deckboss, Raider 1-1, request taxi."*
> Deckboss: *"Raider 1-1, Deckboss, cleared to cat two."*
> Pilot: *"Deckboss, Raider 1-1, under tension cat two."*
> Deckboss: *"Raider 1-1, Deckboss, under tension cat two, spin it up, shooter's discretion."*
> (shooter fires you — no radio call)
> Deckboss: *"Raider 1-2, Deckboss, cat two is clear."*  (10s later, to the next pilot up)
> Pilot: *"Deckboss, Raider 1-1, airborne."*
> Deckboss: *"Raider 1-1, Deckboss, copy airborne."*

---

## Marshal (306.3) — recovery stack

Carrier recovery. Marshal manages the stack altitudes (angels) and signals Charlie when the deck is clear.

| Intent | Say | What happens |
|---|---|---|
| Checking in | `"...marking mom, state 5.2"` (with fuel) | Assigned a stack position + angels (5+pos), given altimeter and BRC |
| Inbound 10 nm | `"...see you at 10"` | Marshal acknowledges radar contact |
| Fuel state report | `"...state 4.8"` | Marshal copies state |
| Established at altitude | `"...established angels 6"` | If deck clear → Charlie. Otherwise hold. |
| Commencing approach | `"...commencing"` | Removed from stack |

Fuel state is parsed automatically — say "state 5.2" or similar in the same call.

### Sample exchange

> Pilot: *"Marshal, Raider 1-1, marking mom, state 5.2."*
> Marshal: *"Raider 1-1, Marshal, hold angels 6, altimeter 29.88, BRC 080, expected approach time on the 30. Stack has 2 aircraft."*
> Pilot: *"Raider 1-1, established angels 6."*
> Marshal: *"Raider 1-1, signal Charlie."*

---

## ATIS — listen only

Five stations, broadcast every **45 seconds**. Tune the frequency, listen for the information letter (Alpha, Bravo, …), copy weather and active runway, and reference it on first contact: `"...with information Bravo."`

The information letter only advances when the weather or active runway changes. If the bot says "information Charlie" three broadcasts in a row, that's expected.

## General tips

- **Say your callsign every transmission.** The intent parser uses it to track readbacks and clearances.
- **Whisper limits transmissions to ~20 seconds.** Long-winded calls get cut. Two short calls beat one long one.
- **If the bot didn't respond, check the logs.** `logs/atc-<icao>.log` will show either `Tower heard` (transcribed but no intent matched) or no entry at all (didn't transcribe / not addressed). Most non-responses are because the field name wasn't said clearly enough.
- **CTAF style works.** "[Field] traffic, [callsign], [intent]" is recognised the same as direct tower address.
- **Emergencies bypass normal sequencing.** Any of "mayday", "pan pan", "declaring emergency" puts you at the top of the queue.
