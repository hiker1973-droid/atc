# JTAC MVP-1 — Design

A JTAC (Joint Terminal Attack Controller) role for SkyeyeATC, using the same
SRS + Whisper + OpenAI TTS + Tacview pipeline as Tower / Marshal / Command.
Pilots call in on a JTAC frequency, request a 9-line for a named target
("target alpha"), execute the run, and report BDA. JTAC speaks doctrine
phraseology and tracks state per (pilot, target).

## Scope

MVP-1 is the minimum to fly a single CAS pass end-to-end:

| Included | Deferred (v2+) |
|---|---|
| N pre-configured targets in YAML | Dynamic target picking from Tacview ground objects |
| Phonetic-letter target designation (alpha…) | Pilot-named targets ("the column near the bridge") |
| 9-line briefing (all 9 lines, doctrine order) | 5-line / quick attack format |
| Type-1 CAS (visual mark) | Type-2 (sensor cue) and Type-3 (free-fire box) |
| Mark types: laser / smoke / IR / none | Mark verification ("affirm laser on") |
| Cleared hot on "in", BDA on "off" | Aircraft-state-aware clearance (g-loaded, fuel, ordnance check) |
| Multiple pilots, different targets concurrently | Same-target contention queueing (MVP says "traffic in target") |
| Danger-close initials handshake (stub: ask + accept any 2 chars) | Full danger-close protocol with bearing/range/heading verification |
| `auto_clear_hot` + `danger_close` behavior knobs | Launcher-dashboard manual clearance approval |
| Friendlies as free-text string | Friendlies live from Tacview, abort if friendlies enter target box |

## YAML config

One file per JTAC instance. Loaded by `atc.exe --jtac-only --config configs/jtac_<callsign>.yaml`.

```yaml
callsign: "Warlord"
voice: "ash"                    # OpenAI TTS voice
freq_mhz: 252.5                 # SRS freq (UHF AM)
srs_client_name: "vSFG-7-JTAC"  # name shown in SRS client list

targets:
  alpha:
    nine_line:
      ip:                       { name: "IP one", lat: 24.8500, lon: 55.4500 }
      heading_deg:              270
      distance_nm:              4.2
      target_elev_ft:           350
      target_description:       "armored platoon, four BTRs"
      target_location:          { lat: 24.8167, lon: 55.5500, format: "dms" }
      mark:                     { type: "laser", code: 1688 }
      friendlies:               "south three klicks"
      egress:                   "south to IP one"
    attack_heading_deg:         270           # null = pilot's discretion
    remarks:                    "expect MANPADS threat south of target"
    danger_close:               false
  bravo:
    # ... same shape
  charlie:
    # ... same shape

behavior:
  auto_clear_hot:           true
  default_target:           null    # null forces "say target designation" when pilot omits
  end_after_bda:            true
  list_targets_on_checkin:  true
```

**Field notes:**
- `targets:` keys must be NATO phonetic letters (alpha/bravo/charlie/…). The intent parser matches the spoken word.
- `target_location.format`: `dms` (degrees-minutes-seconds, default), `decimal`, or `mgrs`. MGRS support is v2 stretch — MVP can stub to "grid not available" if MGRS requested.
- `mark.code` is ignored when `mark.type` is not `laser`.
- `danger_close: true` overrides `auto_clear_hot` for that target — JTAC always demands initials.

## Intent grammar

Lower-cased Whisper output is matched against these patterns. First match wins.

| Pattern (any-of) | Intent | Notes |
|---|---|---|
| `radio check`, `comm check`, `how copy` | RADIO_CHECK | Always responds with "loud and clear" regardless of state |
| `check in`, `checking in`, `on station` (and not `off`) | CHECK_IN | Triggers IDLE → ON_STATION |
| `what targets`, `say targets`, `targets available` | LIST_TARGETS | Inventory; doesn't change state |
| `9-line`, `nine-line`, `ready to copy`, `ready for brief` | REQ_BRIEF | Looks for `target <letter>` substring; falls back to default_target |
| `target <alpha\|bravo\|…>` (standalone, no other verb) | SET_TARGET | Switches pilot's `current_target` without re-briefing |
| `switching to target <X>`, `switch target <X>` | SET_TARGET | Same as above with explicit verb |
| `in`, `in hot`, `rolling in`, `commencing run` | IN | Optional `target <X>` modifier; else use `current_target` |
| `off`, `off cold`, `off dry`, `off target` | OFF | Same target resolution rules as IN |
| `bda`, `splash`, `target destroyed`, `good hits` (after OFF) | BDA | Free text after the trigger word is captured as the BDA report |
| `re-attack`, `re-engage`, `another pass` | REATTACK | Re-enters CLEARED_HOT for same target without new 9-line |
| `abort`, `disregard`, `cancel run` | ABORT | Drops back to ON_STATION for that target |
| 2 letters spoken individually (only valid in DC_INITIALS_PENDING) | INITIALS | e.g. "tango romeo" or "tango-romeo" |
| `say status`, `what's my state` | STATUS | Reads back current target + state, no transition |

