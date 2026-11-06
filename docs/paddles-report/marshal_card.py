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
    # the stack: every jet at its hold, with the racetrack it flies (about 4 NM long, 2 NM wide, on the radial)
    for m in mates:
        ang = m["angels"] or 6
        mine = m is rec
        real = [f for f in m["fixes"] if f[3] is not None]
        hold = next((f for f in real if f[0] == "hold"), None)
        x, y = pt(hold[3], hold[2]) if hold else pt(m["radial"], ang + 15)
        d = (hold[2] if hold else ang + 15)
        r1, r2 = pt(m["radial"], d - 2.5), pt(m["radial"], d + 2.5)
        width = 1.0 * k
        s.append('<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" fill="none" stroke="%s" stroke-width="1.3" stroke-dasharray="4 4" opacity=".8"/>' % (
            min(r1[0], r2[0]), cy - width - (cy - y) * 0 if False else y - width, abs(r1[0] - r2[0]), 2 * width, width, "#58a6ff" if mine else "#484f58"))
        s.append('<circle cx="%.1f" cy="%.1f" r="%d" fill="%s" stroke="%s" stroke-width="2"/>' % (x, y, 11 if mine else 7, "#1f6feb" if mine else "#30363d", YEL if mine else "#8b949e"))
        s.append(pc.text(x, y - 24 if mine else y - 18, "A%d %s" % (ang, name(m)), 15 if mine else 12, "#e6edf3" if mine else MUTED, "middle"))
        if mine:
            s.append(pc.note(x, y + 46, "%d DME" % d, 15, "middle") + pc.arrow(x, y + 32, x, y + 13))
            if m["eat"]:
                s.append(pc.text(x, y + 66, "approach time %s%s" % (m["eat"].strftime("%H:%M"), " (re-issued)" if m["eat_missed"] else ""), 12, MUTED, "middle"))
    # the way in for this jet, with the minutes between the steps
    hold_nm = (rec["angels"] or 6) + 15
    real = [f for f in rec["fixes"] if f[3] is not None]
    if len(real) > 1:
        s.append(pc.poly([pt(f[3], f[2]) for f in real], WHITE, 2.4))
    steps = [(hold_nm - 1, "commence", rec["commence"]), (14, "platform", rec["platform"]), (8, "dirty up", rec["dirty"]), (3, "Paddles", rec["handoff"])]
    prev = None
    for i, (nm, lab, t) in enumerate(steps):
        x, y = pt(radial, nm)
        s.append('<circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>' % (x, y, YEL))
        label = "%s %s" % (lab, t.strftime("%H:%M") if t else "%d NM" % nm)
        up = i % 2 == 0
        s.append(pc.note(x, y + (-44 if up else 58), label, 15, "middle") + pc.arrow(x, y + (-34 if up else 44), x, y + (-7 if up else 8)))
        if prev and prev[2] and t and mins(prev[2], t) is not None:
            px_, py_ = pt(radial, (prev[0] + nm) / 2.0)
            s.append(pc.text(px_, py_ + 18, "+%d min" % round(mins(prev[2], t)), 12, MUTED, "middle"))
        prev = (nm, lab, t)
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
    for off, col in ((0.5, YEL), (0.0, GRN), (-0.7, RED)):  # the glideslope and its tolerance lines, like the groove view
        s.append(pc.poly([(px(0), py(0)), (px(17), py(gs_at(17, off)))], col, 1.5, "5 4"))
    s.append(pc.text(px(16.5), py(gs_at(17)) - 10, "3.5 deg glideslope", 12, GRN, "end"))
    for m in mates:  # the stack levels
        ang = m["angels"] or 6
        mine = m is rec
        x, y = px(ang + 15), py(ang * 1000)
        s.append('<rect x="%.1f" y="%.1f" width="%d" height="5" fill="%s"/>' % (x - 20, y - 2, 40, "#58a6ff" if mine else "#484f58"))
        if not mine:
            s.append(pc.text(x, y - 10, "A%d %s" % (ang, name(m)), 11, MUTED, "middle"))
    hold_nm, ang = (rec["angels"] or 6) + 15, rec["angels"] or 6
    cross = PLATFORM_FT / gs_at(1.0)  # where the glideslope reaches the platform altitude
    prof = [(px(hold_nm), py(ang * 1000)), (px(hold_nm - 1), py(PLATFORM_FT)), (px(cross), py(PLATFORM_FT))] + [(px(d), py(gs_at(d))) for d in (12, 8, 3, 0)]
    s.append(pc.poly(prof, WHITE, 2.8))
    t = lambda x: x.strftime("%H:%M") if x else ""
    marks = [(hold_nm, ang * 1000, "hold A%d, %d DME" % (ang, hold_nm), rec["hold"], 16, -34, "start"),
             (hold_nm - 1, PLATFORM_FT, "commence", rec["commence"], 6, 52, "middle"),
             (cross + 0.4, PLATFORM_FT, "platform 5,000 ft", rec["platform"], 0, 52, "middle"),
             (8, gs_at(8), "dirty up 8 NM, %s ft" % format(int(round(gs_at(8), -1)), ","), rec["dirty"], 20, 40, "start"),
             (3, gs_at(3), "Paddles 3 NM, %s ft" % format(int(round(gs_at(3), -1)), ","), rec["handoff"], 20, 40, "start")]
    for nm, alt, lab, when, dx, dy, anchor in marks:
        x, y = px(nm), py(alt)
        s.append('<circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>' % (x, y, YEL))
        s.append(pc.note(x + dx, y + dy, lab + (" " + t(when) if when else ""), 14, anchor) + pc.arrow(x + dx * 0.6, y + dy + (6 if dy < 0 else -16), x + (3 if dy > 0 else 4), y + (7 if dy > 0 else -6)))
    if rec["commence"] and rec["dirty"]:
        s.append(pc.text(px(5.5), py(gs_at(5.5)) + 30, "%d min from commence to dirty up" % round(mins(rec["commence"], rec["dirty"])), 12, MUTED, "start"))
    s.append(pc.text(PW - 8, 18, "from the side, planned profile", 12, MUTED, "end"))
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
        s.append(pc.poly([(x, y), (cx, cy)], WHITE, 2.2))
        s.append('<circle cx="%.1f" cy="%.1f" r="6" fill="#1f6feb" stroke="%s" stroke-width="2"/>' % (x, y, YEL))
        s.append(pc.note(x, y - 30, "checked in %d NM, angels %d" % (fix[2], fix[4] or 0), 15, "middle") + pc.arrow(x, y - 20, x, y - 8))
        s.append(pc.text(x, y + 26, "%s %s" % (name(rec), rec["checkin"].strftime("%H:%M")), 13, "#e6edf3", "middle"))
    x10, y10 = pt(bearing, 10)
    s.append('<circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>' % (x10, y10, YEL) + pc.note(x10, y10 + 50, "see me at ten", 15, "middle") + pc.arrow(x10, y10 + 38, x10, y10 + 8))
    s.append(pc.note(cx - 40, cy - 70, "stack overhead, angels %d" % (rec["angels"] or 2), 15, "end") + pc.arrow(cx - 36, cy - 60, cx - 14, cy - 12))
    s.append(pc.text(PW - 8, 18, "from above, turned so the way in runs from the left", 12, MUTED, "end"))
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
                 "Marshal at angels %d, report see me at ten" % (rec["angels"] or 2),
                 "Checked in %s%s" % (t(rec["checkin"]), " at %d NM on the %03d bearing, angels %d" % (fix[2], fix[3], fix[4] or 0) if fix else ""), "",
                 result or "", "(example position: the carrier is placed off the coast until Marshal logs its real position)" if carrier == EXAMPLE_CARRIER else ""]
        body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
        return _page(badge, label, body, "Marshal View", "Stack View", case1_stack_view(rec, brc, name, carrier), case1_side_view(rec, mates, name))
    lines = ['Nimitz Class "ABE" | Marshal | %s server' % rec["rig"], rec["checkin"].strftime("%A %B %d, %Y"),
             *([weather] if weather else []), "BRC %s | Case %s" % ("%03d" % (round(brc) % 360 or 360) if brc is not None else "-", rec["case"] or "III"), "",
             "Marshal radial %03d, %d DME, angels %d, %s" % (round(rec["radial"]) % 360 or 360, (rec["angels"] or 6) + 15, rec["angels"] or 6, eat_line),
             "Checked in %s | in the hold %s | commenced %s | platform %s | dirty up %s | Paddles %s" % (
                 t(rec["checkin"]), t(rec["hold"]), t(rec["commence"]), t(rec["platform"]), t(rec["dirty"]), t(rec["handoff"])), "",
             result or "", "(positions: the assigned radial and DME, and the planned profile - real radar fixes appear once Marshal logs them)" if not rec["fixes"] else "",
             "(example position: the carrier is placed off the coast until Marshal logs its real position)" if carrier == EXAMPLE_CARRIER else ""]
    body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
    return _page(badge, label, body, "Stack View", "Commence View", stack_view(rec, mates, brc, name, carrier), commence_view(rec, mates, name))


