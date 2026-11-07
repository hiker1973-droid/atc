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
import pilot_stats as ps

STATE_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-cards.json")
LOGO = "file:///" + os.path.join(os.path.dirname(gp.HERE), "viking_head.png").replace("\\", "/")  # the squadron Viking head (operator 2026-11-06: the blue one); once per card
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
            out.append((o.get("callsign", "").lower(), t, pts, o))
    return out


def match(p, rows):
    """The row for this pass: same callsign, logged within 8 s of the pass line."""
    for cs, t, pts, _ in rows:
        if cs == p.get("logcs", p["callsign"]).lower() and abs((t - p["t"]).total_seconds()) < 8:
            return pts
    return None


def brc_for(p, rows):
    """The carrier's heading logged with this pass's pattern (the BRC), or None."""
    for cs, t, _, o in rows:
        if cs == p.get("logcs", p["callsign"]).lower() and abs((t - p["t"]).total_seconds()) < 8:
            return o.get("brc") or None
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



HAND = 'font-family="Arial, Helvetica, sans-serif" font-style="italic"'
WORDS = {"H": "high", "LO": "low", "LUL": "lined up left", "LUR": "lined up right", "SLO": "slow", "F": "fast"}
WHERE = {"X": "at the start (first third)", "IM": "in the middle", "IC": "in close (last third)", "AR": "at the ramp"}
REMARK_RE = re.compile(r"(__)?(\()?(LUL|LUR|SLO|LO|H|F)(\))?(__)?(X|IM|IC|AR)\b")


def remark(comment):
    """The LSO comment codes in words, the way the DCS card says them: "(LUL)X" is "a little lined up left at the
    start", "__LO__AR" is "VERY low at the ramp"."""
    out = []
    for m in REMARK_RE.finditer(comment or ""):
        amount = "VERY " if m.group(1) or m.group(5) else "a little " if m.group(2) else ""
        out.append("%s%s %s" % (amount, WORDS[m.group(3)], WHERE[m.group(6)]))
    if "[BC]" in (comment or ""):
        out.append("(no ball call)")
    return " ".join(out)


def arrow(x1, y1, x2, y2):
    """A hand-drawn style arrow from a label to the point it names."""
    dx, dy = x2 - x1, y2 - y1
    cx, cy = (x1 + x2) / 2 - dy * 0.18, (y1 + y2) / 2 + dx * 0.18
    ang = math.atan2(y2 - cy, x2 - cx)
    a = (x2 - 9 * math.cos(ang + 0.45), y2 - 9 * math.sin(ang + 0.45))
    b = (x2 - 9 * math.cos(ang - 0.45), y2 - 9 * math.sin(ang - 0.45))
    return ('<path d="M%.1f %.1f Q%.1f %.1f %.1f %.1f" fill="none" stroke="%s" stroke-width="1.6"/>'
            '<polyline points="%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="none" stroke="%s" stroke-width="1.6"/>'
            % (x1, y1, cx, cy, x2, y2, YEL, a[0], a[1], x2, y2, b[0], b[1], YEL))


def note(x, y, s, size=17, anchor="start"):
    return '<text x="%.1f" y="%.1f" fill="%s" font-size="%d" text-anchor="%s" %s>%s</text>' % (x, y, YEL, size, anchor, HAND, html.escape(s))

WIRES_FROM_STERN_M = (55, 67, 79, 92)  # pkg/lso NimitzParams: wire 1 .. 4, metres forward of the stern


def wire_ticks(pos, wire, half, label_at=None):
    """The four arresting wires as ticks across the deck, the one that caught the jet highlighted. pos(m) is the x, y of a
    wire m metres forward of the stern; half is the tick's half length. label_at (x, y) puts "3 wire" with an arrow."""
    out = []
    for n, m in enumerate(WIRES_FROM_STERN_M, 1):
        x, y = pos(m)
        hit = n == wire
        out.append('<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="%s"/>' % (x, y - half * (1.5 if hit else 1), x, y + half * (1.5 if hit else 1), YEL if hit else "#c9d1d9", 3.2 if hit else 1.2))
    if wire and label_at:
        x, y = pos(WIRES_FROM_STERN_M[wire - 1])
        lx, ly = label_at
        out.append(note(lx, ly, "%d wire" % wire, 16, "middle") + arrow(lx, ly + 6, x, y - half * 1.5 - 2))
    return "".join(out)


