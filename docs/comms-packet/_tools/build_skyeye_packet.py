# -*- coding: utf-8 -*-
"""vSFG-7 SKYEYE ATC COMMS PACKET — builder.

A full-screen 16:9 PDF (user ruling 2026-09-13: PDF only, Hornet + Viper, group patch)
covering the three SkyEye roles that are the same on every map: COMMAND, DECKBOSS and
MARSHAL, for Persian Gulf, Syria and Black Sea.

v1.2 adds the AIRFIELD TOWERS (cards 13-18 since v1.21; user asked 2026-09-13 "add some pages
for airfield atc"). Tower facts come from origin/main 922f871: pkg/controller/{controller,
address,sayagain,proactive}.go, pkg/composer/composer.go, pkg/airfield/*.go, cmd/atc/atis.go,
start_towers{,_caucasus,_syria}.bat. Trigger words were checked by running ParseIntent on the
printed phrases (scratch test, not committed). Tower presets are read off the Hornet/Viper
New Pilot Pack preset cards (PG, Black Sea) and the ratified Eastern Med Hornet card (Syria).

v1.21 (user asked 2026-09-15 for "v1.21") follows origin/main b20f6b2: Black Sea Marshal
306.100, the Syria handoff frequency fixed, BRC spoken magnetic, Marshal answering
"checking in" and BRC requests; and the tower parser fixes of 14bd4e3 ("miles", "N mile
final", held departures released), so those warnings are gone. Same day the user asked for
the Marshal calls and responses "exactly" (the marking-moms summary had left out "Marshal at
angels N"): cards 7-9 print each call and the composer's first wording verbatim
(pkg/composer/composer.go, flag defaults from cmd/atc/main.go), with example values.

v1.22 (user asked 2026-10-14 "add to the documentation pdf") adds the JTAC, HITMAN 1-1 on
61.000 FM (Black Sea / Caucasus training only), cards 12 - 13, from origin/main 47c9898:
cmd/atc/jtac.go, cmd/atc/jtac_bda.go, cmd/atc/command_tasking.go, start_command_caucasus.bat
(BDA from a8fd8ed). Command's tasking
call now only hands the flight to Hitman; Hitman picks the 9-line target. Presets are the
Caucasus Training V1.23 cards: Hornet COMM 2 CH 12, Viper COMM 2 CH 8. Cards 12 - 19 of
v1.21 are now 14 - 21. The Marshal frequencies were NOT re-checked for v1.22.

v1.24 (user asked 2026-10-16 "rebuild the comms packet PDFs") follows origin/main 74c6260:
the training V1.25 9-line range north of the ridge (cards 12 - 13: Valley Road Bridge
checkpoint, Kuban no-strike road bridge) and IP inbound accepted within 20 nm (jtac.go
jtacIPInboundNm, 2026-10-15), with "run in heading NNN" when you call it off the line.

v1.25 (user asked 2026-10-16 "update pdf") follows origin/main 15c969b: Hitman 2-1 on 62.000
FM (one sector per JTAC), the four BDA calls (card 13), and two new cards after Hitman -
14 RANGE AREAS (Command's "proceeding to Alpha" replies, A - O with the real threats) and
15 OVERLORD ACTUAL (permission flow, Irene / cleared to engage, off target, Winchester /
Bingo / Joker). Cards 14 - 21 of v1.24 are now 16 - 23.

v1.26 (user asked 2026-10-18 "add to master ATC pdf") follows origin/main 6208ef8: SkyEye has
an LSO. New card 10 PADDLES (128.300 AM, Black Sea Day / Night): ball call, groove calls,
wave-off, bolter - from pkg/lso/lso.go and cmd/atc/lso.go. Same day Paddles was added to Syria
(Foothold, start_lso_syria.bat), same 128.300. Every "no SkyEye LSO" line is gone
(cards 1, 2, 4, 5, the sortie and No answer?). Cards 10 - 23 of v1.25 are now 11 - 24.

v1.28 (user asked 2026-10-19 for an AAR card) follows origin/main 27454b6: new card 17
TEXACO, the SkyEye tanker net (Texaco 1-1, 252.100 AM, Black Sea Day / Night) - the vSFG-7
AAQ Aerial Refueling Procedure calls and Texaco's replies, from cmd/atc/tanker.go,
tanker_speed.go and command_position.go (the full sheet is docs/tanker-flow). Cards 17 - 24
of v1.27 are now 18 - 25.

v1.29 (user asked 2026-10-19 "add Texaco 3-1 to the comms packet") follows origin/main 1c8d29d:
card 17 now has both SkyEye tankers - Texaco 1-1 (KC-130, 252.100) and Texaco 3-1 (KC-135
MPRS, 252.300, --tanker2-freq) - with the same flow, plus the radio check Texaco now answers.

v1.30 (user asked 2026-10-20 "update documentation") follows origin/main 823cf86, the Day
2026-10-19 log fixes: Texaco check-out "moving on to mission" (card 17); Command in hot /
pickle / Fox / splash (card 12); tower "holding", "request handoff", "green jet", and an
emergency gets the runway it names plus souls / fuel copied (card 24).

v1.31 (user asked 2026-10-21 to add the new GCI) follows origin/main 16957f2 (GCI
fence checks and splash, gciAck): new cards 18 - 19 DARKSTAR and GCI CALLS - SkyEye's
GCI brain inside atc.exe (cmd/atc/gci.go, gci_recovery.go, pkg/gci), Darkstar 264.000 on
Black Sea Day / Night (Hornet COMM 1 CH 5, the Caucasus Training / Hornet New Pilot Pack
cards), Overlord 266.000 on Foothold (dial it). Every printed GCI call is in the Phraseology
Reference v2.5 and parsed by TestDocPhrases. Cards 18 - 25 of v1.30 are now 20 - 27.

v1.34 (user asked 2026-10-21 "update documentation") follows origin/main 0aab154: new card
27 MULTI-SHIP - one takeoff and one landing clearance for a flight ("flight of two", wingmen
named after the lead, "<lead> flight" replies, +20 s spacing per jet, a parking spot each;
pkg/controller/flight.go). Card 18: Viper COMM 1 CH 4 on the Black Sea; Foothold Overlord is
Hornet CH 7 / Viper CH 5 (CH 5 / CH 4 = 264 there reach the DCS AI E-3, not ours). Card 19:
terrain hides low bandits (pkg/terrain). Card 27 of v1.33 is now 28.

v1.35 (operator 2026-10-22 "go ahead and make comms kneeboards", separate folder for OMM)
follows origin/main 4721177: new card 20 COMBAT CALLS - Fox / splash copied by the AWACS or
Command with no station name, "302 ejecting" -> "Raider 302 down, launching CSAR.", and hit /
damaged / mayday -> the nearest friendly field (cmd/atc/combat_calls.go, emergency_divert.go;
Foothold skips red-held fields). Cards 20 - 28 of v1.34 are now 21 - 29. Every card is also
written as a DCS kneeboard page to Documentation/OMM/SkyEye ATC Kneeboards (an OMM mod:
Kneeboard/FA-18C_hornet and Kneeboard/F-16C_50 under Saved Games/DCS).

v1.36 (operator 2026-10-22): Paddles' abeam reply is now "Paddles contact, 302, Hornet in the
groove, at three-quarters of a mile call the ball." (card 10; kneeboard page 2 shows channels).

v1.39 (operator 2026-10-27, "update the comms packet") follows origin/main 9de1eb0 (Readback
ATC 0.3.2): Case III approach times ten minutes apart, "radar contact" on commencing, Marshal's
"eight miles, dirty up", and the Case III bolter / wave-off pattern - climb to 1,200, turn
downwind at 4 miles, report abeam, back to Paddles at 3 miles (cards 6, 9).

v1.38 (operator 2026-10-25, after Raider 319's Case III on Night) follows origin/main abe9c29
(Readback ATC 0.2.6): Case III to the squadron's DimOn cards 26 - 27 - the stack starts at angels 6,
DME = angels + 15, marshal radial = final bearing + 180, "marshal on the NNN, DD DME, angels N,
expected approach time is MM", Marshal hands the jet to Paddles at 3 miles (cards 6, 9, 10, 13).

v1.27 (operator 2026-10-18): red deck / green deck. After a trap the pilot calls "red deck",
then "green deck" once clear of the landing area; no green deck by the time the next jet is
at 1/4 nm (or Tacview shows the landing area foul) and Paddles calls "<callsign>, foul deck,
wave off, wave off, proceed to bolter pattern." Card 10 only.

Every call and reply below was read out of the SkyeyeATC code on origin/main —
cmd/atc/{command,deckboss,marshal,command_handoff}.go, pkg/composer/composer.go,
pkg/state/state.go — and the start_*.bat files. The *_responses.md docs were used as a map
only; where a doc and the code disagreed, the code won (e.g. the recovery-case ceiling is
1,500 ft, not the 1,000 ft in marshal_responses.md).

FREQUENCIES — "what SkyEye uses", per theatre:
  Command  282.000  all theatres      (start_command*.bat)
  Deckboss 128.600  all theatres      (start_deckboss{,_syria,_caucasus}.bat)
  Marshal  306.300  Persian Gulf      (start_marshal.bat)
           306.300  Syria             (start_marshal_syria.bat)
           306.300  Black Sea         (start_marshal_caucasus.bat, operator ruling 2026-09-20)
  Hitman   61.000 FM  Black Sea      (start_command_caucasus.bat --jtac-freq)
  GCI      264.000    Black Sea       Darkstar (start_gci_caucasus.bat; Kola too, unconfirmed)
           266.000    Syria           Overlord (start_gci_syria.bat)
  Towers and ATIS: docs/phraseology/facts.json (cmd/atc/docs_test.go reads the bats)

Run:  python "_tools\\build_skyeye_packet.py"   (from the packet folder)
"""
import os
import json
import shutil
import sys

MISSIONS = r"C:\Users\afhin\Saved Games\DCS\Missions"
sys.path.insert(0, os.path.join(MISSIONS, "_MISSION_TOOLKIT", "briefing"))
import missioncard as MC  # noqa: E402,F401
import newpilot_kit as NK  # noqa: E402  guards + patch + draw-call recording
import newpilot_slides as NS  # noqa: E402  the 16:9 PDF

PACKET = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# The pilot handouts folder (2026-10-21 tidy; was Desktop\SkyEye ATC Comms Packet).
DESKTOP_COPY = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"
CACHE = os.path.join(PACKET, "_tools", "_cache")
CARDS = os.path.join(CACHE, "cards")      # kneeboard renders, only used to feed the PDF
# vSFG-7 group patch, docs/vsfg7_patch.png (operator 2026-10-22: "use this logo and update
# anything we have future with this one"; was Missions\New Pilot packs\VMFA225_patch.png).
LOGO = os.path.join(os.path.dirname(PACKET), "vsfg7_patch.png")
NK.set_logo(LOGO, "circle")

VER = "v1.39"
DATE = "27 OCT 2026"   # release date of VER - change it together with VER
STAMP = "%s  ·  %s" % (VER, DATE)

EYE_ALL = "vSFG-7  ·  SKYEYE ATC  ·  HORNET + VIPER"
EYE_CVN = "vSFG-7  ·  SKYEYE ATC  ·  HORNET  ·  CARRIER"
EYE_TWR = "vSFG-7  ·  SKYEYE ATC  ·  HORNET + VIPER  ·  TOWERS"

