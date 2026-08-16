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

Every role runs at **1.10** as of 2026-08-16 (operator preference). The rates
used to differ per role; Marshal (`1.00`) and ATIS (`0.97`) were raised to match
the Deckboss rate that sounded right.

| Role | Rate | Knob |
|---|---|---|
| Tower / Command | `1.10` | `--tts-speed` (default is `speedDeckboss`) |
| Deckboss | `1.10` | `speedDeckboss` |
| Marshal | `1.10` | `speedMarshal` |
| ATIS | `1.10` | `speedATIS` |

The per-role constants are in `cmd/atc/main.go` and are kept as three separate
names even though they're now equal, so one role can be pulled back off 1.10
without disturbing the others. Only the Tower/Command rate is exposed as a flag.

## Voice casting

| Role | Voice | Note |
|---|---|---|
| Tower | `--tts-voice` (`nova`) / `--tts-voice-male` (`onyx`) | See rotation below. |
| Marshal | `--marshal-voice` (`coral` in `start_marshal.bat`) | Good fit — calm and even. |
| Deckboss | `--deckboss-voice` (`shimmer`) | Female, bright and cutting — suits shouting over jet noise. Was `ash` (male) until 2026-08-06; `fable` before that, rejected as soft and storyteller-ish. Shared with Al Dhafra Tower: the four female voices were all taken, and a land field on 251.1 is the safest one to double up on. |
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

The pink-noise amplitude behind each tier was raised ~60% on 2026-08-16 for a
more audible carrier hiss. The tiers keep their old spacing:

| Tier | Noise amplitude | Was |
|---|---|---|
| light (ATIS) | `0.032` | `0.020` |
| medium (Marshal, Deckboss, Command) | `0.064` | `0.040` |
| heavy (Tower) | `0.128` | `0.080` |
| extreme | `0.208` | `0.130` |

The bandpass corners were left alone — hiss level is the knob for "more static";
narrowing the passband changes the voice's character instead. If it now reads as
too hot, step the tier down (`--tower-radio-intensity=medium`) before editing the
constants, since the flags need no rebuild.

PTT key-up click and squelch tail are already applied by `addMicClicks`, which
runs after the radio effect on every TX.

## Cache note

`globalTTSCache` is keyed by voice **+ speed + instructions + text**. Two roles
can share a voice and rate but sound different, so leaving instructions out of
the key would let one role serve the other a cached MP3 in the wrong style.

There is no prewarm. The old `prewarmTTSCache` synthesized ~30 bare response
bodies at startup (twice, with rotation on), but every tower TX is callsign-led,
so no prewarmed entry could ever be read — it cost ~60 TTS calls and ~12s of
rate-limit sleep per tower start for a guaranteed zero hit rate. Making it work
needs the composer to stitch a cached body onto a per-callsign+tower prefix; the
phrase list is in git history if that gets built.
