# -*- coding: utf-8 -*-
"""Paddles (LSO) Comms Flow - one-page pilot sheet (ReportLab).

The quick flow for SkyEye's LSO, Paddles, on 128.300 AM (Caucasus Day / Night and Foothold Syria,
CVN-72 Abe). Every call and reply here is read out of pkg/lso/lso.go (the calls
and when they fire), cmd/atc/lso.go (ball calls, radio check) and the Marshal
handoff in cmd/atc/marshal.go / pkg/composer.

Made 2026-10-18, in the style of docs/hitman-flow.

Run:  python -I docs/paddles-flow/build_paddles_flow.py      (from the repo root)
"""
import os
import shutil

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import KeepTogether, Paragraph, SimpleDocTemplate, Table, TableStyle

HERE = os.path.dirname(os.path.abspath(__file__))
BUILD = "SkyEye main, 18 Oct 2026"
OUT_NAME = "Paddles LSO Comms Flow.pdf"
DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation",
           r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev"]

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=colors.HexColor("#555555"), alignment=1, spaceAfter=3)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=2, spaceAfter=1.5)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.4, leading=8.5)
hdr = ParagraphStyle("hd", parent=cell, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.4, leading=8.6, spaceBefore=1)


def table(head, rows, widths):
    data = [[Paragraph(x, hdr) for x in head]] + [[Paragraph(x, cell) for x in r] for r in rows]
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("TOPPADDING", (0, 0), (-1, -1), 1.6), ("BOTTOMPADDING", (0, 0), (-1, -1), 1.6),
    ]))
    return t


def section(heading, tbl, *notes):
    return KeepTogether([Paragraph(heading, h), tbl] + [Paragraph(n, note) for n in notes])


S = [
    Paragraph("Paddles (LSO) Comms Flow", title),
    Paragraph("vSFG7 SkyEye &middot; Caucasus (Day / Night) + Foothold Syria, CVN-72 Abe &middot; Marshal 306.300 &rarr; "
              "Paddles 128.300 AM &middot; %s" % BUILD, sub),
    Paragraph("<b>Radios:</b> Marshal 306.300 AM (COMM 1 CH 3) &middot; <b>Paddles 128.300 AM</b> "
              "(no kneeboard preset yet: dial it on COMM 2). Paddles is SkyEye's LSO on the Abe, on Day, Night "
              "and Foothold. He talks you down the glideslope from Tacview: you fly, he calls. "
              "Replies below use Raider 302.", note),
]

S.append(section("1. Marshal (306.300): get handed to Paddles",
    table(["Pilot", "Marshal"], [
        ["\"Union Marshal, Raider 302, <b>three mile initial</b>\" (or just \"initial\")",
         "\"Raider 302, Union Marshal, <b>contact paddles</b>, good luck.\" "
         "(or \"switch to paddles\" / \"paddles has you\")"],
        ["\"Raider 302, <b>switching paddles</b>\" (optional; or \"pushing paddles\")",
         "\"Raider 302, Union Marshal, roger pushing paddles, good day.\""],
    ], [3.3, 4.1]),
    "Switch COMM 2 to 128.300. The rest of the recovery (check-in, stack, Charlie) is in the Comms Packet, "
    "Marshal cards."))

S.append(section("2. Abeam (the 180)",
    table(["Pilot", "Paddles"], [
        ["\"<b>302 abeam</b>\" (or \"Paddles, Raider 302, abeam\")",
         "\"<b>Paddles contact, 302. In the groove at three-quarter mile, call the ball.</b>\""],
        ["\"Paddles, Raider 302, <b>radio check</b>\" (or \"comm check\", \"how copy\")",
         "\"Raider 302, Paddles, loud and clear.\""],
    ], [3.3, 4.1]),
    "Give your modex or full callsign with abeam: Paddles answers with whichever you said."))

S.append(section("3. The ball call (about 3/4 nm, on the glideslope)",
    table(["Pilot", "Paddles"], [
        ["\"<b>302, Hornet ball, 5.5</b>\" (modex, type, fuel state)",
         "\"<b>Roger ball.</b>\""],
        ["\"<b>302, Clara, 5.0</b>\" (ball not in sight)", "\"Paddles contact, keep it coming.\""],
        ["<i>No ball call by about 0.6 nm</i>", "\"<b>Call the ball.</b>\""],
    ], [3.3, 4.1]),
    "Any type works: Hornet, Super Hornet, Rhino, Tomcat, Viper, Goshawk ball. Say the modex first: with two jets "
    "in the groove it is how Paddles knows whose ball it is."))

