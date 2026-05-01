# OpenAI TTS Cost Baseline

Established **2026-05-01** after switching from `tts-1` → `gpt-4o-mini-tts`. Revisit log at the bottom — fill in 2 weeks out (≈ 2026-05-15) and again every couple of weeks.

## Operating profile (Training 1)

- Server uptime: 24 / 7
- Active player time: ~4–5 hours/day
- Active SkyeyeATC roles: 3 Towers (OMDM / OMAM / OMAL) + 5 ATIS stations
- Other roles (Marshal / Command / Deckboss / Scudwatch): parked under `dev_only/` for now

## Baseline metrics (logs, since 2026-04-25, ~7 days)

| Source | Events | Chars | Notes |
|---|---:|---:|---|
| Tower TX (OMDM/OMAM/OMAL combined) | 86 | 7,561 | avg ~88 chars per TX |
| ATIS regenerations (5 stations) | 51 | ~20,400 | est. 400 chars / station; only fires on weather change or first-run |
| TTS pre-warm (16 phrases per startup × ~30 chars) | varies | ~480 / startup / role | restart-driven, roughly 3 startups today |
| **7-day total** | — | **~30,000 chars** | ~4,300 chars/day equivalent |

## Pricing reference (verify on your OpenAI billing dashboard)

| Model | Rate | Notes |
|---|---|---|
| `tts-1` (legacy) | **$15.00 / 1M chars** input | Flat, character-based |
| `gpt-4o-mini-tts` (current) | **~$12 / 1M chars effective** | Token-based: ~$0.60/M text-input tokens + ~$12/M audio-output tokens. Audio output dominates. |

## Cost projections at current load

Using ~$12/M chars for `gpt-4o-mini-tts`:

| Load | Chars/day | Cost/day | Cost/month | Cost/year |
|---|---:|---:|---:|---:|
| Current observed | 4,300 | ~$0.05 | ~$1.50 | ~$18 |
| Heavy mission day (200 tower TX + 10× ATIS regens) | 40,000 | ~$0.50 | up to ~$15 | up to ~$180 |

**Verdict:** TTS spend is in the noise vs hosting/other ops costs. No optimization needed today.

## Cost levers (in order of leverage, if you ever want to reduce)

1. **TTS in-memory cache** (`globalTTSCache`, 200-item LRU in `cmd/atc/main.go`) — most tower TX hit cache. Bumping to 1000 items would catch more re-uses; memory cost ~25 MB.
2. **Disk-persist the cache between restarts** — the pre-warm cost (16 phrases × N tower restarts) is purely amortizable. ATIS already does this; tower doesn't.
3. **ATIS `weatherKey()` rounding** — if wind/altimeter rounding is too fine, small fluctuations re-regenerate ATIS. Currently fine for our weather model; revisit if regen count balloons.

## Revisit log

Each entry: date checked, days since baseline, total OpenAI spend on the bill (TTS line item), notes.

| Date | Days since 2026-05-01 | OpenAI TTS spend ($) | Notes |
|---|---:|---:|---|
| 2026-05-01 | 0 | — | Baseline established. Switched to gpt-4o-mini-tts + mic-click splice this date. |
| | | | |

Things to watch on each check-in:
- **Spend growth rate** vs hours-played growth rate. If spend is climbing faster than utilisation, the cache is leaking efficiency.
- **ATIS regen frequency** — `grep '"ATIS generating' logs/atc-om*.log | wc -l`. Spike = weather-key churn or cache file deletions.
- **Tower TX char totals** — script in this doc for reproducibility.

## Reproducing the metrics

```bash
# Tower TX char totals across the rolling window of all OMDM/OMAM/OMAL logs:
for log in C:/SkyeyeATC/logs/atc-om*.log; do
  echo "$(basename $log)"
  grep '"TX via OpenAI TTS' "$log" \
    | awk -F'"text":"' '{print $2}' \
    | awk -F'","time"' '{print $1}' \
    | awk 'BEGIN{n=0; chars=0} {n++; chars+=length($0)} END{print "  TX:", n, "  chars:", chars}'
done

# ATIS regen count:
cat C:/SkyeyeATC/logs/atc-om*.log | grep '"ATIS generating' | wc -l
```
