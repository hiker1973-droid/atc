# -*- coding: utf-8 -*-
"""The squadron greenie board as a picture, posted to Discord every day.

Operator 2026-11-06: "do our own greenieboard postable in discord ... update
daily", in the look of the DCS greenie board (a row per pilot: average, last
landing, the last 15 passes as coloured squares). Run by run_nightly.py after
the Paddles PDF, on the dev box. Passes come from the same Paddles day logs
(Reports\\_data\\paddles) and day summaries (Reports\\_summary) the PDF's board
uses, so every rig counts.

The webhook URL is one line in %USERPROFILE%\\readback-greenie-webhook.txt (a
secret, kept out of the repo). Without it the picture is still made.

Run:  python -I docs/paddles-report/greenie_post.py [--date YYYY-MM-DD] [--no-post]
"""
import argparse
import datetime as dt
import csv
import html
import json
import os
import re
import subprocess
import sys
import tempfile
import urllib.request
import uuid
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from build_paddles_report import POINTS, board_key, collect  # noqa: E402

REPORTS = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Reports"
WEBHOOK_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-webhook.txt")
PILOTS_FILE = os.path.join(os.path.expanduser("~"), "readback-pilots.csv")
SHOWN = 15  # squares per pilot, newest on the right

# fill, ink, label - the DCS greenie board's colours
COLOURS = {
    "_OK_": ("#f2d600", "#ffffff", "_OK_"),
    "OK": ("#66c200", "#ffffff", "OK"),
    "(OK)": ("#1f6b00", "#ffffff", "(OK)"),
    "---": ("#5a2a08", "#ffffff", "---"),
    "C": ("#d90000", "#ffffff", "C"),
    "B": ("#0a8fd8", "#ffffff", "Bolter"),
    "WO": ("#000000", "#ffffff", "Wave-off"),
    "WOFD": ("#8a93a6", "#ffffff", "Foul deck (not scored)"),
}


def roster():
    """callsign (lower case) -> the board label, from readback-pilots.csv
    (columns: callsign, pilot, board label; the label defaults to "callsign | pilot")."""
    out = {}
    if os.path.exists(PILOTS_FILE):
        for r in csv.reader(open(PILOTS_FILE, encoding="utf-8-sig", newline="")):
            if len(r) >= 2 and r[0].strip() and r[0].strip().lower() != "callsign":
                out[r[0].strip().lower()] = (r[2].strip() if len(r) > 2 and r[2].strip()
                                             else "%s | %s" % (r[0].strip(), r[1].strip()))
    return out


def points(p):
    return None if board_key(p) == "WOFD" else POINTS.get(p["grade"], POINTS.get(
        {"B": "B", "WO": "WO"}.get(board_key(p), ""), None))


def load_passes(end):
    """Every pass up to `end`: raw day logs where they still exist, else the summaries."""
    data = os.path.join(REPORTS, "_data", "paddles")
    logs, have = [], set()
    for f in sorted(os.listdir(data)):
        m = re.match(r"(\d{4}-\d{2}-\d{2}) (.+)\.log$", f)
        if m and m.group(1) <= end.isoformat():
            logs.append((m.group(2), os.path.join(data, f)))
            have.add(m.group(1))
    passes = collect(logs, dt.date(2000, 1, 1), end)["pass"]
    summ = os.path.join(REPORTS, "_summary")
    for f in sorted(os.listdir(summ)) if os.path.isdir(summ) else []:
        d = f[:-5]
        if not f.endswith(".json") or d in have or d > end.isoformat():
            continue
        try:
            s = json.load(open(os.path.join(summ, f), encoding="utf-8"))
        except ValueError:
            continue
        for p in (s.get("paddles") or {}).get("passes") or []:
            passes.append(dict(p, t=dt.datetime.fromisoformat(p["t"])))
    return [p for p in passes if p["outcome"] in ("trap", "bolter", "wave off")]


def square(p):
    key = board_key(p)
    fill, ink, _ = COLOURS[key]
    if p["outcome"] == "trap" and p.get("wire"):
        txt = str(p["wire"])
    else:
        txt = {"B": "B", "WO": "WO", "WOFD": "WO", "---": "---"}.get(key, "")
    return '<td class="s" style="background:%s;color:%s">%s</td>' % (fill, ink, html.escape(txt))


