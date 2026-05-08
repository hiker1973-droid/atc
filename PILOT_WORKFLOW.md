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

5. **Commencing** — pushing from the stack.
   - Pilot: *"Marshal, Raider 1, commencing"* (optionally with state)
   - ATC: *"…copy commencing"*
   - Internal: stack collapses silently — no step-down radio call.

6. **3-mile initial → LSO handoff** (Marshal's last call to you).
   - Pilot: *"Marshal, Raider 1, initial"*
   - ATC: *"…push button 72, check in"* — switch to the LSO/Paddles freq.
   - Trigger: `initial`

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
