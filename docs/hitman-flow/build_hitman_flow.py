# -*- coding: utf-8 -*-
"""Hitman JTAC Comms Flow - one-page pilot sheet (ReportLab).

The quick flow for both SkyEye JTACs, Hitman 1-1 (61.000 FM) and Hitman 2-1
(62.000 FM). The full reference is the Comms Packet (cards 12 - 15) and the
Phraseology Reference (sections 5 - 6). Every reply here is read out of
cmd/atc/jtac.go, jtac_bda.go, command_tasking.go and command_areas.go.

History: a one-off script in a session scratchpad (14 - 15 Oct 2026); moved into git
2026-10-16 for Hitman 2-1 and the four BDA calls (origin/main 15c969b).

Run:  python -I docs/hitman-flow/build_hitman_flow.py      (from the repo root)
"""
import os
import shutil

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import KeepTogether, Paragraph, SimpleDocTemplate, Table, TableStyle

HERE = os.path.dirname(os.path.abspath(__file__))
BUILD = "SkyEye 15c969b, 16 Oct 2026"
OUT_NAME = "Hitman JTAC Comms Flow.pdf"
# Documentation only: the second copy in the folder root went with the 2026-10-21 tidy.
DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"]

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
    Paragraph("Hitman JTAC Comms Flow", title),
    Paragraph("vSFG7 SkyEye &middot; Caucasus (Day / Night) &middot; Command 282.000 &rarr; Hitman 1-1 61.000 / "
              "2-1 62.000 FM &middot; %s" % BUILD, sub),
    Paragraph("<b>Radios:</b> Command 282.000 AM &middot; <b>Hitman 1-1 61.000 FM</b> (Hornet COMM 2 CH 12, Viper COMM 2 CH 8) "
              "runs <b>Kuban Valley, targets 1-3</b> &middot; <b>Hitman 2-1 62.000 FM</b> (no preset yet: dial it) runs "
              "<b>Cherkessk, targets 4-5</b>. Set the radio to <b>FM</b> for both. Same calls on either net; "
              "replies below use Hitman one one.", note),
]

S.append(section("1. Command (282.000): get handed to a Hitman",
    table(["Pilot", "Command"], [
        ["\"Command, Raider 32, <b>request tasking</b>\"",
         "\"Raider 32, Command, contact Hitman one one on 61.0 for tasking.\" (or Hitman two one on 62.0: "
         "whichever has a free target; come back for more and you get the other sector)"],
        ["\"<b>Copy</b>\" / \"wilco\"", "\"Raider 32, Command, switch Hitman one one, good hunting.\""],
        ["\"Command, Raider 32, <b>proceeding to Hotel</b>\" (or \"request Hotel\")<br/>"
         "(kneeboard areas D, H, I are the 9-lines)",
         "\"Raider 32, Command, Hotel is nine-line target 4. Contact Hitman two one on 62.0 for tasking.\" "
         "That target is already drawn for you at check-in."],
    ], [3.3, 4.1]),
    "Command does not pick the target; Hitman does. With no targets: \"negative tasking at this time.\""))

S.append(section("2. Check in and get the target",
    table(["Pilot", "Hitman"], [
        ["\"Hitman one one, Raider 32, <b>checking in</b>\"<br/>(or \"ready for 9-line\", \"request tasking\")",
         "\"Raider 32, Hitman one one, <b>target 2, &lt;name&gt;, IP &lt;IP&gt;.</b> Read back.\"<br/>"
         "More than 5 nm from the IP, he adds: \"&lt;IP&gt; bears 270 for 18.\""],
        ["\"Hitman, Raider 32, <b>target 2, &lt;IP&gt;</b>\" (or \"copy\")",
         "\"Raider 32, Hitman one one, good readback. Report IP inbound.\""],
    ], [3.3, 4.1]),
    "Hitman reads only the target and the IP. The rest of the 9-line (elevation, attack headings, egress) is on your "
    "kneeboard card, and Hitman still checks it from the Tacview picture.",
    "<b>Readback tip:</b> don't say \"Hitman\" partway through your readback; a call like that is dropped as "
    "Hitman's own echo. Put Hitman first, or leave his name out."))

