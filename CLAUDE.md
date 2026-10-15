# SkyeyeATC — Claude Context

AI-powered Tower ATC for DCS World, owned by **vSFG-7**. Layered on SkyEye infrastructure: SRS for radio bridging, OpenAI Whisper for STT, OpenAI TTS for voice, Tacview real-time telemetry for aircraft positions. Three airfields plus carrier ops and a few special channels.

## Where you are
This is **Training 1** — the production rig where big missions run: hostname **DCS3**, **192.168.1.220** (the "Training" VM in `C:\Skyeye\SKYEYE-VM-SETUP.md`). **As of 2026-09-14 it runs the Caucasus Training mission** (`vSFG-7_Training_Mission_CA V1.16…`, `--map caucasus` roles, SRS on **:5008**), not Syria or PG. The Syria notes further down (SRS :5002, eight Syria fields) do **not** describe this box — no Syria log exists here; per the setup runbook, Syria runs on the separate **Foothold VM (192.168.1.222)**. Run `hostname` before trusting a theatre- or port-specific line in this file — **but the Foothold VM (.222) also reports `DCS3`**, so check the IP and the running `atc.exe` command lines too (see Recent context 2026-10-15). A third rig, **vSFG7-Training3 / 192.168.1.221**, also runs the Caucasus set but on SRS **:5004**; its deploys are recorded in their own section. SkyeyeATC was developed on a separate dev box (Windows Server 2022, vsfg7-atc) and ported here on **2026-04-25**. Treat this as prod: be cautious with destructive operations during live training, no exploratory refactors mid-session.

## Network (env vars, v1.1.0+)
All `.bat` scripts read four env vars and fail fast if any is missing. Same scripts work on dev rig and Training 1 — only the values differ.

| Env var | Training 1 | Dev rig | Read by |
|---|---|---|---|
| `OPENAI_API_KEY` | sk-proj-... | sk-proj-... | `atc.exe` directly (Whisper STT + TTS) |
| `SRS_EAM` | (rotate before mission) | (same) | All `start_*.bat`, passed as `--eam-password` |
| `SKYEYE_SRS` | `localhost:5008` | (unverified) | All `start_*.bat`, passed as `--srs-addr`. **The port differs by VM — check `netstat` on the box, not this doc.** On DCS3 (Training, .220) `SRS-Server.exe` listens on **:5008** (`SRS.cfg` `SERVER_PORT=5008`, `netstat` and the user env var all verified 2026-09-14); DCSServerBot's SRS extension launches it. The 2026-09-02 note calling 5008 "wrong" and 5002 correct was written against the Syria / Foothold VM. |
| `SKYEYE_TACVIEW` | `localhost:42676` | `localhost:42676` | Tower / Marshal / Scudwatch / Launcher, passed as `--tacview-addr` |
| `SKYEYE_MIZ` | (set per mission) | (same) | All `start_*.bat`, passed as `--miz-path` for the boot weather seed. **Optional** — unset = fall back to newest `.miz` in `--miz-dir`. |