# ---------------------------------------------------------------------------- facts
COMMAND_F, DECKBOSS_F = "282.000", "128.600"
MARSHAL_F = {"PERSIAN GULF": "306.300", "SYRIA": "306.300", "BLACK SEA": "306.300"}
# Hornet preset that reaches each SkyEye role (read off the squadron cards)
H_CMD, H_DKB = "COMM 1 CH 4", "COMM 1 CH 1"
H_MSH = {"PERSIAN GULF": "COMM 1 CH 3", "SYRIA": "COMM 1 CH 3", "BLACK SEA": "COMM 1 CH 3"}
V_CMD = "COMM 1 CH 1"
# JTAC (cmd/atc/jtac.go, --jtac-freq in start_command_caucasus.bat); Caucasus Training V1.23 presets
JTAC_F, H_JTAC, V_JTAC = "61.000", "COMM 2 CH 12", "COMM 2 CH 8"
# Hitman 2-1 (--jtac2-freq, 2026-10-16): no kneeboard preset yet - dial it.
JTAC2_F = "62.000"
# Paddles, the LSO (cmd/atc/lso.go, start_lso_caucasus.bat, 2026-10-17): Caucasus Day / Night
# only; no kneeboard preset yet - dial it.
PADDLES_F = "128.300"
# Texaco 1-1, the tanker net (cmd/atc/tanker.go, --tanker-freq in start_command_caucasus.bat,
# 2026-10-19): Black Sea Day / Night; Hornet COMM 2 CH 8 on the Caucasus Training cards.
TANKER_F, H_TANKER = "252.100", "COMM 2 CH 8"
# Texaco 3-1, the second tanker net (--tanker2-freq, 2026-10-19): KC-135 MPRS, COMM 2 CH 10.
TANKER2_F, H_TANKER2 = "252.300", "COMM 2 CH 10"
# Texaco 2-1 and the Shells (--tanker-net, operator 2026-10-22: "2 Shells for 16's and Texaco
# 2-1 for 18's"): Texaco 2-1 KC-135 MPRS Hornet COMM 2 CH 9; Shell 1-1 / 2-1 KC-135 boom on
# Viper COMM 1 CH 7 / CH 8 (the Caucasus Training V1.35 ladder).
TANKER21_F, H_TANKER21 = "252.200", "COMM 2 CH 9"
SHELL_F, V_SHELL = "253.100", "COMM 1 CH 7"
SHELL2_F, V_SHELL2 = "253.200", "COMM 1 CH 8"

# GCI (cmd/atc/gci.go, start_gci_{caucasus,syria}.bat, 2026-10-21). Black Sea: Hornet COMM 1
# CH 5 "DARKSTAR 1" on the Caucasus Training and Hornet New Pilot Pack cards; Viper COMM 1 CH 4.
# Foothold (DTC_SY_1.0 carts, checked 2026-10-21): Overlord 266 = Hornet CH 7, Viper CH 5.
H_GCI, V_GCI = "COMM 1 CH 5", "COMM 1 CH 4"
H_GCI_SY, V_GCI_SY = "CH 7", "CH 5"

TOTAL = 30
# Generated frequencies shared with the Phraseology Reference (docs/phraseology).
FACTS = json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..",
                                    "phraseology", "facts.json"), encoding="utf-8"))
TOWER_CARDS = "cards 23 - 29"
_CAU = next(t for t in FACTS if t["key"] == "caucasus")
_SYR = next(t for t in FACTS if t["key"] == "syria")
GCI_BS = (_CAU["gci_name"], "%.3f" % _CAU["gci"])     # Darkstar 264.000
GCI_SY = (_SYR["gci_name"], "%.3f" % _SYR["gci"])     # Overlord 266.000


def card(title_plain, title_gold, sublines, n, eyebrow=EYE_ALL):
    c = NK.new_card(eyebrow, title_plain, title_gold, sublines, (n, TOTAL))
    c.footer("vSFG-7  ·  SKYEYE ATC  ·  %s" % STAMP, "CARD %d / %d" % (n, TOTAL))
    return c


def save(c, name):
    path = os.path.join(CARDS, name)
    NK.save_card(c, path, name)
    return path


# ============================================================================ cards
def c01_how():
    c = card("SKYEYE ", "ATC", ["AI controllers on SRS  ·  Command, Deckboss, Marshal, towers",
                               "Persian Gulf  ·  Syria  ·  Black Sea"], 1)
    NK.band(c, "WHAT IT IS", "read this first")
    c.bullets([
        [("An AI controller that talks to you over SRS.", True),
         ("It hears you with speech recognition, answers in a generated voice, and sees "
          "where you are through Tacview - so when you say \"10 DME\" it can check.", False)],
        [("It listens for key words, not exact sentences.", True),
         ("Speak normally, but use the words on these cards - they are the triggers the "
          "code actually matches.", False)],
    ])
    NK.band(c, "THE ROLES", "same calls in every theatre")
    c.kv([("COMMAND  -  squadron net", "%s AM" % COMMAND_F),
          ("DECKBOSS  -  carrier deck", "%s AM" % DECKBOSS_F),
          ("MARSHAL  -  carrier stack", "306.300 AM  ·  every theatre"),
          ("PADDLES  -  LSO, Black Sea + Syria", "%s AM  ·  card 10" % PADDLES_F),
          ("HITMAN 1-1 / 2-1  -  JTAC, Black Sea", "%s / %s FM  ·  cards 13 - 14" % (JTAC_F[:4], JTAC2_F[:4])),
          ("RANGE AREAS + OVERLORD  -  on Command", "cards 15 - 16"),
          ("TEXACO 1-1 / 2-1 / 3-1  -  Hornet tankers", "252.1 / .2 / .3 AM  ·  card 17"),
          ("SHELL 1-1 / 2-1  -  Viper boom tankers", "%s / %s AM  ·  card 18" % (SHELL_F[:5], SHELL2_F[:5])),
          ("DARKSTAR / OVERLORD  -  GCI", "%s / %s AM  ·  cards 19 - 20" % (GCI_BS[1][:5], GCI_SY[1][:5])),
          ("COMBAT CALLS  -  shots, CSAR, damaged jet", "AWACS or Command  ·  card 21"),
          ("TOWERS  -  one per airfield", TOWER_CARDS)])
    NK.band(c, "HOW TO BE HEARD", "every call")
    c.bullets([
        [("Lead with the role name, then your callsign.", True),
         ("\"Marshal, Raider 1, ...\" - for a tower, the field name. Marshal ignores any call "
          "that does not start with its name; Deckboss ignores most of them.", False)],
        [("Say your full callsign every time.", True), ("\"Raider 1\", not \"1\".", False)],
        [("Pause half a second after you key the mic,", True),
         ("then speak. Calls shorter than about 0.2 s are thrown away.", False)],
        [("Wait 2 - 3 seconds for the answer.", True),
         ("Recognition, the reply and the voice take that long. Do not re-key over it.", False)],
        [("Radios AM (Hitman FM), encryption off.", True), ("Keep each call short - two short "
                                                            "calls beat one long one.", False)],
    ])
    NK.subtle(c, "ATIS is listen-only. Paddles answers your ball call; the rest is his calls.")
    return save(c, "01_how.jpg")


def c02_freqs():
    c = card("FREQUEN", "CIES", ["Where SkyEye actually listens  ·  per theatre",
                                 "Hornet preset shown for each; Vipers use Command only."], 2)
    for th in ("PERSIAN GULF", "SYRIA", "BLACK SEA"):
        NK.band(c, th, "Hornet preset")
        rows = [("COMMAND  %s" % COMMAND_F, "%s  ·  Viper %s" % (H_CMD, V_CMD)),
                ("DECKBOSS  %s" % DECKBOSS_F, H_DKB),
                ("MARSHAL  %s" % MARSHAL_F[th], H_MSH[th])]
        if th == "BLACK SEA":
            rows.append(("%s  %s  -  GCI" % (GCI_BS[0].upper(), GCI_BS[1]), H_GCI))
        if th == "SYRIA":
            rows.append(("%s  %s  -  GCI" % (GCI_SY[0].upper(), GCI_SY[1]), "dial it"))
        c.kv(rows)
    NK.band(c, "READING THE PRESET CARDS", "read this")
    c.bullets([
        [("\"AI\" on a carrier preset means SkyEye.", True),
         ("COMM 1 CH 3 \"CVN-72 AI MARSHALL\" 306.300 is SkyEye Marshal in every theatre "
          "(operator ruling 2026-09-20).", False)],
        [("COMM 1 CH 2 \"CVN-72 MARSHAL\" 306.200 is for a live human Marshal.", True),
         ("SkyEye never answers there.", False)],
        [("Deckboss 128.600 is shared", True),
         ("with the DCS AI carrier controller - you may hear both.", False)],
        [("SkyEye Paddles is %s - dial it." % PADDLES_F, True),
         ("Black Sea and Syria. LSO 128.100 on the cards is for a human LSO.", False)],
    ])
    NK.subtle(c, "Voices: towers British or Australian  ·  Command, Marshal, Deckboss American  ·  Hitman 1-1 Australian.")
    return save(c, "02_freqs.jpg")


def c03_deckboss_flow():
    c = card("DECK", "BOSS", ["Carrier deck  ·  %s AM  ·  Hornet COMM 1 CH 1" % DECKBOSS_F,
                             "Cat assignment, conga line, launch."], 3, EYE_CVN)
    NK.band(c, "THE LAUNCH", "in order")
    NK.steps(c, [
        ("REQUEST TAXI", "\"Deckboss, Raider 1, request taxi\" - or green jet."),
        ("", "Reply: cleared to cat N, or join the conga line."),
        ("TENSION", "\"Deckboss, Raider 1, ready cat 2\" - or shoot."),
        ("", "Reply: under tension, spin it up, shooter's discretion."),
        ("T + 10 s", "Your cat is freed; the next jet hears cat N is clear."),
        ("AIRBORNE", "\"Raider 1, airborne\" - no address needed."),
        ("", "Reply: copy airborne, push Marshal and its frequency."),
    ], split=250)
    NK.band(c, "THE RULES")
    c.bullets([
        [("Deckboss does not call the shot.", True),
         ("Nobody says shoot on the radio - the shooter fires you. Run up when you hear "
          "\"spin it up\".", False)],
        [("Four cats: 1 and 2 on the bow, 3 and 4 on the waist.", True),
         ("If all four are taken you join the conga line; it holds six. After that you hear "
          "\"deck is full, hold clear\".", False)],
        [("Checking in twice does not take a second cat.", True),
         ("If you are already in the line you are told your number.", False)],
    ])
    return save(c, "03_deckboss_flow.jpg")


def c04_deckboss_calls():
    c = card("DECKBOSS ", "CALLS", ["Every call it answers  ·  lead with \"Deckboss,\"",
                                    "The airborne call is the only one that works without it."],
             4, EYE_CVN)
    NK.band(c, "YOU SAY", "Deckboss replies")
    c.kv([
        ("request taxi  /  ready for taxi  /  green jet", "cat N or conga"),
        ("ready cat N  /  tension cat N  /  shoot", "under tension"),
        ("airborne  /  clear traffic", "push Marshal"),
        ("remain in bolter", "contact LSO"),
        ("say BRC  /  request BRC", "mother's BRC, magnetic"),
        ("radio check  /  comm check  /  how copy", "loud and clear"),
        ("pushing Marshal  /  pushing command", "short ack"),
    ])
    NK.band(c, "SAMPLE", "Raider 1 and Raider 2 on the deck")
    NK.steps(c, [
        ("RAIDER 1", "Deckboss, Raider 1, request taxi."),
        ("DECKBOSS", "Raider 1, Deckboss, cleared to cat two."),
        ("RAIDER 1", "Deckboss, Raider 1, ready cat two."),
        ("DECKBOSS", "Raider 1, Deckboss, under tension cat two,"),
        ("", "spin it up, shooter's discretion."),
        ("DECKBOSS", "Raider 2, Deckboss, cat two is clear."),
        ("RAIDER 1", "Raider 1, airborne."),
        ("DECKBOSS", "Raider 1, Deckboss, copy airborne,"),
        ("", "push Marshal, three zero six point three."),
    ], split=220)
    NK.subtle(c, "Bolter practice: Deckboss hands you to the LSO - Paddles %s in the Black Sea and Syria." % PADDLES_F)
    return save(c, "04_deckboss_calls.jpg")


