# -*- coding: utf-8 -*-
"""Paddles (LSO) Daily Report - traps, grades and Paddles' own health (ReportLab).

Reads Paddles' log lines (atc-lso.log, JSON per line; a "Role :: {json}" prefix is
skipped) from one or more rigs and builds a PDF for a day: every pass with Paddles'
grade and wire, DCS's own grade beside it, the groove (glideslope and lineup at
X / IM / IC / AR) as a small plot, a pilot table, and what Paddles heard and said.

The pass line is pkg/lso Tracker.finish(); the rest are the Msg()s in
cmd/atc/lso.go and cmd/atc/lso_grpc.go.

Made 2026-10-18 (operator: "run a daily paddles report on traps and data
collected"), in the style of docs/paddles-flow.

Run:  python -I docs/paddles-report/build_paddles_report.py --date 2026-10-18 \
          --log Day=atc-lso-day.log --log Night=atc-lso-night.log [--out DIR]
      --from / --to instead of --date for a range; --sample marks it a sample.
"""
import argparse
import datetime as dt
import json
import os
import re
import shutil
from collections import OrderedDict, defaultdict

from reportlab.graphics.shapes import Drawing, Line, PolyLine, String
from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import KeepTogether, Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"]

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
ORANGE = colors.HexColor("#d9822b")
GREEN = colors.HexColor("#2e7d32")
RED = colors.HexColor("#b3261e")
GREY = colors.HexColor("#555555")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=GREY, alignment=1, spaceAfter=3)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=4, spaceAfter=1.5)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.4, leading=8.5)
hdr = ParagraphStyle("hd", parent=cell, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.4, leading=8.6, spaceBefore=1)
big = ParagraphStyle("b", parent=ss["Normal"], fontSize=15, leading=17, textColor=NAVY, alignment=1,
                     fontName="Helvetica-Bold")
small = ParagraphStyle("sm", parent=cell, alignment=1, textColor=GREY)

# LSO grade points (the usual squadron scale). A foul-deck wave-off is not the
# pilot's: no points.
POINTS = OrderedDict([("_OK_", 5.0), ("OK", 4.0), ("(OK)", 3.0), ("B", 2.5), ("--", 2.0), ("WO", 1.0),
                      ("C", 0.0)])
SEGMENTS = ["X", "IM", "IC", "AR"]


def table(head, rows, widths, style=()):
    data = [[Paragraph(x, hdr) for x in head]] + [[x if not isinstance(x, str) else Paragraph(x, cell)
                                                   for x in r] for r in rows]
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("TOPPADDING", (0, 0), (-1, -1), 1.6), ("BOTTOMPADDING", (0, 0), (-1, -1), 1.6),
    ] + list(style)))
    return t


# --- reading -----------------------------------------------------------------

def read_lines(path):
    with open(path, encoding="utf-8", errors="replace") as f:
        for line in f:
            i = line.find("{")
            if i < 0:
                continue
            try:
                yield json.loads(line[i:])
            except ValueError:
                continue


PASS_RE = re.compile(r"^(?P<cs>.+?) \((?P<model>[^)]*)\): (?P<outcome>[a-z ]+?)(?=[,;]|$)")
SEG_RE = re.compile(r"\b(X|IM|IC|AR) GS ([+-]?\d+\.\d)° LU ([+-]?\d+\.\d)°(?: AoA (\d+\.\d))?")


def parse_pass(s):
    m = PASS_RE.match(s)
    if not m:
        return None
    p = {"callsign": m["cs"], "model": m["model"], "outcome": m["outcome"].strip(), "grade": "",
         "comments": "", "wire": None, "touch": None, "segs": {}, "said": []}
    g = re.search(r", grade (.+?)(?:, hook touched|; \d+ samples|$)", s)
    if g:
        gr = g.group(1)
        if gr.startswith("WO"):
            p["grade"] = "WO (foul deck)" if "foul deck" in gr else "WO"
        else:
            p["grade"], _, p["comments"] = gr.partition(": ")
            p["comments"] = re.sub(r"\s*\d wire$", "", p["comments"]).strip()
    if not p["grade"] and p["outcome"] == "wave off":
        p["grade"] = "WO"
    w = re.search(r"est\. (\d) wire", s)
    if w:
        p["wire"] = int(w.group(1))
    t = re.search(r"hook touched ([+-]\d+) m from the aim point, ([+-]\d+) m right", s)
    if t:
        p["touch"] = (int(t.group(1)), int(t.group(2)))
    for seg, gs, lu, aoa in SEG_RE.findall(s):
        p["segs"][seg] = (float(gs), float(lu), float(aoa) if aoa else None)
    said = s.split("; said: ", 1)
    if len(said) == 2:
        p["said"] = [c.strip() for c in said[1].split(", ") if c.strip()]
    return p


