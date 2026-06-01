# SkyeyeATC — Pilot Cheat Sheet

One-page command reference. Every call leads with the role name (`Tower, …` / `Marshal, …` / `Deckboss, …`) and includes your callsign.

## Frequencies

| Role | Freq | Voice |
|---|---|---|
| ATIS (5 stations) | 248.20 / 248.30 / 248.50 / 248.55 / 248.85 | nova |
| Al Minhad Tower | 250.10 | nova |
| Al Ain Tower | 250.70 | alloy |
| Al Dhafra Tower | 251.10 | shimmer |
| vSFG-7 Command | 282.00 | sage |
| Marshal | 306.30 | coral |
| Deckboss | 128.60 | fable |

---

## Tower (250.10 / 250.70 / 251.10)

**Departure**
| You say | What happens |
|---|---|
| `request startup` | startup approved + altimeter |
| `request taxi` | taxi to active runway, hold short |
| `holding short runway [XX]` | LUAW ack → cleared for takeoff 5 s later (auto) |
| `request takeoff` / `ready for departure` | manual takeoff clearance (fallback) |
| `airborne` / `clear of traffic` | departure release, climb to angels N |
| `pushing button 4` / `pushing command` | courtesy ack, switch to Command |

**Pattern / Arrival**
| You say | What happens |
|---|---|
| `[N] mile initial` | inbound ack, traffic call |
| `3 mile initial` | break clearance (or straight-in if requested) |
| `downwind` / `base` / `final` | pattern sequencing + clear to land |
| `request landing` | landing clearance with wind |
| `runway vacated` / `clear of active` | welcome back, taxi to parking |
| `going around` / `missed approach` | sequenced back into pattern |

**Any time**
| You say | What happens |
|---|---|
| `radio check` / `comm check` | loud and clear |
| `mayday` / `declaring emergency` | priority handling |
| `traffic in sight` / `tally` | acknowledged |
| `negative contact` / `no joy` | repeat traffic call |

---

## Marshal (306.30) — Carrier inbound stack

Recovery case (Case 1 day / Case 3 night-IMC) is auto-detected. Same triggers, different responses. Marshal broadcasts a stack-wide advisory if the case flips mid-recovery.

**All cases**
| You say | What happens |
|---|---|
| `radio check` | loud and clear |
| `marking moms, [N] DME, angels [X], state [N.N]` | weather + case + BRC + altimeter + angels assignment + (Case 3: radial + commence-at) |
| `[N] DME` (with `Marshal,` prefix) | DME ack, continue inbound |
| `state [N.N]` | copy state (priority if <2.0) |
| `say BRC` / `request BRC` | mother's BRC |

**Case 1 (day, good weather)**
| You say | What happens |
|---|---|
| `see you at ten` | radar contact, say state |
| `established angels [X]` | signal Charlie (deck clear) OR hold |
| `commencing` | check in 3-mile initial, paddles handoff |
| `initial` (at 3 nm) | contact paddles, good luck |

**Case 3 (night / IMC)**
| You say | What happens |
|---|---|
| `established angels [X]` | commence at [NN minute] |
| `commencing` | descend platform 5000, final bearing [NNN], confirm BRC [NNN] |
| `platform` (passing 5000 ft) | copy platform, final bearing [NNN] |

Don't call `initial` in Case 3 — call `platform` instead.

---

## Deckboss (128.60) — Carrier deck ops

| You say | What happens |
|---|---|
| `radio check` | loud and clear |
| `request taxi` / `green jet` | cat assignment OR conga line position |
| `ready cat [N]` / `tension cat [N]` / `shoot` | under tension, then auto-shoot 5 s later |
| (auto) cat clear | next conga pilot pulled onto cat at T+15 s |
| `airborne` (post-launch) | copy, good hunting (optional) |
| `remain in bolter pattern` | stay 600 ft, 1 mile out |
| `say BRC` / `request BRC` | mother's BRC |

---

## Command (282.00) — Squadron net

| You say | What happens |
|---|---|
| `checking in` | loud and clear, proceed mission |
| `fence in` / `fence check` | cleared hot |
| `on station` | good hunting |
| `off station` | proceed to assigned pattern |
| `fence out` | safe passage |
| `radio check` | loud and clear |

---

## Address rules

- **Lead with the role name.** *"Tower, Raider 1, …"*, *"Marshal, Raider 1, …"*. Bare *"Raider 1, …"* may be dropped as self-echo.
- **Speak the full callsign once per call.** *"Raider 1"* not just *"1"*.
- **Pause ~½ s after PTT** before speaking — STT uses 200 ms silence to flush.
- One radio per role. AM. Encryption off.

---

## If ATC doesn't respond

1. Wait 2–3 s (STT + TTS round-trip).
2. Did you lead with the role name?
3. Did you say a callsign?
4. Try a different phrasing from the trigger list above.
5. Check SRS overlay — does TX light up when you PTT?

Full troubleshooting in `PILOT_WORKFLOW.md`.