def c05_marshal_case1():
    c = card("MARSHAL  ", "CASE I", ["Carrier stack  ·  306.300  ·  every theatre",
                                    "Lead every call with \"Marshal,\" - or it is ignored."],
             5, EYE_CVN)
    NK.band(c, "THE RECOVERY", "Case I and Case II use these calls")
    NK.steps(c, [
        ("MARKING MOMS", "Marshal, Raider 1, marking moms, 30 DME,"),
        ("", "angels 12, state 6.5."),
        ("", "Reply: radar contact, weather, case, BRC,"),
        ("", "altimeter, Marshal at angels N,"),
        ("", "report see me at ten."),
        ("DME REPORTS", "Marshal, Raider 1, 20 DME - optional, acked."),
        ("SEE YOU AT 10", "Reply: visual at ten, say state."),
        ("STATE", "Marshal, Raider 1, state 5.2 - under 2.0 is priority."),
        ("ESTABLISHED", "Marshal, Raider 1, established angels 3"),
        ("", "Reply: signal Charlie - or your stack position, hold."),
        ("COMMENCING", "Reply: call three mile initial."),
        ("INITIAL", "At 3 nm. Reply: contact paddles, good luck."),
    ], split=280)
    NK.band(c, "THE STACK")
    c.bullets([
        [("\"Marshal at angels N\" is your stack altitude.", True),
         ("The lowest free altitude from angels 2 to angels 9. Marshal counts aircraft already "
          "in the stack and any contact within 50 nm at that altitude.", False)],
        [("When someone commences, the stack steps down silently.", True),
         ("There is no radio call for it - keep flying your assigned angels.", False)],
        [("After initial, switch to Paddles.", True),
         ("Black Sea and Syria: SkyEye Paddles on %s, card 10. Gulf: fly the ball and trap." % PADDLES_F, False)],
    ])
    NK.band(c, "ANY TIME")
    c.bullets([
        [("\"Marshal, Raider 1, say BRC\"", True),
         ("- mother's BRC, magnetic. \"request BRC\" and a bare \"BRC\" work too.", False)],
        [("\"Marshal, Raider 1, checking in\"", True),
         ("- your range and bearing from mother, then marking moms please.", False)],
    ])
    NK.subtle(c, "Word-for-word calls and replies: cards 7 - 9.")
    return save(c, "05_marshal_case1.jpg")


def c06_marshal_case3():
    c = card("MARSHAL  ", "CASE III", ["Night or low weather  ·  same trigger words, new answers",
                                      "Marshal picks the case itself from time and weather."],
             6, EYE_CVN)
    NK.band(c, "WHICH CASE", "decided by SkyEye")
    c.kv([
        ("Night (local 1800 - 0600)", "CASE III"),
        ("Ceiling under 1,500 ft  or  vis under 3 nm", "CASE III"),
        ("Ceiling under 3,000 ft  or  vis under 5 nm", "CASE II"),
        ("Otherwise", "CASE I"),
    ])
    c.bullets([[("If the case changes while you are inbound,", True),
                ("you hear: \"All marshal aircraft, recovery is now Case Three, acknowledge on "
                 "next push.\"", False)]])
    NK.band(c, "CASE III CALLS", "what changes")
    NK.steps(c, [
        ("MARKING MOMS", "Reply: marshal on the NNN, DD DME, angels N,"),
        ("", "expected approach time is MM, report established."),
        ("ESTABLISHED", "Reply: approach time MM. There is no Charlie."),
        ("COMMENCING", "At your MM. Reply: radar contact, descend to"),
        ("", "platform 5,000, final bearing NNN,"),
        ("", "confirm BRC NNN, report platform."),
        ("PLATFORM", "Passing 5,000 ft. Reply: final bearing NNN."),
        ("8 MILES", "Marshal: eight miles, dirty up. No reply."),
        ("3 MILES", "Marshal: contact paddles, call the ball. Switch and"),
        ("", "make the ball call at three-quarters. No initial call."),
        ("BOLTER", "Marshal: climb to 1,200, at 4 miles turn"),
        ("", "downwind, report abeam. Abeam: turn final,"),
        ("", "Paddles again at 3 miles."),
    ], split=280)
    c.bullets([
        [("Marshal radial = final bearing + 180.  Final bearing = BRC - 9.", True),
         ("All magnetic. The stack starts at angels 6; the DME is angels + 15 (angels 6 at 21 DME).", False)],
        [("MM is minutes past the hour.", True),
         ("The first aircraft commences 10 minutes out; each one after is 10 minutes later. Miss "
          "your time and Marshal gives you a new one.", False)],
    ])
    return save(c, "06_marshal_case3.jpg")


# ------------------------------------------------------------ Marshal, word for word
# Every reply below is the FIRST wording of the composer method Marshal calls
# (pkg/composer/composer.go on b20f6b2), filled with the example values in MSH_EX.
# Marshal picks one of 2-4 wordings at random, so a live reply can reorder the same facts.
# Numbers are spoken the way the composer writes them: numberWord spells 0-10 only
# ("angels two", but "angels 12", "30 miles"); bearings, radials and the altimeter
# are digit by digit; BRC is three digits ("BRC 275").
MSH_EX = "Example: BRC 275, alt 29.92, angels 12, 30 nm on the 340."
MSH_NOTE = ("Marshal picks one of 2 - 4 wordings at random - the first is shown. "
            "Same facts, same order of calls.")


def c07_marshal_calls_inbound():
    c = card("MARSHAL ", "CALLS 1", ["Word for word  ·  check-in, marking moms, BRC, radar",
                                     MSH_EX], 7, EYE_CVN)
    NK.band(c, "CHECKING IN", "optional first call")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, checking in."),
        ("MARSHAL", "Raider 1, Union Marshal, radar contact, angels 12,"),
        ("", "30 miles from mother, bearing three four zero,"),
        ("", "say marking mom's and state."),
        ("NO RADAR", "Raider 1, Union Marshal, go ahead with"),
        ("", "marking mom's and state."),
    ], split=150)
    NK.band(c, "MARKING MOMS", "Case I and Case II")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, marking moms 340 for 30,"),
        ("", "angels 12, state 6.5."),
        ("MARSHAL", "Raider 1, Union Marshal, radar contact, angels 12,"),
        ("", "range 30 miles, bearing three four zero from mother,"),
        ("", "mother's weather ceiling unrestricted,"),
        ("", "visibility ten plus, expect Case One recovery,"),
        ("", "BRC 275, altimeter two niner niner two,"),
        ("", "Marshal at angels two, report see me at ten."),
        ("2+ IN STACK", "The reply ends: Stack has 2 aircraft."),
        ("NO RADAR", "The radar contact part is left out."),
    ], split=150)
    NK.band(c, "BRC", "any time")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, say BRC."),
        ("MARSHAL", "Raider 1, Union Marshal, mother's BRC is 275."),
        ("NO CARRIER", "Raider 1, Union Marshal, BRC unknown, mother off scope."),
    ], split=150)
    NK.band(c, "RADAR CHECK AND RADIO CHECK")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, radar check."),
        ("MARSHAL", "Raider 1, Union Marshal, radar contact, angels 12,"),
        ("", "range 30 miles, bearing three four zero from mother."),
        ("NO RADAR", "Raider 1, Union Marshal, negative radar contact,"),
        ("", "say posit relative mother."),
        ("YOU", "Marshal, Raider 1, radio check."),
        ("MARSHAL", "Raider 1, Union Marshal, loud and clear."),
    ], split=150)
    NK.subtle(c, MSH_NOTE)
    return save(c, "07_marshal_calls_inbound.jpg")


def c08_marshal_calls_stack():
    c = card("MARSHAL ", "CALLS 2", ["Word for word  ·  inbound to the trap, and the departure",
                                     "Case I and Case II. Example as card 7."], 8, EYE_CVN)
    NK.band(c, "INBOUND", "in order")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, 20 DME."),
        ("MARSHAL", "Raider 1, Union Marshal, radar contact, 20 DME, continue."),
        ("YOU", "Marshal, Raider 1, see you at ten."),
        ("MARSHAL", "Raider 1, Union Marshal, visual carrier at ten miles,"),
        ("", "say state."),
        ("YOU", "Marshal, Raider 1, state 5.2."),
        ("MARSHAL", "Raider 1, Union Marshal, copy state 5.2."),
        ("UNDER 2.0", "Raider 1, Union Marshal, state 1.8, expedite recovery."),
    ], split=150)
    NK.band(c, "THE STACK", "to the trap")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, established angels two."),
        ("DECK CLEAR", "Raider 1, Union Marshal, signal Charlie."),
        ("DECK BUSY", "Raider 1, Union Marshal, entering stack at angels two,"),
        ("", "position one, hold for Charlie."),
        ("YOU", "Marshal, Raider 1, commencing, state 4.0."),
        ("MARSHAL", "Raider 1, Union Marshal, copy commencing, state 4.0,"),
        ("", "check in three mile initial."),
        ("YOU", "Marshal, Raider 1, initial."),
        ("MARSHAL", "Raider 1, Union Marshal, contact paddles, good luck."),
    ], split=150)
    NK.band(c, "OFF THE BOAT", "after Deckboss pushes you")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, clear seven miles."),
        ("MARSHAL", "Raider 1, Union Marshal, clear of Union control zone,"),
        ("", "push vSFG-7-Command, two eight two point zero,"),
        ("", "channel four, for tasking."),
        ("YOU", "Marshal, Raider 1, pushing command."),
        ("MARSHAL", "Raider 1, Union Marshal, cleared handoff to"),
        ("", "vSFG-7-Command, good day."),
    ], split=150)
    NK.subtle(c, MSH_NOTE)
    return save(c, "08_marshal_calls_stack.jpg")


def c09_marshal_calls_case3():
    c = card("MARSHAL ", "CALLS 3", ["Word for word  ·  Case III (night or low weather)",
                                     "Example: BRC 275, ceiling 800, vis 2, approach time 15."],
             9, EYE_CVN)
    NK.band(c, "MARKING MOMS", "Case III")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, marking moms 340 for 30,"),
        ("", "angels 12, state 6.5."),
        ("MARSHAL", "Raider 1, Union Marshal, radar contact, angels 12,"),
        ("", "range 30 miles from the Abe, bearing three four zero,"),
        ("", "mother's weather ceiling eight hundred overcast,"),
        ("", "visibility 2, Case Three recovery, BRC 275,"),
        ("", "altimeter two niner niner two,"),
        ("", "marshal on the zero eight six, two one DME, angels six,"),
        ("", "expected approach time is one five, report established."),
    ], split=150)
    NK.band(c, "HOLD TO PLATFORM", "in order")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, established angels six."),
        ("MARSHAL", "Raider 1, Union Marshal, roger established angels six,"),
        ("", "marshal on the zero eight six, two one DME,"),
        ("", "expected approach time is one five."),
        ("YOU", "Marshal, Raider 1, commencing."),
        ("MARSHAL", "Raider 1, Union Marshal, copy commencing, radar contact,"),
        ("", "descend to platform five thousand, final bearing"),
        ("", "two six six, confirm BRC two seven five, report platform."),
        ("YOU", "Marshal, Raider 1, platform."),
        ("MARSHAL", "Raider 1, Union Marshal, copy platform,"),
        ("", "final bearing two six six."),
        ("8 MILES", "Raider 1, Union Marshal, eight miles, dirty up."),
        ("3 MILES", "Raider 1, Union Marshal, three miles,"),
        ("", "contact paddles, call the ball."),
    ], split=150)
    NK.band(c, "BOLTER / WAVE-OFF", "Marshal sees it, or you call it")
    NK.steps(c, [
        ("YOU", "Raider 1, bolter.   (or: Raider 1, wave off.)"),
        ("MARSHAL", "Raider 1, Union Marshal, roger bolter, climb to one"),
        ("", "thousand two hundred, at four miles turn downwind,"),
        ("", "report abeam."),
        ("YOU", "Raider 1, abeam."),
        ("MARSHAL", "Raider 1, Union Marshal, roger abeam, four miles past"),
        ("", "abeam turn final, final bearing two six six."),
    ], split=150)
    NK.band(c, "CASE CHANGE", "to everyone in the stack")
    NK.steps(c, [
        ("MARSHAL", "All marshal aircraft, Union Marshal, recovery is now"),
        ("", "Case Three, acknowledge on next push."),
    ], split=150)
    c.bullets([
        [("Radial 086 = final bearing 266 + 180.  Final bearing = BRC 275 - 9.", True),
         ("\"Approach time one five\" is 15 minutes past the hour; the next jet's is ten minutes later.", False)],
    ])
    NK.subtle(c, MSH_NOTE)
    return save(c, "09_marshal_calls_case3.jpg")


