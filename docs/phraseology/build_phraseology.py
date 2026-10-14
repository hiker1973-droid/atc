# -*- coding: utf-8 -*-
"""vSFG-7 ATC PHRASEOLOGY REFERENCE - builder.

Replaces the April 2026 one-off (Persian Gulf only, ReportLab, no source kept). Operator
2026-10-14: "Lets do a builder ... so we can update those versions. Make sure we push
documentation to git." This file, its inputs and the PDF live in the SkyeyeATC repo.

What keeps it true:
  facts.json    frequencies, read out of the start_*.bat files and SkyEye's tables by
                cmd/atc/docs_test.go (TestDocFacts fails when they drift).
  phrases.json  every pilot call printed here, written by this builder; TestDocPhrases
                runs each through the real SkyEye parser and fails if one no longer
                triggers what this document says.
Replies are SkyEye's first wording (pkg/composer, cmd/atc); a live reply may use another of
its two to four wordings with the same facts.

Pages are portrait kneeboard cards (the squadron's missioncard renderer, so they drop into
a kneeboard as they are), assembled into one PDF with PyMuPDF.

Update:  see README.md beside this file.
Run:     python -I docs/phraseology/build_phraseology.py      (from the repo root)
"""
import datetime
import json
import os
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
MISSIONS = r"C:\Users\afhin\Saved Games\DCS\Missions"
sys.path.insert(0, os.path.join(MISSIONS, "_MISSION_TOOLKIT", "briefing"))
import missioncard as MC  # noqa: E402
import pymupdf  # noqa: E402

VER = "v2.0"
DATE = "14 OCT 2026"          # change together with VER
OUT_NAME = "vSFG7_ATC_Phraseology_%s.pdf" % VER
DESKTOP_COPY = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"
LOGO = os.path.join(MISSIONS, "New Pilot packs", "VMFA225_patch.png")
CARDS = os.path.join(HERE, "_cards")      # renders, not committed

FACTS = json.load(open(os.path.join(HERE, "facts.json"), encoding="utf-8"))
TH = {t["key"]: t for t in FACTS}


def git_head():
    try:
        return subprocess.run(["git", "-C", REPO, "rev-parse", "--short", "HEAD"],
                              capture_output=True, text=True).stdout.strip()
    except OSError:
        return "?"


HEAD = git_head()
CS = "Raider 1-1"              # the example callsign everywhere
PHRASES = []                   # -> phrases.json


def f3(x):
    return "%.3f" % x


# --------------------------------------------------------------------------- calls
# One row = what the pilot says (alternatives split by "  /  "), what comes back, and for
# the test: role, the intent it must trigger, and the address it is said with.
def calls(role, rows, address, tower=""):
    """rows: (say, reply, want). Records every alternative for the test; returns kv rows."""
    out = []
    for say, reply, want in rows:
        if want:
            for alt in say.split("  /  "):
                spoken = alt.replace("N mile", "10 mile").replace("NN", "13").replace("cat N", "cat 2")
                PHRASES.append({"role": role, "say": "%s, %s, %s." % (address, CS, spoken),
                                "tower": tower, "want": want})
        out.append((say, reply))
    return out


