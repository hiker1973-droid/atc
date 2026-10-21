<!--
Release post for vSFG-7_SkyEye_ATC_Comms_v1.25.pdf + Phraseology Reference v2.3 (16 OCT 2026):
Hitman 2-1, BDA calls, range areas, Overlord Actual, run close-out (origin/main 15c969b).
The v1.24 post (Hitman north range) is in git history.
Paste everything below the line into Discord, then attach both PDFs.
Gate met at build time: Day, Night and Foothold all run 15c969b.
-->
---
**SkyEye ATC: a second JTAC, real BDA, and Command now runs the ranges** 🎯

**Hitman 2-1, 62.000 FM**
Two JTACs now, one sector each:
```
HITMAN 1-1   61.000 FM   KUBAN VALLEY   TGT 1-3   (Hornet C2 CH12, Viper C2 CH8)
HITMAN 2-1   62.000 FM   CHERKESSK      TGT 4-5   (no preset yet - dial it)
```
Ask Command for tasking and you're sent to whichever has a free target. Come back for more and you get the other sector.

**BDA that tells you what to do next**
• *Good effects / Good hits / Shack*: egress, report ready for further tasking
• *Partial effects*: kill count, re-attack approved
• *No effects*: a correction (*"correct south 300"*), re-attack
• *Unable to assess* (the bridges): Hitman asks *"say your observed effects"*. Answer "bridge dropped" or "no effects"

**Range areas A to O on Command (282.0)**
Tell Command where you're going: *"Command, Raider 1, proceeding to Alpha"*. It answers with what's there, the threat, a bearing and range, and anyone else already working it.
Ask instead (*"permission to proceed to Alpha"*, *"request Alpha"*, *"request to work Alpha"*) and **Overlord Actual** comes up about 10 s later: *"you are Irene for Alpha"* or *"you are cleared to engage Alpha"*. Both mean go. Answer "copy Irene" and Command gives you the brief.
⚠️ The kneeboard under-reports three areas: **E is an SA-10, G is an SA-2, M is the cruiser Moskva's group.** Command tells you the real threat.

**Closing out**
• *"off target"* gets *"copy off Alpha, say intentions"*
• *"Winchester, RTB"* gets copy plus the 30 nm recovery handoff
• *"Bingo, RTB"* gets copy plus the nearest tanker
• *"Joker"* gets copy

**Updated docs** 📄
• **SkyEye ATC Comms Packet v1.25**: Hitman 1-1 / 2-1, Cleared Hot? (BDA), new Range Areas and Overlord Actual cards
• **ATC Phraseology Reference v2.3**: new section 5 (range areas + Overlord), Hitman section updated

---
**New: Darkstar / Overlord GCI** 📡
Our own AWACS controller is on the air, a British voice, giving you the air picture.
```
DARKSTAR   264.000 AM   BLACK SEA DAY / NIGHT, KOLA   (Hornet CH5, Viper CH4)
OVERLORD   266.000 AM   SYRIA (FOOTHOLD)              (Hornet CH7, Viper CH5)
```
**Your name matters.** Your DCS name has to carry your callsign, like *"Raider 302 | Name"* or *"Venom 212 | Name"*, and your SRS name has to be the same. If it isn't, you won't get threat or merged calls.

**How to call:** start with "Darkstar" (or "Overlord", or "Anyface"), then your callsign, then what you want. Say bearings as three digits (*"zero six five"*).
• *"Darkstar, Raider 302, checking in"*: radar contact
• *"... fence in"*: copy fence in, and the picture comes a few seconds later. Command on 282 still copies your fence in and tells you where Darkstar is.
• *"... picture"*, *"... bogey dope"*, *"... declare bullseye 010 for 48 at 25000"*, *"... alpha check"*, *"... splash one"*
• *"... any fast movers"* gets jets only and *"... any slow movers"* gets helicopters only
• *"... fence out"*, then *"... checking out"*: copy, good day. Then call Command for RTB as usual.

**It calls you too:** a picture every 5 minutes, a THREAT when a hostile is inside 25 nm, MERGED inside 3 nm.

📄 **SkyEye ATC Comms Packet v1.34** (cards 18-19) and **ATC Phraseology Reference v2.8** (section 11)

**What Darkstar can see:** only what the mission's AWACS could see - the E-3 out to about 217 nm, the E-2 about 200 nm, and nothing below its radar horizon (a bandit on the deck far out is invisible until it climbs or closes; when one drops out of coverage you hear "faded"). If no AWACS is airborne, Darkstar is **off station**: "All players, Darkstar, off station" - picture and bogey dope get "off station, no picture" until "on station" comes back.

---
**New: multi-ship flights at every tower** ✈️✈️
One call for the whole flight, and one clearance covers every jet - takeoff and landing.
```
LEAD:  Senaki Tower, Raider 301, flight of three, 302 and 303, holding short
TWR:   Raider 301 flight, Senaki Tower, flight of three, runway 27, line up and wait
TWR:   Raider 301 flight, ... cleared for takeoff          <- all three jets
LEAD:  Senaki Tower, Raider 301, flight of two, 302, 10 miles east, inbound full stop
TWR:   Raider 301 flight, ... cleared to land              <- both jets
GND:   Raider 301 flight, ... Raider 301 spot 12, Raider 302 spot 13
```
• Say the size: *"flight of two"*, *"two-ship"* or *"three FA-18s"*, and name the wingmen right after the lead (*"302 and 303"* or *"1-2 and 1-3"*).
• A wingman can call too - he's answered as the flight (named on the first call, or flying within 2 nm of the lead).
• *"Raider 301, flight of two, full stop"* on its own is the landing call for both.
• The next departure waits 60 s plus 20 s per extra jet. Every jet gets its own parking spot.

**AWACS: the mountains now hide you** 🏔️
Darkstar / Overlord only see what the mission's AWACS could really see - now including terrain. Low in a valley behind a ridge (Caucasus, Lebanon, Norway) you drop off the picture until you climb.
⚠️ **Foothold:** Overlord is **266.0 = Hornet CH7 / Viper CH5**. CH5 / CH4 (264) there is the DCS AI E-3, not ours.

📄 **SkyEye ATC Comms Packet v1.34** (new card 27 Multi-ship) and **ATC Phraseology Reference v2.8** (new section 3.3)
