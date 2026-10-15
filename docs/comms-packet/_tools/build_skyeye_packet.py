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
DESKTOP_COPY = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\SkyEye ATC Comms Packet"
CACHE = os.path.join(PACKET, "_tools", "_cache")
CARDS = os.path.join(CACHE, "cards")      # kneeboard renders, only used to feed the PDF
LOGO = os.path.join(MISSIONS, "New Pilot packs", "VMFA225_patch.png")   # vSFG-7 group patch
NK.set_logo(LOGO, "circle")

VER = "v1.23"
DATE = "15 OCT 2026"   # release date of VER - change it together with VER
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

TOTAL = 21
# Generated frequencies shared with the Phraseology Reference (docs/phraseology).
FACTS = json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..",
                                    "phraseology", "facts.json"), encoding="utf-8"))
TOWER_CARDS = "cards 15 - 20"


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
          ("HITMAN 1-1  -  JTAC, Black Sea training", "%s FM  ·  cards 12 - 13" % JTAC_F),
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
    NK.subtle(c, "ATIS is listen-only. There is no SkyEye LSO.")
    return save(c, "01_how.jpg")


def c02_freqs():
    c = card("FREQUEN", "CIES", ["Where SkyEye actually listens  ·  per theatre",
                                 "Hornet preset shown for each; Vipers use Command only."], 2)
    for th in ("PERSIAN GULF", "SYRIA", "BLACK SEA"):
        NK.band(c, th, "Hornet preset")
        c.kv([("COMMAND  %s" % COMMAND_F, "%s  ·  Viper %s" % (H_CMD, V_CMD)),
              ("DECKBOSS  %s" % DECKBOSS_F, H_DKB),
              ("MARSHAL  %s" % MARSHAL_F[th], H_MSH[th])])
    NK.band(c, "READING THE PRESET CARDS", "read this")
    c.bullets([
        [("\"AI\" on a carrier preset means SkyEye.", True),
         ("COMM 1 CH 3 \"CVN-72 AI MARSHALL\" 306.300 is SkyEye Marshal in every theatre "
          "(operator ruling 2026-09-20).", False)],
        [("COMM 1 CH 2 \"CVN-72 MARSHAL\" 306.200 is for a live human Marshal.", True),
         ("SkyEye never answers there.", False)],
        [("Deckboss 128.600 is shared", True),
         ("with the DCS AI carrier controller - you may hear both.", False)],
        [("LIVE LSO 128.100 (COMM 2 CH 2) is silent.", True),
         ("SkyEye has no LSO - fly the ball yourself.", False)],
    ])
    NK.subtle(c, "Voices: towers British or Australian  ·  Command, Marshal, Deckboss American  ·  Hitman Australian.")
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
    NK.subtle(c, "Bolter practice: Deckboss hands you to the LSO, who is silent. Fly the ball.")
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
        ("SEE YOU AT 10", "Reply: visual at ten, report altitude and range."),
        ("STATE", "Marshal, Raider 1, state 5.2 - under 2.0 is priority."),
        ("ESTABLISHED", "Marshal, Raider 1, established angels 3"),
        ("", "Reply: signal Charlie - or your stack position, hold."),
        ("COMMENCING", "Reply: call three mile initial for paddles."),
        ("INITIAL", "At 3 nm. Reply: contact paddles, good luck."),
    ], split=280)
    NK.band(c, "THE STACK")
    c.bullets([
        [("\"Marshal at angels N\" is your stack altitude.", True),
         ("The lowest free altitude from angels 2 to angels 9. Marshal counts aircraft already "
          "in the stack and any contact within 50 nm at that altitude.", False)],
        [("When someone commences, the stack steps down silently.", True),
         ("There is no radio call for it - keep flying your assigned angels.", False)],
        [("After initial there is nobody to talk to.", True),
         ("Paddles has no SkyEye role - fly the ball and trap.", False)],
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
        ("MARKING MOMS", "Reply: hold on the radial NNN at angels N,"),
        ("", "commence at MM, report established."),
        ("ESTABLISHED", "Reply: commence at MM. There is no Charlie."),
        ("COMMENCING", "At your MM. Reply: descend to platform 5,000,"),
        ("", "final bearing NNN, confirm BRC NNN,"),
        ("", "report platform."),
        ("PLATFORM", "Passing 5,000 ft. Reply: final bearing NNN."),
        ("INITIAL", "Do not call it in Case III - platform is your last call."),
    ], split=280)
    c.bullets([
        [("Holding radial = BRC + 180.  Final bearing = BRC - 9.", True),
         ("All magnetic. MM is minutes past the hour. The first aircraft commences 10 minutes "
          "out; each one after adds a minute.", False)],
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
        ("", "report altitude and range."),
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
        ("", "check in three mile initial, paddles handoff."),
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
                                     "Example: BRC 275, ceiling 800, vis 2, commence at 15."],
             9, EYE_CVN)
    NK.band(c, "MARKING MOMS", "Case III")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, marking moms 340 for 30,"),
        ("", "angels 12, state 6.5."),
        ("MARSHAL", "Raider 1, Union Marshal, radar contact, angels 12,"),
        ("", "range 30 miles from the Abe, bearing three four zero,"),
        ("", "mother's weather ceiling eight hundred overcast,"),
        ("", "visibility 2, Case Three recovery, BRC 275,"),
        ("", "altimeter two niner niner two, hold on the zero niner"),
        ("", "five at angels two, commence at one five,"),
        ("", "report established."),
    ], split=150)
    NK.band(c, "HOLD TO PLATFORM", "in order")
    NK.steps(c, [
        ("YOU", "Marshal, Raider 1, established angels two."),
        ("MARSHAL", "Raider 1, Union Marshal, roger established angels two"),
        ("", "on the zero niner five, commence at one five."),
        ("YOU", "Marshal, Raider 1, commencing."),
        ("MARSHAL", "Raider 1, Union Marshal, copy commencing, descend to"),
        ("", "platform five thousand, final bearing two six six,"),
        ("", "confirm BRC two seven five, report platform."),
        ("YOU", "Marshal, Raider 1, platform."),
        ("MARSHAL", "Raider 1, Union Marshal, copy platform,"),
        ("", "final bearing two six six."),
    ], split=150)
    NK.band(c, "CASE CHANGE", "to everyone in the stack")
    NK.steps(c, [
        ("MARSHAL", "All marshal aircraft, Union Marshal, recovery is now"),
        ("", "Case Three, acknowledge on next push."),
    ], split=150)
    c.bullets([
        [("Radial 095 = BRC 275 + 180.  Final bearing 266 = BRC - 9.", True),
         ("\"Commence at one five\" is 15 minutes past the hour.", False)],
    ])
    NK.subtle(c, MSH_NOTE)
    return save(c, "09_marshal_calls_case3.jpg")


