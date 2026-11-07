# -*- coding: utf-8 -*-
"""A Marshal card: the stack and the commence for one Case III approach, in the style of the pass card.

Operator 2026-11-06: "an overview for the marshal stack and commencing like the groove / approach in Trap house".
Left, the Stack View: the carrier in the middle (north up, rings every 10 NM), the marshal radial, every jet in the
stack at its hold fix (angels + 15 DME) with its approach time, and the way in: commence, platform, dirty up at
8 NM, the hand-off to Paddles at 3 NM. Right, the Commence View: the same approach from the side - altitude
against range, the stack levels, the platform at 5,000 ft, the 3.5 degree glideslope - with the times Marshal logged
under it. Real radar fixes (Marshal logs them from the 2026-11-06 update) are drawn where they exist; otherwise the
assigned radial and DME and the planned profile are drawn and the card says so.

Run:  python -I docs/paddles-report/marshal_card.py            (draws an example from the latest Case III approach)
"""
import datetime as dt
import functools
import glob
import html
import json
import math
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I does not put the script's folder on the path
import greenie_post as gp  # noqa: E402
import marshal_report as mr  # noqa: E402
import pass_cards as pc  # noqa: E402

M_NM = 1852.0
PW, PH = 840, 520
RANGE = 30  # NM the views reach
YEL, GRN, RED, WHITE, MUTED = pc.YEL, pc.GRN, pc.RED, pc.WHITE, pc.MUTED
PLATFORM_FT = 5000
PLANNED = False  # operator 2026-11-06: the cards show what the pilot actually did, not the Case I / III instructions
GS_DEG = 3.5


ROOT = os.path.dirname(gp.HERE)  # the repo's docs folder
EXAMPLE_CARRIER = (42.20, 40.80)  # where the example cards put the carrier (off the Georgian coast) until Marshal logs the real position


@functools.lru_cache(maxsize=1)
def land_rings():
    """The outer ring of every land polygon (Natural Earth 1:50m land, public domain), as [(lon, lat)]."""
    d = json.load(open(os.path.join(ROOT, "maps", "ne_50m_land.geojson"), encoding="utf-8"))
    out = []
    for f in d["features"]:
        g = f["geometry"]
        for poly in (g["coordinates"] if g["type"] == "MultiPolygon" else [g["coordinates"]]):
            out.append(poly[0])
    return out


@functools.lru_cache(maxsize=1)
def airfield_points():
    """[(ICAO, name, lat, lon)] for every airfield the repo knows (pkg/airfield)."""
    out = []
    for f in glob.glob(os.path.join(os.path.dirname(ROOT), "pkg", "airfield", "*.go")):
        m = re.search(r'ICAO:\s*"(\w+)".*?Name:\s*"([^"]+)".*?Center:\s*orb\.Point\{\s*([-\d.]+),\s*([-\d.]+)\s*\}', open(f, encoding="utf-8").read(), re.S)
        if m:
            out.append((m.group(1), m.group(2), float(m.group(4)), float(m.group(3))))
    return out


def geo_layer(carrier, pt, view_nm):
    """The map under a view: land and coast, and the airfields in range. pt(bearing, nm) is the view's own projection
    from the carrier, so the map turns and scales with it."""
    lat0, lon0 = carrier
    cosl = math.cos(math.radians(lat0))
    dlat, dlon = view_nm / 60.0, view_nm / (60.0 * cosl)
    parts = []
    for ring in land_rings():
        lons, lats = [q[0] for q in ring], [q[1] for q in ring]
        if max(lons) < lon0 - dlon or min(lons) > lon0 + dlon or max(lats) < lat0 - dlat or min(lats) > lat0 + dlat:
            continue
        pts = []
        for lon, lat in ring:
            e, n = (lon - lon0) * 60.0 * cosl, (lat - lat0) * 60.0
            pts.append(pt(math.degrees(math.atan2(e, n)), math.hypot(e, n)))
        parts.append("M" + " L".join("%.1f,%.1f" % q for q in pts) + " Z")
    out = ['<clipPath id="vw"><rect width="%d" height="%d"/></clipPath>' % (PW, PH),
           '<path d="%s" fill="#1c2430" stroke="#46515f" stroke-width="1.2" clip-path="url(#vw)"/>' % " ".join(parts)]
    for icao, name, lat, lon in airfield_points():
        e, n = (lon - lon0) * 60.0 * cosl, (lat - lat0) * 60.0
        if math.hypot(e, n) > view_nm:
            continue
        x, y = pt(math.degrees(math.atan2(e, n)), math.hypot(e, n))
        if 6 < x < PW - 6 and 6 < y < PH - 6:
            out.append('<circle cx="%.1f" cy="%.1f" r="3.5" fill="#c9d1d9"/>' % (x, y) + pc.text(x + 6, y + 4, name, 11, "#aab4c0"))
    return "".join(out)


def mins(a, b):
    return (b - a).total_seconds() / 60.0 if a and b else None