TOWER_GROUND = [
    ("request taxi  /  ready to taxi", "taxi to runway NN, hold short", "taxi"),
    ("holding short  /  at the hold", "line up and wait, then cleared", "holding_short"),
    ("ready for takeoff  /  request takeoff", "cleared for takeoff", "takeoff"),
    ("airborne  /  departing", "angels N, call seven DME", "airborne"),
    ("rolling runway NN", "same as airborne (CTAF)", "rolling"),
    ("seven DME  /  seven miles", "contact Command 282.0", "dme"),
    ("pushing command", "good day", "pushing_command"),
    ("request startup", "startup approved", "startup"),
]
TOWER_ARRIVAL = [
    ("N mile initial  /  N miles inbound", "runway, wind, altimeter, pattern", "inbound"),
    ("3 mile initial  /  overhead", "break approved, report base", "overhead"),
    ("break", "report downwind", "break"),
    ("downwind", "number N, report base", "downwind"),
    ("base, gear down", "cleared to land, or affirmative", "base"),
    ("on final  /  5 mile final", "runway, wind, cleared to land", "final"),
    ("request straight in  /  ILS approach", "straight-in approved, report final", "straight_in"),
    ("on final, touch and go", "cleared touch and go", "final"),
    ("going around", "climb runway heading, report downwind", "go_around"),
    ("runway vacated  /  clear of the active", "taxi to parking", "vacated"),
]
TOWER_ANYTIME = [
    ("say again", "your last call, within 2 min", "say_again"),
    ("wind check  /  say altimeter", "wind and altimeter", "wind_check"),
    ("radar check", "angels, range, bearing", "radar_check"),
    ("radio check", "loud and clear", "radio_check"),
    ("hung ordnance", "straight in, dearm at the end", "hung"),
    ("mayday mayday mayday  /  pan pan", "the runway is yours", "emergency"),
]
COMMAND = [
    ("radio check  /  how copy", "loud and clear", "radio_check"),
    ("checking in", "proceed mission", "check_in"),
    ("fence in, state 6.2", "copy fence in, cleared hot", "fence_in"),
    ("on station", "good hunting", "on_station"),
    ("off station", "proceed to assigned pattern", "off_station"),
    ("fence out", "safe passage", "fence_out"),
    ("RTB  /  request recovery", "expect handoff at 30 nm", "rtb"),
    ("state 5.2", "copy state; under 2.0 bingo", "state"),
    ("say bullseye", "bullseye NNN for NN", "bullseye"),
    ("request tanker", "bearing, range, angels", "tanker"),
    ("request tasking", "Black Sea: contact Hitman 1-1", "tasking"),
]
JTAC = [
    ("checking in  /  request tasking", "9-line target N, as fragged", "check_in"),
    ("copy", "good readback, report IP inbound", "copy"),
    ("IP inbound", "continue", "ip_inbound"),
    ("in from the south  /  in heading 330", "cleared hot, or abort", "in"),
    ("off  /  off safe", "egress per line 9", "off"),
    ("request BDA", "impacts, then kills", "bda"),
    ("abort", "copy abort, re-attack from the IP", "abort"),
    ("radio check", "loud and clear", "radio_check"),
]
MARSHAL = [
    ("checking in", "range and bearing, marking moms please", "check_in"),
    ("marking moms, angels 12, state 6.5", "case, BRC, Marshal at angels N", "marking_moms"),
    ("say BRC", "mother's BRC, magnetic", "brc"),
    ("20 DME", "radar contact, continue", "dme"),
    ("see you at ten", "report altitude and range", "see_you_at_ten"),
    ("state 5.2", "copy state; under 2.0 expedite", "state"),
    ("established angels 2", "signal Charlie, or hold", "established"),
    ("commencing", "check in three mile initial", "commencing"),
    ("platform", "final bearing NNN (Case III)", "platform"),
    ("initial", "contact paddles", "initial"),
    ("clear seven miles", "push Command 282.0", "clear"),
    ("radar check", "radar contact, angels, range", "radar_check"),
    ("radio check", "loud and clear", "radio_check"),
]
DECKBOSS = [
    ("request taxi  /  green jet", "cleared to cat N, or the conga line", "taxi"),
    ("ready cat N  /  shoot", "under tension, spin it up", "tension"),
    ("airborne", "push Marshal 306.3", "airborne"),
    ("say BRC", "mother's BRC, magnetic", "brc"),
    ("pushing Marshal", "short ack", "push"),
    ("radio check", "loud and clear", "radio_check"),
]


# --------------------------------------------------------------------------- cards
EYE = "vSFG-7  ·  AI ATC PHRASEOLOGY"
PAGES = []


def card(title_plain, title_gold, sublines):
    c = MC.Card(EYE, title_plain, title_gold, sublines)
    c._n = len(PAGES) + 1
    PAGES.append(c)
    return c


def bul(*items):
    return [[(a, True), (b, False)] for a, b in items]