S.append(section("3. The run",
    table(["Pilot", "Hitman checks", "Hitman says"], [
        ["\"Raider 32, <b>IP inbound</b>\"", "Within 20 nm of the IP, nose on the target", "\"Continue.\""],
        ["", "Within 20 nm but off the run-in line", "\"Continue. Run in heading 297 from &lt;IP&gt;.\""],
        ["", "Pointed more than 30&deg; away from the target", "\"Continue. Check heading, target bears 090.\""],
        ["", "More than 20 nm from the IP", "\"Negative, you are not at &lt;IP&gt;. &lt;IP&gt; bears 270 for 26. Report IP inbound from &lt;IP&gt;.\""],
        ["\"Raider 32, <b>in from the west</b>\"<br/>(or \"in hot\", \"in heading 090\", \"rolling in\")",
         "0.5-15 nm from the target and inside the final attack heading window", "\"<b>Cleared hot.</b>\""],
        ["", "Outside the window, too far, or too close",
         "\"Abort, abort, abort. Outside final attack heading, 060 to 120. Re-attack from &lt;IP&gt;.\" "
         "(or \"...you are 18 miles from the target...\" / \"...too close...\")"],
    ], [2.1, 2.3, 3.0]),
    "After \"cleared hot\", Hitman watches for up to 4 minutes. If you swing outside the attack window (with 10&deg; of "
    "slack) before a weapon comes off, he calls \"abort, abort, abort\" on his own. "
    "If Tacview can't see you, Hitman skips every check and clears you hot on your radio calls alone."))

S.append(section("4. Off target and BDA",
    table(["Pilot / when", "Hitman"], [
        ["\"Raider 32, <b>pickle</b>\" / \"<b>rifle</b>\" after cleared hot", "\"Copy pickle, will report BDA.\""],
        ["\"Raider 32, <b>off</b>\"", "\"Copy off. Egress back to &lt;IP&gt;, &lt;egress&gt;. Report IP inbound for re-attack.\""],
        ["<i>~15 s after your last impact</i><br/><b>Good effects</b>",
         "\"BDA. Impact on the aim point. Good effects on target (or Good hits / Shack). Target destroyed, Raider 32. "
         "Thanks for the help. Egress angels 14, report ready for further tasking.\""],
        ["<b>Partial effects</b>", "\"BDA. 2 impacts, closest 30 metres north... Partial effects. Large tanks, 2 of 4 destroyed. "
         "Re-attack approved, report IP inbound from &lt;IP&gt;.\""],
        ["<b>No effects</b>", "\"BDA. Impact 300 metres north of the aim point. No effects observed. "
         "Re-attack, <b>correct south 300</b>. Report IP inbound from &lt;IP&gt;.\""],
        ["<b>Unable to assess</b> (the bridges)", "\"BDA. Impact on the aim point. Unable to assess, smoke and dust over the target. "
         "<b>Say your observed effects.</b>\""],
        ["\"Raider 32, <b>bridge dropped</b>\" (or \"good hits\")<br/>\"Raider 32, <b>no effects</b>\"",
         "\"Copy, good effects. Thanks for the help. Report ready for further tasking.\"<br/>"
         "\"Copy, no effects. Re-attack approved, report IP inbound from &lt;IP&gt;.\""],
        ["\"<b>Request BDA</b>\"", "The same BDA again (before any attack: \"negative BDA, no attack yet\")"],
        ["\"Raider 32, <b>abort</b>\"", "\"Copy abort. Report IP inbound from &lt;IP&gt; for re-attack.\""],
    ], [2.3, 5.1]),
    "\"Off\" is answered once per pass. To re-attack, call \"IP inbound\" again. For a new target, \"ready for further "
    "tasking\" or \"request tasking\" to Hitman, or ask Command. The wording varies between passes; the meaning doesn't."))

S.append(section("Radio checks and tips",
    table(["Item", "Detail"], [
        ["Radio check", "\"Radio check\", \"comm check\", \"comcheck\" and \"how copy\" get \"loud and clear\" on both Hitmen and Command."],
        ["Who is who", "Hitman 1-1 is an Australian voice, Hitman 2-1 an American one. Each only knows his own sector's targets."],
        ["Your callsign", "Say it with its number (\"Raider 3-0-2\"). Anything after the number is not part of the callsign."],
        ["No answer?", "Wait 3-4 s (recognition + voice), then repeat once. Check FM on 61.0 or 62.0. Report callsign, frequency and time on Discord."],
    ], [1.5, 5.9])))

out = os.path.join(HERE, OUT_NAME)
doc = SimpleDocTemplate(out, pagesize=letter, leftMargin=0.55 * inch, rightMargin=0.55 * inch,
                        topMargin=0.28 * inch, bottomMargin=0.25 * inch,
                        title="Hitman JTAC Comms Flow", author="vSFG7")
doc.build(S)
for d in DESKTOP:
    if os.path.isdir(d):
        shutil.copy2(out, os.path.join(d, OUT_NAME))
print("ok", out)
