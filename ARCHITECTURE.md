# SkyeyeATC — Architecture

AI-powered Tower ATC for DCS World (vSFG-7). One `atc.exe` binary, run in
multiple modes via flags — one OS process per radio role. All processes share
the same Go packages and connect to the same SRS, Tacview, DCS-gRPC, and
OpenAI endpoints.

## System diagram

```
                  ┌───────────────── EXTERNAL ─────────────────┐
                  │                                            │
                  │  DCS World mission                         │
                  │     │                                      │
                  │     ├──→ DCS-gRPC          :50051          │
                  │     └──→ Tacview RT        :42676          │
                  │                                            │
                  │  Pilots (SRS clients) ──→  SRS server      │
                  │                              :5008         │
                  │                                            │
                  │  OpenAI API                                │
                  │     • Whisper (gpt-4o-mini-transcribe)     │
                  │     • TTS  (onyx, speed 0.97)              │
                  │     • Chat (Arabic translate for ATIS)     │
                  └────────────────────┬───────────────────────┘
                                       │
            ┌──────────────────────────┼─────────────────────────────┐
            │                          │                             │
            │           atc.exe processes (one OS process per role)  │
            │                          │                             │
            │  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐    │
            │  │ Tower OMDM  │  │ Tower OMAM  │  │ Tower OMAL   │    │
            │  │ 250.1       │  │ 251.1       │  │ 250.7        │    │
            │  │ dash :6001  │  │ dash :6002  │  │ dash :6003   │    │
            │  └──────┬──────┘  └──────┬──────┘  └──────┬───────┘    │
            │         │                │                │            │
            │  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴───────┐    │
            │  │  ATIS-only  │  │ Marshal     │  │  Command     │    │
            │  │ 5 stations  │  │ "Union M."  │  │ "vSFG-7 Cmd" │    │
            │  │ 45s cadence │  │ 306.3 OMDM  │  │ 282.0        │    │
            │  └─────────────┘  └─────────────┘  └──────────────┘    │
            │                                                        │
            │  ── dev_only/ (parked on Training 1) ──                │
            │     Deckboss 306.2   ·   Scudwatch 264.0               │
            └────────────────────┬───────────────────────────────────┘
                                 │
            ┌────────────────────┴────────────────────────────────────┐
            │  Shared Go packages (linked into each atc.exe)          │
            │                                                         │
            │  pkg/airfield   OMDM, OMAM, OMAL — rwy, freqs, elev     │
            │  pkg/state      MarshalStack, weather, deck state       │
            │  pkg/controller intent classify, Tacview lookups,       │
            │                  conflict detect, callsign normalize    │
            │  pkg/composer   ICAO phraseology, 3 variants/intent     │
            │                                                         │
            │  Per-process loop:                                      │
            │   SRS TCP (Sync→EAM→UDP-prime) ──► UDP voice in         │
            │     └─ Whisper ─► controller.Classify ─►                │
            │            composer.<Intent>() ─► TTS ─► SRS UDP out    │
            └────────────────────┬────────────────────────────────────┘
                                 │
                      JSONL ──►  logs/atc-{omdm,omam,omal}.log
                                 │
                                 ▼
                          launcher.exe :7000
                          (start/stop bats, tail logs, dashboard)
```

## Process inventory

| Mode | Flag | Freq | Log file | Dashboard | Bat |
|---|---|---|---|---|---|
| Tower OMDM (Al Minhad) | `--airfield OMDM` | 250.1 | `atc-omdm.log` | :6001 | `start_towers.bat` |
| Tower OMAM (Al Dhafra) | `--airfield OMAM` | 251.1 | `atc-omam.log` | :6002 | `start_towers.bat` |
| Tower OMAL (Al Ain) | `--airfield OMAL` | 250.7 | `atc-omal.log` | :6003 | `start_towers.bat` |
| ATIS (5 stations) | `--atis-only` | 248.2 / 248.3 / 248.5 / 248.55 / 248.85 | tower logs | — | `start_atis.bat` |
| Marshal | `--marshal-only --airfield OMDM` | 306.3 | `atc-omdm.log` | — | `start_marshal.bat` |
| Command | `--command-only` | 282.0 | (own console) | — | `start_command.bat` |
| Launcher | (separate binary) | — | `launcher.log` | :7000 | `start_launcher.bat` |
| Deckboss (parked) | `--airfield OMDM` (Deckboss role) | 306.2 | `atc-omdm.log` | — | `dev_only/start_deckboss.bat` |
| Scudwatch (parked) | `--scudwatch-only` | 264.0 | (own) | — | `dev_only/run_scudwatch.bat` |

