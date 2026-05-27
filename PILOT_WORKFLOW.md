# SkyeyeATC — Pilot Workflow

How vSFG-7 squadron pilots interact with the AI ATC system on Training 1. Covers ATIS, Tower (3 fields), Command, and Marshal.

---

## Active frequencies

| Role | Freq (MHz) | Purpose |
|---|---|---|
| ATIS Dhafra | 248.20 | Weather + active runway, OMAM |
| ATIS Minhad | 248.30 | Weather + active runway, OMDM |
| ATIS Khasab | 248.50 | Weather (advisory) |
| ATIS Liwa | 248.55 | Weather (advisory) |
| ATIS Al Ain | 248.85 | Weather + active runway, OMAL |
| Al Ain Tower | 250.70 | OMAL — ground + tower |
| Al Minhad Tower | 250.10 | OMDM — ground + tower |
| Al Dhafra Tower | 251.10 | OMAM — ground + tower |
| vSFG-7 Command | 282.00 | Squadron command net |
| Marshal | 306.30 | Carrier inbound stack |

All frequencies are **AM**, encryption **off**.

---

## Land departure (cold start → en-route)

1. **Get ATIS** — tune your field's ATIS, copy runway and altimeter. ATIS rebroadcasts every 45 seconds.

2. **Startup** — tune the field's Tower freq.
   - Pilot: *"Al Minhad Tower, Raider 1, request startup"*
   - ATC (Ground): *"Raider 1, Al Minhad Ground, startup approved, altimeter 29.92, advise ready to taxi"*
   - Triggers: `request startup` · `ready for startup` · `ready to start`

3. **Taxi** — same Tower freq.
   - Pilot: *"Al Minhad Tower, Raider 1, request taxi"*
   - ATC: *"…taxi to runway 27, altimeter 29.92, hold short, advise ready"*
   - Triggers: `request taxi` · `request ground` · `taxi to` · `ready to taxi`

4. **Hold short** — at the hold line.
   - Pilot: *"Al Minhad Tower, Raider 1, holding short runway 27"*
   - ATC: *"…hold short runway 27, number one, advise ready"* (or line-up-and-wait if traffic on final is far enough out)
   - Triggers: `holding short` · `hold short` · `short of runway` · `at the hold`

5. **Takeoff** — when ready.
   - Pilot: *"Al Minhad Tower, Raider 1, request takeoff"*
   - ATC: *"…wind 270 at 8, runway 27, cleared for takeoff"*
   - Triggers: `request takeoff` · `request departure` · `ready for departure` · `ready for takeoff` · `lineup`

6. **Departure release** — once airborne and clear of pattern.
   - Pilot: *"Al Minhad Tower, Raider 1, airborne"*
   - ATC: *"…proceed seven miles, climb angels 3, frequency change approved, good day"*
   - Triggers: `airborne` · `departing` · `clear traffic` · `clear of traffic` · `7 miles` · `cleared airspace`

7. **Switch to Command** — Tower hands off:
   - ATC: *"…contact vSFG-7-Command, two eight two point zero, channel four"*
   - Tune **282.0**.

---

## On Command (282.0)

Mission-level coordination — fence checks, on/off-station, RTB.

| Pilot says | ATC response |
|---|---|
| *"vSFG-7-Command, Raider 1, checking in"* | *"loud and clear, proceed mission"* |
| *"…fence in"* / *"fence check"* | *"copy fence in, you are cleared hot"* |
| *"…on station"* | *"affirmative, good hunting"* |
| *"…off station"* | *"copy, proceed to assigned pattern"* |
| *"…fence out"* | *"copy fence out, safe passage"* |
| *"…radio check"* / *"comm check"* | *"loud and clear"* / *"five by five"* |

---

## Land arrival (en-route → parking)

1. **Get ATIS** for the destination field, copy runway/altimeter.

2. **Switch to Tower** when inbound.

3. **Inbound call** — at 10–30 nm out:
   - Pilot: *"Al Minhad Tower, Raider 1, 10 mile initial"*
   - Also accepted: `inbound` · `30 mile initial` · any `XX mile initial`
   - ATC: gives runway, may report lead/traffic, instructs pattern entry.

