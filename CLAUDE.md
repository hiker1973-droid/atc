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

Voices: all roles use `onyx` except Deckboss which uses `ash` (calm authoritative) for audible differentiation. Switchable via `--deckboss-voice <echo|ballad|sage|onyx|…>` without rebuilds.

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

## Recent context (2026-05-27)
- **v1.1.1 cut and pushed** at `f5e0ec4` — controller hygiene patch covering the 2026-05-26 OMDM post-mortem, plus the deckboss STT fix. Commits in this release:
  - `b401f3f` — Deckboss address-led guard hardened. Whisper renders "Deck boss" as one word often enough ("TechBoss", "DuckBoss") that the spaced-only `addrPrefixes` list in `cmd/atc/deckboss.go:98` silently dropped legitimate §1/§2/§6 calls as self-echo — pilot would call comm-check and Deckboss would never respond. Added no-space variants (`techboss`, `duckboss`, `decboss`, `checkboss`). **General rule:** when adding a Whisper-defensive prefix, add both spaced and no-space forms.
  - `f161c01` — callsign extractor strips leading filler ("this is"/"i am") and trailing connectors (is/was/now) after the trigger cut. Added triggers: `declaring`, `in-flight`, `clear of`, `is clear`. Fixes the "Venom 2-0-0 is, Al Minhad Tower, ..." TX bleed that recurred in three different shapes yesterday.
  - `f161c01` — go-around debounce: 60s per-callsign cooldown via `goAroundLastTx` map on `ATCController`, plus reapply `fresh.GoAroundWarned=true` after `Remove`/`GetOrCreate` re-enqueue. Root cause of the 6×-in-70s storm was the per-aircraft flag silently zero-resetting on the new struct.
  - `73f0bd3` — `findCarrierContact` logs at info only on chosen-callsign **transition** (with `prev` field), debug otherwise. Replaces the uncommitted Debug→Info diagnostic that was sitting in the working tree since the CVN-naming work in `7574d1b`.
  - `f770223` — Whisper min-frame floor bumped from `> 3` (≈60ms) to `> 9` (≈200ms) across all four srsLoops (main / command / marshal / deckboss). Driven by the 22 hallucinations dropped in yesterday's OMDM window. **Tuning knob** — if pilots report rushed short calls being dropped, dial back to `> 6`. Not yet validated live.
  - `f5e0ec4` — `dev_only/marshal_smoke.md` — manual §1-§6 Case 1 Recovery checklist for exercising `findCarrierContact` after CVN-naming changes.
- **Versioning** — v1.1.1 is a **patch** (hygiene/safety), not the v1.2.0 line. Next minor still scoped for Deckboss-live + DME-style feature work.
- **Restart procedure validated** — full rebuild + role-restart cycle (ATIS → Towers → Marshal → Command → Deckboss). When `build.bat` fails with "atc.exe being used by another process", kill all `atc.exe` PIDs first. Launcher is `launcher.exe` (separate binary, separate process) — `build.bat` rebuilds both, so kill it too if a launcher is running.
- **Duplicate Tower processes** observed pre-restart (two each of OMAL/OMAM/OMDM, same dashboard ports). Cause unknown — likely a `start_towers.bat` re-run without first stopping the previous set. Only one of each pair can bind its dashboard port; the other is a zombie. Resolved by killing all six and re-running `start_towers.bat` once.
- **Departure queue improvements landed** post-v1.1.1: `a2bf187` angels 3-6 (was 5-7), `8eb1782` 60s `DepartureSpacingSec`, `cce8296` queue-position suffix on hold responses, `2ad751a` Tacview hold-short position gate behind `--position-check`. Worth verifying live after the next mission cycle.

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