def c10_departure():
    c = card("OFF THE ", "BOAT", ["Launch to Command  ·  Deckboss -> Marshal -> Command",
                                  "Three legs, three frequencies."], 10, EYE_CVN)
    NK.band(c, "THE CHAIN", "in order")
    NK.steps(c, [
        ("1  DECKBOSS", "Raider 1, airborne."),
        ("", "Reply: copy airborne, push Marshal and its frequency."),
        ("2  MARSHAL", "Marshal, Raider 1, clear seven miles."),
        ("", "Reply: clear of Union control zone,"),
        ("", "push vSFG-7-Command,"),
        ("", "two eight two point zero, channel four, for tasking."),
        ("3  COMMAND", "Command, Raider 1, checking in."),
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
                             "The squadron net. Both jets, every theatre."], 11)
    NK.band(c, "YOU SAY", "Command replies")
    c.kv([
        ("radio check  /  comm check  /  how copy", "loud and clear"),
        ("checking in  /  check in", "proceed mission"),
        ("fence in  /  fence check", "cleared hot"),
        ("on station", "good hunting"),
        ("off station  /  departing station", "proceed to assigned pattern"),
        ("fence out", "safe passage"),
        ("RTB  /  request recovery  /  bingo home plate", "expect handoff at 30 nm"),
        ("state 5.2", "copy state"),
        ("say bullseye  /  request bullseye", "bullseye NNN for NN"),
        ("request tanker  /  say texaco", "bearing, range, angels"),
        ("request tasking  (Black Sea training)", "contact Hitman 1-1, 61.0  -  card 12"),
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
    c = card("HITMAN ", "1-1", ["JTAC  ·  %s FM  ·  Hornet %s  ·  Viper %s" % (JTAC_F, H_JTAC, V_JTAC),
                               "Black Sea (Caucasus) training  ·  the five 9-line cards"], 12)
    NK.band(c, "THE RUN", "every call starts \"Hitman, Raider 1, ...\"")
    NK.steps(c, [
        ("COMMAND", "request tasking  ->  contact Hitman 1-1 on 61.0"),
        ("CHECK IN", "checking in  ->  Hitman picks the target, 9-line"),
        ("COPY", "copy  ->  report IP inbound"),
        ("IP INBOUND", "IP inbound  ->  continue"),
        ("IN", "in from the <dir>  ->  cleared hot, or abort"),
        ("OFF", "off  ->  egress per line 9"),
    ], split=190)
    NK.band(c, "THE 9-LINE", "by reference to your kneeboard card")
    c.bullets([
        [("Hitman picks the target, at random.", True),
         ("Not one another flight is working, not the one you just did. Find that card in "
          "your kneeboard - Hitman reads you its number, name, elevation, final attack "
          "heading and egress, and the IP's bearing if you are not there yet.", False)],
        [("Another target:", True),
         ("after \"off\", say \"request tasking\" to Hitman - or ask Command again.", False)],
    ])
    NK.subtle(c, "JTAC LIVE 60.000 (Hornet COMM 2 CH 15) is for a human JTAC. Hitman never answers there.")
    return save(c, "12_jtac_flow.jpg")


def c13_jtac_checks():
    c = card("CLEARED ", "HOT?", ["What Hitman checks  ·  from Tacview, against your 9-line card",
                                  "Bearings and headings magnetic."], 13)
    NK.band(c, "HITMAN CHECKS", "or tells you why not")
    c.kv([
        ("IP inbound", "within 5 nm of the IP"),
        ("  not there", "negative - IP bears NNN for NN"),
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
    c.bullets([
        [("Impacts, then kills.", True),
         ("Where you hit against the aim point, then what is destroyed, priority targets "
          "first. \"request BDA\" repeats it.", False)],
        [("Bridges cannot be assessed", True),
         ("- Hitman gives impacts, Enguri's checkpoint, and any hit on the Rioni no-strike "
          "road bridge.", False)],
    ])
    NK.band(c, "ALSO", "any time")
    c.kv([("abort", "copy abort, re-attack from the IP"),
          ("radio check", "loud and clear")])
    return save(c, "13_jtac_checks.jpg")


def c12_sortie():
    c = card("A FULL ", "SORTIE", ["Off the boat and back  ·  every SkyEye call, in order",
                                  "Hornet, Case I, Persian Gulf frequencies."], 14, EYE_CVN)
    NK.band(c, "OUTBOUND")
    NK.steps(c, [
        ("DECKBOSS 128.6", "request taxi  ·  ready cat N  ·  airborne"),
        ("MARSHAL 306.3", "clear seven miles"),
        ("COMMAND 282.0", "checking in  ·  fence in  ·  on station"),
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
        ("MARSHAL 306.3", "commencing  ·  initial at 3 nm"),
        ("PADDLES", "No SkyEye LSO. Ball, trap, clear the landing area."),
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
             15, EYE_TWR)
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
             16, EYE_TWR)
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
             17, EYE_TWR)
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
                               "Wait for each reply before the next call."], 18, EYE_TWR)
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
                             "Give your distance with inbound, initial or final."], 19, EYE_TWR)
    NK.band(c, "THE OVERHEAD", "VMC")
    NK.steps(c, [
        ("INBOUND", "Minhad Tower, Raider 1, 10 mile initial."),
        ("", "Reply: runway, wind, altimeter, pattern, report initial."),
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
                                 "Silent calls are heard but get no reply."], 20, EYE_TWR)
    NK.band(c, "GROUND AND DEPARTURE", "tower replies")
    c.kv([
        ("request taxi  /  ready to taxi", "taxi, hold short"),
        ("holding short  /  at the hold", "line up and wait"),
        ("ready for takeoff  /  request takeoff", "cleared for takeoff"),
        ("airborne  /  departing  /  rolling", "angels, expect handoff"),
        ("(at 7 miles the tower calls you)", "contact Command"),
        ("seven DME  /  seven miles", "copy (optional)"),
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
        ("radio check  /  checking in", "loud and clear"),
        ("hung ordnance", "straight in, dearm"),
        ("mayday  /  pan pan  /  emergency", "runway is yours"),
        ("tally  /  no joy  /  altitude check  /  roger", "silent"),
    ])
    NK.subtle(c, "Anything else gets \"say again your request\".")
    return save(c, "18_tower_calls.jpg")


