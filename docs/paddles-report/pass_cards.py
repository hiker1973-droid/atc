# -*- coding: utf-8 -*-
"""A card for each new graded pass, posted under the greenie board in Discord.

Operator 2026-11-06: the DCS greenie board's pass card (grade, pilot, the LSO
comment and wire, the groove and approach views). Paddles logs each pass's
track ("Paddles: pass track": metres astern : glideslope error : lineup
error, degrees) beside the pass line; the card draws the glideslope against
distance (groove view) and the lineup against distance (approach view). A pass
logged before tracks existed gets its four segment means instead.

greenie_post.py calls post_cards() after it updates the board. A card is
posted once per pass: the first run only marks the passes already logged, so
nothing old is posted. State: %USERPROFILE%\\readback-greenie-cards.json.
"""
import datetime as dt
import html
import json
import os
import re
import time

import greenie_post as gp

STATE_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-cards.json")
LOGO = "file:///" + os.path.join(os.path.dirname(gp.HERE), "vsfg7_patch.png").replace("\\", "/")
W, H = 420, 250  # one chart
X0, X1 = 2400.0, 0.0  # metres astern, left to right


def pass_key(p):
    return "%s|%s|%s" % (p.get("rig", ""), p["t"].isoformat(), p["callsign"].lower())


def read_tracks(days):
    """(callsign lower, time, [(astern, gs, lu)]) for every "Paddles: pass track" line of those days."""
    data = os.path.join(gp.REPORTS, "_data", "paddles")
    out = []
    for f in sorted(os.listdir(data)):
        m = re.match(r"(\d{4}-\d{2}-\d{2}) (.+)\.log$", f)
        if not m or m.group(1) not in days:
            continue
        for line in open(os.path.join(data, f), encoding="utf-8", errors="replace"):
            if "Paddles: pass track" not in line:
                continue
            try:
                o = json.loads(line)
                t = dt.datetime.fromisoformat(o["time"])
                pts = [tuple(float(x) for x in seg.split(":")) for seg in o["track"].split("|")]
            except (ValueError, KeyError):
                continue
            out.append((o.get("callsign", "").lower(), t, pts))
    return out


def track_for(p, tracks):
    for cs, t, pts in tracks:
        if cs == p["callsign"].lower() and abs((t - p["t"]).total_seconds()) < 8:
            return pts
    return None


def segment_points(p):
    """The four segment means as a coarse track: X, IM, IC, AR at the middle of each."""
    mid = {"X": 1900.0, "IM": 1065.0, "IC": 555.0, "AR": 185.0}
    return [(mid[s], v[0], v[1]) for s, v in p.get("segs", {}).items() if s in mid]


def chart(title, pts, idx, band, limit, unit):
    """One chart: error (degrees) against distance astern; the green band is on-path, dashed red is a big error."""
    top = 3.0
    def px(a):
        return 34 + (X0 - a) / (X0 - X1) * (W - 44)
    def py(v):
        v = max(-top, min(top, v))
        return 22 + (top - v) / (2 * top) * (H - 44)
    svg = ['<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">' % (W, H, W, H)]
    svg.append('<rect x="34" y="%.1f" width="%d" height="%.1f" fill="rgba(63,185,80,.20)"/>' % (py(band), W - 44, py(-band) - py(band)))
    for v, col, dash in ((0, "#8b949e", ""), (limit, "#f85149", "4 4"), (-limit, "#f85149", "4 4")):
        svg.append('<line x1="34" x2="%d" y1="%.1f" y2="%.1f" stroke="%s" stroke-width="1" stroke-dasharray="%s"/>' % (W - 10, py(v), py(v), col, dash))
        svg.append('<text x="4" y="%.1f" fill="#8b949e" font-size="11">%+.1f</text>' % (py(v) + 4, v))
    for a in (2000, 1500, 1000, 500, 0):
        svg.append('<text x="%.1f" y="%d" fill="#8b949e" font-size="11" text-anchor="%s">%s</text>' % (px(a) + (6 if a == 0 else 0), H - 6, "end" if a == 0 else "middle", "ramp" if a == 0 else "%.1f nm" % (a / 1852.0)))
    pts = sorted(pts, key=lambda q: -q[0])
    if len(pts) > 1:
        svg.append('<polyline fill="none" stroke="#e6edf3" stroke-width="2.2" points="%s"/>' % " ".join("%.1f,%.1f" % (px(q[0]), py(q[idx])) for q in pts))
    elif pts:
        svg.append('<circle cx="%.1f" cy="%.1f" r="4" fill="#e6edf3"/>' % (px(pts[0][0]), py(pts[0][idx])))
    svg.append("</svg>")
    return '<div class="ch"><div class="ct">%s</div>%s<div class="cu">%s</div></div>' % (title, "".join(svg), unit)