def _page(badge, label, body, tab1, tab2, left, right):
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:14px 16px;width:1696px}
.top{display:flex;align-items:center;gap:18px;margin-bottom:10px}
.b{min-width:96px;height:58px;line-height:58px;text-align:center;border-radius:7px;font:700 30px Arial,sans-serif;background:#0a8fd8;color:#fff;padding:0 14px}
.n{font:700 28px Arial,sans-serif;color:#e6edf3}.logo{margin-left:auto;height:92px;width:auto}
.ps{display:flex;gap:16px}.pn{flex:none;width:840px;background:#0d1117;border:1px solid #30363d;border-radius:6px;overflow:hidden}
.tab{background:#1f6feb;color:#fff;font:700 22px Arial,sans-serif;text-align:center;padding:9px 0}
.info{margin-top:14px;font-size:20px;line-height:1.35;color:#e6edf3}
.ft{font:12px Arial,sans-serif;color:#8b949e;margin-top:8px}
</style></head><body><div class="w"><div class="top"><div class="b">%s</div><div class="n">%s</div><img class="logo" src="%s"></div>
<div class="ps"><div class="pn"><div class="tab">%s</div>%s</div><div class="pn"><div class="tab">%s</div>%s</div></div>
<div class="info">%s</div><div class="ft">VMFA-225 Marshal</div></div></body></html>""" % (
        html.escape(badge), html.escape(label + " | Marshal"), pc.LOGO, tab1, left, tab2, right, body)


def build(rec, c3, events, passes, carrier=None):
    names = {cs: label for cs, label, _, _ in gp.roster()}
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    key = canon.get(rec["callsign"].lower(), rec["callsign"].lower())
    name = lambda r: names.get(canon.get(r["callsign"].lower(), r["callsign"].lower()), r["callsign"]).split(" | ")[-1]
    mates = [r for r in c3 if r["rig"] == rec["rig"] and abs((r["checkin"] - rec["checkin"]).total_seconds()) < 3600]
    p = rec.get("pass")
    result = ("Paddles: %s%s" % (p["grade"], ", %d wire" % p["wire"] if p.get("wire") and not p["grade"].startswith("WO") else "")) if p else "No Paddles pass seen"
    ctx = {"ceiling": rec.get("ceiling"), "vis": rec.get("vis")}
    return card_html(rec, mates, names.get(key, rec["callsign"]), name, rec["brc"], ctx, result, carrier or rec.get("carrier") or EXAMPLE_CARRIER)


def main():
    mr.fetch(10)
    events = mr.read_events(10)
    recs = mr.approaches(events)
    passes = gp.load_passes(dt.date.today())
    mr.link_passes(recs, passes)
    c3 = [r for r in recs if r["case"] == "III" and r.get("radial") is not None]
    c1 = [r for r in recs if r["case"] in ("I", "II") and r["fixes"]]
    out_dir = os.path.join(gp.REPORTS, "_data", "paddles")
    done = 0
    for rec, group, fname in ((c3[-1] if c3 else None, c3, "sample-marshal-card.png"), (c1[-1] if c1 else None, c1, "sample-marshal-card-case1.png")):
        if not rec:
            continue
        png = gp.screenshot(build(rec, group, events, passes), 1010, 1760)
        open(os.path.join(out_dir, fname), "wb").write(png)
        print(os.path.join(out_dir, fname))
        done += 1
    return 0 if done else 1


if __name__ == "__main__":
    sys.exit(main())
