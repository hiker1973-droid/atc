# vSFG-7 SkyEye ATC Comms Packet — builder notes

**Built 21 OCT 2026.  v1.31.  Product = one full-screen 16:9 PDF, 28 pages.**
`vSFG-7_SkyEye_ATC_Comms_v1.31.pdf` — cover + 27 cards: How it works · Frequencies ·
Deckboss · Deckboss calls · Marshal Case I · Marshal Case III · Marshal calls 1 · 2 · 3 ·
Paddles · Off the boat · Command · Hitman 1-1 / 2-1 · Cleared hot? · Range areas ·
Overlord Actual · Texaco · **Darkstar** · **GCI calls** · A full sortie · Airfield ATC ·
Tower radios · Syria towers · Departure · Arrival · Tower calls · No answer?

## v1.31 (21 OCT 2026) — the GCI, Darkstar / Overlord, from origin/main `16957f2`

User asked 2026-10-21 to add the new GCI controller to the pilot documentation.
- **New card 18 DARKSTAR:** SkyEye's GCI brain in atc.exe, our voice (Lily, British).
  Darkstar 264.000 AM Black Sea Day / Night (Hornet COMM 1 CH 5, as on the Caucasus Training
  `fleet.py` and the Hornet New Pilot Pack card); Overlord 266.000 AM Syria / Foothold (dial
  it - channels differ per Foothold card). The flow Command check-in -> GCI check-in -> fence in
  (picture follows) -> fly -> fence out -> checking out -> Command RTB; the naming rule (DCS and
  SRS name "Raider 302 | Name"); Command's "Darkstar is up on two six four point zero for picture".
- **New card 19 GCI CALLS:** radio check, checking in, fence in / out, alpha check, picture,
  bogey dope, fast / slow movers, declare, snaplock, spiked / strobe, vector to tanker, splash,
  checking out; unasked PICTURE (5 min), THREAT (25 nm, 3 min), MERGED (3 nm), FADED, SUNRISE;
  quiet in recoveries (12 nm / 10,000 ft carrier, 10 nm / 4,000 ft AGL field).
  From `cmd/atc/gci.go` (gciCheckOut, gciAck, gciAliases), `gci_recovery.go`, `pkg/gci`.
- Card 1 roles row, card 2 Black Sea / Syria GCI rows, card 11 Off the boat step 4,
  card 12 Command fence in, card 20 sortie, card 27 No answer? (step 6, naming quirk).
- Cards 18 - 25 of v1.30 are now 20 - 27 (`TOWER_CARDS` = cards 21 - 26). Build stamp 16957f2.
- GCI frequencies come from `facts.json` (`gci`, `gci_name`, read from `start_gci_*.bat` by
  TestDocFacts). Phraseology Reference v2.5 section 11 prints the same calls; TestDocPhrases
  runs each through gciCheckOut / gciAck / SkyEye's parser.
- Not done: Kola (Darkstar 264.000 assumed, not confirmed on the Kola card) is not printed.
  "vector to bullseye" is not printed - the GCI's parser has no bullseye location and would
  not answer it.

## v1.30 (20 OCT 2026) — Day 2026-10-19 log fixes, from origin/main `823cf86`

User asked 2026-10-20 "push and update documentation".
- **Card 12 Command:** new row "in hot / pickle / Fox 3 / splash -> cleared hot · copy" (`a5264fb`).
- **Card 17 Texaco:** check-out "checking out / moving on to mission -> cleared to depart" (`a5264fb`).
- **Card 24 Tower calls:** "holding" alone = holding short; "request handoff" -> contact Command
  (`bfe1f15`); "green jet" -> loud and clear; mayday "(+ runway NN)" -> the runway you name;
  new row "N souls on board, fuel state -> crash crew standing by" (`823cf86`).
- Build stamp 823cf86. `docs/tanker-flow` sheet now covers Texaco 1-1 + 3-1, the three
  send-offs, radio check and the new words. Not done: Phraseology Reference (no Texaco section).

## v1.29 (19 OCT 2026) — Texaco 3-1, from origin/main `1c8d29d`

User asked 2026-10-19 "add Texaco 3-1 to the comms packet".
- **Card 17 TEXACO** now covers both SkyEye tankers: new "THE TANKERS" table - Texaco 1-1
  KC-130 TACAN 51Y, 252.100 AM COMM 2 CH 8, angels 13 ~260 KIAS; Texaco 3-1 KC-135 MPRS
  TACAN 53Y, 252.300 AM COMM 2 CH 10, angels 24 ~265 KIAS (`--tanker2-freq`, `cmd/atc/tanker.go`).
  Same flow on both; Command step "contact the nearest Texaco"; note: radio check any time.
- Card 1 roles row "TEXACO 1-1 / 3-1 252.1 / 252.3 AM"; No answer? step 6 "Texaco 252.1/3".
- Build stamp 1c8d29d. Not done: docs/tanker-flow sheet still Texaco 1-1 only; Phraseology Reference.

## v1.28 (19 OCT 2026) — Texaco, the SkyEye tanker net, from origin/main `27454b6`