**Hallucination filters** (already in main.go via `isWhisperHallucination`): any matched intent must come from a transcript that passes the existing filter. Add JTAC-specific anti-patterns to the prompt sent to Whisper: `"JTAC, Warlord, target alpha, target bravo, target charlie, 9-line, in hot, cleared hot"`.

## State machine

Per-pilot state. Each pilot can be at a different state per target.

```
   ┌───────┐
   │ IDLE  │
   └───┬───┘
       │ CHECK_IN
       ▼
 ┌─────────────┐
 │ ON_STATION  │◄──────────────────────────┐
 └──────┬──────┘                           │
        │ REQ_BRIEF                        │ ABORT (any state)
        │ (target T resolved)              │
        ▼                                  │
   ┌──────────┐  danger_close=true   ┌─────┴──────────────────┐
   │ BRIEFED  │─────────────────────►│ DC_INITIALS_PENDING    │
   └────┬─────┘                      └────────┬───────────────┘
        │ IN (auto_clear_hot=true)            │ INITIALS heard
        │ danger_close=false                  │
        ▼                                     ▼
                  ┌─────────────┐
                  │ CLEARED_HOT │
                  └──────┬──────┘
                         │ OFF
                         ▼
                 ┌──────────────┐
                 │ AWAITING_BDA │
                 └──────┬───────┘
                        │ BDA received
                        ▼
                  ┌──────────┐
                  │ COMPLETE │──── REQ_BRIEF target T' ──► BRIEFED(T')
                  └──────────┘
```

### Transition table

