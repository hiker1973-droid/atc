# -*- coding: utf-8 -*-
"""Nightly roll-up: day summaries kept for good, a monthly and a yearly PDF, 10 days of dailies.

Operator 2026-10-24: "for reporting can you only keep past 10 days but also do a
monthly and yearly summary?". Windows Task Scheduler on the dev box runs this
at 00:25 (task "SkyEye Report Rollup"), after the Paddles (00:05), Health
(00:10) and Pilot (00:15) reports for the day before. It:

1. Writes Reports/_summary/YYYY-MM-DD.json for each day that has raw data
   and no summary yet (and always for the day reported): the day's numbers
   from the Health manifest and logs, every Paddles pass and field landing,
   and DCSServerBot's pilot totals. These small files are never deleted; the
   monthly and yearly reports and the Paddles greenie board read them.
2. Builds Reports/YYYY-MM/YYYY-MM Monthly.pdf (month to date; on the 1st it
   finishes the month before) and Reports/YYYY Yearly.pdf.
3. Deletes the daily PDFs and raw data older than --keep days (10), only for
   days that have a summary. One-off "history" reports are kept.

Run:  python -I docs/reports-rollup/run_rollup.py [--date YYYY-MM-DD] [--keep 10] [--no-prune]
"""
import argparse
import calendar
import datetime as dt
import glob
import json
import os
import re
import shutil
import sys
from collections import Counter, defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
DOCS = os.path.dirname(HERE)
sys.path.insert(0, os.path.join(DOCS, "health-report"))
sys.path.insert(0, os.path.join(DOCS, "paddles-report"))
import build_health_report as health  # noqa: E402
import build_paddles_report as paddles  # noqa: E402

from reportlab.lib import colors  # noqa: E402
from reportlab.lib.pagesizes import letter  # noqa: E402
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet  # noqa: E402
from reportlab.lib.units import inch  # noqa: E402
from reportlab.platypus import KeepTogether, Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle  # noqa: E402

# One Reports folder (operator 2026-10-21), as the nightly reports use.
REPORTS = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Reports"
DAY_RE = re.compile(r"^(\d{4}-\d{2}-\d{2})")

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
GREY = colors.HexColor("#555555")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=GREY, alignment=1, spaceAfter=4)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=6, spaceAfter=2)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.2, leading=8.4)
num = ParagraphStyle("nu", parent=cell, alignment=2)
hdr = ParagraphStyle("hd", parent=cell, fontSize=6.8, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.2, leading=8.5, textColor=GREY, spaceBefore=1)
big = ParagraphStyle("b", parent=ss["Normal"], fontSize=14, leading=16, textColor=NAVY, alignment=1,
                     fontName="Helvetica-Bold")
small = ParagraphStyle("sm", parent=cell, alignment=1, textColor=GREY)

HEALTH_KEYS = ("lines", "starts", "heard", "replied", "misses", "srs", "tacview", "tts", "whisper", "eleven",
               "eacrash", "atis_new", "warn", "error")
PILOT_KEYS = ("hours", "sessions", "sorties", "landings", "traps", "killsAir", "killsGround", "kills", "deaths",
              "crashes", "ejections", "teamkills")


# --- day summaries -------------------------------------------------------------

def summary_path(root, date):
    return os.path.join(root, "_summary", "%s.json" % date)


def raw_days(root):
    """Every day with raw data in _data."""
    days = set()
    for p in glob.glob(os.path.join(root, "_data", "health", "*")):
        m = DAY_RE.match(os.path.basename(p))
        if m and os.path.isdir(p):
            days.add(m.group(1))
    for p in glob.glob(os.path.join(root, "_data", "paddles", "*.log")) + \
            glob.glob(os.path.join(root, "_data", "pilots", "*.json")):
        b = os.path.basename(p)
        m = DAY_RE.match(b)
        if m and "history" not in b:
            days.add(m.group(1))
    return days


