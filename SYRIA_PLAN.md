# Syria (Eastern Med) Map — Design & Rollout Plan

Adds the vSFG-7 **Eastern Med AOR** (Syria) as a fifth theatre alongside Persian Gulf,
Caucasus, Germany and Iraq. Source of frequencies: the ratified
**"Hornet Radio Presets — Eastern Med"** card (`Syria\Foothold\00_vSFG7_Comm_Presets_Hornet_Syria.jpg`).

Status: **IMPLEMENTED on `feat/syria-map`. Not yet flown.**
8 airfields, registry wiring, ATIS stations, launcher + dashboard, configs and start
scripts. `go build`, `go vet` and `go test ./...` all pass. `main` is untouched.

> Deployment target: the **Foothold VM** `192.168.1.222`, **SRS `localhost:5002`**
> (confirmed 2026-08-29 — note this differs from Training's 5008 and from the
> `configs/*.yaml` currently in the repo, which all say 5008).

---

## 1. Airfield roster

Eight fields — the ones the squadron card actually briefs. **Tower frequencies are the
card, verbatim.** Navaids, headings and positions are read out of the DCS install's
`Mods\terrains\Syria\beacons.lua` (151 records, all carrying `positionGeo`), never typed.

| # | Field | ICAO | Tower | ATIS | ILS | RWY (ILS) | TACAN | VOR/NDB |
|---|---|---|---|---|---|---|---|---|
| 1 | Incirlik | LTAG | **360.100** | 360.200 | 111.700 / 109.300 | **05 / 23** | 21X (DAN) | — |
| 2 | Ramat David | LLRD | **251.300** | 256.150 | 111.100 | **15** | 84X (RMD)¹ | NDB 368 RMD |
| 3 | King Hussein / Mafraq | OJMF | **250.450** | 255.550 | 111.700 | **13** | 106X (ABC)¹ | — |
| 4 | Hatay | LTDA | **250.300** | **249.300** ² | 108.900 | **22** | — | VOR 112.05 HTY · NDB 336 |
| 5 | Gaziantep | LTAJ | **250.100** | **249.400** ² | 109.100 | **11** | — | VOR 116.70 GAZ · NDB 432 |
| 6 | RAF Akrotiri | LCRA | **252.000** | **249.500** ² | 109.700 | **10 / 28** ⁴ | 107X (AKR) | NDB 365 AK |
| 7 | Paphos | LCPH | **249.100** ³ | 249.000 | 108.900 | **11** | 79X (PHA) | VOR 117.90 · NDB 328 |
| 8 | **H4** | OJHR | **252.250** | 240.850 | — | — | — | **none** ⚠ |

¹ VORTAC, so the channel is the paired TACAN channel, not a separate beacon.
³ **Moved off TEXACO 1's 252.100** (user ruling 2026-08-29) to sit beside its own
ATIS on 249.000. The tanker keeps 252.100 — the squadron-wide standard.

² **Assigned, not from the card** — the card gives these three tower-only. Picked from
the 249.x family so they sit with the card's own Paphos ATIS 249.000, and checked
≥ 0.3 MHz clear of all 93 frequencies already in play in the mission.

### Runway headings
Derived from each ILS **localizer's own `direction`** field (true), not from memory:
Incirlik 55°/235°, Ramat David 146°, Mafraq 132°, Hatay 224°, Gaziantep 106°,
Akrotiri 111°, Paphos 114°. Syria magvar is ≈ **+5.5°E**, so subtract ~5° for the
magnetic designator.

### ATIS second language — set PER FIELD, not per theatre

Every other theatre broadcasts English plus one language for the whole map. The
eight Eastern Med fields sit in four countries, so Arabic — the theatre default
that Syria would otherwise inherit from the PG/Iraq pattern — is genuinely local
at only two of them. Each station therefore carries its own `Lang`:

| Field | Second language | Why |
|---|---|---|
| Incirlik, Hatay, Gaziantep | **Turkish** | Turkey |
| Ramat David | **Hebrew** | Israel |
| Paphos | **Greek** | Cyprus |
| King Hussein / Mafraq, H4 | **Arabic** | Jordan |
| RAF Akrotiri | **English only** | UK Sovereign Base Area — operates in English |

English is always broadcast first at every field, unchanged. `Lang: "English"`
is how a station says "no second pass"; an empty `Lang` still falls back to
`atisSecondLangForMap`, so the other four theatres are untouched.

---

## 2. Three things to resolve before coding

