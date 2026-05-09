# SkyeyeATC — Operator Runbook

For whoever runs the rig (currently Training 1 + Foothold). Day-to-day monitoring, log triage, restart decisions, error signatures and their fixes. Pilot-facing reference is `PILOT_WORKFLOW.md`.

---

## Daily start checklist

In order. Each step verifiable before moving to the next.

1. **SRS server up?**
   ```
   netstat -ano | findstr :5008
   ```
   (or `:5002` on Foothold) — should show `LISTENING`.
2. **OpenAI key in env?**
   ```
   echo %OPENAI_API_KEY%
   ```
   Should print a `sk-proj-...` value, not the literal `%OPENAI_API_KEY%`.
3. **Tacview real-time enabled?** — in DCS server: Options → Special → Tacview → "Enable real-time telemetry on port 42676". Required for Tower's radar-check, speed warnings, conflict detection, and Marshal's stack/BRC.
4. **Launch order** (each in its own console window):
   1. `start_launcher.bat` — dashboard at `http://localhost:7000/`
   2. `start_atis.bat` — 5 ATIS stations
   3. `start_towers.bat` — 3 towers (OMDM/OMAM/OMAL)
   4. *(Training 1 only)* `start_command.bat` and `start_marshal.bat`
5. **Verify TCP** — every role should have a live connection to SRS:
   ```
   netstat -ano | findstr ":5008.*ESTABLISHED"
   ```
   Expect 1 entry per role (5 on Foothold: ATIS + 3 Towers + Launcher; ~7 on Training 1).
6. **Verify registration** — tail `atc-omdm.log` for the most recent `SRS registered — listening` line per tower; Marshal/Command produce `Marshal registered on SRS` / `Command channel registered on SRS`.

---

## Monitoring

### What to tail

```
tail -F C:\SkyeyeATC\logs\atc-omdm.log C:\SkyeyeATC\logs\atc-omam.log C:\SkyeyeATC\logs\atc-omal.log
```

(or use `logtail.exe`, or filter via your favorite JSONL viewer)

### Healthy signals

| Pattern | What it means |
|---|---|
| `"Tacview nominal"` every 60s with `lastData":"0s ago"` | Tacview feeding fresh data |
| `"recognized"` followed by `"TX via OpenAI TTS"` within ~2s | Pilot call → ATC response, full pipeline working |
| `"ATIS loaded from disk cache"` | TTS cache hit, no OpenAI bill for this broadcast |
| `"airContacts": N` with N > 0 | Tacview has visibility into the mission |

### Warning signals (worth watching)

| Pattern | What it likely means | Action |
|---|---|---|
| `"intent miss — pilot transmission not recognized"` | Whisper transcribed but no controller intent matched | Note the `raw` text; if it's something pilots commonly say, file an issue to add the trigger |
| `"SRS disconnected — reconnecting in 5s"` | Brief network hiccup | Self-heals in 5-10s. If it loops > 1 min, SRS server may be down |
| `"Tacview telemetry offline"` | Tacview stream broke | Check DCS still has the export option enabled. Tower keeps running but radar-check / speed warnings disable until it reconnects |
| `"hallucination filtered"` | Whisper produced garbage from background noise; we discarded | Normal, no action |
| `"TX queue full"` | TTS+TX is slower than incoming pilot calls | Rare. If frequent, OpenAI is slow — check OpenAI status |
| `"ATIS broadcast already in progress — skipping"` | Audio runtime exceeded 45s tick interval | Check OpenAI latency; if persistent, bump interval back to 60-90s in `cmd/atc/atis.go:336` and `cmd/atc/main.go:537` |

### Error signals (act on these)

| Pattern | Cause | Fix |
|---|---|---|
| `"SRS TCP failed"` looping | SRS server down or wrong port | Restart SRS server. Verify role's bat has correct port. |
| `"ExternalAudio file error"` | DCS-SR-ExternalAudio.exe failed mid-playback | Usually transient (system load). Multiple in 60s = ATIS overlap; stagger broadcasts more. Exit code `0xc000013a` = DLL init failed (likely concurrent ATIS broadcasts on adjacent freqs) |
| `"Whisper API error: ..."` | OpenAI returned an error | Check `OPENAI_API_KEY` and OpenAI status. 401 = bad key. 429 = rate limit |
| `"TTS prewarm failed"` | OpenAI TTS unreachable at startup | If transient, role still works — TTS retries inline. If persistent, check OpenAI key + connectivity |

---

## When to restart what

### Just restart the affected role

- ATIS misbroadcasting wrong runway/wx → restart `start_atis.bat`
- Tower stops responding to a specific intent → restart that field's tower window
- Marshal not visible in client list → restart `start_marshal.bat` (we fixed the underlying TCP-reader bug, but bouncing forces a fresh register)

### Restart all atc.exe roles

- After `git pull` + `build.bat` (new binary)
- Persistent SRS reconnect loop across multiple roles (likely SRS server itself is the issue)
- After editing any `pkg/airfield/*.go` — affects all towers