def health_summary(root, date):
    mpath = os.path.join(root, "_data", "health", date, "manifest.json")
    if not os.path.exists(mpath):
        return None
    man = json.load(open(mpath, encoding="utf-8"))
    out = {"problems": man.get("problems") or [], "rigs": []}
    for rig in man.get("rigs") or []:
        roles = []
        for e in rig.get("logs") or []:
            m, _, _, _, _ = health.analyse(e, date)
            if not m["lines"] and not e.get("running", True):
                continue  # a field this rig's mission doesn't use
            st, why = health.status(m, e["kind"])
            r = {"label": e["label"], "kind": e["kind"], "status": st, "why": why}
            r.update({k: m[k] for k in HEALTH_KEYS})
            roles.append(r)
        out["rigs"].append({"rig": rig["rig"], "version": rig.get("version", ""),
                            "credits": rig.get("credits") or 0, "roles": roles})
    return out


def paddles_summary(root, date):
    d = os.path.join(root, "_data", "paddles")
    logs = []
    for p in sorted(glob.glob(os.path.join(d, "%s *.log" % date))):
        logs.append((os.path.basename(p)[len(date) + 1:-4], p))
    fields = []
    for p in sorted(glob.glob(os.path.join(d, "field", "%s *.log" % date))):
        fields.append((os.path.basename(p)[len(date) + 1:].split(" ", 1)[0], p))
    if not logs and not fields:
        return None
    day = dt.date.fromisoformat(date)
    ev = paddles.collect(logs, day, day)
    passes = []
    for p in ev["pass"]:
        g, w = paddles.dcs_for(p, ev)
        passes.append({"t": p["t"].isoformat(), "rig": p["rig"], "callsign": p["callsign"], "model": p["model"],
                       "outcome": p["outcome"], "grade": p["grade"], "wire": p["wire"],
                       "points": paddles.grade_points(p), "dcsGrade": g, "dcsWire": w})
    landed = []
    for f in paddles.collect(fields, day, day)["field"] if fields else []:
        landed.append({"t": f["t"].isoformat(), "rig": f["rig"], "callsign": f["callsign"], "model": f["model"],
                       "runway": f["runway"], "outcome": f["outcome"], "grade": f["grade"]})
    return {"passes": passes, "fields": landed, "dcsGrades": len(ev["dcs"]), "misses": len(ev["miss"])}


def pilots_summary(root, date):
    p = os.path.join(root, "_data", "pilots", "%s.json" % date)
    if not os.path.exists(p):
        return None
    day = json.load(open(p, encoding="utf-8")).get("day") or {}
    out = []
    for x in day.get("pilots") or []:
        r = {"name": x.get("name", ""), "trapPoints": x.get("trapPoints") or 0,
             "aircraft": x.get("aircraft") or {}}
        r.update({k: x.get(k) or 0 for k in PILOT_KEYS})
        out.append(r)
    return {"pilots": out, "rigs": [{"rig": r.get("rig"), "configured": r.get("configured"), "err": r.get("err", "")}
                                    for r in day.get("rigs") or []]}


def summarise(root, date):
    s = {"date": date, "made": dt.datetime.now().isoformat(timespec="seconds"),
         "health": health_summary(root, date), "paddles": paddles_summary(root, date),
         "pilots": pilots_summary(root, date)}
    path = summary_path(root, date)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(s, f, indent=1, ensure_ascii=False)
    return s


def load_summaries(root, start, end):
    out = []
    for p in sorted(glob.glob(os.path.join(root, "_summary", "*.json"))):
        d = os.path.basename(p)[:-5]
        if start.isoformat() <= d <= end.isoformat():
            try:
                out.append(json.load(open(p, encoding="utf-8")))
            except ValueError:
                continue
    return out


# --- adding up -----------------------------------------------------------------