def p01_how():
    c = card("ATC ", "PHRASEOLOGY", ["SkyEye AI controllers on SRS  ·  every theatre",
                                     "Built from SkyeyeATC %s  ·  %s  ·  %s" % (HEAD, VER, DATE)])
    c.band("THE CONTROLLERS", "who answers")
    c.kv([("TOWER  -  one per airfield, ATIS beside it", "page 2 - 3"),
          ("COMMAND  -  squadron net", "%s AM" % f3(TH["pg"]["command"])),
          ("HITMAN 1-1  -  JTAC, Black Sea training", "%s FM" % f3(TH["caucasus"]["jtac"])),
          ("DECKBOSS  -  carrier deck", "%s AM" % f3(TH["pg"]["deckboss"])),
          ("MARSHAL  -  carrier stack", "%s AM" % f3(TH["pg"]["marshal"]))])
    c.band("HOW TO BE HEARD", "every call")
    c.bullets(bul(
        ("Address first, then your full callsign.", "\"Batumi Tower, Raider 1-1, request taxi.\" "
         "No address, no answer - most silences are a misheard field name."),
        ("Use the words on these pages.", "SkyEye listens for key words, not exact sentences; "
         "these are the ones it matches."),
        ("Pause half a second after keying,", "then speak. Keep it under 20 seconds - two short "
         "calls beat one long one."),
        ("Wait 2 - 3 seconds for the answer.", "Recognition, reply and voice take that long. "
         "Do not re-key over it."),
        ("Radios AM, encryption off.", "Hitman 1-1 is FM (61.000)."),
    ))
    c.band("SPEECH FIXES", "SkyEye corrects these")
    c.bullets(bul(
        ("Callsigns:", "reader / radar / rater -> Raider; vino / venue / demon -> Venom."),
        ("Field names:", "a letter or two off still counts (helmand -> Minhad, delta for -> Dhafra)."),
    ))
    c.subtle("ATIS is listen-only. There is no SkyEye LSO - paddles never answers.")
    return c


def freq_card(keys, n):
    c = card("FREQUEN", "CIES %d" % n, ["What SkyEye listens on  ·  read from the start scripts",
                                         "Tower + ATIS per field  ·  AM unless marked"])
    for k in keys:
        t = TH[k]
        roles = ["Command %s" % f3(t["command"])]
        if t.get("jtac"):
            roles.append("Hitman %s FM" % f3(t["jtac"]))
        if t.get("deckboss"):
            roles.append("Deckboss %s" % f3(t["deckboss"]))
        if t.get("marshal"):
            roles.append("Marshal %s" % f3(t["marshal"]))
        c.band(t["title"].upper())
        c.subtle("  ·  ".join(roles), size=15)
        atis = {a["icao"]: a for a in t["atis"]}
        rows = []
        for tw in t["towers"]:
            a = atis.pop(tw["icao"], None)
            rows.append((tw["name"].upper(), "%s   ATIS %s" % (f3(tw["freq"]), f3(a["freq"]) if a else "  -  ")))
        for a in atis.values():
            rows.append((a["name"].upper() + "  (no tower)", f3(a["freq"])))
        c.kv(rows)
    c.subtle("Presets differ per mission - check the frequency, not only the channel number.")
    return c


def p_tower_departure():
    c = card("DEPAR", "TURE", ["Parking to Command  ·  same calls at every field",
                                      "Lead with the field name: \"Batumi Tower, Raider 1-1, ...\""])
    c.band("YOU SAY", "tower replies")
    c.kv(calls("tower", TOWER_GROUND, "Batumi Tower", "Batumi Tower"))
    c.band("EXAMPLE", "Batumi, runway 13")
    c.steps([
        ("PILOT", "Batumi Tower, Raider 1-1, request taxi."),
        ("TOWER", "Raider 1-1, Batumi Tower, taxi to runway one three, hold short."),
        ("PILOT", "Batumi Tower, Raider 1-1, holding short."),
        ("TOWER", "Raider 1-1, runway one three, line up and wait."),
        ("TOWER", "Raider 1-1, wind ..., runway one three, cleared for takeoff."),
        ("PILOT", "Batumi Tower, Raider 1-1, airborne."),
        ("TOWER", "Raider 1-1, climb angels four, contact tower at seven DME."),
        ("PILOT", "Batumi Tower, Raider 1-1, seven DME."),
        ("TOWER", "Raider 1-1, contact Command, two eight two point zero."),
    ], split=150)
    c.band("IF YOU ARE HELD")
    c.bullets(bul(
        ("Traffic inside 20 nm or 60 s spacing holds you.", "Stay put - the tower clears you when it "
         "is clear. You do not need to call again."),
        ("\"ready for takeoff\" skips line up and wait", "when there is no traffic."),
        ("\"seven miles\" alone is the departure call.", "Inbound, say \"inbound\" or \"initial\" with it."),
    ))
    return c


