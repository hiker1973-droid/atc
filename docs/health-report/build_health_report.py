# -*- coding: utf-8 -*-
"""SkyEye Daily Health Report - every role on every rig, one PDF (ReportLab).

Operator 2026-10-18: "also do a daily health report on ATC/ATIS/Marshall ect".
run_health.py fetches each rig's role logs for the day (launcher /api/day-log)
and writes a manifest; this builds the PDF from it: a status per role (green /
amber / red), pilot calls heard and answered, calls not understood, restarts,
SRS / Tacview drops, TTS and Whisper failures, ATIS ExternalAudio crashes, the
day's top warnings and errors, and ElevenLabs credits.

The message names are the Msg()s in cmd/atc and pkg/controller.

Run:  python -I docs/health-report/build_health_report.py --manifest M.json --out DIR
"""
import argparse
import datetime as dt
import json
import os
import re
from collections import Counter, OrderedDict, defaultdict

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import KeepTogether, Paragraph, SimpleDocTemplate, Table, TableStyle

ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
GREY = colors.HexColor("#555555")
STATUS = {"green": "#2e7d32", "amber": "#d98e04", "red": "#c62828", "silent": "#8a96a8"}
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=GREY, alignment=1, spaceAfter=3)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=4, spaceAfter=1.5)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.2, leading=8.3)
num = ParagraphStyle("nu", parent=cell, alignment=2)
hdr = ParagraphStyle("hd", parent=cell, fontSize=6.8, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.4, leading=8.6, spaceBefore=1)
big = ParagraphStyle("b", parent=ss["Normal"], fontSize=15, leading=17, textColor=NAVY, alignment=1,
                     fontName="Helvetica-Bold")
small = ParagraphStyle("sm", parent=cell, alignment=1, textColor=GREY)

HEARD = {"recognized", "Command heard", "Marshal heard", "Deckboss heard"}
# "Tower TX" replaced "TX via OpenAI TTS + ExternalAudio" on 2026-10-23; older logs keep the old name.
REPLIED = {"Tower TX", "TX via OpenAI TTS + ExternalAudio", "Command TX", "Marshal TX", "Deckboss TX", "Paddles TX", "GCI TX"}
# GCI (Darkstar / Overlord, cmd/atc/gci.go): a heard call SkyEye's parser could
# not place is a miss; it also catches pilots talking to each other on 264.
GCI_MISS = "GCI: not addressed to us or not understood - silent"
# What the GCI said unprompted, by SkyEye call type (the "call" field of GCI TX).
GCI_CALLS = {"brevity.ThreatCall": "gci_threat", "brevity.MergedCall": "gci_merged",
             "brevity.FadedCall": "gci_faded", "brevity.PictureResponse": "gci_picture"}
# Start-up banners and other lines that are not problems.
NOISE = re.compile(r"^═|^Speed warnings and conflict detection DISABLED|^TACVIEW OFFLINE|"
                   r"^\.miz weather read failed|^Tacview connected — no air contacts yet")


def table(head, rows, widths, style=()):
    data = [[Paragraph(x, hdr) for x in head]] + [[x if not isinstance(x, str) else Paragraph(x, cell)
                                                   for x in r] for r in rows]
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("TOPPADDING", (0, 0), (-1, -1), 1.4), ("BOTTOMPADDING", (0, 0), (-1, -1), 1.4),
        ("LEFTPADDING", (0, 0), (-1, -1), 2.5), ("RIGHTPADDING", (0, 0), (-1, -1), 2.5),
    ] + list(style)))
    return t


def read_lines(path):
    try:
        f = open(path, encoding="utf-8", errors="replace")
    except OSError:
        return
    with f:
        for line in f:
            i = line.find("{")
            if i < 0:
                continue
            try:
                yield json.loads(line[i:])
            except ValueError:
                continue


def analyse(entry, date):
    """Counts for one role's log on one rig."""
    m = Counter()
    problems = Counter()
    first, last = {}, {}
    misses = []
    for o in read_lines(entry["path"]):
        ts = o.get("time", "")
        if not ts.startswith(date):
            continue
        msg, lvl = o.get("message", ""), o.get("level", "")
        m["lines"] += 1
        if msg == "vsfg7-atc starting":
            m["starts"] += 1
        if msg in HEARD:
            m["heard"] += 1
        if msg in REPLIED:
            m["replied"] += 1
        if msg == "GCI TX" and o.get("call") in GCI_CALLS:
            m[GCI_CALLS[o["call"]]] += 1
        if msg.startswith("GCI: recovering"):
            m["gci_held"] += 1
        if msg.startswith("Whisper prompt echo - recovered"):
            m["echo_saved"] += 1
        if "intent miss" in msg or msg == "no callsign in call - not answered" or msg == GCI_MISS:
            m["misses"] += 1
            misses.append((ts[11:16], o.get("text", "")))
        if "SRS disconnected" in msg:
            m["srs"] += 1
        if "Tacview" in msg and "disconnected" in msg:
            m["tacview"] += 1
        if "TTS failed" in msg or "TTS synthesis failed" in msg:
            m["tts"] += 1
        if msg.startswith("ElevenLabs TTS failed"):
            m["eleven"] += 1
        if msg.startswith("Whisper transcription error"):
            m["whisper"] += 1
        if msg.startswith("ExternalAudio crashed"):
            m["eacrash"] += 1
        if msg == "ATIS generating new audio":
            m["atis_new"] += 1
        if lvl in ("warn", "error") and not NOISE.search(msg):
            m[lvl] += 1
            key = (lvl, msg[:90])
            problems[key] += 1
            first.setdefault(key, ts[11:16])
            last[key] = ts[11:16]
    return m, problems, first, last, misses


