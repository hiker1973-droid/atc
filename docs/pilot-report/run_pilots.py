# -*- coding: utf-8 -*-
"""Nightly pilot report: who flew yesterday and what they did, plus the last 7 days.

Windows Task Scheduler on the dev box runs this at 00:15 for the day before
(task "SkyEye Pilot Report"; operator 2026-10-21: "Can you run a pilot report
PDF nightly as well?"). The numbers are DCSServerBot's (slot time, takeoffs,
landings, kills, deaths, crashes, ejections, greenieboard traps), merged
across the rigs by the launcher's /api/pilots - the same data as the
dashboard's Pilots tab. The JSON it was built from is kept beside the PDF.

Run:  python -I docs/pilot-report/run_pilots.py [--date YYYY-MM-DD] [--launcher URL]
"""
import argparse
import datetime as dt
import json
import os
import urllib.request

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter, landscape
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

OUT = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation\Pilot Reports"
LAUNCHER = "http://127.0.0.1:7000"

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
GREY = colors.HexColor("#555555")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=GREY, alignment=1, spaceAfter=5)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=11, textColor=NAVY, spaceBefore=8, spaceAfter=3)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.6, leading=9)
num = ParagraphStyle("nu", parent=cell, alignment=2)
hdr = ParagraphStyle("hd", parent=cell, fontSize=7, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.6, leading=9, textColor=GREY, spaceBefore=2)
big = ParagraphStyle("b", parent=ss["Normal"], fontSize=16, leading=18, textColor=NAVY, alignment=1,
                     fontName="Helvetica-Bold")
small = ParagraphStyle("sm", parent=cell, alignment=1, textColor=GREY)


def fetch(base, query):
    with urllib.request.urlopen(base + "/api/pilots?" + query, timeout=90) as r:
        return json.loads(r.read())


def esc(s):
    return str(s or "").replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def table(head, rows, widths, left=(0,)):
    """Numbers right-aligned; the columns in left (text) left-aligned."""
    data = [[Paragraph(x, hdr) for x in head]]
    for r in rows:
        data.append([Paragraph(str(x), cell if i in left else num) for i, x in enumerate(r)])
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("TOPPADDING", (0, 0), (-1, -1), 2), ("BOTTOMPADDING", (0, 0), (-1, -1), 2),
    ]))
    return t


def tiles(items):
    row1 = [Paragraph(str(v), big) for v, _ in items]
    row2 = [Paragraph(l, small) for _, l in items]
    t = Table([row1, row2], colWidths=[10.0 / len(items) * inch] * len(items))
    t.setStyle(TableStyle([("BOX", (0, 0), (-1, -1), 0.5, colors.HexColor("#9aa7b8")),
                           ("INNERGRID", (0, 0), (-1, -1), 0.25, colors.HexColor("#d0d7e2")),
                           ("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#f5f7fa"))]))
    return t


def zero(v, fmt="{:g}"):
    return fmt.format(v) if v else '<font color="#aaaaaa">0</font>'


def top(m, n=2):
    return ", ".join(k for k, _ in sorted((m or {}).items(), key=lambda kv: -kv[1])[:n])


def local(ts):
    if not ts or ts.startswith("0001"):
        return ""
    t = dt.datetime.fromisoformat(ts.replace("Z", "+00:00"))
    return t.astimezone().strftime("%H:%M")