def collect(logs, start, end):
    """logs: [(rig, path)]. Returns the day's events, de-duplicated."""
    seen = set()
    ev = defaultdict(list)
    for rig, path in logs:
        for o in read_lines(path):
            ts = o.get("time", "")
            try:
                t = dt.datetime.fromisoformat(ts)
            except ValueError:
                continue
            if not (start <= t.date() <= end):
                continue
            key = (rig, ts, o.get("message"), o.get("text"), o.get("pass"), o.get("call"))
            if key in seen:
                continue
            seen.add(key)
            msg = o.get("message", "")
            o["_rig"], o["_t"] = rig, t
            if msg == "Paddles: pass":
                p = parse_pass(o.get("pass", ""))
                if p:
                    p.update(rig=rig, t=t)
                    ev["pass"].append(p)
            elif msg == "Paddles: DCS LSO grade":
                ev["dcs"].append(o)
            elif msg == "Paddles: vs DCS LSO":
                ev["vs"].append(o)
            elif msg == "Paddles: ball call":
                ev["ball"].append(o)
            elif msg == "Paddles: deck call":
                ev["deck"].append(o)
            elif msg == "Paddles: groove call":
                ev["groove"].append(o)
            elif msg in ("Command TX", "Paddles TX") and "In the groove at three-quarter mile" in o.get("text", ""):
                ev["abeam"].append(o)
            elif msg == "Command intent miss" and o.get("channel") == "Paddles":
                ev["miss"].append(o)
            elif msg in ("Paddles: Tacview disconnected, reconnecting in 5s", "Paddles: DCS-gRPC events lost, reconnecting",
                         "Paddles: call dropped, too late", "Paddles: TTS failed", "Paddles: TX queue full"):
                ev["health"].append(o)
    for k in ev:
        ev[k].sort(key=lambda o: o["t"] if k == "pass" else o["_t"])
    return ev


# --- drawing -----------------------------------------------------------------

def groove_plot(segs, w=1.55 * inch, hgt=0.52 * inch, lim=3.0):
    """Glideslope (navy) and lineup (orange) error at X, IM, IC, AR; +/-1 deg dotted."""
    d = Drawing(w, hgt)
    pad_l, pad_r, pad_y = 12, 4, 4
    xs = [pad_l + i * (w - pad_l - pad_r) / 3 for i in range(4)]

    def y(v):
        v = max(-lim, min(lim, v))
        return hgt / 2 + v / lim * (hgt / 2 - pad_y)

    d.add(Line(pad_l, y(0), w - pad_r, y(0), strokeColor=colors.HexColor("#9aa7b8"), strokeWidth=0.5))
    for v in (1, -1):
        d.add(Line(pad_l, y(v), w - pad_r, y(v), strokeColor=colors.HexColor("#c8d0db"), strokeWidth=0.4,
                   strokeDashArray=[1, 2]))
    d.add(String(1, y(1) - 2, "+1", fontSize=4.5, fillColor=GREY))
    d.add(String(1, y(-1) - 2, "-1", fontSize=4.5, fillColor=GREY))
    for i, s in enumerate(SEGMENTS):
        d.add(String(xs[i] - 3, 0, s, fontSize=4.5, fillColor=GREY))
    for idx, color in ((0, NAVY), (1, ORANGE)):
        pts = []
        for i, s in enumerate(SEGMENTS):
            if s in segs:
                pts += [xs[i], y(segs[s][idx])]
        if len(pts) >= 4:
            d.add(PolyLine(pts, strokeColor=color, strokeWidth=1.1))
    return d