```bat
taskkill /IM atc.exe /F
:: relaunch via the bats
```

### Restart SRS server

- `serverSettings.cfg` edits don't take effect until restart
- TCP / UDP listening but no clients can connect (very rare)
- After moving lobby freqs / EAM passwords

### Don't restart unless you have to

- Single intent miss → it's a phrasing issue, not a state issue
- Brief Tacview offline (< 30s) → self-heals
- Single `ExternalAudio file error` → transient

---

## Build + deploy

### Pull updates and roll out

```bat
cd C:\SkyeyeATC
move atc.exe atc.exe.old   :: rename so build can overwrite
git pull origin main
build.bat
taskkill /IM atc.exe /F
:: relaunch the bats
del atc.exe.old             :: only after all old processes have exited
```

The `move` step is required because Windows holds an open handle to a running `.exe`; you can't overwrite it but you CAN rename it. After `build.bat` produces the new `atc.exe`, kill the old processes and relaunch — they'll pick up the new binary.

### Hot-swap a single role (advanced)

If you only need to bounce one role and want to keep the others running:

```bat
move atc.exe atc.exe.old
build.bat
taskkill /PID <role-pid> /F
:: relaunch that role's bat
del atc.exe.old   :: when convenient
```

Other still-running roles keep their loaded `atc.exe.old` image until they exit.

### Build prereqs

- Go 1.26+
- `C:\Skyeye` cloned (build.bat references it for vendored deps)
- `ffmpeg.exe` on PATH (runtime, not build)

---

## Known gotchas (lessons learned, 2026-04-25 → 2026-05-09)

1. **Don't pass `--grpc-addr`** — flag was removed. Old `run_alain/dhafra/minhad.bat` still pass it and fail at launch. Delete those bats if they exist.

2. **`atc.exe` rename trick for hot rebuilds.** Windows locks running `.exe`. `move atc.exe atc.exe.old` lets `build.bat` produce the new one without killing live processes.

3. **SRS handshake order matters.** Tower sends Sync→EAM. Command/Marshal/Deckboss originally sent EAM→Sync, which authenticated but never bound them to SRS's audio routing table. Fixed in `924784f`. If you see "process alive, no UDP voice received" symptoms again, diff the role's register sequence against `srsLoop` in `main.go`.

4. **Marshal/Command/Deckboss need a TCP reader.** Without one, they don't respond to SRS server pings and get dropped from the client list silently. Fixed in `32b8200` for Marshal. If a non-tower role goes invisible, check `netstat` for ESTABLISHED to :5008 from that PID — if missing, the connection died and the role isn't reconnecting.

5. **Tacview Pilot vs Name field.** DCS Tacview's `Name=` is the aircraft TYPE (`F-14B`), `Pilot=` is the player callsign (`Raider 032`). The parser was only reading Name, so radar-check / speed warnings / conflict detection silently failed. Fixed in `6f6b6b7`.

6. **`GLOBAL_LOBBY_FREQUENCIES` overlap.** Setting the lobby freq to an operational freq (e.g. 282.0 when Command is on 282) silently breaks in-jet TX on that freq. Lobby freqs have separate TX gating in SRS. Always set lobby to a freq nothing operational uses (e.g. `285.0`).

7. **`EXTERNAL_AWACS_MODE=false` + `SPECTATORS_AUDIO_DISABLED=false`.** EAM clients sneak in as spectators. Works but fragile. Should be `EXTERNAL_AWACS_MODE=true` and `EAM_BLUE_PASSWORD=blue42` to match `--eam-password`.

8. **Don't hardcode `C:\SkyeyeATC`** in new Go code. Use `os.Executable()` so the binary works from any directory.

9. **Don't commit `OPENAI_API_KEY`** to bat files. Set globally with `setx OPENAI_API_KEY "..."`. The variable is already duplicated across too many bats — centralizing.

---

## Useful commands

```bat
:: List all atc.exe processes with their args
wmic process where "name='atc.exe'" get processid,commandline

:: List ESTABLISHED connections to SRS (one per healthy role)
netstat -ano | findstr ":5008.*ESTABLISHED"

:: Tail today's events filtered for activity
findstr /C:"recognized" /C:"TX via" /C:"intent miss" /C:"warn" /C:"error" C:\SkyeyeATC\logs\atc-omdm.log

:: Disk usage of logs (rotate at 50MB but accumulate)
dir C:\SkyeyeATC\logs

:: Check a specific role's pprof (if --pprof-port set)
curl -s http://localhost:7770/debug/pprof/goroutine?debug=1 | findstr "main\."
```

---

## Reference

- `PILOT_WORKFLOW.md` — pilot-facing flow
- `README_TRAINING1.md` — Training 1 install + ops
- `README_FOOTHOLD.md` — Foothold install + ops
- `CLAUDE.md` — project context for Claude Code
- `*_responses.md` — phraseology spec docs (source of truth for what each role says)
