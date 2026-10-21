# -*- coding: utf-8 -*-
"""Darkstar / Overlord GCI Comms Flow - one-page pilot sheet (ReportLab).

The GCI is SkyEye's brain built into atc.exe with our voice (cmd/atc/gci.go,
pkg/gci; operator 2026-10-21). Every reply here is read out of the code:
gci.go (check in, radio check, checking out, fast / slow movers),
gci_recovery.go (quiet in recoveries), pkg/gci/composer (SkyEye's wording),
command.go (the hint after "fence in"), gciAck (fence in / out, splash). Darkstar 264.000 on Caucasus Day /
Night (Hornet COMM1 CH5 on the Black Sea card), Overlord 266.000 on Syria.

Run:  python -I docs/gci-flow/build_gci_flow.py      (from the repo root)
"""
import os
import shutil
import subprocess

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import KeepTogether, Paragraph, SimpleDocTemplate, Table, TableStyle

HERE = os.path.dirname(os.path.abspath(__file__))
OUT_NAME = "Darkstar GCI Comms Flow.pdf"
DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"]
try:
    HEAD = subprocess.run(["git", "-C", HERE, "log", "-1", "--format=%h"], capture_output=True, text=True).stdout.strip()
except OSError:
    HEAD = "?"
BUILD = "SkyEye %s, 21 Oct 2026" % HEAD

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=colors.HexColor("#555555"), alignment=1, spaceAfter=3)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=4, spaceAfter=1.5)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.4, leading=8.6)
hdr = ParagraphStyle("hd", parent=cell, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.4, leading=8.6, spaceBefore=1.5)


def table(head, rows, widths):
    data = [[Paragraph(x, hdr) for x in head]] + [[Paragraph(x, cell) for x in r] for r in rows]
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("TOPPADDING", (0, 0), (-1, -1), 1.8), ("BOTTOMPADDING", (0, 0), (-1, -1), 1.8),
    ]))
    return t


def section(heading, tbl, *notes):
    return KeepTogether([Paragraph(heading, h), tbl] + [Paragraph(n, note) for n in notes])


S = [
    Paragraph("Darkstar / Overlord GCI Comms Flow", title),
    Paragraph("vSFG7 SkyEye &middot; AI AWACS on SRS &middot; Black Sea Day / Night: <b>Darkstar 264.000 AM</b> "
              "(Hornet COMM 1 CH 5) &middot; Syria / Foothold: <b>Overlord 266.000 AM</b> &middot; %s" % BUILD, sub),
    table(["Before you call", "Detail"], [
        ["<b>Your DCS name</b>", "Put the squadron callsign in it: <b>Raider 302 | Jedi</b>, <b>Venom 212 | Dabs</b> "
                                 "(it may sit anywhere, e.g. \"vSFG-7 | Raider 302 | Jedi\"). Without Raider / Venom NNN "
                                 "the GCI cannot find you on scope."],
        ["<b>Your SRS name</b>", "The same as your DCS name, or threat and merged calls will not reach you."],
        ["<b>Every call</b>", "\"<b>Darkstar</b>\" (or \"Overlord\", or \"Anyface\"), <b>your callsign</b>, the request: "
                              "\"Darkstar, Raider 3-0-2, picture.\""],
        ["<b>Numbers</b>", "Bearings three digits, one at a time (\"zero six five\"); ranges and altitudes as normal "
                           "numbers (\"twenty-five\", \"twelve thousand\"). Plain three, five, nine - not tree, fife, niner."],
    ], [1.25, 6.15]),
]

