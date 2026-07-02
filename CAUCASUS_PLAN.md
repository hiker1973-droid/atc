# Caucasus (Black Sea) Map — Design & Rollout Plan

Adds the vSFG-7 **Black Sea AOR** (Caucasus) as a second theatre alongside the
Persian Gulf. Source: `CA KNEEBOARD/` (March 2026). Carrier ops (Marshal /
Deckboss / LSO) are theatre-agnostic and reused as-is.

Status: **airfield definitions written** (`pkg/airfield/ug*.go`). Wiring +
ATIS + start scripts are the next phase, gated on the decisions in §5.

---

## 1. Airfield roster (reconciled)

| Field | ICAO | DCS name | Tower | ATIS | RWY | MAG | Elev ft | Len ft | TACAN | ILS |
|---|---|---|---|---|---|---|---|---|---|---|
| Batumi | UGSB | Batumi | **260.000** | 230.000 | 13/31 | 119/299 | 32 | 6,792 | 16X (BTM) | 13 / **110.30?** |
| Kobuleti | UG5X | Kobuleti | 262.000 | 232.000 | 07/25 | 063/243 | 59 | 7,406 | 67X (KBL) | 07 / 111.50 |
| Senaki-Kolkhi | UGKS | Senaki-Kolkhi | 261.000 | 234.000 | 09/27 | 088/268 | 43 | 7,256 | 31X (TSK) | 09 / 108.90 |
| Kutaisi | UGKO | Kutaisi | 263.000 | 233.000 | 07/25 | 074/247 | 147 | 7,936 | 44X (KTS) | 07 / 109.75 |

Center coords are the TACAN antenna positions from the AOR card; runway
thresholds in the `.go` files are computed from center + heading + length.

**Kneeboard discrepancies found & resolved:**
- **Batumi tower freq** — AOR card says `261.000`, presets card says `260.000`.
  Used **260.000** (presets = the freq pilots actually dial; 261 duplicated Senaki).
- **Batumi ILS** — AOR card says `09 / 108.90`, but Batumi's runway is 13/31 and
  108.90 is Senaki's ILS (copy error). Real DCS Batumi ILS is RWY 13; placeholder
  `110.30` pending confirmation. **VERIFY.**
- **MagVar** — set to `+6.5°E` for all four (Caucasus); confirm against DCS.

## 2. Frequencies

- **Command**: 282.000 — same as PG, so the departure handoff target is unchanged.
  On Black Sea, Command is **COMM1 preset 1** (`channel one`), vs PG's `channel four`.
  Airfield `HandoffPreset` set to "channel one" accordingly.
- **ATIS**: 230 (Batumi), 232 (Kobuleti), 233 (Kutaisi), 234 (Senaki).
- No conflicts once Batumi = 260 (each tower and ATIS is unique).

## 3. ATIS station list (for the ATIS wiring)

Each paired-tower station mirrors its tower's live runway (same as PG). ILS/TACAN
strings live on the ATIS station, not the `Airfield`:

| Name | Freq | ICAO | TACAN | ILS |
|---|---|---|---|---|
| Batumi ATIS | 230.000 | UGSB | TACAN 16X. | ILS 110.30 runway 13. |
| Kobuleti ATIS | 232.000 | UG5X | TACAN 67X. | ILS 111.50 runway 07. |
| Kutaisi ATIS | 233.000 | UGKO | TACAN 44X. | ILS 109.75 runway 07. |
| Senaki ATIS | 234.000 | UGKS | TACAN 31X. | ILS 108.90 runway 09. |

## 4. Carrier ops — freq remap (Black Sea)

The carrier roles are reused unchanged; only the freqs differ. Black Sea CVN-72
plan (from Common Freq/Nav card) vs current PG bat config:

| Role (SkyeyeATC) | PG freq | Black Sea freq | Notes |
|---|---|---|---|
| Marshal | 306.300 | **306.100** (LIVE MARSHALL) / **306.300** is "AI Airboss" | see §5 |
| Deckboss | 128.600 | **306.200** | 128.600 is "LSO (AI)" on Black Sea |
| Command | 282.000 | 282.000 | unchanged |

This is a **bat-file config change only** (per-role `--marshal-freq` /
`--deckboss-freq`), no code — but the role↔freq mapping needs a decision (§5).

## 5. Open decisions (before wiring)

1. **Wiring approach** — how to add the fields to the process:
   - *(recommended)* **Airfield registry** — replace the `switch flagAirfield`
     (`cmd/atc/main.go:276`), the hardcoded ATIS station list
     (`cmd/atc/atis.go`), and the hardcoded `{OMDM,OMAM,OMAL}` handoff scan
     (`cmd/atc/command_handoff.go:228`) with a `map[string]*Airfield` keyed by
     ICAO plus a `--map pg|caucasus` flag selecting the active set. Collapses
     three hardcoded lists into one and makes map #3 trivial.
   - *(quick)* Just extend each hardcoded list with the CA ICAOs and add a CA
     ATIS branch. Less clean, faster.
2. **Carrier freq mapping** — does "Marshal" run on 306.100 or 306.300 on Black
   Sea, and does "Deckboss" move to 306.200 (with 128.600 reserved for the
   future AI LSO)? Bat config only.
3. **Confirm the flagged data** — Batumi ILS runway/freq, MagVar, and whether
   break directions are left-pattern at all four fields.

## 6. Remaining work (after decisions)

- [ ] Wire the four `Airfield` vars into the selector (registry or switch).
- [ ] Dashboard ports: extend `towerDashboardPortByICAO` (UGSB→6001… or a CA range).
- [ ] CA ATIS station set (Batumi/Kobuleti/Kutaisi/Senaki) — 45s cadence, staggered.
- [ ] `start_towers_caucasus.bat` + `start_atis` CA set + carrier freq overrides;
      or a `SKYEYE_MAP` env var driving `start_all.bat`.
- [ ] Point `SKYEYE_MIZ` at the Caucasus mission `.miz` when running Black Sea.
- [ ] Live validation on a CA mission (runway selection, ATIS mirror, handoff).

## 7. What's reused unchanged

State machine, conflict detection, composer/phraseology, Marshal/Deckboss/(future
LSO) carrier ops, weather (`.miz` via `SKYEYE_MIZ`), SRS/Tacview/Whisper/TTS.