def page(passes, now):
    by = defaultdict(list)
    for p in sorted(passes, key=lambda p: p["t"]):
        by[p["callsign"]].append(p)
    rows = []
    for cs, ps in by.items():
        pts = [x for x in map(points, ps) if x is not None]
        rows.append((sum(pts) / len(pts) if pts else -1, ps[-1]["t"], cs, ps))
    rows.sort(key=lambda r: (-r[0], -r[1].timestamp()))
    allpts = [x for p in passes for x in [points(p)] if x is not None]
    board_avg = "%.2f" % (sum(allpts) / len(allpts)) if allpts else "-"
    last = max((p["t"] for p in passes), default=None)
    ago = ""
    if last:
        h = (now - last.replace(tzinfo=None) if last.tzinfo else now - last).total_seconds() / 3600
        ago = "%d hours ago" % h if h < 48 else "%d days ago" % (h // 24)
    names = roster()
    body = []
    for avg, lt, cs, ps in rows:
        cells = "".join(square(p) for p in ps[-SHOWN:]) + '<td class="e"></td>' * (SHOWN - len(ps[-SHOWN:]))
        body.append('<tr><td class="n">%s</td><td class="a">%s</td><td class="d">%s</td>%s</tr>' % (
            html.escape(names.get(cs.lower(), cs)), "%.1f" % avg if avg >= 0 else "-", lt.strftime("%d-%m"), cells))
    key = "".join('<span class="k" style="background:%s">%s</span>' % (f, html.escape(l))
                  for f, _, l in COLOURS.values())
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#3b445a;font-family:Consolas,'Courier New',monospace;color:#fff}
.w{margin:8px;border:1px solid #aab;padding:6px 8px}
h1{margin:0;text-align:center;font:700 38px 'Segoe UI',Arial,sans-serif}
.i{margin:6px 6px 8px auto;width:max-content;border:1px solid #ddd;padding:8px 40px;text-align:center;font-size:22px;line-height:1.5}
table{border-collapse:collapse;width:100%%}td{border:1px solid #fff;height:52px;text-align:center;font-size:26px}
td.n{text-align:left;width:480px;padding-left:12px;font-size:26px}td.a{width:118px}td.d{width:118px}
td.s{width:54px;font-weight:700}td.e{width:54px}
.k{display:inline-block;padding:2px 10px;margin:0 8px;font-size:22px}
.l{text-align:center;margin-top:14px;font-size:22px}
</style></head><body><div class="w"><h1>Greenieboard</h1>
<div class="i">Last landing: %s<br>Board Avg: %s</div>
<table>%s</table><div class="l">Legend: %s</div></div></body></html>""" % (
        ago or "-", board_avg, "".join(body), key)


def screenshot(src_html, height):
    for edge in (r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
                 r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"):
        if os.path.exists(edge):
            break
    else:
        raise RuntimeError("Microsoft Edge was not found")
    with tempfile.TemporaryDirectory() as d:
        src, out = os.path.join(d, "g.html"), os.path.join(d, "g.png")
        open(src, "w", encoding="utf-8").write(src_html)
        subprocess.run([edge, "--headless", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
                        "--user-data-dir=" + os.path.join(d, "p"), "--window-size=1700,%d" % height,
                        "--screenshot=" + out, "file:///" + src.replace("\\", "/")],
                       check=True, capture_output=True, timeout=90)
        return open(out, "rb").read()


def post(webhook, png, text):
    b = uuid.uuid4().hex
    parts = ['--%s\r\nContent-Disposition: form-data; name="payload_json"\r\nContent-Type: application/json\r\n\r\n%s\r\n'
             % (b, json.dumps({"content": text})),
             '--%s\r\nContent-Disposition: form-data; name="files[0]"; filename="greenieboard.png"\r\n'
             'Content-Type: image/png\r\n\r\n' % b]
    body = parts[0].encode() + parts[1].encode() + png + ("\r\n--%s--\r\n" % b).encode()
    req = urllib.request.Request(webhook, body, {"Content-Type": "multipart/form-data; boundary=" + b,
                                                 "User-Agent": "ReadbackGreenie/1"})
    urllib.request.urlopen(req, timeout=60).read()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", default=(dt.date.today() - dt.timedelta(days=1)).isoformat())
    ap.add_argument("--no-post", action="store_true")
    a = ap.parse_args()
    end = dt.date.fromisoformat(a.date)
    passes = load_passes(end)
    if not passes:
        print("no graded passes yet")
        return 0
    pilots = len({p["callsign"] for p in passes})
    png = screenshot(page(passes, dt.datetime.now()), 230 + 54 * pilots)
    out = os.path.join(REPORTS, "_data", "paddles", "greenieboard.png")
    open(out, "wb").write(png)
    # A copy beside the costs file: the launcher's Stats page shows it (/api/greenie.png).
    open(os.path.join(os.path.expanduser("~"), "readback-greenieboard.png"), "wb").write(png)
    print("ok", out, "pilots:", pilots, "passes:", len(passes))
    if a.no_post or not os.path.exists(WEBHOOK_FILE):
        if not a.no_post:
            print("no webhook file at", WEBHOOK_FILE, "- picture made, not posted")
        return 0
    hook = open(WEBHOOK_FILE, encoding="utf-8").read().strip()
    post(hook, png, "**Greenieboard** - through %s" % end.strftime("%d %b %Y"))
    print("posted")
    return 0


if __name__ == "__main__":
    sys.exit(main())
