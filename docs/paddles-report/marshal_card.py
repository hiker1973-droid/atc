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
import html
import math
import os
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


def stack_view(rec, mates, brc, name):
    """From above, turned so the marshal radial runs left across the panel (ship on the right): the rings, every jet at its
    hold, and the way in. A small arrow shows where north is."""
    radial = rec["radial"]
    off = 270 - radial  # the radial points left on the screen
    cx, cy = PW - 130, PH / 2 + 10
    k = (cx - 50) / float(RANGE)

    def pt(brg, nm):
        a = math.radians(brg + off)
        return cx + nm * k * math.sin(a), cy - nm * k * math.cos(a)
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, PH)]
    for nm in (10, 20, 30):
        s.append('<circle cx="%d" cy="%d" r="%.1f" fill="none" stroke="#30363d" stroke-width="1.2"/>' % (cx, cy, nm * k))
        s.append(pc.text(cx - nm * k, 44, "%d NM" % nm, 13, "#8b949e", "middle"))
    x30, y30 = pt(radial, RANGE)
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#58a6ff" stroke-width="1.4" stroke-dasharray="6 5" opacity=".7"/>' % (cx, cy, x30, y30))
    s.append(pc.text(x30 + 6, y30 - 8, "%03d radial" % (round(radial) % 360 or 360), 13, "#58a6ff", "start"))
    # north arrow
    nx, ny = 50, 70
    a = math.radians(off)
    s.append('<line x1="%d" y1="%d" x2="%.1f" y2="%.1f" stroke="#8b949e" stroke-width="2"/>' % (nx, ny, nx + 26 * math.sin(a), ny - 26 * math.cos(a)))
    s.append(pc.text(nx + 36 * math.sin(a), ny - 36 * math.cos(a) + 5, "N", 14, MUTED, "middle"))
    # the carrier, along its BRC
    rot = (brc if brc is not None else 0) + off - 90
    s.append('<g transform="translate(%d %d) rotate(%.1f)"><path d="M22 0 L-12 -9 L-18 -9 L-18 9 L-12 9 Z" fill="#8b949e"/></g>' % (cx, cy, rot))
    if brc is not None:
        s.append(pc.text(cx, cy - 34, "BRC %03d" % (round(brc) % 360 or 360), 14, "#e6edf3", "middle", 'font-weight="bold"'))
    # the stack: every jet at its hold
    for m in mates:
        ang = m["angels"] or 6
        real = [f for f in m["fixes"] if f[3] is not None]
        hold = next((f for f in real if f[0] == "hold"), None)
        x, y = pt(hold[3], hold[2]) if hold else pt(m["radial"], ang + 15)
        mine = m is rec
        s.append('<circle cx="%.1f" cy="%.1f" r="%d" fill="%s" stroke="%s" stroke-width="2"/>' % (x, y, 11 if mine else 7, "#1f6feb" if mine else "#30363d", YEL if mine else "#8b949e"))
        eat = m["eat"].strftime("%H:%M") if m["eat"] else ""
        s.append(pc.text(x, y - 20 if mine else y - 14, "A%d %s" % (ang, name(m)), 15 if mine else 12, "#e6edf3" if mine else MUTED, "middle"))
        if eat:
            s.append(pc.text(x, y + 28 if mine else y + 22, "approach time %s" % eat, 13 if mine else 11, MUTED, "middle"))
    # the way in for this jet: commence, platform, dirty up, hand-off
    hold_nm = (rec["angels"] or 6) + 15
    real = [f for f in rec["fixes"] if f[3] is not None]
    if len(real) > 1:
        s.append(pc.poly([pt(f[3], f[2]) for f in real], WHITE, 2.4))
    for i, (nm, lab) in enumerate(((hold_nm - 1, "commence"), (14, "platform"), (8, "dirty up"), (3, "Paddles"))):
        x, y = pt(radial, nm)
        s.append('<circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>' % (x, y, YEL))
        label = "%s %s" % (lab, "%d NM" % nm if lab != "commence" else (rec["commence"].strftime("%H:%M") if rec["commence"] else ""))
        s.append(pc.note(x, y + (-40 if i % 2 == 0 else 50), label, 15, "middle"))
        s.append('<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1" opacity=".5"/>' % (x, y + (-30 if i % 2 == 0 else 38), x, y + (-6 if i % 2 == 0 else 6), YEL))
    s.append(pc.text(PW - 8, 18, "from above, turned so the radial runs left", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def commence_view(rec, mates, name):
    x0, base = 60, PH - 40
    kx, ky = (PW - x0 - 30) / float(RANGE), (PH - 90) / 8000.0

    def px(nm): return x0 + nm * kx
    def py(ft): return base - ft * ky
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (PW, PH), pc.ship_side(8, base)]
    for ft in (2000, 4000, 6000, 8000):
        s.append('<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#21262d"/>' % (x0, py(ft), PW - 10, py(ft)))
        s.append(pc.text(PW - 12, py(ft) - 4, "%d ft" % ft, 12, "#6e7681", "end"))
    for nm in (10, 20, 30):
        s.append('<line x1="%.1f" y1="%d" x2="%.1f" y2="%d" stroke="#21262d"/>' % (px(nm), 40, px(nm), base))
        s.append(pc.text(px(nm), base + 18, "%d NM" % nm, 12, "#6e7681", "middle"))
    gs = [(px(d), py(d * M_NM * math.tan(math.radians(GS_DEG)) * 3.28084)) for d in range(0, 17)]
    s.append(pc.poly(gs, YEL, 1.6, "5 4"))
    s.append(pc.text(px(15.5), py(15.5 * M_NM * math.tan(math.radians(GS_DEG)) * 3.28084) - 8, "3.5 deg glideslope", 12, YEL, "end"))
    for m in mates:  # the stack levels
        ang = m["angels"] or 6
        mine = m is rec
        x, y = px(ang + 15), py(ang * 1000)
        s.append('<rect x="%.1f" y="%.1f" width="%d" height="5" fill="%s"/>' % (x - 18, y - 2, 36, "#58a6ff" if mine else "#484f58"))
        s.append(pc.text(x, y - 9, "A%d %s" % (ang, name(m)), 13 if mine else 11, "#e6edf3" if mine else MUTED, "middle"))
    hold_nm, ang = (rec["angels"] or 6) + 15, rec["angels"] or 6
    gs_at = lambda nm: nm * M_NM * math.tan(math.radians(GS_DEG)) * 3.28084
    cross = PLATFORM_FT / (M_NM * math.tan(math.radians(GS_DEG)) * 3.28084)  # where the glideslope reaches the platform
    prof = [(px(hold_nm), py(ang * 1000)), (px(hold_nm - 1), py(PLATFORM_FT)), (px(cross), py(PLATFORM_FT))] + [(px(d), py(gs_at(d))) for d in (cross - 0.01, 8, 3, 0)]
    s.append(pc.poly([prof[0], prof[1], prof[2]] + [(px(d), py(gs_at(d))) for d in (12, 8, 3, 0)], WHITE, 2.6))
    for nm, lab in ((hold_nm - 1, "commence"), (cross + 0.5, "platform 5,000 ft"), (8, "dirty up"), (3, "Paddles")):
        alt = PLATFORM_FT if nm >= cross else gs_at(nm)
        if lab == "commence":
            alt = PLATFORM_FT
        s.append('<circle cx="%.1f" cy="%.1f" r="4" fill="%s"/>' % (px(nm), py(alt), YEL))
        s.append(pc.note(px(nm) + 6, py(alt) - 12, lab, 14))
    s.append(pc.text(PW - 8, 18, "from the side, planned profile", 12, MUTED, "end"))
    return "".join(s) + "</svg>"


def card_html(rec, mates, label, name, brc, ctx=None, result=None):
    badge = "Case " + (rec["case"] or "III")
    t = lambda x: x.strftime("%H:%M") if x else "-"
    eat_line = "approach time %s" % t(rec["eat"]) + (" (re-issued: it had passed)" if rec["eat_missed"] else "")
    weather = ""
    if ctx and ctx.get("ceiling") is not None:
        ceil, vis = ctx["ceiling"], ctx.get("vis") or 0
        weather = "Weather: %s, visibility %s" % ("ceiling unrestricted" if ceil >= 20000 or ceil <= 0 else "ceiling %s ft" % format(int(round(ceil, -2)), ","), "10+ NM" if vis >= 10 else "%.0f NM" % vis)
    lines = ['Nimitz Class "ABE" | Marshal | %s server' % rec["rig"], rec["checkin"].strftime("%A %B %d, %Y"),
             *([weather] if weather else []), "BRC %s | Case %s" % ("%03d" % (round(brc) % 360 or 360) if brc is not None else "-", rec["case"] or "III"), "",
             "Marshal radial %03d, %d DME, angels %d, %s" % (round(rec["radial"]) % 360 or 360, (rec["angels"] or 6) + 15, rec["angels"] or 6, eat_line),
             "Checked in %s | in the hold %s | commenced %s | platform %s | dirty up %s | Paddles %s" % (
                 t(rec["checkin"]), t(rec["hold"]), t(rec["commence"]), t(rec["platform"]), t(rec["dirty"]), t(rec["handoff"])), "",
             result or "", "(positions: the assigned radial and DME, and the planned profile - real radar fixes appear once Marshal logs them)" if not rec["fixes"] else ""]
    body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
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
<div class="ps"><div class="pn"><div class="tab">Stack View</div>%s</div><div class="pn"><div class="tab">Commence View</div>%s</div></div>
<div class="info">%s</div><div class="ft">VMFA-225 Marshal</div></div></body></html>""" % (
        html.escape(badge), html.escape(label + " | Marshal"), pc.LOGO, stack_view(rec, mates, brc, name), commence_view(rec, mates, name), body)


def main():
    mr.fetch(10)
    events = mr.read_events(10)
    recs = mr.approaches(events)
    passes = gp.load_passes(dt.date.today())
    mr.link_passes(recs, passes)
    c3 = [r for r in recs if r["case"] == "III" and r.get("radial") is not None]
    if not c3:
        print("no Case III approach logged")
        return 1
    rec = c3[-1]
    names = {cs: label for cs, label, _, _ in gp.roster()}
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    key = canon.get(rec["callsign"].lower(), rec["callsign"].lower())
    name = lambda r: names.get(canon.get(r["callsign"].lower(), r["callsign"].lower()), r["callsign"]).split(" | ")[-1]
    mates = [r for r in c3 if r["rig"] == rec["rig"] and abs((r["checkin"] - rec["checkin"]).total_seconds()) < 3600]
    p = rec.get("pass")
    result = ("Paddles: %s%s" % (p["grade"], ", %d wire" % p["wire"] if p.get("wire") and not p["grade"].startswith("WO") else "")) if p else "No Paddles pass seen"
    ctx = {"ceiling": next((o.get("ceiling") for t, rg, m, o in events if rg == rec["rig"] and "Case 3 aircraft checking in" in m and o.get("callsign") == rec["callsign"] and t == rec["checkin"]), None),
           "vis": next((o.get("vis") for t, rg, m, o in events if rg == rec["rig"] and "Case 3 aircraft checking in" in m and o.get("callsign") == rec["callsign"] and t == rec["checkin"]), None)}
    page = card_html(rec, mates, names.get(key, rec["callsign"]), name, rec["brc"], ctx, result)
    png = gp.screenshot(page, 1010, 1760)
    out = os.path.join(gp.REPORTS, "_data", "paddles", "sample-marshal-card.png")
    open(out, "wb").write(png)
    print(out)
    return 0


if __name__ == "__main__":
    sys.exit(main())