User asked 2026-10-19 for an AAR card.
- **New card 17 TEXACO:** Texaco 1-1, 252.100 AM (Hornet COMM 2 CH 8), Black Sea Day / Night,
  KC-130 TACAN 51Y, 13,000 ft about 260 KIAS. The vSFG-7 AAQ Aerial Refueling Procedure flow
  (initial -> how much -> trail -> port observation -> basket -> DCS Ready pre-contact ->
  starboard reform -> checking out), one jet per basket, from `cmd/atc/tanker.go`,
  `tanker_speed.go` and `command_position.go`. Full sheet: `docs/tanker-flow`.
- Card 1 roles, card 12 Command ("request tanker" -> card 17), No answer? (Texaco 252.1).
- Cards 17 - 24 of v1.27 are now 18 - 25 (`TOWER_CARDS` = cards 19 - 24). Build stamp 27454b6.
- Not done: Phraseology Reference (no Texaco section).

## v1.27 (18 OCT 2026) — red deck / green deck

Operator 2026-10-18: the landed Hornet calls "red deck", then "green deck" when clear of the
landing area; no green deck by the time the next Hornet is at 1/4 mile and Paddles calls
"XXX, foul deck, wave off, wave off, proceed to bolter pattern."
- **Card 10:** the foul-deck row is now "at 1/4 nm" with the callsign line, plus a note:
  after your trap, "red deck", then "green deck". From `pkg/lso/lso.go` (`FoulDeckCall`,
  `foulDeckM`, `redDeckFor` 3 min) and `cmd/atc/lso.go` (`lsoRedDeckRe` / `lsoGreenDeckRe`).
- Paddles sheet: section 5 updated, new section 6 "After your trap: red deck, green deck".

## v1.26 (18 OCT 2026) — Paddles, the SkyEye LSO, from origin/main `6208ef8`

Operator 2026-10-18: "Also add to master ATC pdf", with the new one-page
`docs/paddles-flow/Paddles LSO Comms Flow.pdf`.
- **New card 10 PADDLES:** 128.300 AM, Black Sea Day / Night only, no kneeboard preset yet
  (dial it). Ball call / Clara / "call the ball" / radio check; the groove calls (high, low,
  power, easy, lineup, fast / slow); wave-off, foul deck, bolter; nothing spoken on a trap.
  From `pkg/lso/lso.go` (calls and thresholds) and `cmd/atc/lso.go` (ball calls).
- Every "no SkyEye LSO" line replaced: card 1 (roles table + footnote), card 2 (128.300 vs
  the LIVE LSO 128.100 human preset), card 4 (bolter note), card 5 (after initial), the sortie
  card and No answer? (frequency check + quirk).
- Cards 10 - 23 of v1.25 are now 11 - 24 (`TOWER_CARDS` = cards 18 - 23). Build stamp 6208ef8.
- Not done: Phraseology Reference (no Paddles section yet); kneeboard preset for 128.300.

## v1.25 (16 OCT 2026) — Hitman 2-1, BDA calls, range areas + Overlord, from origin/main `15c969b`

Operator 2026-10-16: "update pdf" after the day's SkyEye work (all live on Day/Night/Foothold
13:36, `15c969b`).
- **Card 1:** roles list Hitman 1-1 / 2-1 (61 / 62 FM) and range areas + Overlord (cards 14 - 15).
- **Card 11 Command:** tasking row "contact Hitman 1-1 or 2-1"; new row "proceeding to / request
  Alpha -> cards 14 - 15".
- **Card 12:** HITMAN 1-1 / 2-1 — 2-1 on 62.000 FM, **no kneeboard preset yet (dial it)**; one
  sector each (Kuban Valley 1-3 / Cherkessk 4-5), Command sends you to the one with a free target.
  Fixed a stale line: Hitman reads target, name and IP only (since 2026-10-14), not elevation /
  headings / egress.
- **Card 13:** BDA band = the four calls (good / partial / no effects + correction / unable ->
  "say your observed effects").
- **New card 14 RANGE AREAS:** the A - O table with what Command says and the real threat. Notes
  that the kneeboard card calls E (SA-10) and G (SA-2) "soft x5" and leaves M (Moskva group) blank.
- **New card 15 OVERLORD ACTUAL:** the ask -> stand by -> ~10 s Overlord "Irene / cleared to
  engage" -> copy -> brief -> off target -> Winchester RTB flow, plus the other phrasings.
- Cards 14 - 21 of v1.24 are now 16 - 23 (`TOWER_CARDS` = cards 17 - 22). Build stamp 15c969b.
- **Phraseology Reference v2.3:** new section 5 "Range Areas and Overlord Actual" (6 rows,
  dialogue, callout), Hitman now section 6 "Hitman 1-1 / 2-1" with sectors and the four BDA
  calls, Hitman 2-1 in the frequency map (facts.json `jtac2`), Deckboss/Marshal/Tips renumbered
  7-9. `TestDocPhrases` now checks the new Command wants (area, overlord, off_target, winchester,
  bingo, joker); `TestDocFacts` reads `--jtac2-freq`.