def groove_side(pts, wire=None):
    """The glideslope from the side: height above the deck against distance astern."""
    base, x0 = SH - 16, 84
    def px(d): return x0 + d * 0.36
    def py(d, deg): return base - d * math.tan(math.radians(deg)) * 1.5
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, SH), ship_side(8, base), wire_ticks(lambda m: (x0 - m * 0.36, base + 2.5), wire, 9, (x0 - 30, base - 44))]
    for off, col in ((0.5, YEL), (0.0, GRN), (-0.7, RED)):
        s.append(poly([(px(0), py(0, GS_DEG + off)), (px(2000), py(2000, GS_DEG + off))], col, 1.6, "5 4"))
    track = [(px(a), py(a, GS_DEG + g)) for a, g, *_ in sorted(pts, key=lambda q: q[0]) if 0 <= a <= 2000]
    s.append(poly(track, WHITE, 2.6) or dots(track))
    s.append(text(PW - 8, 18, "side view, glideslope %.1f degrees" % GS_DEG, 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def wind_arrow(x, y, wind_dir, wind_kts, brc):
    """The wind as the DCS card draws it: an arrow (where the wind is going, in the ship's frame - bow to the left)
    and "270 at 12 kn" under it."""
    rel = math.radians(wind_dir + 180 - brc)  # the direction it blows toward, from the ship's heading
    dx, dy = -math.cos(rel) * 22, -math.sin(rel) * 22  # bow left, starboard up
    a = math.atan2(dy, dx)
    head = "%.1f,%.1f %.1f,%.1f %.1f,%.1f" % (x + dx - 9 * math.cos(a - 0.5), y + dy - 9 * math.sin(a - 0.5), x + dx, y + dy,
                                              x + dx - 9 * math.cos(a + 0.5), y + dy - 9 * math.sin(a + 0.5))
    return ('<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="2.6"/><polyline points="%s" fill="none" stroke="%s" stroke-width="2.6"/>'
            % (x - dx, y - dy, x + dx, y + dy, YEL, head, YEL)) + text(x, y + 44, "Wind %03d at %d kn" % (round(wind_dir) % 360 or 360, round(wind_kts)), 14, YEL, "middle")


def groove_top(pts, groove_s=None, wind=None, wire=None):
    """The lineup from above: the centreline, the lineup lines and the 1/4 .. 1 nm rings."""
    y0, x0 = TH / 2 + 4, 84
    def px(d): return x0 + d * 0.36
    def py(d, deg): return y0 - d * math.tan(math.radians(deg)) * 0.75
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, TH), ship_top(8, y0), wire_ticks(lambda m: (84 - m * 0.36, y0), wire, 8, (84 - 28, y0 - 40))]
    s.append(poly([(px(0), y0), (px(2000), y0)], "#c9d1d9", 1.0, "2 5"))
    for deg, col in ((0.4, GRN), (0.75, YEL), (1.5, RED)):
        for sign in (1, -1):
            s.append(poly([(px(0), y0), (px(2000), py(2000, sign * deg))], col, 1.4, "5 4"))
    for nm, lab in ((0.25, "1/4 NM"), (0.5, "1/2 NM"), (0.75, "3/4 NM"), (1.0, "1 NM")):
        x = px(nm * M_NM)
        s.append('<path d="M%.1f %d q%d %d 0 %d" fill="none" stroke="#484f58" stroke-width="1"/>' % (x - 5, 24, 8, (TH - 48) / 2, TH - 48))
        s.append(text(x, TH - 8, lab, 12, MUTED, "middle"))
    track = [(px(a), py(a, l)) for a, _, l, *__ in sorted(pts, key=lambda q: q[0]) if 0 <= a <= 2000]
    s.append(poly(track, WHITE, 2.6) or dots(track))
    if groove_s:  # T-19: seconds in the groove, pointed at the half-mile point
        x, y = px(0.5 * M_NM), y0 - 0.75 * 0.5 * M_NM * math.tan(math.radians(0.3))
        s.append(note(x - 70, y0 - 110, "T-%d" % round(groove_s)) + arrow(x - 56, y0 - 100, x, y - 6))
    near = min(((abs(a - M_NM), a, g) for a, g, *_ in pts if 0 < a <= 2000), default=None)
    if near and near[0] < 450:  # Wake Alt: the height over the water at one mile
        a, g = near[1], near[2]
        ft = (a * math.tan(math.radians(GS_DEG + g)) + 19.5) * 3.28084
        x, y = px(a), py(a, next(r[2] for r in pts if r[0] == a))
        s.append(note(x - 170, TH - 36, "Alt at 1 NM: %d ft" % round(ft)) + arrow(x - 60, TH - 50, x - 3, y + 6))
    if wind:
        s.append(wind_arrow(84, TH - 72, *wind))
    s.append(text(PW - 8, 18, "from above, lineup", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def carrier_top(sx, sy, brc=None):
    """The carrier from above, bow to the left, stern at (sx, sy): hull, island, the angled landing area (9 degrees to
    port) and its centreline running astern; the BRC (the ship's heading) written over it with an arrow."""
    k = 0.085
    s = ['<path d="M%d %d L%d %d L%d %d L%d %d L%d %d Z" fill="#6e7681"/>' % (sx, sy - 16, sx - 96, sy - 16, sx - 126, sy, sx - 96, sy + 16, sx, sy + 16)]
    s.append('<rect x="%d" y="%d" width="22" height="8" fill="#495057"/>' % (sx - 70, sy - 16))  # the island, starboard side
    dx = 100.0
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#c9d1d9" stroke-width="7" stroke-linecap="round" opacity=".55"/>'
             % (sx - 6, sy - 7, sx - 6 - dx, sy - 7 + dx * math.tan(math.radians(9.14))))  # the angled deck
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#c9d1d9" stroke-width="1" stroke-dasharray="3 6" opacity=".6"/>'
             % (sx - 6, sy - 7, sx - 6 + 2200 * k, sy - 7 - 2200 * k * math.tan(math.radians(9.14))))  # the landing axis, astern
    if brc is not None:
        s.append(text(sx - 63, sy - 30, "BRC %03d" % round(brc) if round(brc) < 360 else "BRC 000", 15, "#e6edf3", "middle", "font-weight=\"bold\""))
        s.append('<polyline points="%d,%d %d,%d" stroke="#e6edf3" stroke-width="1.6" fill="none"/>' % (sx - 126, sy - 22, sx - 96, sy - 22))
        s.append('<polyline points="%d,%d %d,%d %d,%d" stroke="#e6edf3" stroke-width="1.6" fill="none"/>' % (sx - 121, sy - 26, sx - 126, sy - 22, sx - 121, sy - 18))
    return "".join(s)


