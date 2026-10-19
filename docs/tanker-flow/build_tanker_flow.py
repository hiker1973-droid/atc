# -*- coding: utf-8 -*-
"""Texaco Tanker Comms Flow - one-page pilot sheet (ReportLab).

The quick flow for the SkyEye tanker net, Texaco 1-1 on 252.100 AM (training
test). The pilot calls are the vSFG-7 AAQ "Aerial Refueling Procedure" comms
table; every reply here is read out of cmd/atc/tanker.go and
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
BUILD = "SkyEye, 19 Oct 2026"
OUT_NAME = "Texaco Tanker Comms Flow.pdf"
DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation",
           r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev"]

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=colors.HexColor("#555555"), alignment=1, spaceAfter=3)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=4, spaceAfter=1.5)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.8, leading=9.1)
hdr = ParagraphStyle("hd", parent=cell, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.8, leading=9.1, spaceBefore=1.5)


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
    Paragraph("Texaco Tanker Comms Flow", title),
    Paragraph("vSFG7 SkyEye &middot; Caucasus (Day / Night) &middot; Command 282.000 &rarr; Texaco 1-1 252.100 AM "
              "&middot; training test &middot; %s" % BUILD, sub),
    Paragraph("<b>Texaco 1-1:</b> KC-130, two baskets (port and starboard) &middot; <b>252.100 AM</b> (Hornet COMM 2 CH 8) "
              "&middot; TACAN <b>51Y TX1</b> &middot; <b>13,000 ft, velocity 336</b>. Calls follow the vSFG-7 AAQ "
              "Aerial Refueling Procedure: observation is always <b>port</b>, reform always <b>starboard</b>. "
              "Say your callsign, with its number, in every call.", note),
]

S.append(section("1. Command (282.000): find the tanker (optional)",
    table(["Pilot", "Command"], [
        ["\"Command, Knight 1-1, <b>request tanker</b>\"<br/>(or \"say Texaco\", \"nearest tanker\")",
         "\"Knight 1-1, Command, Texaco 1-1 bears 045 for 32, angels 13. "
         "<b>Contact Texaco one one on two five two point one.</b>\""],
    ], [3.0, 4.4]),
    "Switch to the tanker at about 20 DME. Flight safe on interflight first: \"Flight, switches safe, noses cold.\""))

S.append(section("2. Texaco 1-1 (252.100): initial to checking out",
    table(["Step", "Pilot (SRS)", "Texaco"], [
        ["<b>Initial</b><br/>~20 DME",
         "\"Texaco 1, Knight 1-1, flight of 2, 15 mile initial for <b>port observation</b> with intent to refuel\"",
         "\"Knight 1-1, Texaco one one, cleared port observation, one three thousand, velocity three three six. "
         "<b>How much do you request?</b>\""],
        ["<b>Give</b>",
         "\"Knight 1-1, <b>request 5.0</b>\"<br/>(or \"five thousand pounds\", \"full\")",
         "\"Knight 1-1, Texaco copies five thousand pounds.\""],
        ["<b>1 mile trail</b>",
         "\"Texaco 1, Knight 1-1, flight of 2, <b>1 mile trail</b> for port observation\"",
         "\"Knight 1-1, Texaco, continue port observation.\""],
        ["<b>Observation</b>",
         "\"Texaco 1, Knight 1-1, <b>established port observation</b>\"",
         "\"Knight 1-1, Texaco, copy. <b>Port and starboard baskets open.</b>\""],
        ["<b>Basket</b>",
         "\"Knight 1-3, <b>moving to starboard basket</b>\"",
         "\"Knight 1-3, Texaco, cleared starboard basket.\"<br/>"
         "Taken: \"standby, starboard basket occupied. Port basket open.\""],
        ["<b>At the basket</b>",
         "\"Knight 1-3, <b>established starboard basket</b>\"",
         "\"Knight 1-3, Texaco, copy, call ready pre-contact.\""],
        ["<b>In DCS</b>",
         "AI comms menu: <b>Ready pre-contact</b>",
         "DCS tanker: \"Cleared contact\" (or \"Return pre-contact\": a little closer and higher). Plug and take fuel."],
        ["<b>Reform</b>",
         "\"Knight 1-3, <b>satisfied, moving to starboard reform</b>\"<br/>"
         "\"Knight 1-3, <b>established starboard reform</b>\"",
         "\"Knight 1-3, Texaco, cleared starboard reform.\"<br/>\"Knight 1-3, Texaco, copy.\""],
        ["<b>Checking out</b>",
         "\"Texaco 1, Knight 1-1, <b>checking out, resuming navigation, pushing 282</b>\"",
         "\"Knight 1-1, Texaco, copy, cleared to depart. Good hunting.\""],
    ], [0.95, 3.15, 3.3]),
    "<b>Flight lead</b> makes the initial, trail, observation and checking-out calls for the flight. "
    "<b>Each jet</b> makes its own basket and reform calls. "
    "<b>Shortcut:</b> give your amount in the initial (\"...with intent to refuel, request 6.0\"): Texaco copies it "
    "instead of asking.",
    "Texaco's altitude comes live from Tacview; velocity is the procedure's KC-130 figure, 336 at every altitude."))

S.append(section("Words Texaco understands",
    table(["Step", "Any of these"], [
        ["Initial", "15 mile initial &middot; intent to refuel &middot; request fuel / rejoin &middot; checking in &middot; inbound"],
        ["Give", "5.0 / five point oh &middot; 5 thousand &middot; 5000 pounds &middot; full &middot; top off"],
        ["Trail", "1 mile trail &middot; in trail &middot; astern"],
        ["Observation", "established port observation &middot; established (before a basket)"],
        ["Basket", "moving to port / starboard basket (or drogue, hose; left / right) &middot; moving to the basket "
                   "(Texaco gives the open one) &middot; established ... basket &middot; ready pre-contact"],
        ["Reform", "satisfied &middot; moving to starboard reform &middot; refuel complete &middot; disconnecting &middot; "
                   "established starboard reform"],
        ["Checking out", "checking out &middot; resuming navigation &middot; pushing 282 &middot; departing &middot; "
                         "clear of the tanker"],
    ], [1.2, 6.2])))

S.append(section("Rules and tips",
    table(["Item", "Detail"], [
        ["Baskets", "One jet per basket, two at most behind the KC-130. Texaco won't clear you to a basket another jet "
                    "holds; it is free again when that jet calls reform (or after 10 minutes without a call)."],
        ["Moving", "One jet moves at a time. Cross under and behind other aircraft. Never overtake the tanker. "
                   "Don't reform the flight order in the reform area."],
        ["Departing", "Stay in starboard reform until the flight is done, then a right clearing turn and/or climb; "
                      "never descend."],
        ["No callsign heard", "\"Station calling Texaco one one, say again your callsign.\" Call again with your callsign."],
        ["Fuel won't flow?", "SkyEye's calls are radio only. DCS still needs the AI menu <b>Ready pre-contact</b> "
                             "before the basket will pass fuel."],
        ["No answer?", "Wait 3-4 s, then repeat once. Check 252.100 <b>AM</b>. Report callsign, frequency and time on Discord."],
    ], [1.4, 6.0])))

out = os.path.join(HERE, OUT_NAME)
doc = SimpleDocTemplate(out, pagesize=letter, leftMargin=0.55 * inch, rightMargin=0.55 * inch,
                        topMargin=0.32 * inch, bottomMargin=0.28 * inch,
                        title="Texaco Tanker Comms Flow", author="vSFG7")
doc.build(S)
for d in DESKTOP:
    if os.path.isdir(d):
        shutil.copy2(out, os.path.join(d, OUT_NAME))
print("ok", out)