- Post gate: already met (all rigs on 15c969b at build time). Open: a kneeboard preset for 62.000
  and the kneeboard threat labels for E / G / M (training builder `targetcard.py`).

## v1.24 (16 OCT 2026) — Hitman on the training V1.25 north range, from origin/main `74c6260`

Operator 2026-10-16: "rebuild the comms packet PDFs" after the north-range merge.
- **Card 12:** subtitle "five 9-line cards, north of the ridge".
- **Card 13:** IP inbound accepted **within 20 nm** (`jtacIPInboundNm`, operator 2026-10-15; the
  v1.23 card still said 5 nm); new row "off the run-in line -> continue, run in heading NNN".
  BDA bridge line now names the Valley Road Bridge checkpoint and the Kuban no-strike road bridge
  (was Enguri / Rioni, the old Senaki range).
- Build stamp (card 21): origin/main 74c6260. Phraseology Reference **v2.2** carries the same
  (section 5 example + `phrases.json` readback now target 4, Indigo, matching
  `docs_test.go docReadbackTarget`; "100-127 nm north of Senaki"; 20 nm callout).
- `TestDoc*` green. Not deployed at build time: post `DISCORD_POST.md` only after the Caucasus
  rig runs 74c6260 and Training V1.25 is on the server.
- Not touched: `Desktop\SKYEYE ATC Dev\Documentation\Hitman 1-1 JTAC Comms Flow.pdf` (ReportLab
  one-off, no builder in git, SkyEye a00f12e). It uses placeholders, not target names, and already
  says 20 nm, but lacks the "run in heading" reply.

## v1.23 (15 OCT 2026) — the tower's 7-mile departure call, from origin/main `404e399`

Operator asked 2026-10-15 for the tower to call departures at 7 miles (`dd764a0`) and for the
release to say "expect handoff at seven" (`404e399`), then "update docs".
- **Card 18 Departure:** AIRBORNE reply "angels N, expect handoff at seven DME"; the 7 DME pilot
  step is now **7 MILES — the tower calls you** ("leaving my airspace, contact command"). Good to
  know: no 7 DME call needed (a late one gets "copy"); "clear the active" after takeoff = airborne.
- **Card 20 Tower calls:** airborne row "angels, expect handoff"; new row "(at 7 miles the tower
  calls you) → contact Command"; "seven DME / seven miles → copy (optional)".
- Cards 16 / 17: "7 DME handoff" → "7-mile handoff". Card 2 voices line = the current
  ElevenLabs cast by accent (no voice names - they change).
- Build stamp (card 21) is now origin/main 404e399. Phraseology Reference v2.1 carries the same
  change (table, dialogue, "The 7-mile handoff" callout) and fixes the Hitman readback example
  (`target 3, Cobra`, checked by `TestDocPhrases`).
- Not deployed at build time: post only after the rigs run `404e399`.

## Moved into git, 14 OCT 2026

