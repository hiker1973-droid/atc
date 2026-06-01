# SkyeyeATC — Squadron Guide

vSFG-7 pilot-facing overview of the AI ATC system on Training 1. Quick reference
for what to say, what's new in **v1.4.0**, and how the roles differ. For a
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
| ATIS × 5 stations | 248.20 / 248.30 / 248.50 / 248.55 / 248.85 | nova | Weather + active runway broadcast, ~every 45s |
| Al Minhad Tower (OMDM) | 250.10 | nova | Ground + tower |
| Al Ain Tower (OMAL) | 250.70 | alloy | Ground + tower |
| Al Dhafra Tower (OMAM) | 251.10 | shimmer | Ground + tower |
| Marshal | 306.30 | coral | Carrier inbound stack |
| Deckboss | 128.60 | fable | Carrier deck ops (cat assignments, launch) |
| Command | 282.00 | sage | Squadron command / mission handoff |

As of v1.4.0 every live role has a distinct voice (all female-leaning) — even
if you miss the callsign you can tell which role is talking, and which tower
when you've got two fields up in parallel. ATIS shares its voice with Minhad
Tower (nova) but you hear them on different freqs so it doesn't collide
in practice.

---

## What's new since v1.4.0 — Case 3 carrier recovery

Marshal now understands **Case 3 (night / IMC) recovery**. The flow and triggers
are the same as Case 1; the phraseology changes to match the kneeboard's CV-1
descent profile.

Marshal auto-detects the recovery case from mission time + ceiling + visibility:
- **Case 1** — day, ceiling ≥ 3000 ft, vis ≥ 5 nm (default flow)
- **Case 2** — marginal day (ceiling 1500–3000 ft *or* vis 3–5 nm)
- **Case 3** — **night, or** ceiling < 1500 ft, *or* vis < 3 nm

The case can flip mid-recovery (dusk, weather change with new `--static-*`
flags on relaunch). When it does, you'll hear a stack-wide advisory:
*"All marshal aircraft, recovery is now Case Three, acknowledge on next push."*

**Case 3 phraseology differences:**