# --- report ------------------------------------------------------------------

def outcome_text(p):
    o = p["outcome"]
    if o == "trap":
        return "<font color='#2e7d32'><b>Trap</b></font>"
    if o == "bolter":
        return "<font color='#d9822b'><b>Bolter</b></font>"
    if o == "wave off":
        return "<font color='#b3261e'><b>Wave off</b></font>"
    return o.capitalize()


def grade_points(p):
    g = p["grade"]
    if g == "WO (foul deck)":
        return None
    return POINTS.get(g)


def dcs_for(p, ev):
    """DCS's grade for this pass: the vs-DCS line, else a DCS grade for the same
    callsign within 90 s."""
    for v in ev["vs"]:
        if v.get("callsign") == p["callsign"] and abs((v["_t"] - p["t"]).total_seconds()) < 120:
            return v.get("dcsGrade", ""), v.get("dcsWire")
    for d in ev["dcs"]:
        if d.get("callsign") == p["callsign"] and abs((d["_t"] - p["t"]).total_seconds()) < 90:
            return d.get("grade", ""), d.get("wire")
    return None, None


def build(ev, start, end, rigs, out_path, sample=False, problems=()):
    passes = ev["pass"]
    real = [p for p in passes if p["outcome"] in ("trap", "bolter", "wave off")]
    traps = [p for p in real if p["outcome"] == "trap"]
    bolters = [p for p in real if p["outcome"] == "bolter"]
    wos = [p for p in real if p["outcome"] == "wave off"]
    pts = [x for x in (grade_points(p) for p in real) if x is not None]
    wires = [p["wire"] for p in traps if p["wire"]]
    pilots = sorted({p["callsign"] for p in passes})
    day = start.strftime("%d %b %Y") if start == end else "%s - %s" % (start.strftime("%d %b"), end.strftime("%d %b %Y"))

    S = [Paragraph("Paddles Daily Report%s" % ("  <font size=10 color='#b3261e'>SAMPLE</font>" if sample else ""), title),
         Paragraph("vSFG7 SkyEye &middot; %s &middot; %s &middot; CVN-72 Abe &middot; Paddles 128.300 AM" %
                   (day, " / ".join(rigs)), sub)]

    def tile(v, label):
        return [Paragraph(v, big), Paragraph(label, small)]

    boarding = "%d%%" % round(100 * len(traps) / (len(traps) + len(bolters))) if traps or bolters else "-"
    tiles = [tile(str(len(real)), "passes"), tile(str(len(traps)), "traps"), tile(str(len(bolters)), "bolters"),
             tile(str(len(wos)), "wave-offs"), tile(boarding, "boarding rate"),
             tile("%.1f" % (sum(pts) / len(pts)) if pts else "-", "avg grade pts"),
             tile("%.1f" % (sum(wires) / len(wires)) if wires else "-", "avg wire"),
             tile(str(len(pilots)), "pilots")]
    tt = Table([[x[0] for x in tiles], [x[1] for x in tiles]], colWidths=[0.925 * inch] * 8)
    tt.setStyle(TableStyle([("BOX", (0, 0), (-1, -1), 0.6, NAVY), ("INNERGRID", (0, 0), (-1, -1), 0.3, SHADE),
                            ("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#f5f7fa")),
                            ("TOPPADDING", (0, 0), (-1, -1), 2), ("BOTTOMPADDING", (0, 0), (-1, -1), 2)]))
    S.append(tt)
    S.append(Paragraph("Grade points: _OK_ 5 &middot; OK 4 &middot; (OK) 3 &middot; B 2.5 &middot; -- 2 &middot; "
                       "WO 1 &middot; C 0. A foul-deck wave-off is not scored. Boarding rate = traps / (traps + "
                       "bolters).", note))

    # Passes
    rows = []
    for p in passes:
        g = p["grade"] or "-"
        if p["comments"]:
            g += "<br/><font size=6.4 color='#555555'>%s</font>" % p["comments"]
        dg, dw = dcs_for(p, ev)
        dcs = "%s%s" % (dg, " &middot; %d wire" % dw if dw else "") if dg else "<font color='#888888'>none</font>"
        touch = "%+d m, %+d m R" % p["touch"] if p["touch"] else "-"
        said = ", ".join(re.sub(r" \d+\.\ds$", "", c) for c in p["said"]) or "-"
        rows.append(["%s<br/><font size=6.4 color='#555555'>%s</font>" % (p["t"].strftime("%H:%M"), p["rig"]),
                     "<b>%s</b><br/><font size=6.4 color='#555555'>%s</font>" % (p["callsign"], p["model"]),
                     outcome_text(p), g, str(p["wire"]) if p["wire"] else "-", dcs, touch,
                     groove_plot(p["segs"]), "<font size=6.6>%s</font>" % said])
    if rows:
        S.append(KeepTogether([Paragraph("Passes", h), table(
            ["Time", "Pilot", "Result", "Paddles grade", "Wire", "DCS LSO", "Hook down", "Groove: GS / lineup", "Paddles said"],
            rows, [0.5, 0.95, 0.55, 1.05, 0.42, 0.72, 0.74, 1.65, 0.82])]))
        S.append(Paragraph("<b>Groove plot:</b> <font color='#1f3350'><b>glideslope</b></font> and "
                           "<font color='#d9822b'><b>lineup</b></font> error in degrees at X (start), IM (middle), "
                           "IC (in close) and AR (at the ramp); above the centre line is high / right. Dotted lines "
                           "are &plusmn;1&deg;. <b>Hook down</b> is metres past the aim point and right of the "
                           "landing line. Wire is Paddles' estimate; DCS LSO is DCS's own grade from DCS-gRPC.", note))
    else:
        S.append(Paragraph("Passes", h))
        S.append(Paragraph("No passes flown.", note))

    # Pilots
    by = defaultdict(list)
    for p in real:
        by[p["callsign"]].append(p)
    prow = []
    for cs in sorted(by):
        ps = by[cs]
        pp = [x for x in (grade_points(p) for p in ps) if x is not None]
        tw = [p["wire"] for p in ps if p["outcome"] == "trap" and p["wire"]]
        ic = [p["segs"]["IC"][0] for p in ps if "IC" in p["segs"]]
        prow.append(["<b>%s</b>" % cs, str(len(ps)), str(sum(p["outcome"] == "trap" for p in ps)),
                     str(sum(p["outcome"] == "bolter" for p in ps)), str(sum(p["outcome"] == "wave off" for p in ps)),
                     "%.1f" % (sum(pp) / len(pp)) if pp else "-", "%.1f" % (sum(tw) / len(tw)) if tw else "-",
                     "%+.1f&deg;" % (sum(ic) / len(ic)) if ic else "-"])
    if prow:
        S.append(KeepTogether([Paragraph("Pilots", h), table(
            ["Pilot", "Passes", "Traps", "Bolters", "Wave-offs", "Avg pts", "Avg wire", "Avg GS in close"],
            prow, [1.6, 0.7, 0.7, 0.7, 0.75, 0.75, 0.75, 1.45])]))

    # DCS's own grades (AI included) and the comparison
    if ev["dcs"]:
        drow = [[d["_t"].strftime("%H:%M"), d["_rig"], d.get("callsign", ""), d.get("type", ""), d.get("grade", ""),
                 str(d.get("wire", "") or "-"), "player" if any(p["callsign"] == d.get("callsign") for p in passes)
                 else "AI / no Paddles pass"] for d in ev["dcs"]]
        S.append(KeepTogether([Paragraph("DCS LSO grades (DCS-gRPC)", h), table(
            ["Time", "Rig", "Callsign", "Type", "Grade", "Wire", "Paddles pass?"], drow,
            [0.5, 0.6, 1.5, 1.2, 0.7, 0.5, 2.4])]))
    vs = len(ev["vs"])
    S.append(Paragraph("<b>Paddles vs DCS:</b> %s" % (
        "%d pass(es) graded by both." % vs if vs else
        "no pass graded by both yet. DCS sends grades for AI jets and for players it scores; a matched pass "
        "logs \"vs DCS LSO\" and lands in this table."), note))

    # Radio and health
    abeam = len(ev["abeam"])
    balls = ev["ball"]
    clara = sum(1 for b in balls if b.get("clara"))
    reds = sum(1 for d in ev["deck"] if d.get("red"))
    greens = sum(1 for d in ev["deck"] if d.get("red") is False)
    foul = sum(1 for p in passes if p["grade"] == "WO (foul deck)")
    calls = defaultdict(int)
    for gcall in ev["groove"]:
        calls[gcall.get("call", "")] += 1
    top = ", ".join("%s &times;%d" % (c.rstrip("."), n) for c, n in sorted(calls.items(), key=lambda x: -x[1])[:6]) or "-"
    misses = "<br/>".join("%s %s: \"%s\"" % (m["_t"].strftime("%H:%M"), m["_rig"], m.get("text", "")) for m in ev["miss"]) or "none"
    hc = defaultdict(int)
    for o in ev["health"]:
        hc[(o["_rig"], o["message"].replace("Paddles: ", "").split(",")[0])] += 1
    health = "<br/>".join("%s: %s &times;%d" % (rg, k, n) for (rg, k), n in sorted(hc.items())) or "clean"
    S.append(KeepTogether([Paragraph("Radio and Paddles health", h), table(["Item", "Count / detail"], [
        ["Abeam replies (\"Paddles contact ... call the ball\")", str(abeam)],
        ["Ball calls answered (Clara)", "%d (%d)" % (len(balls), clara)],
        ["Red deck / green deck calls", "%d / %d" % (reds, greens)],
        ["Foul-deck wave-offs", str(foul)],
        ["Groove calls (most said)", top],
        ["Calls Paddles did not understand", misses],
        ["Link and speech problems", health],
    ], [2.6, 4.8])]))

    # Notes
    notes = ["<b>Not reached:</b> %s - this rig's passes are missing." % x for x in problems]
    ic_all = [p["segs"]["IC"][0] for p in traps if "IC" in p["segs"]]
    if ic_all:
        notes.append("Average glideslope in close on traps: %+.1f&deg; (Paddles' earlier ~0.5&deg; high bias shows "
                     "here as a steady positive number)." % (sum(ic_all) / len(ic_all)))
    for m in ev["miss"]:
        if re.search(r"three-quarter mile|3/4 mile", m.get("text", ""), re.I):
            notes.append("A ball call without \"ball\" (\"%s\") went unanswered: say \"Hornet ball\"." % m.get("text", ""))
            break
    if notes:
        S.append(KeepTogether([Paragraph("Notes", h)] + [Paragraph("&bull; " + n, note) for n in notes]))

    doc = SimpleDocTemplate(out_path, pagesize=letter, leftMargin=0.5 * inch, rightMargin=0.5 * inch,
                            topMargin=0.3 * inch, bottomMargin=0.3 * inch,
                            title="Paddles Daily Report %s" % day, author="vSFG7")
    doc.build(S)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--log", action="append", required=True, help="RIG=path (repeat)")
    ap.add_argument("--date")
    ap.add_argument("--from", dest="start")
    ap.add_argument("--to", dest="end")
    ap.add_argument("--out", default=os.path.dirname(os.path.abspath(__file__)))
    ap.add_argument("--sample", action="store_true")
    ap.add_argument("--problem", action="append", default=[], help="a rig that could not be read (noted)")
    ap.add_argument("--no-copy", action="store_true", help="don't copy to the Desktop Documentation folder")
    a = ap.parse_args()
    if a.date:
        start = end = dt.date.fromisoformat(a.date)
    else:
        start, end = dt.date.fromisoformat(a.start), dt.date.fromisoformat(a.end)
    logs = [tuple(x.split("=", 1)) for x in a.log]
    rigs = list(OrderedDict.fromkeys(r for r, _ in logs))
    ev = collect(logs, start, end)
    name = "Paddles Report %s%s.pdf" % (end.isoformat(), " SAMPLE" if a.sample else "")
    out = os.path.join(a.out, name)
    build(ev, start, end, rigs, out, a.sample, a.problem)
    if not a.no_copy:
        for d in DESKTOP:
            if os.path.isdir(d):
                shutil.copy2(out, os.path.join(d, name))
    print("ok", out, "passes:", len(ev["pass"]))


if __name__ == "__main__":
    main()