def stack_view(rec, mates, brc, name, carrier=None):
    """From above, turned so the marshal radial runs left across the panel (ship on the right). Rings every 5 NM, the
    carrier with its angled deck and BRC, every jet at its hold with the racetrack it flies, the way in, and
    hand-written callouts like the approach view. A small arrow shows where north is."""
    radial = rec["radial"]
    off = 270 - radial  # the radial points left on the screen
    cx, cy = PW - 130, PH / 2 + 10
    k = (cx - 50) / float(RANGE)

    def pt(brg, nm):
        a = math.radians(brg + off)
        return cx + nm * k * math.sin(a), cy - nm * k * math.cos(a)
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, PH)]
    if carrier:
        s.append(geo_layer(carrier, pt, RANGE + 12))
    for nm in range(5, RANGE + 1, 5):
        major = nm % 10 == 0
        s.append('<circle cx="%d" cy="%d" r="%.1f" fill="none" stroke="%s" stroke-width="%s"/>' % (cx, cy, nm * k, "#3b434d" if major else "#232a33", 1.3 if major else 1))
        s.append(pc.text(cx - nm * k, 44, "%d NM" % nm, 13 if major else 11, "#8b949e" if major else "#58616b", "middle"))
    x30, y30 = pt(radial, RANGE)
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#58a6ff" stroke-width="1.4" stroke-dasharray="6 5" opacity=".7"/>' % (cx, cy, x30, y30))
    s.append(pc.text(x30 + 6, y30 - 8, "%03d radial" % (round(radial) % 360 or 360), 13, "#58a6ff", "start"))
    nx, ny = 52, 118  # north arrow
    a = math.radians(off)
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#8b949e" stroke-width="2"/>' % (nx, ny, nx + 26 * math.sin(a), ny - 26 * math.cos(a)))
    s.append(pc.text(nx + 36 * math.sin(a), ny - 36 * math.cos(a) + 5, "N", 14, MUTED, "middle"))
    # the carrier: hull, island and angled deck, turned to its BRC
    rot = (brc if brc is not None else 0) + off - 270
    s.append('<g transform="rotate(%.1f %d %d)">%s</g>' % (rot, cx, cy, pc.carrier_top(cx, cy, None)))
    if brc is not None:
        s.append(pc.text(cx, cy - 34, "BRC %03d" % (round(brc) % 360 or 360), 14, "#e6edf3", "middle", 'font-weight="bold"'))
    # every jet's logged hold fix, and this jet's flown track (green until it commenced, yellow after) with its logged fixes
    for m in mates:
        hold = next((f for f in m["fixes"] if f[0] == "hold" and f[3] is not None), None)
        if hold and m is not rec:
            x, y = pt(hold[3], hold[2])
            s.append('<circle cx="%.1f" cy="%.1f" r="6" fill="#30363d" stroke="#8b949e" stroke-width="2"/>' % (x, y) + pc.text(x, y - 14, "A%d %s" % (m["angels"] or 6, name(m)), 12, MUTED, "middle"))
    com_t = rec["commence"].timestamp() if rec.get("commence") else None
    legs = ([], [])
    for ut, dist, brg, alt in rec.get("track") or []:
        legs[1 if (com_t and ut >= com_t) else 0].append(pt(brg, dist))
    if legs[0] and legs[1]:
        legs[0].append(legs[1][0])
    for pts_, col in ((legs[0], GRN), (legs[1], YEL)):
        if len(pts_) > 1:
            s.append(pc.poly(pts_, col, 2.6))
    if not (rec.get("track") or []) and len([f for f in rec["fixes"] if f[3] is not None]) > 1:
        s.append(pc.poly([pt(f[3], f[2]) for f in rec["fixes"] if f[3] is not None], WHITE, 2.4, "6 5"))
    for n, (kind, when, d, brg, ang) in enumerate(f for f in rec["fixes"] if f[3] is not None):
        x, y = pt(brg, d)
        s.append('<circle cx="%.1f" cy="%.1f" r="5" fill="%s" stroke="%s" stroke-width="2"/>' % (x, y, "#1f6feb" if kind == "hold" else YEL, YEL))
        up = n % 2 == 0
        s.append(pc.note(x, y + (-30 if up else 46), "%s A%s %s" % (kind, ang if ang is not None else "?", when.strftime("%H:%M")), 14, "middle") + pc.arrow(x, y + (-22 if up else 32), x, y + (-8 if up else 8)))
    if not rec["fixes"] and not rec.get("track"):
        s.append(pc.text(cx - 200, cy - 80, "no radar positions recorded for this approach", 15, MUTED, "middle"))
    s.append(pc.text(PW - 8, 18, "from above, turned so the radial runs left", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def commence_view(rec, mates, name):
    """From the side: the ship, the stack levels, the descent at commence to the 5,000 ft platform and the 3.5 degree
    glideslope with its tolerance lines, and the callouts with their times."""
    x0, base = 60, PH - 40
    kx, ky = (PW - x0 - 30) / float(RANGE), (PH - 90) / 8000.0

    def px(nm): return x0 + nm * kx
    def py(ft): return base - ft * ky
    gs_at = lambda nm, off=0.0: nm * M_NM * math.tan(math.radians(GS_DEG + off)) * 3.28084
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, PH), pc.ship_side(8, base)]
    for ft in (2000, 4000, 6000, 8000):
        s.append('<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#21262d"/>' % (x0, py(ft), PW - 10, py(ft)))
        s.append(pc.text(PW - 12, py(ft) - 4, "%d ft" % ft, 12, "#6e7681", "end"))
    for nm in (5, 10, 15, 20, 25, 30):
        s.append('<line x1="%.1f" y1="%d" x2="%.1f" y2="%d" stroke="%s"/>' % (px(nm), 40, px(nm), base, "#262d36" if nm % 10 == 0 else "#1c2128"))
        s.append(pc.text(px(nm), base + 18, "%d NM" % nm, 12, "#8b949e" if nm % 10 == 0 else "#58616b", "middle"))
    tr = rec.get("track") or []
    com_t = rec["commence"].timestamp() if rec.get("commence") else None
    legs = ([], [])
    for ut, dist, brg, alt in tr:
        legs[1 if (com_t and ut >= com_t) else 0].append((px(dist), py(alt)))
    if legs[0] and legs[1]:
        legs[0].append(legs[1][0])
    for pts_, col in ((legs[0], GRN), (legs[1], YEL)):
        if len(pts_) > 1:
            s.append(pc.poly(pts_, col, 2.8))
    t = lambda x: x.strftime("%H:%M") if x else ""
    real = [f for f in rec["fixes"] if f[3] is not None and f[4] is not None]
    for n, (kind, when, d, brg, ang) in enumerate(real):
        x, y = px(d), py(ang * 1000)
        s.append('<circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>' % (x, y, YEL))
        dy = -30 if n % 2 == 0 else 40
        s.append(pc.note(x + 10, y + dy, "%s A%d %s" % (kind, ang, t(when)), 14, "start") + pc.arrow(x + 12, y + dy + (6 if dy < 0 else -16), x + 3, y + (-6 if dy < 0 else 6)))
    if not tr and not real:
        s.append(pc.text(PW / 2, PH / 2, "no radar positions recorded for this approach", 15, MUTED, "middle"))
    s.append(pc.text(PW - 8, 18, "from the side, flown (Tacview)" if tr else "from the side, logged radar fixes", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def case1_stack_view(rec, brc, name, carrier=None):
    """Case I, from above, turned so the way in from the jet's check-in runs from the left (ship on the right). Rings every
    10 NM, the check-in fix, "see me at ten", and the stack overhead the ship."""
    fix = next((f for f in rec["fixes"] if f[3] is not None), None)
    bearing = fix[3] if fix else (brc or 90)
    far = 90
    off = 270 - bearing
    cx, cy = PW - 90, PH / 2 + 10
    k = (cx - 40) / float(far)

    def pt(brg, nm):
        a = math.radians(brg + off)
        return cx + nm * k * math.sin(a), cy - nm * k * math.cos(a)
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, PH)]
    if carrier:
        s.append(geo_layer(carrier, pt, far + 10))
    for nm in range(10, far + 1, 10):
        s.append('<circle cx="%d" cy="%d" r="%.1f" fill="none" stroke="#3b434d" stroke-width="1.2"/>' % (cx, cy, nm * k))
        s.append(pc.text(cx - nm * k, 44, "%d NM" % nm, 12, "#8b949e", "middle"))
    xf, yf = pt(bearing, far)
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#58a6ff" stroke-width="1.4" stroke-dasharray="6 5" opacity=".7"/>' % (cx, cy, xf, yf))
    nx, ny = 52, 118
    a = math.radians(off)
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#8b949e" stroke-width="2"/>' % (nx, ny, nx + 26 * math.sin(a), ny - 26 * math.cos(a)))
    s.append(pc.text(nx + 36 * math.sin(a), ny - 36 * math.cos(a) + 5, "N", 14, MUTED, "middle"))
    rot = (brc if brc is not None else 0) + off - 270
    s.append('<g transform="rotate(%.1f %d %d)">%s</g>' % (rot, cx, cy, pc.carrier_top(cx, cy, None)))
    if brc is not None:
        s.append(pc.text(cx, cy - 34, "BRC %03d" % (round(brc) % 360 or 360), 14, "#e6edf3", "middle", 'font-weight="bold"'))
    if fix:
        x, y = pt(fix[3], fix[2])
        if rec.get("track"):
            s.append(pc.poly([pt(brg_, d_) for _, d_, brg_, _ in rec["track"]], WHITE, 2.2))
        s.append('<circle cx="%.1f" cy="%.1f" r="6" fill="#1f6feb" stroke="%s" stroke-width="2"/>' % (x, y, YEL))
        s.append(pc.note(x, y - 30, "checked in %d NM, angels %d" % (fix[2], fix[4] or 0), 15, "middle") + pc.arrow(x, y - 20, x, y - 8))
        s.append(pc.text(x, y + 26, "%s %s" % (name(rec), rec["checkin"].strftime("%H:%M")), 13, "#e6edf3", "middle"))
    s.append(pc.text(PW - 8, 18, "from above, turned so the way in runs from the left", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def _case1_pattern(rec, mates, brc, name, W, H, far, detail):
    """Case I Stack View, from above and centred on the ship (BRC up): the stack overhead, the initial 3 NM astern, the
    break, downwind to port and the turn to the groove. Planned until real tracks are logged; rings every 1 NM."""
    cx, cy = W / 2, H / 2 + (10 if detail else 0)
    k = (min(W, H) / 2 - 14) / far
    b = brc if brc is not None else 0

    def xy(right_nm, ahead_nm):  # ship-relative: right of the BRC, ahead along the BRC
        return cx + right_nm * k, cy - ahead_nm * k
    s = []
    for nm in ((3, 5, 10) if far > 8 else [n for n in (0.5, 1, 2, 3, 4) if n < far]):
        s.append('<circle cx="%d" cy="%d" r="%.1f" fill="none" stroke="#3b434d" stroke-width="1.2"%s/>' % (cx, cy, nm * k, ' stroke-dasharray="3 5"' if nm == 3 else ""))
        s.append(pc.text(cx + nm * k * 0.72 + 4, cy - nm * k * 0.69, "%g NM" % nm, 12, "#8b949e", "start"))
    if far > 8:
        s.append('<circle cx="%d" cy="%d" r="%.1f" fill="none" stroke="#58a6ff" stroke-width="1.3" stroke-dasharray="3 3"/>' % (cx, cy, rec["zoom"] * k))
    s.append('<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#58a6ff" stroke-width="1.3" stroke-dasharray="6 5" opacity=".6"/>' % (cx, cy + far * k, cx, cy - far * k))
    # north arrow: north is (360 - BRC) degrees from up
    a = math.radians(-b)
    nx, ny = 52, 118
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#8b949e" stroke-width="2"/>' % (nx, ny, nx + 26 * math.sin(a), ny - 26 * math.cos(a)))
    s.append(pc.text(nx + 36 * math.sin(a), ny - 36 * math.cos(a) + 5, "N", 14, MUTED, "middle"))
    GREEN = "#3fb950"

    def bez(p0, p1, p2, p3, n=40):
        out = []
        for q in range(n + 1):
            t = q / float(n)
            u = 1 - t
            out.append(tuple(u**3 * p0[m] + 3 * u * u * t * p1[m] + 3 * u * t * t * p2[m] + t**3 * p3[m] for m in (0, 1)))
        return out
    # DimOn Case I card: port holding pattern, 5 NM max across, ship at position 1 on the circle (centre 2.5 NM to port),
    # min 2,000 ft, 1,000 ft separation; commence from position 3 on BRC+210 deg, then Initial 3 NM astern at 800 ft, 350 kts.
    CC, RX, RY = (-1.6, 0.0), 1.6, 2.5  # oval: 5 NM long on the BRC, 3.2 NM wide, ship at position 1
    # 1) the marshal stack (green dots), wide view only
    has_track, has_pat = bool(rec.get("track")), bool(rec.get("pattern"))
    if not detail and PLANNED:
        s.append('<g opacity="%s">' % (".3" if has_track else "1"))
        s.append(pc.poly([xy(CC[0] + RX * math.cos(math.radians(n * 5)), CC[1] + RY * math.sin(math.radians(n * 5))) for n in range(0, 73)], GREEN, 2.2, "7 5"))
        s.append("</g>")
        for i, m in enumerate(sorted(mates, key=lambda m: m["angels"] or 2) if not has_track else []):
            mine = m is rec
            t = math.radians(180 + 35 * i) if mine or True else 0
            x, y = xy(CC[0] + RX * math.cos(t), CC[1] + RY * math.sin(t))
            s.append('<circle cx="%.1f" cy="%.1f" r="%d" fill="%s" stroke="%s" stroke-width="1.5"/>' % (x, y, 5 if mine else 3.5, GREEN, YEL if mine else "#8b949e"))
            if mine:
                s.append(pc.text(x - 8, y - 8, "A%d %s" % (m["angels"] or 2, name(m)), 12, "#e6edf3", "end"))
        s.append(pc.note(W / 2, 36, "MARSHAL STACK: port holding, 5 NM max", 14, "middle"))
        s.append(pc.text(W / 2, 54, "angels 2-9, 1,000 ft apart, min 2,000 ft", 12, MUTED, "middle"))
        x3, y3 = xy(-3.2, 0.0)
        s.append(pc.text(x3 - 4, y3 + 22, "3: commence", 12, YEL, "end") + pc.text(x3 - 4, y3 + 36, "BRC+210", 12, YEL, "end"))
    # 2) commence (yellow): from position 3 on BRC+210 round to the Initial 3 NM astern, then inbound up the BRC
    com = bez((-3.2, 0.0), (-4.2, -2.5), (0.0, -4.5), (0.0, -3.0)) + [(0.0, 0.35)]
    if PLANNED:
        s.append('<g opacity="%s">' % (".3" if (has_track and not detail) else "1") + pc.poly([xy(*q) for q in (com if not detail else [(0.0, -3.0), (0.0, 0.35)])], YEL, 2.4, "9 5") + "</g>")
    # 3) Paddles (white dashes): break (level turn) to port, downwind 1.25 NM abeam, the 180, the 90, the 45, the groove
    r1 = 0.625
    paddles = []
    for i in range(0, 19):
        t = math.radians(i * 10)
        paddles.append((-r1 + r1 * math.cos(t), 0.35 + r1 * math.sin(t)))
    A, G = (-1.25, -0.3), (0.12, -0.5)
    turn = bez(A, (-1.25, -1.35), (0.15, -1.3), G, 40)
    paddles += [A] + turn[1:] + [(0.0, 0.0)]
    if PLANNED:
        s.append('<g opacity="%s">' % (".3" if has_pat and detail else "1") + pc.poly([xy(*q) for q in paddles], "#ffffff", 2.4, "7 5") + "</g>")
    # the real tracks: the Marshal stack track (Tacview, range / bearing from the carrier) on the wide view, green until the
    # commence and yellow after; the Paddles pass pattern (ship frame, metres) on the magnified view in solid white
    if not detail and has_track:
        com_t = rec["commence"].timestamp() if rec.get("commence") else None
        legs = ([], [])
        for ut, dist, brg, alt in rec["track"]:
            ang = math.radians(brg - b)
            legs[1 if (com_t and ut >= com_t) else 0].append((dist * math.sin(ang), dist * math.cos(ang)))
        if legs[0] and legs[1]:
            legs[0].append(legs[1][0])
        for pts_, col in ((legs[0], GREEN), (legs[1], YEL)):
            if len(pts_) > 1:
                s.append(pc.poly([xy(*q) for q in pts_], col, 2.6))
        ex, ey = xy(*(legs[1] or legs[0])[-1])
        s.append('<circle cx="%.1f" cy="%.1f" r="3.5" fill="#fff"/>' % (ex, ey))
        # the Marshal stack altitude: at the start, at each 500 ft change, and at the end of the track
        last_alt, shown = None, []
        trk = rec["track"]
        for i, (ut, dist, brg, alt) in enumerate(trk):
            if last_alt is None or abs(alt - last_alt) >= 500 or i == len(trk) - 1:
                if not shown or math.hypot(*(a - b_ for a, b_ in zip(xy(dist * math.sin(math.radians(brg - b)), dist * math.cos(math.radians(brg - b))), shown[-1]))) > 30:
                    px_, py_ = xy(dist * math.sin(math.radians(brg - b)), dist * math.cos(math.radians(brg - b)))
                    shown.append((px_, py_))
                    s.append('<circle cx="%.1f" cy="%.1f" r="2.5" fill="#fff"/>' % (px_, py_)
                             + pc.text(px_ + 5, py_ - 5, "%s ft" % format(int(round(alt, -2)), ","), 11, "#e6edf3", "start"))
                last_alt = alt
    if detail and has_pat:
        s.append(pc.poly([xy(-pt[1] / M_NM, -pt[0] / M_NM) for pt in rec["pattern"]], "#ffffff", 2.8))
    sc = 0.3 if detail else 0.12
    s.append('<g transform="translate(%d %d) scale(%.2f) translate(%d %d) rotate(-270 %d %d)">%s</g>' % (cx, cy + 34 * sc, sc, -cx, -cy, cx, cy, pc.carrier_top(cx, cy, None)))
    if detail:
        s.append(pc.text(W - 10, 52, "ABE  BRC %03d" % (round(b) % 360 or 360), 14, "#e6edf3", "end", 'font-weight="bold"'))
        if has_pat:  # the numbers the pilot actually flew: speed and height over the ship, the break, downwind, abeam, roll-out
            pat = rec["pattern"]
            pts_ = [(*xy(-row[1] / M_NM, -row[0] / M_NM), row[2]) for row in pat]
            rings_ = [(cx + n * k * 0.72 + 2, cy - n * k * 0.69 - 14, cx + n * k * 0.72 + 52, cy - n * k * 0.69 + 5) for n in (0.5, 1, 2, 3, 4) if n < far]
            s += pc.callouts(pat, pts_, [(W - 190, 40, W, 62), (-600, -600, 4, 2000), (W - 4, -600, W + 600, 2000)] + rings_)  # the last two keep notes on the panel
        else:
            s.append(pc.text(W / 2, H / 2, "no pattern track recorded", 15, MUTED, "middle"))
    else:
        for nm in (3,):
            pass
        if not has_track:
            s.append(pc.text(W / 2, H - 40, "no stack track recorded", 13, MUTED, "middle"))
    kx, ky = 20, H - 50
    for n, (col, lab) in enumerate(((GREEN, "in the stack (Tacview)"), (YEL, "commencing"), ("#ffffff", "Paddles pattern"))):
        if detail:
            yy = ky + n * 20
            s.append('<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2.6"/>' % (kx, yy, kx + 34, yy, col) + pc.text(kx + 46, yy + 4, lab, 12, "#c9d1d9", "start"))
    return "".join(s)


def case1_pattern_view(rec, mates, brc, name):
    """Case I Stack View: left, 10 NM around the ship (BRC up); right, the close-in pattern magnified (4 NM)."""
    lw = 330
    pat = rec.get("pattern") or []
    rec["zoom"] = min(4.2, max(2.0, max([max(abs(r[0]), abs(r[1])) for r in pat] + [0]) / M_NM + 0.6))
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, PH)]
    s.append('<svg x="0" y="0" width="%d" height="%d">%s</svg>' % (lw, PH, _case1_pattern(rec, mates, brc, name, lw, PH, 11, False)))
    s.append('<line x1="%d" y1="34" x2="%d" y2="%d" stroke="#30363d"/>' % (lw, lw, PH - 6))
    s.append('<svg x="%d" y="0" width="%d" height="%d">%s</svg>' % (lw, PW - lw, PH, _case1_pattern(rec, mates, brc, name, PW - lw, PH, rec["zoom"], True)))
    s.append(pc.text(lw / 2, PH - 8, "10 NM around ABE", 12, MUTED, "middle"))
    s.append(pc.text(PW - 14, PH - 8, "magnified: %g NM around ABE" % round(rec["zoom"], 1), 12, MUTED, "end"))
    s.append(pc.text(PW - 8, 18, "from above, ABE centred, BRC up - %s" % ("flown track (Tacview)" if rec.get("track") or rec.get("pattern") else "no track recorded"), 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def case1_side_view(rec, mates, name):
    """Case I, from the side: the stack overhead the ship (angels 2 up), 'see me at ten', and, after Charlie, the way out to
    the initial 3 NM astern at 800 ft and the break over the ship."""
    far = 12
    x0, base = 60, PH - 40
    kx, ky = (PW - x0 - 30) / float(far), (PH - 90) / 10000.0

    def px(nm): return x0 + nm * kx
    def py(ft): return base - ft * ky
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, PH), pc.ship_side(8, base)]
    for ft in (2000, 4000, 6000, 8000, 10000):
        s.append('<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#21262d"/>' % (x0, py(ft), PW - 10, py(ft)))
        s.append(pc.text(PW - 12, py(ft) - 4, "angels %d" % (ft // 1000), 12, "#6e7681", "end"))
    for nm in (3, 6, 9, 12):
        s.append('<line x1="%.1f" y1="%d" x2="%.1f" y2="%d" stroke="#262d36"/>' % (px(nm), 40, px(nm), base))
        s.append(pc.text(px(nm), base + 18, "%d NM" % nm, 12, "#8b949e", "middle"))
    angels = rec["angels"] or 2
    for m in mates:  # the stack: a bar for each jet overhead the ship
        a = m["angels"] or 2
        mine = m is rec
        s.append('<rect x="%.1f" y="%.1f" width="46" height="6" fill="%s"/>' % (px(0) + 10, py(a * 1000) - 3, "#58a6ff" if mine else "#484f58"))
        s.append(pc.text(px(0) + 10, py(a * 1000) - 10, "A%d %s" % (a, name(m)), 14 if mine else 11, "#e6edf3" if mine else MUTED))
    s.append(pc.poly([(px(12), py(angels * 1000)), (px(0) + 56, py(angels * 1000))], WHITE, 2.6))  # level to the stack
    s.append('<circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>' % (px(10), py(angels * 1000), YEL) + pc.note(px(10), py(angels * 1000) - 36, "see me at ten", 15, "middle") + pc.arrow(px(10), py(angels * 1000) - 26, px(10), py(angels * 1000) - 7))
    s.append(pc.poly([(px(0) + 56, py(angels * 1000)), (px(3), py(800))], YEL, 2, "6 5"))  # after Charlie: out to the initial
    s.append('<circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>' % (px(3), py(800), YEL) + pc.note(px(3) + 12, py(800) - 24, "Charlie, then initial 3 NM, 800 ft", 15, "start") + pc.arrow(px(3) + 10, py(800) - 18, px(3) + 2, py(800) - 6))
    s.append(pc.poly([(px(3), py(800)), (px(0) + 6, py(800))], WHITE, 2.6))
    s.append(pc.note(px(0) + 90, py(800) + 40, "break over the ship, then Paddles", 14, "start") + pc.arrow(px(0) + 88, py(800) + 32, px(0) + 30, py(800) + 2))
    s.append(pc.text(PW - 8, 18, "from the side, planned: level, stack, then Charlie and the initial", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def card_html(rec, mates, label, name, brc, ctx=None, result=None, carrier=None):
    case1 = rec["case"] != "III"
    badge = "Case " + (rec["case"] or "III")
    t = lambda x: x.strftime("%H:%M") if x else "-"
    eat_line = "approach time %s" % t(rec["eat"]) + (" (re-issued: it had passed)" if rec["eat_missed"] else "")
    weather = ""
    if ctx and ctx.get("ceiling") is not None:
        ceil, vis = ctx["ceiling"], ctx.get("vis") or 0
        weather = "Weather: %s, visibility %s" % ("ceiling unrestricted" if ceil >= 20000 or ceil <= 0 else "ceiling %s ft" % format(int(round(ceil, -2)), ","), "10+ NM" if vis >= 10 else "%.0f NM" % vis)
    if case1:
        fix = next((f for f in rec["fixes"] if f[3] is not None), None)
        weather = "Weather: %s, visibility %s" % ("ceiling %s ft" % format(int(round(rec["ceiling"], -2)), ",") if rec.get("ceiling") and rec["ceiling"] < 20000 else "ceiling unrestricted",
                                                  "10+ NM" if (rec.get("vis") or 0) >= 10 else "%.0f NM" % (rec.get("vis") or 0)) if rec.get("ceiling") is not None else ""
        lines = ['Nimitz Class "ABE" | Marshal | %s server' % rec["rig"], rec["checkin"].strftime("%A %B %d, %Y %H:%M"), *([weather] if weather else []),
                 "BRC %s | Case %s" % ("%03d" % (round(brc) % 360 or 360) if brc is not None else "-", rec["case"] or "I"), "",
                 *(["No Marshal check-in on this approach", "Pattern flown from the Paddles track (Tacview), landing %s" % t(rec["checkin"])] if rec.get("no_marshal") else
                   ["Marshal assigned angels %d" % (rec["angels"] or 2),
                    "Checked in %s%s" % (t(rec["checkin"]), " at %d NM on the %03d bearing, angels %d" % (fix[2], fix[3], fix[4] or 0) if fix else "")]), "",
                 result or "", "(example position: the carrier is placed off the coast until Marshal logs its real position)" if carrier == EXAMPLE_CARRIER else ""]
        body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
        return _page(badge, label, body, "Marshal View", "Stack View", case1_stack_view(rec, brc, name, carrier), case1_pattern_view(rec, mates, brc, name))
    lines = ['Nimitz Class "ABE" | Marshal | %s server' % rec["rig"], rec["checkin"].strftime("%A %B %d, %Y"),
             *([weather] if weather else []), "BRC %s | Case %s" % ("%03d" % (round(brc) % 360 or 360) if brc is not None else "-", rec["case"] or "III"), "",
             "Marshal radial %03d, %d DME, angels %d, %s" % (round(rec["radial"]) % 360 or 360, (rec["angels"] or 6) + 15, rec["angels"] or 6, eat_line),
             "Checked in %s | in the hold %s | commenced %s | platform %s | dirty up %s | Paddles %s" % (
                 t(rec["checkin"]), t(rec["hold"]), t(rec["commence"]), t(rec["platform"]), t(rec["dirty"]), t(rec["handoff"])), "",
             result or "", "(no radar positions were logged for this approach)" if not rec["fixes"] and not rec.get("track") else "",
             "(example position: the carrier is placed off the coast until Marshal logs its real position)" if carrier == EXAMPLE_CARRIER else ""]
    body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
    return _page(badge, label, body, "Stack View", "Commence View", stack_view(rec, mates, brc, name, carrier), commence_view(rec, mates, name))


def _page(badge, label, body, tab1, tab2, left, right):
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:14px 16px;width:1696px}
.top{display:flex;align-items:center;gap:18px;margin-bottom:10px}
.b{min-width:96px;height:58px;line-height:58px;text-align:center;border-radius:7px;font:700 30px Arial,sans-serif;background:#0a8fd8;color:#fff;padding:0 14px}
.n{font:700 28px Arial,sans-serif;color:#e6edf3}.logo{margin-left:auto;height:92px;width:auto;filter:drop-shadow(0 0 2px #fff) drop-shadow(0 0 5px rgba(255,255,255,.55))}
.ps{display:flex;gap:16px}.pn{flex:none;width:840px;background:#0d1117;border:1px solid #30363d;border-radius:6px;overflow:hidden}
.tab{background:#1f6feb;color:#fff;font:700 22px Arial,sans-serif;text-align:center;padding:9px 0}
.info{margin-top:14px;font-size:20px;line-height:1.35;color:#e6edf3}
.ft{font:12px Arial,sans-serif;color:#8b949e;margin-top:8px}
</style></head><body><div class="w"><div class="top"><div class="b">%s</div><div class="n">%s</div><img class="logo" src="%s"></div>
<div class="ps"><div class="pn"><div class="tab">%s</div>%s</div><div class="pn"><div class="tab">%s</div>%s</div></div>
<div class="info">%s</div><div class="ft">VMFA-225 Marshal</div></div></body></html>""" % (
        html.escape(badge), html.escape(label + " | Marshal"), pc.LOGO, tab1, left, tab2, right, body)


def stack_track(rec, events):
    """[(unix, rangeNm, bearingMag, altFt)] this jet flew through Marshal, from the "Marshal: stack track" log lines
    (Marshal samples every jet in the stack every 10 s from Tacview), from check-in to the hand-off."""
    t0 = rec["checkin"].timestamp() - 60
    t1 = (rec["handoff"] or rec["checkin"] + dt.timedelta(minutes=45)).timestamp() + 60
    out = []
    for t, rig, msg, o in events:
        if rig != rec["rig"] or msg != "Marshal: stack track" or (o.get("callsign") or "").lower() != rec["callsign"].lower():
            continue
        for seg in (o.get("track") or "").split("|"):
            try:
                ut, d, brg, alt = (float(x) for x in seg.split(":"))
            except ValueError:
                continue
            if t0 <= ut <= t1:
                out.append((ut, d, brg, alt))
    out.sort()
    return out if len(out) >= 4 else []


def pass_pattern(rec):
    """[[metres astern, metres to port, ft, kts]] from the Paddles "pass pattern" log line for the pass that followed."""
    p = rec.get("pass")
    if not p or not p.get("rig"):
        return None
    rows = pc.read_logged({p["t"].date().isoformat()}, "Paddles: pass pattern", "pattern")
    return pc.match(p, rows)


def build(rec, c3, events, passes, carrier=None):
    names = {cs: label for cs, label, _, _ in gp.roster()}
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    key = canon.get(rec["callsign"].lower(), rec["callsign"].lower())
    name = lambda r: names.get(canon.get(r["callsign"].lower(), r["callsign"].lower()), r["callsign"]).split(" | ")[-1]
    mates = [r for r in c3 if r["rig"] == rec["rig"] and abs((r["checkin"] - rec["checkin"]).total_seconds()) < 3600]
    rec["track"] = stack_track(rec, events)
    rec["pattern"] = pass_pattern(rec)
    p = rec.get("pass")
    result = ("Paddles: %s%s" % (p["grade"], ", %d wire" % p["wire"] if p.get("wire") and not p["grade"].startswith("WO") else "")) if p else "No Paddles pass seen"
    ctx = {"ceiling": rec.get("ceiling"), "vis": rec.get("vis")}
    return card_html(rec, mates, names.get(key, rec["callsign"]), name, rec["brc"], ctx, result, carrier or rec.get("carrier") or EXAMPLE_CARRIER)


def post_cards(pngs):
    """Post each card as a new message to the Marshal cards' own channel (the pass cards' channel if there is none)."""
    f = next((x for x in (gp.MARSHAL_WEBHOOK_FILE, gp.CARDS_WEBHOOK_FILE) if os.path.exists(x)), gp.WEBHOOK_FILE)
    hook = open(f, encoding="utf-8").read().strip()
    for text, png in pngs:
        gp.send(hook + "?wait=true", "POST", png, text)
        print("posted", text)


POSTED = os.path.join(gp.REPORTS, "_data", "paddles", "marshal-cards-posted.json")
SETTLE = dt.timedelta(minutes=10)  # an approach is carded once its hand-off (or last call) is this old, so the Paddles pass is in


def post_new():
    """Post a card for every finished approach not posted yet (greenie_watch runs this after a recovery goes quiet).
    The first run only records what already exists, so the backlog is not posted."""
    mr.fetch(3)
    for back in (0, 1):
        gp.fetch_today((dt.date.today() - dt.timedelta(days=back)).isoformat())
    events = mr.read_events(3)
    recs = mr.approaches(events)
    mr.link_passes(recs, gp.load_passes(dt.date.today()))
    done_ = {}
    first = not os.path.exists(POSTED)
    if not first:
        done_ = json.load(open(POSTED, encoding="utf-8"))
    now = dt.datetime.now(recs[0]["checkin"].tzinfo) if recs else dt.datetime.now()
    c3 = [r for r in recs if r["case"] == "III" and r.get("radial") is not None]
    c1 = [r for r in recs if r["case"] in ("I", "II") and r["fixes"]]
    out = []
    for group in (c3, c1):
        for rec in group:
            key = "%s|%s|%s" % (rec["rig"], rec["callsign"].lower(), rec["checkin"].isoformat())
            last = rec["handoff"] or rec["commence"] or rec["checkin"]
            if key in done_ or now - last < SETTLE:
                continue
            done_[key] = rec["checkin"].isoformat()
            if first:
                continue
            png = gp.screenshot(build(rec, group, events, gp.load_passes(dt.date.today())), 1010, 1760)
            out.append(("Marshal | %s | %s | Case %s" % (rec["callsign"], rec["checkin"].strftime("%Y-%m-%d %H:%M"), rec["case"]), png))
    if out:
        post_cards(out)
    json.dump(done_, open(POSTED, "w", encoding="utf-8"))
    print("marshal cards: %d posted%s" % (len(out), " (first run: recorded the backlog only)" if first else ""))
    return 0


def main():
    if "--post-new" in sys.argv:
        return post_new()
    mr.fetch(10)
    for back in (0, 1):  # the Paddles logs (pass pattern lines) for today and yesterday, from every rig
        gp.fetch_today((dt.date.today() - dt.timedelta(days=back)).isoformat())
    events = mr.read_events(10)
    recs = mr.approaches(events)
    passes = gp.load_passes(dt.date.today())
    mr.link_passes(recs, passes)
    c3 = [r for r in recs if r["case"] == "III" and r.get("radial") is not None]
    c1 = [r for r in recs if r["case"] in ("I", "II") and r["fixes"]]
    out_dir = os.path.join(gp.REPORTS, "_data", "paddles")
    done, posts = 0, []
    for rec, group, fname in ((c3[-1] if c3 else None, c3, "sample-marshal-card.png"), (c1[-1] if c1 else None, c1, "sample-marshal-card-case1.png")):
        if not rec:
            continue
        png = gp.screenshot(build(rec, group, events, passes), 1010, 1760)
        open(os.path.join(out_dir, fname), "wb").write(png)
        print(os.path.join(out_dir, fname))
        done += 1
        posts.append(("Marshal | %s | %s | Case %s" % (rec["callsign"], rec["checkin"].strftime("%Y-%m-%d %H:%M"), rec["case"]), png))
    if "--post" in sys.argv and posts:
        post_cards(posts)
    return 0 if done else 1


if __name__ == "__main__":
    sys.exit(main())
