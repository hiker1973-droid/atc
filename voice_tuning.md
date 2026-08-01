# Voice Tuning

How each role sounds, and which knobs change it. Companion to the
`*_responses.md` files — those decide *what* gets said, this decides *how*.

## The delivery instructions

`gpt-4o-mini-tts` accepts an `instructions` string that steers tone, cadence,
pacing, and affect. It is the reason to use that model over `tts-1`, which
ignores the field entirely. The field was previously unset, so every role read
in the same neutral register no matter which voice was assigned — the only
difference between Tower and Deckboss was timbre.

Each role now carries a delivery brief, overridable per role. Set a flag to the
empty string to send no instructions for that role.

| Role | Flag | Default brief |
|---|---|---|
| Tower | `--voice-style-tower` | Clipped, matter-of-fact, brisk. Flat affect, no warmth, no rising intonation. Grouped numbers run together as one phrase. |
| Marshal | `--voice-style-marshal` | Calm, measured, deliberate. Unhurried and steady even in bad weather. |
| Deckboss | `--voice-style-deckboss` | Loud, urgent, punchy. Deck-edge handset over jet noise. Very short clauses. |
| Command | `--voice-style-command` | Steady, businesslike, confident. Slight urgency, no emotion. |
| ATIS | `--voice-style-atis` | Recorded-broadcast delivery. Even, neutral, identical inflection throughout. |

Defaults live as the `style*` consts in `cmd/atc/main.go`. They are ordinary
prose — rewrite them freely and rebuild; there is no schema.

## Speech rate

`--tts-speed` (default `1.05`) covers Tower and Command. Marshal and Deckboss
and ATIS have their own rates, because the roles genuinely differ:

| Role | Rate | Why |
|---|---|---|
| Tower / Command | `1.05` (`--tts-speed`) | Real tower controllers run fast and clipped. |
| Deckboss | `1.10` (`speedDeckboss`) | Deck calls are urgent and short. |
| Marshal | `1.00` (`speedMarshal`) | A Case III approach is read to a pilot who is writing it down. |
| ATIS | `0.97` (`speedATIS`) | Clarity on a loop beats pace. |

The per-role constants are in `cmd/atc/main.go`; only the Tower/Command rate is
exposed as a flag.

## Voice casting

| Role | Voice | Note |
|---|---|---|
| Tower | `--tts-voice` (`nova`) / `--tts-voice-male` (`onyx`) | See rotation below. |
| Marshal | `--marshal-voice` (`coral` in `start_marshal.bat`) | Good fit — calm and even. |
| Deckboss | `--deckboss-voice` (`ash`) | Was `fable` in `start_deckboss.bat`, which is soft and storyteller-ish — wrong for someone shouting over jet noise. |
| Command | `--command-voice` (`sage` in `start_command.bat`) | Good fit. |

## Tower voice rotation

`--voice-rotate-hours` (default 4) alternates the female and male tower voices
on a wall-clock bucket. That has a rough edge: a sortie straddling a bucket
boundary hears the tower change voice mid-pattern — one controller on downwind,
another on final.

Setting `--voice-rotate-hours=0` no longer means "always use the female slot".
It now **pins the voice to the airfield**, hashed from the ICAO, so each field
has one stable controller identity for the whole session and across restarts.
Running three towers then reads as three distinct people rather than one whose
voice keeps changing. Rotation left on behaves exactly as before.

## Radio effect

`applyRadioEffect` (bandpass → compressor → gain → pink-noise mix) now ends in
`alimiter`. `volume=1.15` on top of an 8:1 compressor and then summing noise can
push past full scale, and digital clipping sounds like crackle rather than like
a driven radio; the limiter catches those peaks and adds the hard-driven UHF
character compression alone doesn't give.

Intensity is per role: `--radio-effect-intensity` (default `medium`),
`--tower-radio-intensity` (`heavy`), `--atis-radio-intensity` (`light`).

PTT key-up click and squelch tail are already applied by `addMicClicks`, which
runs after the radio effect on every TX.

## Cache note

`globalTTSCache` is keyed by voice **+ speed + instructions + text**. Two roles
can share a voice and rate but sound different, so leaving instructions out of
the key would let one role serve the other a cached MP3 in the wrong style.

`prewarmTTSCache` builds the same profile the tower hot path uses — if the two
ever drift, every prewarmed entry lands under a key that is never read.