def add_up(sums):
    """One period's totals from its day summaries."""
    a = {"days": len(sums), "flown": 0, "pilots": {}, "roles": {}, "passes": [], "fields": [], "credits": 0,
         "heard": 0, "misses": 0, "replied": 0, "red": 0, "amber": 0, "eacrash": 0}
    for s in sums:
        ps = (s.get("pilots") or {}).get("pilots") or []
        if any(p["hours"] > 0 for p in ps):
            a["flown"] += 1
        for p in ps:
            q = a["pilots"].setdefault(p["name"], Counter(days=0))
            q["days"] += 1
            q["points"] += p["traps"] * p["trapPoints"]
            for k in PILOT_KEYS:
                q[k] += p[k]
            a.setdefault("_aircraft", defaultdict(Counter))[p["name"]].update(p.get("aircraft") or {})
        hl = s.get("health") or {}
        for rig in hl.get("rigs") or []:
            a["credits"] += rig.get("credits") or 0
            for r in rig["roles"]:
                q = a["roles"].setdefault((rig["rig"], r["label"]), Counter())
                q["days"] += 1
                q[r["status"]] += 1
                for k in HEALTH_KEYS:
                    q[k] += r.get(k) or 0
                a["heard"] += r.get("heard") or 0
                a["misses"] += r.get("misses") or 0
                a["replied"] += r.get("replied") or 0
                a["eacrash"] += r.get("eacrash") or 0
                if r["status"] in ("red", "amber"):
                    a[r["status"]] += 1
        pd = s.get("paddles") or {}
        a["passes"] += pd.get("passes") or []
        a["fields"] += pd.get("fields") or []
    return a


# --- drawing -------------------------------------------------------------------

def esc(s):
    return str(s if s is not None else "").replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def zero(v, fmt="{:g}"):
    return fmt.format(v) if v else '<font color="#aaaaaa">0</font>'


def table(head, rows, widths, left=(0,)):
    data = [[Paragraph(x, hdr) for x in head]]
    for r in rows:
        data.append([x if not isinstance(x, str) else Paragraph(x, cell if i in left else num)
                     for i, x in enumerate(r)])
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("TOPPADDING", (0, 0), (-1, -1), 1.5), ("BOTTOMPADDING", (0, 0), (-1, -1), 1.5),
        ("LEFTPADDING", (0, 0), (-1, -1), 2.5), ("RIGHTPADDING", (0, 0), (-1, -1), 2.5),
    ]))
    return t


def tiles(items):
    w = 7.5 / len(items)
    t = Table([[Paragraph(str(v), big) for v, _ in items], [Paragraph(l, small) for _, l in items]],
              colWidths=[w * inch] * len(items))
    t.setStyle(TableStyle([("BOX", (0, 0), (-1, -1), 0.6, NAVY), ("INNERGRID", (0, 0), (-1, -1), 0.3, SHADE),
                           ("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#f5f7fa")),
                           ("TOPPADDING", (0, 0), (-1, -1), 2), ("BOTTOMPADDING", (0, 0), (-1, -1), 2)]))
    return t


def pct(n, d):
    # GCI also "misses" pilots talking to each other, so it can pass 100%.
    return "%d%%" % min(100, round(100 * n / d)) if d else "-"


def headline(a):
    hours = sum(p["hours"] for p in a["pilots"].values())
    flew = sum(1 for p in a["pilots"].values() if p["hours"] > 0)
    traps = sum(1 for p in a["passes"] if p["outcome"] == "trap")
    return tiles([(flew, "pilots flew"), ("%.0f" % hours, "hours in slot"),
                  (sum(p["sorties"] for p in a["pilots"].values()), "sorties"),
                  (traps, "Paddles traps"), (sum(p["kills"] for p in a["pilots"].values()), "kills"),
                  (a["heard"], "pilot calls heard"), (pct(a["misses"], a["heard"]), "not understood"),
                  ("{:,}".format(a["credits"]) if a["credits"] else "-", "ElevenLabs credits")])


def period_row(label, a):
    traps = sum(1 for p in a["passes"] if p["outcome"] == "trap")
    return [label, zero(sum(1 for p in a["pilots"].values() if p["hours"] > 0)),
            zero(sum(p["hours"] for p in a["pilots"].values()), "{:.1f}"),
            zero(sum(p["sorties"] for p in a["pilots"].values())), zero(traps),
            zero(sum(p["kills"] for p in a["pilots"].values())), zero(a["heard"]), pct(a["misses"], a["heard"]),
            zero(a["red"]), zero(a["amber"]), zero(a["credits"], "{:,}")]