| From | Event | To | TX response |
|---|---|---|---|
| IDLE | CHECK_IN | ON_STATION | "{cs}, JTAC, read you Lima Charlie." + (if `list_targets_on_checkin`) target inventory |
| IDLE | LIST_TARGETS | IDLE | "JTAC has {N} targets: {list}." |
| ON_STATION | REQ_BRIEF (target T) | BRIEFED(T) | full 9-line for T |
| ON_STATION | REQ_BRIEF (no target, `default_target=null`) | ON_STATION | "Say target designation." |
| ON_STATION | REQ_BRIEF (no target, `default_target=X`) | BRIEFED(X) | full 9-line for X |
| ON_STATION | SET_TARGET (X) | ON_STATION (current_target=X) | "Copy, ready when you are for {X}." |
| ON_STATION | LIST_TARGETS | (no change) | inventory |
| BRIEFED(T) | IN + `danger_close[T]=false` | CLEARED_HOT(T) | "{cs}, cleared hot." |
| BRIEFED(T) | IN + `danger_close[T]=true` | DC_INITIALS_PENDING(T) | "Target {T} is danger close. Say your initials." |
| BRIEFED(T) | SET_TARGET (T'), T' already BRIEFED | BRIEFED(T') | "Switching to {T'}." |
| BRIEFED(T) | SET_TARGET (T'), T' not yet briefed | BRIEFED(T) | "Ready 9-line for {T'} when ready." (no auto-rebrief — explicit request required) |
| BRIEFED(T) | REQ_BRIEF (target T) | BRIEFED(T) | re-read 9-line for T |
| BRIEFED(T) | ABORT | ON_STATION | "Abort copied for {T}." |
| BRIEFED(T) | STATUS | (no change) | "{cs} briefed on {T}, awaiting in call." |
| DC_INITIALS_PENDING(T) | INITIALS (e.g. "TR") | CLEARED_HOT(T) | "Initials Tango-Romeo, cleared hot." |
| DC_INITIALS_PENDING(T) | other | (no change) | "Say your initials." |
| DC_INITIALS_PENDING(T) | ABORT | ON_STATION | "Abort copied." |
| CLEARED_HOT(T) | OFF | AWAITING_BDA(T) | "Copy off. BDA when able." |
| CLEARED_HOT(T) | ABORT | ON_STATION | "Abort copied. Off station for {T}." |
| CLEARED_HOT(T) | STATUS | (no change) | "{cs} cleared hot on {T}." |
| AWAITING_BDA(T) | BDA (text) | COMPLETE(T) if `end_after_bda` else BRIEFED(T) | "Copy BDA. End of mission, {T}." / "Copy BDA, ready for re-attack." |
| AWAITING_BDA(T) | REATTACK | CLEARED_HOT(T) | "Cleared to re-attack {T}." (no new 9-line) |
| AWAITING_BDA(T) | STATUS | (no change) | "{cs} awaiting BDA on {T}." |
| COMPLETE(T) | REQ_BRIEF (target T') | BRIEFED(T') | full 9-line for T' |
| COMPLETE(T) | "off station" / "RTB" / "JTAC out" | IDLE | "{cs}, JTAC, copy off station. Good work." |

**Cross-cutting:**
- `RADIO_CHECK` is handled at any state, no transition: "{cs}, JTAC, loud and clear."
- `STATUS` always returns the pilot's (target, state) — useful for debug and for pilot regaining context after a long pause.
- Unrecognized intent: silent (no TX). MVP doesn't say "say again" to avoid TTS spam on every garbled transmission.

### Same-target contention

If pilot B sends IN on target T while pilot A is in CLEARED_HOT(T) or AWAITING_BDA(T):
- Pilot B stays in BRIEFED(T).
- JTAC TX: "{cs B}, stand by, traffic in target {T}."
- When pilot A transitions to COMPLETE(T) or ABORT, JTAC TX: "{cs B}, target {T} clear, cleared in when ready."
- Pilot B has to re-issue IN to advance.

This is the **only multi-pilot coordination MVP does**. Anything more (formation calls, sequenced attacks across multiple targets) is v2.

## Files

| File | New / Modified | Notes |
|---|---|---|
| `cmd/atc/jtac.go` | NEW | `jtacLoop` mirrors `commandLoop` — TCP/UDP register, audio parser (uses Tower's offset `[2:4]` from `063f239`), intent dispatch, per-pilot state map |
| `pkg/state/jtac.go` | NEW | `JTACSession` struct, target lookup, state transitions |
| `pkg/composer/jtac.go` | NEW | `Brief9Line`, `ClearedHot`, `RequestBDA`, `EndOfMission`, `DangerCloseChallenge`, `TrafficInTarget`, `Inventory` |
| `pkg/airfield/` | (none) | JTAC is not airfield-bound; config is freestanding |
| `pkg/controller/controller.go` | (none for MVP) | v2 adds `GetGroundTargets` for dynamic target lookup |
| `cmd/atc/main.go` | MODIFIED | `--jtac-only`, `--config <yaml>` (already taken? check), wire `jtacLoop` |
| `configs/jtac_<callsign>.yaml` | NEW per mission | One file per JTAC instance |
| `start_jtac.bat` | NEW | Env-var pattern matching `start_command.bat`; passes `--config configs/jtac_<callsign>.yaml` |

**Naming gotcha:** `--config` may already mean something else in main.go. Verify before adopting; otherwise use `--jtac-config`.

## Frequency selection

JTAC needs a freq the **pilot side actually TXes on**. The current SRS-Client `RadioModelsCustom` parse failure means 282 / 306.3 are silent client-side. Confirmed working from the transmission log:

- 250.1, 250.7, 251.1 (Tower-owned)
- 264.0 (GCI Darkstar)

Picking a JTAC freq requires either:
1. **Live pilot TX test** on a candidate freq (e.g. 252.5, 261.0, 234.0) — watch transmission log for the pilot's GUID at that freq.
2. **Repair `RadioModelsCustom`** on pilot's PC first, then any in-band UHF works.

MVP-1 deployment is blocked on resolving this. Recommend testing a candidate freq before committing the YAML.

## Open design questions

1. **Whisper prompt seeding** — should `jtacLoop` use a different `transcribeFramesWithPrompt` than Tower/Marshal? "JTAC, Warlord, target alpha, in hot, cleared hot" biases Whisper away from civilian-aviation lexicon and toward CAS terms. Probably yes.
2. **Coordinate phraseology** — DMS vs decimal for `format: dms`: "two-four degrees four-nine minutes zero-zero seconds north" is ~6 seconds of TTS per coord. Long. MGRS is shorter but adds a parser. MVP-1 should ship DMS and decimal; pick MGRS as the first v2 improvement.
3. **Re-attack** — `REATTACK` transitions AWAITING_BDA → CLEARED_HOT without a fresh 9-line. Is that right, or should the pilot have to re-request 9-line? Doctrine answer: re-attack on same target with same approach reuses the 9-line. New approach (different IP, different mark) needs a new 9-line. MVP-1 reuses; v2 could add `re-attack with new IP` if needed.
4. **Multiple JTAC instances** — operationally fine (run two `atc.exe --jtac-only --config X` processes on different freqs), but they don't coordinate. If a pilot is briefed by JTAC-A on alpha and then keys JTAC-B's freq saying "in alpha", JTAC-B has no idea. Acceptable for MVP; cross-JTAC state would require shared storage.
5. **Test mode** — `--jtac-test-tx` parallel to `--marshal-test-tx` for SRS routing verification before pilots show up.

## Implementation order (when greenlit)

1. `pkg/state/jtac.go` — types and transition function in isolation, unit-testable
2. `pkg/composer/jtac.go` — phraseology with table-driven tests for each 9-line shape
3. `cmd/atc/jtac.go` — wire SRS register + UDP parser (clone `commandLoop`, swap intent dispatch)
4. `cmd/atc/main.go` — flag wiring, YAML loader
5. `start_jtac.bat` + sample `configs/jtac_warlord.yaml`
6. Smoke test: `--jtac-test-tx`, then pilot test on chosen freq

Estimated effort: 1 day to first-runnable, half-day to phraseology tuning. Total ~1.5 dev days assuming the freq question is settled.
