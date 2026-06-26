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
| (NOT IN USE YET) Deckboss (carrier) | 306.2 | Two-way |
| (NOT in USE YET)Marshal (carrier stack) | 306.3 | Two-way |
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

## Deckboss (306.2) — carrier launch director

Triggered by what you say, not by where you are on the boat.

| Intent | Say | What happens |
|---|---|---|
| Check in for launch | `"...green jet"` | Assigned a cat (1–4) or queued in the conga line |
| Tensioned, ready | `"...ready cat 3"`, `"ready on the cat"` | Deckboss calls "under tension" |
| Tension confirmed | `"...tension"` | Silent — you launch |
| Airborne | `"...airborne"`, `"clear of traffic"` | Frees your cat; next pilot in conga gets it |
| Radio check | `"...radio check"`, `"5x5"`, `"five by five"` | Standard reply |

The deck has 4 cats. If all are taken, you're queued. If the conga line is also full you'll be told the deck is full — try again later.

### Sample exchange

> Pilot: *"Deckboss, Raider 1-1, green jet."*
> Deckboss: *"Raider 1-1, Deckboss, taxi to cat 2, hold for tension."*
> Pilot: *"Raider 1-1, ready cat 2."*
> Deckboss: *"Raider 1-1, under tension, stand by."*
> Pilot: *"Raider 1-1, tension."*  (Deckboss silent)
> Pilot: *"Raider 1-1, airborne."*  (cat 2 freed)

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