def build(day, rep, week, out_pdf):
    pilots = rep.get("pilots") or []
    S = [Paragraph("vSFG-7 Pilot Report", title),
         Paragraph("%s &middot; Day / Night / Foothold &middot; DCSServerBot statistics via the SkyEye launcher"
                   % day.strftime("%a %d %b %Y"), sub)]
    rigs = rep.get("rigs") or []
    linked = [r for r in rigs if r.get("configured")]
    S.append(Paragraph("Data: " + " &middot; ".join(
        ("%s %s" % (esc(r.get("rig")), "OK" if r.get("configured") and not r.get("err")
                    else ("error - " + esc(r.get("err"))) if r.get("configured") else "not linked"))
        for r in rigs), note))
    if not linked:
        S.append(Paragraph("<b>No rig is linked to DCSServerBot yet.</b> Set SKYEYE_DCSSB_DB on each server "
                           "(the Pilots tab shows how) and press Update; tomorrow's report will fill in.", cell))

    tot = lambda k: sum(p.get(k) or 0 for p in pilots)
    S.append(Spacer(1, 4))
    S.append(tiles([(len(pilots), "pilots flew"), ("%.1f" % tot("hours"), "hours in slot"),
                    (tot("sorties"), "sorties"), (tot("landings"), "landings"), (tot("traps"), "carrier traps"),
                    (tot("kills"), "kills"), (tot("deaths"), "deaths")]))

    S.append(Paragraph("Yesterday, by pilot", h))
    if pilots:
        rows = [[esc(p["name"]), "%.1f" % p["hours"], zero(p["sorties"]), zero(p["landings"]),
                 zero(p["traps"]) + ((" (%.1f)" % p["trapPoints"]) if p.get("traps") else ""),
                 zero(p.get("killsAir")), zero(p.get("killsGround")), zero(p["deaths"]), zero(p["crashes"]),
                 zero(p["ejections"]), esc(top(p.get("aircraft"))), esc(top(p.get("servers"), 3))]
                for p in sorted(pilots, key=lambda p: -p["hours"])]
        S.append(table(["Pilot", "Hours", "Sorties", "Landings", "Traps (avg)", "Air kills", "Gnd kills",
                        "Deaths", "Crashes", "Ejects", "Aircraft", "Servers"], rows,
                       [1.9, 0.5, 0.55, 0.6, 0.75, 0.55, 0.6, 0.5, 0.62, 0.5, 1.6, 1.33], left=(0, 10, 11)))
    else:
        S.append(Paragraph("Nobody flew.", cell))

    traps = sorted((t for p in pilots for t in (p.get("recentTraps") or [])), key=lambda t: t.get("time") or "")
    if traps:
        S.append(Paragraph("Carrier traps (greenieboard)", h))
        S.append(table(["Time", "Pilot", "Aircraft", "Grade", "Wire", "Points", "Day / night", "Server"],
                       [[local(t.get("time")), esc(t["name"]), esc(t["aircraft"]), esc(t["grade"]),
                         t.get("wire") or "-", "%g" % t.get("points", 0), "night" if t.get("night") else "day",
                         esc(t.get("server"))] for t in traps],
                       [0.6, 2.0, 1.3, 0.7, 0.5, 0.6, 0.8, 1.6], left=(0, 1, 2, 3, 6, 7)))

    wp = sorted(week.get("pilots") or [], key=lambda p: -p["hours"])[:25]
    S.append(Paragraph("Last 7 days - top %d by hours" % len(wp), h))
    if wp:
        S.append(table(["Pilot", "Hours", "Slots", "Sorties", "Landings", "Traps (avg)", "Kills", "Deaths",
                        "Main aircraft", "Last seen"],
                       [[esc(p["name"]), "%.1f" % p["hours"], p["sessions"], zero(p["sorties"]), zero(p["landings"]),
                         zero(p["traps"]) + ((" (%.1f)" % p["trapPoints"]) if p.get("traps") else ""),
                         zero(p["kills"]), zero(p["deaths"]), esc(top(p.get("aircraft"), 1)),
                         dt.datetime.fromisoformat(p["lastSeen"].replace("Z", "+00:00")).astimezone().strftime("%a %d %b")
                         if p.get("lastSeen") and not p["lastSeen"].startswith("0001") else ""]
                        for p in wp],
                       [2.2, 0.55, 0.5, 0.6, 0.65, 0.8, 0.5, 0.55, 1.6, 0.9], left=(0, 8, 9)))
    else:
        S.append(Paragraph("Nobody flew this week.", cell))
    S.append(Paragraph("Hours are time in a slot (DCSServerBot hop on to hop off), not airborne time. Traps and "
                       "points are DCSServerBot's greenieboard (DCS's LSO); SkyEye Paddles grades are in the "
                       "Paddles report.", note))

    doc = SimpleDocTemplate(out_pdf, pagesize=landscape(letter), leftMargin=0.45 * inch, rightMargin=0.45 * inch,
                            topMargin=0.45 * inch, bottomMargin=0.45 * inch, title="vSFG-7 Pilot Report %s" % day)
    doc.build(S)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", help="day to report (default yesterday)")
    ap.add_argument("--launcher", default=LAUNCHER)
    ap.add_argument("--out", default=OUT)
    a = ap.parse_args()
    day = dt.date.fromisoformat(a.date) if a.date else dt.date.today() - dt.timedelta(days=1)
    rep = fetch(a.launcher, "date=" + day.isoformat())
    week = fetch(a.launcher, "days=7")
    data = os.path.join(a.out, "data")
    os.makedirs(data, exist_ok=True)
    with open(os.path.join(data, "%s.json" % day), "w", encoding="utf-8") as f:
        json.dump({"day": rep, "week": week}, f)
    out_pdf = os.path.join(a.out, "Pilot Report %s.pdf" % day)
    build(day, rep, week, out_pdf)
    print("wrote", out_pdf)


if __name__ == "__main__":
    main()