The packet now lives in **SkyeyeATC `docs/comms-packet/`** (operator: "make sure we push
documentation to git"). Build from the repo root: `python -I docs/comms-packet/_tools/build_skyeye_packet.py`;
it copies the PDF to the Desktop `Documentation` folder and moves the previous version to `Documentation\Old` (2026-10-21 tidy; the old `SkyEye ATC Comms Packet` folder is in `Archive`). Old versions are also git history.
See `docs/README.md`. Same day, still v1.22 (unreleased):
- **Marshal 306.300 / COMM 1 CH 3 in every theatre** (operator ruling 2026-09-20, `527c0ed`;
  every `start_marshal*.bat`). v1.21 and the first v1.22 build printed 306.100 / CH 2 for Syria
  and the Black Sea. Cards 1, 2, 5, 10, 14, 21 and the facts at the top.
- **Card 2** label warnings rewritten for the current cards: "AI MARSHALL" CH 3 is SkyEye,
  CH 2 306.200 is a live human Marshal, Deckboss shares 128.600 with the DCS AI carrier;
  voices line now Command onyx (stern English).
- **Card 17 Syria** rebuilt from `docs/phraseology/facts.json`: the five Foothold fields
  (was the eight-field set retired 2026-09-18), no channel numbers.
- **Card 16** adds Tbilisi 267.000 / ATIS 237.000 (Hornet CH 18, Viper CH 10, V1.23 cards).

## v1.22 (14 OCT 2026) — JTAC Hitman 1-1, from origin/main `47c9898`

Operator asked 2026-10-14 to "add to the documentation pdf" the new AI JTAC. **22 pages, 21 cards.**
- **New cards 12 HITMAN 1-1 (the run) and 13 CLEARED HOT? (what Hitman checks)**, after Command.
  Facts from `cmd/atc/jtac.go`, `cmd/atc/command_tasking.go`, `start_command_caucasus.bat`:
  61.000 FM, Hornet COMM 2 CH 12, Viper COMM 2 CH 8 (Caucasus Training V1.23 presets; operator
  ruling 2026-10-14). Command's tasking reply only hands off ("contact Hitman one one on 61.0 for
  tasking"); Hitman draws the target at check-in (operator: the real flow). Checks: IP inbound
  within 5 nm; in call inside the final attack heading and 0.5 - 15 nm; post-cleared watch;
  no picture = no checks, still cleared hot.
- Card 1: HITMAN row in THE ROLES, "Radios AM (Hitman FM)". Card 11: `request tasking` row.
  Card 21 (No answer?): step 7 radio FM for Hitman; build stamp names 47c9898. Cover subtitle
  lists JTAC.
- **Renumbered:** v1.21 cards 12 - 19 are now 14 - 21 (`TOWER_CARDS` = cards 15 - 20). Card
  image names unchanged (`12_sortie.jpg` etc.); new ones are `12_jtac_flow.jpg`, `13_jtac_checks.jpg`.
- ~~Not re-checked for v1.22: the Marshal frequencies~~ - fixed the same day, see "Moved into git".
- Not deployed at build time: post `DISCORD_POST.md` only after the Caucasus rig pulls 47c9898,
  rebuilds and restarts Command, and Caucasus Training V1.23 is uploaded.

## v1.21 (15 SEP 2026) — carrier fixes + tower faults cleared, from origin/main `b20f6b2`

Operator asked for the packet update and named it **"v1.21"** (not v1.3). Describes SkyeyeATC
`b20f6b2` (Marshal/BRC/handoff fixes + Caucasus carrier roles, pushed 15 SEP) and `14bd4e3`
(tower parser fixes). **⚠ `b20f6b2` was NOT deployed when this was built** — Training 1 and
Foothold report `v1.5.1-78-g14bd4e3`. Until they pull + rebuild, and Training 1 runs
`start_region_caucasus.bat`, Black Sea Marshal is still 306.300 and Syria handoffs still say
306.3. `DISCORD_POST.md` says: post only after the rebuild.

- **Black Sea Marshal 306.100, COMM 1 CH 2** (operator ruling 15 SEP: the card's LIVE MARSHALL;
  `start_marshal_caucasus.bat`). Cards 1, 2, 5, 9, 16 and the `MARSHAL_F`/`H_MSH` facts.
- **Syria handoff fault closed** — `--handoff-marshal-freq 306.1` on Syria (and Caucasus)
  Command + Deckboss. Warnings removed from cards 7 and 8; card 7 now says Deckboss names the
  right Marshal per theatre.
- **Marshal:** card 5 gains ANY TIME — `say BRC` / `request BRC` / bare `BRC` (magnetic) and bare
  `checking in` (range/bearing + asks for marking moms). Card 6: radial/final bearing are
  magnetic. Card 4: Deckboss BRC magnetic. Card 7: "clear mothers" works. Card 8: pilots who
  launched from the boat are handed to Marshal even when a field is nearer (`chooseRecovery`).
  Card 16: Marshal needs the number in the callsign (`marshalCallsignFromText` returns "" without
  one).
- **Card 5 "see you at 10" reply corrected** — v1.0–1.2 printed "radar contact, say state";
  `MarshalRadarContact` actually says "visual carrier at ten miles, report altitude and range".
- **Tower faults 1–3 closed** (`14bd4e3`, running on both rigs): card 14 loses SAY MILE NOT
  MILES and gains DISTANCES ("15 miles inbound" / bare "15 miles" / "5 mile final" work; a DME
  without inbound/initial is still the departure call — that is the design, tested
  `intent_test.go`); card 13 "stay put, the tower calls you" (`hold_release_test.go`); card 10
  adds the automatic takeoff clearance; card 15 adds "N miles inbound" and "N mile final";
  card 16 loses the "miles" quirk. Fault 4 (handoff channel, card 11) unchanged.
- Build: first run clean (16 cards re-laid, 0 smaller type, 0 on two slides). All changed
  pages rendered with PyMuPDF and read; stamp `v1.21 · 15 SEP 2026` on all 17 pages; no stale
  strings. v1.2 moved to `Old\`.
- **Same day, still v1.21 — Marshal word for word.** Operator: "list the Marshal
  commands/response exactly … I did not see on the marking moms response assigning a Marshal
  at XX". Cards 7–9 added (**Marshal calls 1/2/3**: inbound + BRC + radar; stack + departure;
  Case III), each call with the composer's **first** wording verbatim, example values BRC 275,
  altimeter 29.92, angels 12, 30 nm on the 340 (Case III: ceiling 800, vis 2, EAT :15). Read
  from `pkg/composer/composer.go` on `b20f6b2`; departure uses flag defaults vSFG-7-Command /
  282.0 / "channel four". Number formats follow the code (`numberWord` spells 0–10 only, so
  "angels 12", "30 miles"; BRC as "BRC 275"; radials digit by digit). Card 5's marking-moms
  summary now says "Marshal at angels N, report see me at ten"; its stack bullet explains that
  "Marshal at angels N" is the stack altitude. Old cards 7–16 → 10–19; tower references now
  "cards 13 - 18" (`TOWER_CARDS`), post "see cards 14 - 15", "19 slides".
- That build: first stop was card 5's new row (667 > 652 px). A probe that patched the kit
  guards to record instead of exit, over all 19 cards, found one more (card 7 subline) — then a
  clean build: 19 cards re-laid, 0 smaller type, 20 pages, stamp on every page. Cards 5, 7, 8,
  9, 12 rendered and read.

## v1.2 (13 SEP 2026) — airfield towers, from origin/main `922f871`

Operator: "can you add some pages for airfield atc to the pdf?" — answers the open tower
question: **towers go in this packet** (cards 10–15), not a separate product. Theatres
unchanged (Persian Gulf, Syria, Black Sea); Germany and Iraq towers are not printed.

- **10 Airfield ATC** — addressing (field name first, "Tower" optional, bare "Tower," works,
  edit-distance forgiveness, King Hussein / H-4 aliases, CTAF), what the tower knows (mission
  weather, Tacview, IMC = ceiling <1,500 ft or vis <3 nm → straight-in only, from
  `state.UpdateFlightConditions`; night does not matter to towers), proactive calls (350 kt
  inside 10 nm, go-around inside 5 nm with a departure rolling, wind-shift runway change), and
  the two round-3 calls that are off by default.
- **11 Tower radios** — PG + Black Sea tower/ATIS frequencies (`pkg/airfield/*.go`,
  `cmd/atc/atis.go`) with Hornet / Viper COMM 1 presets, and the handoff-channel fault below.
- **12 Syria towers** — eight fields with Hornet presets off the ratified Eastern Med card;
  Paphos 249.100 vs CH 19 252.100 (TEXACO 1), H3 = H4, unpreset ATIS, the two-card problem,
  DCS F-menu tower on a different frequency (all from memory `syria-theatre`, re-read in code).
- **13 Departure** — taxi → holding short → LUAW → +5 s cleared (`AutoReleaseDelay`) →
  airborne (angels 3–6 random, `DepartureRelease`) → 7 DME → Command. Holds: inbound inside
  15 nm hold short, 15–20 nm LUAW (`LineUpWaitRadiusNm`, `HoldShortRadiusNm`), 60 s spacing.
- **14 Arrival** — overhead flow, >15 nm "continue inbound", wheels check, options, straight
  in, IMC, hung ordnance, and the two parser faults below.
- **15 Tower calls** — every trigger → reply, incl. the silent ones.
- Cards 1, 9 and 16 point at the tower cards; card 16 adds the "miles" symptom and Paphos.

**How the words were checked:** every phrase printed on cards 13–15 (plus the suspect forms)
was run through `ParseIntent` on `origin/main` in a scratch `_test.go` inside a detached
worktree under the session scratchpad. The worktree was removed afterwards; nothing was
committed or left in `C:\SkyeyeATC`. That run is where the faults below came from.

Tower presets printed (COMM 1; ATIS is the next channel on every card except Viper PG CH 20):

| Field | Tower | ATIS | Hornet | Viper |
|---|---|---|---|---|
| Al Dhafra | 251.100 | 248.200 | 8 | 12 |
| Al Minhad | 250.100 | 248.300 | 10 | 14 |
| Al Ain | 250.700 | 248.850 | 16 | 19 |
| Batumi | 260.000 | 230.000 | 8 | 12 |
| Kobuleti | 262.000 | 232.000 | 10 | 14 |
| Kutaisi | 263.000 | 233.000 | 12 | 18 |
| Senaki | 261.000 | 234.000 | 14 | 16 |
| Incirlik / Ramat David / Mafraq / Hatay / Gaziantep / H4 / Akrotiri / Paphos | per card 12 | | 8/10/12/14/15/16/18/none | no Syria Viper card |

Sources: `New Pilot packs\_HORNET_NEW_PILOT_PACK\KNEEBOARD_DROP-IN\{1 - Persian Gulf,3 - Black Sea (Caucasus)}`,
the matching `_VIPER_` folders, and `Syria\Foothold\00_vSFG7_Comm_Presets_Hornet_Syria.jpg`.

## TOWER FAULTS (found 13 SEP 2026) — 1–3 FIXED in `14bd4e3`, warnings removed in v1.21; 4 still printed

🔴 **"miles" is read as the 7 DME departure call.** In `ParseIntent` the `RequestDistanceCheck`
case (`containsAny` on "5 miles" / "7 miles" / "five miles" / "seven miles", substring) comes
before the inbound case, so "5 / 7 / 15 / 17 / 25 miles inbound" → `HandoffToCommand` and
`s.Remove` (dropped from the landing queue). "15 mile initial" parses fine. `ContinueInbound`
itself tells a 30 nm caller "report fifteen miles". A DME with no "initial"/"inbound" is a
departure call too ("12 DME"). `USER_COMMANDS.md`'s own example "5 miles inbound" is the broken
form. Printed on cards 14 and 16. Fix idea: whole-word match, and skip when inbound words are
present.

🔴 **A distance in the final call is not a landing call.** "10 mile final" / "5 mile final" →
`RequestDistanceInitial` (the inbound case's "mile" beats the later "final" case); "3 mile
final" → `RequestOverhead`. `RedirectToStraightIn` / `IMCDistanceInitialAck` ask for "ten mile
final" and one `HungOrdnanceAck` variant for "five mile final". Card 14 says "on final".

🟠 **A departure held for traffic or spacing is never released automatically.** The monitor's
release needs `ac.HoldingShort`, which is set only on the no-traffic LUAW path;
`state.SetHoldingShort` has no caller. Card 13: call "holding short" again.

🟠 **The 7 DME handoff channel is per map, not per jet.** PG fields say "channel four"
(Hornet Command; Viper Command is CH 1); Caucasus fields say "channel one" (Viper Command;
Hornet CH 1 is Deckboss 128.6). Card 11.

- "tally" / "visual" / "no joy" / "altitude check" parse but the controller returns silently
  ("basic mode — not active"); `USER_COMMANDS.md` lists them as working. Card 15 says silent.
- "initial" or "break" alone → `RequestBreak` → "report downwind", skipping the break approval.
- Viper PG preset card CH 20 "AL AIN INTL. ATIS 119.850"; SkyEye's Al Ain ATIS is 248.850.

## v1.1 (13 SEP 2026) — Command card, from origin/main `922f871`

Rebuilt after three ATC rounds landed on `main` (`be580cf`, `922f871`). Scope ruling unchanged —
**Command, Deckboss, Marshal only**; the new tower calls (say again, wind check, touch and go /
low approach / option, hung ordnance, field-name addressing, unknown-traffic and runway-vacate
calls) are **not** in this packet.
- **Card 8 Command** gains `state N.N` (under 2.0 = bingo; also works added to fence in/out —
  `CommandFenceIn/Out/FuelState`, previously unwired), `say / request bullseye` and
  `request tanker / say texaco` (`cmd/atc/command_position.go`: magnetic bearings, own-coalition
  bullseye object from Tacview, nearest friendly tanker by type or Texaco/Arco/Shell callsign,
  "unable" / "negative tanker on scope" with no picture).
- **Card 8 handoff** now says "when the rig has Tacview connected" — the open item below is closed.
  Round three also fixed Command's Tacview reader, which froze every position at first sighting,
  so **the 30 nm handoff printed in v1.0 could never have fired before `922f871`.**
- **Card 9 sortie** lists state / tanker / bullseye under ON THE JOB.
- Deckboss and Marshal cards unchanged. Marshal's "state 5 point 2" now parses as 5.2 — no card
  change needed.
- Syria `--handoff-marshal-freq` is **still unset** on `main` (checked `922f871`), so the Syria
  warnings on cards 2/7/8/9 stay.

## HOW TO REBUILD

```
python "_tools\build_skyeye_packet.py"          (from this folder)
```

Needs the shared modules in `Saved Games\DCS\Missions\_MISSION_TOOLKIT\briefing\`
(`missioncard.py`, `newpilot_kit.py`, `newpilot_slides.py`) and the group patch at
`Saved Games\DCS\Missions\New Pilot packs\VMFA225_patch.png`. The kneeboard renders in
`_tools\_cache\cards\` only feed the PDF — they are not a product (user ruling: PDF only).
A new `VER` moves the older PDF into `Old\`. `VER` and `DATE` change together; `DATE` is the
release date, set by hand.

## USER RULINGS (13 SEP 2026)

- Roles: **Command, Deckboss, Marshal** only. Theatres: **Persian Gulf, Syria, Black Sea**.
- Print **what SkyEye actually uses**, not the preset card.
- Marshal **per theatre as run today** (PG 306.300, Syria 306.100).
- Black Sea: **same pages, no note** — even though SkyEye runs no Marshal/Deckboss there.
  **Superseded 15 SEP 2026:** Black Sea now has its own Marshal (306.100) and Deckboss (128.600).
- **PDF only**, **Hornet + Viper** in one packet, **vSFG-7 group patch**.
- **Later the same day: towers are in** — "add some pages for airfield atc to the pdf" (v1.2).
  ATIS stays listen-only; frequencies are printed beside each tower.

## FREQUENCIES PRINTED, AND WHERE THEY CAME FROM

| Role | Freq | Hornet preset | Source |
|---|---|---|---|
| Command | 282.000 | COMM 1 CH 4 (Viper COMM 1 CH 1) | every `start_command*.bat` |
| Deckboss | 128.600 | COMM 1 CH 1 — card label "CVN-72 AI" | `start_deckboss.bat`; origin `start_deckboss_syria.bat` |
| Marshal PG | 306.300 | COMM 1 CH 3 — card label "CVN-72 AI AIRBOSS" | `start_marshal.bat` |
| Marshal Syria | 306.100 | COMM 1 CH 2 — "LIVE MARSHAL" | `start_marshal_syria.bat` |
| Marshal Black Sea | 306.100 | COMM 1 CH 2 — "LIVE MARSHALL" | `start_marshal_caucasus.bat` (operator ruling 15 SEP 2026; was 306.300 by default in v1.0–1.2 when no script existed) |

Hornet presets were read off the squadron cards: PG and Black Sea carrier blocks are
identical (C1/1 128.600, C1/2 306.100, C1/3 306.300, C1/4 282.000, C2/1 DECKBOSS 306.200,
C2/2 LSO 128.100); Syria is `Syria\Foothold\_assets\cards\_presets_FA-18C_hornet.html`.
The cards do not show per-channel modulation; SkyEye transmits AM.

## WHAT THE CODE SAYS (origin/main, fetched 13 SEP 2026)

Every call and reply on the cards was read from `cmd/atc/{command,deckboss,marshal,
command_handoff}.go`, `pkg/composer/composer.go` and `pkg/state/state.go`. The
`*_responses.md` docs were a map only. **Where they disagreed, the code won:**

- **Recovery case** (`state.ComputeRecoveryCase`): night (local <0600 or >=1800) → III;
  ceiling <1,500 ft or vis <3 nm → III; ceiling <3,000 ft or vis <5 nm → II; else I.
  `marshal_responses.md` and `PILOT_WORKFLOW.md` still say Case III is ceiling <1,000 ft.
- **Case II uses the Case I wording** — only Case III has its own composer methods.
- **Deckboss airborne reply** hands off to Marshal (since 2026-08-16). `CHEAT_SHEET.md` and
  `USER_COMMANDS.md` still say a bare "copy airborne", and `handoff_responses.md` §3 says the
  handoff was removed. Stale.
- **Command fence in / fence out / RTB** are live in `command.go`; `command_responses.md`
  omits fence in/out. RTB words are checked before fence out.
- **Address words:** Marshal drops any call not starting `marshal`/`union marshal`
  (+ Whisper variants); Deckboss requires the address except for `airborne`/`clear traffic`;
  Command extracts the callsign after "command" but does not require it.

**Re-checked in code on the notes pass (13 SEP 2026), after the packet shipped** — these had
been taken from the docs: conga cap 6 (`state.CongaCapacity`); Case III first commence
+10 min, +1 min per slot (`marshalCase3LeadTimeMin` / `IntervalMin`); stack counts Tacview
contacts within 50 nm of the carrier (`AssignMarshalAngels`); Deckboss requires the address
on request taxi, tension/ready cat, BRC and radio check but not airborne; Command has no
address guard (`extractCallsignSkippingAddress` then `commandResponse`). All confirmed.

✅ **Closed in v1.1:** Card 8's automatic 30 nm handoff only exists when the rig has
**`SKYEYE_TACVIEW` set** — both `start_command.bat` and `start_command_syria.bat` pass
`%TACVIEW_FLAG%`, which is empty otherwise (`start_command.bat` even prints "Command will run
without proactive tower handoff"). v1.0 did not say so; v1.1's card 8 reads "when the rig has
Tacview connected".

## KNOWN FAULTS PRINTED ON THE CARDS — NOT FIXED IN SKYEYE

✅ **CLOSED 15 SEP 2026 (`b20f6b2`, warnings removed in v1.21) — was:** 🔴 **Syria handoffs name the wrong Marshal.** `--handoff-marshal-freq` defaults to 306.3
(`cmd/atc/main.go`) and neither `start_deckboss_syria.bat` nor `start_command_syria.bat`
sets it. So in Syria, Deckboss's airborne reply says "push Marshal, three zero six point
three" and Command's 30 nm carrier handoff says 306.3, while Syria Marshal listens on
**306.1**. Fix is one flag in each of those two scripts (`--handoff-marshal-freq 306.1`).
**Not changed** — the repo is prod-adjacent, the user did not ask for SkyEye changes, and
the working copy has someone else's uncommitted edits (below). Cards 2, 7, 8 and 9 warn
Syria pilots instead. When fixed, remove those warnings and bump the packet.

- **Deckboss 128.600 is the DCS AI carrier frequency** (operator ruling 2026-09-02, per
  origin CLAUDE.md). Pilots may hear the AI controller on the same frequency.
- **The preset card labels do not match SkyEye** (card 2): C1/1 "CVN-72 AI" = Deckboss,
  C1/3 "AI AIRBOSS" = Marshal (PG/BS); the card's DECKBOSS 306.200 and (outside Syria) LIVE
  MARSHAL 306.100 get no answer; LSO 128.100 is silent — there is no LSO role.
- `start_region_syria.bat` comments still say "Deckboss 306.200" while the Deckboss script
  it calls launches on 128.6 (origin). Comment drift only.

## STATE OF C:\SkyeyeATC WHEN THIS WAS BUILT — READ-ONLY, NOTHING TOUCHED

- Local `main` was **6 commits behind origin/main** (GUID fix, ExternalAudio 16 kHz, Syria
  Deckboss 128.6 ruling). Facts were read with `git show origin/main:<path>`; nothing was
  pulled or checked out.
- Working copy has **uncommitted edits** to `start_deckboss_syria.bat` (comments) and
  `SYRIA_PLAN.md`. Left alone.
- **Which binary the Foothold VM (Syria) and Training 1 (PG) actually run was not
  verified.** The packet describes origin/main.

## THINGS THAT WENT WRONG, AND THE GUARDS THAT NOW CATCH THEM

1. **A probe built on the kit's guards only sees ONE overflow per block.** `check_steps`
   raises on the first bad row, so a "catch and continue" probe never measured the rows
   after it — a second overflow on card 6 surfaced only on the next real build. The
   scratch probe was rewritten to measure every row, band, subline, title, kv row and
   footer itself. Worth promoting into the kit if a third product needs it.
2. **Greedy two-column flow left the right half of 8 of 10 slides empty**, and on the
   Frequencies slide stranded the last bullet at the top of column 2, away from its band.
   Every build check passed; found by rendering the PDF with PyMuPDF and reading it.
   `newpilot_slides.build_pdf(..., balance=True)` now splits a card's columns evenly at a
   band boundary (falls back to greedy when no split fits). **Off by default** so the
   pilot packs' released PDFs do not move on a rebuild; this packet turns it on.
3. The first sample exchange trimmed Deckboss's real replies ("shooter's discretion", the
   Marshal frequency). Restored to the composer's wording.

All 11 pages were rendered and read after each build. The shipped v1.1 PDF carries
`v1.1 · 13 SEP 2026` on every page (re-checked by text extraction on the notes pass).

**v1.2 build (13 SEP 2026):** first build stopped on one guard — card 13's step detail "Reply:
proceed to angels N, contact tower at 7 DME." was 674 px against 672 — shortened to "Reply:
angels N, contact tower at seven DME." Second build: 16 cards re-laid, 0 with smaller type,
0 on two slides. All 17 pages rendered with PyMuPDF and read; text extraction shows
`v1.2 · 13 SEP 2026` on every page. v1.1 moved to `Old\`.

## RELEASE POSTS (13 SEP 2026)

**Packet post** — **`DISCORD_POST.md`** in this folder, first written for v1.0 and edited in
place for v1.1 (kept verbatim so a new version edits it rather than rewriting it). Frequencies
sit in a code fence because Discord does not render tables. Its header lists what must change
with the PDF:

- Title version/date follow `VER`/`DATE`.
- The **"Syria:" bullet** (Deckboss and Command push Marshal on 306.3) goes when the Syria
  handoff flag is fixed — same trigger as removing the warnings on cards 2/7/8/9.
- ⚠ The post says Deckboss **and** Command give the wrong Syria Marshal frequency. Deckboss
  always does; Command only when the rig has `SKYEYE_TACVIEW` set (its handoff watcher needs
  Tacview). Close enough for a warning, not a precise statement — same caveat as card 8.

**ATC update post** — **`ATC_UPDATE_POST.md`** in this folder: the squadron summary of all
SkyEye changes merged 13 SEP 2026 (`f75b2a4`, `32ca2bc`, `5614b9c`, `d8ab76e` via `be580cf` /
`922f871`), towers included. It is **wider than this packet** (the packet is Command / Deckboss /
Marshal only), and it says "merged, not live": no rig had pulled and rebuilt when it was
written. When rigs are rebuilt, or the two round-3 tower flags (`--unknown-traffic-calls`,
`--runway-vacate-chase`) are turned on, its Status and "switched off" sections go stale.

## OPEN

- Never flown against SkyEye. Black Sea carrier roles exist from `b20f6b2` but have not run.
- ✅ Syria handoff flag — fixed in `b20f6b2`; warnings and the "Syria:" post bullet removed in v1.21.
- **v1.21 is ahead of the rigs** until Training 1 and Foothold rebuild on `b20f6b2`.
- If Marshal is ever standardised on one frequency, card 2's table and cards 5/9/10 change.
- ✅ **Tower question closed (v1.2):** tower pages added to this packet. Still open: the four
  tower faults above are printed as warnings. **14 SEP 2026: faults 1–3 ("miles", "N mile
  final", held departures) fixed with tests and merged to SkyeyeATC `origin/main` as
  `14bd4e3` (commit `2c370f7`) — not deployed yet.** When rigs have pulled and rebuilt, remove
  the warnings on cards 13/14/16 and the DISCORD_POST "mile, not miles" bullet → v1.3.
  Fault 4 (handoff channel) unchanged. When `ParseIntent`'s "miles" / "N mile
  final" handling or the held-departure release is fixed and deployed, cards 13/14/16 lose those
  warnings → next version. Never flown against a live tower since the addressing fix.
- Germany and Iraq towers are built in SkyEye but not printed (theatre ruling unchanged).
- 🔴 **Two other products contradict this packet — recorded there, not changed:**
  - **Hornet New Pilot Pack v1.3, card 00_04 "The Boat"** prints SkyEye-era carrier radios
    off the squadron card: Syria rows `DECKBOSS - SkyEye 306.200 C2/1`,
    `LIVE MARSHALL - SkyEye 306.100`; PG/Black Sea rows `LIVE MARSHALL 306.100`,
    `DECKBOSS 306.200`. SkyEye actually runs Deckboss **128.600** everywhere and PG Marshal
    **306.300**. A pilot reading both products gets two answers.
  - **`Syria\Foothold\FOOTHOLD_SY.md` §carrier stack** lists DECKBOSS 306.200 as SkyEye.
  Resolution is a squadron decision: move SkyEye to the card, or reissue the card/packs.
- **`newpilot_kit.py` / `newpilot_slides.py` now feed THREE products** (both pilot packs and
  this packet). A guard or slide-layout change needs all three rebuilt; the `05_` training
  cards affect only the two packs.
