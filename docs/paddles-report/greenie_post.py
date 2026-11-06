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
import urllib.error
import urllib.request
import uuid
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from build_paddles_report import POINTS, board_key, collect  # noqa: E402

REPORTS = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Reports"
WEBHOOK_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-webhook.txt")
PILOTS_FILE = os.path.join(os.path.expanduser("~"), "readback-pilots.csv")
OVERRIDES_FILE = os.path.join(os.path.expanduser("~"), "readback-grade-overrides.csv")
MANUAL_FILE = os.path.join(os.path.expanduser("~"), "readback-manual-passes.csv")
MESSAGE_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-message.txt")
RIGS = [("Day", "192.168.1.220"), ("Night", "192.168.1.221"), ("Foothold", "192.168.1.222")]
SHOWN = 15  # squares per pilot, newest on the left

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
    out = []
    if os.path.exists(PILOTS_FILE):
        for r in csv.reader(open(PILOTS_FILE, encoding="utf-8-sig", newline="")):
            r = [x.strip() for x in r] + ["", "", "", "", ""]
            if r[0].lower() == "callsign" or not (r[0] or r[1] or r[2]):
                continue
            out.append((r[0].lower(), r[2] or " | ".join(x for x in (r[0], r[1]) if x),
                        r[3].lower() == "reserve", [a.strip().lower() for a in r[4].split(";") if a.strip()]))
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
    for p in passes:
        p["case3"] = p.get("case") == "III"  # Paddles writes ", case III" in the pass line (2026-11-06 on)
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
    passes = [p for p in passes if p["outcome"] in ("trap", "bolter", "wave off")]
    # Passes added by hand (date, time, callsign, wire, grade (OK, (OK), ---, C, B, WO, _OK_), case, note); case III draws the circle.
    if os.path.exists(MANUAL_FILE):
        for r in csv.reader(open(MANUAL_FILE, encoding="utf-8-sig", newline="")):
            r = [x.strip() for x in r] + [""] * 7
            if r[0].lower() == "date" or not r[0]:
                continue
            passes.append({"t": dt.datetime.fromisoformat("%sT%s:00" % (r[0], r[1] or "12:00")).astimezone(), "callsign": r[2],
                           "outcome": "wave off" if r[4].upper().startswith("WO") else "bolter" if r[4] == "B" else "trap",
                           "grade": r[4], "wire": int(r[3]) if r[3] else None,
                           "case3": r[5].upper() in ("III", "3"), "comments": r[6]})
    # LSO corrections: readback-grade-overrides.csv (date, callsign, wire, grade, note).
    if os.path.exists(OVERRIDES_FILE):
        for r in csv.reader(open(OVERRIDES_FILE, encoding="utf-8-sig", newline="")):
            r = [x.strip() for x in r] + ["", "", "", ""]
            if r[0].lower() == "date" or not r[0]:
                continue
            for p in passes:
                if (p["t"].date().isoformat() == r[0] and p["callsign"].lower() == r[1].lower()
                        and str(p.get("wire") or "") == r[2]):
                    p["grade"] = r[3]
    return passes


def square(p):
    key = board_key(p)
    fill, ink, _ = COLOURS[key]
    if p["outcome"] == "trap" and p.get("wire"):
        txt = str(p["wire"])
    else:
        # No wire number (a pass entered by hand without one): show the grade itself.
        txt = {"B": "B", "WO": "WO", "WOFD": "WO"}.get(key, key)
    small = ' style="font-size:17px"' if len(txt) > 2 else ""
    if p.get("case3"):
        txt = '<span class="c3"%s>%s</span>' % (small, html.escape(txt))
    else:
        txt = '<span%s>%s</span>' % (small, html.escape(txt)) if small else html.escape(txt)
    return '<td class="s" style="background:%s;color:%s">%s</td>' % (fill, ink, txt)