def p_tower_arrival():
    c = card("ARRI", "VAL", ["Overhead or straight in  ·  one call per leg",
                                    "Give your distance with inbound, initial or final."])
    c.band("YOU SAY", "tower replies")
    c.kv(calls("tower", TOWER_ARRIVAL, "Batumi Tower", "Batumi Tower"))
    c.band("EXAMPLE", "the overhead")
    c.steps([
        ("PILOT", "Batumi Tower, Raider 1-1, 10 mile initial."),
        ("TOWER", "Raider 1-1, runway one three, wind ..., altimeter ..., report initial."),
        ("PILOT", "Batumi Tower, Raider 1-1, 3 mile initial."),
        ("TOWER", "Raider 1-1, left break runway one three, number one, report base."),
        ("PILOT", "Batumi Tower, Raider 1-1, base, gear down."),
        ("TOWER", "Raider 1-1, runway one three, cleared to land."),
        ("PILOT", "Batumi Tower, Raider 1-1, clear of the active."),
        ("TOWER", "Raider 1-1, taxi to parking."),
    ], split=150)
    c.bullets(bul(
        ("Past 15 nm:", "\"continue inbound, report N miles\"."),
        ("Say \"gear down\" in your base or final call,", "or the tower adds \"check wheels down\"."),
        ("IMC (ceiling under 1,500 ft or vis under 3 nm):", "no overhead - straight in only."),
    ))
    return c


def p_tower_anytime():
    c = card("TOWER ", "CALLS", ["Calls that work in any phase", "and the calls the tower makes to you"])
    c.band("YOU SAY", "tower replies")
    c.kv(calls("tower", TOWER_ANYTIME, "Batumi Tower", "Batumi Tower"))
    c.band("THE TOWER CALLS YOU", "no request needed - needs the rig's Tacview")
    c.kv([("Over 350 kt inside 10 nm", "speed check"),
          ("Inside 5 nm while a jet is rolling", "go around"),
          ("Wind shift changes the runway", "runway in use is now NN"),
          ("Held short, and the traffic has cleared", "cleared for takeoff")])
    c.band("SILENT BY DESIGN")
    c.bullets(bul(("tally  /  no joy  /  roger  /  wilco", "are heard and logged, no reply."),
                  ("Anything else", "gets \"say again your request\".")))
    return c


def p_command():
    c = card("COMM", "AND", ["%s AM  ·  the squadron net, every theatre" % f3(TH["pg"]["command"]),
                             "Stern English voice  ·  lead with \"Command,\""])
    c.band("YOU SAY", "Command replies")
    c.kv(calls("command", COMMAND, "Command"))
    c.band("THE AUTOMATIC HANDOFF", "you do not ask for it")
    c.bullets(bul(
        ("Once you have talked to Command it watches you on Tacview.", "Inside 30 nm of your recovery "
         "- a field, or the carrier if you launched from it - it calls you once with the tower or Marshal."),
        ("Bullseye and tanker need the radar picture.", "No picture: \"unable\" or \"negative tanker on scope\"."),
        ("Tasking is Black Sea training only.", "Elsewhere: \"negative tasking at this time\"."),
    ))
    return c


def p_jtac():
    j = TH["caucasus"]["jtac"]
    c = card("HITMAN ", "1-1", ["JTAC  ·  %s FM  ·  Hornet COMM 2 CH 12  ·  Viper COMM 2 CH 8" % f3(j),
                               "Black Sea training 9-lines  ·  lead with \"Hitman,\""])
    c.band("YOU SAY", "Hitman replies")
    c.kv(calls("jtac", JTAC, "Hitman"))
    c.band("WHAT HITMAN CHECKS", "from Tacview, against your 9-line card")
    c.bullets(bul(
        ("He picks the target.", "One of the five cards at random - find it in your kneeboard."),
        ("IP inbound:", "within 5 nm of the IP, nose on the target."),
        ("In call:", "inside the card's final attack heading and 0.5 - 15 nm out, or abort, abort, abort."),
        ("After cleared hot", "he aborts a run that swings 10 deg outside the window before release."),
        ("BDA about 15 s after impact:", "impacts against the aim point, then kills, priority targets "
         "first. Bridges cannot be assessed."),
        ("No picture of you:", "no checks - you are still cleared hot."),
    ))
    c.subtle("JTAC LIVE 60.000 is for a human JTAC. Hitman never answers there.")
    return c


def p_deckboss():
    c = card("DECK", "BOSS", ["Carrier deck  ·  %s AM  ·  lead with \"Deckboss,\"" % f3(TH["pg"]["deckboss"]),
                             "Shares 128.600 with the DCS AI carrier controller."])
    c.band("YOU SAY", "Deckboss replies")
    c.kv(calls("deckboss", DECKBOSS, "Deckboss"))
    c.band("THE RULES")
    c.bullets(bul(
        ("Nobody calls the shot.", "Run up on \"spin it up\" - the shooter fires you."),
        ("Four cats, then the conga line (six).", "Full: \"deck is full, hold clear\"."),
        ("The airborne call needs no address:", "\"Raider 1-1, airborne\"."),
    ))
    return c