**✅ 2a. RESOLVED — the field is H4 (OJHR).** User confirmed 2026-08-29. Original finding: The card lists `H3 252.250` and
`H3 ATIS 240.850`, but DCS Syria's Jordan section is `OJMF Mafraq` and **`OJHR — H4
Airbase`**. CombatWombat's index has no H3 either, and `beacons.lua` contains the string
"H3" **zero** times. H3 is an Iraqi field, off-map to the east.

So one of these is true and it changes the roster:
- the card means **H4 (OJHR)** and the label is wrong; or
- the card means a Syrian field under a squadron nickname; or
- it is a leftover from an Iraq-map card.

**H4 also has no ILS, TACAN, VOR or NDB in `beacons.lua` at all** — it is a bare desert
strip. It can still have Tower + ATIS, but its ATIS will report no approach aids.

**✅ 2b. RESOLVED — Akrotiri is 10/28.** ⁴ You called "go with true", which off the
localizer bearing alone gives 11. I then found the surveyed AIRFIELD SUMMARY, which
states **RWY 10/28 with the ILS on 28 (109.70 / 291°)** — matching the real RAF Akrotiri.
The chart is the authoritative document, so the build uses **10/28**; flipping it is a
two-line change in `tools/gen_syria_airfields.py` if you disagree. Original finding: The localizer says 111° true (≈106° mag →
RWY 11), but RAF Akrotiri is really **10/28**. One of the two is off by a designator.
This is the same class of error the Caucasus port caught three of, so I would rather
check it than ship it.

**✅ 2c. RESOLVED — Paphos Tower moved to 249.100.** Original finding: Both are 252.100 on the card itself —
COMM1 CH19 against COMM2 CH8. Harmless in the Hornet (different radios) but it means a
SkyEye Paphos Tower would transmit on the tanker frequency. Recommend moving **Paphos
Tower** rather than the tanker, since 252.1 is the squadron-wide tanker standard.

---

## 3. Carrier roles — CONFIRMED ON

The squadron is standing up Marshal / Deckboss / LSO for Syria (user ruling 2026-08-29),
so the card's carrier block is authoritative and the **mission has been edited to match**:

| Role | Freq | Provided by |
|---|---|---|
| CVN-72 AI ATC | 128.600 | the `.miz` — ship moved off Foothold's stock 272.000 |
| LIVE MARSHALL | 306.100 | **SkyEye** |
| DECKBOSS | 306.200 | **SkyEye** |
| LIVE LSO | 128.100 | **SkyEye** |

Carrier navaids: **ABE / TACAN 72X / ICLS 12 / Link 4 336.000 / ACLS on.**

These differ from PG (Marshal 306.300 there), so the carrier role configs need a Syria
variant rather than reuse — same situation `CAUCASUS_PLAN.md` §4 describes.

---

## 4. Command / handoff

`COMMAND` is **282.000**, the same as PG and Caucasus, so the departure handoff target is
unchanged. On the Eastern Med card COMMAND is **COMM1 preset 4**, so `HandoffPreset`
should read `"channel four"` (PG is also channel four; Caucasus is channel one).

⚠ Nothing in the mission transmits on 282.000 — it is a human/SkyEye net only.

---

## 5. Still open

1. ~~H3 vs H4~~ — **resolved: H4 (OJHR)**.
2. ~~Akrotiri runway~~ — **resolved: 10/28 per the surveyed chart** (§2b).
3. ~~Paphos vs TEXACO 1~~ — **resolved: Paphos Tower 249.100**.
4. **Elevations, runway lengths and threshold lat/lons** are not yet extracted. The
   localizer/glideslope positions in `beacons.lua` give a good threshold approximation,
   and CombatWombat's per-field charts carry surveyed elevation and length. Neither has
   been transcribed yet — that is the next step once the roster is agreed.
5. **`configs/*.yaml` say `srs_addr: localhost:5008`.** Syria is **5002** (confirmed), so either the
   Syria configs carry a different value or the port becomes a per-deployment override.


---

## 6. 🔴 THE CARD AND THE DCS TERRAIN DISAGREE ON 7 OF 8 TOWERS

Found while extracting the surveyed airfield summary. **The build uses the CARD**, because
the card is what a pilot has on a preset and therefore what SkyEye must transmit on. But
DCS's own AI tower for these fields is on a *different* frequency, so anyone using
built-in DCS ATC will not be where the card says.

| Field | Card (used) | DCS terrain |
|---|---|---|
| Incirlik | 360.100 | 360.10 ✅ the only match |
| Ramat David | 251.300 | 251.05 |
| King Hussein / Mafraq | 250.450 | 250.40 |
| Hatay | 250.300 | 250.25 |
| Gaziantep | 250.100 | 250.05 |
| Akrotiri | 252.000 | 251.70 |
| Paphos | 249.100 (moved) | 251.80 |
| H4 | 252.250 | 250.10 |

Three of them (Gaziantep, Hatay, Mafraq) are exactly **+0.05** off, which looks like a
transcription pattern rather than a deliberate choice. **Worth deciding whether the card
should be reconciled to the terrain** — if it were, SkyEye and DCS ATC would agree and
the preset would work for both.

## 7. Still to do

- **Fly it.** Nothing here has been confirmed on the air.
- **Carrier roles are not started by `start_region_syria.bat`.** Marshal/Deckboss/LSO
  need Syria variants (PG's Marshal is 306.300; Syria is 306.100) before they can be
  wired into the region launcher.
- **Thresholds are computed, not surveyed** — verify against DCS before enabling
  `--position-check`.
- **`DCSName` on all eight is a guess** at the Mission Editor name and is marked VERIFY.
- **Runway lengths** used for the threshold maths are estimates; only the designators,
  headings, elevations and positions are from the surveyed chart.