# --------------------------------------------- Paddles, the LSO (pkg/lso, cmd/atc/lso.go)
def c10_paddles():
    c = card("PADD", "LES", ["SkyEye LSO  ·  %s AM  ·  Black Sea + Syria  ·  dial it" % PADDLES_F,
                            "You call the ball; he talks you down. No readbacks."], 10, EYE_CVN)
    NK.band(c, "ABEAM AND THE BALL CALL", "after Marshal's \"contact paddles\"")
    NK.steps(c, [
        ("ABEAM", "302 abeam."),
        ("PADDLES", "Paddles contact, 302, Hornet in the groove,"),
        ("", "at three-quarters of a mile call the ball."),
        ("YOU", "302, Hornet ball, 5.5."),
        ("PADDLES", "Roger ball."),
        ("CLARA", "302, Clara, 5.0.  ->  Paddles contact, keep it coming."),
        ("NO CALL", "By 0.6 nm: Call the ball."),
        ("RADIO CHECK", "Paddles, Raider 302, radio check.  ->  loud and clear."),
    ], split=150)
    NK.band(c, "IN THE GROOVE", "he calls, you fly")
    c.kv([
        ("a little high  /  high", "A little high.  /  You're high."),
        ("still high after 3 s", "You're high, bring it down."),
        ("low, far out  /  in close", "You're low, power.  /  Power!"),
        ("still low  /  low and sinking", "POWER!  /  Power, power, POWER!"),
        ("over-correcting after power", "Easy with it."),
        ("left  /  right of the line", "Right for lineup.  /  Come left."),
        ("Hornet AoA off by 2", "You're fast.  /  You're slow."),
    ])
    NK.band(c, "WAVE OFF AND BOLTER")
    c.kv([
        ("low or off line in close", "Wave off, wave off, wave off."),
        ("deck foul at 1/4 nm", "302, foul deck, wave off, wave off, proceed to bolter pattern."),
        ("past the wires", "Bolter, bolter, bolter."),
        ("trap", "nothing - the grade goes in the log"),
    ])
    NK.subtle(c, "After your trap, on 128.3: \"red deck\", then \"green deck\" once clear - Paddles sends you to Deckboss.")
    NK.subtle(c, "Say the modex first - with two in the groove it is how he knows whose ball.")
    return save(c, "10_paddles.jpg")


def c10_departure():
    c = card("OFF THE ", "BOAT", ["Launch to Command  ·  Deckboss -> Marshal -> Command",
                                  "Three legs, three frequencies."], 11, EYE_CVN)
    NK.band(c, "THE CHAIN", "in order")
    NK.steps(c, [
        ("1  DECKBOSS", "Raider 1, airborne."),
        ("", "Reply: copy airborne, push Marshal and its frequency."),
        ("2  MARSHAL", "Marshal, Raider 1, clear seven miles."),
        ("", "Reply: clear of Union control zone,"),
        ("", "push vSFG-7-Command,"),
        ("", "two eight two point zero, channel four, for tasking."),
        ("3  COMMAND", "Command, Raider 1, checking in."),
        ("4  %s" % GCI_BS[0].upper(), "Black Sea: %s %s for picture - card 19." % (GCI_BS[0], GCI_BS[1][:5])),
    ], split=250)
    NK.band(c, "GET THE CLEAR CALL RIGHT")
    c.bullets([
        [("Say \"clear\" and a distance:", True),
         ("\"clear seven miles\", \"clear seven DME\", \"clear of mother\" or \"clear "
          "mothers\".", False)],
        [("Do not add recovery words to it.", True),
         ("If the call also has marking moms, state, initial, established, commencing, "
          "platform or bolter in it, Marshal treats you as inbound.", False)],
        [("\"Channel four\" is the Hornet's Command preset.", True),
         ("Vipers never launch from the boat - their Command is COMM 1 CH 1.", False)],
        [("Deckboss names the right Marshal for the theatre:", True),
         ("306.300 in every theatre.", False)],
    ])
    return save(c, "10_departure.jpg")


def c11_command():
    c = card("COMM", "AND", ["%s AM  ·  Hornet COMM 1 CH 4  ·  Viper COMM 1 CH 1" % COMMAND_F,
                             "The squadron net. Both jets, every theatre."], 12)
    NK.band(c, "YOU SAY", "Command replies")
    c.kv([
        ("radio check  /  comm check  /  how copy", "loud and clear"),
        ("checking in  /  check in", "proceed mission"),
        ("fence in  /  fence check", "cleared hot  ·  where the GCI is"),
        ("in hot  /  pickle  /  Fox 3  /  splash", "cleared hot  ·  copy"),
        ("302 ejecting  /  I'm hit  /  mayday", "CSAR  ·  nearest field  -  card 21"),
        ("on station", "good hunting"),
        ("off station  /  departing station", "proceed to assigned pattern"),
        ("fence out", "safe passage"),
        ("RTB  /  request recovery  /  bingo home plate", "expect handoff at 30 nm"),
        ("state 5.2", "copy state"),
        ("say bullseye  /  request bullseye", "bullseye NNN for NN"),
        ("request tanker  /  say texaco", "bearing, range, angels  ·  Black Sea: cards 17 - 18"),
        ("request tasking  (Black Sea training)", "contact Hitman 1-1 or 2-1  -  card 13"),
        ("proceeding to / request Alpha  (Black Sea)", "range areas  -  cards 15 - 16"),
    ])
    c.bullets([
        [("\"RTB, fence out\" is read as RTB.", True),
         ("The recovery words are checked before fence out.", False)],
        [("Add your state to any call.", True),
         ("\"fence out, state 1.8\" - under 2.0 Command tells you you are bingo.", False)],
    ])
    NK.band(c, "BULLSEYE AND TANKER", "from the radar picture")
    c.bullets([
        [("Bearings are magnetic.", True),
         ("Bullseye is your own side's. The tanker is the nearest friendly one - by type, or a "
          "Texaco, Arco or Shell callsign.", False)],
        [("No picture, no answer:", True),
         ("you hear \"unable\" or \"negative tanker on scope\".", False)],
    ])
    NK.band(c, "THE AUTOMATIC HANDOFF", "you do not ask for it")
    c.bullets([
        [("Once you have talked to Command, it watches you on Tacview", True),
         ("- when the rig has Tacview connected. Inside 30 nm of your nearest recovery - a "
          "field or the carrier - it calls you once.", False)],
        [("Field:", True), ("\"contact <field> tower on <frequency>, switching now approved, "
                            "good landing.\"", False)],
        [("Carrier:", True), ("\"contact Marshal on <frequency>, switching now approved, call "
                              "marking mom.\"", False)],
        [("Launched from the boat?", True),
         ("You are sent to Marshal inside 30 nm of it, even when a field is nearer.", False)],
    ])
    return save(c, "11_command.jpg")


# ---------------------------------------------------------------- JTAC (cmd/atc/jtac.go)
def c12_jtac_flow():
    c = card("HITMAN ", "1-1 / 2-1", ["1-1  %s FM  ·  Hornet %s  ·  Viper %s" % (JTAC_F, H_JTAC, V_JTAC),
                                     "2-1  %s FM  ·  no preset yet - dial it" % JTAC2_F], 13)
    NK.band(c, "THE RUN", "every call starts \"Hitman, Raider 1, ...\"")
    NK.steps(c, [
        ("COMMAND", "request tasking  ->  contact Hitman 1-1 or 2-1"),
        ("CHECK IN", "checking in  ->  Hitman picks the target, 9-line"),
        ("COPY", "copy  ->  report IP inbound"),
        ("IP INBOUND", "IP inbound  ->  continue"),
        ("IN", "in from the <dir>  ->  cleared hot, or abort"),
        ("OFF", "off  ->  egress per line 9"),
    ], split=190)
    NK.band(c, "THE 9-LINE", "by reference to your kneeboard card")
    c.bullets([
        [("Two JTACs, one sector each.", True),
         ("Hitman 1-1 runs Kuban Valley (1 - 3), Hitman 2-1 Cherkessk (4 - 5). Command sends "
          "you to the one with a free target - the other sector when you come back for more.", False)],
        [("Hitman picks the target, at random.", True),
         ("Not one another flight is working, not the one you just did. Hitman reads its "
          "number, name and IP, and the IP's bearing if you are not there yet - elevation, "
          "attack headings and egress are on that kneeboard card.", False)],
        [("Another target:", True),
         ("after \"off\", say \"request tasking\" to Hitman - or ask Command again.", False)],
    ])
    NK.subtle(c, "JTAC LIVE 60.000 (Hornet COMM 2 CH 15) is for a human JTAC. Hitman never answers there.")
    return save(c, "12_jtac_flow.jpg")


def c13_jtac_checks():
    c = card("CLEARED ", "HOT?", ["What Hitman checks  ·  from Tacview, against your 9-line card",
                                  "Bearings and headings magnetic."], 14)
    NK.band(c, "HITMAN CHECKS", "or tells you why not")
    c.kv([
        ("IP inbound", "within 20 nm of the IP"),
        ("  not there", "negative - IP bears NNN for NN"),
        ("  off the run-in line", "continue, run in heading NNN"),
        ("  nose off the target", "continue, check heading"),
        ("In call", "inside the final attack heading"),
        ("  and", "0.5 - 15 nm from the target"),
        ("  outside either", "abort, abort, abort - and why"),
    ])
    NK.band(c, "AFTER CLEARED HOT", "Hitman keeps watching")
    c.bullets([
        [("Swing more than 10 deg outside the final attack heading", True),
         ("while still pointed at the target, before a weapon comes off, and Hitman calls "
          "the abort. Go round from the IP.", False)],
        [("Pulling off, a release, or 1.5 nm to go", True), ("ends the watch.", False)],
        [("No radar picture of you", True),
         ("(Tacview down, callsign not matched): no checks - still cleared hot.", False)],
    ])
    NK.band(c, "BDA", "about 15 s after your bombs land")
    c.kv([("good effects  /  good hits  /  shack", "egress, ready for more tasking"),
          ("partial effects", "kills so far, re-attack approved"),
          ("no effects", "correct <dir> <metres>, re-attack"),
          ("unable to assess  (the bridges)", "say your observed effects")])
    c.bullets([
        [("Asked for your effects?", True),
         ("\"bridge dropped\" or \"good hits\" closes it; \"no effects\" gets a re-attack. "
          "\"request BDA\" repeats the call.", False)],
    ])
    NK.band(c, "ALSO", "any time")
    c.kv([("abort", "copy abort, re-attack from the IP"),
          ("radio check", "loud and clear")])
    return save(c, "13_jtac_checks.jpg")