PERIOD_HEAD = ["Pilots", "Hours", "Sorties", "Traps", "Kills", "Calls heard", "Not underst.", "Red roles",
               "Amber roles", "ElevenLabs"]
PERIOD_W = [0.5, 0.5, 0.55, 0.5, 0.5, 0.65, 0.65, 0.55, 0.6, 0.75]


def pilots_section(a, top=40):
    ac = a.get("_aircraft") or {}
    rows = []
    for name, p in sorted(a["pilots"].items(), key=lambda kv: -kv[1]["hours"])[:top]:
        if p["hours"] <= 0 and not p["sorties"]:
            continue
        main = ", ".join(k for k, _ in sorted(ac.get(name, {}).items(), key=lambda kv: -kv[1])
                         if k != "?")[:40]
        rows.append([esc(name), zero(p["days"]), "%.1f" % p["hours"], zero(p["sorties"]), zero(p["landings"]),
                     zero(p["traps"]) + ((" (%.1f)" % (p["points"] / p["traps"])) if p["traps"] else ""),
                     zero(p["killsAir"]), zero(p["killsGround"]), zero(p["deaths"]), zero(p["crashes"]),
                     zero(p["ejections"]), esc(main)])
    out = [Paragraph("Pilots (DCSServerBot)", h)]
    if not rows:
        return out + [Paragraph("Nobody flew.", note)]
    return out + [table(["Pilot", "Days", "Hours", "Sorties", "Landings", "Traps (avg)", "Air kills", "Gnd kills",
                         "Deaths", "Crashes", "Ejects", "Aircraft"], rows,
                        [1.55, 0.38, 0.45, 0.48, 0.55, 0.65, 0.48, 0.5, 0.45, 0.5, 0.42, 1.09], left=(0, 11))]


def board_passes(a):
    out = []
    for p in a["passes"]:
        q = dict(p)
        q["t"] = dt.datetime.fromisoformat(p["t"])
        out.append(q)
    return out


def paddles_table(a):
    """Carrier passes by pilot - the year's greenie board in numbers."""
    by = defaultdict(list)
    for p in a["passes"]:
        if p["outcome"] in ("trap", "bolter", "wave off"):
            by[p["callsign"]].append(p)
    out = [Paragraph("Carrier passes (Paddles)", h)]
    if not by:
        return out + [Paragraph("No graded passes.", note)]
    rows = []
    for cs, ps in by.items():
        pts = [p["points"] for p in ps if p.get("points") is not None]
        g = Counter(p["grade"] for p in ps)
        rows.append((-(sum(pts) / len(pts)) if pts else 0, cs, ps, pts, g))
    rows.sort(key=lambda r: (r[0], -len(r[2])))
    return out + [table(["Pilot", "Avg", "Passes", "Traps", "Bolters", "Wave-offs", "_OK_", "OK", "(OK)", "---",
                         "3 wires"],
                        [[esc(cs), "%.2f" % -avg if pts else "-", str(len(ps)),
                          zero(sum(1 for p in ps if p["outcome"] == "trap")),
                          zero(sum(1 for p in ps if p["outcome"] == "bolter")),
                          zero(sum(1 for p in ps if p["outcome"] == "wave off")),
                          zero(g["_OK_"]), zero(g["OK"]), zero(g["(OK)"]), zero(g["---"] + g["--"]),
                          zero(sum(1 for p in ps if p["outcome"] == "trap" and p.get("wire") == 3))]
                         for avg, cs, ps, pts, g in rows],
                        [1.6, 0.5, 0.55, 0.5, 0.55, 0.65, 0.5, 0.5, 0.5, 0.5, 0.6])]


def fields_line(a):
    if not a["fields"]:
        return []
    g = Counter(f["grade"] for f in a["fields"])
    return [Paragraph("<b>Senaki field landings graded:</b> %d (%s)." % (
        len(a["fields"]), ", ".join("%s %d" % (k, n) for k, n in g.most_common())), note)]


