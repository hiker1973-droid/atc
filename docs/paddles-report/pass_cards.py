# -*- coding: utf-8 -*-
"""A card for each new graded pass, posted under the greenie board in Discord.

Operator 2026-11-06: the DCS greenie board's pass card. Left, the Groove View:
the pass against the glideslope from the side (ship at the left, dashed
yellow / green / red glideslope lines, the white track) and from above (the
lineup lines and the 1/4 to 1 nm rings). Right, the Approach View: the jet's
last minutes round the ship from above. Under them, the pilot, the grade and
the LSO comment.

Paddles logs each pass's groove ("Paddles: pass track": metres astern :
glideslope error : lineup error, degrees) and its last minutes in the ship's
frame ("Paddles: pass pattern": metres astern : metres to port : feet) beside
the pass line. A pass logged before that gets its four segment averages
instead, and no approach view.

greenie_post.py calls post_cards() after it updates the board. A card is
posted once per pass: the first run only marks the passes already logged, so
nothing old is posted. State: %USERPROFILE%\\readback-greenie-cards.json.
"""
import datetime as dt
import html
import json
import math
import os
import re
import time

import greenie_post as gp

STATE_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-cards.json")
LOGO = "file:///" + os.path.join(os.path.dirname(gp.HERE), "vsfg7_patch.png").replace("\\", "/")
GS_DEG = 3.5  # the Nimitz glideslope (pkg/lso NimitzParams)
M_NM = 1852.0
PW, SH, TH = 840, 250, 290  # a panel's width; the side view's and the top-down view's height
YEL, GRN, RED, WHITE, MUTED = "#e8b400", "#2ea043", "#f85149", "#f0f6fc", "#8b949e"


def pass_key(p):
    return "%s|%s|%s" % (p.get("rig", ""), p["t"].isoformat(), p["callsign"].lower())


def read_logged(days, message, field):
    """(callsign lower, time, [[numbers]]) for each log line with this message; the field is "a:b:c|a:b:c"."""
    data = os.path.join(gp.REPORTS, "_data", "paddles")
    out = []
    for f in sorted(os.listdir(data)):
        m = re.match(r"(\d{4}-\d{2}-\d{2}) (.+)\.log$", f)
        if not m or m.group(1) not in days:
            continue
        for line in open(os.path.join(data, f), encoding="utf-8", errors="replace"):
            if message not in line:
                continue
            try:
                o = json.loads(line)
                t = dt.datetime.fromisoformat(o["time"])
                pts = [[float(x) for x in seg.split(":")] for seg in o[field].split("|")]
            except (ValueError, KeyError):
                continue
            out.append((o.get("callsign", "").lower(), t, pts))
    return out


def match(p, rows):
    """The row for this pass: same callsign, logged within 8 s of the pass line."""
    for cs, t, pts in rows:
        if cs == p["callsign"].lower() and abs((t - p["t"]).total_seconds()) < 8:
            return pts
    return None


def segment_points(p):
    """The four segment means as a coarse track: X, IM, IC, AR at the middle of each."""
    mid = {"X": 1900.0, "IM": 1065.0, "IC": 555.0, "AR": 185.0}
    return [[mid[s], v[0], v[1]] for s, v in p.get("segs", {}).items() if s in mid]


def poly(points, colour, width=2.4, dash=""):
    if len(points) < 2:
        return ""
    return '<polyline fill="none" stroke="%s" stroke-width="%s" stroke-dasharray="%s" points="%s"/>' % (
        colour, width, dash, " ".join("%.1f,%.1f" % q for q in points))


def dots(points):
    return "".join('<circle cx="%.1f" cy="%.1f" r="4" fill="%s"/>' % (x, y, WHITE) for x, y in points)


def text(x, y, s, size=13, colour=MUTED, anchor="start", style=""):
    return '<text x="%.1f" y="%.1f" fill="%s" font-size="%d" text-anchor="%s" %s>%s</text>' % (
        x, y, colour, size, anchor, style, html.escape(s))


def ship_side(x, y):
    return ('<rect x="%d" y="%d" width="76" height="9" rx="2" fill="#6e7681"/><rect x="%d" y="%d" width="12" height="9" fill="#6e7681"/>'
            % (x, y - 2, x + 40, y - 10))


