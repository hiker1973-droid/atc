# vsfg7-gci — Custom GCI Development Plan

Custom Ground Control Intercept controller for vSFG-7, built natively into the `atc.exe` binary.
Replaces SkyEye with a system tuned to our ops, our callsigns, and our shared ATC state.

---

## Why custom vs SkyEye

| | SkyEye | vsfg7-gci |
|---|---|---|
| Callsign | Darkstar / Overlord | vSFG-7 defined |
| Voice | Fixed | Our OpenAI TTS pipeline |
| Radio effect | None | Our bandpass/compressor chain |
| ATC awareness | None | Shared state — knows pattern, marshal stack |
| Dashboard | None | Live contact feed in monitor |
| IADS integration | None | Planned SkyNet data feed |
| Deployment | Separate binary | Built into atc.exe |
| Configuration | YAML file | Our existing flag/config pattern |

---

## Callsigns (proposed)

| Role | Callsign | Freq | VM |
|---|---|---|---|
| Primary GCI (Dev) | **Darkstar** | 264.0 AM | Dev 192.168.1.221 |
| Secondary GCI (Foothold) | **Overlord** | 266.0 AM | Foothold 192.168.1.222 |

> Callsigns TBD — update before Phase 1 build.

---

## Architecture

```
Tacview real-time telemetry (port 42676)
        │
        ▼
gciLoop (goroutine in atc.exe)
        │
   contact tracker ──► coalition classifier ──► threat assessor
        │
   intent parser (pilot SRS calls)          proactive push engine
        │                                          │
        └──────────────┬───────────────────────────┘
                       ▼
               response composer
                       │
             OpenAI TTS ──► radio EQ ──► SRS TX
                       │
               dashboard SSE feed
```

### Shared state with ATC

```
AirfieldState (existing)
        │
        ├── LandingQueue  ◄── GCI checks before vectoring inbounds
        ├── ActiveRunway  ◄── GCI deconflicts approach corridor
        └── MarshalStack  ◄── GCI holds intercepts during recovery ops

GCIState (new)
        ├── ContactTracker   — all air contacts by coalition
        ├── ThreatList       — hostile/unknown contacts with TTI
        ├── CommittedPairs   — friendly/hostile pairs being worked
        └── SectorOwnership  — which GCI owns which airspace
```

---

## Phase 1 — Core GCI (Reactive)

**Goal:** Pilot asks, GCI answers. Basic picture, bogey dope, declare.

### Intent types

| Intent | Pilot says | GCI responds |
|---|---|---|
| Radio check | `"Darkstar, radio check"` | Loud and clear |
| Check in | `"Darkstar, Raider 1-1, checking in"` | Radar contact, say altitude |
| Bogey dope | `"Darkstar, bogey dope"` | BRAA to nearest hostile |
| Picture | `"Darkstar, picture"` | Threat count and general azimuth |
| Declare | `"Darkstar, declare [bearing]"` | Hostile / Friendly / Unknown |
| Buddy spike | `"Darkstar, buddy spike"` | Friendly ID confirmation |
| Snap | `"Darkstar, snap [callsign]"` | BRAA to specified contact |
| Disengage | `"Darkstar, disengage"` | Disengage, break [direction] |

### BRAA format
```
"Raider 1-1, Darkstar — Bogey, BRAA 045 for 35, angels 15, hot, single group."
```

### Bullseye format (Phase 2+)
```
"Raider 1-1, Darkstar — group bullseye 270/40, angels 18, track west."
```

### Deliverables
- [ ] `pkg/gci/tracker.go` — contact tracking from Tacview telemetry
- [ ] `pkg/gci/classifier.go` — coalition/threat classification
- [ ] `pkg/gci/braa.go` — BRAA and bullseye geometry calculations
- [ ] `pkg/gci/composer.go` — GCI response strings
- [ ] `cmd/atc/gci.go` — gciLoop, SRS integration, intent parser
- [ ] `--gci-freq` flag — enable GCI on frequency (e.g. 264.0)
- [ ] `--gci-callsign` flag — GCI callsign (default: Darkstar)
- [ ] `--gci-voice` flag — TTS voice (default: onyx)
- [ ] `--bullseye-lat`, `--bullseye-lon` flags — bullseye reference point

---

## Phase 2 — Enhanced GCI

**Goal:** Richer picture calls, commit/abort, threat warnings.

### Additional intents

| Intent | Pilot says | GCI responds |
|---|---|---|
| Commit | `"Darkstar, commit"` | Commit, [target BRAA], press |
| Abort | `"Darkstar, abort"` | Abort, [safe heading] |
| Music | `"Darkstar, music on / music off"` | Radar on/off acknowledgement |
| Bingo vector | `"Darkstar, bingo"` | Vector to nearest friendly airfield |
| Fence check | `"Darkstar, fence check"` | Current threat picture summary |

### Proactive calls (push)
- **Threat warning** — unprompted BRAA when hostile closes within configurable range
- **Mud spike** — SAM radar lock warning (if SkyNet telemetry available)
- **Faded** — contact lost notification
- **Clean** — sector clear call

### Configurable thresholds
```
--gci-threat-range-nm    Proactive call range (default 40nm)
--gci-push-interval      Min seconds between unprompted calls (default 30)
```

