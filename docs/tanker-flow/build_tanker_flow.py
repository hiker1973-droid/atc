# -*- coding: utf-8 -*-
"""Texaco Tanker Comms Flow - one-page pilot sheet (ReportLab).

The quick flow for the SkyEye tanker net, Texaco 1-1 on 252.100 AM (training
test). Every reply here is read out of cmd/atc/tanker.go and
command_position.go (the Command handoff).

Run:  python -I docs/tanker-flow/build_tanker_flow.py      (from the repo root)
"""
import os
import shutil

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import KeepTogether, Paragraph, SimpleDocTemplate, Table, TableStyle

HERE = os.path.dirname(os.path.abspath(__file__))
BUILD = "SkyEye fb4473f, 19 Oct 2026"
OUT_NAME = "Texaco Tanker Comms Flow.pdf"
DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation",
           r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev"]

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=colors.HexColor("#555555"), alignment=1, spaceAfter=3)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=4, spaceAfter=1.5)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=8, leading=9.4)
hdr = ParagraphStyle("hd", parent=cell, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=8, leading=9.4, spaceBefore=1.5)


def table(head, rows, widths):
    data = [[Paragraph(x, hdr) for x in head]] + [[Paragraph(x, cell) for x in r] for r in rows]
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("TOPPADDING", (0, 0), (-1, -1), 2), ("BOTTOMPADDING", (0, 0), (-1, -1), 2),
    ]))
    return t


def section(heading, tbl, *notes):
    return KeepTogether([Paragraph(heading, h), tbl] + [Paragraph(n, note) for n in notes])


S = [
    Paragraph("Texaco Tanker Comms Flow", title),
    Paragraph("vSFG7 SkyEye &middot; Caucasus (Day / Night) &middot; Command 282.000 &rarr; Texaco 1-1 252.100 AM "
              "&middot; training test &middot; %s" % BUILD, sub),
    Paragraph("<b>Texaco 1-1:</b> KC-130, two wing drogues (hose and drogue, left and right) &middot; "
              "<b>252.100 AM</b> (Hornet COMM 2 CH 8) &middot; TACAN <b>51Y TX1</b> &middot; orbit <b>angels 13, 250 KIAS</b>. "
              "Say your callsign, with its number, in every call.", note),
]

S.append(section("1. Command (282.000): find the tanker (optional)",
    table(["Pilot", "Command"], [
        ["\"Command, Raider 302, <b>request tanker</b>\"<br/>(or \"say Texaco\", \"nearest tanker\")",
         "\"Raider 302, Command, Texaco 1-1 bears 045 for 32, angels 13. "
         "<b>Contact Texaco one one on two five two point one.</b>\""],
    ], [3.0, 4.4]),
    "Bearings are magnetic from your jet. Calling \"Bingo\" to Command also gives you the nearest tanker."))

S.append(section("2. Texaco 1-1 (252.100): check in to depart",
    table(["Step", "Pilot", "Texaco"], [
        ["<b>Check in</b>",
         "\"Texaco 1-1, Raider 302, one Hornet, 15 miles, <b>intent to refuel</b>\"",
         "\"Raider 302, Texaco one one, angels one three, two five zero knots, <b>how much do you request?</b>\""],
        ["<b>Give</b>",
         "\"Raider 302, <b>request 5.0</b>\"<br/>(or \"five thousand pounds\", \"full\", \"top off\")",
         "\"Raider 302, Texaco copies five thousand pounds.\"<br/>(\"full\": \"Texaco copies, full give.\")"],
        ["<b>Trail</b>",
         "\"Raider 302, <b>one mile trail</b>\"",
         "\"Raider 302, Texaco, form on my <b>left</b>.\" (second flight: my <b>right</b>)"],
        ["<b>In position</b>",
         "\"Raider 302, <b>established</b>\"",
         "\"Raider 302, Texaco, copy, you are <b>cleared port side, pre-contact</b>.\" "
         "(right drogue: starboard side)"],
        ["<b>In DCS</b>",
         "Comms menu: <b>Ready pre-contact</b>, then plug and take fuel",
         "DCS handles the plug and the fuel transfer"],
        ["<b>Done</b>",
         "\"Raider 302, <b>refuel complete</b>\"",
         "\"Raider 302, Texaco, copy, <b>cleared to depart</b>. Good hunting.\""],
    ], [0.9, 3.0, 3.5]),
    "<b>Shortcut:</b> check in and give in one call: \"Texaco 1-1, Raider 302, intent to refuel, request 6.0\" &rarr; "
    "\"...angels one three, two five zero knots, copies six thousand pounds.\"",
    "Angels come live from Tacview, so if Texaco moves, the number moves. Speed is always the planned 250 KIAS."))

S.append(section("Words Texaco understands",
    table(["Step", "Any of these"], [
        ["Check in", "intent to refuel &middot; request fuel / gas &middot; need fuel &middot; checking in &middot; inbound"],
        ["Give", "5.0 / five point oh &middot; 5 thousand &middot; 5000 pounds &middot; full &middot; top off &middot; fill it up &middot; max"],
        ["Trail", "one mile trail &middot; in trail &middot; astern"],
        ["In position", "established &middot; in position &middot; on your left / right &middot; port / starboard wing &middot; "
                        "observation &middot; pre-contact"],
        ["Done", "refuel complete &middot; disconnecting &middot; done &middot; thanks &middot; departing &middot; clear of the tanker"],
    ], [1.2, 6.2])))

S.append(section("Tips",
    table(["Item", "Detail"], [
        ["Order", "Texaco only moves you on in order: check in, give, trail, in position. \"Established\" before a give "
                  "is not answered; give your amount first."],
        ["Two flights", "The first flight to call trail gets the left drogue; a second flight within 10 minutes gets the right."],
        ["No callsign heard", "\"Station calling Texaco one one, say again your callsign.\" Call again with your callsign."],
        ["Fuel won't flow?", "SkyEye's clearance is radio only. DCS still needs your comms-menu <b>Ready pre-contact</b> "
                             "before the drogue will pass fuel."],
        ["No answer?", "Wait 3-4 s, then repeat once. Check 252.100 <b>AM</b>. Report callsign, frequency and time on Discord."],
    ], [1.4, 6.0])))

out = os.path.join(HERE, OUT_NAME)
doc = SimpleDocTemplate(out, pagesize=letter, leftMargin=0.55 * inch, rightMargin=0.55 * inch,
                        topMargin=0.35 * inch, bottomMargin=0.3 * inch,
                        title="Texaco Tanker Comms Flow", author="vSFG7")
doc.build(S)
for d in DESKTOP:
    if os.path.isdir(d):
        shutil.copy2(out, os.path.join(d, OUT_NAME))
print("ok", out)
