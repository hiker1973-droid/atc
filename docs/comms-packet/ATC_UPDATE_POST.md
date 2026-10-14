<!--
Squadron post summarising the SkyEye ATC changes merged to origin/main on 13 SEP 2026:
f75b2a4, 32ca2bc, 5614b9c (merge be580cf) and d8ab76e (merge 922f871). Written 13 SEP 2026.
Paste everything below the line into Discord.

KEEP IN STEP:
- "Status" says merged but NOT live. Update it as rigs pull main + run a full build.bat
  (a launcher-only redeploy is not enough).
- "Built, but switched off" = --unknown-traffic-calls and --runway-vacate-chase, both default
  OFF. Move them up if a rig turns them on.
- The Comms Packet covered only the Command part of this until v1.2 (13 SEP 2026), which added
  tower cards 10-15 (DISCORD_POST.md). The "Comms Packet v1.1" section below predates that.
- Deliberately left out (admin, not pilot): Foothold Syria fixes (SKYEYE_MIZ on the 4.9.0
  mission, single ATIS process, rebuild for the ExternalAudio 16 kHz fix) and the two
  disagreeing Syria preset cards (Paphos CH19 252.100 collides with TEXACO 1).
-->

---

## 📡 SkyEye ATC update — 13 SEP 2026

### 🛬 Towers: they understand more calls
- **Call a tower by its field name.** "Akrotiri, Raider 331, ready to taxi" now works without saying "Tower". Misheard names like "Acrotari" still get through. You can also say **King Hussein** for Mafraq and **H-4** for H4. A plain "checking in" gets an answer.
- **Emergencies come first.** A mayday or pan pan that included your position ("mayday, 10 miles south") was being handled as a normal inbound call. That's fixed.
- **Fewer wrong matches.** "Returning to base" no longer counts as a base-leg call, and "totally" no longer counts as "tally". "Request visual approach" now gets you a straight-in.
- **"Request for taxi"** and CTAF-style **"rolling runway 09"** now work. Both were calls towers had been missing.

### 🆕 New tower calls
- **"Say again"**: tower repeats the last thing it said to you (within 2 min).
- **"Wind check"** / **"say altimeter"**: current wind and altimeter.
- **"Touch and go"** / **"low approach"** / **"request the option"**: you get "cleared touch and go" instead of "cleared to land".
- **"Hung ordnance"**: straight-in arrival, no overhead break, dearm at the end of the runway.

### 🎯 Command
- **"Say bullseye"**: your bearing and range from bullseye.
- **"Request tanker"** / **"say Texaco"**: bearing, range and angels to the nearest friendly tanker.
- **Fuel calls answered.** "State 5 point 2" now reads as 5.2, and anything under 2.0 is treated as bingo.
- **Fixed:** Command's radar picture stopped updating after it first saw each aircraft. Because of that, **the 30 nm handoff to recovery never triggered.** It should work now.
- Command says "unable" or "negative tanker on scope" when it can't answer, instead of staying silent.

### 🧪 Built, but switched off for now
- Tower calls up players who come within 8 nm without talking to it: "say intentions".
- Tower asks you to "report clear of the runway" if you land and sit on it.

### 📄 Comms Packet v1.1
Adds Command fuel state/bingo, bullseye and tanker calls, plus a note that the 30 nm handoff needs Tacview.

### ⚠️ Status
All of this is **merged but not live on any server yet**. It goes live as each rig is updated and rebuilt, and none of it has been tested over SRS so far. If a call gets ignored, tell us what you said, word for word.
