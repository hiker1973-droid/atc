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
- `"Akrotiri, Raider 1-1, ..."` (field name only — "Tower" is optional when the field name opens the call)

The system is fuzzy-matched on the field name (`minhad`, `dhafra`, `al ain`) so common Whisper mishears are forgiven. Squadron callsigns the bot auto-corrects: **Raider** (mishears: reader/radar/rater), **Venom** (mishears: vino/venue/demon).

---

## Tower — request types it understands

### Ground / Departure

| Intent | Say something like | Notes |
|---|---|---|
| Request taxi | `"...request taxi"`, `"ready to taxi"`, `"taxi to active"` | |
| Holding short | `"...holding short runway 27"`, `"at the hold"`, `"short of runway"` | Tower will sequence with inbounds within 15 nm. Held for traffic or departure spacing, you are cleared for takeoff once it passes — no second call needed |
| Ready for takeoff | `"...ready for takeoff"`, `"ready for departure"`, `"request takeoff"`, `"line up"` | "request **for** takeoff/taxi" works too |
| Rolling (CTAF) | `"...rolling runway 09"`, `"on the roll"` | Departure release; spaces the next departure 60 s behind you |
| Distance check (post-departure) | `"...7 DME"`, `"seven miles"`, `"cleared airspace"` | Releases the runway from your slot. Any call with initial / inbound / final / overhead / straight in / RTB in it is an arrival instead, so "15 miles inbound" is not a departure call |
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
| Final / landing | `"...on final"`, `"request landing"`, `"final, gear down"`, `"10 mile final"` | A distance in the final call is still a landing call |
| Touch and go / low approach / option | `"...on final, touch and go"`, `"request low approach"`, `"request the option"` | Replaces "cleared to land" with "cleared touch and go" etc. `"full stop"` = normal landing |
| Hung ordnance | `"...hung ordnance"`, `"hung store"` | Straight in, no overhead, hold at the end of the runway for dearm |
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

### Tower may call you first (optional — off unless the rig enables it)

| When | Tower says | Flag |
|---|---|---|
| You close inside 8 nm, below 5,000 ft above the field, without having talked to the tower in the last 10 min | `"...six miles northeast of the field, say intentions."` — once per 10 min, human pilots only | `--unknown-traffic-calls` |
| You were cleared to land and have sat on the field at taxi speed for 90 s without calling vacated | `"...report clear of the runway."` — once per landing | `--runway-vacate-chase` |

Both calls work with `say again`.

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
| Fuel state | `"...state 5.2"`, `"state 1 point 8"` | Copies state; below 2.0 you're told you're bingo. Also works tacked onto fence in / fence out |
| Bullseye | `"...say bullseye"`, `"request bullseye"` | Your position from your coalition's bullseye, magnetic bearing and range. Needs the Tacview feed |
| Tanker | `"...request tanker"`, `"where's the tanker"`, `"say texaco"` | Magnetic bearing, range and angels to the nearest tanker on your side. Needs the Tacview feed |
| Tasking | `"...request tasking"`, then `"...copy"` | Hands you to **Hitman 1-1 on 61.000 FM** (Caucasus training), who picks one of the five 9-line targets at random and clears you hot (see JTAC below). Command names no target. Ask Command again, or ask Hitman, after a run for another. Where Command runs without the JTAC (`--jtac-freq` unset), Command itself sends you to an IP and clears you hot on your readback |

If your transmission doesn't hit one of these, Command stays silent (logged as "Command intent miss"). The fence-out match is checked **before** fence-in, so "fence out" never gets misclassified.

### Sample exchange

> Pilot: *"Command, Raider 1-1, fence in."*
> Command: *"Raider 1-1, Command, copy fence in, you are cleared hot."*

> Pilot: *"Command, Raider 1-1, request tasking."*
> Command: *"Raider 1-1, Command, contact Hitman one one on six one point zero zero zero for tasking."*
> Pilot: *"Command, Raider 1-1, copy."*
> Command: *"Raider 1-1, Command, switch Hitman one one, good hunting."*

