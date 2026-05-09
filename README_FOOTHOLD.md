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
- **`OPENAI_API_KEY` env var** set system-wide (`setx OPENAI_API_KEY "sk-proj-..."`). Confirm in a fresh shell: `echo %OPENAI_API_KEY%`.
- **`ffmpeg.exe`** on PATH (used for radio effect / audio re-encode).
- **DCS-SR-ExternalAudio.exe** present where atc.exe expects it (set via `--external-audio` flag if non-default).

You do **not** need DCS-gRPC for ATIS-only/Tower-only deployment — it's only used by Scudwatch which isn't deployed here.

---

## 2. What to copy to Foothold

Copy `C:\SkyeyeATC\` from Training 1 (or `git clone https://github.com/hiker1973-droid/atc.git` and run `build.bat`).

Required:

| Item | Notes |
|---|---|
| `atc.exe`, `launcher.exe` | Built binaries (or build on foothold) |
| `configs\*.yaml` | Per-airfield configs (already point at `localhost:5008` — needs editing) |
| `start_atis.bat`, `start_towers.bat`, `start_launcher.bat` | Launch scripts |
| `cmd\`, `pkg\`, `go.mod`, `go.sum`, `build.bat` | Source — only needed if rebuilding on foothold |
| `atis_cache\` | Optional — copy to skip first-run TTS regeneration |

Skip:
- `logs\` (regenerates)
- `start_command.bat`, `start_marshal.bat`, `dev_only/` (roles not deployed here)
- `*.exe~`, `*.lnk` (build/operator artifacts)

---

## 3. Post-Copy Config

### 3a. Update SRS port in launch scripts

Each `start_*.bat` defaults to `localhost:5008`. Foothold needs `5002`. Edit:

```bat
set SRS=localhost:5002
```

In:
- `start_atis.bat`
- `start_towers.bat`
- `start_launcher.bat` (`--srs-addr localhost:5002`)

### 3b. Update SRS port in YAML configs

`configs\alain.yaml`, `configs\dhafra.yaml`, `configs\minhad.yaml` — change any `srs:` host reference from `:5008` → `:5002`.

### 3c. Centralize OpenAI key (one-time)

Set globally so we don't have to keep editing bats:

```bat
setx OPENAI_API_KEY "sk-proj-..."
```

Open a fresh shell after `setx` (the current shell won't pick it up).

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
| `SRS TCP failed` looping | SRS not on `:5002`, or `set SRS=` in bat still says `5008` |
| `TTS prewarm failed` / `OpenAI 401` | `OPENAI_API_KEY` missing in the shell that launched atc.exe |
| `Whisper returned empty transcription` | OpenAI rate limit or pilot mic muted — usually self-heals |
| `ExternalAudio file error` | `ffmpeg` not on PATH, or DCS-SR-ExternalAudio.exe path wrong |
| Tower hands off to "command on 282.0" — but Command isn't deployed | Expected. Tower's handoff line is data-driven from `pkg/airfield/*.go`. Either ignore it, or edit `HandoffFreqMHz` / `HandoffPreset` to point somewhere relevant on Foothold |
| ATIS skipping cycles ("broadcast already in progress") | Audio runtime exceeds 45s interval — drop back toward 60-90s in `cmd/atc/atis.go:336` and `main.go:537` (rebuild required) |
| Pilots see Towers but not ATIS in client list | ATIS is broadcast-only — no UI presence required. Pilots should still hear it on the freq |

---

## 7. Useful Reference Docs

- `PILOT_WORKFLOW.md` — squadron-facing reference for pilot interactions (frequencies, sample exchanges, trigger phrases)
- `departure_responses.md` / `arrival_responses.md` — tower phraseology spec
- `CLAUDE.md` — project overview for Claude Code on the rig
