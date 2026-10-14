<!--
Release post for vSFG-7_SkyEye_ATC_Comms_v1.22.pdf + Caucasus Training V1.23 (Hitman 1-1).
v1.0 written 13 SEP 2026; v1.1 / v1.2 the same day; v1.21 on 15 SEP; v1.22 on 14 OCT 2026
(the v1.21 post is backed up in %TEMP%\skyeye_packet_backup_2026-10-14).
Paste everything below the line into Discord. Tables stay in a code fence: Discord does not
render markdown tables.

KEEP IN STEP WITH THE PDF:
- Bump the version/date in the title whenever the PDF version changes.
- v1.22 describes SkyeyeATC origin/main 47c9898. POST IT ONLY AFTER the Caucasus rig has
  pulled, run build.bat and restarted Command with start_command_caucasus.bat (that is what
  starts Hitman on 61.000 FM), AND Caucasus Training V1.23 Day/Night are uploaded - the V1.22
  missions have no 61.000 preset.
- Caveat not in the post: Hitman's checks need the rig's SKYEYE_TACVIEW; without it he clears
  hot on the calls alone.
- v1.22 also corrects Marshal to 306.300 / COMM 1 CH 3 everywhere and the Syria tower card (five fields).
-->

---

🎯 **NEW: Hitman 1-1, an AI JTAC for the Caucasus training 9-lines**
**SkyEye ATC Comms Packet v1.22 (14 OCT 2026) · Caucasus Training V1.23 Day + Night**

You can now work the five 9-line targets in Caucasus training with an AI JTAC. He briefs you, checks your run and clears you hot, or waves you off.

**Where to find him: 61.000 FM**
```
JET      PRESET          LABEL
Hornet   COMM 2 CH 12    JTAC HITMAN 1-1
Viper    COMM 2 CH 8     JTAC HITMAN 1-1
```
• Viper: **AXEMAN 41.100 has moved to COMM 2 CH 16.**
• **JTAC LIVE 60.000** (Hornet COMM 2 CH 15, Viper COMM 2 CH 12) is still there for a human JTAC. Hitman never answers on it.

**The run**
1️⃣ **Command (282.0):** "Command, Raider 1, request tasking" → *"contact Hitman one one on 61.0 for tasking."* Say "copy" and switch.
2️⃣ **Check in:** "Hitman, Raider 1, checking in". **Hitman picks your target** and gives the 9-line by reference to your kneeboard card: target number and name, elevation, final attack heading, egress. If you're not at the IP yet, he tells you where it is.
3️⃣ "Hitman, Raider 1, **copy**" → *"report IP inbound."*
4️⃣ "Hitman, Raider 1, **IP inbound**" → *"continue."*
5️⃣ "Hitman, Raider 1, **in from the south**" → *"cleared hot."*
6️⃣ "Hitman, Raider 1, **off**" → egress per line 9. Say "request tasking" to Hitman for another target.

**He's watching (via Tacview)**
• **IP inbound** only counts within 5 nm of the IP.
• **The in call** must be inside the card's **final attack heading** and 0.5–15 nm from the target. If it isn't, you get *"abort, abort, abort"* and the reason. Go round from the IP.
• **After cleared hot:** if you swing more than 10° outside the final attack heading while you're still pointed at the target and before anything comes off, he calls the abort.
• If SkyEye can't see you, there are no checks, and you're still cleared hot.

**Comms Packet v1.22**
• Two new cards: **12 Hitman 1-1** (the run) and **13 Cleared hot?** (what he checks).
• Command's card has the tasking call.
• The old cards 12–19 are now 14–21.
• **Marshal is 306.300 (COMM 1 CH 3) in every theatre.** v1.21 said 306.100 for Syria and the Black Sea.
• The Syria towers card now shows the five Foothold fields.

**⚠ Know before you fly**
• **Lead every call with "Hitman", then your callsign with its number.**
• **Radio on FM** for 61.000. The Hornet and Viper both pick FM automatically in the 30–88 MHz band.
• **Fly the V1.23 mission:** V1.22 doesn't have the 61.000 preset.

Questions, or a call Hitman didn't answer? Post the exact words you said.
