# SkyeyeATC — Claude Context

AI-powered Tower ATC for DCS World, owned by **vSFG-7**. Layered on SkyEye infrastructure: SRS for radio bridging, OpenAI Whisper for STT, OpenAI TTS for voice, Tacview real-time telemetry for aircraft positions. Three airfields plus carrier ops and a few special channels.

## Where you are
This is **Training 1** — the production rig where big missions run. SkyeyeATC was developed on a separate dev box (Windows Server 2022, vsfg7-atc) and ported here on **2026-04-25**. Treat this as prod: be cautious with destructive operations during live training, no exploratory refactors mid-session.

## Network (env vars, v1.1.0+)
All `.bat` scripts read four env vars and fail fast if any is missing. Same scripts work on dev rig and Training 1 — only the values differ.

| Env var | Training 1 | Dev rig | Read by |
|---|---|---|---|
| `OPENAI_API_KEY` | sk-proj-... | sk-proj-... | `atc.exe` directly (Whisper STT + TTS) |
| `SRS_EAM` | (rotate before mission) | (same) | All `start_*.bat`, passed as `--eam-password` |
| `SKYEYE_SRS` | `localhost:5004` | `localhost:5004` | All `start_*.bat`, passed as `--srs-addr` |
| `SKYEYE_TACVIEW` | `localhost:42676` | `localhost:42676` | Tower / Marshal / Scudwatch / Launcher, passed as `--tacview-addr` |