def status(m, kind):
    if not m["lines"]:
        return "silent", "no log lines"
    why = []
    red = m["tts"] > 0 or m["whisper"] > 3 or (m["heard"] >= 5 and m["misses"] / m["heard"] > 0.4)
    if m["tts"]:
        why.append("TTS failed %d" % m["tts"])
    if m["whisper"]:
        why.append("Whisper errors %d" % m["whisper"])
    if m["heard"] >= 4 and m["misses"] / m["heard"] > 0.2:
        why.append("%d%% not understood" % round(100 * m["misses"] / m["heard"]))
    if m["srs"]:
        why.append("SRS drops %d" % m["srs"])
    if m["tacview"] > 2:
        why.append("Tacview drops %d" % m["tacview"])
    if m["eleven"]:
        why.append("ElevenLabs fallbacks %d" % m["eleven"])
    if m["error"]:
        why.append("errors %d" % m["error"])
    note = []
    if kind == "gci":
        # Not problems - what the GCI did, so a quiet night is told from a dead one.
        note = ["%s %d" % (k, m[f]) for k, f in (("pictures", "gci_picture"), ("threats", "gci_threat"),
                                                 ("merged", "gci_merged"), ("faded", "gci_faded"),
                                                 ("recoveries held", "gci_held")) if m[f]]
    if m["echo_saved"]:
        note.append("%d calls saved by the Whisper retry" % m["echo_saved"])
    if red:
        return "red", ", ".join(why + note)
    if why:
        return "amber", ", ".join(why + note)
    return "green", ", ".join(note)


def dot(st):
    return "<font color='%s' size=9>&#9679;</font> <font size=6.2 color='#555555'>%s</font>" % (STATUS[st], st)