def c19_trouble():
    c = card("NO ", "ANSWER?", ["Work down the list  ·  most misses are the first three",
                                "Tell the operator what you said, word for word."], 21)
    NK.band(c, "CHECK", "in order")
    NK.steps(c, [
        ("1", "Waited 2 - 3 seconds?"),
        ("2", "Led with the role or field name - Marshal, Minhad Tower?"),
        ("3", "Said your full callsign?"),
        ("4", "Used a trigger word from the calls cards?"),
        ("5", "Half-second pause after keying - call not clipped?"),
        ("6", "Right frequency? Marshal 306.3 everywhere; Hitman 61.0 FM."),
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
        [("Paddles never answers.", True), ("There is no LSO role yet.", False)],
        [("Deckboss uses the DCS AI carrier frequency.", True),
         ("You may hear the AI carrier controller on 128.600 as well.", False)],
    ])
    NK.subtle(c, "Built from the SkyeyeATC code, origin/main 404e399, 15 OCT 2026.")
    return save(c, "19_trouble.jpg")


CARD_FUNCS = [c01_how, c02_freqs, c03_deckboss_flow, c04_deckboss_calls, c05_marshal_case1,
              c06_marshal_case3, c07_marshal_calls_inbound, c08_marshal_calls_stack,
              c09_marshal_calls_case3, c10_departure, c11_command, c12_jtac_flow,
              c13_jtac_checks, c12_sortie, c13_towers,
              c14_tower_radios, c15_syria_towers, c16_tower_departure, c17_tower_arrival,
              c18_tower_calls, c19_trouble]


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
                     "Command  ·  Deckboss  ·  Marshal  ·  JTAC  ·  Towers\nPersian Gulf  ·  Syria  ·  Black Sea",
                     airframe="HORNET  ·  VIPER", squadron="vSFG-7  ·  AI ATC ON SRS",
                     version=VER, date=DATE, cover_logo=LOGO, cover_logo_mode="circle",
                     page_logo=LOGO, page_logo_mode="circle", cache=CACHE, balance=True,
                     footer="vSFG-7  ·  SKYEYE ATC COMMS  ·  %s" % STAMP, divider_text="")
    print("PDF  %d pages  ->  %s (%.1f MB)" % (n, out, os.path.getsize(out) / 1e6))
    if os.path.isdir(DESKTOP_COPY):
        shutil.copy2(out, os.path.join(DESKTOP_COPY, os.path.basename(out)))


if __name__ == "__main__":
    main()
