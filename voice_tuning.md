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

Every role runs at **1.20** as of 2026-09-25 (operator request). Before that,
**1.15** from 2026-09-19 and **1.10** from 2026-08-16; the rates used to differ
per role, and Marshal (`1.00`) and ATIS (`0.97`) were raised to match the
Deckboss rate that sounded right. ATIS regenerates its audio on its first broadcast after
a restart, so a speed change needs a rebuild + role restart, not a cache clear.

| Role | Rate | Knob |
|---|---|---|
| Tower / Command | `1.20` | `--tts-speed` (default is `speedDeckboss`) |
| Deckboss | `1.20` | `speedDeckboss` |
| Marshal | `1.20` | `speedMarshal` |
| ATIS | `1.20` | `speedATIS` |

The per-role constants are in `cmd/atc/main.go` and are kept as three separate
names even though they're now equal, so one role can be pulled back off 1.20
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

Every TX is prepared by `prepareTX` in two ffmpeg runs: one `volumedetect`
measurement, then one pass through `radioFilter` that does everything else —
level → bandpass → compressor → pink-noise mix → output gain → `alimiter` → PTT
key-up click and squelch tail spliced on. (Until 2026-10-10 the effect and the
clicks were separate passes, `applyRadioEffect` and `addMicClicks`.)

**Levelling.** The chain opens by bringing the voice to −20 dB mean
(`voiceTargetMeanDB`). gpt-4o-mini-tts does not return every voice at the same
level — on 2026-10-10 `sage` measured −34 dB mean against −20 dB for
`alloy`/`nova`/`shimmer` and −25 dB for `onyx` — and nothing after it makes the
difference up, so Command (and Syria Marshal, also `sage`) went out about 14 dB
under the towers and buried in their own hiss. −20 dB is where the tower voices
already sit, so they are unchanged. With `--radio-effect=false` the voice is
still levelled, just not put through the radio chain.

Each TX logs `voice levelled` with the callsign, measured `meanDB` and applied
`gainDB` — at info when the correction is 6 dB or more, debug otherwise. A role
that shows up at info on every TX is on a voice that comes back quiet (or hot);
that is the line to look for the next time one role is reported quieter than
the rest.

**Output gain.** `--output-gain-db` (default `14`) is applied after the noise
mix, so it raises voice and hiss together and leaves each tier's character
alone. Before it, the compressor (no makeup gain) and `amix` (which halves both
inputs) left every role peaking near −20 dB and averaging −34 dB — 11 dB under
the key-up click in front of it, with the limiter never engaging. At 14 the
voice peaks around −6 dB and averages −21 dB, level with the click.
`--output-gain-db=0` is the old level and needs no rebuild.

`alimiter` stays last in the voice path: digital clipping sounds like crackle
rather than like a driven radio, and the limiter adds the hard-driven UHF
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

The PTT key-up click and squelch tail are part of the same pass and are not
affected by the tier or by `--output-gain-db`.

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