# ---------------------------------------------- range areas (cmd/atc/command_areas.go)
def c14_range_areas():
    c = card("RANGE ", "AREAS", ["Command  ·  %s AM  ·  Black Sea training" % COMMAND_F,
                                 "Letters match the kneeboard TARGET RANGES card."], 15)
    NK.band(c, "WHAT COMMAND TELLS YOU", "threat")
    c.kv([
        ("A   Maykop airfield  ·  SA-6, parked aircraft", "SA-6"),
        ("B   Krasnodar Pashkovsky  ·  SA-6, aircraft", "SA-6"),
        ("C   22 nm NE Maykop  ·  SA-6, armour, infantry", "SA-6"),
        ("D   9-line targets 1 + 2", "Hitman 1-1"),
        ("E   191 nm NW Anapa  ·  SA-10 site", "SA-10"),
        ("F   14 nm SW Krasnodar  ·  armour, infantry", "none"),
        ("G   9 nm N Krymsk  ·  SA-2 site", "SA-2"),
        ("H / I   9-line targets 4 / 5", "Hitman 2-1"),
        ("J   9 nm SW Krymsk  ·  armour, trucks", "none"),
        ("K   104 nm N Anapa  ·  Kuznetsov, 2 corvettes", "SA-N-9, SA-15"),
        ("L   36 nm NE Krasnodar  ·  train", "none"),
        ("M   187 nm W Anapa  ·  cruiser Moskva group", "SA-N-6, SA-N-9"),
        ("N / O   Anapa trucks  /  Kutaisi boats", "none"),
    ], size=14, rh=27)
    c.bullets([
        [("The kneeboard card under-reports three:", True),
         ("E and G are listed \"soft x5\" - they are an SA-10 and an SA-2 site. M is blank - "
          "it is the Moskva's group.", False)],
    ])
    return save(c, "14_range_areas.jpg")


def c15_overlord():
    c = card("OVERLORD ", "ACTUAL", ["Ask for an area, close out the run  ·  Command %s AM" % COMMAND_F,
                                     "Overlord is a second voice on Command's frequency."], 16)
    NK.band(c, "THE EXCHANGE", "\"Command, Raider 1, ...\"")
    NK.steps(c, [
        ("ASK", "permission to proceed to Alpha  ->  stand by for Overlord"),
        ("~10 S", "Overlord: you are Irene for Alpha, report copy"),
        ("COPY", "copy Irene  ->  Command: what is there, threat, bearing"),
        ("OFF", "off target  ->  copy off Alpha, say intentions"),
        ("HOME", "Winchester, RTB  ->  handoff at 30 nm"),
    ], split=150)
    NK.band(c, "ALSO", "Command replies")
    c.kv([
        ("request Alpha  /  request to work Alpha", "goes to Overlord"),
        ("proceeding / pushing / inbound Alpha", "brief at once, no Overlord"),
        ("negative Irene", "another flight has it - ask again"),
        ("Bingo, RTB", "nearest tanker, handoff at 30 nm"),
        ("Joker", "copy Joker"),
    ])
    c.bullets([
        [("\"Irene\" and \"cleared to engage\" both mean go.", True),
         ("Overlord uses either. A flight already on your area is called out - call off target "
          "when you are done so it frees up.", False)],
    ])
    return save(c, "15_overlord.jpg")


# ---------------------------------------------------------- Texaco (cmd/atc/tanker.go)
def c16_texaco():
    c = card("TEXA", "CO", ["Texaco 1-1, 2-1 and 3-1  ·  Black Sea Day / Night",
                            "the same calls on all three  ·  pick the nearest"],
             17, "vSFG-7  ·  SKYEYE ATC  ·  HORNET  ·  TANKER")
    NK.band(c, "THE FLOW", "two baskets each  ·  \"Texaco 1 (2 or 3), Raider 1-1\"")
    NK.steps(c, [
        ("COMMAND", "request tanker  ->  bearing, range, contact the nearest Texaco"),
        ("INITIAL", "15 mile initial for port observation, intent to refuel"),
        ("", "->  cleared port observation ... How much do you request?"),
        ("GIVE", "request 5.0  (or full)  ->  cleared for five thousand pounds"),
        ("TRAIL", "1 mile trail for port observation  ->  continue"),
        ("OBSERVATION", "established port observation  ->  baskets open"),
        ("BASKET", "moving to starboard basket  ->  cleared starboard basket"),
        ("", "established starboard basket  ->  call ready pre-contact"),
        ("", "ready pre-contact  ->  cleared contact starboard basket"),
        ("DCS MENU", "Ready pre-contact  ->  cleared contact, plug, take fuel"),
        ("REFORM", "satisfied, moving to starboard reform  ->  cleared"),
        ("CHECK OUT", "checking out  /  moving on to mission  ->  cleared to depart"),
    ], split=170)
    NK.band(c, "RULES")
    c.bullets([
        [("Observation is always port, reform always starboard.", True),
         ("Lead makes the initial, trail, observation and check-out calls; each jet its own "
          "basket and reform calls.", False)],
        [("One jet per basket.", True),
         ("A held basket gets \"standby, basket occupied\" - it frees when that jet calls reform, "
          "or after 10 minutes.", False)],
        [("Fuel won't flow?", True),
         ("SkyEye is radio only - DCS still needs the AI menu Ready pre-contact.", False)],
    ])
    NK.band(c, "THE TANKERS", "Hornet presets")
    c.kv([("TEXACO 1-1  -  KC-130  ·  TACAN 51Y", "%s AM  ·  %s" % (TANKER_F, H_TANKER)),
          ("     angels 13, about 260 KIAS", ""),
          ("TEXACO 2-1  -  KC-135 MPRS  ·  TACAN 52Y", "%s AM  ·  %s" % (TANKER21_F, H_TANKER21)),
          ("     angels 24, about 265 KIAS", ""),
          ("TEXACO 3-1  -  KC-135 MPRS  ·  TACAN 53Y", "%s AM  ·  %s" % (TANKER2_F, H_TANKER2)),
          ("     angels 24, about 265 KIAS", "")])
    NK.subtle(c, "Radio check any time - loud and clear.  Angels and velocity come live off Tacview.")
    return save(c, "16_texaco.jpg")


# ------------------------------------------------- Shell, the Viper's boom (cmd/atc/tanker.go)
def c16_shell():
    c = card("SHE", "LL", ["Shell 1-1 and Shell 2-1  ·  Black Sea Day / Night",
                           "KC-135 boom  ·  the same calls on both  ·  pick the nearer one"],
             18, "vSFG-7  ·  SKYEYE ATC  ·  VIPER  ·  TANKER")
    NK.band(c, "THE FLOW", "one boom, one jet  ·  \"Shell 1 (or 2), Venom 1-1\"")
    NK.steps(c, [
        ("COMMAND", "request tanker  ->  bearing, range, contact the nearest Shell"),
        ("INITIAL", "15 mile initial for port observation, intent to refuel"),
        ("", "->  cleared port observation ... How much do you request?"),
        ("GIVE", "request 5.0  (or full)  ->  cleared for five thousand pounds"),
        ("TRAIL", "1 mile trail for port observation  ->  continue"),
        ("OBSERVATION", "established port observation  ->  boom open"),
        ("BOOM", "moving to the boom  ->  cleared to the boom"),
        ("", "established pre-contact  ->  call ready pre-contact"),
        ("", "ready pre-contact  ->  cleared contact"),
        ("DCS MENU", "Ready pre-contact  ->  cleared contact, boom on, take fuel"),
        ("REFORM", "satisfied, moving to starboard reform  ->  cleared"),
        ("CHECK OUT", "checking out  /  moving on to mission  ->  cleared to depart"),
    ], split=170)
    NK.band(c, "RULES")
    c.bullets([
        [("Observation is always port, reform always starboard.", True),
         ("Lead makes the initial, trail, observation and check-out calls; each jet its own "
          "boom and reform calls.", False)],
        [("One jet on the boom.", True),
         ("The next gets \"standby, boom occupied\" and holds port observation - the boom frees "
          "when that jet calls reform, or after 10 minutes.", False)],
        [("Fuel won't flow?", True),
         ("SkyEye is radio only - DCS still needs the AI menu Ready pre-contact.", False)],
    ])
    NK.band(c, "THE TANKERS", "Viper presets")
    c.kv([("SHELL 1-1  -  KC-135  ·  TACAN 61Y", "%s AM  ·  %s" % (SHELL_F, V_SHELL)),
          ("SHELL 2-1  -  KC-135  ·  TACAN 62Y", "%s AM  ·  %s" % (SHELL2_F, V_SHELL2)),
          ("     both angels 22, Mach 0.68 (about 288 KIAS)", "")])
    NK.subtle(c, "Radio check any time - loud and clear.  Angels and velocity come live off Tacview.")
    return save(c, "16_shell.jpg")


# ------------------------------------------------- GCI (cmd/atc/gci.go, gci_recovery.go)
EYE_GCI = "vSFG-7  ·  SKYEYE ATC  ·  HORNET + VIPER  ·  GCI"


def c17_gci():
    c = card("DARK", "STAR", ["GCI  ·  %s %s AM Black Sea  ·  %s %s AM Syria" % (GCI_BS + GCI_SY),
                              "SkyEye's AWACS controller, our voice. Picture, threats, bogey dope."],
             19, EYE_GCI)
    NK.band(c, "THE FREQUENCIES", "Hornet  ·  Viper preset")
    c.kv([("%s  -  Black Sea Day / Night, Kola" % GCI_BS[0].upper(),
           "%s AM  ·  %s  ·  %s" % (GCI_BS[1], H_GCI.replace("COMM 1 ", ""), V_GCI.replace("COMM 1 ", ""))),
          ("%s  -  Syria (Foothold)" % GCI_SY[0].upper(), "%s AM  ·  %s  ·  %s" % (GCI_SY[1], H_GCI_SY, V_GCI_SY))])
    NK.subtle(c, "Foothold: CH 5 Hornet / CH 4 Viper is 264, the DCS AI E-3 - not SkyEye.")
    NK.band(c, "THE FLOW", "\"%s, Raider 302, ...\"" % GCI_BS[0])
    NK.steps(c, [
        ("COMMAND", "checking in on 282  ·  Command still runs your recovery"),
        ("CHECK IN", "checking in  ->  radar contact"),
        ("FENCE IN", "fence in  ->  copy fence in, picture follows"),
        ("FLY", "picture  ·  bogey dope  ·  declare  ·  splash  -  card 20"),
        ("FENCE OUT", "fence out  ->  copy fence out"),
        ("CHECK OUT", "checking out  ->  copy, good day.  Then Command: RTB"),
    ], split=170)
    NK.band(c, "TO BE HEARD", "and to get threat calls")
    c.bullets([
        [("Your name must carry your callsign.", True),
         ("DCS name \"Raider 302 | Name\" or \"Venom 212 | Name\" - the GCI finds Raider / Venom and the "
          "number anywhere in it. SRS name the same as DCS, or threat and merged calls never reach you.", False)],
        [("Lead with \"%s\", \"%s\" or \"Anyface\", then your callsign." % (GCI_BS[0], GCI_SY[0]), True),
         ("Bearings as three digits (\"zero six five\"), ranges and altitudes as numbers. No \"niner\".", False)],
        [("Command still copies fence in", True),
         ("and adds \"%s is up on two six four point zero for picture\" - the fence check goes to the GCI, "
          "the picture follows." % GCI_BS[0], False)],
    ])
    NK.subtle(c, "Overlord Actual on Command (cards 15 - 16) is the range officer, not the GCI.")
    return save(c, "17_gci.jpg")


