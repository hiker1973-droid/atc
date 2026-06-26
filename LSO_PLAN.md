# vsfg7-lso — LSO Development Plan
## v1.3 Feature Branch

Custom Landing Signal Officer for CVN-72 carrier operations.
Replaces Tactical Paddles with a fully integrated LSO built into `atc.exe`.
Requires DCS-gRPC re-enabled on all VMs.

---

## Prerequisites — Complete before LSO coding starts

### 1. Re-enable DCS-gRPC

Current status: disabled on all VMs due to LotATC Export.lua conflict.

**Fix per VM:**
```yaml
# In DCSServerBot nodes.yaml — comment out LotATC extension block
# extensions:
#   LotATC:
#     ...

# In DCS Saved Games/Scripts/Export.lua — keep hook manually
# do not let DCSServerBot regenerate this file
# lock read-only after edit: attrib +R Export.lua
```

Port assignments (do not change):
| VM | gRPC Port |
|---|---|
| Training | 50051 |
| Foothold | 50052 |
| Dev | 50053 |

Bind to `0.0.0.0` not `127.0.0.1` in DCS-gRPC config.

### 2. Verify Tactical Paddles coexistence

Tactical Paddles connects as a gRPC **client** — it does not own the port.
Multiple clients can read the same gRPC stream simultaneously.

```bat
:: While DCS running — verify gRPC is up and Paddles still connects
netstat -ano | findstr :50053
```

### 3. Test gRPC data for LSO-required fields

Before building LSO, verify gRPC exposes:
- [ ] Aircraft position (lat/lon/alt)
- [ ] Aircraft AoA
- [ ] Gear state (up/down)
- [ ] Hook state (up/down)
- [ ] Wire caught (1-4) or bolter
- [ ] Carrier position and heading (BRC)
- [ ] DCS weather (ceiling, visibility, wind)

---

## Architecture

```
DCS-gRPC (:50053)
        │
        ├──► Tactical Paddles (existing client — unchanged)
        │
        └──► lsoLoop (new goroutine in atc.exe)
                │
          approach monitor
                │
         ┌──────┴──────┐
         │             │
    glideslope      lineup
    geometry        geometry
         │             │
         └──────┬───────┘
                │
         deviation calc
                │
         grade accumulator
                │
         ┌──────┴──────────────┐
         │                     │
    radio calls (SRS TX)   grade logger (PostgreSQL)
```

### Shared state with existing systems

```
LSOState
    │
    ├──► MarshalStack     — knows who is in the stack / commencing
    ├──► DeckbossState    — knows which cat just launched
    └──► AirfieldState    — weather, altimeter, active runway
```

---

## File structure

```
cmd/atc/
  lso.go              # lsoLoop, gRPC feed, SRS TX, flag wiring

pkg/lso/
  state.go            # LSOState, PassState, GradeRecord structs
  geometry.go         # Glideslope math, lineup deviation, carrier-relative coords
  grades.go           # Grade determination logic (OK/Fair/Cut/WO/Bolter)
  composer.go         # LSO radio call strings
  logger.go           # PostgreSQL grade logging
```

---

## New flags — `atc.exe`

| Flag | Default | Description |
|---|---|---|
| `--lso-freq` | `0` | Enable LSO on this frequency MHz (e.g. 128.100) |
| `--lso-callsign` | `Paddles` | LSO callsign |
| `--lso-voice` | `nova` | TTS voice — nova recommended (softer tone) |
| `--lso-grpc-addr` | `127.0.0.1:50053` | DCS-gRPC address |
| `--lso-db-conn` | — | PostgreSQL connection string for grade logging |
| `--lso-auto-monitor` | `true` | Auto-watch all approaches, no check-in required |

---

## State machine

```
IDLE
  │
  │  aircraft > 3nm, descending toward carrier
  ▼
MONITORING
  │
  │  aircraft 3/4 mile, gear down, hook down
  ▼
BALL CALL
  │  LSO: "Raider 1-1, Hornet ball."
  │  Pilot: "Raider 1-1, Hornet ball, 4.2."
  │  LSO: "Roger ball."
  ▼
IN CLOSE  (real-time corrections every 2-3s)
  │
  ├──► WAVE OFF (dangerous deviation)
  │      LSO: "Wave off, wave off."
  │      └──► IDLE (reset after 30s)
  │
  ├──► BOLTER (missed all wires)
  │      LSO: "Bolter, bolter, bolter."
  │      └──► IDLE (reset)
  │
  └──► TRAPPED (wire caught)
         LSO: grade call + wire number
         └──► grade logged ──► IDLE (reset)
```