def build(man, out_path):
    date = man["date"]
    day = dt.date.fromisoformat(date)
    S = [Paragraph("SkyEye Daily Health Report", title),
         Paragraph("vSFG7 SkyEye &middot; %s &middot; %s" % (day.strftime("%a %d %b %Y"),
                                                            " / ".join(r["rig"] for r in man["rigs"])), sub)]
    per_rig = []
    allprob = defaultdict(lambda: [0, "99:99", "00:00", set()])
    allmiss = []
    totals = Counter()
    states = Counter()
    for rig in man["rigs"]:
        rows = []
        for e in rig["logs"]:
            m, probs, first, last, misses = analyse(e, date)
            if not m["lines"] and not e.get("running", True):
                continue  # a field this rig's mission doesn't use
            st, why = status(m, e["kind"])
            states[st] += 1
            totals.update({k: m[k] for k in ("heard", "replied", "misses", "eacrash", "starts", "srs", "tts")})
            for k, n in probs.items():
                a = allprob[k]
                a[0] += n
                a[1] = min(a[1], first[k])
                a[2] = max(a[2], last[k])
                a[3].add("%s %s" % (rig["rig"], e["label"]))
            for t, text in misses:
                allmiss.append((t, rig["rig"], e["label"], text))
            extra = ""
            if e["kind"] == "atis":
                extra = "%d new, %d EA crashes" % (m["atis_new"], m["eacrash"])
            rows.append([Paragraph(dot(st), cell), "<b>%s</b>" % e["label"],
                         Paragraph(str(m["starts"]), num), Paragraph(str(m["heard"]), num),
                         Paragraph(str(m["replied"]), num), Paragraph(str(m["misses"]), num),
                         Paragraph(str(m["srs"]), num), Paragraph(str(m["tacview"]), num),
                         Paragraph(str(m["tts"] + m["whisper"]), num), Paragraph(str(m["warn"]), num),
                         Paragraph(str(m["error"]), num),
                         "<font size=6.4>%s</font>" % (why or extra or "-")])
        per_rig.append((rig, rows))

    def tile(v, label):
        return [Paragraph(v, big), Paragraph(label, small)]

    heard = totals["heard"]
    credits = sum(r.get("credits") or 0 for r in man["rigs"])
    tiles = [tile("%d" % states["green"], "roles green"), tile("%d" % states["amber"], "amber"),
             tile("%d" % states["red"], "red"), tile(str(heard), "pilot calls heard"),
             tile(str(totals["replied"]), "replies sent"),
             tile("%d%%" % round(100 * totals["misses"] / heard) if heard else "-", "not understood"),
             tile(str(totals["eacrash"]), "ATIS audio crashes"),
             tile("{:,}".format(credits) if credits else "-", "ElevenLabs credits")]
    tt = Table([[x[0] for x in tiles], [x[1] for x in tiles]], colWidths=[0.935 * inch] * 8)
    tt.setStyle(TableStyle([("BOX", (0, 0), (-1, -1), 0.6, NAVY), ("INNERGRID", (0, 0), (-1, -1), 0.3, SHADE),
                            ("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#f5f7fa")),
                            ("TOPPADDING", (0, 0), (-1, -1), 2), ("BOTTOMPADDING", (0, 0), (-1, -1), 2)]))
    S.append(tt)
    if man.get("problems"):
        S.append(Paragraph("<font color='#c62828'><b>Not reached:</b></font> %s" % "; ".join(man["problems"]), note))

    head = ["Status", "Role", "Starts", "Heard", "Replied", "Missed", "SRS", "Tacview", "TTS/STT", "Warn",
            "Error", "Why / notes"]
    widths = [0.62, 1.38, 0.42, 0.42, 0.5, 0.47, 0.33, 0.5, 0.52, 0.38, 0.4, 1.66]
    for rig, rows in per_rig:
        info = rig.get("version", "")
        S.append(KeepTogether([Paragraph("%s &middot; <font size=8 color='#555555'>%s%s</font>" % (
            rig["rig"], rig["host"], (" &middot; build " + info) if info else ""), h), table(head, rows, widths)]))
    S.append(Paragraph(
        "<b>Starts</b>: the role (re)started - Updates and region restarts. <b>Heard / Replied / Missed</b>: pilot "
        "calls transcribed, transmissions sent, calls not understood. <b>SRS / Tacview</b>: link drops. "
        "<b>TTS / STT</b>: speech or Whisper failures. Towers that share a log (Batumi + Deckboss) are one row. "
        "<b>Red</b>: speech failed, Whisper failing, or over 40% of calls not understood; <b>amber</b>: link "
        "drops, ElevenLabs fallbacks, errors, or over 20% not understood. ATIS ExternalAudio crashes are resent "
        "and not flagged.", note))

    if allprob:
        prow = []
        for (lvl, msg), (n, f, l, where) in sorted(allprob.items(), key=lambda kv: (kv[0][0] != "error", -kv[1][0]))[:18]:
            color = "#c62828" if lvl == "error" else "#d98e04"
            w = sorted(where)
            prow.append(["<font color='%s'><b>%s</b></font>" % (color, lvl), msg.replace("<", "&lt;"),
                         ", ".join(w[:4]) + (" +%d" % (len(w) - 4) if len(w) > 4 else ""),
                         Paragraph(str(n), num), "%s - %s" % (f, l)])
        S.append(KeepTogether([Paragraph("Top warnings and errors", h),
                               table(["Level", "Message", "Where", "Count", "First - last"], prow,
                                     [0.45, 3.2, 2.35, 0.45, 1.15])]))

    if allmiss:
        allmiss.sort()
        mrow = [[t, rg, lb, '"%s"' % tx.replace("<", "&lt;")] for t, rg, lb, tx in allmiss[:40]]
        more = len(allmiss) - len(mrow)
        S.append(KeepTogether([Paragraph("Calls not understood (%d)" % len(allmiss), h),
                               table(["Time", "Rig", "Role", "What the pilot said (Whisper)"], mrow,
                                     [0.45, 0.6, 1.5, 4.75])] +
                              ([Paragraph("... and %d more in the data folder." % more, note)] if more else [])))
    else:
        S.append(Paragraph("Calls not understood", h))
        S.append(Paragraph("None.", note))

    if credits:
        S.append(Paragraph("<b>ElevenLabs:</b> %s. Days are UTC, as ElevenLabs bills them." % ", ".join(
            "%s {:,}".format(r["credits"]) % r["rig"] for r in man["rigs"] if r.get("credits")), note))

    doc = SimpleDocTemplate(out_path, pagesize=letter, leftMargin=0.45 * inch, rightMargin=0.45 * inch,
                            topMargin=0.3 * inch, bottomMargin=0.3 * inch,
                            title="SkyEye Daily Health Report %s" % date, author="vSFG7")
    doc.build(S)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--manifest", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    man = json.load(open(a.manifest, encoding="utf-8"))
    out = os.path.join(a.out, "SkyEye Health Report %s.pdf" % man["date"])
    build(man, out)
    print("ok", out)


if __name__ == "__main__":
    main()