def approach_view(pat, brc=None, wind=None, wire=None):
    """The jet's last minutes round the ship, from above (ship at the left pointing left; port is down)."""
    h = SH + TH
    sx, sy, k = 330, 118, 0.08
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, h)]
    if not pat:
        s.append(text(PW / 2, h / 2, "no approach track recorded for this pass", 15, MUTED, "middle"))
        return "".join(s) + "</svg>"
    ring_boxes = []
    for nm in (0.5, 1, 2, 3):
        r = nm * M_NM * k
        s.append('<circle cx="%d" cy="%.0f" r="%.1f" fill="none" stroke="#30363d" stroke-width="1.2"/>' % (sx, sy, r))
        # the small rings are labelled above and to the right; the ship is near the top of the panel, so the 2 and 3 NM
        # rings are labelled below and to the right, where they are on the panel
        ang = math.radians(25 if nm <= 1 else -70 if nm == 2 else -40)
        lx, ly = sx + r * math.cos(ang) + 4, sy - r * math.sin(ang) - 4 * (1 if nm <= 1 else -3)
        s.append(text(lx, ly, "%g NM" % nm, 13, "#8b949e"))
        ring_boxes.append((lx - 2, ly - 14, lx + len("%g NM" % nm) * 7 + 2, ly + 5))
    # the downwind corridor: 1.2 / 1.3 / 1.5 NM to port of the ship, parallel to its heading
    for nm, col in ((1.2, YEL), (1.3, GRN), (1.5, RED)):
        y = sy + nm * M_NM * k
        s.append(poly([(sx - 140, y), (sx + 3300 * k, y)], col, 1.3, "6 5"))
        s.append(text(sx + 3300 * k + 6, y + 4, "%.1f NM" % nm, 12, col))
    s.append(carrier_top(sx, sy, brc))  # the stern (the aft = 0 point) at the rings' centre
    if wire:  # the wires on the angled deck (the deck is drawn bigger than the rings' scale), the one that caught the jet highlighted
        ca, sa = math.cos(math.radians(9.14)), math.sin(math.radians(9.14))
        s.append(wire_ticks(lambda m: (sx - 6 - m * 0.38 * ca, sy - 7 + m * 0.38 * sa), wire, 7, (sx - 52, sy + 44)))
        ring_boxes.append((sx - 80, sy + 22, sx - 20, sy + 52))
    pts = [(sx + row[0] * k, sy + row[1] * k, row[2]) for row in pat]
    s.append(poly([(x, y) for x, y, _ in pts], WHITE, 2.4))
    if wind:
        s.append(wind_arrow(100, 468, *wind))
    ring_boxes += [(sx - 130, sy - 52, sx + 4, sy + 20)]  # the ship and its BRC label
    s += callouts(pat, pts, ring_boxes)
    s.append(text(PW - 8, 18, "approach, from above", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def break_index(pat):
    """Where the break began: the first point, after a straight run in, where the jet's course turns more than 15
    degrees off that run for three samples in a row. None when it never turns (a straight-in)."""
    n = len(pat)
    if n < 12:
        return None
    course = lambda i: math.degrees(math.atan2(pat[i + 1][1] - pat[i][1], pat[i + 1][0] - pat[i][0]))
    base = course(2)
    off = lambda i: abs((course(i) - base + 180) % 360 - 180)
    for i in range(3, n - 4):
        if all(off(i + j) > 15 for j in range(3)) and abs(pat[i][0]) < 3 * M_NM:
            return i
    return None


def _hit(a, b):
    return a[0] < b[2] and b[0] < a[2] and a[1] < b[3] and b[1] < a[3]


def callouts(pat, pts, avoid=()):
    """The DCS card's hand-written numbers: speed and height over the ship, the apex of the break, downwind,
    the height abeam and where the jet rolled out. A note that would land on another note (or on a ring label in
    `avoid`) is moved to the next free spot, so the words never overlap (Turbo's pass 2026-11-06)."""
    out = []
    taken = list(avoid)

    def put(px, py, s, size, cands, sub=None):
        """A note for the point (px, py): the first of cands (dx, dy, anchor) whose box is free, then its arrow."""
        w = max(len(s) * size * 0.5, len(sub) * 5.5 if sub else 0)

        def box(dx, dy, anc):
            lx = px + dx
            x0 = lx if anc == "start" else lx - w if anc == "end" else lx - w / 2
            return (x0 - 3, py + dy - size, x0 + w + 3, py + dy + (17 if sub else 6))
        pick = next((c for c in cands if not any(_hit(box(*c), t) for t in taken)), cands[0])
        taken.append(box(*pick))
        dx, dy, anc = pick
        lx, ly = px + dx, py + dy
        out.append(note(lx, ly, s, size, anc) + (text(lx, ly + 14, sub, 10, MUTED, anc) if sub else ""))
        ax, ay = lx + (-4 if anc == "start" else 4 if anc == "end" else 0), (ly + 8 + (14 if sub else 0) if dy < 0 else ly - size - 2)
        d = math.hypot(ax - px, ay - py) or 1.0
        out.append(arrow(ax, ay, px + (ax - px) / d * 7, py + (ay - py) / d * 7))
    n = len(pat)
    kts = lambda i: pat[i][3] if len(pat[i]) > 3 and pat[i][3] > 40 else None
    label = lambda i: ("%d kts " % kts(i) if kts(i) else "") + "%d ft" % pat[i][2]
    brk = next((i for i in range(n) if pat[i][1] > 600), n)   # the break: first well to port
    bi = break_index(pat)
    if bi is not None:  # "in the break": where the turn began and how far from the boat that was
        x, y, _ = pts[bi]
        d = abs(pat[bi][0]) / M_NM  # along the ship's heading: ahead of the ship is "past ABE"
        out.append('<circle cx="%.1f" cy="%.1f" r="6" fill="none" stroke="%s" stroke-width="2"/>' % (x, y, YEL))
        put(x, y, "in the break: %.1f NM %s ABE" % (d, "past" if pat[bi][0] < 0 else "short of"), 17,
            [(36, -74, "start"), (-36, -74, "end"), (36, 80, "start"), (-36, 80, "end"), (36, -110, "start")])
    over = min(range(0, min(brk, n)), key=lambda i: abs(pat[i][0]), default=None)
    if over is not None and abs(pat[over][0]) < 600 and over > 2:
        x, y, _ = pts[over]
        put(x, y, label(over), 15, [(-10, -46, "end"), (10, -46, "start"), (-10, 56, "end"), (10, 56, "start")])
    ahead = [i for i in range(brk, n) if pat[i][0] < 0]
    if ahead:
        i = min(ahead, key=lambda i: pat[i][0])
        x, y, _ = pts[i]
        put(x, y, "%d ft" % pat[i][2], 15, [(-14, -24, "end"), (14, -24, "start"), (-14, 40, "end"), (14, 40, "start")])
    far = max((row[1] for row in pat), default=0)
    down = next((i for i in range(brk, n) if pat[i][1] > 0.8 * far), None)
    if down is not None and far > 800:
        x, y, _ = pts[down]
        put(x, y, label(down), 15, [(14, -34, "start"), (-14, -34, "end"), (14, 44, "start"), (-14, 44, "end")])
        ab = min((i for i in range(down, n) if pat[i][1] > 0.6 * far), key=lambda i: abs(pat[i][0]), default=None)
        if ab is not None and abs(pat[ab][0]) < 700:
            x, y, _ = pts[ab]
            put(x, y, "%d ft" % pat[ab][2], 17, [(-20, 60, "end"), (20, 60, "start"), (-20, -50, "end"), (20, -50, "start")], "contact abeam")
        roll = next((i for i in range((ab or down) + 1, n) if abs(pat[i][1]) < 250 and pat[i][0] > 0), None)
        if roll is not None:
            x, y, _ = pts[roll]
            put(x, y, "%.1f NM" % (pat[roll][0] / M_NM), 17, [(20, -40, "start"), (20, 56, "start"), (-20, -40, "end"), (-20, 56, "end")], "rolled out point")
    return out


def read_context(days):
    """(callsign lower, time, line) for each "Paddles: pass context" log line of those days."""
    data = os.path.join(gp.REPORTS, "_data", "paddles")
    out = []
    for f in sorted(os.listdir(data)):
        m = re.match(r"(\d{4}-\d{2}-\d{2}) (.+)\.log$", f)
        if not m or m.group(1) not in days:
            continue
        for line in open(os.path.join(data, f), encoding="utf-8", errors="replace"):
            if "Paddles: pass context" not in line:
                continue
            try:
                o = json.loads(line)
                out.append((o.get("callsign", "").lower(), dt.datetime.fromisoformat(o["time"]), o))
            except (ValueError, KeyError):
                continue
    return out


def context_for(p, rows):
    for cs, t, o in rows:
        if cs == p.get("logcs", p["callsign"]).lower() and abs((t - p["t"]).total_seconds()) < 8:
            return o
    return None


MAPS = (("UG", "Caucasus"), ("XR", "Caucasus"), ("OM", "Persian Gulf"), ("OK", "Persian Gulf"), ("LC", "Syria"), ("OL", "Syria"),
        ("OS", "Syria"), ("LT", "Syria"), ("ED", "Germany"), ("ET", "Germany"), ("EB", "Germany"), ("OR", "Iraq"), ("EN", "Kola"))


def map_name(ctx, rig):
    """The DCS map: from the carrier's field in the context, else the rig's usual map."""
    icao = ((ctx or {}).get("icao") or "").upper()
    for prefix, name in MAPS:
        if icao.startswith(prefix):
            return name
    return "Syria" if rig == "Foothold" else "Caucasus"


def weather_line(ctx):
    """A brief weather summary: wind, visibility, ceiling, altimeter."""
    if not ctx or not ctx.get("altInHg"):
        return ""
    ceil = ctx.get("ceilingFt") or 0
    vis = ctx.get("visNm") or 0
    return "Wind %03d at %d kts, vis %s, %s, QNH %.2f" % (
        round(ctx.get("windDir") or 0) % 360 or 360, round(ctx.get("windKts") or 0),
        "10+ NM" if vis >= 10 or vis <= 0 else "%.0f NM" % vis,
        "no ceiling" if ceil <= 0 or ceil >= 20000 else "ceiling %s ft" % format(int(round(ceil, -2)), ","), ctx["altInHg"])


def mission_time(ctx):
    mt = (ctx or {}).get("missionTime")
    try:
        return dt.datetime.fromisoformat(mt.replace("Z", "+00:00")).strftime("%A %B %d, %Y %H:%M:%SZ")
    except (AttributeError, ValueError):
        return ""


def groove_line(pts):
    """The speed and angle of attack down the groove, from the track Paddles logged (speed:AoA are the 4th and 5th numbers)."""
    rows = [r for r in pts if len(r) >= 5 and r[3] > 40]
    if not rows:
        return ""
    near = min(rows, key=lambda r: abs(r[0] - M_NM))
    aoas = [r[4] for r in rows if r[4] > 0]
    out = "Groove: %d kts at %.1f NM" % (round(near[3]), near[0] / M_NM)
    if aoas:
        out += ", AoA %.1f avg (on speed %.1f)" % (sum(aoas) / len(aoas), ps.HORNET_ON_SPEED_AOA)
    return out


def card_html(p, label, pts, coarse, pat=None, brc=None, ctx=None, stats_line=""):
    key = gp.board_key(p)
    fill, ink, name = gp.COLOURS[key]
    badge = "WO" if key in ("WOFD", "WO") else ("B" if key == "B" else p["grade"] or key)
    comment = (p.get("comments") or "").strip()
    if p.get("wire"):
        comment = (comment + " WIRE# %d" % p["wire"]).strip()
    model = (p.get("model") or "").replace("FA-18C_hornet", "F/A-18C").replace("_", " ")
    case = ("Case " + p["case"]) if p.get("case") else ""
    wind = (ctx["windDir"], ctx.get("windKts") or 0, brc) if ctx and ctx.get("windDir") is not None and brc is not None else None
    if p.get("manual"):  # added by hand: the time of day is only a place in the order, so show the date alone
        when = p["t"].strftime("%A %B %d, %Y")
        comment = ""
    else:
        when = mission_time(ctx) or p["t"].strftime("%A %B %d, %Y %H:%M:%S") + " (real time)"
    daynight = ("Night" if ctx.get("isNight") else "Day") if ctx and "isNight" in ctx else ""
    wod = ps.wind_over_deck(ctx, brc)
    wod_line = ("Wind over deck %d kts (ship %d, headwind %d, crosswind %d)" % (round(wod[0]), round((ctx or {}).get("shipKts") or 0),
                                                                              round(wod[1]), round(abs(wod[2])))) if wod and (ctx or {}).get("shipKts") else ""
    lines = ['Nimitz Class "ABE" | %s' % map_name(ctx, p.get("rig")), when, *([weather_line(ctx)] if weather_line(ctx) else []),
             *([wod_line] if wod_line else []), *([groove_line(pts)] if not coarse and groove_line(pts) else []),
             " | ".join(b for b in (model, case, daynight) if b), "",
             name if key in ("WOFD", "WO") else badge, comment, "", remark(p.get("comments")),
             *([stats_line] if stats_line else []),
             "(added by hand: no track was recorded)" if p.get("manual") else
             "(segment averages only: this pass was logged before tracks)" if coarse else ""]
    body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:14px 16px;width:1696px}
.top{display:flex;align-items:center;gap:18px;margin-bottom:10px}
.b{min-width:96px;height:58px;line-height:58px;text-align:center;border-radius:7px;font:700 32px Arial,sans-serif;background:%s;color:%s;padding:0 14px}
.n{font:700 28px Arial,sans-serif;color:#e6edf3}.cs{font:700 24px Arial,sans-serif;color:#fff;background:#0a8fd8;border-radius:7px;padding:8px 16px}.logo{margin-left:auto;height:92px;width:auto;filter:drop-shadow(0 0 2px #fff) drop-shadow(0 0 5px rgba(255,255,255,.55))}
.ps{display:flex;gap:16px}.pn{flex:none;width:840px;background:#0d1117;border:1px solid #30363d;border-radius:6px;overflow:hidden}
.tab{background:#1f6feb;color:#fff;font:700 22px Arial,sans-serif;text-align:center;padding:9px 0}
.info{margin-top:14px;font-size:20px;line-height:1.35;color:#e6edf3}
.ft{font:12px Arial,sans-serif;color:#8b949e;margin-top:8px}
</style></head><body><div class="w"><div class="top"><div class="b">%s</div><div class="n">%s</div>%s<img class="logo" src="%s"></div>
<div class="ps"><div class="pn"><div class="tab">Groove View</div>%s%s</div><div class="pn"><div class="tab">Approach View</div>%s</div></div>
<div class="info"><div>%s</div></div><div class="ft">VMFA-225 Greenieboard &middot; %s</div></div></body></html>""" % (
        fill, ink, html.escape(badge), html.escape(label), ("<div class=\"cs\">%s</div>" % html.escape(case.upper())) if case else "", LOGO, groove_side(pts, p.get("wire")), groove_top(pts, (p.get("pattern") or {}).get("groove", (None,))[0], wind, p.get("wire")), approach_view(pat, brc, wind, p.get("wire")), body,
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
    contexts = read_context(days) if todo else []
    everyone, _ = ps.grouped(gp.load_passes(dt.date.today())) if todo else ({}, None)
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
        png = gp.screenshot(card_html(p, names.get(key, p["callsign"]), pts, coarse, match(p, patterns), brc_for(p, patterns), context_for(p, contexts),
                                      ps.pilot_line(names.get(key, p["callsign"]), everyone.get(key, []))), 1080, 1760)
        gp.send(hook + "?wait=true", "POST", png, "")
        posted.add(pass_key(p))
        n += 1
        time.sleep(1.0)
    json.dump({"posted": sorted(posted)[-2000:]}, open(STATE_FILE, "w", encoding="utf-8"))
    return n