def page(passes, now):
    names = {cs: label for cs, label, _, _ in roster()}
    # Another way Paddles hears a pilot (their bare name) counts as their callsign.
    canon = {a: cs for cs, _, _, al in roster() for a in al if cs}
    # Only pilots in readback-pilots.csv are on the board (flight callsigns like "Raider 1-1" are not pilots).
    if names:
        passes = [p for p in passes if canon.get(p["callsign"].lower(), p["callsign"].lower()) in names]
    by, shown = defaultdict(list), {}
    for p in sorted(passes, key=lambda p: p["t"]):
        cs = p["callsign"]
        key = canon.get(cs.lower(), cs.lower())
        shown.setdefault(key, cs)
        by[key].append(p)
    rows = []
    for cs, ps in by.items():
        pts = [x for x in map(points, ps) if x is not None]
        rows.append((sum(pts) / len(pts) if pts else -1, ps[-1]["t"], cs, ps))
    # Everyone in the CSV (but the Reserve group) is on the board; no passes yet = an empty row at the bottom.
    for cs, label, reserve, _ in roster():
        if not reserve and not any(c.lower() == cs for c in by):
            rows.append((-2, dt.datetime.min.replace(tzinfo=None), label, []))
    rows.sort(key=lambda r: (-r[0], -(r[1].timestamp() if r[3] else 0)))
    allpts = [x for p in passes for x in [points(p)] if x is not None]
    board_avg = "%.2f" % (sum(allpts) / len(allpts)) if allpts else "-"
    last = max((p["t"] for p in passes), default=None)
    ago = ""
    if last:
        h = (now - last.replace(tzinfo=None) if last.tzinfo else now - last).total_seconds() / 3600
        ago = "%d hours ago" % h if h < 48 else "%d days ago" % (h // 24)
    body = []
    for avg, lt, cs, ps in rows:
        cells = "".join(square(p) for p in reversed(ps[-SHOWN:])) + '<td class="e"></td>' * (SHOWN - len(ps[-SHOWN:]))
        body.append('<tr><td class="n">%s</td><td class="a">%s</td><td class="d">%s</td>%s</tr>' % (
            html.escape(names.get(cs.lower()) or shown.get(cs, cs)), "%.1f" % avg if avg >= 0 else "-",
            lt.strftime("%d-%m") if ps else "-", cells))
    key = "".join('<span class="k" style="background:%s">%s</span>' % (f, html.escape(l))
                  for f, _, l in COLOURS.values())
    logo = "file:///" + os.path.join(os.path.dirname(HERE), "vsfg7_patch.png").replace("\\", "/")
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;padding:10px 12px;border-radius:6px}
.h{position:relative;height:150px}
.h img{position:absolute;left:4px;top:0;width:140px;height:140px;border-radius:50%%;box-shadow:0 0 0 2px #30363d,0 0 14px rgba(88,166,255,.3)}
h1{margin:0;position:absolute;left:0;right:0;top:44px;text-align:center;font:700 44px Arial,Helvetica,sans-serif;color:#58a6ff}
.i{position:absolute;right:4px;top:6px;border:1px solid #30363d;background:#1c2330;border-radius:5px;padding:8px 36px;text-align:center;font-size:22px;line-height:1.5}
table{border-collapse:collapse;width:100%%}td{border:1px solid #30363d;height:52px;text-align:center;font-size:26px}
tr:nth-child(even){background:#1c2330}
td.n{text-align:left;width:480px;padding-left:12px;font-size:26px;color:#e6edf3}td.a{width:118px}td.d{width:118px;color:#8b949e}
td.s{width:54px;font-weight:700}td.e{width:54px}
.c3{display:inline-block;width:46px;height:46px;line-height:46px;border-radius:50%%;background:rgba(0,0,0,.45)}
.k{display:inline-block;padding:2px 10px;margin:0 8px;font-size:22px;color:#fff}
.l{text-align:center;margin-top:14px;font-size:22px;color:#8b949e}
</style></head><body><div class="w"><div class="h"><img src="%s" alt=""><h1>VMFA-225 Greenieboard</h1>
<div class="i">Last landing: %s<br>Board Avg: %s</div></div>
<table>%s</table><div class="l">Legend: %s</div></div></body></html>""" % (
        logo, ago or "-", board_avg, "".join(body), key)


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


def multipart(png, text):
    b = uuid.uuid4().hex
    head = ('--%s\r\nContent-Disposition: form-data; name="payload_json"\r\nContent-Type: application/json\r\n\r\n%s\r\n'
            '--%s\r\nContent-Disposition: form-data; name="files[0]"; filename="greenieboard.png"\r\n'
            'Content-Type: image/png\r\n\r\n') % (b, json.dumps({"content": text, "attachments": []}), b)
    return head.encode() + png + ("\r\n--%s--\r\n" % b).encode(), "multipart/form-data; boundary=" + b


def send(url, method, png, text):
    body, ctype = multipart(png, text)
    req = urllib.request.Request(url, body, {"Content-Type": ctype, "User-Agent": "ReadbackGreenie/1"}, method=method)
    return json.loads(urllib.request.urlopen(req, timeout=60).read() or b"{}")


def post(webhook, png, text):
    """Edit the board's one message in place; make it (and remember its id) the first time,
    or when someone deleted it."""
    mid = open(MESSAGE_FILE).read().strip() if os.path.exists(MESSAGE_FILE) else ""
    if mid:
        try:
            send("%s/messages/%s" % (webhook, mid), "PATCH", png, text)
            return "edited"
        except urllib.error.HTTPError as e:
            if e.code != 404:
                raise
    made = send(webhook + "?wait=true", "POST", png, text)
    open(MESSAGE_FILE, "w").write(str(made["id"]))
    return "posted"


def fetch_today(day):
    """Today's Paddles logs from every rig, saved like the nightly run saves them (a later fetch of the same day overwrites)."""
    data = os.path.join(REPORTS, "_data", "paddles")
    for rig, host in RIGS:
        try:
            with urllib.request.urlopen("http://%s:7000/api/lso-log?date=%s" % (host, day), timeout=60) as r:
                body, ctype = r.read(), r.headers.get("Content-Type", "")
        except (urllib.error.URLError, OSError) as e:
            print("%s: %s" % (rig, e))
            continue
        if "html" not in ctype:
            with open(os.path.join(data, "%s %s.log" % (day, rig)), "wb") as f:
                f.write(body)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", default=(dt.date.today() - dt.timedelta(days=1)).isoformat())
    ap.add_argument("--no-post", action="store_true")
    ap.add_argument("--refresh", action="store_true", help="hourly: fetch today's logs from the rigs first, board through today")
    a = ap.parse_args()
    if a.refresh:
        a.date = dt.date.today().isoformat()
        fetch_today(a.date)
    end = dt.date.fromisoformat(a.date)
    passes = load_passes(end)
    if not passes:
        print("no graded passes yet")
        return 0
    known = {c for c, _, _, _ in roster()}
    pilots = len({canon_cs for canon_cs in (p["callsign"].lower() for p in passes) if canon_cs in known or not known}
                 | {c for c, _, r, _ in roster() if not r})
    png = screenshot(page(passes, dt.datetime.now()), 260 + 54 * pilots)
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
    what = post(hook, png, "**Greenieboard** - updated %s" % dt.datetime.now().strftime("%d %b %H:%M"))
    print(what)
    return 0


if __name__ == "__main__":
    sys.exit(main())
