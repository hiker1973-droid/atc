# SkyEye LSO ("Paddles")

Status: **planning — nothing built.** Operator 2026-10-16: "Start a separate
folder for LSO. It will have its own radio channel, not on Marshal."

## Decisions so far

- **Own role, own frequency.** Paddles is a separate SkyEye role (its own
  `atc.exe` process, launcher entry and dashboard port), not part of Marshal.
  Marshal stays on 306.3; Paddles gets its own channel — **frequency TBD**
  (the 2026 v1.3 plan below proposed 128.100).
- **Silent first.** Step 1 only logs what Paddles would have said; nothing is
  transmitted until the data proves good enough.

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