S.append(section("1. The flow, in order",
    table(["Step", "Pilot (SRS)", "Darkstar"], [
        ["<b>Command</b><br/>282.000",
         "\"Command, Raider 302, checking in.\" - tasking, Hitman, tankers as usual",
         "\"... proceed mission.\" (a fence in on Command adds \"Darkstar is up on two six four point zero for picture.\")"],
        ["<b>Check in</b><br/>264.000",
         "\"Darkstar, Raider 302, <b>checking in</b>.\"",
         "\"RAIDER 3 0 2, DARKSTAR, <b>radar contact</b>.\""],
        ["<b>Radio check</b>",
         "\"Darkstar, Raider 302, <b>radio check</b>.\" (or comm check)",
         "\"RAIDER 3 0 2, DARKSTAR, <b>good copy</b>.\" / \"... <b>5 by 5</b>.\" "
         "Off scope: \"..., but I don't see you on scope.\""],
        ["<b>Fence in</b>",
         "\"Darkstar, Raider 302, <b>fence in</b>.\"",
         "\"RAIDER 3 0 2, DARKSTAR, <b>copy fence in, picture follows</b>.\" - then the picture"],
        ["<b>Alpha check</b>",
         "\"Darkstar, Raider 302, <b>alpha check</b>.\"",
         "\"..., contact, alpha check bullseye 010/48\" (your position, a check on the INS)"],
        ["<b>Picture</b>",
         "\"Darkstar, Raider 302, <b>picture</b>.\"",
         "\"Darkstar, 2 groups. Group bullseye 010/48, 25000, track southwest, hostile, 2 contacts, Flanker. "
         "Group ...\" (the top 3 groups)"],
        ["<b>Bogey dope</b>",
         "\"..., <b>bogey dope</b>.\" &middot; \"..., any <b>fast movers</b>?\" (jets only) &middot; "
         "\"..., <b>slow movers</b>.\" (helicopters only)",
         "\"Raider 3 0 2, group BRAA 045/32, 25000, hot, hostile, Flanker\" (the nearest threat to you)"],
        ["<b>Declare</b>",
         "\"..., <b>declare</b> bullseye one two four, fifty-one, seven thousand.\"",
         "That group: hostile / friendly / furball / clean"],
        ["<b>Snaplock</b><br/>(BVR, quick)",
         "\"..., <b>snaplock</b> one two five, ten, eight thousand.\"",
         "\"threat BRAA 125/10, 8000, hot, hostile, ...\""],
        ["<b>Spiked / strobe</b>",
         "\"..., <b>spiked</b> zero eight zero.\" (or strobe)",
         "\"spike range 35, 16000, flank northeast, hostile\" - or \"clean 080\""],
        ["<b>Vector</b>",
         "\"..., <b>vector to tanker</b>.\" (or to bullseye)",
         "The nearest tanker for your aircraft: bearing, range, altitude"],
        ["<b>Splash</b>",
         "\"Darkstar, Raider 302, <b>splash one</b>.\"",
         "\"RAIDER 3 0 2, DARKSTAR, <b>copy splash one</b>.\""],
        ["<b>Fence out</b>",
         "\"Darkstar, Raider 302, <b>fence out</b>.\"",
         "\"RAIDER 3 0 2, DARKSTAR, <b>copy fence out</b>.\""],
        ["<b>Check out</b>",
         "\"Darkstar, Raider 302, <b>checking out</b>.\"",
         "\"RAIDER 3 0 2, DARKSTAR, <b>copy, good day</b>.\" Then Command: RTB, Marshal / tower."],
    ], [1.0, 3.0, 3.4]),
    "Fence checks go to Darkstar now (and still work on Command). "
    "Overlord on Foothold says the same with its own name and \"two six six point zero\". "
    "The GCI is the air picture only: tasking, tankers, Hitman and RTB stay with Command."))

S.append(section("2. What Darkstar says without being asked (tuned to 264 only)",
    table(["Call", "When"], [
        ["<b>Picture</b>", "Every 5 minutes while anyone is on frequency, if the picture changed."],
        ["<b>Threat</b>", "A hostile within 25 nm of you (further for long-range fighters); repeats every 3 minutes "
                          "while it stays a threat. BRAA when it is about you alone, bullseye when it is about several."],
        ["<b>Merged</b>", "A hostile jet inside 3 nm of you."],
        ["<b>Faded</b>", "A hostile near you has been off the scope 30 seconds. <b>Not a kill</b> - DCS can hide live aircraft."],
        ["<b>Sunrise</b>", "Darkstar coming online. Mid-mission it means the GCI just restarted (usually an Update)."],
    ], [1.25, 6.15])))

S.append(section("3. Rules and tips",
    table(["Item", "Detail"], [
        ["<b>Quiet in recoveries</b>", "No threat calls to you, and the automatic picture waits, while you are within "
                                       "12 nm of the carrier below 10,000 ft or within 10 nm of a field below 4,000 ft. "
                                       "Anything you ask is still answered."],
        ["<b>On the ramp</b>", "You may not be on scope until airborne - a radio check says so."],
        ["<b>Speak</b>", "Think, then key: steady pace, slight pause between numbers. Misspoke? Release and start again."],
        ["<b>No answer?</b>", "Wait 3-4 s, repeat once. Check 264.000 / 266.000 <b>AM</b> and that you opened with "
                              "\"Darkstar\". Report callsign, frequency and time on Discord."],
    ], [1.25, 6.15])))

out = os.path.join(HERE, OUT_NAME)
doc = SimpleDocTemplate(out, pagesize=letter, leftMargin=0.55 * inch, rightMargin=0.55 * inch,
                        topMargin=0.32 * inch, bottomMargin=0.28 * inch,
                        title="Darkstar / Overlord GCI Comms Flow", author="vSFG7")
doc.build(S)
for d in DESKTOP:
    if os.path.isdir(d):
        shutil.copy2(out, os.path.join(d, OUT_NAME))
print("ok", out)