4. **3-mile initial → break clearance.** No separate "in the break" call needed.
   - Pilot: *"Al Minhad Tower, Raider 1, 3 mile initial"*
   - ATC: *"…runway 27, approved for left break"* (direction picked from the field's ramp side)
   - Triggers: any `XX mile initial` where distance ≤ 3

5. **Pattern reports** — call each leg.
   - *"…downwind"* → traffic call / continue
   - *"…base"* / *"turning base"* → may clear to land
   - *"…final"* / *"on final"* → cleared to land

6. **After touchdown:**
   - Pilot: *"…runway vacated"*
   - ATC: *"welcome back, taxi to parking"*
   - Triggers: `runway vacated` · `vacated` · `clear of runway` · `clear active` · `off the runway`

---

## Carrier ops (Marshal)

Marshal handles the inbound stack to the boat — case recovery info, BRC, stack angels assignment, and final push to LSO. Six pilot calls, in order:

1. **Marking mom** — initial check-in. Tune Marshal **306.3**.
   - Pilot: *"Marshal, Raider 1, marking mom, 25 miles, angels 12, state 6.5"*
   - ATC: *"…Case One recovery, BRC 088, altimeter 29.92, Marshal angels 3, report see me at ten"*
   - **Case** is computed from weather ceiling (Case 1 ≥3000 ft, Case 2 ≥1000 ft, Case 3 below).
   - **BRC** is the carrier's current heading from Tacview (3-digit, e.g. `088`); the BRC clause is dropped if Tacview can't see the carrier.
   - **Stack angels** = lowest unoccupied slot between angels 2 and 9.
   - Triggers: `marking mom` · `marking moms`

2. **See me at ten** — visual on the boat at 10 nm.
   - Pilot: *"Marshal, Raider 1, see you at ten"*
   - ATC: *"…radar contact, ten miles, say state"*
   - Triggers: `see you at 10` · `see you at ten`

3. **State** — fuel report.
   - Pilot: *"Marshal, Raider 1, state 4.2"*
   - ATC: *"…copy state 4.2"* (or *"…expedite recovery"* / *"you are priority"* if state < 2.0)

4. **Established in stack.**
   - Pilot: *"Marshal, Raider 1, established angels 3"*
   - ATC, deck clear: *"…signal Charlie"* (cleared to commence)
   - ATC, deck busy: *"…roger, hold angels 3"*
   - Triggers: `established angels` · `established at angels`

5. **Commencing → LSO handoff** (Marshal hands you to paddles here).
   - Pilot: *"Marshal, Raider 1, commencing"* (optionally with state)
   - ATC: *"…copy commencing, state 5.0, contact paddles"* — switch to LSO/Paddles freq.
   - Internal: stack collapses silently — no step-down radio call.
   - Note: prior versions stopped at "copy commencing" and relied on §6 below for the handoff. Now §5 hands off in one call.

6. **3-mile initial → LSO handoff** (fallback if you skipped commencing).
   - Pilot: *"Marshal, Raider 1, initial"*
   - ATC: *"…contact paddles, good luck"* — switch to the LSO/Paddles freq.
   - Trigger: `initial`
   - Note: prior versions cited "push button 72" — dropped since pilots know to switch.

---

## Special intents (any tower freq)

| Intent | What to say |
|---|---|
| **Radar check** | *"radar check"* — ATC reads back angels/range/bearing from Tacview |
| **Comm check** | *"radio check"* / *"comm check"* / *"comp check"* / *"how copy"* |
| **Emergency** | *"mayday"* / *"pan pan"* / *"declaring emergency"* — priority handling |
| **Going around** | *"going around"* / *"missed approach"* — sequenced back into pattern |
| **Traffic in sight** | *"traffic in sight"* / *"tally"* / *"visual"* |
| **Negative contact** | *"negative contact"* / *"no joy"* / *"no contact"* |

---

## Tips for being heard

- **Address the field by name.** *"Al Minhad Tower, ..."* / *"Al Dhafra Tower, ..."* / *"Al Ain Tower, ..."* — bare *"Tower, ..."* also works on the field's freq.
- **Pause briefly after PTT** before speaking; the system uses 400 ms of silence to flush your transmission.
- **Speak full callsigns once per call.** *"Raider 1"* not just *"1"*. ATC needs a callsign to respond.
- **If ATC didn't respond,** it likely missed the intent. Try a different phrasing from the trigger lists above. Common gotcha: *"requesting landing on runway 27"* misses; *"on final"* or *"request landing"* hits.
- **One radio per role** — encryption **off**, modulation **AM** for all SkyeyeATC freqs.

---

## Glossary

| Term | Meaning |
|---|---|
| **Angels N** | Altitude in thousands of feet. *"Angels 5"* = 5,000 ft. |
| **BRC** | Base Recovery Course. The carrier's current magnetic heading; pilots use it to intercept the recovery pattern. |
| **Bingo** | Fuel state at which you must RTB. Marshal flags state < 2.0 as priority. |
| **Case 1 / 2 / 3** | Carrier recovery procedure based on weather. Case 1 = visual day (ceiling ≥ 3000 ft). Case 2 = marginal day. Case 3 = night or hard IFR. Marshal auto-derives this from weather ceiling. |
| **CTAF** | Common Traffic Advisory Frequency. The shared traffic freq when no controlled tower is active. |
| **DME** | Distance Measuring Equipment. *"7 DME"* = 7 nautical miles by DME. |
| **EAM** | External AWACS Mode — how SkyeyeATC's atc.exe roles register with SRS as headless clients. |
| **Fence in / out** | Crossing into or out of the combat area; arm/disarm systems. |
| **Marking mom** | "Marking on Mother" — initial inbound check-in to Marshal. *Mom* = Mother = the carrier. |
| **PTT** | Push-to-talk. |
| **Signal Charlie** | Cleared to commence approach to the carrier. |
| **Tally / Visual / No joy** | Tally = visual on bandit; Visual = visual on friendly; No joy = no visual contact. |
| **TX / RX** | Transmit / Receive. |

---

## Troubleshooting "ATC didn't respond"

If you make a call and Tower stays silent, work down this list:

1. **Wait 2-3 seconds.** STT + intent matching + TTS round-trip is ~1.5-2.5s under normal load.
2. **Did you address the field?** *"Tower, Raider 1, …"* on the field's freq is fine. *"Raider 1, …"* with no addressee may be ignored.
3. **Re-key with a different phrasing.** The trigger lists above show what hits. Common misses:
   - *"requesting landing on runway 27"* → use *"on final"* or *"request landing"*
   - *"wheels down"* / *"touchdown"* → use *"runway vacated"* or *"clear active"*
   - *"ready to roll"* → use *"request takeoff"* or *"ready for departure"*
4. **Did you say a callsign?** Tower needs one to respond. If you forgot it, repeat the call with *"Raider 1"* (or your callsign).
5. **Check your SRS overlay** — does the radio's TX indicator light up when you PTT? If no light:
   - Wrong radio selected for transmit (see Tips section)
   - Encryption stuck on that freq — toggle off
   - Frequency outside the radio's TX band

If still silent on Tower freq but other pilots are getting responses, message the operator — Whisper STT may be hung or the role is offline.

---

## Test-your-setup checklist

When you join a new mission, do this once before you matter:

- [ ] **SRS standalone client** is connected to the squadron server (address bar shows correct host:port).
- [ ] **Radios tuned and set to AM**, encryption off.
- [ ] **PTT bind** working — push and watch the SRS overlay TX indicator light up.
- [ ] **Mic test**: tune your Tower freq, key up, say *"Tower, [callsign], radio check"*. Expect *"loud and clear"* / *"five by five"* in 2-3s.
- [ ] **ATIS check**: tune your destination's ATIS, listen for the current runway and altimeter. If you hear the broadcast clearly you're hearing OK; if pilots' radio chatter overlaps it, you may have stuck/squelched encryption.

If radio check fails, see Troubleshooting above before pushing to taxi.