## JTAC — Hitman 1-1 (61.000 FM) — Caucasus training 9-lines

Hornet COMM2 CH12, Viper COMM2 CH8. Runs inside the Caucasus Command process
(`--jtac-freq 61.0`). Hitman picks the target, as a real JTAC does: one of the
five cards at random, not one another flight is working, not the one you just
did. You can check in with Hitman without going through Command. Lead every call
with `Hitman, ...`.

Hitman checks the run against your 9-line card with the Tacview picture. With no
picture of you (Tacview down, or your callsign not matched) the checks are skipped
and the exchange still ends in "cleared hot".

| Intent | Say | What happens |
|---|---|---|
| Check in | `"...checking in"`, `"ready for 9-line"`, `"request tasking"` | Hitman picks your target and gives only its number, name and IP, e.g. "target 3, Fuel Depot, IP Cobra"; the rest of the 9-line is on your card. If you are more than 5 nm from the IP, where the IP is |
| Readback | `"...target 3, Cobra"` (the target or the IP), or `"...copy"` | "Good readback, report IP inbound" |
| IP inbound | `"...IP inbound"` | "Continue" if you are within 5 nm of the IP. Further away: refused, with the IP's bearing and range. Nose more than 30° off the target: "check heading, target bears ..." |
| In | `"...in from the north"`, `"in heading 330"`, `"...in"` | **Cleared hot** when your magnetic heading is inside the card's final attack heading and you are 0.5–15 nm from the target. Otherwise **abort, abort, abort** with the reason; go round from the IP |
| (no call) | — | After cleared hot, Hitman watches the run. If you swing more than 10° outside the final attack heading while still pointed at the target, before a weapon comes off, it calls the abort. Pulling off (target well off the nose), a release, or reaching 1.5 nm ends the watch |
| Off | `"...off"`, `"off safe"` | Egress per line 9; report IP inbound for a re-attack |
| (no call) BDA | — | About 15 s after your weapons land, Hitman gives the BDA once per pass: where your impacts were against the aim point ("on the aim point" inside 20 m, else metres and direction), then the kills against what the mission placed, priority objects first ("Large tanks, 2 of 4 destroyed. 3 of 14 on target destroyed."), or "Target destroyed. Good work." |
| BDA | `"...request BDA"`, `"say BDA"`, `"battle damage"` | The same report on demand |
| Abort | `"...abort"` | Copies; re-attack from the IP |
| Radio check | `"...radio check"` | Loud and clear |

**BDA limits.** The bridges (targets 1 and 2) are map objects that Tacview does
not carry: Hitman says "unable to assess the bridge" and gives impacts, plus the
Enguri checkpoint vehicles and a call-out for any impact on the Rioni no-strike
road bridge. A target whose objects are not in the Tacview stream at all gets
impacts and "unable to assess damage". Kills last until the mission restarts.

**Voice.** `ash` with a ground-JTAC delivery (`--voice-style-jtac`) and the
`field` radio effect (`--jtac-radio-intensity`): narrower band and more hiss than
Command, as a manpack FM set on the ground.

> Pilot: *"Hitman 1-1, Raider 1-1, checking in."*
> Hitman: *"Raider 1-1, Hitman one one, target 1, Enguri Bridge, IP Anvil. Read back."*
> Pilot: *"Hitman, Raider 1-1, target 1, Anvil."* → *"... good readback. Report IP inbound."*
> Pilot: *"Hitman, Raider 1-1, IP inbound."* → *"... continue."*
> Pilot: *"Hitman, Raider 1-1, in from the southeast."* → *"... cleared hot."*
> Pilot: *"Hitman, Raider 1-1, off."* → *"... copy off. Egress back to Anvil, angels 15. Report IP inbound for re-attack."*

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