def c17_gci_calls():
    c = card("GCI ", "CALLS", ["Every call the GCI answers  ·  as Raider 302 hears them",
                              "Bullseye and BRAA numbers are examples."], 20, EYE_GCI)
    NK.band(c, "YOU SAY", "%s replies" % GCI_BS[0])
    c.kv([
        ("radio check  /  comm check", "good copy  ·  5 by 5"),
        ("checking in", "radar contact"),
        ("fence in  /  fence out", "copy  ·  picture follows fence in"),
        ("alpha check", "contact, alpha check bullseye 010/48"),
        ("picture", "up to 3 groups, from bullseye"),
        ("bogey dope", "group BRAA 045/32, 25000, hot, hostile"),
        ("any fast movers  /  any slow movers", "bogey dope: jets  /  helicopters"),
        ("declare bullseye 010 for 48 at 25000", "hostile, friendly, furball, clean"),
        ("snaplock 125 for 10 at 8000", "threat BRAA 125/10, 8000, hot"),
        ("spiked 080  /  strobe 080", "spike range 35 ...  ·  clean 080"),
        ("vector to tanker", "bearing and range"),
        ("splash one  /  splash", "copy splash one"),
        ("checking out", "copy, good day"),
    ], size=14, rh=27)
    NK.band(c, "UNASKED", "only if you are on the GCI frequency")
    c.kv([
        ("every 5 minutes", "PICTURE"),
        ("a hostile inside 25 nm  ·  repeats every 3 min", "THREAT"),
        ("inside 3 nm of a hostile (fixed wing)", "MERGED"),
        ("a hostile off scope 30 s  /  the GCI comes up", "FADED  /  SUNRISE"),
        ("no AWACS airborne  /  back up", "OFF / ON STATION"),
    ], size=14, rh=27)
    c.bullets([
        [("It sees what the mission's AWACS sees:", True),
         ("E-3 about 217 nm, E-2 about 200 nm, nothing below its radar horizon or behind a ridge - "
          "low in a valley you are hidden. Off station: picture and bogey dope get \"off station, "
          "no picture\".", False)],
    ])
    return save(c, "17_gci_calls.jpg")


def c17_combat():
    # cmd/atc/combat_calls.go (gciCombatCall, downReply), command_position.go
    # (commandShotResponse) and emergency_divert.go (damagedRe, divertReply, divertFields),
    # origin/main 4721177. Operator wording for the down call (2026-10-21).
    c = card("COMBAT ", "CALLS", ["Shots, kills, ejections, battle damage  ·  on the AWACS or Command",
                                  "No station name needed - just your number first."], 21, EYE_GCI)
    NK.band(c, "SHOTS AND KILLS", "the AWACS copies  ·  Command copies too")
    c.kv([
        ("302, Fox 3, 080 for 56, 25,000", "Raider 302, Darkstar, copy Fox three"),
        ("302, splash one", "Raider 302, Darkstar, copy splash one"),
        ("splash one  (no callsign)", "Darkstar, copy splash one"),
    ], size=14, rh=27)
    NK.band(c, "DOWN", "you, your wingman, or anyone who sees it")
    c.kv([
        ("302 ejecting  /  eject, eject  /  bailing out", "Raider 302 down, launching CSAR."),
        ("Raider 301, 302 is down", "Raider 302 down, launching CSAR."),
    ], size=14, rh=27)
    NK.subtle(c, "\"302 down to angels 5\" is not a down call. CSAR is the radio call only - nothing spawns.")
    NK.band(c, "HIT OR DAMAGED", "the nearest friendly field")
    c.kv([
        ("Command, Raider 302, I'm hit", "nearest field, bearing, miles"),
        ("mayday  /  pan pan  /  declaring an emergency", "elevation, tower frequency"),
        ("engine fire  /  lost an engine  /  hydraulic failure", "same answer"),
        ("fuel leak  /  battle damage  /  request nearest field", "same answer"),
    ], size=14, rh=27)
    c.bullets([
        [("Example:", True),
         ("\"Raider 302, Command, copy. Nearest field Kobuleti, bearing zero-eight-three, 18 miles, "
          "elevation 59. Kobuleti tower two six two point zero, declare your emergency.\"", False)],
        [("Bearing is magnetic, from where Tacview has you.", True),
         ("Not on the scope: \"No radar contact, say position, and the nearest field.\"", False)],
        [("Foothold never sends you to a red-held field.", True),
         ("Read from the campaign save. None friendly: \"No friendly field available, say intentions.\"", False)],
        [("Then call that tower with your emergency.", True),
         ("It gives you the runway you name and copies souls and fuel - card 28.", False)],
        [("Any AWACS name works.", True),
         ("Darkstar, Overlord, Magic or Wizard - the SkyEye AWACS on your frequency answers.", False)],
    ])
    return save(c, "17_combat.jpg")


def c12_sortie():
    c = card("A FULL ", "SORTIE", ["Off the boat and back  ·  every SkyEye call, in order",
                                  "Hornet, Case I, Persian Gulf frequencies."], 22, EYE_CVN)
    NK.band(c, "OUTBOUND")
    NK.steps(c, [
        ("DECKBOSS 128.6", "request taxi  ·  ready cat N  ·  airborne"),
        ("MARSHAL 306.3", "clear seven miles"),
        ("COMMAND 282.0", "checking in  ·  fence in  ·  on station"),
        ("GCI 264 / 266", "Black Sea / Syria: check in  ·  fence in  ·  picture"),
    ], split=280)
    NK.band(c, "ON THE JOB")
    NK.steps(c, [
        ("COMMAND 282.0", "state  ·  request tanker  ·  say bullseye"),
        ("COMMAND 282.0", "off station  ·  fence out  ·  RTB"),
        ("COMMAND 282.0", "Inside 30 nm: contact Marshal, call marking mom."),
    ], split=280)
    NK.band(c, "RECOVERY")
    NK.steps(c, [
        ("MARSHAL 306.3", "marking moms  ·  DME reports  ·  see you at ten"),
        ("MARSHAL 306.3", "state  ·  established angels N  ·  Charlie"),
        ("MARSHAL 306.3", "commencing  ·  Case I initial / Case III platform"),
        ("PADDLES 128.3", "Black Sea + Syria: ball call, fly his calls (card 10)."),
    ], split=280)
    c.bullets([
        [("Syria and Black Sea:", True),
         ("the same frequencies - Marshal 306.300 on COMM 1 CH 3.", False)],
        [("Vipers:", True), ("only the Command calls apply here. Towers are %s." % TOWER_CARDS, False)],
    ])
    return save(c, "12_sortie.jpg")


# ---------------------------------------------------------------- airfield towers
# Tower name is airfield.Name + " Tower" (cmd/atc/main.go). Every tower is UHF AM.
def c13_towers():
    c = card("AIRFIELD ", "ATC", ["One AI tower per field  ·  UHF AM  ·  Gulf, Black Sea, Syria",
                                  "Same calls at every field; only name and frequency change."],
             23, EYE_TWR)
    NK.band(c, "HOW TO BE HEARD", "every tower call")
    c.bullets([
        [("Open with the field name, then your callsign.", True),
         ("\"Minhad Tower, Raider 1, request taxi\". \"Tower\" can be dropped when the field "
          "name comes first: \"Akrotiri, Raider 1, ...\".", False)],
        [("No field name, no answer.", True),
         ("\"Raider 1, request taxi\" is ignored. A bare \"Tower, Raider 1, ...\" does work.", False)],
        [("Misheard names are forgiven.", True),
         ("A letter or two off still counts. Mafraq also answers to King Hussein, and H4 to "
          "H-4.", False)],
        [("CTAF style works too:", True),
         ("\"Senaki traffic, Raider 1, rolling runway 09\".", False)],
    ])
    NK.band(c, "WHAT THE TOWER KNOWS")
    c.bullets([
        [("Runway, wind and altimeter come from the mission weather.", True),
         ("Listen to ATIS first - it gives the same runway.", False)],
        [("It sees you only through Tacview.", True),
         ("Radar check, traffic holds, go-around and speed calls need the rig's Tacview feed.", False)],
        [("IMC changes the pattern.", True),
         ("Ceiling under 1,500 ft or visibility under 3 nm: no overhead, straight-in only.", False)],
    ])
    NK.band(c, "THE TOWER CALLS YOU", "no request needed")
    c.kv([
        ("Over 350 kt inside 10 nm", "speed check"),
        ("Inside 5 nm while a jet is rolling", "go around"),
        ("Wind shift changes the runway", "runway in use is now NN"),
        ("Held short, and the traffic has cleared", "cleared for takeoff"),
    ])
    NK.subtle(c, "Built but switched off: \"say intentions\" and \"report clear of the runway\".")
    return save(c, "13_towers.jpg")


def c14_tower_radios():
    c = card("TOWER ", "RADIOS", ["Gulf and Black Sea  ·  tower, ATIS, COMM 1 preset",
                                  "Presets read off the squadron Hornet and Viper cards."],
             24, EYE_TWR)
    NK.band(c, "PERSIAN GULF", "Hornet  /  Viper")
    c.kv([
        ("AL DHAFRA TOWER  251.100  ·  ATIS 248.200", "CH 8  /  CH 12"),
        ("AL MINHAD TOWER  250.100  ·  ATIS 248.300", "CH 10  /  CH 14"),
        ("AL AIN TOWER  250.700  ·  ATIS 248.850", "CH 16  /  CH 19"),
    ])
    NK.band(c, "BLACK SEA", "Hornet  /  Viper")
    c.kv([
        ("BATUMI TOWER  260.000  ·  ATIS 230.000", "CH 8  /  CH 12"),
        ("KOBULETI TOWER  262.000  ·  ATIS 232.000", "CH 10  /  CH 14"),
        ("KUTAISI TOWER  263.000  ·  ATIS 233.000", "CH 12  /  CH 18"),
        ("SENAKI TOWER  261.000  ·  ATIS 234.000", "CH 14  /  CH 16"),
        ("TBILISI TOWER  267.000  ·  ATIS 237.000", "CH 18  /  CH 10"),
    ])
    NK.band(c, "THE 7-MILE HANDOFF NAMES A CHANNEL", "check it")
    c.bullets([
        [("Towers send you to Command on 282.000 and add a channel.", True),
         ("Gulf towers say \"channel four\"; Black Sea towers say \"channel one\".", False)],
        [("That channel is only right for one jet.", True),
         ("Gulf: Viper Command is CH 1, not 4. Black Sea: Hornet Command is CH 4 - Hornet CH 1 "
          "is Deckboss.", False)],
        [("Trust the frequency.", True), ("282.000 is right in every theatre.", False)],
        [("ATIS is one channel above its tower on every card,", True),
         ("except Viper Gulf CH 20: printed 119.850, but SkyEye's Al Ain ATIS is 248.850.", False)],
    ])
    return save(c, "14_tower_radios.jpg")


def c15_syria_towers():
    # Towers and ATIS from docs/phraseology/facts.json (cmd/atc/docs_test.go reads them out
    # of start_towers_syria.bat / start_atis_syria.bat): the five Foothold fields since
    # 2026-09-18. Channel numbers differ between the Foothold cards, so none are printed.
    th = next(t for t in FACTS if t["key"] == "syria")
    atis = {a["icao"]: a["freq"] for a in th["atis"]}
    c = card("SYRIA ", "TOWERS", ["Eastern Med  ·  Foothold  ·  %d fields  ·  tower and ATIS" % len(th["towers"]),
                                  "Read from the start scripts - dial the frequency."],
             25, EYE_TWR)
    NK.band(c, "THE TOWERS", "tower  ·  ATIS")
    c.kv([(t["name"].upper(), "%.3f   ATIS %.3f" % (t["freq"], atis[t["icao"]]) if t["icao"] in atis
           else "%.3f" % t["freq"]) for t in th["towers"]])
    NK.band(c, "KNOW BEFORE YOU FLY", "Syria")
    c.bullets([
        [("Check the frequency, not only the channel.", True),
         ("The Foothold preset cards number the channels per airframe.", False)],
        [("ATIS speaks English, then a local language", True),
         ("(Akrotiri English only).", False)],
        [("The DCS F-menu tower is somewhere else.", True),
         ("Except at Incirlik, DCS's own controller is not on SkyEye's frequency.", False)],
        [("The 7-mile handoff says \"channel four\"", True), ("- Hornet Command, 282.000.", False)],
    ])
    return save(c, "15_syria_towers.jpg")