| Pilot call | Case 1 | Case 3 |
|---|---|---|
| `marking moms` | stack angels, see me at ten | radial + angels + **commence at NN** |
| `established angels N` | signal Charlie *or* hold | **commence at NN** (no Charlie) |
| `commencing` | check 3-mile initial, paddles handoff | descend platform 5000, **final bearing**, **confirm BRC** |
| `platform` (new) | — | copy platform, **final bearing** |
| `initial` | contact paddles | (don't use — call platform) |

- **Assigned radial** = BRC + 180 (reciprocal). 250 kt holding pattern, 1-min legs, 6 nm DME inbound leg per the kneeboard.
- **Commence at NN** = minute-of-hour you leave the hold. First aircraft gets +10 min lead, each subsequent slot adds 1 min.
- **Final bearing** = BRC − 9° (deck-angle offset).
- **From the Abe** wording — Case 3 radar callouts reference CVN-72 ("the Abe") instead of generic "from mother".

The full Case 3 walkthrough is in `PILOT_WORKFLOW.md` under "Carrier ops — Case 3 variations". The phraseology spec is in `marshal_responses.md`.

---

## What's new in v1.4.0

### Holding short → cleared for takeoff (no second call)
Call *"Tower, [callsign], holding short runway 27"* and Tower now runs the
two-stage clearance automatically:

1. Immediate ack: *"…runway two seven, line up and wait."* — you taxi onto
   the runway and hold position.
2. ~5 seconds later: *"…wind two seven zero at one zero, runway two seven,
   cleared for takeoff. Report airborne when clear."*

You no longer have to call *"request takeoff"* / *"ready for departure"* as
a second transmission — Tower clears you on its own once it's scanned final.
The manual *"request takeoff"* trigger still works (§6) if you skipped the
hold-short call or got held for spacing and want to nudge.

The 5 s gap is real: Tower re-checks no new inbound within hold-short range
and that the 60 s departure-spacing window has passed before TX2 fires.
If a new inbound shows up inside the gap, the auto-clearance is skipped
silently and the proactive monitor takes over from there.

### "Pushing button 4" now acks
The §9 handoff says *"…contact vSFG-7-Command, two eight two point zero,
channel four"*. Pilots who shortform the response as *"…pushing button 4"*
/ *"…pushing channel 4"* / *"…pushing 4"* now get a courtesy ack instead
of silence. Previous wording (*"pushing command"* / *"switching command"*)
still works.

### Distinct voice per role
Each live role gets its own voice (see table above). Two birds in parallel
on different tower freqs are now audibly distinguishable. Deckboss moved
off `ash` (male) to `fable` to match the rest of the female-leaning roster.

### Departure queue visible on the operator dashboard
The launcher dashboard (operator screen at `http://localhost:7000/`) now has a
full-width "Departures — Hold Short & Takeoff Queue" panel above the
landing-pattern row. Aggregates all three towers; per-aircraft state pill is
one of `queued` / `hold-short` / `luaw` / `cleared`, with a live countdown
on the LUAW gap. Per-airfield colour cue (OMDM purple, OMAM pink, OMAL gold).

What this means for pilots: the operator can now actually see you stacked
in the departure queue — slot number, state, and "X seconds to takeoff
clearance" while you're in the LUAW gap. If something looks stuck, the
operator has the full picture without tailing logs. **If a call seems to
have missed and you're holding short, give it the 5 s gap before re-calling
— the operator can see whether you're already in the auto-release window.**

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

## Known issues (current)

Things that work most of the time but have caveats. Different from
the "Known limits" section below (which is "feature doesn't exist") — these are
"feature exists but watch for X." Entries are tagged by the release they apply to.

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

- **Proactive departure monitor wired via the LUAW path.** v1.4.0's auto-
  release sets the `HoldingShort` flag the proactive monitor was waiting on
  in v1.2.0. So if scheduleAutoRelease misses its window (e.g. a new inbound
  appeared inside the 5 s gap), the proactive monitor will fire the clearance
  on its next tick once the field clears. The `request takeoff` /
  `request departure` path doesn't set the flag and still relies on
  pilot-triggered calls — same as v1.2.0.

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
- `Tower, [callsign], switching to command` · `pushing command` · `pushing button 4` · `pushing channel 4` · `pushing 4`

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
- `Marshal, [callsign], see you at ten` (Case 1 — radar contact at 10 nm)
- `Marshal, [callsign], state [N.N]` (fuel update)
- `Marshal, [callsign], established angels [X]` (in stack — Charlie in Case 1, EAT push in Case 3)
- `Marshal, [callsign], commencing` · `commencing, state [N.N]` (leaving stack — Case 1 to 3-mile initial, Case 3 to platform)
- `Marshal, [callsign], platform` (Case 3 only — passing 5000 ft descending)
- `Marshal, [callsign], initial` (Case 1 only — paddles handoff at 3 nm)
- `Marshal, [callsign], say BRC` · `request BRC` (mother's heading)

### To Deckboss (128.60 — carrier deck)
- `Deckboss, [callsign], radio check`
- `Deckboss, [callsign], request taxi` · `green jet` (ready for cat assignment)
- `Deckboss, [callsign], ready cat [N]` · `tension cat [N]` · `shoot` (under tension)
- `[callsign], airborne` (post-launch ack — slot is already cleared after shoot, so this is optional)
- `Deckboss, [callsign], remain in bolter pattern` (trap practice — Deckboss replies "stay 600 ft, 1 mile out")
- `Deckboss, [callsign], say BRC` · `request BRC` (mother's heading)

### To Command (282.00 — squadron net)
- `Command, [callsign], checking in` (mission-net check-in, then proceed)

### ATIS (248.20–248.85)
Listen only — no transmit. Rebroadcasts every ~45 seconds with weather,
runway, and altimeter.

---

## Voice cues
- **nova** → ATIS broadcasts AND Al Minhad Tower (different freqs, no collision)
- **shimmer** → Al Dhafra Tower
- **alloy** → Al Ain Tower
- **coral** → Marshal
- **fable** → Deckboss
- **sage** → Command

If two Tower voices sound the same to you, you're not on what you think
you're on — check the freq.

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