Set with `setx VAR value`, then open a new cmd so the new values are in scope (setx doesn't affect the running shell). Symptom of a missing var: each script aborts at the top with `ERROR: <VAR> env var not set` (except `SKYEYE_MIZ`, which is optional and silently falls back).

**Weather seeding (`SKYEYE_MIZ`).** Every `atc.exe` role seeds boot weather with priority `--miz-path` → newest `.miz` in `--miz-dir` (default `C:\Users\Administrator\Saved Games\DCS.dcs_serverrelease\Missions`) → `--static-*` flags (`cmd/atc/main.go:356`). Because "newest `.miz`" picks whatever was saved last on disk — **not** the running mission — it silently grabbed the wrong file (e.g. `IronHammer_v0_46` outranked the older Training `.miz`s, so every role broadcast Iron Hammer weather, 2026-07-01). Set `SKYEYE_MIZ` to the exact training `.miz` full path (spaces/parens OK — scripts quote it) to pin the source deterministically. Rotate it per mission like `SRS_EAM`. This is only the *boot* seed.

**Live weather since 2026-09-19 (`--miz-watch-file`).** Install `tools/dcs-hooks/vsfg7-current-mission.lua` in the DCS server's `Saved Games\DCS.dcs_serverrelease\Scripts\Hooks\` (restart the server once). On every mission load it writes the loaded `.miz` path to `C:\SkyeyeATC\current_mission.txt`; every `atc.exe` role polls that file every 30 s and, when it names a different mission, applies that mission's weather live (`cmd/atc/mizwatch.go`, log line "Weather reloaded — DCS server loaded a different mission"). ATIS follows its tower and regenerates. The launcher's `/api/miz-weather` prefers the same file. This is what makes a weather **rotation** work without re-pinning `SKYEYE_MIZ`; `SKYEYE_MIZ` remains the boot fallback. No file = boot weather, as before.

**Installed and validated on DCS3 (Training) 2026-09-19.** Hook copied into `Saved Games\DCS.dcs_serverrelease\Scripts\Hooks\`; DCS was closed gracefully and **DCSServerBot restarted it on its own** (the scheduler's `00-24: YYYYYYY` means it brings the server back within a minute — don't start `DCS_server.exe` by hand). Validated on the 15:45 rotation: hook wrote the new `.miz` at 15:46:21 and all 7 role processes logged `Weather reloaded` within 30s (096°/8kt → 106°/17kt, altimeter 29.96 → 30.16), all four ATIS stations regenerated with `weatherChanged=true` and stepped ident Alpha → Bravo. The ~30s of `SRS TCP failed` / `Tacview telemetry offline` warns at 15:45 are the rotation blip, not the hook — every role re-registered by 15:46:01 unaided.

**`current_mission.txt` is untracked and must stay that way** — it is per-rig runtime state, rewritten on every mission load.

**MizEdit does not touch weather (checked 2026-09-20).** DCSServerBot's MizEdit extension rewrites each `.miz` on load, so mission-file mtimes change constantly — that is *sounds only*: `config/presets.yaml` defines a single `Sounds` preset (5 `.ogg` files into `l10n/DEFAULT` for the scheduler's restart warnings) and `nodes.yaml` `settings:` lists only `Sounds`. **RealWeather — the extension that *would* inject live real-world weather — is `enabled: false`** on this instance; `realweather.exe` has never run and never appears in `dcssb-DCS3.log`. So the `.miz` on disk is authoritative for weather and our parse of it is what pilots should hear. Cross-check that reconciles the two: DCSSB's `getWeatherInfo` reported ground wind `speed 4 dir 276`, our roles read `windKts 7.775 windDirTrue 96` — 4 m/s = 7.78 kt and DCS stores wind direction 180° off (blowing-to vs from), so the values agree exactly. **If weather ever looks injected, re-check `RealWeather.enabled` before suspecting `mizwatch.go`.**

**Caucasus carrier freqs — "AI MARSHALL" on the card means US (2026-09-20 ruling).** SkyeyeATC's Marshal is **306.300** = Hornet COMM1 CH3 "CVN-72 AI MARSHALL"; **306.200** (CH2, "CVN-72 MARSHAL") is kept clear for a **live human** Marshal, so we never transmit over a squadron member controlling by voice. Command and Deckboss use `--handoff-marshal-freq 306.3`; Deckboss stays 128.600. This freq has flipped four times (306.1 → 306.3 → 306.2 → 306.3) because the card's "AI" label was read as *the DCS mission's* AI rather than *the robot controller, i.e. us*. Operator's words: "306.3 is our SkyeyeATC Marshall, 306.2 is when we have a live human Marshall." **Do not move it back to 306.200.** See `CAUCASUS_PLAN.md` §4.

DCS-gRPC is not externalized — `:50051` is stable across boxes.

## Repo layout
- `cmd/atc/` — main `atc.exe`, runs in different modes via flags (tower / `--atis-only` / `--command-only` / `--marshal-only` / `--scudwatch-only` / `--deckboss-freq`)
- `cmd/launcher/` — `launcher.exe`, web dashboard at `:7000`
- `cmd/logtail/` — log tail/filter CLI
- `pkg/airfield/` — OMDM, OMAM, OMAL definitions (runways, freqs, elev)
- `pkg/state/`, `pkg/controller/`, `pkg/composer/` — state machine, intent + conflict detection, ICAO phraseology
- `configs/{alain,dhafra,minhad}.yaml` — per-airfield configs (hardcode `localhost:5008`, which matches the live SRS server — verified 2026-06-26: `SRS-Server.exe` listens on `:5008` and `SKYEYE_SRS=localhost:5008`. These YAMLs are not currently read by `atc.exe` anyway.)

## Sites and roles
**This section describes the PG (Persian Gulf) set.** On DCS3 the live set as of 2026-09-14 is **Caucasus** (`atc-ugsb.log`, `atc-ug5x.log`, `atc-ugks.log`, `atc-ugko.log`, present since 2026-09-01). Syria — eight fields with their own frequencies and log slugs (`atc-ltag.log`, `atc-llrd.log`, `atc-ojmf.log`, `atc-ltda.log`, `atc-ltaj.log`, `atc-lcra.log`, `atc-lcph.log`, `atc-ojhr.log`) — runs on the Foothold VM; see **Syria (Eastern Med)** below. One map runs at a time per rig.

Three SRS-bridged tower instances, each writing JSONL to `C:/SkyeyeATC/logs/`:

| Log file | Site | Tower freq | Extra roles in this log |
|---|---|---|---|
| `atc-omal.log` | OMAL / Al Ain | 250.7 | — |
| `atc-omam.log` | OMAM / Al Dhafra | 251.1 | — |
| `atc-omdm.log` | OMDM / Al Minhad | 250.1 | **Deckboss** (128.6, "Deckboss", `shimmer`) — interleaved |
| `atc-marshal.log` | — | — | Marshal only (306.3, "Union Marshal", `onyx`) |
| `atc-command.log` | — | — | Command only (282.0, "vSFG-7-Command", `sage`) |
| `atc-atis.log` | — | — | All 5 ATIS stations |

Log slug is set in `cmd/atc/main.go:231` — `--marshal-only` / `--command-only` / `--atis-only` get their own file; otherwise the log goes to `atc-<airfield>.log`. Deckboss has no `--deckboss-only` flag (it runs as `--airfield OMDM --deckboss-freq 128.6`), so its events mix into `atc-omdm.log` alongside Minhad Tower — filter on `Deckboss`/`128.6` when monitoring.

Role names in logs: `Tower`, `Deckboss`, `Marshal` (single L). User's verbal "Marshall" = the `Marshal` role. When the user says "monitor towers / deckboss / marshal", prefer **`tools/tail-all.ps1`** (canonical, site-prefixed, filters on `heard`/`TX`/role lifecycle/warn/error). Older `tmp_tail*.ps1` files at repo root are throwaway scaffolding from prior sessions — superseded.

Common fault signatures: `level=warn` + `SRS disconnected` · `level=error` + `ExternalAudio file error` · `TTS synthesis failed` · `Whisper returned empty transcription` · `SRS TCP failed`.

## ATIS broadcast cadence
**Per station, keyed off whether it broadcasts a second language** (2026-09-01):
- **45s** — English-only stations (`atisIntervalSec`)
- **90s** — bilingual stations (`atisIntervalBilingualSec`), which read the whole report twice

One source of truth: `atisBroadcastIntervalSec(station, map)` in `cmd/atc/atis.go`, which asks the same `atisSecondLang` resolver that decides the second language. Both loops call it — `atisOnlyLoop` (`--atis-only` / `start_atis.bat`) and the tower-bundled path (`--atis-broadcast`) — so **the old "two call sites, keep them in sync" hazard is gone**; don't reintroduce a literal. A new bilingual field picks up 90s automatically, and a station whose `Lang` is `"English"` (Akrotiri) keeps 45s even on a bilingual map.

Why: bilingual audio measured with ffprobe over the cache (2026-09-01) runs **Al Dhafra 71.5s, Minhad 67.7s, Hatay 65.2s against the 45s tick** — note the worst offenders are **PG** fields, so PG has been over the tick since bilingual ATIS landed; Syria just made it visible. When the audio outruns the tick the station stops idling entirely. `broadcast()` is called synchronously from the ticker loop and `time.Ticker`'s channel is buffered depth 1, so a tick firing mid-broadcast waits in that buffer and fires the instant the transmission returns — **back-to-back TX, ~100% duty cycle**, not the 45s the number implied. `TestBilingualIntervalClearsObservedAudio` pins 90s above that 63s measurement.

**Do not diagnose this from the `broadcasting` `TryLock` warning.** That mutex guards concurrent *entry*, which a single synchronous caller per station cannot cause — `ATIS broadcast already in progress — skipping` has **never** appeared in this deployment. An over-running ATIS is silent in the logs; measure the gap between transmissions instead.

Startup staggered `15 + i*37` seconds to desync TTS calls. TTS only regenerates when weather/runway changes; otherwise it rebroadcasts from cache. **Every station caches as `atis_cache/atis_<ICAO>_bilingual.mp3`** — the `_bilingual` suffix is unconditional (`atis.go` `cachePath`), so an English-only station like Akrotiri still uses it. Don't read the filename as evidence a station is bilingual; check its duration or its `Lang` field (LCRA's file is 23.6s, i.e. one English pass). PG's 5 stations: Dhafra (248.2), Minhad (248.3), Liwa (248.55), Al Ain (248.85), Kish (248.5).

## Operator workflow
Start scripts in `C:\SkyeyeATC\`. Recommended order:
1. `start_launcher.bat` — opens dashboard at http://localhost:7000/
2. `start_atis.bat` — all 5 ATIS stations
3. `start_towers.bat` — Minhad / Dhafra / Al Ain (dashboards on 6001 / 6002 / 6003)
4. `start_marshal.bat` — Marshal carrier stack on 306.3 (when carrier ops in mission)

Stop: close the console window, `stop_marshal.bat` to kill only Marshal, or use launcher dashboard buttons.

### Training 1 active roles (2026-05-19)
**Tower**, **ATIS**, **Marshal** (306.3), **Command** (282.0), and **Deckboss** (128.6 — DCS carrier UHF) all run live on Training 1. Bat files at repo root: `start_atis.bat`, `start_towers.bat`, `start_marshal.bat`, `start_command.bat`, `start_deckboss.bat`, `start_launcher.bat`. Post-reboot one-shot: **`start_all.bat`** fires ATIS → towers → marshal → command → deckboss → launcher in order.

Voices are differentiated per role for audible separation on cockpit comms — all female / female-leaning. Set via `--tts-voice` in `start_towers.bat` (per-tower override) and per-role voice flags elsewhere: OMDM Tower `shimmer`, OMAM Tower `shimmer`, OMAL Tower `alloy`, Marshal `coral` (`start_marshal.bat` `--marshal-voice`), Command `sage` (`start_command.bat` `--command-voice`), Deckboss `shimmer` (`start_deckboss.bat` `--deckboss-voice`). Reassigned 2026-05-29 from prior all-nova-towers; OMDM went `nova` → `shimmer` 2026-08-13 (`adac495`). **Superseded for Command and the accents 2026-10-14 (`5e97223`):** Command is now `onyx`, a stern English man, in every `start_command*.bat`; towers and ATIS speak British or Australian, with the accent fixed per field by ICAO hash in `cmd/atc/voice_accents.go` and the gender taken from the voice the bat already gives it. Marshal and Deckboss are unchanged. See Recent context (2026-10-14).

**Speech rate.** **Every role runs at 1.20 as of 2026-09-25** (operator: "speed atc voice up to 1.20") — 1.15 from 2026-09-19, 1.10 from `adac495`, before that a hardcoded 1.05. `--tts-speed` defaults to `speedDeckboss` and no `start_*.bat` sets the flag, so that constant is the live rate for **every tower in all theatres plus Command**. Marshal and ATIS have their own constants (`speedMarshal` / `speedATIS`, once 1.00 and 0.97) but have been raised in step at every bump, so all three sit at 1.20 — they stay separate names only so one role can be pulled back alone. Override per-process with `--tts-speed` if one field needs to differ. ATIS re-synthesizes on its first broadcast after restart, so a rate change needs a rebuild + role restart, not a cache clear — and because the TTS cache keys on the voice *profile* (voice + speed + instructions), a rate change invalidates every cached line, so expect re-synthesis latency on the first calls after a restart. **1.20 deployed 2026-09-25 10:44. Two rigs, and the record is per-rig — read the hostname line.** On **DCS3 / 192.168.1.220 (Training 1, Caucasus)**, verified from this box at the time: rebuilt both binaries, 8 `atc.exe` roles plus the launcher — ATIS, the 4 Caucasus towers (UGSB/UG5X/UGKS/UGKO), Command 282.0, Marshal 306.3, Deckboss 128.6 — all re-registered on SRS **:5008**, and `mizwatch` picked up the loaded `vSFG-7_Training_Mission_CA V1.19` within seconds while `SKYEYE_MIZ` stayed pinned to `V1.18` (expected — the pin is only the boot seed). All four ATIS stations re-synthesized at the new rate by 10:49:22. Evidence: `Get-CimInstance Win32_Process` showed every role on `--srs-addr localhost:5008`; `Get-NetTCPConnection` shows **:5008 listening and :5002 not listening at all**; `atc-ug{sb,5x,ks,ko}.log` are being appended to live, and **no Syria log exists on this box** (`atc-ltag/llrd/lcra/oslk/olba.log` all absent). A **separate 2026-09-25 11:00 edit recorded the same deploy as Syria / 9 roles / SRS :5002 / `Foothold_SY_4.9.0_vSFG7_v1.10_08`** and asserted that Caucasus has never run here, that its logs stop 2026-09-08, and that no V1.18/V1.19 mission exists. **None of that is true of DCS3**, where the V1.18 `.miz` is on disk and the Caucasus roles were running throughout. Those are almost certainly real observations from the **Foothold VM (192.168.1.222)**, which is exactly where CLAUDE.md already says Syria and :5002 live — not verifiable from here, so they are kept as *that* rig's record, not deleted. **The lesson is the one at the top of this file: `hostname` first.** Both sessions wrote "checked against the running processes" and both were right about their own box; the shared doc is what makes a rig-specific observation read as a global fact. Scope every deploy record to a hostname. See `voice_tuning.md`.

**OMDM sets both voice slots.** `start_towers.bat` passes `--tts-voice shimmer --tts-voice-male shimmer` for Minhad. Belt and braces: `--voice-rotate-hours` (the female/male swap) defaults to 0 = off since 2026-10-14, so every tower keeps its script `--tts-voice` all day. It was 4h until then and turned Senaki (alloy -> ElevenLabs Lisa) into onyx -> George every other 4h slot. With rotation on, the male slot at `onyx` would flip Minhad off shimmer half the day. Deliberate, not a copy-paste error.

**The all-unique rule no longer holds** — Deckboss went `ash` → `shimmer` on 2026-08-06 (the operator wanted it female; `ash` is male, and this line previously claimed `fable`, which the bat file had never used). OpenAI has only four clearly female voices — `nova`, `shimmer`, `coral`, `sage` — and all four were already taken, so Deckboss had to double up. `shimmer` was the safest share: its other holder is a land field on 251.1 that a carrier pilot on 128.6 won't have tuned. `nova` was ruled out because OMDM Tower runs in the *same process* as Deckboss, `coral` because Marshal is the other carrier voice, `sage` because everyone monitors Command. If a fifth female voice ever ships, give it to Deckboss and restore the rule. **Superseded in part 2026-08-13** — OMDM Tower also moved to `shimmer`, so it is now shared three ways (OMDM Tower 250.1, OMAM Tower 251.1, Deckboss 128.6). The "`nova` was ruled out because OMDM Tower is the same process" reasoning no longer applies; the operator chose the shared voice on purpose, so a carrier pilot working the beach hears one voice on both. Switchable in the bat files without rebuilds; the code defaults in `cmd/atc/main.go` and `cmd/atc/deckboss.go` were moved to `shimmer` to match.

Still parked under `dev_only/` (do not launch on Training 1):
- `dev_only/run_scudwatch.bat` — Scudwatch threat broadcaster 264.0

Pending plans / decisions captured under `dev_only/` (not executable, just notes):
- `dev_only/web_access_plan.md` — remote browser access to the launcher dashboard. Options A/B/C/D discussed 2026-05-29, deferred ("we will start in a while"). Read this before resuming that work.

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

## Git workflow

Remote is `github.com/hiker1973-droid/atc`, default branch `main`.

- **The operator works directly on `main`.** Asked to commit, land it on `main` and
  push unless they say otherwise. Working on a side branch and stopping there just
  costs a round trip — on 2026-09-02 a `fix/...` branch had to be merged and pushed
  in a follow-up because that was the intent all along. Branch only for genuinely
  speculative work, and say so.
- **`main` should match what is running on the rig.** Deploys build from the working
  tree (`go build -o atc.exe ./cmd/atc`), so uncommitted edits mean the live binary
  has no commit behind it. After a deploy, commit — do not leave the rig running
  code that is not in git.
- **Use `git commit -F <file>`, not `-m` with a here-string.** Multi-line messages
  through PowerShell trip `'/' is outside repository`. Write the message to the
  scratchpad and pass `-F`.
- **`LF will be replaced by CRLF` warnings are noise.** The repo is CRLF; tooling
  writes LF and git normalizes. It does not mean the file is mangled — check
  `git diff --stat` and you will see only the real lines changed. Related: `gofmt -l`
  lists most of `cmd/atc/` for the same reason, so a file appearing there is not
  evidence your edit is unformatted. Check `gofmt -d <file>` for your own hunks.
- **Ignored, so never commit them:** `atc.exe`, `launcher.exe`, `atis_cache/`.
  Rollback copies (`atc.rollback.exe`, `launcher.rollback.exe`) and scratch backups
  (`*.bat.bak`, `atis_cache_old_44k/`) are untracked on purpose — leave them alone
  rather than adding or deleting on your own initiative.
- **Split commits by concern.** A portable bug fix and theatre-specific operator
  config belong in separate commits so the config can be reverted without losing the
  fix — e.g. `d231884` (GUID fix, every theatre) vs `a8a0986` (Syria frequencies).
- Commit messages here carry the reasoning, not just the change: what the symptom
  looked like, what was ruled out, and what is still open. Several of this repo's
  hardest bugs were invisible in the diff and only findable from the message.

## Syria (Eastern Med) — Foothold VM, live there as of 2026-09-02

Foothold Syria runs on the **Foothold VM (192.168.1.222)**, not on DCS3 — no Syria log exists on DCS3. One map at a time, so the PG scripts are the wrong ones there: `start_all.bat` would put Marshal on 306.300 and Deckboss on PG's 128.600. Use `start_region_syria.bat` (ATIS → towers → Command → Marshal → Deckboss); it does **not** start the launcher.

**The squadron card is the authority on frequencies** — `00_2_vSFG7_Hornet_Presets_SY_Foothold.jpg` at the repo root (untracked; supplied by the operator 2026-09-02). Every frequency in the code was checked against it and matched. Do not "fix" a frequency without checking the card.

| Role | Freq | Card ref |
|---|---|---|
| Incirlik LTAG tower / ATIS | 360.100 / 360.200 | COMM 1 CH 8 / 9 |
| Ramat David LLRD tower / ATIS | 251.300 / 256.150 | CH 10 / 11 |
| King Hussein OJMF tower / ATIS | 250.450 / 255.550 | CH 12 / 13 |
| Hatay LTDA tower | 250.300 | CH 14 |
| Gaziantep LTAJ tower | 250.100 | CH 15 |
| H4 OJHR tower / ATIS | 252.250 / 240.850 | CH 16 / 17 |
| Akrotiri LCRA tower | 252.000 | CH 18 |
| Paphos LCPH tower / ATIS | 249.100 / 249.000 | CH 19 / 20 |
| Marshal | 306.300 (stern female voice, since 2026-09-19; was 306.100 CH 2) | COMM 1 CH 3 "CVN-72 AI MARSHALL" |
| Command | 282.000 | COMM 1 CH 4 |
| **Deckboss** | **128.600** | **deviates — card says COMM 2 CH 1 = 306.200** |

Dashboards 6041–6048 for the towers, 6004 Marshal, 6005 Deckboss.

### ROSTER CUT TO FIVE FIELDS (2026-09-19, operator ruling)

**Only five Syria fields now run ATC and ATIS.** The table above still lists
every field the card covers — it is the frequency reference, not the roster.

| Field | Role | Tower | ATIS | Dashboard |
|---|---|---|---|---|
| Akrotiri LCRA | primary | 252.000 | 249.500 | 6046 |
| Incirlik LTAG | primary | 360.100 | 360.200 | 6041 |
| Ramat David LLRD | primary | 251.300 | 256.150 | 6042 |
| **Bassel Al-Assad OSLK** | divert | **250.600** | **249.600** | **6049** |
| **Beirut OLBA** | divert | **250.650** | **249.700** | **6050** |

**King Hussein (OJMF), Hatay (LTDA), Gaziantep (LTAJ), Paphos (LCPH) and H4
(OJHR) are PARKED, not deleted.** Their airfield definitions and ATIS stations
are still in the code. Restoring one is three edits: uncomment its line at the
bottom of `start_towers_syria.bat`, add its ICAO to `--atis-stations` in
`start_atis_syria.bat`, and add it to the `Syria` list in
`pkg/airfield/registry.go`. That list holds only the five live fields because
Command's handoff scan walks it -- a parked field there gets pilots told to call a
tower nobody is running.

**`--atis-stations` is the ATIS selector** (new, `587609d`) — a comma-separated
ICAO list narrowing `--atis-only`. Empty means the whole theatre set, so PG /
Caucasus / Germany / Iraq are unchanged. It does **not** fail open: an unknown
ICAO warns, and a filter matching nothing is fatal rather than quietly putting
all eight Syria stations back on the air.

**The two divert fields are on the Foothold v1.10 Hornet presets** (COMM1 CH14-17;
the old eight-field card never had them). Bassel's 250.600 is the DCS terrain
tower. **Beirut is 250.650, not the terrain's 253.200** — 253.200 is the SHELL 2
tanker (operator ruling 2026-09-18). ATIS 249.600 / 249.700 are assigned by us.

Navaids, per DimOn Aerodrome Data (01 Feb 2026): Bassel Al-Assad has parallel
runways 17L/35R and 17R/35L with ILS 109.10 (IBA) on **17R**, VOR/DME LTK 114.80,
NDB 414. Beirut has VOR/DME KAD 112.60, NDB 351 and three ILS (16 110.10, 17 109.50,
03 110.70). Positions and runways come from DimOn; thresholds are still computed, so
verify before enabling `--position-check`.

**Deckboss deviation.** The card puts Deckboss on 306.200, and 128.600 on COMM 1 CH 1 = CVN-72 **AI**, the mission's own carrier controller. Operator ruled 128.600 anyway on 2026-09-02, so we transmit on top of the DCS AI and a pilot using COMM 2 CH 1 as briefed will not hear us. Called out in `start_deckboss_syria.bat`. **If Deckboss is ever reported silent, start here.** Cautionary tale: pilots seen tuned to `128.600 + 306.200` are on COMM 1 CH 1 + COMM 2 CH 1 — i.e. correctly tuned to Deckboss on **306.200**. That was misread once as evidence they wanted Deckboss moved.

Hatay / Gaziantep / Akrotiri ATIS (249.300 / 249.400 / 249.500) are **assigned by us**, not on the card — the card gives those three tower-only, so pilots have no preset for them.

LSO 128.100 is on the card but `atc.exe` has no LSO role; handoff is verbal.

## Kola — Operation WHITE NIGHT (on main since b019258, 2026-10-09; built on DCSServer only, NOT flown)

Theatre `--map kola`, bats `start_*_kola.bat`, ports 6051-6054. Four fields are defined
(ENBO Bodo, ENDU Bardufoss, ENNA Banak, EFRO Rovaniemi) but **only the field in play is
live** — Bodo for mission 1 — by operator ruling ("only the appropriate airfield"). The
four places to change when the campaign moves to the next base, where every number came
from, and what is still unratified (Marshal 306.300 / Deckboss 128.600 by analogy, the
assigned ATIS frequencies nobody has on a preset) are in `KOLA_PLAN.md`.

- ATIS is English + the field's own language: Norwegian in Norway, Finnish at Rovaniemi
  (per-station `Lang`, same mechanism as Syria).
- Magvar is per field here (5°E to 12°E across the map) and is **dated 2015**, the WHITE
  NIGHT year — DCS takes magvar from the mission date, 2026 reads about 3° more (see
  `KOLA_PLAN.md` §2). Runway headings are computed
  from TRUE bearings — the Mission Editor's `course_deg` is a grid bearing, 6-9° off.

## ExternalAudio crashes mid-transmission — read before debugging "SRS not transmitting"

All TX goes out through `DCS-SR-ExternalAudio.exe`. **It has a .NET bug that kills it partway through a transmission:**

```
Playing audio - sent 4000ms - 8%
Process terminated.
A callback was made on a garbage collected delegate of type MMTimerProc::Invoke
exit status 0x80131623
```

The station keys up, sends a few seconds, and dies. Registration, frequency and client list all look perfect — **the only evidence is `level=error` + `ExternalAudio file error` in `atc-*.log`.** On 2026-09-02 there were 827 of these, 798 of them ATIS, dying between 3% and 79% through the file. To the operator it presents as "SRS is connected but nothing ever transmits", on every field at once, and it is easily misdiagnosed as a frequency or SRS-config fault. It cost most of a day.

**It is load-dependent, which is why PG looked fine and Syria did not.** Crashes by date: 6–31/day through August on PG (5 ATIS, 2 long bilingual), then **118 on 2026-09-01** when Syria's 8 stations / 7 bilingual / 44–58s reports went live. Same latent bug, roughly ten times the exposure.

**Mitigation (`6f4afb2`): everything handed to ExternalAudio is now 16kHz mono.** ExternalAudio always converts input to mono 16-bit PCM @ 16kHz before Opus-encoding, so 44100 bought nothing and cost a resample on our longest files. `srsAudioRateHz` in `cmd/atc/main.go` pins **both** ffmpeg encode stages — `concatMP3WithSilence` and `runAudioFilter` (the single radio-effect + mic-click pass since 2026-10-10; it was two, `applyRadioEffect` and `addMicClicks`). The last matters most: it runs last, so whatever it emits is what ExternalAudio actually reads. Crash split at the time was 1586 log lines at 44100Hz against 4 at 24000Hz. **This is a mitigation, not a cure — the bug is upstream, and it did not stop.** An earlier version of this line claimed "zero crashes since"; that was wrong. Counted 2026-09-25, `atc-atis.log` has logged the same `0x80131623` / `MMTimerProc::Invoke` fault **every single day** since the fix, peaking at **875 on 2026-09-09** and settling to **~60-90/day** since the roster cut to five fields on 09-19. The audio really is 16kHz — the crash output itself reads `Read MP3 @ 16000` — so the resample was never the cause.

Akrotiri ATIS stays 24kHz — English-only, never hits the concat, 4 of 827 failures.

**It is overwhelmingly an ATIS problem.** Since 2026-09-02, `atc-atis.log` carries thousands of these against **1** across all five Syria tower logs plus Command and Marshal combined. Long bilingual reports are the exposure; a controller's one-line TX essentially never trips it. So the per-day count is a *baseline*, not an alarm — when ATIS is reported silent or clipped, count the day's `ExternalAudio file error` lines and compare against ~60-90; only a jump is news. Check transmission length first.

## Debugging SRS: what is and is not trustworthy

- **`clients-list.json` is NOT a reliable census.** (`Saved Games\...\Config\`, written because `CLIENT_EXPORT_ENABLED=True`.) Entries appear and vanish between reads, and it has consistently shown only **6 of 8** towers even when all eight log `SRS registered — listening` and hold ESTABLISHED TCP + UDP to `:5002`. Useful for spotting *identity* corruption; never conclude "not registered" from absence.
- **`TRANSMISSION_LOG_ENABLED=true`** was set in `SRS.cfg` on 2026-09-02 for diagnostics (backup `SRS.cfg.bak-2026-09-02`); it writes `<SRS>\Server\<date>-transmissionlog.csv`. It stayed **empty** with no pilots connected — an empty log does **not** prove nothing transmitted.
- **`<SRS>\Server\serverlog.txt`** is the server's own log (~80MB). Constant connect/disconnect churn there is normal — that is ExternalAudio's transient client, one per transmission.
- **Theatre history differs by rig.** DCS3 (Training) has had `atc-ug*.log` files since 2026-09-01 and runs Caucasus live as of 2026-09-14. The older claim that Caucasus "never ran on this rig" was written from the Syria / Foothold VM. Check `C:\SkyeyeATC\logs` on the box you're on before assuming a baseline.
- SRS port is per rig: **:5008** on DCS3, **:5002** per the Syria notes. Check `netstat` / `SKYEYE_SRS` on the box itself.

## OPEN: King Hussein (OJMF) + H4 (OJHR) missing from the SRS client list

Reproducible across three restart patterns including a clean full restart with 4s spacing. Both towers log `SRS registered — listening` on 250.450 / 252.250 and hold ESTABLISHED TCP **and** UDP to `:5002`, but never appear in `clients-list.json`. Ruled out with evidence — do not re-chase these:

- **Not a GUID collision** — PIDs distinct mod 1e6 since `d231884`
- **Not a client cap** — killing Marshal + Command to free slots did not admit them
- **Not a unitId collision** — FNV-32a computed for all 19 role callsigns, all unique
- **Not launch timing or order** — same result with 2s, 3s and 4s staggers

**Untested: whether pilots can actually hear these two fields.** Given the export is unreliable (above), this may be cosmetic. The next step is a listening test on 250.450 / 252.250 against a known-good field like Gaziantep 250.100 — not more local analysis.

## Recent context (2026-10-15) — vSFG7-Training3 / 192.168.1.221 (Caucasus, SRS :5004)
- **A third rig.** `hostname` prints **vSFG7-Training3**, the IP is **192.168.1.221**, SRS listens on **:5004** only (no :5002 or :5008), and Tacview :42676 is up. It runs the same Caucasus set as DCS3: ATIS, the 5 towers UGSB/UG5X/UGKS/UGKO/UGTB (dashboards 6011-6015), Command 282.0 + Hitman 61.0, Marshal 306.3, Deckboss 128.6 (logs to `atc-ugsb.log`), and the launcher. The DCS3 lines in this file (:5008) do not describe it.
- **Deployed `b25ea72` at 07:22** (15 commits from `9ad336d`: calm-wind runway rotation, tower voice rotation off by default, the Raider 302 recognition fixes, Deckboss "Dark/Black boss", Marshal "range N", Foothold tasking (Syria only), Marshal = Spartan, Command / Deckboss = round 11 voices 3 / 5). Vet and tests passed, and each new ElevenLabs voice ID returned 200 on a one-line test before the restart. Restarted the towers, Command, Marshal, Deckboss and the launcher by replaying their running command lines 5 s apart. **ATIS was left running** (no ATIS code changed; a restart makes every station regenerate). Rollback: `atc.rollback-2026-10-15a.exe` / `launcher.rollback-2026-10-15a.exe`.
- **Deployed `402ed9c` at 08:01, towers only** (the 7-mile departure handoff to Command and its own Departure voice, `FF59babHL8N8gfTgtBMT`, curl-tested 200). Nothing in it touches Command, Marshal, Deckboss or ATIS, so only the five towers restarted. All re-registered with no warns. Start-up runways: UGSB 13, UG5X 07, UGKS 27, UGKO 25, UGTB 31L, each logging `light-wind runway rotation on`. Rollback: `atc.rollback-2026-10-15b.exe` (= the 07:22 build).
  - **Mixed builds are expected here:** towers run `402ed9c`; Command, Marshal and Deckboss run the 07:22 build from `atc.rollback-2026-10-15a.exe`; ATIS still runs the pre-07:22 build from the same file. Their behaviour matches main. **Never delete a rollback a running role was started from** — it is locked anyway, and reusing its name fails the rename.
  - Mission: `vSFG-7_Night_Training_Mission_CA V1.23.VFR 4 Broken Cumulus (Fall)`.
  - No pilot traffic since either deploy, so the new voices and the 7-mile call are not yet heard on the air.
- **The 08:37 mission rotation** (to `CA V1.23.VFR 1 Clear Night (Fall)`, wind 106°/4 kt, 29.96) took SRS and Tacview down for ~90 s. Every role re-registered by 08:38:41 and reloaded weather by 08:38:49 with no intervention.
- **Deployed `ff7639e` at 08:44, full restart including ATIS** (`b26e6ad` ExternalAudio mid-play crash fix — GC budget + resend-the-rest — matters most for ATIS; `ff7639e` Senaki's alternate controller `irzJNDCo2SzXrIpo6Yd6` on 2-hour shifts, curl-tested 200, passed as `--elevenlabs-tower-alt-voice` per `start_towers_caucasus.bat`). All five ATIS stations regenerated 08:45-08:47 (three `ATIS TTS attempt failed`, each recovered on retry) and were transmitting by 08:50. ExternalAudio accepted the GC setting (no "dropped for the session" line); no crash or resend in the first 5 min — the real test is the day's count against the 6-14 cut-offs of the morning. Rollback: `*.rollback-2026-10-15c.exe`; ATIS still runs from `atc.rollback-2026-10-15c.exe`.
- **Deployed `ec7a177` at 09:09** (ElevenLabs usage log + launcher credits panel). Restarted the launcher and the 8 ElevenLabs roles; ATIS (OpenAI-only) left running. All re-registered. Rollback: `*.rollback-2026-10-15d.exe`.
  - **The panel's account read returns `HTTP 401`** here: the launcher uses `ELEVENLABS_API_KEY`, and this box still has the original TTS-only key. It needs the new restricted key (User: Read + Workspace analytics: Read, and text-to-speech, since every role reads the same var) set by the operator with `setx`, then the launcher restarted. Until then the panel falls back to SkyEye's own per-rig count.
  - The fleet view's `training1` row (192.168.1.220:7000) was **connection refused** at 09:10; `night` and `foothold` answered.
- **Auto mode refuses a launch that reads keys with `[Environment]::GetEnvironmentVariable(...,'User')`** ("Credential Materialization"). The tool shell already inherits `OPENAI_API_KEY` and `ELEVENLABS_API_KEY`, so launch without re-reading them; check the ElevenLabs one only for its `sk_` prefix.

## Recent context (2026-10-15) — Foothold VM / 192.168.1.222 (Syria, SRS :5002)
- **This box's `hostname` also prints `DCS3`.** The IP (`Get-NetIPAddress`) is **192.168.1.222**, SRS listens on **:5002** only (no :5008), and the running roles are the Syria five. So the "run `hostname` first" rule at the top does not separate the two rigs on its own. **Before a restart, read the running `atc.exe` command lines** (`Get-CimInstance Win32_Process`), `current_mission.txt` and the listening SRS port, then restart whatever theatre is actually running. The 2026-10-14 section below is DCS3's record, not this box's.
- **Deployed `402ed9c` at 08:23** (15 commits from `3e69d53`: the 7-mile departure handoff to Command, the Departure voice, new Command and Deckboss voices, Marshal "range N", callsign trimming, and Deckboss "Dark/Black boss"). Vet and tests passed. The new binaries were built as `.new` first, then all `atc*` and the launcher were stopped, swapped in, and brought back with `start_region_syria.bat` followed 60 s later by `start_launcher.bat`. About 60 s off the air.
  - All 10 processes came back up: ATIS (LTAG,LLRD,LCRA,OSLK,OLBA); the Incirlik 360.1, Ramat David 251.3, Akrotiri 252.0, Bassel 250.6 and Beirut 250.65 towers; Command 282.0; Marshal 306.3; Deckboss 128.6; and the launcher. Every SRS role logged registered by 08:24:02.
  - Rollback copies `atc.rollback.exe` / `launcher.rollback.exe` now hold **`3e69d53`**.
  - Weather: every role switched to the loaded `Foothold_SY_5.2.0_vSFG7_v1.12_11_VFR_Summer_TowerCu_Morning`. `SKYEYE_MIZ` is pinned to `v1.12_15_…CbTowers_Dusk`, which is the boot seed only, so this is expected.
  - Incirlik ATIS logged one `ATIS TTS attempt failed … took=45000` on regeneration. That is the normal restart stall and the retry handles it.
  - **Tacview :42676 is not listening on this box** (also the case all of 2026-10-14), so every tower logs `TACVIEW OFFLINE` with conflict detection and speed warnings off. This was already the case before the deploy; DCS real-time telemetry needs turning on.
  - Not yet heard on the air from this box: the 7-mile handoff, the Departure voice and the new Command/Deckboss voices.
- **Redeployed `ff7639e` at 08:43** (`b26e6ad`: ATIS no longer cut off when ExternalAudio crashes mid-play; `ff7639e`: Senaki shifts, which only touches `start_towers_caucasus.bat` and so changes nothing on this box). Vet and tests passed. Same `.new` build, swap, and `start_region_syria.bat` + `start_launcher.bat` restart as before. All 10 processes were back and every SRS role was registered by 08:43:51. Incirlik and Ramat David ATIS regenerated on the first attempt. **Rollback copies now hold `b54a9a0` (= `402ed9c` code).**
- **New `ELEVENLABS_API_KEY` (`sk_049…`) set via `setx` at ~09:00, then redeployed `ec7a177` at 09:09** (launcher ElevenLabs credits panel, `39f262e` + `ec7a177`). All 10 processes are back and every SRS role registered by 09:09:40, with the new key loaded. A TTS POST with the new key returns 200. **The panel's account half shows `accountErr: HTTP 401 Invalid API key`**: the new key, like the old one, can't read `/v1/user/subscription` (no `user_read`), even though TTS works. A 401 there therefore says nothing about TTS. The per-rig `elevenlabs_usage/*.jsonl` half works without it. Rollback copies hold `ca7bd25` (= `ff7639e` code).
- **Key replaced again at 09:30 with `sk_199…`, which does have `user_read`.** TTS returns 200 and so does `/v1/user/subscription`. All 10 processes were restarted and re-registered by 09:30:58. The launcher panel now reads `source: account` with no `accountErr`. At that point it showed 8,649 of 127,855 credits (6.8%) used this period. The `sk_049…` key is no longer in use.
- **The auto-mode permission classifier refused the stop-and-restart** the first time, even though the request said "pull, rebuild and deploy". It ran after the operator said "go ahead and deploy". Build and test first, so a refusal leaves the rig untouched.

## Recent context (2026-10-14) — DCS3 / 192.168.1.220 (Caucasus, SRS :5008)
- **Deployed `5e97223` + `2e30904` on DCS3.** Pulled five commits: Hitman 1-1 JTAC (`40ef6da`, `47c9898`, `a8fd8ed`), British/Australian voices (`5e97223`), and the Kola magvar fix (`7e8e855`). Vet and tests passed. Full restart via `start_all_caucasus.bat` brought up 10 processes: ATIS, 5 towers (UGSB/UG5X/UGKS/UGKO/**UGTB**), Command, Marshal 306.3, Deckboss 128.6, and the launcher. Rollback copies `atc.rollback.exe` / `launcher.rollback.exe` hold the pre-pull `373f2d1` build. Live mission per `current_mission.txt`: `vSFG-7_Training_Mission_CA V1.22`. `SKYEYE_MIZ` is still pinned to V1.18, which is the boot seed only, so this is expected.
- **Hitman 1-1 runs inside the Command process**, on **61.000 FM** (`--jtac-freq 61.0 --jtac-voice ash`, `start_command_caucasus.bat`). It logs to `atc-command.log` as `JTAC net starting` / `Command channel registered on SRS freq=61 name=Hitman-1-1`. **60.000 is the live human JTAC**, so never move Hitman there. Not yet heard on the air: the FM transmission and BDA are untested with a pilot.
- **Fixed: OpenAI calls had no timeout (`2e30904`).** The voice change made every ATIS station regenerate at once. Kobuleti, then Batumi, logged `ATIS generating new audio` and never wrote the cache or broadcast again. The station sat holding its lock, silent on the air and silent in the log. No ffmpeg child was running, so the stall was in the HTTP call. TTS, translate and both Whisper paths used `http.DefaultClient` with the process-lifetime ctx. Now they go through the shared `openAIClient` with a 60 s timeout. It fired once live: `ATIS English TTS failed … context deadline exceeded (… while reading body)` for Batumi at 09:02:25. **"While reading body" means OpenAI kept streaming audio and never finished, i.e. runaway generation, not a dead connection.** The station retried on its next 90 s tick and succeeded. **Symptom to recognise:** a station whose last line is `generating new audio`, followed by nothing at all. Since this fix it shows up as one `level=error` line, so a single one is benign. Repeated errors for the same station mean that station's TTS request itself is the problem.
- **Still open:** a translate timeout falls back to English-only, and that sticks until the next weather change (same as any translate failure before).
- **Tooling trap:** PowerShell 5.1 `Get-Content -Raw | Set-Content -Encoding utf8` on a Go file reads it as ANSI and writes a BOM. It garbled every `—`/`→`/`·` in `main.go`; the build still passed, and the damage only showed as a 265-line diff for a 6-line change. Edit source with the Edit tool or `sed` in Bash, never a PowerShell round-trip.
- **`cmd /c start_all_caucasus.bat` from a tool call blocks**, because a child window keeps it open. `Start-Process cmd -ArgumentList '/c',<path> -WindowStyle Minimized` returns immediately, **and its `timeout` staggers actually run** (towers came up 5 s apart).
- **ElevenLabs live on DCS3 at 10:42 (`82c684b` + `bd0f529`).** Command = Daniel, Hitman = Charlie, towers = Alice / Lily (Senaki) / George by accent and gender, Marshal = Adam, Caucasus Deckboss = Eric. **ATIS stays on OpenAI.** The softer radio filter applies to every role. Each role's OpenAI voice is the fallback; Marshal and Deckboss now fall back to men (`echo` / `verse`), not the old female voices. Rollback copies were refreshed at this deploy, so they now hold `309e86a`, the OpenAI-only build. **Still unconfirmed on the air** — no pilot traffic since.
- **`ELEVENLABS_API_KEY` is a User-scope env var (`setx`).** It is read once, at process start. A shell opened before the `setx` launches roles *without* it, and they silently run OpenAI. Before relaunching from a tool shell, do `$env:ELEVENLABS_API_KEY=[Environment]::GetEnvironmentVariable('ELEVENLABS_API_KEY','User')`.
- **Key traps.**
  - A real key starts with `sk_`. The first value supplied was the key **ID** (64 hex characters). ElevenLabs answers that with `400 api_key_id_used_as_api_key`.
  - At this deploy the backoff in `tts_elevenlabs.go` covered only 401/402/429, so the bad key never backed off: every TX tried ElevenLabs, failed, and fell back. Since `b454543` a 400/403 whose body names the API key also turns ElevenLabs off for 10 minutes; other 400s (a withdrawn voice, a bad setting) stay per-call.
  - The live key has no `user_read` permission, so `/v1/user/subscription` returns 401 even though TTS works. **To check a key, POST a short `/v1/text-to-speech/<voice>` call**, not the account endpoints.
- **How to tell ElevenLabs is actually being used.** A successful call logs only at **debug** (`ElevenLabs TTS`). Failure is `warn` `ElevenLabs TTS failed - falling back to OpenAI`, or `ElevenLabs off - OpenAI voices until it is retried`. There is no TTS prewarm, so **the first pilot call after a restart is the first ElevenLabs request.** "No warns" proves nothing until something has transmitted. The tower TX line still reads `TX via OpenAI TTS + ExternalAudio voice=<openai voice>` whichever engine spoke (`cmd/atc/main.go:708`, hard-coded label), so judge by the absence of the fallback warn, not that line.
- **Later deploys the same day on DCS3.**
  - **11:19:** `f91df5a` (ATIS: 45 s per attempt, 3 tries, a length check), `84b3293` (Gideon / Scotty / Lisa; Marshal on Flash), `b454543` (a bad key backs off).
    - The ATIS retry fired four times on the first regeneration: Batumi RU, Senaki RU, Kutaisi EN twice, with Kutaisi needing all three tries.
    - All five reports came back at full length, 44-56 s.
    - `ffprobe` is not installed on this box; measure with `C:\ffmpeg-master-latest-win64-gpl\bin\ffmpeg.exe -i <file>`.
  - **11:57:** `6dd6b0a`, the short Hitman brief (operator request): **"target N, <name>, IP <IP>. Read back."** The pilot reads back "target N, <IP>" or "copy". A readback that names Hitman *mid-call* is dropped as our own echo.
  - The 11:19 and 11:57 deploys did not refresh the rollback copies; 12:04 did (below).
  - **12:04:** `310444c`, Deckboss → **Hank Harvey**, landed from another session at 12:00. `start_deckboss_caucasus.bat` no longer passes `--elevenlabs-deckboss-voice`, so the code default is the voice. The OpenAI fallback stays `verse`.
    - All 10 processes are re-registered with no warnings.
    - **Rollback copies were refreshed at this deploy and now hold the 11:57 build** (short Hitman brief, Eric on Deckboss).
    - Hank is not yet heard on the air.
    - The 12:04 restart's ATIS regeneration hit four more OpenAI stalls: Batumi EN, Kutaisi EN, Kobuleti RU twice. All five stations recovered to full length (50-55 s) by 12:07. That makes three restarts today where gpt-4o-mini-tts stalled; treat the `ATIS TTS attempt failed` warns as normal on a restart, and only worry if a station gives up.
  - **13:21:** `9ad336d`, Command → **Spartan** (American baritone, operator: "need american"), with its own steadier `elCommand` settings and an American OpenAI fallback. Hitman is unchanged.
    - All 10 processes are re-registered with no warnings.
    - Rollback copies now hold the 12:04 build (Gideon on Command, Hank on Deckboss).
    - Spartan is not yet heard on the air.
  - **13:42:** `f5a062f` (light-wind runway rotation: below `--runway-calm-kts` 5 kt the runway changes every 4 h slot; slots turn at 02/06/10/14/18/22 local) plus `b5251ff` / `3e69d53` (Foothold save-file tasking, `--map syria` only). Wind was 3.9 kt, so every field rotated: UGSB 31, UG5X 25, UGKS 09, UGKO 07, UGTB 13R at 13:42. Senaki has since shown 27 (14:00), 09 (18:00) and 27 (06:00 next day), all expected.
  - **16:47:** `9c96f2f` (the three held recognition gaps below, closed by another session at 14:19), `62bd959` (tower voice rotation off by default — Senaki had been flipping from Lisa to George every other 4 h slot since 14:00) and `85eabf6` (Marshal, below). Senaki confirmed on `voice=alloy` live at 18:49.
  - **20:22:** Deckboss-only restart with `e31c66c` / `a178aa1` to unstick Raider 308 mid-flight (below). Done by renaming the running `atc.exe` (Windows allows it) and relaunching that one process with its original command line — the other nine kept the 16:47 build. Useful when one role needs a fix and the rest are busy.
  - **2026-10-15 06:46:** `3d20e82` + `754347e`, full restart, all 10 re-registered. **Rollback copies hold the 20:22 build.**
- **First live pilot traffic on the new build (Raider 302, 10:59-11:48).** Senaki, Kobuleti, Tbilisi, Deckboss, Command and Hitman all answered comm checks. Hitman briefed a 9-line on 61.0 FM. No ElevenLabs fallbacks. Three recognition gaps were found and **held by operator ruling ("Hitman brief only")**:
  1. Hitman and Command lack `comp check` / `com check` / `comcheck`. The towers have them. `jtac.go:158`, `command.go:30`.
  2. Tbilisi has no entry in `towerKeywordAliases` (`pkg/controller/controller.go:1148`), so Whisper's "Double-Ace" and "To Velocity" were dropped silently. Only the exact word "tbilisi" matches.
  3. `jtacWhisperPrompt` lacks "tasking", so "request tasking" came back as "request taxiing" and missed.
  - All three fixed in `9c96f2f`, deployed 16:47. Hitman's com-check gap bit again live at 15:59 and 16:09 before the deploy.
- **Short Hitman brief heard live** at 16:11: "target 2, Rioni Rail Bridge, IP Brick. Read back." Raider 302 never read it back, so the readback path is still untested.
- **Evening traffic (Raider 302 15:59-16:39, then 18:49-18:56; Venom 201/203 at Senaki 19:43-21:09; Raider 308 on Deckboss 20:08-20:23) and what it fixed:**
  - **Marshal missed four calls in a row (`85eabf6`).** Its own see-you-at-ten reply says "report your altitude and range", and it had no trigger for the answer ("Angels 9, Range 25"). "range N" is now a DME report. "entering stack position 3, angels 2" now counts as §4 established.
  - **Tower callsign bleed blocked a takeoff (`3d20e82`).** "Venom 203 requests taxi to the active" became the callsign whole, since the trigger list knew "request taxi" but not "requests". That phantom headed Senaki's departure queue, so the real Venom 203's LUAW auto-release logged `auto-release skipped — not at departure-queue head` and he sat on the runway until he asked himself. "Venom 201 reporting 9 miles" did the same in the landing sequence ("number three" for a number two). Fix is general: `trimTrailingTriggers` now cuts after the number whenever a word follows. **Symptom to recognise:** a TX opening with the pilot's whole sentence, or a `queueHead` in the skip log that isn't a callsign. Senaki's `/status` on 6013 shows the queues.
  - **Deckboss (`e31c66c`, `a178aa1`, `3d20e82`).** Raider 308 needed three tries for a comm check: "comcheck" (one word) was missing from §6, and Whisper rendered the address as "Dark boss" and "Black boss". The guard and the callsign reader now share `deckbossAddrPrefixes`; before, they had separate lists. "Deck boss, Raider, 308" was answered "Raider, Deckboss" — the comma cut. `extractCallsignSkippingAddress` now rejoins "word, digits".
  - **"request permission to the active"** is now a taxi request (`3d20e82`).
  - **§8 departure release said "contact tower at seven DME" (`754347e`).** It meant "call me back at 7 DME", where the tower hands off to Command, but it sounded like a frequency change. Now "report seven DME". **Don't point it at Command** — that skips the tower's 7 DME handoff.
  - **Senaki gave Raider 302 three clearances (16:05-16:07)** after he called holding short twice: LUAW, LUAW + clear, then the delayed auto-release fired "cleared for takeoff" the same second he reported airborne. Not reproduced on single calls in the evening. **Open:** the auto-release should not fire on an aircraft already cleared or airborne.
  - **Still open:** Command does not answer "radar check" (Senaki does). Whisper returned its own prompt word list three times on Senaki (19:53, 20:58, 21:04), each dropping a real call as `Whisper hallucination`. Garbled field names ("Snagdriver greater 302") and an unaddressed "N203, clear the active" were silently dropped — correct behaviour.
  - Command's proactive recovery handoff (`Command: proactive recovery handoff issued`, Venom 203 at 29 nm, 20:41) fired and its own TX echoed back as an intent miss; it was correctly not answered.
- **Possible Hitman self-echo, unresolved.** At 11:48:39 a never-seen SRS origin (`9HcRJ…`, 485 frames) returned Hitman's full 9-line word for word, 21 s after Hitman keyed, and Hitman answered "good readback". The short "loud and clear" at 11:46 did not echo, so it is not proven. The mid-call guard in `6dd6b0a` covers it either way. **Watch for another new origin after a long Hitman TX.**
- **11:30 rotation** loaded `V1.23.LIGHTNING Isolated TS Cells (Winter)`, altimeter **29.92**. That is the mission's real value, not the static fallback; check `current_mission.txt` before suspecting defaults.

## Recent context (2026-09-14)
- **SkyEye (Darkstar) GCI shares DCS3 with the ATC roles.** Set up from `C:\Skyeye\SKYEYE-VM-SETUP.md`: SkyEye **v1.10.0** at `C:\Skyeye\skyeye-windows-amd64`, managed by **DCSServerBot** (`C:\CDCS-BotsDCSServerBot\DCSServerBot-3.0.4.13`, started by hand from `run.cmd`, not a Windows service) with `autoupdate: true`. Config is `Saved Games\DCS.dcs_serverrelease\Config\SkyEye.yaml`; VECTOR destinations are in `Config\locations.json` (21 Caucasus fields). Darkstar transmits on **264.0 AM** as `GCI Darkstar [BOT]` and uses the same SRS (:5008, blue EAM), Tacview (:42676) and gRPC (:50051) as the ATC roles. v1.8.1 rollback copy: `C:\Skyeye\rollback-v1.8.1-20260914-1152`.
- **DCSServerBot owns `SRS-Server.exe`, `DCS_server.exe` and `skyeye.exe`** — they are children of its `run.py`. Its scheduler (`config\plugins\scheduler.yaml`) restarts DCS, SRS and SkyEye at 00:00 / 04:30 / 08:35 / 11:30 / 15:45 / 22:00 local, so ATC roles lose SRS briefly at those times. Killing the bot's `python run.py` does not stop its children.
- **Don't relaunch `skyeye.exe` by hand.** DCSSB's watchdog restarts it within seconds (`SkyEye died - restarting ...` in `logs\dcssb-DCS3.log`), so a manual launch puts two Darkstars on 264.0 — happened 2026-09-14 ~11:53–11:56. To swap the binary: stop `skyeye.exe`, wait until the exe file unlocks (a forced kill took >20s to exit twice), copy the new exe, and let the watchdog start it.
- **Tacview real-time telemetry is :42676 on DCS3**, not the 42674 in the setup runbook's Training config.
- The watchdog-started SkyEye left `Logs\skyeye.log` empty, so an empty log is not proof it's down — check for `GCI Darkstar [BOT]` in `clients-list.json` or its TCP connections to :5008 / :42676.

## Recent context (2026-09-02)
- **`SKYEYE_MIZ` went stale mid-session.** The Missions folder was refreshed at 08:59 — `v1.1` deleted, replaced by **29 `v1.3` weather variants** — so the pinned path silently fell back to `--static-*` defaults on every role. Repinned to the loaded mission, found via `Logs\dcs.log`: grep the `[LOTATC]  ->  ...miz` line, which names what DCS actually loaded. **The server rotates missions, so this pin goes stale on its own — re-check whenever weather looks wrong.**
- **SRS client GUID fix (`d231884`).** Every role derived its GUID from `time.Now().UnixNano()` alone, which is not unique on Windows — the clock is far coarser than a nanosecond. Eight towers launching within ~81ms drew colliding GUIDs; SRS merged them into shared slots and the losers went silent while still looking connected. Symptom was mangled entries: `Akrotiri Tower` carrying Hatay's 250.300, `H4 Tower` carrying Ramat David's 251.300. Now PID-based via `srsClientGUID`. **The `timeout` staggers in `start_towers_syria.bat` are skipped when the bat is driven non-interactively** (`Input redirection is not supported`) — that is what bunched the launches, so give towers real spacing when scripting them.
- **16kHz audio fix (`6f4afb2`)** — see the ExternalAudio section above.
- **Card reconciliation (`ee2f38e`)** — King Hussein + H4 towers restored after being removed earlier the same day while chasing the GUID bug; Deckboss kept on 128.600 by ruling. Every other frequency already matched the card.
- **Leftovers on disk, all untracked:** `atis_cache_old_44k/` (17 superseded 44.1kHz ATIS mp3s, safe to delete), `start_towers_syria.bat.bak`, and the presets JPG.
- **The ffmpeg format-string trap.** A bad verb in the ATIS concat filter does not fail loudly: `concatMP3WithSilence` errors, the caller falls back to English-only, and bilingual ATIS quietly disappears behind a single `warn` line. `%[1].3f` is wrong — Go wants the argument index *after* the precision — and renders `%!f(BADINDEX)`. `atisConcatFilter()` is split out and tested for exactly this.

## Recent context (2026-08-06)
- **Deckboss phraseology reworked, `92a428c`** (pushed to `main`, **not yet deployed to Training 1**). Three operator-requested changes: (1) §2 under tension now ends with "shooter's discretion" — two variants, `under tension cat {CAT}, spin it up, shooter's discretion` / `copy under tension, go gates, shooter's discretion`; (2) §4 airborne is a bare `copy airborne`, and the Command departure handoff that rode on that ack is **removed** — pilots switch on their own; (3) the auto `Cat {CAT}, shoot, shoot, shoot` TX is **gone entirely** (`DeckbossShoot` deleted) because Deckboss no longer calls the shot. Its timer survives, cut 5s → 10s, and now only does slot management. **Timeline changed: was T+0 tension / T+5 shoot / T+15 cat clear, now T+0 / T+10 clear.**
- **Both new responses repeat their own trigger words — the self-echo guards had to change.** "shooter's **discretion**" contains the bare `shoot` trigger for §2; the existing address-led guard already drops the callsign-led echo, but it is now load-bearing for a second reason (noted in code — don't remove it). §4 has no address guard and its old protection was literally *avoiding the word "airborne"*, which the new ack breaks. Replaced with a mid-string check: drop any non-address-led call that carries a Deckboss token, which is exactly our own echo shape (`Raider 032, Deckboss, copy airborne.`) and never a pilot call. Both pilot phrasings still land — address-led and bare.
- **`--handoff-command-freq` / `--handoff-command-preset` no longer reach Deckboss** (consequence of dropping the §4 handoff). `--handoff-command-name` still feeds Deckboss's pilot-initiated `pushing command` ack. All three remain live for Marshal. `handoff_responses.md` §3 is marked removed with the old variants kept for reference.
- **Deckboss voice `ash` → `shimmer`** (female, operator request). Note the prior CLAUDE.md voice line claimed `fable`, which `start_deckboss.bat` had **never** used — it had been `ash` since the 128.6 bring-up. Second time this voice line has drifted from the bat files (see 2026-05-29 below); **read the bat file, don't trust the doc.** Changed in `start_deckboss.bat` plus the code defaults in `main.go` and `deckboss.go` so a bare launch is female too. See the voice section above for why the all-unique rule is now broken.
- **TTS cache keys on the voice profile**, so the voice change silently invalidates every cached Deckboss line — expect re-synthesis latency on the first few calls after restart, then it settles.

## Recent context (2026-08-01)
- **Deployed `c9781f0` to Training 1** — pulled 3 commits (Iraq theatre `3f108a0`, phraseology + inter-role handoffs `b57ddc6`, per-role TTS delivery style `c9781f0`), rebuilt both binaries, full restart of the PG set. Prior binaries kept as `atc.rollback.exe` / `launcher.rollback.exe`. Wind-driven runways came up **OMDM=09 / OMAM=13R / OMAL=19** (13R not the usual 31L — wind was 160°, not a regression).
- **`SKYEYE_MIZ` had gone stale — new failure mode worth knowing.** It pointed at `V0.9.75 Convoy PID (Day)`, deleted when the Missions folder was refreshed to V0.9.78. A **missing** `--miz-path` does *not* fall back to newest-.miz — it goes straight to `--static-*` defaults, so every role silently ran default weather and `/api/miz-weather` 500'd. Repinned to `V0.9.78.CONVOY TRAINING (VFR 4 Broken Cumulus)`. **Check the pinned file still exists after any Missions-folder refresh**, not just that the var is set.
- **Deckboss Tacview-driven cat assignment** — new `--deck-position-check` flag (**default off**). On a §1 check-in Deckboss reads the caller's Tacview position, projects it onto the carrier BRC axis, and prefers **bow cats 1/2** when forward / **waist cats 3/4** when aft, instead of always taking the lowest free cat. Fails open to first-free on missing carrier, unknown callsign, stale (>30s) contact, or an aircraft off the boat. New `controller.DeckSpotNm` / `controller.GetDeckContacts` / `state.AssignCatPreferred`; 15 new tests. Roll out by adding the flag to `start_deckboss.bat` for a session and watching the `cat assigned from Tacview deck position` logs before promoting to default.
- **Latent bug noted, not fixed** — `state.AssignCat` (the default, flag-off path) doesn't check whether a callsign already holds a cat, so a repeated §1 check-in assigns a second slot and leaks the first. The new `AssignCatPreferred` guards it; fix `AssignCat` when the flag is promoted.
- **Deckboss runs with runway rotation enabled** — the three towers pass `--runway-rotation=false`, `start_deckboss.bat` doesn't, so `atc-omdm.log` carries a `runway rotation enabled — slot-based, ignores wind` line from the Deckboss process. Harmless (Deckboss doesn't drive tower runway selection) but inconsistent.

## Recent context (2026-07-02)
- **Deployed + live on Training 1 (main `94ea75f`):** full kill-all → rebuild → `start_all.bat` restart earlier in the day, then a Deckboss-only bounce. Current running roles are the `94ea75f` binary. Weather pinned to Convoy PID (Day) via `SKYEYE_MIZ`; runways OMDM=09/OMAM=31L/OMAL=19.
- **ATIS↔ATC runway alignment hardened** (`1622ecd`) — ATIS now treats the paired tower as the *sole* runway source: adopts + remembers it (`lastTowerRwy`), holds last-known across a tower blip, and *suppresses* the runway line rather than speak the cross-airfield OMDM static fallback (ATIS starts before towers). Killed the per-cycle "picked up runway change" log spam.
- **`SKYEYE_MIZ` weather-source fix** (`195244d`) — roles were seeding boot weather from *newest .miz on disk* (Iron Hammer `v0_46`, saved 06-27) instead of the training mission. New optional `SKYEYE_MIZ` env var → `--miz-path` in all `start_*.bat` (quoted; nested-cmd/c quoting verified). Launcher `/api/miz-weather` also honors `--miz-path` now (`c63aa03`). See env-var table + weather-seeding note up top. **Rotate `SKYEYE_MIZ` per mission like `SRS_EAM`.**
- **LUAW auto-release diagnostics — DEPLOYED + VALIDATED** (`6fc3ce1`, merged to main). Instrumented `scheduleAutoRelease` (`controller.go`): every early-return now logs its reason (was 5 silent returns), plus an "auto-release armed" log at launch. Purely additive. **Live flight test 2026-07-02 09:44 (Raider 311, OMDM 09): the two-stage LUAW → auto-release → cleared-for-takeoff fired flawlessly** (`armed` → `after LUAW`). **The 2026-07-01 stall did NOT reproduce** → it's *conditional*, prime suspect = a **second holding-short call ~65s after the first** (yesterday had two OMAM calls; today's single call worked). Diagnostics now catch the exact tripping guard whenever it recurs. (Also debunked: the "mutex held across transmit" theory — `transmitFn` is a non-blocking `txChan` enqueue, `main.go:569`.)
- **Deckboss §8 bolter → LSO handoff** (`94ea75f`, deployed) — response changed from 600ft/1nm pattern params to "copy remain in bolter, contact LSO"; added the bare `"remain in bolter"` trigger (the flown form; old list only had `remain bolter`/`in the bolter`, neither substring-matched). Self-echo safe (address guard is `HasPrefix`, Deckboss TX leads with callsign).
- **Two tower dashboards were dead** (OMDM 6001, OMAL 6003 — leftover port holders from a prior start); the full restart recovered them. **Launcher died overnight** (was up 17:24, gone by 09:xx); it's stable standalone (ran 50s clean foreground) — a detached-launch quirk, not a crash loop; watch for recurrence. Launch it detached (`Start-Process launcher.exe … -WindowStyle Minimized`) and pass `--miz-path` as ONE quoted arg or the spaced path mangles → `/api/miz-weather` 500.
- **CAUCASUS (Black Sea) map — DESIGNED + MERGED to main (`d805e5c`), NOT deployed** *(as of 2026-07-02; superseded — live on DCS3 by 2026-09-14, see top of file)*. Four fields from `CA KNEEBOARD/` (gitignored): Batumi UGSB / Kobuleti UG5X / Senaki UGKS / Kutaisi UGKO — see `CAUCASUS_PLAN.md` for the full reconciled roster + freqs. Wiring: `airfield.ByICAO`/`FieldsForMap` registry (`pkg/airfield/registry.go`) replaced the `--airfield` switch; new `--map pg|caucasus` flag drives the ATIS set (`atisStationsForMap`) + Command handoff scan; CA dashboard ports 6011–6014; `start_{towers,atis,command,all}_caucasus.bat`. **PG behavior byte-identical (default `--map pg`).** **Binary NOT yet swapped** — live `atc.exe` is locked by running roles, so a CA-capable binary is staged as `atc.exe.new` (verified: contains UGSB/caucasus). Next PG restart: `go build` (or `move /y atc.exe.new atc.exe`) makes `atc.exe` CA-capable. **Open before CA goes live:** (1) carrier freq mapping — CA card has LSO(AI)=128.6/Airboss=306.3/Deckboss=306.2/Marshall=306.1 vs PG Marshal 306.3/Deckboss 128.6 (bat-config only; CA marshal/deckboss scripts deferred); (2) verify Batumi ILS (card's "09/108.90" is a Senaki copy-error → placeholder 110.30 RWY13), MagVar +6.5, left-pattern assumption; (3) thresholds are computed, not surveyed (position-check is off by default so OK). **One map runs at a time** — one DCS mission → one Tacview/weather feed; switching = stop current set → repoint `SKYEYE_MIZ` → start the other set.
- **Pending fix batch** (candidates, dev-rig test then deploy): #2 `taking off runway`→RunwayVacated misclassification (`controller.go:1172` `"off runway"` substring-matches "taking off runway" → tower told a departing a/c "taxi to parking"); #3 `"proceeding to runway XX for takeoff"` intent-miss (no trigger); #4 Command `"fencing in"` FENCE-crossing status call (no trigger); departure handoff wording "contact **tower** at 7 DME" should be Command/departure; Deckboss has no intent-miss logging (bolter drop was silent); STT/alias robustness (field-name garble "Almanon/Almanar", callsign digit-doubling 311→3311, trailing-connector strip).

## Recent context (2026-06-26)
- **v1.5.0 cut and pushed** — annotated tag at `5b7f79b`, MINOR bump (batch since v1.4.0 includes the new Marshal §5a platform intent + new Case 3 phraseology — features on an existing role, so per [[version-tag-scheme]] MINOR not PATCH). Eight commits since v1.4.0:
  - `5b7f79b` (this session) — **three fixes from the 2026-06-25 evening "not responding / slow / had to transmit multiple times" report:**
    - **ATIS↔ATC runway mismatch (root cause).** `buildATISText` (`cmd/atc/atis.go`) was computing the live active runway (`state.activeRwy`, synced from the paired tower via `fetchTowerRunway` → tower `/status` on 6001/6002/6003) and then **discarding it** — pilots heard a *hardcoded* "Active runway NN" baked into each station's `Advisory` string. Permanent guaranteed divergence. Fix: ATIS now speaks the live runway (spelled via new local `spellRunwayWords`, mirrors unexported `composer.spellRunway`), **only for stations with a paired tower** (Liwa/Kish stay advisory-only). Hardcoded runways stripped from Dhafra/Minhad/Al Ain advisories. ATIS lags the tower by ≤1 broadcast cycle (~45s poll) after a wind shift — inherent to the cross-process design, not a bug.
    - **Wind-based runway selection** — `--runway-rotation=false` added to all three towers in `start_towers.bat` (propagates via `start_all.bat`). Was 4h time-rotation (`RotationRunway`); now wind-driven (`airfield.ActiveRunway`). Bat-only so the rotation feature + code default (`--runway-rotation` true at `main.go:199`) stay intact for the dev rig; reversible without rebuild. Disabling logs `runway rotation disabled — reverting to wind-driven selection`.
    - **Whisper false-drop** — removed `"base, final, runway"` from the `isWhisperHallucination` compound list (`main.go`). It matched no real prompt-echo (the prompt says "base, short final, clear active") but killed legit pattern calls like "left base, final, runway 09", forcing re-transmits. Confirmed live: Raider 311's 20:21:23 base call dropped → pilot repeated 18s later.
  - Other commits in the batch: `91df029` / `da2216c` marshal Case 3 §5/§5a + §1 marking mom's + §4 established (the feature driving the MINOR bump), `c573af5` marshal Case 3 polish + docs, `d1fda34` OMDM/OMAM field-name aliases, `bb0ca1f` Deckboss Whisper variants + KISH (OIBK) replaces Khasab on 248.5, plus doc commits.
- **Deployed + verified on Training 1** — full restart (kill all 7 `atc.exe` + launcher → `go build` both binaries → `start_all.bat` via `cmd /c`). All wind-driven: **OMDM=09, OMAM=31L, OMAL=19**; ATIS mirrors each (Dhafra→31L, Al Ain→19, Minhad→09); zero errors post-restart. Note: `cmd /c start_all.bat` from non-interactive PowerShell throws `Input redirection is not supported` on the inter-launch `timeout.exe` calls — harmless, all `start` windows still spawn (staggers skipped). Also `git commit`/`git push` here-strings via PowerShell tripped `'/' is outside repository` — used `git commit -F <file>` instead.

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
Produces `atc.exe` and `launcher.exe`. Requires Go 1.26+. `go.mod` has no SkyEye dependency (checked 2026-09-14), so the "SkyEye cloned at `C:\Skyeye`" line still in `build.bat`'s comment is stale — on DCS3 `C:\Skyeye` holds the SkyEye **release binary** for the Darkstar GCI, not a source clone.
