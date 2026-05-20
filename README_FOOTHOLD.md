# SkyeyeATC — Foothold Install & Operator Reference

Deployment guide for installing **ATIS + 3 Tower instances** on the **Foothold** Windows VM (`192.168.1.222`). Foothold runs a slimmer role set than Training 1 — no Marshal, Command, Deckboss, or Scudwatch.

| Setting | Value |
|---|---|
| Host IP (LAN) | `192.168.1.222` |
| SRS server | `localhost:5002` |
| Tacview real-time | `localhost:42676` |
| Roles | ATIS (5 stations) + 3 Towers (OMDM, OMAM, OMAL) |

---

## 1. Prereqs (verify on Foothold)

- **SRS Server** running on `:5002`. Confirm via `netstat -ano | findstr :5002` — should show `LISTENING`.
- **Tacview real-time telemetry** enabled on `:42676` (DCS Tacview export setting).
- **Four env vars set system-wide** (v1.1.0+ requirement — bats fail-fast without them):
  ```bat
  setx OPENAI_API_KEY  "sk-proj-..."
  setx SRS_EAM         "blue42"
  setx SKYEYE_SRS      "localhost:5002"
  setx SKYEYE_TACVIEW  "localhost:42676"
  ```
  After `setx`, **open a fresh shell** so the new values are in scope. Verify with `echo %SKYEYE_SRS%`.
- **`ffmpeg.exe`** on PATH (used for radio effect / audio re-encode).
- **DCS-SR-ExternalAudio.exe** present where atc.exe expects it (set via `--external-audio` flag if non-default).

You do **not** need DCS-gRPC for ATIS-only/Tower-only deployment — it's only used by Scudwatch which isn't deployed here.

---

## 2. Install via git (recommended)

On the Foothold VM, open a shell where `git` and `go` are on PATH:

```bat
cd C:\
git clone https://github.com/hiker1973-droid/atc.git SkyeyeATC
cd C:\SkyeyeATC
build.bat
```

`build.bat` produces `atc.exe` and `launcher.exe`. Requires Go 1.26+ and SkyEye cloned at `C:\Skyeye` (build.bat references it for vendored deps).

### Pulling updates later

When dev pushes changes (most code lives on `main`):

```bat
cd C:\SkyeyeATC
taskkill /IM atc.exe /F
git pull origin main
build.bat
```

Then relaunch the bats (section 4). The `taskkill` is required before `build.bat` — Windows won't overwrite a loaded `.exe`. Alternatively, do the rename trick:

```bat
move atc.exe atc.exe.old
build.bat
:: kill + relaunch atc.exe processes when ready
del atc.exe.old
```

This lets you build the new binary while the old one is still running, then bounce processes on your own schedule.

### Alt: copy from Training 1

If you don't want git on Foothold, copy `C:\SkyeyeATC\` wholesale from Training 1.

Required:

| Item | Notes |
|---|---|
| `atc.exe`, `launcher.exe` | Built binaries (or build on foothold) |
| `start_atis.bat`, `start_towers.bat`, `start_launcher.bat` | Launch scripts (env-var driven — no per-host edits needed) |
| `cmd\`, `pkg\`, `go.mod`, `go.sum`, `build.bat` | Source — only needed if rebuilding on foothold |
| `atis_cache\` | Optional — copy to skip first-run TTS regeneration |

Skip:
- `logs\` (regenerates)
- `start_command.bat`, `start_marshal.bat`, `dev_only/` (roles not deployed here)
- `configs\*.yaml` (not currently read by `atc.exe` — historically pointed at `:5008` and never updated; cosmetic only)
- `*.exe~`, `*.lnk` (build/operator artifacts)

---

## 3. Post-Copy Config

As of v1.1.0+ all per-host config is via env vars (see section 1). **No bat-editing is required after copy.** If the bats fail-fast with `ERROR: <VAR> env var not set`, you missed a `setx` or didn't open a fresh shell — re-do section 1 and try again.

If the four env vars are set correctly, skip ahead to section 4.

---

## 4. Operator Workflow

All scripts in `C:\SkyeyeATC\`. Each opens its own console window with `cmd /k` so logs are visible.

### Recommended start order

1. **`start_launcher.bat`** — dashboard at `http://localhost:7000/` (or `http://192.168.1.222:7000/` from another machine on LAN).
2. **`start_atis.bat`** — all 5 ATIS stations (Dhafra, Minhad, Liwa, Al Ain, Khasab).
3. **`start_towers.bat`** — Al Minhad / Al Dhafra / Al Ain Tower instances. Dashboards on ports 6001 / 6002 / 6003.