def p_marshal():
    c = card("MAR", "SHAL", ["Carrier stack  ·  %s AM  ·  lead with \"Marshal,\"" % f3(TH["pg"]["marshal"]),
                            "Marshal ignores a call without its name or your callsign's number."])
    c.band("YOU SAY", "Marshal replies")
    c.kv(calls("marshal", MARSHAL, "Marshal"))
    c.band("CASE III", "night, or ceiling under 1,500 ft / vis under 3 nm")
    c.bullets(bul(
        ("Marking moms gets a holding radial and a push time:", "hold on the radial NNN at angels N, "
         "commence at MM."),
        ("Commencing:", "descend to platform 5,000, final bearing NNN. Call \"platform\" passing 5,000 ft "
         "- that is your last call."),
        ("Radial = BRC + 180.  Final bearing = BRC - 9.", "All magnetic."),
    ))
    return c


def p_trouble():
    c = card("NO ", "ANSWER?", ["Work down the list", "Tell the operator what you said, word for word."])
    c.band("CHECK", "in order")
    c.steps([("1", "Waited 2 - 3 seconds?"),
             ("2", "Led with the field or role name?"),
             ("3", "Said your full callsign, with its number?"),
             ("4", "Used a call from these pages?"),
             ("5", "Half-second pause after keying - not clipped?"),
             ("6", "Right frequency (page 2 - 3)? AM - or FM for Hitman?"),
             ("7", "SRS shows your TX light? Encryption off?")], split=90)
    c.band("KNOWN QUIRKS")
    c.bullets(bul(
        ("The same call can get a different reply.", "Each answer has two to four wordings."),
        ("Position calls need Tacview.", "Radar check, bullseye, Hitman's checks and the 30 nm handoff "
         "go quiet or say \"unable\" without it."),
        ("ATIS letter only changes with the weather.", "Alpha three times running is normal."),
    ))
    return c


def build():
    MC.set_logo(LOGO, "circle")
    os.makedirs(CARDS, exist_ok=True)
    for f in os.listdir(CARDS):
        os.remove(os.path.join(CARDS, f))
    p01_how()
    freq_card(["pg", "caucasus", "syria"], 1)
    freq_card(["germany", "iraq", "kola"], 2)
    p_tower_departure()
    p_tower_arrival()
    p_tower_anytime()
    p_command()
    p_jtac()
    p_deckboss()
    p_marshal()
    p_trouble()
    total = len(PAGES)
    bad = [(c._n, c.overflows) for c in PAGES if c.overflows]
    if bad:
        sys.exit("text overflows (page, rows): %r" % bad)
    paths = []
    for c in PAGES:
        c.footer("vSFG-7  ·  ATC PHRASEOLOGY  ·  %s  ·  %s  ·  %s" % (VER, DATE, HEAD), "PAGE %d / %d" % (c._n, total))
        p = os.path.join(CARDS, "%02d.png" % c._n)
        c.save(p)
        paths.append(p)

    json.dump(PHRASES, open(os.path.join(HERE, "phrases.json"), "w", encoding="utf-8", newline="\n"), indent=1)
    out = os.path.join(HERE, OUT_NAME)
    doc = pymupdf.open()
    for p in paths:
        img = pymupdf.open(p)
        r = img[0].rect
        w = 612.0
        page = doc.new_page(width=w, height=w * r.height / r.width)
        page.insert_image(page.rect, filename=p)
    doc.set_metadata({"title": "vSFG-7 AI ATC Phraseology Reference %s" % VER, "author": "vSFG-7",
                      "subject": "SkyEye ATC pilot calls, built from SkyeyeATC %s" % HEAD})
    for f in os.listdir(HERE):
        if f.startswith("vSFG7_ATC_Phraseology_") and f.endswith(".pdf") and f != OUT_NAME:
            os.remove(os.path.join(HERE, f))       # older versions live in git history
    doc.save(out, deflate=True, garbage=3)
    if os.path.isdir(DESKTOP_COPY):
        shutil.copy2(out, os.path.join(DESKTOP_COPY, OUT_NAME))
    print("PDF %d pages, %d phrases -> %s (%.1f MB)" % (total, len(PHRASES), out, os.path.getsize(out) / 1e6))


if __name__ == "__main__":
    build()
