# Marshal Smoke Test — CVN-naming + Case 1 Recovery

Manual checklist to exercise Marshal end-to-end after carrier-related changes
(`findCarrierContact` carrier-match logic, BRC plumbing, stack assignment).

Dev-only. Do **not** run on Training 1 during a live mission — uses 306.3 and
will produce real radio traffic. Schedule a quiet slot or run on the dev rig.

## Pre-flight

1. Carrier must be on the Tacview feed. Confirm with:
   ```powershell
   Get-Content C:\SkyeyeATC\logs\atc-marshal.log -Tail 50 | Select-String "Tacview contact first-seen.*Carrier|CVN-"
   ```
   Expect to see the carrier group string (`Carrier strike group-X`) and/or the
   CVN-named unit (`CVN-72 ABE`, `CVN-73 GW`, etc.) in the last few minutes.

2. Marshal is online and registered:
   ```powershell
   Get-Content C:\SkyeyeATC\logs\atc-marshal.log -Tail 30 | Select-String "ATC online|registered on SRS|stack online"
   ```

3. Tail Marshal in a second window for the duration of the test:
   ```powershell
   Get-Content C:\SkyeyeATC\logs\atc-marshal.log -Wait -Tail 0 | Select-String -NotMatch "Tacview.*offline|Tacview nominal|SRS.*retrying"
   ```

## Carrier match logging (v1.1.1 transition-only behavior)

Every Marshal intent calls `findCarrierContact`. Expect:
- **First** call after Marshal starts → one `level=info` line with
  `message="carrier match (transition)"` and `prev=""`.
- **Subsequent** calls with the same carrier → `level=debug` only (won't show
  in default Info-level log).
- Carrier swap (mission changes / different group string wins) → another
  `level=info` transition line with the previous callsign in `prev`.

To force a transition deliberately, restart Marshal after a mission change.

## §1 Marking moms

Radio: **`Marshal, Raider 99, marking moms, 30 DME, angels 5, state 6.0.`**

Expected response shape (variant random; should match `marshal_responses.md` §1):
```
Raider 99, Marshal, radar contact, angels {R_ANG}, range {R_DIST},
bearing {R_BRG} from mother, mother's weather {WX}, expect Case One
recovery, BRC {BRC}, altimeter {ALTIMETER}, Marshal angels {ANGELS},
report see me at ten.
```

Log expectations:
- `Marshal heard` with the verbatim transmission
- `Marshal transcribed` with `callsign=Raider 99`
- `Marshal TX` with stack-angel assignment
- Stack: `Marshal stack: assigned angels N` info line

If the `{RADAR}` clause is missing from the response, `findCarrierContact`
returned false — verify Tacview has the caller as a contact within range.

## §1b DME position report

Radio: **`Marshal, Raider 99, 7 DME.`**

Expected: short ack per §1b. With radar:
```
Raider 99, Marshal, radar contact, 7 DME, continue.
```

## §2 See me at ten

Radio: **`Marshal, Raider 99, see you at ten.`**

Expected: `Raider 99, Marshal, radar contact, 10 miles, say state.`

## §3 State

Radio: **`Marshal, Raider 99, state 5.2.`**

Expected (normal state ≥2.0): `Raider 99, Marshal, copy state 5.2.` (or variant)

## §4 Established

Radio: **`Marshal, Raider 99, established angels 5.`**

Expected (deck clear → Charlie): `Raider 99, Marshal, signal Charlie.` (or variant)

## §5 Commencing (ack + check-in instruction; pilot stays on Marshal)

Radio: **`Marshal, Raider 99, commencing, state 5.0.`**

Expected: response acks commencing and instructs pilot to call 3-mile
initial for the actual paddles handoff. Three variants — any of:
- `Raider 99, Marshal, copy commencing, state 5.0, check in three mile initial, paddles handoff.`
- `Raider 99, Marshal, commencing, state 5.0, report three mile initial for paddles.`
- `Raider 99, Marshal, roger commencing, state 5.0, call three mile initial, paddles will have you.`

Log check: `Marshal: stack step-down (internal, no TX)` info line if other
aircraft were in stack — collapse should NOT transmit.

## §6 3NM Initial → LSO handoff (no TACAN)

Radio: **`Marshal, Raider 99, initial.`**

Expected: actual paddles handoff (after §5 commencing ack). No TACAN button. Three variants — any of:
- `Raider 99, Marshal, contact paddles, good luck.`
- `Raider 99, Marshal, switch to paddles, good luck.`
- `Raider 99, Marshal, paddles has you, good luck.`

Prior version cited "push button 72, check in" — dropped 2026-05-27.

## Failure modes to watch for

| Symptom | Likely cause |
|---|---|
| No `{RADAR}` clause in §1 response | `findCarrierContact` returned false — check Tacview connection and that caller is visible |
| Stack assignment stuck at angels 9 | All slots reserved; check `MarshalStack.ReservedAngels` count vs `marshalMaxAngels=9` |
| BRC missing from §1 | Carrier `HeadingDeg` not populated in Tacview — check carrier unit type |
| Multiple "carrier match (transition)" lines per minute | `findCarrierContact` is flapping between carriers — investigate which units are tied in the lower() match |
| `Marshal intent miss` for valid §1 trigger | Self-echo guard fired; confirm pilot led with the address word "Marshal," |

## Post-test cleanup

If running on dev rig, stop Marshal cleanly via `stop_marshal.bat`. On Training 1,
do not leave Marshal running unattended — it will respond to any 306.3 traffic.