def card_html(p, label, pts, coarse):
    key = gp.board_key(p)
    fill, ink, name = gp.COLOURS[key]
    badge = {"WOFD": "WO", "B": "B"}.get(key, p["grade"] if p["grade"] else key)
    if key == "WOFD":
        badge = "WO"
    comment = (p.get("comments") or "").strip()
    if p.get("wire"):
        comment = (comment + " WIRE# %d" % p["wire"]).strip()
    model = (p.get("model") or "").replace("FA-18C_hornet", "F/A-18C").replace("F-14B", "F-14B").replace("_", " ")
    case = ("CASE " + p["case"]) if p.get("case") else ""
    bits = [b for b in (model, case, p["t"].strftime("%d %b %H:%M")) if b]
    note = "coarse: segment averages" if coarse else ""
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:14px 16px;width:920px}
.top{display:flex;align-items:center;gap:16px}
.b{min-width:84px;height:54px;line-height:54px;text-align:center;border-radius:7px;font-weight:700;font-size:30px;background:%s;color:%s;padding:0 12px}
.n{font-size:24px;font-weight:700;color:#e6edf3}.cm{font-size:18px;margin-top:3px}.mt{font-size:15px;color:#8b949e;margin-top:3px}
.logo{margin-left:auto;width:64px;height:64px;border-radius:50%%;box-shadow:0 0 0 2px #30363d}
.cs{display:flex;gap:16px;margin-top:12px}.ch{background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:6px 8px}
.ct{font-size:16px;font-weight:700;color:#58a6ff;margin:2px 0 0 4px}.cu{font-size:12px;color:#8b949e;margin:0 0 2px 4px}
.ft{font-size:12px;color:#8b949e;margin-top:8px}
</style></head><body><div class="w"><div class="top"><div class="b">%s</div><div><div class="n">%s</div><div class="cm">%s</div><div class="mt">%s</div></div><img class="logo" src="%s"></div>
<div class="cs">%s%s</div><div class="ft">VMFA-225 &middot; %s</div></div></body></html>""" % (
        fill, ink, html.escape(badge), html.escape(label), html.escape(comment or name), html.escape(" | ".join(bits)), LOGO,
        chart("Groove view", pts, 1, 0.6, 1.2, "glideslope error, degrees (+ high)"),
        chart("Approach view", pts, 2, 0.75, 1.5, "lineup error, degrees (+ right)"),
        html.escape(note or "track from Paddles"))


def post_cards(hook, log_passes, days):
    """Post a card for each pass not yet posted (the first run only marks what is already logged)."""
    first = not os.path.exists(STATE_FILE)
    state = {"posted": []} if first else json.load(open(STATE_FILE, encoding="utf-8"))
    posted = set(state["posted"])
    todo = [p for p in sorted(log_passes, key=lambda p: p["t"]) if p.get("rig") and pass_key(p) not in posted]
    if first:
        posted.update(pass_key(p) for p in todo)
        todo = []
    names = {cs: label for cs, label, _, _ in gp.roster()}
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    tracks = read_tracks(days) if todo else []
    n = 0
    for p in todo:
        key = canon.get(p["callsign"].lower(), p["callsign"].lower())
        if names and key not in names:
            posted.add(pass_key(p))  # not a roster pilot: no card, don't look again
            continue
        pts = track_for(p, tracks)
        coarse = not pts
        if coarse:
            pts = segment_points(p)
        png = gp.screenshot(card_html(p, names.get(key, p["callsign"]), pts, coarse), 470, 960)
        gp.send(hook + "?wait=true", "POST", png, "")
        posted.add(pass_key(p))
        n += 1
        time.sleep(1.0)
    json.dump({"posted": sorted(posted)[-2000:]}, open(STATE_FILE, "w", encoding="utf-8"))
    return n