def ship_top(x, y):
    return '<path d="M%d %d l62 -9 l14 0 l0 18 l-14 0 z" fill="#6e7681"/>' % (x, y)


def groove_side(pts):
    """The glideslope from the side: height above the deck against distance astern."""
    base, x0 = SH - 16, 84
    def px(d): return x0 + d * 0.36
    def py(d, deg): return base - d * math.tan(math.radians(deg)) * 1.5
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, SH), ship_side(8, base)]
    for off, col in ((0.5, YEL), (0.0, GRN), (-0.7, RED)):
        s.append(poly([(px(0), py(0, GS_DEG + off)), (px(2000), py(2000, GS_DEG + off))], col, 1.6, "5 4"))
    track = [(px(a), py(a, GS_DEG + g)) for a, g, _ in sorted(pts, key=lambda q: q[0]) if 0 <= a <= 2000]
    s.append(poly(track, WHITE, 2.6) or dots(track))
    s.append(text(PW - 8, 18, "side view, glideslope %.1f degrees" % GS_DEG, 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def groove_top(pts):
    """The lineup from above: the centreline, the lineup lines and the 1/4 .. 1 nm rings."""
    y0, x0 = TH / 2 + 4, 84
    def px(d): return x0 + d * 0.36
    def py(d, deg): return y0 - d * math.tan(math.radians(deg)) * 1.4
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, TH), ship_top(8, y0)]
    s.append(poly([(px(0), y0), (px(2000), y0)], "#c9d1d9", 1.0, "2 5"))
    for deg, col in ((0.4, GRN), (0.75, YEL), (1.5, RED)):
        for sign in (1, -1):
            s.append(poly([(px(0), y0), (px(2000), py(2000, sign * deg))], col, 1.4, "5 4"))
    for nm, lab in ((0.25, "1/4 NM"), (0.5, "1/2 NM"), (0.75, "3/4 NM"), (1.0, "1 NM")):
        x = px(nm * M_NM)
        s.append('<path d="M%.1f %d q%d %d 0 %d" fill="none" stroke="#484f58" stroke-width="1"/>' % (x - 5, 24, 8, (TH - 48) / 2, TH - 48))
        s.append(text(x, TH - 8, lab, 12, MUTED, "middle"))
    track = [(px(a), py(a, l)) for a, _, l in sorted(pts, key=lambda q: q[0]) if 0 <= a <= 2000]
    s.append(poly(track, WHITE, 2.6) or dots(track))
    s.append(text(PW - 8, 18, "from above, lineup", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def approach_view(pat):
    """The jet's last minutes round the ship, from above (ship at the left pointing left; port is down)."""
    h = SH + TH
    sx, sy, k = 230, h / 2, 0.085
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, h)]
    if not pat:
        s.append(text(PW / 2, h / 2, "no approach track for this pass (logged before tracks)", 15, MUTED, "middle"))
        return "".join(s) + "</svg>"
    for nm in (0.5, 1, 2, 3):
        r = nm * M_NM * k
        s.append('<circle cx="%d" cy="%.0f" r="%.1f" fill="none" stroke="#30363d" stroke-width="1.2"/>' % (sx, sy, r))
        s.append(text(sx + r * math.cos(math.radians(25)) + 4, sy - r * math.sin(math.radians(25)) - 4, "%g NM" % nm, 12, "#6e7681"))
    s.append(ship_top(sx - 76, sy))  # the stern (the aft = 0 point) at the rings' centre
    pts = [(sx + a * k, sy + p_ * k, ft) for a, p_, ft in pat]
    s.append(poly([(x, y) for x, y, _ in pts], WHITE, 2.4))
    start = next((q for q, row in zip(pts, pat) if row[0] <= 2400), None)
    if start:
        s.append('<circle cx="%.1f" cy="%.1f" r="5" fill="none" stroke="%s" stroke-width="2"/>' % (start[0], start[1], YEL))
        s.append(text(start[0] + 8, start[1] - 10, "groove start %.1f NM" % (2400 / M_NM), 14, YEL, "start", 'font-style="italic"'))
    abeam = min(((abs(row[0]), q, row) for q, row in zip(pts, pat) if row[1] > 0 and row[0] < 3000), key=lambda t: t[0], default=None)
    if abeam and abeam[0] < 400:
        s.append('<circle cx="%.1f" cy="%.1f" r="5" fill="none" stroke="%s" stroke-width="2"/>' % (abeam[1][0], abeam[1][1], YEL))
        s.append(text(abeam[1][0] + 8, abeam[1][1] + 22, "%d ft abeam" % abeam[2][2], 14, YEL, "start", 'font-style="italic"'))
    s.append(text(PW - 8, 18, "approach, from above", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def card_html(p, label, pts, coarse, pat=None):
    key = gp.board_key(p)
    fill, ink, name = gp.COLOURS[key]
    badge = "WO" if key in ("WOFD", "WO") else ("B" if key == "B" else p["grade"] or key)
    comment = (p.get("comments") or "").strip()
    if p.get("wire"):
        comment = (comment + " WIRE# %d" % p["wire"]).strip()
    model = (p.get("model") or "").replace("FA-18C_hornet", "F/A-18C").replace("_", " ")
    case = ("Case " + p["case"]) if p.get("case") else ""
    when = p["t"].strftime("%A %B %d, %Y %H:%M:%S")
    lines = [when, 'Nimitz Class "ABE"', " | ".join(b for b in (model, case) if b), "",
             name if key in ("WOFD", "WO") else badge, comment, "",
             "(segment averages only: this pass was logged before tracks)" if coarse else ""]
    body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Consolas,'Courier New',monospace;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:14px 16px;width:1696px}
.top{display:flex;align-items:center;gap:18px;margin-bottom:10px}
.b{min-width:96px;height:58px;line-height:58px;text-align:center;border-radius:7px;font:700 32px Arial,sans-serif;background:%s;color:%s;padding:0 14px}
.n{font:700 28px Arial,sans-serif;color:#e6edf3}.logo{margin-left:auto;width:72px;height:72px;border-radius:50%%;box-shadow:0 0 0 2px #30363d}
.ps{display:flex;gap:16px}.pn{flex:none;width:840px;background:#0d1117;border:1px solid #30363d;border-radius:6px;overflow:hidden}
.tab{background:#1f6feb;color:#fff;font:700 22px Arial,sans-serif;text-align:center;padding:9px 0}
.info{display:flex;gap:18px;align-items:center;margin-top:14px;font-size:20px;line-height:1.35;color:#e6edf3}.info img{width:84px;height:84px;border-radius:50%%}
.ft{font:12px Arial,sans-serif;color:#8b949e;margin-top:8px}
</style></head><body><div class="w"><div class="top"><div class="b">%s</div><div class="n">%s</div><img class="logo" src="%s"></div>
<div class="ps"><div class="pn"><div class="tab">Groove View</div>%s%s</div><div class="pn"><div class="tab">Approach View</div>%s</div></div>
<div class="info"><img src="%s"><div>%s</div></div><div class="ft">VMFA-225 Greenieboard &middot; %s</div></div></body></html>""" % (
        fill, ink, html.escape(badge), html.escape(label), LOGO, groove_side(pts), groove_top(pts), approach_view(pat), LOGO, body,
        "track from Paddles" if not coarse else "coarse card")


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
    tracks = read_logged(days, "Paddles: pass track", "track") if todo else []
    patterns = read_logged(days, "Paddles: pass pattern", "pattern") if todo else []
    n = 0
    for p in todo:
        key = canon.get(p["callsign"].lower(), p["callsign"].lower())
        if names and key not in names:
            posted.add(pass_key(p))  # not a roster pilot: no card, don't look again
            continue
        pts = match(p, tracks)
        coarse = not pts
        if coarse:
            pts = segment_points(p)
        png = gp.screenshot(card_html(p, names.get(key, p["callsign"]), pts, coarse, match(p, patterns)), 980, 1760)
        gp.send(hook + "?wait=true", "POST", png, "")
        posted.add(pass_key(p))
        n += 1
        time.sleep(1.0)
    json.dump({"posted": sorted(posted)[-2000:]}, open(STATE_FILE, "w", encoding="utf-8"))
    return n
