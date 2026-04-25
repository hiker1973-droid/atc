# SkyeyeATC — Claude Context

AI-powered Tower ATC for DCS World, owned by **vSFG-7**. Layered on SkyEye infrastructure: SRS for radio bridging, OpenAI Whisper for STT, OpenAI TTS for voice, Tacview real-time telemetry for aircraft positions. Three airfields plus carrier ops and a few special channels.

## Where you are
This is **Training 1** — the production rig where big missions run. SkyeyeATC was developed on a separate dev box (Windows Server 2022, vsfg7-atc) and ported here on **2026-04-25**. Treat this as prod: be cautious with destructive operations during live training, no exploratory refactors mid-session.

## Network (Training 1, differs from dev)
- SRS: `localhost:5008` (dev was `192.168.1.221:5004`)
- DCS-gRPC: `localhost:50051`
- Tacview real-time telemetry: `localhost:42676`
- OpenAI key: read from `OPENAI_API_KEY` env var. Set globally with `setx OPENAI_API_KEY "..."` — do not re-add it to individual `.bat` files.

## Repo layout
- `cmd/atc/` — main `atc.exe`, runs in different modes via flags (tower / `--atis-only` / `--command-only` / `--marshal-only` / `--scudwatch-only` / `--deckboss-freq`)
- `cmd/launcher/` — `launcher.exe`, web dashboard at `:7000`
- `cmd/logtail/` — log tail/filter CLI
- `pkg/airfield/` — OMDM, OMAM, OMAL definitions (runways, freqs, elev)
- `pkg/state/`, `pkg/controller/`, `pkg/composer/` — state machine, intent + conflict detection, ICAO phraseology
- `configs/{alain,dhafra,minhad}.yaml` — per-airfield configs (already point at `localhost:5008`)

## Sites and roles
Three SRS-bridged tower instances, each writing JSONL to `C:/SkyeyeATC/logs/`:

| Log file | Site | Tower freq | Extra roles |
|---|---|---|---|
| `atc-omal.log` | OMAL / Al Ain | 250.7 | — |
| `atc-omam.log` | OMAM / Al Dhafra | 251.1 | — |
| `atc-omdm.log` | OMDM / Al Minhad | 250.1 | Deckboss (306.2), Marshal (306.3) |

Role names in logs: `Tower`, `Deckboss`, `Marshal` (single L). User's verbal "Marshall" = the `Marshal` role. When the user says "monitor towers / deckboss / marshal", tail all three log files filtering on `heard`, `TX`, role lifecycle (`online`/`offline`/`stack online`/`registered on SRS`), and all warn/error levels. Prefix each event with the site code so the user can tell which instance produced it.

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
4. `start_command and deckboss.bat` — Command channel (282.0) + Deckboss (306.2)
5. `start_marshal.bat` — Marshal stack (306.3)
6. `run_scudwatch.bat` — optional, Scudwatch / Darkstar-1-1 threat broadcaster (264.0)

Stop: close the console window, or use launcher dashboard buttons.

## Gotchas
- **`--grpc-addr` is not a valid flag.** It was removed from `atc.exe`. Old `run_alain.bat` / `run_dhafra.bat` / `run_minhad.bat` files still pass it and will fail at launch — delete them if they are still present.
- **Don't hardcode `C:\SkyeyeATC`** in new Go code. Use `os.Executable()` / `filepath.Dir(argv[0])` so the launcher works from any directory.
- **Don't re-duplicate `OPENAI_API_KEY`** across more `.bat` files. It's already in too many; centralize via `setx`.
- **Build with portability in mind** — Training 1 will not be the last rig.

## How the user works
- Tests roles one at a time and asks Claude to tail the relevant log(s) for the session. They want filtered, site-prefixed events, not raw JSONL.
- Production rig — prefer stable behavior and minimum-blast-radius changes over experimental refactors.
- `/ultrareview` is user-triggered for branch / PR reviews; Claude doesn't launch it.

## Recent context (2026-04-25)
- ATIS interval changed 120s → 45s in both call sites; rebuilt `atc.exe`.
- `README_TRAINING1.md` written with transfer prep + operator commands.
- Pre-transfer cleanup recommended: delete the three `run_alain/dhafra/minhad.bat` (broken `--grpc-addr` flag), clear stale `atis_cache/` and `logs/`.

## Build
```bat
build.bat
```
Produces `atc.exe` and `launcher.exe`. Requires Go 1.26+ and SkyEye cloned at `C:\Skyeye`.
