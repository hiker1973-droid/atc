# SkyeyeATC — Squadron Guide

vSFG-7 pilot-facing overview of the AI ATC system on Training 1. Quick reference
for what to say, what's new in **v1.2.0**, and how the roles differ. For a
step-by-step "cold start to RTB" walkthrough see `PILOT_WORKFLOW.md`.

---

## What it is

An AI tower controller for DCS World. Talks to you over SRS using speech
recognition (Whisper) and AI-generated voice (TTS). Knows where every aircraft
is via Tacview — so when you call "10 mile final" or "marking moms 30 DME"
it can actually see you.

Six roles, all live on Training 1:

| Role | Frequency | Voice | Purpose |
|---|---|---|---|
| ATIS × 5 stations | 248.20 / 248.30 / 248.50 / 248.55 / 248.85 | onyx | Weather + active runway broadcast, ~every 45s |
| Tower × 3 fields | 250.10 / 250.70 / 251.10 | onyx | Ground + tower for OMDM / OMAL / OMAM |
| Marshal | 306.30 | ballad | Carrier inbound stack |
| Deckboss | 128.60 | ash | Carrier deck ops (cat assignments, launch) |
| Command | 282.00 | sage | Squadron command / mission handoff |

Voices differ on purpose — even if you miss the callsign you can tell roles
apart by ear.

---

## What's new in v1.2.0

### Smarter departure sequencing
When two aircraft are ready to launch back-to-back, the second now hears
something like:

> *"Raider 39, Al Minhad Tower, hold short runway zero niner, departure spacing
> in forty five seconds. You're number two for departure behind Raider 032."*

Tower enforces a minimum 60-second gap between successive takeoffs. **You don't
need to re-ask** — when your window opens, Tower clears you automatically.

### Lower departure altitudes (3-6 instead of 5-7)
Initial climb-out altitudes are now randomized angels 3-6 across all three
towers (was 5-7). Better fit for UAE airspace MOA/CAS floors. The exact
altitude is randomized per call so successive departures don't stack.

### Cleaner replies
Tower no longer parrots your phrase into its response. Old behavior:

> Pilot: "Tower, Venom 200 is airborne."
> Tower: ~~"Venom 200 is, Al Minhad Tower, proceed to angels seven..."~~

New:

> Tower: "Venom 200, Al Minhad Tower, proceed to angels five, contact tower at seven DME."

Same fix applies to "this is X declaring emergency", "is clear of the active
runway", and a few other phrasings that used to leak into the reply.

### Fewer dropped short calls
STT minimum audio length bumped to 200ms (was 60ms). Stops Whisper from
hallucinating trigger words on radio key bumps and near-silence. If you have a
very fast call that's getting dropped, let us know — we can dial it back.

### Carrier ops polish
- Marshal stack assignments now include your position number on entry
  (*"entering stack at angels seven, position three, hold for Charlie"*).
- Carrier match prefers the CVN-named unit (CVN-72, CVN-73, etc.) over the
  generic "Carrier strike group" label when both are visible.
- DME position reports are recognized inbound: *"Marshal, Raider 39, 7 DME"*
  gets a radar-confirmed ack.
- **Commencing acks + sends you to the 3-nm initial; "initial" is the
  paddles handoff.** Marshal acks commencing and instructs you to check
  in at the 3-mile initial: *"Raider 39, Marshal, copy commencing, state
  5.0, check in three mile initial, paddles handoff."* You stay on
  Marshal freq, continue the approach, and call *"Marshal, Raider 39,
  initial"* at 3 nm — Marshal then hands off to paddles. No TACAN
  button cited ("push button 72" was dropped since pilots know to switch).

### Quality-of-life
- Go-around storm fix — Tower no longer repeats a go-around clearance multiple
  times in a row.
- Marshal-test radio chatter is gated to the dev rig only — won't appear on 306.3
  during live missions.

---

## Known issues (v1.2.0)

Things that work most of the time but have caveats this release. Different from
the "Known limits" section below (which is "feature doesn't exist") — these are
"feature exists but watch for X."

- **Carrier match not live-validated.** The CVN-named-ship preference (CVN-72,
  CVN-73, etc. winning over generic "Carrier strike group" labels) only got
  test coverage, not real-mission exercise. If Marshal addresses the wrong
  ship or seems confused about which boat is yours, **drop the exact transcript
  in chat** so we can dial in the keyword priority.

- **`--position-check` is opt-in.** Tower's Tacview hold-short validation
  defaults off. If the operator enables it for your session and you get
  *"unable to confirm hold-short position, say position"* — that means either
  you're still on the taxiway, or the runway-threshold coordinates in our
  config are off. Verify visually and re-call; ping if it persists.

- **Very short calls (sub-200ms) can be dropped.** STT min-audio was tightened
  to suppress Whisper hallucinations. A rushed *"Tower-callsign-holding-short"*
  delivered as one syllable might not register. **Slow down half a beat** and
  it'll land cleanly.

- **No LSO/Paddles role yet.** Marshal hands you off to paddles on
  "commencing" (or "initial" if you skipped commencing), but the LSO
  freq itself is silent — no "call the ball" / waveoff calls. Land using
  visual ball + lineup judgment. Planned for a later release.

- **Departure-spacing only via pilot-triggered calls.** v1.2.0's 60-second
  spacing gate fires when you call "request takeoff" or "holding short" —
  that's the normal flow and is correct. The proactive monitor path (Tower
  clears the queue automatically when no one's asking) is dormant for an
  unrelated state-flag wiring gap. Doesn't affect normal ops; flagged here
  because it's documented in CLAUDE.md for the operator.