---

## Glideslope geometry

Carrier glidepath: **3.5°** for F/A-18C.

```
Ideal altitude at range X (nm) = X × 6076 × tan(3.5°) feet

Deviation = actual_alt_AGL - ideal_alt
```

### Correction thresholds

| Deviation | Call |
|---|---|
| > +40ft | "You're high — wave off" |
| +20 to +40ft | "Easy with it" |
| ±10ft | On glidepath — no call |
| -10 to -20ft | "Power" |
| < -20ft | "Power — wave off" |

### Lineup thresholds

| Deviation | Call |
|---|---|
| > 15ft right | "Come left" |
| > 15ft left | "Right for line" |
| > 30ft either | "Wave off — line up" |

### AoA thresholds (F/A-18C)

| AoA | Call |
|---|---|
| > 9.5° | "Slow" |
| 7.4–9.5° | On speed (no call) |
| < 7.4° | "Fast" |

---

## Grade determination

Grades assigned after pass based on accumulated deviations:

| Grade | Symbol | Criteria |
|---|---|---|
| Underlined OK | \_OK\_ | Perfect pass, no deviations |
| OK | OK | Minor deviations, corrected |
| Fair | F | Significant deviations, safe |
| No grade | \_ | Below standards |
| Cut | C | Unsafe deviation, dangerous |
| Wave off (LSO) | WO | LSO initiated wave off |
| Bolter | BOLTER | Missed all wires |
| Pattern Wave off | PWO | Waved off before groove |

### Deviation notes (appended to grade)
```
"power"        — low in close
"high at start" — high at 3/4 mile
"lined up left" — lineup deviation
"fast"         — above on-speed AoA
"LIG"          — little in glidepath (low)
"AFU"          — all fouled up (multiple deviations)
```

---

## Radio calls

### Ball call
```
LSO:   "Raider 1-1, Hornet, ball."
Pilot: "Raider 1-1, Hornet ball, 4.2."
LSO:   "Roger ball. [wind X down the angle]"
```

### In-close corrections
```
"Power."
"Easy with it."
"Come left."
"Right for line up."
"You're high."
"Slow."
"Fast."
```

### Wave off
```
"Wave off, wave off."          — standard
"Wave off — power."            — low wave off
"Wave off — line up."          — lineup wave off
"Bolter, bolter, bolter."      — missed wires
```

### Grade calls (after trap)
```
"OK three wire."
"Fair pass, two wire — little power in close."
"No grade, one wire — high start, lined up left."
"Cut pass — dangerous, see me on the deck."
```

### Case III (night/IMC)
```
"Raider 1-1, say needles."
"Fly your needles."
"Three quarters of a mile, call the ball."
```

---

## Grade logging — PostgreSQL schema

```sql
CREATE TABLE lso_grades (
    id           SERIAL PRIMARY KEY,
    timestamp    TIMESTAMPTZ DEFAULT NOW(),
    callsign     TEXT NOT NULL,
    aircraft     TEXT,
    wire         INT,           -- 1-4, 0=bolter
    grade        TEXT NOT NULL, -- OK, F, C, WO, BOLTER, PWO
    grade_symbol TEXT,          -- _OK_, OK, (OK), F, etc.
    notes        TEXT,          -- deviation notes
    fuel_state   FLOAT,
    wind_kts     FLOAT,
    wind_dir     INT,
    case_type    INT,            -- 1, 2, 3
    carrier_brc  INT,
    session_id   TEXT            -- DCS mission name/time
);

CREATE INDEX idx_lso_callsign ON lso_grades(callsign);
CREATE INDEX idx_lso_timestamp ON lso_grades(timestamp);
```

---

## Dashboard integration

Add LSO tab to `vSFG7_Monitor.html`:

- Live approach status (who is in the groove)
- Last 10 grades per session
- Per-pilot grade summary
- Wire distribution chart

Add to `/status` endpoint:
```json
{
  "lso": {
    "active": true,
    "inGroove": "Raider 1-1",
    "state": "IN_CLOSE",
    "lastGrade": { "callsign": "Venom 2-2", "grade": "OK", "wire": 3, "notes": "" },
    "sessionGrades": [...]
  }
}
```

---

## Startup bat update

```bat
:: Add to start_vsfg7.bat — carrier ops VM
start "Deckboss + LSO" cmd /k "atc.exe --airfield OMDM --srs-addr %SRS% --eam-password %EAM% --deckboss-freq 306.2 --lso-freq 128.100 --lso-callsign Paddles --lso-voice nova --lso-grpc-addr 192.168.1.221:50053 --no-atis --log-level %LOG%"
```

---

## Development order

```
Step 1 — gRPC re-enable + validation
  - Re-enable DCS-gRPC on Dev VM
  - Verify Tactical Paddles still connects
  - Confirm all required fields available in gRPC feed
  - Write gRPC test client to log carrier/aircraft data

Step 2 — pkg/lso/state.go
  - LSOState struct
  - PassState struct (per-approach tracking)
  - GradeRecord struct
  - State machine transitions

Step 3 — pkg/lso/geometry.go
  - Carrier-relative coordinate system
  - Glideslope deviation calculation
  - Lineup deviation calculation
  - AoA check
  - Moving carrier compensation (BRC tracking)

Step 4 — pkg/lso/grades.go
  - Deviation accumulator
  - Grade determination logic
  - Deviation note generation

Step 5 — pkg/lso/composer.go
  - Ball call strings
  - Correction call strings
  - Wave off strings
  - Grade call strings
  - Case I / II / III variants

Step 6 — pkg/lso/logger.go
  - PostgreSQL connection
  - Grade insert
  - Session management

Step 7 — cmd/atc/lso.go
  - lsoLoop goroutine
  - gRPC feed subscription
  - SRS TX integration
  - Flag wiring
  - Independent TX cooldown

Step 8 — Dashboard LSO tab
  - Live approach status
  - Grade feed SSE
  - Per-pilot summary
```

---

## Known challenges

| Challenge | Notes |
|---|---|
| Moving carrier | BRC changes as carrier turns — geometry must update in real time |
| gRPC wire detection | Test if wire number is exposed — may need to infer from deceleration |
| Call timing | Corrections must fire within 2-3s of deviation — gRPC update rate critical |
| Multi-aircraft | Two aircraft in the groove simultaneously — state machine must be per-contact |
| Case II/III | Night/IMC adds ACLS calls — needs ceiling/vis from gRPC weather |
| Bolter detection | Aircraft goes to full power after wire — distinguishable from touch and go |
| Tactical Paddles overlap | Both LSO and Paddles may call simultaneously — consider muting Paddles on same freq |

---

## What this replaces vs Tactical Paddles

| Feature | Tactical Paddles | vsfg7-lso |
|---|---|---|
| Ball call | ✅ | ✅ |
| In-close corrections | ✅ Human only | ✅ Automated |
| Wave off | ✅ | ✅ |
| Grade logging | ✅ Cloud | ✅ Local PostgreSQL |
| Discord posting | ✅ | Planned v1.4 |
| Web grade viewer | ✅ paddles.contact | ✅ Monitor dashboard |
| Human LSO override | ✅ | ❌ Not in v1.3 |
| Marshal integration | ❌ | ✅ |
| Deckboss integration | ❌ | ✅ |
| ATC weather awareness | ❌ | ✅ |
| Same voice as towers | ❌ | ✅ |

---

## Version targets

| Version | Scope |
|---|---|
| v1.3 | LSO — ball call, corrections, wave off, grade logging, dashboard tab |
| v1.3.1 | Case II/III support, ACLS calls |
| v1.4 | GCI Phase 1 (Darkstar/Overlord reactive), JTAC Phase 1 |
| v1.5 | GCI Phase 2 (proactive threat calls), JTAC dynamic with gRPC targets |
| v1.6 | ATC + GCI + LSO full integration, shared state |