S.append(section("4. In the groove: Paddles calls, you fly (no readback)",
    table(["You are...", "Paddles says"], [
        ["A little high (0.7&deg; or more)", "\"A little high.\""],
        ["High (1.2&deg; or more)", "\"You're high.\" Still high 3 s later: \"<b>You're high, bring it down.</b>\""],
        ["Low (0.6&deg; or more), far out", "\"You're low, power.\""],
        ["Low, inside about 0.6 nm", "\"<b>Power!</b>\" Still low right after: \"<b>POWER!</b>\""],
        ["In close, well low (1&deg;+) and still sinking", "\"<b>Power, power, POWER!</b>\""],
        ["Coming up through the glideslope fast after a power call", "\"Easy with it.\""],
        ["Left of the landing line (1.5&deg;+)", "\"Right for lineup.\" Far out and 3&deg;+ left: \"You're lined up left.\""],
        ["Right of the landing line (1.5&deg;+)", "\"Come left.\" Far out and 3&deg;+ right: \"You're lined up right.\""],
        ["Off on-speed AoA by 2&deg; (Hornet only)", "\"You're fast.\" / \"You're slow.\""],
    ], [3.3, 4.1]),
    "Calls start at about 0.8 nm (1.3 nm once you've called the ball). Before your ball call: at least 2 s apart, "
    "the same call not again within 8 s. <b>After it: a call every second</b> while you're off, until the ramp. "
    "Low and lineup come before high: the most dangerous error gets the call. "
    "Over the ramp he says nothing but a wave-off."))

S.append(section("5. Wave-offs and bolters",
    table(["When", "Paddles"], [
        ["Inside about 300 m (0.16 nm): more than 3 m below the glideslope, or 10 m off the landing line<br/>"
         "(300 - 600 m: 1.3&deg; low or 5&deg; off line)",
         "\"<b>Wave off, wave off, wave off.</b>\""],
        ["Foul deck at 1/4 nm (about 460 m): the jet ahead called \"red deck\" and not yet \"green deck\", "
         "or another aircraft is on the landing area",
         "\"<b>302, foul deck, wave off, wave off, proceed to bolter pattern.</b>\""],
        ["Touches down and rolls on well past the 4 wire", "\"<b>Bolter, bolter, bolter.</b>\""],
        ["Trap", "Nothing on the radio. Paddles logs the pass, his grade and the wire."],
    ], [3.3, 4.1]),
    "A wave-off is mandatory: power, climb straight ahead, then back to the pattern. After a bolter, fly the "
    "pattern and call the ball again on the next pass. Grades are not spoken yet; the squadron sees them in the "
    "server log next to DCS's own LSO grade."))

S.append(section("6. After your trap: red deck, green deck",
    table(["Pilot (on 128.300)", "Paddles"], [
        ["\"<b>302, red deck</b>\" (as soon as you stop in the wires)",
         "Nothing. The deck is foul: the next jet is waved off at 1/4 nm."],
        ["\"<b>302, green deck</b>\" (once clear of the landing area)",
         "\"302, Paddles, roger green deck, <b>contact Deckboss, one two eight point six</b>.\" "
         "The deck is clear: the next jet lands. Switch to Deckboss to taxi."],
    ], [3.3, 4.1]),
    "Both calls on Paddles' 128.300, not Marshal. Forget \"green deck\" and the jet behind you gets waved off. "
    "A red deck with no green lapses after 3 minutes."))

S.append(section("Tips",
    table(["Item", "Detail"], [
        ["Your modex", "Say it digit by digit (\"three zero two\" or \"3-0-2\"). The ball call needs only the modex, "
         "not the full callsign."],
        ["\"Ball\" heard as \"bolt\"", "Speech recognition often writes \"Hornet ball\" as \"Hornet bolt\"; Paddles "
         "accepts it. Speak clearly and don't clip the start of the call."],
        ["Who is it?", "Paddles is one voice (a woman), on 128.300 only, on Caucasus Day / Night and Foothold (Syria)."],
        ["No answer?", "Wait 2-3 s, then repeat the ball call once. Check AM on 128.300. Report callsign, "
         "frequency and time on Discord."],
    ], [1.5, 5.9])))

out = os.path.join(HERE, OUT_NAME)
doc = SimpleDocTemplate(out, pagesize=letter, leftMargin=0.55 * inch, rightMargin=0.55 * inch,
                        topMargin=0.28 * inch, bottomMargin=0.25 * inch,
                        title="Paddles LSO Comms Flow", author="vSFG7")
doc.build(S)
for d in DESKTOP:
    if os.path.isdir(d):
        shutil.copy2(out, os.path.join(d, OUT_NAME))
print("ok", out)