def health_section(a):
    rows = []
    for (rig, label), q in sorted(a["roles"].items(), key=lambda kv: (kv[0][0], kv[0][1])):
        rows.append([esc(rig), "<b>%s</b>" % esc(label), str(q["days"]), zero(q["green"]), zero(q["amber"]),
                     zero(q["red"]), zero(q["silent"]), zero(q["starts"]), zero(q["heard"]), zero(q["replied"]),
                     pct(q["misses"], q["heard"]), zero(q["srs"]), zero(q["tacview"]), zero(q["tts"] + q["whisper"]),
                     zero(q["eacrash"]), zero(q["error"])])
    out = [Paragraph("Roles (Health report)", h)]
    if not rows:
        return out + [Paragraph("No health data.", note)]
    return out + [table(["Rig", "Role", "Days", "Green", "Amber", "Red", "Silent", "Starts", "Heard", "Replied",
                         "Not underst.", "SRS", "Tacview", "TTS/STT", "ATIS EA", "Errors"], rows,
                        [0.55, 1.3, 0.33, 0.38, 0.4, 0.3, 0.38, 0.4, 0.4, 0.45, 0.5, 0.32, 0.42, 0.45, 0.42, 0.36],
                        left=(0, 1)),
                  Paragraph("Days, green, amber, red and silent (no log lines) count the role's daily status from the nightly Health "
                            "report. Heard / replied / not understood are pilot calls. SRS and Tacview are link "
                            "drops, TTS/STT speech or Whisper failures, ATIS EA the ExternalAudio crashes (resent, "
                            "not flagged).", note)]


def coverage(sums, start, end):
    have = {s["date"] for s in sums}
    days = [(start + dt.timedelta(days=i)).isoformat() for i in range((end - start).days + 1)]
    gone = [d for d in days if d not in have]
    txt = "Days with data: %d of %d." % (len(days) - len(gone), len(days))
    if gone and len(gone) <= 12:
        txt += " None for " + ", ".join(dt.date.fromisoformat(d).strftime("%d %b") for d in gone) + "."
    elif gone:
        txt += " Reports started %s." % dt.date.fromisoformat(min(have)).strftime("%d %b %Y") if have else ""
    return Paragraph(txt, note)


def doc(path, ttl):
    return SimpleDocTemplate(path, pagesize=letter, leftMargin=0.5 * inch, rightMargin=0.5 * inch,
                             topMargin=0.35 * inch, bottomMargin=0.35 * inch, title=ttl, author="vSFG7")


def build_monthly(root, month_start, end, out_path):
    sums = load_summaries(root, month_start, end)
    a = add_up(sums)
    last = calendar.monthrange(month_start.year, month_start.month)[1]
    full = end.day == last
    ttl = "vSFG-7 Monthly Report %s" % month_start.strftime("%B %Y")
    S = [Paragraph("vSFG-7 Monthly Report", title),
         Paragraph("%s &middot; %s &middot; Day / Night / Foothold" % (
             month_start.strftime("%B %Y"), "whole month" if full else "1 - %d %s (month to date)" % (
                 end.day, month_start.strftime("%b"))), sub),
         headline(a), coverage(sums, month_start, end)]
    rows = []
    for s in sums:
        rows.append(period_row(dt.date.fromisoformat(s["date"]).strftime("%a %d"), add_up([s])))
    S.append(Paragraph("By day", h))
    S.append(table(["Day"] + PERIOD_HEAD, rows + [["<b>Month</b>"] + period_row("", a)[1:]] if rows else [],
                   [0.75] + PERIOD_W))
    S += pilots_section(a)
    board = paddles.greenie_board(board_passes(a), month_start, end)
    S += board or [Paragraph("Greenie board", h), Paragraph("No graded passes this month.", note)]
    S += fields_line(a)
    S += health_section(a)
    S.append(Paragraph("Built from the nightly reports' day summaries (Reports\\_summary). Daily PDFs are kept "
                       "10 days.", note))
    doc(out_path, ttl).build(S)