def c16_tower_departure():
    c = card("DEPART", "URE", ["Parking to Command  ·  every SkyEye field",
                               "Wait for each reply before the next call."], 26, EYE_TWR)
    NK.band(c, "THE DEPARTURE", "in order")
    NK.steps(c, [
        ("REQUEST TAXI", "Minhad Tower, Raider 1, request taxi."),
        ("", "Reply: taxi to runway NN, altimeter, hold short."),
        ("HOLDING SHORT", "Minhad Tower, Raider 1, holding short."),
        ("", "Reply: line up and wait."),
        ("+ 5 s", "Reply: wind, runway, cleared for takeoff."),
        ("AIRBORNE", "Minhad Tower, Raider 1, airborne."),
        ("", "Reply: angels N, expect handoff at seven DME."),
        ("7 MILES", "The tower calls you - you say nothing."),
        ("", "Tower: leaving my airspace, contact command."),
        ("COMMAND", "Command, Raider 1, checking in."),
    ], split=270)
    NK.band(c, "IF YOU ARE HELD")
    c.bullets([
        [("Traffic inside 20 nm holds you.", True),
         ("Inside 15 nm: \"hold short, traffic on N mile final\". 15 - 20 nm: line up and wait, "
          "no takeoff clearance.", False)],
        [("60 s between departures.", True),
         ("Too soon after the last jet: \"hold short, departure spacing N seconds\".", False)],
        [("Stay put - the tower calls you.", True),
         ("When the traffic or the spacing clears you get your takeoff clearance.", False)],
    ])
    NK.band(c, "GOOD TO KNOW")
    c.bullets([
        [("\"ready for takeoff\" skips line up and wait:", True),
         ("with no traffic you are cleared at once.", False)],
        [("\"rolling runway NN\" (CTAF), \"departing\" or \"clear the active\"", True),
         ("after takeoff gets the same reply as airborne.", False)],
        [("No 7 DME call needed.", True),
         ("The tower calls you at 7 miles (Tacview). Call it anyway and the reply is \"copy\".", False)],
        [("Optional:", True),
         ("\"pushing command\" gets a short good day; \"request startup\" gets Ground's "
          "startup approval.", False)],
    ])
    return save(c, "16_tower_departure.jpg")


def c17_tower_arrival():
    c = card("ARRI", "VAL", ["Overhead or straight in  ·  one call per leg",
                             "Give your distance with inbound, initial or final."], 27, EYE_TWR)
    NK.band(c, "THE OVERHEAD", "VMC")
    NK.steps(c, [
        ("INBOUND", "Minhad Tower, Raider 1, 10 mile initial."),
        ("", "Reply: runway, wind, altimeter, report initial."),
        ("INITIAL", "Minhad Tower, Raider 1, 3 mile initial."),
        ("", "Reply: left break runway NN, number N, report base."),
        ("DOWNWIND", "Optional. Reply: number N, report base."),
        ("BASE", "Minhad Tower, Raider 1, base, gear down."),
        ("", "Number one: cleared to land. Otherwise: affirmative."),
        ("FINAL", "Minhad Tower, Raider 1, on final."),
        ("", "Reply: runway, wind, cleared to land."),
        ("VACATED", "Minhad Tower, Raider 1, clear of the active."),
    ], split=250)
    c.bullets([
        [("Past 15 nm you hear \"continue inbound, report N miles\".", True),
         ("Answer \"15 miles inbound\" or \"15 mile initial\".", False)],
        [("Say \"gear down\" in your base or final call,", True),
         ("or the tower adds \"check wheels down\".", False)],
        [("\"initial\" or \"break\" on its own is taken as your break", True),
         ("(reply: report downwind). Give the distance: \"3 mile initial\".", False)],
        [("\"touch and go\", \"low approach\" or \"the option\"", True),
         ("in the final call gets that clearance instead.", False)],
    ])
    NK.band(c, "DISTANCES")
    c.bullets([
        [("\"15 miles inbound\" and \"15 mile initial\" both work.", True),
         ("So does a bare \"15 miles\". \"7 miles\" on its own is the 7 DME departure call.", False)],
        [("A DME needs \"inbound\" or \"initial\".", True),
         ("\"12 DME inbound\" works; \"12 DME\" on its own is a departure call.", False)],
        [("\"5 mile final\" is a landing call,", True),
         ("the same as \"on final\".", False)],
    ])
    NK.band(c, "STRAIGHT IN, IMC, HUNG ORDNANCE")
    c.bullets([
        [("\"request straight in\", \"ILS\" or \"visual approach\":", True),
         ("straight-in runway NN approved, altimeter, report final.", False)],
        [("IMC: the overhead is refused.", True),
         ("Inbound, initial and break get \"straight-in only, report ten mile final\". Then call "
          "\"ten mile final\".", False)],
        [("\"hung ordnance\":", True),
         ("straight in, no overflight, stop at the end of the runway for dearm.", False)],
    ])
    return save(c, "17_tower_arrival.jpg")


def c18_tower_calls():
    c = card("TOWER ", "CALLS", ["Every call a tower answers  ·  lead with the field name",
                                 "Silent calls are heard but get no reply."], 28, EYE_TWR)
    NK.band(c, "GROUND AND DEPARTURE", "tower replies")
    c.kv([
        ("request taxi  /  ready to taxi", "taxi, hold short"),
        ("holding short  /  at the hold  /  holding", "line up and wait"),
        ("ready for takeoff  /  request takeoff", "cleared for takeoff"),
        ("airborne  /  departing  /  rolling", "angels, expect handoff"),
        ("(at 7 miles the tower calls you)", "contact Command"),
        ("seven DME  /  seven miles  /  request handoff", "copy, or contact Command"),
        ("pushing command", "good day"),
    ])
    NK.band(c, "ARRIVAL AND LANDING")
    c.kv([
        ("N mile initial  /  N miles inbound", "field information"),
        ("3 mile initial  /  overhead", "break approved"),
        ("downwind", "number N, report base"),
        ("base  /  on final  /  N mile final", "cleared to land"),
        ("request straight in  /  ILS", "straight-in approved"),
        ("touch and go  /  low approach  /  the option", "cleared touch and go"),
        ("going around", "report downwind"),
        ("runway vacated  /  clear of the active", "taxi to parking"),
    ])
    NK.band(c, "ANY TIME")
    c.kv([
        ("say again", "your last call, 2 min"),
        ("wind check  /  say altimeter", "wind and altimeter"),
        ("radar check", "angels, range, bearing"),
        ("radio check  /  checking in  /  green jet", "loud and clear"),
        ("hung ordnance", "straight in, dearm"),
        ("mayday  /  pan pan  /  emergency  (+ runway NN)", "runway is yours - the one you name"),
        ("N souls on board, fuel state", "crash crew standing by"),
        ("tally  /  no joy  /  altitude check  /  roger", "silent"),
    ])
    NK.subtle(c, "Anything else gets \"say again your request\".")
    return save(c, "18_tower_calls.jpg")


def c18_multiship():
    # pkg/controller/flight.go (origin/main 0aab154, 2026-10-21): one takeoff and one landing
    # clearance cover the flight (operator ruling).
    c = card("MULTI-", "SHIP", ["Flights of 2 - 6  ·  one clearance for the whole flight",
                                "The lead talks. Name the wingmen on the first call."], 29, EYE_TWR)
    NK.band(c, "DEPARTURE", "the lead calls")
    NK.steps(c, [
        ("HOLDING SHORT", "Senaki Tower, Raider 301, flight of three,"),
        ("", "302 and 303, holding short."),
        ("", "Reply: Raider 301 flight, flight of three, line up, wait."),
        ("TAKEOFF", "Raider 301 flight, cleared for takeoff - all three jets."),
        ("AIRBORNE", "Senaki Tower, Raider 301 flight, airborne."),
        ("7 MILES", "Departure calls the flight once."),
    ], split=270)
    NK.band(c, "ARRIVAL")
    NK.steps(c, [
        ("INBOUND", "Senaki Tower, Raider 301, flight of two, 302,"),
        ("", "10 miles east, inbound full stop."),
        ("", "Reply: Raider 301 flight, flight of two, report initial."),
        ("BASE / FINAL", "Raider 301 flight, cleared to land - both jets."),
        ("VACATED", "Senaki Tower, Raider 301 flight, clear of the active."),
        ("", "Ground: Raider 301 spot 12, Raider 302 spot 13."),
    ], split=270)
    NK.band(c, "GOOD TO KNOW")
    c.bullets([
        [("Say the size:", True),
         ("\"flight of two\", \"two-ship\" or \"three FA-18s\". Wingmen right after the lead: "
          "\"302 and 303\", \"Raider 302, Raider 303\" or \"1-2 and 1-3\".", False)],
        [("A wingman may call too:", True),
         ("named, or flying within 2 nm of the lead, he is answered as the flight.", False)],
        [("\"Raider 301, flight of two, full stop\"", True),
         ("on its own is the landing call: cleared to land for both.", False)],
        [("Spacing and parking:", True),
         ("the next departure waits 60 s plus 20 s per extra jet. Every jet gets a spot; "
          "unnamed wingmen get \"spots 12, 13 and 14\".", False)],
    ])
    return save(c, "18_multiship.jpg")


def c19_trouble():
    c = card("NO ", "ANSWER?", ["Work down the list  ·  most misses are the first three",
                                "Tell the operator what you said, word for word."], 30)
    NK.band(c, "CHECK", "in order")
    NK.steps(c, [
        ("1", "Waited 2 - 3 seconds?"),
        ("2", "Led with the role or field name - Marshal, Minhad Tower?"),
        ("3", "Said your full callsign?"),
        ("4", "Used a trigger word from the calls cards?"),
        ("5", "Half-second pause after keying - call not clipped?"),
        ("6", "Frequency? Marshal 306.3, Paddles 128.3, Hitman 61FM."),
        ("", "Texaco 252.1/2/3, Shell 253.1/2.  GCI: Darkstar 264.0, Overlord 266.0."),
        ("7", "SRS shows your TX light? Radio AM (Hitman FM), encryption off?"),
    ], split=120)
    NK.band(c, "KNOWN QUIRKS")
    c.bullets([
        [("Your callsign may be misheard.", True),
         ("Raider as reader or radar, Venom as vino. SkyEye corrects most; if not, slow down "
          "and say the digits one at a time.", False)],
        [("The same call can get a different reply.", True),
         ("Each answer has two to four versions picked at random - the meaning is the same.", False)],
        [("Marshal needs the number in your callsign.", True),
         ("\"Marshal, Raider 1, ...\" - a call with no number gets no answer.", False)],
        [("Paddles answers only the ball call and a radio check.", True),
         ("Everything else he says is his own call on your pass - no readback.", False)],
        [("The GCI needs your callsign in your name.", True),
         ("DCS and SRS name \"Raider 302 | Name\" - no Raider / Venom number, no threat calls.", False)],
        [("Deckboss uses the DCS AI carrier frequency.", True),
         ("You may hear the AI carrier controller on 128.600 as well.", False)],
    ])
    NK.subtle(c, "Built from the SkyeyeATC code, origin/main 9de1eb0, 27 OCT 2026.")
    return save(c, "19_trouble.jpg")


CARD_FUNCS = [c01_how, c02_freqs, c03_deckboss_flow, c04_deckboss_calls, c05_marshal_case1,
              c06_marshal_case3, c07_marshal_calls_inbound, c08_marshal_calls_stack,
              c09_marshal_calls_case3, c10_paddles, c10_departure, c11_command, c12_jtac_flow,
              c13_jtac_checks, c14_range_areas, c15_overlord, c16_texaco, c16_shell, c17_gci, c17_gci_calls, c17_combat, c12_sortie, c13_towers,
              c14_tower_radios, c15_syria_towers, c16_tower_departure, c17_tower_arrival,
              c18_tower_calls, c18_multiship, c19_trouble]