### Deliverables
- [ ] `pkg/gci/threat.go` — threat assessment and TTI calculation
- [ ] `pkg/gci/push.go` — proactive call engine with cooldown
- [ ] `pkg/gci/sector.go` — sector ownership and airspace deconfliction
- [ ] Commit/abort response logic
- [ ] Bingo vector routing to nearest friendly field

---

## Phase 3 — ATC + GCI Integration

**Goal:** GCI and ATC are aware of each other — no conflicting instructions.

### Integration points

| Scenario | Behavior |
|---|---|
| Inbound on GCI intercept, pattern is full | GCI holds commit until pattern clears |
| Marshal stack active | GCI deconflicts recovery corridor — no vectors into 10nm |
| Emergency declared on tower | GCI clears sector, broadcasts traffic advisory |
| Pilot handed off from tower to command | GCI auto checks in pilot at fence |

### Deliverables
- [ ] `GCIState` linked to `AirfieldState` via shared pointer
- [ ] Pattern corridor exclusion zone
- [ ] Marshal stack protection radius
- [ ] Cross-channel pilot tracking (tower callsign = GCI contact ID)
- [ ] Dashboard GCI tab — live contact feed with BRAA overlays

---

## Phase 4 — IADS Integration (stretch)

**Goal:** SkyNet IADS data feeds GCI with SAM threat picture.

- Read SkyNet emitter state from Tacview or DCS-gRPC
- Correlate SAM radar activity with air contacts
- Generate mud spike / SAM spike calls
- Integrate with SkyEye's existing IADS hook if available

---

## File structure (target)

```
cmd/atc/
  gci.go              # gciLoop, flag handling, SRS TX

pkg/gci/
  tracker.go          # Contact struct, ContactTracker, Tacview feed
  classifier.go       # Coalition detection, IFF, friendly/hostile/unknown
  braa.go             # Bearing, range, altitude, aspect math
  bullseye.go         # Bullseye reference point calculations
  composer.go         # GCI response string generation
  threat.go           # Threat assessment, TTI, sort order
  push.go             # Proactive call engine, cooldown management
  sector.go           # Sector ownership, airspace deconfliction
  state.go            # GCIState struct
```

---

## Startup flags (Phase 1 target)

```bat
:: Dev VM — Darkstar on 264.0
atc.exe --airfield OMDM ... --gci-freq 264.0 --gci-callsign Darkstar --gci-voice onyx

:: Foothold VM — Overlord on 266.0
atc.exe --airfield OMDM ... --gci-freq 266.0 --gci-callsign Overlord --gci-voice echo
```

---

## Sample exchanges (target)

### Bogey dope
```
Pilot:    "Darkstar, Raider 1-1, bogey dope."
Darkstar: "Raider 1-1, Darkstar — bogey, BRAA 035 for 42, angels 18, hot, single group."
```

### Picture
```
Pilot:    "Darkstar, picture."
Darkstar: "Raider 1-1, picture — two groups. Lead group BRAA 040 for 38, angels 20, track west.
           Trail group BRAA 055 for 52, angels 15, track southwest."
```

### Proactive threat warning
```
Darkstar: "Raider 1-1, Darkstar — threat. BRAA 010 for 28, angels 22, hot. Recommend hard left."
```

### Bingo vector
```
Pilot:    "Darkstar, bingo."
Darkstar: "Raider 1-1, Darkstar — vector Al Minhad, heading 185, 47 miles. Altimeter 29.88."
```

### Declare
```
Pilot:    "Darkstar, declare, 045."
Darkstar: "Raider 1-1, Darkstar — hostile."
```

---

## Development order

```
Phase 1 — Core reactive GCI
  1. pkg/gci/tracker.go       wire up Tacview, populate ContactTracker
  2. pkg/gci/classifier.go    coalition/IFF classification
  3. pkg/gci/braa.go          BRAA and bullseye geometry
  4. pkg/gci/composer.go      response strings
  5. cmd/atc/gci.go           gciLoop, SRS, intent parser, flags

Phase 2 — Enhanced + proactive
  6. pkg/gci/threat.go        TTI, sort, threat assessment
  7. pkg/gci/push.go          proactive engine
  8. pkg/gci/sector.go        airspace deconfliction

Phase 3 — ATC integration
  9. Shared state linkage
  10. Dashboard GCI tab

Phase 4 — IADS (stretch)
  11. SkyNet integration
```

---

## Technical notes

- Tacview already connected in `atc.exe` — reuse existing connection, do not open a second
- Coalition detection: DCS exports pilot coalition in Tacview `Color` property (`Blue` / `Red`)
- BRAA needs aircraft true heading from Tacview for aspect (`hot` / `flanking` / `beaming` / `dragging`)
- Bullseye reference point for Persian Gulf ops TBD — set to central Gulf reference
- Whisper prompt for GCI: `Darkstar, Overlord, bogey dope, picture, declare, commit, abort, BRAA, bullseye, hostile, friendly, unknown`
- GCI TX cooldown independent from tower cooldown (same pattern as marshal/deckboss independent cooldowns)
- GCI loop runs as goroutine alongside tower loops — started via `--gci-freq` flag