Set with `setx VAR value`, then open a new cmd so the new values are in scope (setx doesn't affect the running shell). Symptom of a missing var: each script aborts at the top with `ERROR: <VAR> env var not set`.

DCS-gRPC is not externalized — `:50051` is stable across boxes.

## Repo layout
- `cmd/atc/` — main `atc.exe`, runs in different modes via flags (tower / `--atis-only` / `--command-only` / `--marshal-only` / `--scudwatch-only` / `--deckboss-freq`)
- `cmd/launcher/` — `launcher.exe`, web dashboard at `:7000`
- `cmd/logtail/` — log tail/filter CLI
- `pkg/airfield/` — OMDM, OMAM, OMAL definitions (runways, freqs, elev)
- `pkg/state/`, `pkg/controller/`, `pkg/composer/` — state machine, intent + conflict detection, ICAO phraseology
- `configs/{alain,dhafra,minhad}.yaml` — per-airfield configs (still hardcode `localhost:5008` — stale; SRS server actually listens on 5004 here, but these YAMLs are not currently read by `atc.exe` so the mismatch is cosmetic)

## Sites and roles
Three SRS-bridged tower instances, each writing JSONL to `C:/SkyeyeATC/logs/`:

| Log file | Site | Tower freq | Extra roles in this log |
|---|---|---|---|
| `atc-omal.log` | OMAL / Al Ain | 250.7 | — |
| `atc-omam.log` | OMAM / Al Dhafra | 251.1 | — |
| `atc-omdm.log` | OMDM / Al Minhad | 250.1 | **Deckboss** (128.6, "Deckboss", `ash`) — interleaved |
| `atc-marshal.log` | — | — | Marshal only (306.3, "Union Marshal", `onyx`) |
| `atc-command.log` | — | — | Command only (282.0, "vSFG-7-Command", `sage`) |
| `atc-atis.log` | — | — | All 5 ATIS stations |

Log slug is set in `cmd/atc/main.go:231` — `--marshal-only` / `--command-only` / `--atis-only` get their own file; otherwise the log goes to `atc-<airfield>.log`. Deckboss has no `--deckboss-only` flag (it runs as `--airfield OMDM --deckboss-freq 128.6`), so its events mix into `atc-omdm.log` alongside Minhad Tower — filter on `Deckboss`/`128.6` when monitoring.

Role names in logs: `Tower`, `Deckboss`, `Marshal` (single L). User's verbal "Marshall" = the `Marshal` role. When the user says "monitor towers / deckboss / marshal", prefer **`tools/tail-all.ps1`** (canonical, site-prefixed, filters on `heard`/`TX`/role lifecycle/warn/error). Older `tmp_tail*.ps1` files at repo root are throwaway scaffolding from prior sessions — superseded.

Common fault signatures: `level=warn` + `SRS disconnected` · `level=error` + `ExternalAudio file error` · `TTS prewarm failed` · `Whisper returned empty transcription` · `SRS TCP failed`.

## ATIS broadcast cadence
**45 seconds per station** (was 120s, changed 2026-04-25). Set at two call sites — keep them in sync if it ever moves:
- `cmd/atc/atis.go:336` — `atisOnlyLoop` (used by `--atis-only` / `start_atis.bat`)
- `cmd/atc/main.go:537` — tower-bundled path (used when `--atis-broadcast` is on)

5 ATIS stations: Dhafra (248.2), Minhad (248.3), Liwa (248.55), Al Ain (248.85), Khasab (248.5). Startup staggered `15 + i*37` seconds to desync TTS calls. Per-station `broadcasting` mutex (`TryLock`) drops the next tick if previous broadcast is still in progress — if you see `ATIS broadcast already in progress — skipping` in logs repeatedly, audio is exceeding 45s and the interval should creep back toward 60–90s. TTS only regenerates when weather/runway changes; otherwise it rebroadcasts `atis_cache/atis_<ICAO>.mp3`.

## Operator workflow
Start scripts in `C:\SkyeyeATC\`. Recommended order:
1. `start_launcher.bat` — opens dashboard at http://localhost:7000/
2. `start_atis.bat` — all 5 ATIS stations
3. `start_towers.bat` — Minhad / Dhafra / Al Ain (dashboards on 6001 / 6002 / 6003)
4. `start_marshal.bat` — Marshal carrier stack on 306.3 (when carrier ops in mission)

Stop: close the console window, `stop_marshal.bat` to kill only Marshal, or use launcher dashboard buttons.

### Training 1 active roles (2026-05-19)
**Tower**, **ATIS**, **Marshal** (306.3), **Command** (282.0), and **Deckboss** (128.6 — DCS carrier UHF) all run live on Training 1. Bat files at repo root: `start_atis.bat`, `start_towers.bat`, `start_marshal.bat`, `start_command.bat`, `start_deckboss.bat`, `start_launcher.bat`. Post-reboot one-shot: **`start_all.bat`** fires ATIS → towers → marshal → command → deckboss → launcher in order.

Voices are differentiated per role for audible separation on cockpit comms — all female / female-leaning, all six unique. Set via `--tts-voice` in `start_towers.bat` (per-tower override) and per-role voice flags elsewhere: OMDM Tower `nova`, OMAM Tower `shimmer`, OMAL Tower `alloy`, Marshal `coral` (`start_marshal.bat` `--marshal-voice`), Command `sage` (`start_command.bat` `--command-voice`), Deckboss `fable` (`start_deckboss.bat` `--deckboss-voice`). Reassigned 2026-05-29 from prior all-nova-towers / ash-Deckboss. Switchable in the bat files without rebuilds.

Still parked under `dev_only/` (do not launch on Training 1):
- `dev_only/run_scudwatch.bat` — Scudwatch threat broadcaster 264.0

The role *code* for all parked roles stays on `main`; only the launch scripts are gated.

**Dev-only Marshal test mode:** `start_marshal_test.bat` (root, local-only on dev rig — not pushed) launches Marshal with `--marshal-test-tx`, which transmits "test" on 306.3 every 30s. Used for SRS routing diagnostics. Do not run on Training 1 — it would broadcast test traffic on the carrier freq during live missions.

## Gotchas
- **`--grpc-addr` is not a valid flag.** It was removed from `atc.exe`. Old `run_alain.bat` / `run_dhafra.bat` / `run_minhad.bat` files still pass it and will fail at launch — delete them if they are still present.
- **Don't hardcode `C:\SkyeyeATC`** in new Go code. Use `os.Executable()` / `filepath.Dir(argv[0])` so the launcher works from any directory.
- **Don't re-duplicate `OPENAI_API_KEY`** across more `.bat` files. It's already in too many; centralize via `setx`.
- **Build with portability in mind** — Training 1 will not be the last rig.

## How the user works
- Tests roles one at a time and asks Claude to tail the relevant log(s) for the session. They want filtered, site-prefixed events, not raw JSONL.
- Production rig — prefer stable behavior and minimum-blast-radius changes over experimental refactors.
- `/ultrareview` is user-triggered for branch / PR reviews; Claude doesn't launch it.

## Recent context (2026-05-29)
- **v1.4.0 cut** — LUAW two-stage redesign + distinct female voices per role + §10 pushing-button-4 broadening + live departure-queue dashboard. Four feature commits, squadron doc alignment, then the dashboard rolled in. (Note: the prior "post-v1.2.1 batch (untagged, on `main`)" wording in the 2026-05-27 section below was stale — the dev rig had already tagged that batch as v1.3.0 at `e75fa8c`; this batch is therefore v1.4.0 not v1.3.0. Also: v1.4.0 tag was initially placed at the doc commit `b77b57f` after only three commits, then moved forward to include the dashboard work — the tag-move was a non-destructive force-replace on a same-session push.)
- Commits in this release:
  - `7a7a30c` — tower: holding-short LUAW + 5s auto-release (was single-TX takeoff). §3 no-traffic path now fires `HoldShortLineUpAndWait` (TX1) and schedules `scheduleAutoRelease` to fire `ClearedForTakeoff` (TX2) after `AutoReleaseDelay = 5s` with re-checked conditions (no new inbound within `HoldShortRadiusNm`, `DepartureSpacingSec` window honored, still queue head, not already cleared). Proactive monitor gets an `AutoReleaseAt` time-gate so it can't race past the 5s gap. Sets `HoldingShort = true` on entry — fixes the dormant-proactive-fallback path noted in v1.2.0. Prior `HoldShort` composer kept vestigial for a possible future `--no-auto-release` opt-out flag.
  - `4ff25ae` — config: distinct female voices per role. OMDM=`nova`, OMAM=`shimmer`, OMAL=`alloy`, Marshal=`coral` (kept), Command=`sage` (kept), Deckboss=`fable` (was `ash`). `start_towers.bat` per-tower `--tts-voice`, `start_deckboss.bat` `--deckboss-voice fable`. Bat edits only — no rebuild needed; takes effect on next role restart.
  - `e1d754e` — tower: §10 also matches `pushing button` / `pushing channel` / `pushing 4` / `pushing four`. Surfaced by Raider 13's `pushing button 4` getting no TX during the 2026-05-29 16:00 tail. Channel 4 is the Command preset (`HandoffPreset: "channel four"`) on all three towers; conservatively scoped to those literals (not generic `pushing N`) to avoid acking wingman-freq switches.
  - `b6932a0` — dashboard: live departure queue with slot # and on-runway state. New `GetDepartureQueueSnapshot` on state + `GetDepartureQueue` controller passthrough. `StatusSnapshot.Departures []DepartureEntry` exposes callsign, slot, state (`queued` / `hold-short` / `luaw` / `cleared`), and `secsToGo` countdown for the LUAW gap. Launcher UI gets a new full-width "Departures — Hold Short & Takeoff Queue" panel above row-3, aggregated across all three towers with per-airfield colour cue and per-state pills. Operator-facing visibility into the departure side which the dashboard had never surfaced.
- **Squadron docs aligned** — `SQUADRON_GUIDE.md` header bumped to v1.4.0, new What's-New section, voice table + cues refreshed, departure-spacing known-issue rewritten to reflect proactive path now wired via LUAW. `PILOT_WORKFLOW.md` steps 4–6 collapsed/renumbered for auto-release flow, §10 shortform mentioned at handoff step. (Dashboard panel is operator-facing only — not surfaced in squadron / pilot docs.)
- **Not yet deployed to Training 1** — `atc.exe` processes still on the 2026-05-27 21:12 binaries. Per the prior decision, build + role restart happens **post-mission** (after 20:00 tonight). Restart procedure: kill all `atc.exe` PIDs (and `launcher.exe` — needs restart for the new ui.html as well as the new JSON) → `build.bat` (or `go build`) → `start_all.bat`.
- **Voice drift root cause noted** — the prior CLAUDE.md voice line ("all roles use `onyx` except Deckboss = `ash`") had been wrong since at least v1.2.x: `--tts-voice` default in `cmd/atc/main.go:154` was `nova` and bat files used `coral` / `sage` / `ash`. Caught when monitoring the 2026-05-29 tail saw `voice:"nova"` on a Minhad TX. Doc + bats now both reflect the same per-role mapping.
- **Tail/monitoring during this batch** — surfaced live evidence of the §3 single-TX flow (Raider 39 holding short 27 → immediate "you are number one for takeoff" single TX) which validated the LUAW redesign target, and the §10 miss on `pushing button 4` which became the third commit. The misleading-log finding (recognized-text logged pre-normalization) closed task #3 as already-implemented rather than needing code.

## Recent context (2026-05-27)
- **v1.1.1 cut and pushed** at `73f0bd3` — controller hygiene patch covering the 2026-05-26 OMDM post-mortem, plus the Deckboss STT fix. Commits in this release:
  - `b401f3f` — Deckboss address-led guard hardened. Whisper renders "Deck boss" as one word often enough ("TechBoss", "DuckBoss") that the spaced-only `addrPrefixes` list in `cmd/atc/deckboss.go:98` silently dropped legitimate §1/§2/§6 calls as self-echo — pilot would call comm-check and Deckboss would never respond. Added no-space variants (`techboss`, `duckboss`, `decboss`, `checkboss`). **General rule:** when adding a Whisper-defensive prefix, add both spaced and no-space forms.
  - `f161c01` — callsign extractor strips leading filler ("this is"/"i am") and trailing connectors (is/was/now) after the trigger cut. Added triggers: `declaring`, `in-flight`, `clear of`, `is clear`. Fixes the "Venom 2-0-0 is, Al Minhad Tower, ..." TX bleed that recurred in three different shapes that day. Also: go-around debounce — 60s per-callsign cooldown via `goAroundLastTx` map on `ATCController`, plus reapply `fresh.GoAroundWarned=true` after `Remove`/`GetOrCreate` re-enqueue. Root cause of the 6×-in-70s storm was the per-aircraft flag silently zero-resetting on the new struct.
  - `73f0bd3` — `findCarrierContact` logs at info only on chosen-callsign **transition** (with `prev` field), debug otherwise.
- **v1.2.0 cut and pushed** at `2ad751a` — departure queue overhaul. Five commits since v1.1.1:
  - `f770223` — Whisper min-frame floor bumped from `> 3` (≈60ms) to `> 9` (≈200ms) across all four srsLoops (main / command / marshal / deckboss). **Tuning knob** — dial back to `> 6` if pilots report rushed short calls being dropped.
  - `f5e0ec4` — `dev_only/marshal_smoke.md` checklist for §1-§6 Case 1 Recovery.
  - `a2bf187` — departure altitude range 5/6/7 → 3/4/5/6. `composer.go:119` is now `3 + rand.Intn(4)`; matching TTS prewarm cache updated. Per-airfield `DepartureAngels` config remains ignored (composer randomizes regardless) — kept for a future per-airfield knob.
  - `8eb1782` — departure spacing timer: `DepartureSpacingSec=60s` on `AirfieldState.LastDepartureClearedAt`. Pilot-triggered requests inside the window get a `HoldForSpacing` response; proactive monitor silently skips and retries. Driven by yesterday's 19:30:44 / 19:31:07 Raider 032 / Raider 39 double-clear 23s apart.
  - `cce8296` — queue-position suffix: hold responses for aircraft at positions 2-3 gain a single-TX suffix ("You're number two for departure behind Raider 032"). `AircraftState.AnnouncedQueuePos` flag prevents re-announce.
  - `2ad751a` — Tacview hold-short position gate: `--position-check` CLI flag (default off). When on, validates pilot is within `HoldShortValidationNm=0.5` of runway threshold before treating "holding short" as legit. Fails open on missing telemetry; logs `level=warn` on mismatch. New `airfield.ThresholdFor(designator)` helper for runway threshold lookup.
- **Spec docs updated** in `departure_responses.md`: §6b (verify hold-short position), §7a (queue-position suffix), §7b (hold for spacing), §8 (angels range).
- **New CLI flag** — `--position-check` (default false). Roll out via: one session opt-in to observe `position check — pilot claims hold-short but Tacview shows them elsewhere` warns, then promote to default in `.bat` files once threshold-coords are validated.
- **Latent bug noted, not fixed** — `state.AircraftState.HoldingShort` is set only by `state.SetHoldingShort` which is **never called from controller code**. The proactive departure monitor at `controller.go:792+` requires `next.HoldingShort==true` so that path is currently dormant. The pilot-triggered paths (`handleHoldingShortRequest` / `handleTakeoffRequest`) bypass the flag and work fine — but v1.2.0's spacing timer and position gate will only fully apply via the proactive path once `SetHoldingShort` gets wired into the RequestHoldingShort case.
- **v1.2.1 cut and pushed** at `f90947a` — Marshal LSO handoff phraseology (single change). Commit `d0081f2` made `commencing` the one-call ack + LSO handoff and dropped the TACAN button reference. `marshalTacanChannel` constant removed; `MarshalPushButton` no longer takes a TACAN arg. Spec doc updated; `PILOT_WORKFLOW.md` / `SQUADRON_GUIDE.md` / `dev_only/marshal_smoke.md` aligned. **This was later reversed — see Post-v1.2.1 below.**
- **Post-v1.2.1 phraseology + Deckboss flow batch (untagged, on `main`)** — four commits past v1.2.1, ready for the next version cut:
  - `50c0592` / `548aa2b` — **REVERTS v1.2.1's commencing-as-handoff.** Marshal now acks `commencing` and instructs the pilot to check in at the 3-mile initial. §6 `initial` becomes the actual paddles handoff (was demoted to fallback in v1.2.1; now back to primary). Spec + composer + pilot/operator docs updated.
  - `7688d64` — **Deckboss §2a flow change** (behavioral) + **new §8 bolter pattern intent** (new feature). §2a: cat slot freed and next-conga pulled at shoot, not at airborne — next-up hears `<callsign>, Deckboss, cat N is clear` immediately after the previous pilot is fired off. §4 airborne demoted to "copy good hunting" ack with the old slot-management as fallback for the unparseable-cat-number edge case. §8: new `remain in bolter pattern` trigger → `DeckbossBolterPattern` composer method, replies with "stay 600 feet, 1 mile out" (touch-and-go trap practice). `DeckbossCatClear` now embeds "Deckboss," in the text so next-up knows who's calling.
  - `b683185` — Cat-clear delay bumped 3s → 10s post-shoot. Lands at T+15 from under-tension (T+0 tension, T+5 shoot, T+15 clear). Gives launching aircraft time to taxi off the cat before next-up announcement.
  - **Version implication:** the bolter intent is a new feature on an existing role, so per [[version-tag-scheme]] the next tag should be **MINOR (v1.3.0)** rather than v1.2.2. Cut once these are validated live on Training 1.
  - **Deployed** to Training 1 at 2026-05-27 21:12 (kill all 7 atc.exe → `go build` → `start_all.bat`). Clean startup, TTS cache warmed, no warns/errors.
- **Restart procedure validated** — full rebuild + role-restart cycle (ATIS → Towers → Marshal → Command → Deckboss). When `build.bat` fails with "atc.exe being used by another process", kill all `atc.exe` PIDs first. Launcher is `launcher.exe` (separate binary, separate process) — `build.bat` rebuilds both, so kill it too if a launcher is running.
- **Duplicate Tower processes** observed pre-restart (two each of OMAL/OMAM/OMDM, same dashboard ports). Cause unknown — likely a `start_towers.bat` re-run without first stopping the previous set. Only one of each pair can bind its dashboard port; the other is a zombie. Resolved by killing all six and re-running `start_towers.bat` once.

## Recent context (2026-05-17)
- **Deckboss live on 128.6** (DCS carrier UHF, was 306.2). Brought up with: IPv6 UDP fix (`net.ResolveUDPAddr` + `net.DialUDP` like Marshal — `net.Dial("udp", ...)` was binding `::1` and silently dropping audio), §1/§2 trigger alignment with `deckboss_responses.md` (now accepts `request taxi`/`green jet` and `(tension|ready)+cat`), self-echo guard on §1/§2/§6, voice default `ash` via new `--deckboss-voice` flag, `Clea to launch` typo fixed. Commits `716e588` / `7b95c74` / `fe792d8`.
- **DME position report §1b wired** in Marshal — "Marshal, Raider 39, 7 DME" → ack with radar confirm if Tacview has the caller. Commit `793837e`.
- **MarshalEstablishedAck includes stack position** — "entering stack at angels XX, position YY". Commit `f8a9c71`.
- **Versioning** — semver tags (`vMAJOR.MINOR.PATCH`). `v1.1.0` cut at `793837e` to collapse the Training-1-era backlog since `v1.0.1`. Next minor when the next feature batch lands (likely v1.2.0 with Deckboss-live + DME).

## Recent context (2026-05-11)
- **UDP audio parser bug fixed** on Training 1 in local commit `063f239` (Marshal/Command read `audioLen` from `[4:6]` — actually `freqSegLen` — instead of `[2:4]`). Origin landed the equivalent fix independently in `96cfe21`; merge of `2026-05-16` took origin's version and added per-callsign unitId / UDP IPv6 / Union Marshal rename on top.
- **Pilot-side SRS-Client `RadioModelsCustom` parse failure** was observed silently swallowing pilot TX on 282 specifically (Tower freqs OK, Marshal/Command silent). Training 1 Command was temporarily moved to 230.0 (commit `5956c10`). Pilot-side fix landed 2026-05-18; Command reverted to 282.0 same day.

## Recent context (2026-05-08)
- **Marshal/Command 282.0 routing bug fixed.** Root cause was the SRS handshake order in `command.go`/`marshal.go`/`deckboss.go`: they sent EAM before Sync, while Tower's `srsLoop` sent Sync before EAM. SRS only adds clients to the audio routing table on Sync — broken roles authenticated but never received UDP voice. Commit `924784f`. Marshal and Command both deploying live on Training 1 as of 2026-05-08.
- Audio perf: STT switched to `gpt-4o-mini-transcribe`, flush ticker tightened from 500ms → 200ms in `srsLoop` (commit `461299f`).
- New `RequestStartup` intent (`request startup` / `ready for startup` / `ready to start`) — Ground-callsign approval before taxi (commit `dafad18`). Trigger additions: `request departure`, `comcheck`, `comp check`.
- Departure release distance bumped 5 → 7 nm across all three towers (commit `eb9803a`).
- `controller.go` airborne fix: dropped `isCTAF` guard so `airborne`/`departing`/`clear traffic` triggers fire on direct tower address (commit `d78f5c8`).
- OMAM runway pair order swapped — lower parallel `31L/13R` is now the default over upper `31R/13L` (commit `f83b9d7`).

## Older context (2026-04-25)
- ATIS interval changed 120s → 45s in both call sites; rebuilt `atc.exe`.
- `README_TRAINING1.md` written with transfer prep + operator commands.
- Pre-transfer cleanup recommended: delete the three `run_alain/dhafra/minhad.bat` (broken `--grpc-addr` flag), clear stale `atis_cache/` and `logs/`.

## Build
```bat
build.bat
```
Produces `atc.exe` and `launcher.exe`. Requires Go 1.26+ and SkyEye cloned at `C:\Skyeye`.