def build_yearly(root, year, end, out_path):
    start = dt.date(year, 1, 1)
    sums = load_summaries(root, start, end)
    a = add_up(sums)
    S = [Paragraph("vSFG-7 Yearly Report", title),
         Paragraph("%d &middot; %s &middot; Day / Night / Foothold" % (
             year, "whole year" if end == dt.date(year, 12, 31) else "to %s" % end.strftime("%d %b")), sub),
         headline(a)]
    first = min((s["date"] for s in sums), default=None)
    if first:
        S.append(Paragraph("Reports from %s: %d days with data." % (
            dt.date.fromisoformat(first).strftime("%d %b %Y"), len(sums)), note))
    rows = []
    for m in range(1, 13):
        ms = [s for s in sums if s["date"][5:7] == "%02d" % m]
        if ms:
            rows.append(period_row(dt.date(year, m, 1).strftime("%B"), add_up(ms)))
    S.append(Paragraph("By month", h))
    S.append(table(["Month"] + PERIOD_HEAD, rows + [["<b>Year</b>"] + period_row("", a)[1:]] if rows else [],
                   [0.75] + PERIOD_W))
    S += pilots_section(a, top=60)
    S += paddles_table(a)
    S += fields_line(a)
    S += health_section(a)
    S.append(Paragraph("Built from the nightly reports' day summaries (Reports\\_summary). Monthly reports are in "
                       "each month's folder.", note))
    doc(out_path, "vSFG-7 Yearly Report %d" % year).build(S)


# --- keeping 10 days -----------------------------------------------------------

def prune(root, cutoff, dry=False):
    """Delete daily PDFs and raw data from before cutoff, for days with a summary."""
    gone = []

    def has(d):
        return os.path.exists(summary_path(root, d))

    for p in glob.glob(os.path.join(root, "????-??", "????-??-?? *.pdf")):
        b = os.path.basename(p)
        d = b[:10]
        if d < cutoff and "history" not in b.lower() and has(d):
            gone.append(p)
    for p in glob.glob(os.path.join(root, "_data", "health", "????-??-??")):
        d = os.path.basename(p)
        if d < cutoff and has(d):
            gone.append(p)
    for pat in (("paddles", "????-??-?? *"), ("paddles", "field", "????-??-?? *"), ("pilots", "????-??-??.json")):
        for p in glob.glob(os.path.join(root, "_data", *pat)):
            b = os.path.basename(p)
            if b[:10] < cutoff and "history" not in b and has(b[:10]):
                gone.append(p)
    for p in gone:
        if dry:
            continue
        if os.path.isdir(p):
            shutil.rmtree(p)
        else:
            os.remove(p)
    return gone


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", default=(dt.date.today() - dt.timedelta(days=1)).isoformat(),
                    help="the day just reported (default yesterday)")
    ap.add_argument("--out", default=REPORTS)
    ap.add_argument("--keep", type=int, default=10, help="days of daily PDFs and raw data to keep")
    ap.add_argument("--no-prune", action="store_true")
    ap.add_argument("--dry-run", action="store_true", help="list what would be deleted; delete nothing")
    a = ap.parse_args()
    day = dt.date.fromisoformat(a.date)
    root = a.out

    made = []
    for d in sorted(raw_days(root)):
        if d == a.date or not os.path.exists(summary_path(root, d)):
            summarise(root, d)
            made.append(d)
    print("summaries:", ", ".join(made) or "none new")

    month = day.replace(day=1)
    mdir = os.path.join(root, day.strftime("%Y-%m"))
    os.makedirs(mdir, exist_ok=True)
    mpdf = os.path.join(mdir, "%s Monthly.pdf" % day.strftime("%Y-%m"))
    build_monthly(root, month, day, mpdf)
    ypdf = os.path.join(root, "%d Yearly.pdf" % day.year)
    build_yearly(root, day.year, day, ypdf)
    print("wrote", mpdf)
    print("wrote", ypdf)

    if not a.no_prune:
        cutoff = (day - dt.timedelta(days=a.keep - 1)).isoformat()
        gone = prune(root, cutoff, a.dry_run)
        print("%s %d old daily files/folders (before %s)%s" % (
            "would delete" if a.dry_run else "deleted", len(gone), cutoff,
            "".join("\n  " + os.path.relpath(p, root) for p in gone) if a.dry_run else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