- **Pilot-to-pilot chatter sometimes shows up in operator logs.** When you
  talk to your wingman on a Tower freq, Tower transcribes it but **doesn't
  respond** (the address-led guard catches it correctly). Operators will see
  "intent miss" log entries — that's expected, not a bug.

- **Whisper mishears that haven't been auto-corrected yet.** New patterns
  surface every session. Recent catches: *"Reader"* / *"Radar"* → Raider;
  *"Mahat Tower"* / *"Manhattan Tower"* → Al Minhad. If you keep getting a
  consistent mishear that doesn't get fixed within one call, ping with the
  exact word Whisper heard so we can add it to the normalizer.

---

## What you can say

### To Tower (250.10 / 250.70 / 251.10)
Pre-flight:
- `Tower, [callsign], radio check` · `comm check` · `comms check`
- `Tower, [callsign], request startup` · `ready for startup`
- `Tower, [callsign], request taxi` · `request ground`
- `Tower, [callsign], holding short runway [XX]`
- `Tower, [callsign], request takeoff` · `request departure` · `ready for takeoff`

Departure:
- `Tower, [callsign], airborne` · `departing` · `clear of traffic`
- `Tower, [callsign], 7 DME` · `7 miles` · `cleared airspace`
- `Tower, [callsign], switching to command`

Pattern / inbound:
- `Tower, [callsign], inbound, [N] miles, heading [HHH]`
- `Tower, [callsign], 10 mile initial` · `overhead`
- `Tower, [callsign], downwind` · `base` · `short final` · `on final`
- `Tower, [callsign], request landing`

Emergency:
- `Tower, [callsign], declaring emergency` · `mayday` · 

Post-landing:
- `Tower, [callsign], clear of the active` · `runway vacated`
- `Tower, [callsign], going around` · `missed approach`

### To Marshal (306.30 — carrier inbound)
- `Marshal, [callsign], radio check`
- `Marshal, [callsign], marking moms, [N] DME, angels [X], state [N.N]`
- `Marshal, [callsign], [N] DME` (en-route position report)
- `Marshal, [callsign], see you at ten` (radar contact request at 10 nm)
- `Marshal, [callsign], state [N.N]` (fuel update)
- `Marshal, [callsign], established angels [X]` (in stack, ready for Charlie)
- `Marshal, [callsign], commencing` · `commencing, state [N.N]` — Marshal acks AND hands off to paddles in one call
- `Marshal, [callsign], initial` — fallback handoff at 3 nm if you skipped commencing

### To Deckboss (128.60 — carrier deck)
- `Deckboss, [callsign], radio check`
- `Deckboss, [callsign], request taxi` · `green jet` (ready for cat assignment)
- `Deckboss, [callsign], ready cat [N]` · `tension cat [N]` · `shoot` (under tension)
- `[callsign], airborne` (post-launch)
- `Deckboss, [callsign], say BRC` · `request BRC` (mother's heading)

### To Command (282.00 — squadron net)
- `Command, [callsign], checking in` (mission-net check-in, then proceed)

### ATIS (248.20–248.85)
Listen only — no transmit. Rebroadcasts every ~45 seconds with weather,
runway, and altimeter.

---

## Voice cues
- **onyx** (deep, neutral) → Tower or ATIS
- **ballad** (warmer, smoother) → Marshal
- **ash** (calm, authoritative) → Deckboss
- **sage** (clipped, businesslike) → Command

If you hear "ballad" on what you thought was Tower, you're on the wrong freq.

---

## Address rules

Every role expects you to **lead with the role name**:
- "Tower, Raider 1, ..." ✓
- "Raider 1, holding short ..." — may be dropped as self-echo

Some short forms accepted (Tower drops "Al " for Al Minhad / Al Ain / Al Dhafra)
but always safer to use the full name.

---

## Known limits

- **No LSO/Paddles role yet** — Marshal hands you to paddles on
  "commencing", but there's nothing on the LSO freq to answer (planned,
  not built).
- **No taxiway routing** — Tower clears you to a runway, not via specific
  taxiways. Use the airfield diagram + sense.
- **Single-ship voice** — wingmen ack on their own callsign each. No
  flight-lead ack-for-all yet.
- **CAS/MOA deconfliction** — basic departure spacing only. Tower doesn't
  see DCS airspace boundaries; trust the briefing.

---

## Troubleshooting

**Tower doesn't respond to my call.**
- Did you lead with the role name? "Tower, ..." is required.
- Try a rephrase using one of the exact triggers above.
- ATIS-only frequencies don't transmit a response — listen only.

**My callsign isn't being heard right.**
- Whisper has known mishears: "Raider" → "Reader" / "Radar", "Venom" → "Vino" /
  "Demon", "Viper" → "Wiper" / "Hyper". The system corrects most of these
  automatically; if it doesn't, slow your call slightly or use the digits
  individually (`"zero three two"` instead of `"thirty-two"`).

**I called "holding short" and got asked to verify position.**
- That's the new `--position-check` gate (off by default; enabled per session).
  Tower's Tacview shows you not at the threshold. Either taxi the rest of the
  way and re-call, or report by visual position on the ground.

**The same call gets a different response on a retry.**
- Each response has 2-4 variants picked randomly to avoid robotic
  repetition. Intentional. The intent is the same.

---

## Where this lives

- **Pilot workflow** (cold start → RTB, step by step): `PILOT_WORKFLOW.md`
- **Phraseology spec** (every trigger + response shape): `*_responses.md`
  (`arrival_responses.md`, `departure_responses.md`, `marshal_responses.md`,
  `deckboss_responses.md`, `command_responses.md`)
- **Operator runbook** (rig admin): `OPERATOR_RUNBOOK.md`
- **Architecture** (how it's built): `ARCHITECTURE.md`

Bug reports / feature asks → ping in the squadron Discord or file in the repo.
