# SkyeyeATC — Training 1 Transfer & Operator Reference

Transfer guide for moving SkyeyeATC from the dev box (vsfg7-atc) to **Training 1**, plus the day-to-day commands operators use once it's installed.

Training 1 SRS port: **5008** (vs. 5004 on dev).

---

## 1. Transfer Prep (do this on dev before copying)

1. **Stop everything running.** Close all `atc.exe` / `launcher.exe` windows so logs and `atis_cache/` aren't being written during the copy.
2. **Delete stale broken scripts.** These pass the `--grpc-addr` flag which `atc.exe` no longer accepts and will fail at launch:
   - `run_alain.bat`
   - `run_dhafra.bat`
   - `run_minhad.bat`
3. **Clear caches** (optional but recommended — they get regenerated on first run):
   - `atis_cache\*.mp3`
   - `logs\*.log`
4. **Confirm a clean build.** Run `build.bat`; both `atc.exe` and `launcher.exe` should report success.

## 2. What to copy to Training 1

Copy the entire `C:\SkyeyeATC\` folder. Required:

| Item | Notes |
|---|---|
| `atc.exe`, `launcher.exe`, `logtail.exe` | Built binaries |
| `configs\*.yaml` | Per-airfield configs |
| `start_*.bat`, `launcher.bat`, `start_launcher.bat` | Launch scripts |
| `build.bat`, `cmd\`, `pkg\`, `go.mod`, `go.sum` | Source — only needed if rebuilding on Training 1 |

Skippable: `logs\`, `atis_cache\`, `*.exe~`, `New Text Document.txt`, `atc-prev.exe`.

## 3. Post-Transfer Config (on Training 1)

### 3a. Centralize the OpenAI key
Today the key is duplicated in `start_atis.bat`, `start_towers.bat`, and the parked bats under `dev_only/`. Set it once as a Windows user env var instead:

```bat
setx OPENAI_API_KEY "sk-proj-..."
```

Then strip the `set OPENAI_API_KEY=...` lines from each `.bat` (or leave them — `setx` wins on a fresh shell).

### 3b. Update SRS / Tacview hosts
Edit each `start_*.bat` and change:

```bat
set SRS=192.168.1.221:5004        →   set SRS=localhost:5008
set TACVIEW=192.168.1.221:42676   →   set TACVIEW=localhost:42676   (or Training 1's actual host)
```

Same in `start_launcher.bat` (`--srs-addr` and `--tacview-addr` args).

The YAML configs already point at `localhost:5008`, so no edit needed there.

### 3c. Verify prereqs on Training 1
- SRS server running on `:5008`
- DCS-gRPC enabled on `:50051` (`MissionScripting.lua` desanitised, gRPC extension on)
- Tacview real-time telemetry enabled on `:42676`
- `OPENAI_API_KEY` reachable from a fresh shell: `echo %OPENAI_API_KEY%`

### 3d. Rebuild (only if you copied source instead of binaries)
```bat
build.bat
```
Requires Go 1.26+ and SkyEye cloned at `C:\Skyeye`.

---

## 4. Operator Commands (day-to-day)

All scripts live in `C:\SkyeyeATC\` and are double-clickable. Each opens its own console window with `cmd /k` so you can read it.

### Recommended start order (Training 1)
Only Tower + ATIS run on Training 1 — they're stable and battle-tested. Carrier ops, Command, and Scudwatch are parked under `dev_only/` for iteration on the dev rig.

1. **`start_launcher.bat`** — opens browser dashboard at http://localhost:7000/. Use this to start/stop the rest from a UI instead of running each `.bat` by hand.
2. **`start_atis.bat`** — all 5 ATIS stations (Dhafra, Minhad, Liwa, Al Ain, Khasab) on their own freqs. Now broadcasts every **45 seconds per station** (was 120s).
3. **`start_towers.bat`** — Al Minhad / Al Dhafra / Al Ain Tower instances. Each binds dashboard ports 6001 / 6002 / 6003.

### Parked roles (do not launch on Training 1)
These bats live in `dev_only/` and are intended for the dev rig only:
- `dev_only/start_command and deckboss.bat` — Command (282.0) + Deckboss (306.2)
- `dev_only/start_marshal.bat` — Marshal stack (306.3)
- `dev_only/run_scudwatch.bat` — Scudwatch / Darkstar-1-1 (264.0)

### Frequency cheat sheet
| Role | Freq MHz | Site |
|---|---|---|
| Al Minhad Tower | 250.1 | OMDM |
| Al Dhafra Tower | 251.1 | OMAM |
| Al Ain Tower | 250.7 | OMAL |
| Deckboss | 306.2 | OMDM |
| Marshal | 306.3 | OMDM |
| Command | 282.0 | — |
| Scudwatch | 264.0 | — |
| ATIS Dhafra / Minhad / Liwa / Al Ain / Khasab | 248.2 / 248.3 / 248.55 / 248.85 / 248.5 | — |

### Stopping
Close each console window (`Ctrl+C` then `Y`, or just close the window). The launcher dashboard has stop buttons too.

### Logs
JSONL in `C:\SkyeyeATC\logs\`:
- `atc-omdm.log` — Minhad Tower + Deckboss + Marshal
- `atc-omam.log` — Dhafra Tower
- `atc-omal.log` — Al Ain Tower

Tail with `logtail.exe` or any JSONL viewer. Filter on `level=warn|error`, plus `Tower heard|TX`, `Deckboss`, `Marshal`.

---

## 5. Common Issues After Transfer

| Symptom | Likely cause |
|---|---|
| `SRS TCP failed` repeating in logs | SRS not on `:5008`, or wrong host in `start_*.bat` |
| `TTS prewarm failed` / `OpenAI 401` | `OPENAI_API_KEY` missing in the shell that launched `atc.exe` |
| `Whisper returned empty transcription` | OpenAI rate limit or pilot mic muted — usually self-heals |
| `ExternalAudio file error` | `ffmpeg` not on PATH, or the EAM external-audio path in atc.exe broken |
| ATIS skipping cycles | Audio longer than 45s — station's broadcasting mutex is blocking. Drop interval back toward 60–90s if you see "broadcast already in progress" warnings |
| `--grpc-addr ... unknown flag` | You're running an old `run_alain/dhafra/minhad.bat` — delete those |

---

## 6. Using Claude Code on Training 1

Once installed on the Training 1 VM, drop these in `CLAUDE.md` at `C:\SkyeyeATC\` so Claude has context:
- Production rig — be cautious with destructive ops during sessions
- SRS on `:5008` (not 5004 like dev)
- Logs at `C:\SkyeyeATC\logs\atc-{omdm,omam,omal}.log`
- Operator-facing flow is the launcher dashboard at `http://localhost:7000/`

Common asks Claude can help with on Training 1:
- "Tail the towers and tell me what you see in the last 5 minutes"
- "Why did Marshal go quiet at 14:32?"
- "Adjust ATIS interval / wind / runway in `configs/*.yaml` and restart the affected service"
- "Add a new ATIS station for OMXX"