def main():
    os.makedirs(CARDS, exist_ok=True)
    assert len(CARD_FUNCS) == TOTAL
    pages = [f() for f in CARD_FUNCS]
    out = os.path.join(PACKET, "vSFG-7_SkyEye_ATC_Comms_%s.pdf" % VER)
    # Older versions live in git history (the packet moved into SkyeyeATC/docs on
    # 2026-10-14); the Desktop packet folder gets a copy of the new one.
    for fn in os.listdir(PACKET):
        if fn.startswith("vSFG-7_SkyEye_ATC_Comms_v") and fn.endswith(".pdf") and \
                fn != os.path.basename(out):
            os.remove(os.path.join(PACKET, fn))
    n = NS.build_pdf([("SKYEYE ATC", pages)], out, "SKYEYE ATC",
                     "Command  ·  Deckboss  ·  Marshal  ·  Paddles  ·  JTAC  ·  Texaco  ·  GCI  ·  Towers\nPersian Gulf  ·  Syria  ·  Black Sea",
                     airframe="HORNET  ·  VIPER", squadron="vSFG-7  ·  AI ATC ON SRS",
                     version=VER, date=DATE, cover_logo=LOGO, cover_logo_mode="circle",
                     page_logo=LOGO, page_logo_mode="circle", cache=CACHE, balance=True,
                     footer="vSFG-7  ·  SKYEYE ATC COMMS  ·  %s" % STAMP, divider_text="")
    print("PDF  %d pages  ->  %s (%.1f MB)" % (n, out, os.path.getsize(out) / 1e6))
    kneeboards(pages)
    if os.path.isdir(DESKTOP_COPY):
        shutil.copy2(out, os.path.join(DESKTOP_COPY, os.path.basename(out)))
        retire_old_versions(DESKTOP_COPY, "vSFG-7_SkyEye_ATC_Comms_v", os.path.basename(out))


# An OMM / OvGME mod (operator 2026-10-22): its folder mirrors Saved Games\DCS, so enabling
# it drops the cards into the Hornet's and the Viper's kneeboard. Rebuilt whole each time.
OMM_MOD = os.path.join(DESKTOP_COPY, "OMM", "SkyEye ATC Kneeboards")
KNEEBOARD_JETS = ["FA-18C_hornet", "F-16C_50"]


def kb_freqs(viper=False):
    """Kneeboard page 2 (operator 2026-10-22): the mission comms card already gives the
    presets, so the kneeboard set gets only what each AI ATC role listens on - per theatre
    we fly, straight from facts.json (read out of the start scripts by TestDocFacts).
    The Viper's page has no carrier roles and only Viper channels."""
    def f(x):
        return "%.3f" % x

    def ch(th, key):
        v = KB_CH[th][key]
        if not viper:
            return v
        return next((p[2:] for p in v.split("  ·  ") if p.startswith("V ")), "dial")

    carrier = (("DECKBOSS", "deckboss"), ("MARSHAL", "marshal"), ("PADDLES", "lso"))
    if viper:
        c = card("AI ATC ", "FREQS", ["What SkyEye listens on, and the Viper preset",
                                      "1-4 = COMM 1 channel 4  ·  AM unless FM"], 2,
                 "vSFG-7  ·  SKYEYE ATC  ·  VIPER")
    else:
        c = card("AI ATC ", "FREQS", ["What SkyEye listens on, and the preset that reaches it",
                                      "H = Hornet, V = Viper  ·  1-4 = COMM 1 channel 4  ·  AM unless FM"], 2)
    jets = "Viper" if viper else "Hornet  ·  Viper"
    cau = _CAU
    NK.band(c, "BLACK SEA", "Caucasus Day / Night  ·  " + jets)
    rows = [("COMMAND  %s" % f(cau["command"]), ch("caucasus", "command")),
            ("%s AWACS  %s" % (cau["gci_name"].upper(), f(cau["gci"])), ch("caucasus", "gci")),
            ("HITMAN 1-1  %s FM" % f(cau["jtac"]), ch("caucasus", "jtac")),
            ("HITMAN 2-1  %s FM" % f(cau["jtac2"]), ch("caucasus", "jtac2")),
            ]
    nets = {n["name"]: n for n in cau.get("tankers", [])}
    if viper:
        rows += [("%s  %s" % (n.upper(), f(nets[n]["freq"])), ch("caucasus", n)) for n in ("Shell 1-1", "Shell 2-1")]
    else:
        rows += [("TEXACO 1-1  %s" % f(cau["tanker"]), ch("caucasus", "tanker")),
                 ("TEXACO 2-1  %s" % f(nets["Texaco 2-1"]["freq"]), ch("caucasus", "Texaco 2-1")),
                 ("TEXACO 3-1  %s" % f(cau["tanker2"]), ch("caucasus", "tanker2"))]
    if not viper:
        rows += [("%s  %s" % (n, f(cau[k])), ch("caucasus", k)) for n, k in carrier]
    rows += kb_towers(cau, ch)
    c.kv(rows, size=13, rh=25)
    syr = _SYR
    NK.band(c, "SYRIA", "Foothold  ·  " + jets)
    rows = [("COMMAND  %s" % f(syr["command"]), ch("syria", "command")),
            ("%s AWACS  %s" % (syr["gci_name"].upper(), f(syr["gci"])), ch("syria", "gci"))]
    if not viper:
        rows += [("%s  %s" % (n, f(syr[k])), ch("syria", k)) for n, k in carrier]
    rows += kb_towers(syr, ch)
    c.kv(rows, size=13, rh=25)
    if viper:
        NK.subtle(c, "Tower row: tower channel; ATIS is the next one up (Foothold: dial it). \"dial\" = no preset.")
        return save(c, "02_ai_atc_freqs_viper.jpg")
    NK.subtle(c, "Tower row: tower channel; ATIS is the next one up (Foothold Viper: dial it). \"dial\" = no preset.")
    return save(c, "02_ai_atc_freqs.jpg")


def kb_freqs_viper():
    return kb_freqs(viper=True)


# Presets that reach each SkyEye role, read off the mission preset kneeboard cards
# (2026-10-22): Caucasus Training V1.35 _BUILD/_cards/01_bs_{hornet,viper}_presets.png and
# Foothold v1.15 _assets/cards/_presets_{FA-18C_hornet,F-16C_50}.html. Those missions are
# built elsewhere - re-read the cards when their comm ladders change. "H 1-4" = Hornet COMM 1
# CH 4; Viper ATIS on Foothold has no preset.
KB_CH = {
    "caucasus": {"command": "H 1-4  ·  V 1-1", "gci": "H 1-5  ·  V 1-4", "jtac": "H 2-12  ·  V 2-8",
                 "jtac2": "dial", "tanker": "H 2-8", "tanker2": "H 2-10", "Texaco 2-1": "H 2-9",
                 "Shell 1-1": "V 1-7", "Shell 2-1": "V 1-8",
                 "deckboss": "H 1-1", "marshal": "H 1-3", "lso": "H 2-4",
                 "UGSB": "H 1-8  ·  V 1-12", "UG5X": "H 1-10  ·  V 1-14", "UGKO": "H 1-12  ·  V 1-18",
                 "UGKS": "H 1-14  ·  V 1-16", "UGTB": "H 1-18  ·  V 1-10",
                 "XRMF": "dial"},  # Mozdok (blue since ff695e1): no preset on the cards
    "syria": {"command": "H 1-4  ·  V 1-1", "gci": "H 1-7  ·  V 1-5", "deckboss": "H 1-1",
              "marshal": "H 1-3", "lso": "dial",
              "LCRA": "H 1-12  ·  V 1-18", "OSLK": "H 1-14  ·  V 1-17", "OLBA": "H 1-16  ·  V dial",
              "LTAG": "H 1-8  ·  V 1-15", "LLRD": "H 1-10  ·  V 1-16"},
}


def kb_towers(th, ch):
    atis = {a["icao"]: a["freq"] for a in th["atis"]}
    rows = []
    for tw in sorted(th["towers"], key=lambda x: x["name"]):
        a = atis.get(tw.get("icao"))
        name = tw["name"].upper().replace(" TOWER", "")
        rows.append(("%s  %.3f%s" % (name, tw["freq"], "  ·  ATIS %.3f" % a if a else ""),
                     ch(th["key"], tw["icao"])))
    return rows


# Each jet's kneeboard deck: the comms cards with card 2 swapped for the plain frequencies
# card. The Viper has no carrier, so no Deckboss, Marshal, Paddles or boat cards (operator
# 2026-10-22: "16's do not need marshal, LSO, Deckboss"). Pages keep their packet numbers so
# "card 21" still means card 21. The Hornet gets Texaco (baskets), the Viper Shell (boom).
CARRIER_CARDS = [c03_deckboss_flow, c04_deckboss_calls, c05_marshal_case1, c06_marshal_case3,
                 c07_marshal_calls_inbound, c08_marshal_calls_stack, c09_marshal_calls_case3,
                 c10_paddles, c10_departure, c12_sortie]
JET_DECKS = {
    "FA-18C_hornet": [kb_freqs if f is c02_freqs else f for f in CARD_FUNCS if f is not c16_shell],
    "F-16C_50": [kb_freqs_viper if f is c02_freqs else f for f in CARD_FUNCS
                 if f not in CARRIER_CARDS and f is not c16_texaco],
}
KNEEBOARD_FUNCS = JET_DECKS["FA-18C_hornet"] + [kb_freqs_viper, c16_shell]


def kneeboards(pages):
    """The squadron kneeboard format: every page EXACTLY 1289 x 2190, each card re-rendered at
    the largest type scale that fills one page (fit_kneeboards.py; Caucasus Training NOTES.md
    "THE KNEEBOARD PAGE FORMAT", operator-approved 2026-10-13). Never a picture shrunk to fit -
    operator 2026-10-22 "make sure cards are properly formated for kneeboards"."""
    if not os.path.isdir(DESKTOP_COPY):
        return
    import fit_kneeboards
    stage = os.path.join(CACHE, "kneeboard_pages")
    if os.path.isdir(stage):
        shutil.rmtree(stage)
    fitted = fit_kneeboards.fit(stage)
    if os.path.isdir(OMM_MOD):
        shutil.rmtree(OMM_MOD)
    counts = []
    for jet in KNEEBOARD_JETS:
        d = os.path.join(OMM_MOD, "Kneeboard", jet)
        os.makedirs(d)
        for i, f in enumerate(JET_DECKS[jet], 1):
            name = fitted[f.__name__][0]
            stem = os.path.splitext(name)[0].split("_", 1)[1].replace("_viper", "")
            shutil.copy2(os.path.join(stage, name), os.path.join(d, "SkyEye_%02d_%s.jpg" % (i, stem)))
        counts.append("%s %d" % (jet, len(JET_DECKS[jet])))
    scales = [s for _, s in fitted.values()]
    print("kneeboard type scale %.2f - %.2f (PDF cards 1.95)" % (min(scales), max(scales)))
    with open(os.path.join(OMM_MOD, "README.txt"), "w", encoding="utf-8", newline="\r\n") as f:
        f.write("vSFG-7 SkyEye ATC kneeboards  %s  %s\n\n" % (VER, DATE))
        f.write("OMM / OvGME: put the \"SkyEye ATC Kneeboards\" folder in your Saved Games mods\n"
                "folder and enable it. It installs into Saved Games\\DCS\\Kneeboard\\: %s pages.\n"
                "The Viper deck has no carrier cards. By hand: copy the Kneeboard folder into\n"
                "Saved Games\\DCS.\n" % ", ".join(counts))
    print("kneeboards  %s  ->  %s" % (", ".join(counts), OMM_MOD))


def retire_old_versions(folder, prefix, keep):
    """Move older versions of this PDF in the Desktop Documentation folder to its Old\
    (what was done by hand at every version; operator 2026-10-21 tidy)."""
    old = os.path.join(folder, "Old")
    for f in os.listdir(folder):
        if f.startswith(prefix) and f.endswith(".pdf") and f != keep:
            os.makedirs(old, exist_ok=True)
            dst = os.path.join(old, f)
            if os.path.exists(dst):
                os.remove(os.path.join(folder, f))
            else:
                shutil.move(os.path.join(folder, f), dst)


if __name__ == "__main__":
    main()
