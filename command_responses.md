# Command Channel Responses (282.0 MHz, vSFG-7-Command)

Edit any of the response text below and send the file back. I'll port the changes into `cmd/atc/command.go`, rebuild, and ship as `v1.0.2` (or `v1.1.0` if you also want new intents — see "Adding new intents" at the bottom).

## How this works

When a pilot transmits on 282.0, Whisper transcribes it. The transcript is matched against the **trigger phrases** for each intent (case-insensitive substring match). On a match, atc.exe randomly picks one of the 3 responses and speaks it back via OpenAI TTS in the `onyx` voice.

Placeholders that get filled in at runtime:
- `{CALLSIGN}` — the pilot's callsign as detected (e.g. `Raider 032`)
- `{CHANNEL}` — `vSFG-7-Command` (or whatever `--command-name` is set to)

So the source line `"{CALLSIGN}, {CHANNEL}, loud and clear."` is spoken as `"Raider 032, vSFG-7-Command, loud and clear."`

---

## 1. Radio check / Comm check

**Trigger phrases** (any of these, case-insensitive):
- `radio check`
- `comm check`
- `comms check`
- `how copy`

**Responses** (one picked at random):
1. `{CALLSIGN}, {CHANNEL}, loud and clear.`
2. `{CALLSIGN}, {CHANNEL}, five by five.`
3. `{CALLSIGN}, {CHANNEL}, reading you loud and clear, go ahead.`

---

## 2. Check-in

**Trigger phrases:**
- `check in`
- `checking in`
- `check-in`

**Responses:**
1. `{CALLSIGN}, {CHANNEL}, read you loud and clear, proceed mission.`
2. `{CALLSIGN}, {CHANNEL}, loud and clear, you are cleared to proceed, good hunting.`
3. `{CALLSIGN}, {CHANNEL}, copy check-in, read you five by five, proceed with mission.`

---

## 3. On station

**Trigger phrases:**
- `on station`
- `on-station`
- `onstation`

**Responses:**
1. `{CALLSIGN}, {CHANNEL}, affirmative, good hunting.`
2. `{CALLSIGN}, {CHANNEL}, copy on station, good hunting.`
3. `{CALLSIGN}, {CHANNEL}, roger on station, you are cleared hot, good hunting.`

---

## 4. Off station

**Trigger phrases:**
- `off station`
- `off-station`
- `offstation`
- `departing station`

**Responses:**
1. `{CALLSIGN}, {CHANNEL}, copy, proceed to assigned pattern.`
2. `{CALLSIGN}, {CHANNEL}, roger off station, proceed to assigned pattern, good work.`
3. `{CALLSIGN}, {CHANNEL}, copy off station, return to pattern, well done.`


---

## Order of matching

Intents are matched **top to bottom**. First match wins. Important consequences:

- **Fence out is checked BEFORE Fence in** so "fence out" doesn't accidentally match "fence in" (it wouldn't anyway, but the ordering is defensive — see the source comment in `command.go:62-64`).
- If you add a new intent that overlaps with an existing one, place the more specific one first.

## Adding new intents

If you want to add a new intent (e.g. `winchester`, `bingo`, `state your position`, `request bullseye`), add a new section below in the same format:

```
## N. Intent name

**Trigger phrases:**
- `phrase one`
- `phrase two`

**Responses:**
1. `{CALLSIGN}, {CHANNEL}, response 1.`
2. `{CALLSIGN}, {CHANNEL}, response 2.`
3. `{CALLSIGN}, {CHANNEL}, response 3.`
```

A new intent is a `v1.1.0` minor bump (per the semver convention). Pure response-text edits are `v1.0.2` patch.

## Editing tips

- Keep responses concise — TTS audio time matters; pilots' radios get jammed if responses are too long.
- The TTS will pronounce numbers naturally — `032` becomes "zero three two", `5x5` becomes "five by five" if you write it as "five by five".
- Avoid characters that might confuse the TTS: stick to letters, basic punctuation (commas, periods), and natural phrasing.
- The channel name `vSFG-7-Command` will be pronounced as "VSFG seven command" or similar — if you want it cleaner, we could change `--command-name` to something like `Overlord` or `Anvil` and update the .bat. Mention if you want this.

---

## Source reference

Current code lives in `cmd/atc/command.go:23-83` (function `commandResponse`). When you send this file back, I'll regenerate that function from your edits.