Do **NOT** launch on foothold:
- `start_command.bat` / `start_marshal.bat` (not deployed here)
- Anything under `dev_only/`

### Frequency cheat sheet

| Role | Freq (MHz) | Site |
|---|---|---|
| Al Minhad Tower | 250.100 | OMDM |
| Al Dhafra Tower | 251.100 | OMAM |
| Al Ain Tower | 250.700 | OMAL |
| ATIS Dhafra | 248.20 | OMAM |
| ATIS Minhad | 248.30 | OMDM |
| ATIS Khasab | 248.50 | — |
| ATIS Liwa | 248.55 | — |
| ATIS Al Ain | 248.85 | OMAL |

All AM, encryption off.

### Stopping

Close each console window, or use the launcher dashboard's stop buttons.

### Logs

JSONL in `C:\SkyeyeATC\logs\`:
- `atc-omdm.log` — Al Minhad Tower
- `atc-omam.log` — Al Dhafra Tower
- `atc-omal.log` — Al Ain Tower

Logs auto-rotate at 50 MB. Tail with `logtail.exe` or any JSONL viewer; filter on `level=warn|error`, plus `recognized` and `TX via`.

---

## 5. SRS Server Settings to Verify

In `serverSettings.cfg` for the SRS instance on `:5002`:

| Setting | Required value | Why |
|---|---|---|
| `EXTERNAL_AWACS_MODE` | `true` | Lets atc.exe register as EAM clients (otherwise they sneak in as spectators, fragile) |
| `EXTERNAL_AWACS_MODE_BLUE_PASSWORD` | `blue42` | Must match `--eam-password` in the bats |
| `GLOBAL_LOBBY_FREQUENCIES` | empty, or set to a freq nothing operational uses | Lobby freqs in SRS have separate TX gating; **must not** overlap any tower/ATIS freq (250.1, 250.7, 251.1, 248.2, 248.3, 248.5, 248.55, 248.85) |
| `COALITION_AUDIO_SECURITY` | `false` | Otherwise EAM-blue clients can't hear/be-heard by red coalition pilots |
| `STRICT_RADIO_ENCRYPTION` | `false` | Avoids silent TX drops on encryption-mismatch |
| `TRANSMISSION_LOG_ENABLED` | `true` (recommended for first month) | Lets us see what hits the server when debugging silent TX issues |

---

## 6. Common Issues After Transfer

| Symptom | Likely cause |
|---|---|
| `ERROR: <VAR> env var not set` at bat startup | One of the four env vars (`OPENAI_API_KEY`, `SRS_EAM`, `SKYEYE_SRS`, `SKYEYE_TACVIEW`) wasn't set, or the bat was launched in a shell that pre-dates the `setx`. Open a fresh shell. |
| `SRS TCP failed` looping | SRS not listening on the address in `$SKYEYE_SRS`, or value typo. Re-check with `netstat -ano | findstr :5002` |
| `TTS prewarm failed` / `OpenAI 401` | `OPENAI_API_KEY` missing or invalid in the shell that launched atc.exe |
| `Whisper returned empty transcription` | OpenAI rate limit or pilot mic muted — usually self-heals |
| `ExternalAudio file error` | `ffmpeg` not on PATH, or DCS-SR-ExternalAudio.exe path wrong |
| Tower hands off to Command on a freq nobody is on | Expected — Tower's handoff line is data-driven from `pkg/airfield/*.go` `HandoffFreqMHz` (currently `282.0`). Either ignore it on Foothold (Command isn't deployed) or edit `HandoffFreqMHz` / `HandoffPreset` to suit your op |
| ATIS skipping cycles ("broadcast already in progress") | Audio runtime exceeds 45s interval — drop back toward 60-90s in `cmd/atc/atis.go:336` and `main.go:537` (rebuild required) |
| Pilots see Towers but not ATIS in client list | ATIS is broadcast-only — no UI presence required. Pilots should still hear it on the freq |

---

## 7. Useful Reference Docs

- `PILOT_WORKFLOW.md` — squadron-facing reference for pilot interactions (frequencies, sample exchanges, trigger phrases)
- `departure_responses.md` / `arrival_responses.md` — tower phraseology spec
- `CLAUDE.md` — project overview for Claude Code on the rig
