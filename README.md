# vsfg7-atc

AI-powered Tower ATC for DCS World — Al Minhad, Al Dhafra, Al Ain.
Built on SkyEye's infrastructure (SRS, Whisper STT, Piper TTS, Tacview telemetry).

**Copyright © 2026 vSFG-7. All rights reserved.** Proprietary — see [`LICENSE`](LICENSE) for terms. Internal vSFG-7 use only; no external redistribution or reuse without written permission.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.26+ | Same version SkyEye uses |
| SkyEye source | Cloned at `../skyeye` relative to this repo |
| DCS-gRPC | Running on training VM port 50051 |
| SRS | Running on training VM port 5008 |
| Tacview real-time telemetry | Port 42674 (default) |
| OpenAI API key | For Whisper STT |

---

## Directory Layout

```
C:\
  Skyeye\          ← SkyEye clone (Wizard 1-1 GCI runs from here)
  SkyeyeATC\       ← this repo
    cmd\atc\
    pkg\
      airfield\    ← OMDM, OMAM, OMAL definitions
      state\       ← ATC state machine
      controller\  ← intent parser + conflict detection
      composer\    ← ICAO phraseology generator
    configs\       ← per-airfield YAML configs
    build.bat
    run_minhad.bat
    run_dhafra.bat
    run_alain.bat
```

---

## Build

```bat
cd C:\vsfg7\vsfg7-atc
build.bat
```

This produces `atc.exe` in the current directory.

---

## Run

Set your OpenAI API key in each `.bat` file, then run one per airfield:

```bat
REM Terminal 1 — Al Minhad Tower (118.550 MHz)
run_minhad.bat

REM Terminal 2 — Al Dhafra Tower (126.800 MHz)
run_dhafra.bat

REM Terminal 3 — Al Ain Tower (119.850 MHz)
run_alain.bat
```

All three instances share the same SRS server and gRPC server.
Each binds to its own tower frequency independently.

---

## DCS-gRPC Setup (Training VM)

Confirm gRPC is enabled in `MissionScripting.lua` desanitisation and in
your DCSServerBot `nodes.yaml`:

```yaml
extensions:
  grpc:
    enabled: true
    port: 50051
```

---

## Tacview Real-Time Telemetry

The ATC controller uses Tacview telemetry to track aircraft positions for
conflict detection (hold short, line up and wait, proactive go-arounds).

In DCS → Options → Special → Tacview:
- Enable real-time telemetry
- Port: 42674
- Password: (leave blank or set --tacview-pwd flag)

---

## Conflict Detection Thresholds

| Zone | Distance | Action |
|---|---|---|
| Hold Short | < 15 nm inbound | Departure held at hold short |
| Line Up and Wait | 15–20 nm inbound | Enter runway, hold for takeoff |
| Cleared for Takeoff | > 20 nm | No conflict — clear immediately |
| Proactive Go-Around | < 5 nm inbound + departure rolling | Tower calls go-around |

---

## Airfield Reference

| ICAO | Name | Runway | Heading | Tower MHz | Elev ft |
|---|---|---|---|---|---|
| OMDM | Al Minhad | 09/27 | 090°/270° | 118.550 | 190 |
| OMAM | Al Dhafra | 13L/31R + 13R/31L | 128°/308° | 126.800 | 52 |
| OMAL | Al Ain | 01/19 | 010°/190° | 119.850 | 814 |

MagVar: +2.25°E (all three, per AFD-3351 July 2020)

---

## Adding More Airfields

1. Create `pkg/airfield/XXXX.go` following the pattern of `omdm.go`
2. Add to `AllAirfields` slice in `pkg/airfield/omal.go`
3. Add case to switch in `cmd/atc/main.go`
4. Add config in `configs/`
5. Add run script

