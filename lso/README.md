# SkyEye LSO ("Paddles")

Status: **v1 built 2026-10-17, talking, not yet flown.** Operator 2026-10-17:
"Lets do LSO but use open AI for now. Put it on Channel 4 (Strike) Comm 2 at
128.3." Rulings: live calls from the first build (no silent step), 128.300 is
Paddles only, Day + Night (Abe) first.

## What v1 does

- Role `--lso-only` (`cmd/atc/lso.go`), bat `start_lso_caucasus.bat`, started by
  `start_region_caucasus.bat` / `start_all_caucasus.bat`; log `atc-lso.log`.
- 128.300 AM, SRS name "Paddles", OpenAI voice `ash`; every fixed call is
  rendered at start-up so a call costs only the ExternalAudio start.
- Own Tacview reader: carrier (CVN_*) heading and speed from its last 15 s;
  player jets (Pilot "X | name") within ~3 nm. Feed measured 2026-10-17 on Day:
  ~4.7 Hz per aircraft, no AoA/IAS in the stream (AoA = pitch - flight path).
- `pkg/lso`: Nimitz deck (AIRBOSS values: stern -164 m, wires 55/67/79/92 m from
  the stern, angled deck -9.14 deg, deck 20.1 m), 3.5 deg glideslope to the 3 wire
  flown by the hook (2.5 m below the jet's reference point).
- Calls: roger ball / Clara (on the pilot's ball call), call the ball (none by
  ~0.6 nm), high / a little high, low / power, right for lineup / come left,
  fast / slow (Hornet only, AoA estimate +/-2 deg), wave off (low or badly lined
  up in close), bolter. Max one call per 2 s, same call not within 4.5 s; stale
  calls dropped.
- One log line per pass: outcome, touchdown point vs the 3 wire, est. wire,
  sample rate and max gap, X/IM/IC/AR glideslope + lineup (+ AoA), calls said.

## Calibrate next

- Landing point: `--lso-landing-lateral-m` and the touchdown numbers in the
  pass log (traps should centre on 0 m from the 3 wire).
- Thresholds and call timing from flown passes; ExternalAudio start latency.
- Not in v1: foul-deck wave off, grades spoken, LSO on Foothold.

## Steps

1. **Silent shadow** (log only) — see below.
2. **Live calls** on the Paddles frequency from pre-rendered clips: roger ball,
   high/low, power, lineup, wave off, bolter. First measure trigger-to-audio
   latency (TTS ~0.3-0.5 s plus an ExternalAudio spawn per transmission); a
   wave-off later than ~1 s is useless.
3. **Grades and wire** into the comms report and the dashboard.

## Step 1 — silent shadow

Data: the role's own Tacview feed (as every role has today), plus the carrier
contact and BRC the same way Marshal finds them (`findCarrierContact`).

- **Feed check:** per-jet sample rate and largest gap in the groove; parse ACMI
  `AOA` and pitch if DCS sends them (`tacviewLoop` reads only `T=` and `IAS`
  today). Decides whether real-time calls and "fast/slow" are possible.
- **Geometry** from the carrier's live position and heading: landing area
  ~9° left of BRC, touchdown point aft of the carrier point (calibrated from
  real traps), 3.5° glideslope error (ft / deg), lineup (ft), closure net of
  carrier speed.
- **Pass detection:** airborne, < 1 nm astern, < 1,000 ft, closing, aligned →
  trap (stops on deck; wire estimated from stop point) / bolter or
  touch-and-go / wave-off. Segments X (start), IM, IC, AR.
- **Shadow calls:** timestamped "would say" calls from fixed thresholds per
  segment, rate-limited. "Roger ball" on a ball call heard on the Paddles
  frequency (Whisper renders "ball" as "bolt" — heard on 306.3 2026-10-16).
- **Output:** one summary log line per pass, e.g.
  `LSO pass Raider 302: 1.0→0 nm in 17 s, 4.1 Hz (max gap 0.6 s), AoA yes;
  GS +40 ft X / +15 IM / −10 IC; lineup 20 ft left; trap, est. 3 wire;
  would call: roger ball 0.0 s, high 2.1 s, right for lineup 6.3 s`
- **Tests:** synthetic tracks (OK, high start, low in close, lined up left,
  bolter, wave-off).
- **Validation:** 3-5 deliberate passes flown and compared with the log /
  Tacview replay; then tune touchdown point and thresholds.

## Open questions

- Paddles frequency / preset channel.
- All three rigs, or Day + Night (Abe) first?
- Operator test passes, or wait for squadron recoveries?
- Paddles voice (step 2).

## Data source options

- **Tacview (chosen for step 1):** already connected on every rig; no DCS change.
- **DCS-gRPC:** the v1.3 plan's source (AoA, gear, hook). It was disabled over a
  LotATC Export.lua conflict, and that plan predates the rigs — revisit only
  if Tacview lacks AoA.
- **DCS server hook / mission script:** landing events, fast positions; no AoA,
  no wire number.
- **MOOSE AIRBOSS:** full LSO with grading, but brings its own Marshal —
  reference for grading rules only.

## Files

- `LSO_PLAN_v1.3_grpc.md` — the earlier (2026) DCS-gRPC plan: thresholds,
  grade table, call list, state machine. Superseded on data source and
  hosting, still useful for the phraseology and grading.
- Code, when built: `pkg/lso/` (geometry, pass detection, calls) and the role
  wiring in `cmd/atc/`.
