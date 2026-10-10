# Kola theatre — Operation WHITE NIGHT

Added 2026-10-09, on `main` since `b019258`. **Not flown.** Binaries were built on DCSServer
(192.168.1.231) only; Training 1, Night and Foothold were running other theatres and still need
`git pull origin main` + `build.bat` on the rig before Kola exists there.

The squadron's monthly mission moves from the Persian Gulf to Kola with a ten-mission
F-16C / FA-18C campaign (WHITE NIGHT). Hornets fly from CVN-72; the land fields are the
Viper bases, one per phase.

## 1. Roster — only the field in play

Operator ruling 2026-10-09: *"only the appropriate airfield"*.

| Missions | Field | ICAO | Tower | ATIS | Dashboard | Runway (mag) | ATIS languages | State |
|---|---|---|---|---|---|---|---|---|
| 1 | Bodo | ENBO | 251.150 | 249.150 | 6051 | 07/25 (070/250) | English + Norwegian | **LIVE** |
| 2-5 | Bardufoss | ENDU | 250.650 | 248.650 | 6052 | 28/10 (278/098) | English + Norwegian | parked |
| 8-10 | Banak | ENNA | 250.550 | 248.550 | 6053 | 34/16 (341/161) | English + Norwegian | parked |
| 6-7 | Rovaniemi | EFRO | 250.950 | 248.950 | 6054 | 21/03 (204/024) | English + Finnish | parked |

To bring a parked field up, change **four** places (the same rule as Syria's roster cut):

1. `start_towers_kola.bat` — uncomment its two lines
2. `start_atis_kola.bat` — add its ICAO to `--atis-stations`
3. `pkg/airfield/registry.go` — add it to the `Kola` slice (Command's handoff scan walks it)
4. `cmd/launcher/ui.html` — add its row to `THEATRES.kola.towers` and its port to `wxPorts`

`pkg/airfield/kola_test.go` and `cmd/atc/atis_kola_test.go` pin the live set and will fail
until their expectations are updated too — that is deliberate.

## 2. Where the numbers come from

- **Centre, elevation, magvar, runway ends**: DCS Mission Editor via dcs-sms
  (`me airbase get`, `me coords to-geo`, `me coords magvar`), 2026-10-09. Thresholds are
  real DCS runway edges.
- **Runway headings**: TRUE bearing between the two thresholds minus the field's magvar.
  ⚠ The ME's `course_deg` is a GRID bearing and is 6-9° off true up here — do not use it.
- **Magvar is per field**: 8.0°E Bodo, 10.8°E Bardufoss, 13.6°E Rovaniemi, 14.8°E Banak.
- **Tower**: the DCS map's own UHF tower frequency. ⚠ The squadron's
  `vSFG7_Comm_Presets_Hornet_Kola.jpg` prints Bodo as 250.450 and Rovaniemi as 250.650
  (each a neighbouring field's frequency). The WHITE NIGHT missions bake the map's values
  into the jets' presets and DTC, so SkyEye transmits on those.
- **ATIS**: ASSIGNED BY US, tower minus two MHz. The squadron card has no ATIS frequency
  and **no jet has it on a preset** — pilots dial it. Bodo's 249.150 is printed on card 4 of
  both WHITE NIGHT kneeboard decks since mission build v0.5, and that build asserts the card
  against `pkg/airfield/enbo.go`, so changing it here without rebuilding the deck fails the build.
- **ILS / TACAN / VOR**: DCS `Mods\terrains\Kola\beacons.lua`. ⚠ A localizer's `direction` there
  is the RECIPROCAL of the landing course (checked against Batumi 13 and Senaki 09 on Caucasus).
  The first cut read it the other way and announced every ILS on the wrong runway end; corrected
  the same day in `ff90e69`. ILS: Bodo 07 110.30 / 25 108.70, Bardufoss 28 108.70, Banak 34
  108.30, Rovaniemi 21 111.70. Bodo's 25 localizer sits ~10° off the runway course in the file —
  read as an offset approach, not verified in the sim. TACAN mode (X) is assumed; the file gives
  only the channel.
- **Map clock**: the Kola map runs on UTC+3 everywhere, Norway included.

## 3. Carrier roles

CVN-72 is in the Vestfjord. `start_marshal_kola.bat` runs Marshal on **306.300** and
`start_deckboss_kola.bat` runs Deckboss on **128.600**, applying the standing rulings
(2026-09-20 and 2026-09-15) by analogy. **Not ratified for Kola.** ⚠ The WHITE NIGHT
kneeboard (card 4, Hornet) prints Marshal as 306.100, the card's LIVE MARSHALL preset — it
disagrees with this script until one of them is changed. The Kola Hornet card
prints 306.300 as "CVN-72 AI AIRBOSS", 306.100 as "CVN-72 LIVE MARSHALL" and Deckboss on
COMM2 CH1 306.200. Both scripts pass `--airfield ENBO` for the BRC magvar.

Command stays on 282.000; `start_command_kola.bat` sets `--handoff-marshal-freq 306.3`
explicitly (Syria shipped relying on the default and handed pilots to the wrong Marshal).

## 4. Open

- Break directions are "left" at all four fields — placeholder, as on the other theatres.
- `DepartureAngels` (Bodo 5, Bardufoss 6, Banak 4, Rovaniemi 3) are judgement against the
  terrain, not measured.
- `HandoffPreset` says "channel one": Command is COMM1 CH1 on the Viper Kola ladder and CH4
  on the Hornet's. The land fields are Viper bases, so the Viper number is spoken.
- TTS will read "Bodo" as written; the Norwegian pass will say Bodø.
- The SRS client is still coalition 2 only.
- No `configs/*.yaml` were added — the newer theatres (Caucasus) run without them.