## Per-process voice loop

Each non-ATIS process runs the same SRS bridge:

1. **Connect** — TCP dial SRS, send `Sync` (registers callsign + freq), send
   `EAM` (ExternalAWACS auth). Order matters: SRS only adds the client to its
   audio routing table on `Sync`, so EAM-before-Sync silently authenticates
   but receives no UDP voice. See `command_channel_freeze_bug.md`.
2. **UDP prime** — immediately `udpConn.Write(guid)` so SRS learns our UDP
   source address. Without this, the first voice packet doesn't reach us
   until the 10s keepalive ticker fires. UDP socket is created with
   `net.ResolveUDPAddr` + `net.DialUDP` (not `net.Dial`) so Windows binds
   IPv4 — string-form `net.Dial("udp", ...)` resolves to `::1` and SRS
   routes audio to the IPv4 client entry, dropping it.
3. **Keepalive** — UDP-only `Write(guid)` every 10s. The TCP reader replies
   to server pings with a Sync echo; we never re-send Sync ourselves
   (re-sending Sync tears down the routing entry).
4. **Voice in** — UDP packet header is
   `[pktLen(2)] [audioLen(2)] [freqSegLen(2)]`. Read `audioLen` from
   offset 2 (NOT 4 — that's freqSegLen). Frames are grouped by transmission
   GUID and flushed to Whisper when the gap exceeds 400 ms.
5. **STT → intent → TTS** — Whisper text passes through
   `controller.Classify` (callsign extraction, intent matching), then
   `composer.<Intent>()` (ICAO phraseology, 3-variant random pick), then
   OpenAI TTS, then `transmitExternalAudioFile` back to SRS over the
   ExternalAudio file path.

## Shared packages

- **`pkg/airfield`** — airfield definitions (OMDM, OMAM, OMAL). Runways,
  parallel pairs, frequencies, elevations, magnetic variation.
- **`pkg/state`** — `MarshalStack` (carrier marshal queue with
  position/angels/fuel/phase), shared weather + deck state.
- **`pkg/controller`** — intent classification, Tacview contact lookups
  (`LookupCallerRelativeToCarrier`, `GetCarrierBRC`, `GetCarrierPosition`),
  conflict detection, callsign normalization
  (`extractCallsignSkippingAddress`).
- **`pkg/composer`** — every TX string. ICAO phraseology, 3 variants per
  intent picked at random. Composer is constructed with a `towerCallsign`
  (e.g. `"Union Marshal"`, `"vSFG-7-Command"`, `"Al Minhad Tower"`) so the
  same composer method produces role-flavored output across processes.

## Source-of-truth for radio calls

All Marshal / Tower / Command / Deckboss phraseology lives in `*_responses.md`
at the repo root. Composer methods MUST match what's in those docs.
Undocumented TX gets reverted (per saved feedback).

| Role | Spec doc |
|---|---|
| Tower (arrival) | `arrival_responses.md` |
| Tower (departure) | `departure_responses.md` |
| Marshal | `marshal_responses.md` |
| Command | `command_responses.md` |
| Deckboss | `deckboss_responses.md` |

## Known constraints

- **Production rig.** Prefer stable behavior over experimental refactors
  during live training.
- **No hardcoded paths.** `C:\SkyeyeATC` is the Training 1 root; new code
  must use `os.Executable()` / `filepath.Dir(argv[0])` so the launcher works
  from any directory.
- **`OPENAI_API_KEY`** is a global env var (`setx OPENAI_API_KEY ...`). Don't
  duplicate it into `.bat` files.
- **Build with portability in mind** — Training 1 will not be the last rig.
