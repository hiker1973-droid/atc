# -*- coding: utf-8 -*-
"""The pattern report: how the squadron flew the Case I pattern.

Operator 2026-11-06: "I am looking more on how they flew the pattern". Two parts:
  - an overlay map: every pass's flown pattern in the carrier's frame (ship at the top, bow to the left, as on the
    pass card), coloured by the pass's grade, over the kneeboard's guides (the 1.0-1.5 NM downwind corridor, the
    0.5 / 1 / 2 / 3 NM rings), with the break marked. Tracks come from the "Paddles: pass pattern" log lines, which
    exist for passes flown after the 2026-11-06 update;
  - a table per pilot of the numbers Paddles logs for every Case I pass (initial, break, abeam, the 90, the 45,
    groove time) against the kneeboard, plus, from the tracks, how far past ABE they broke and where they rolled out.
greenie_watch.py posts it on Sunday after the Marshal report.

Run:  python -I docs/paddles-report/pattern_report.py [--days 30] [--post]
"""
import collections
import datetime as dt
import html
import math
import os
import statistics
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I does not put the script's folder on the path
import greenie_post as gp  # noqa: E402
import pass_cards as pc  # noqa: E402
import pilot_stats as ps  # noqa: E402
from build_paddles_report import PAT_LIMITS  # noqa: E402

M_NM = 1852.0
W, H = 1400, 600  # the map
SX, SY, K = 430, 110, 0.085  # ship's stern, and pixels per metre (the card's approach view, a little larger)


def tracks(passes, days):
    """[(pass, [[aft, port, ft, kts]])] for passes that have a logged pattern track."""
    dayset = {(dt.date.today() - dt.timedelta(days=i)).isoformat() for i in range(days + 1)}
    rows = pc.read_logged(dayset, "Paddles: pass pattern", "pattern")
    out = []
    for p in passes:
        pts = pc.match(p, rows) if p.get("rig") else None
        if pts and len(pts) > 5:
            out.append((p, pts))
    return out


def overlay(trk):
    """The map: all the tracks over the kneeboard guides."""
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (W, H)]
    s.append('<rect x="%d" y="%.1f" width="%d" height="%.1f" fill="rgba(63,185,80,.13)"/>' % (
        SX - 160, SY + 1.0 * M_NM * K, 3500 * K + 160, 0.5 * M_NM * K))  # the downwind corridor, 1.0-1.5 NM to port
    for nm in (0.5, 1, 2, 3):
        r = nm * M_NM * K
        s.append('<circle cx="%d" cy="%d" r="%.1f" fill="none" stroke="#30363d" stroke-width="1.2"/>' % (SX, SY, r))
        s.append(pc.text(SX + r * 0.8 + 4, SY + r * 0.6 + 14, "%g NM" % nm, 12, "#6e7681"))
    s.append(pc.text(SX + 3400 * K, SY + 1.0 * M_NM * K - 6, "downwind corridor 1.0 - 1.5 NM", 13, "#3fb950", "end"))
    s.append(pc.ship_top(SX - 76, SY))
    for p, pat in trk:
        key = gp.board_key(p)
        colour = gp.COLOURS[key][0]
        if key in ("OK", "(OK)", "_OK_"):
            colour = {"OK": "#66c200", "(OK)": "#3fb950", "_OK_": "#f2d600"}[key]
        pts = [(SX + r[0] * K, SY + r[1] * K) for r in pat]
        s.append(pc.poly(pts, colour, 1.8) + '<!-- %s -->' % html.escape(p["callsign"]))
        bi = pc.break_index(pat)
        if bi is not None:
            s.append('<circle cx="%.1f" cy="%.1f" r="4" fill="%s" stroke="#fff" stroke-width="1"/>' % (pts[bi][0], pts[bi][1], colour))
    s.append(pc.text(W - 10, 20, "from above, ship at the top, bow to the left; a dot is where the break began", 12, "#8b949e", "end"))
    return "".join(s) + "</svg>"


def pattern_numbers(plist):
    """Median of each logged Case I number, over the passes that flew it."""
    vals = collections.defaultdict(list)
    for p in plist:
        pt = p.get("pattern") or {}
        for k, i, key in (("initial", 0, "initial"), ("break", 0, "break"), ("abeam", 0, "abeam_nm"), ("abeam", 1, "abeam_ft"),
                          ("90", 0, "90"), ("45", 0, "45"), ("groove", 0, "groove")):
            if k in pt and "abeam" in pt:  # only passes that flew a break and downwind (not a straight-in)
                vals[key].append(pt[k][i])
    return {k: statistics.median(v) for k, v in vals.items()}, sum(1 for p in plist if "abeam" in (p.get("pattern") or {}))


def cell(v, key, fmt):
    if v is None:
        return "-"
    lo, hi, slack = PAT_LIMITS[key]
    txt = fmt % v
    bad = v < lo - slack or v > hi + slack
    return "<span class='bad'>%s</span>" % txt if bad else txt


def track_numbers(items):
    """From the tracks: nm past ABE at the break, the abeam distance, where the jet rolled out (medians)."""
    brk, abeam, roll = [], [], []
    for p, pat in items:
        bi = pc.break_index(pat)
        if bi is not None and pat[bi][0] < 0:
            brk.append(-pat[bi][0] / M_NM)
        far = max((r[1] for r in pat), default=0)
        if far > 800:
            ab = min((r for r in pat if r[1] > 0.6 * far), key=lambda r: abs(r[0]), default=None)
            if ab and abs(ab[0]) < 700:
                abeam.append(ab[1] / M_NM)
            down = next((i for i, r in enumerate(pat) if r[1] > 0.8 * far), None)
            if down is not None:
                rl = next((r for r in pat[down:] if abs(r[1]) < 250 and r[0] > 0), None)
                if rl:
                    roll.append(rl[0] / M_NM)
    med = lambda v: "%.1f NM" % statistics.median(v) if v else "-"
    return med(brk), med(abeam), med(roll)


def build(days=30):
    now = dt.datetime.now().astimezone()
    passes = [p for p in gp.load_passes(now.date()) if p["t"] >= now - dt.timedelta(days=days) and p.get("rig")]
    grp, names = ps.grouped(passes)
    trk = tracks(passes, days)
    trk_by = collections.defaultdict(list)
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    for p, pat in trk:
        trk_by[canon.get(p["callsign"].lower(), p["callsign"].lower())].append((p, pat))
    rows = []
    for k, plist in sorted(grp.items(), key=lambda kv: names[kv[0]]):
        med, n = pattern_numbers(plist)
        if not n and k not in trk_by:
            continue
        bk, ab, rl = track_numbers(trk_by.get(k, []))
        faults = collections.Counter(f for p in plist for f in (p.get("pattern_faults") or []))
        rows.append("<tr><td class='n'>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%s / %s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class='f'>%s</td></tr>" % (
            html.escape(names[k]), n, cell(med.get("initial"), "initial", "%d ft"), cell(med.get("break"), "break", "%d ft"),
            cell(med.get("abeam_nm"), "abeam_nm", "%.1f nm"), cell(med.get("abeam_ft"), "abeam_ft", "%d ft"),
            cell(med.get("90"), "90", "%d ft"), cell(med.get("45"), "45", "%d ft"), cell(med.get("groove"), "groove", "%.1f s"),
            bk, rl, html.escape(faults.most_common(1)[0][0]) if faults else "-"))
    logo = "file:///" + os.path.join(os.path.dirname(gp.HERE), "viking_head_white.png").replace("\\", "/")
    title = "%s - %s (the past %d days)" % ((now - dt.timedelta(days=days)).strftime("%d %b"), now.strftime("%d %b %Y"), days)
    note = ("Flown tracks for %d passes." % len(trk)) if trk else "No flown tracks yet: Paddles started recording them with the 6 Nov update, so this map fills in as the squadron flies. The table below uses the numbers logged for every Case I pass."
    page = """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:16px 20px;width:1460px}
.top{display:flex;align-items:center;gap:18px}h1{margin:0;font-size:34px;color:#58a6ff}.sub{color:#8b949e;font-size:17px;margin-top:4px}.logo{margin-left:auto;height:84px}
h2{font-size:18px;margin:16px 0 6px;color:#e6edf3;border-bottom:1px solid #30363d;padding-bottom:4px}
.map{background:#0d1117;border:1px solid #30363d;border-radius:6px;overflow:hidden;width:1400px;margin-top:8px}
table{border-collapse:collapse;width:100%%}th{color:#8b949e;font-size:13px;text-align:left;padding:5px 8px;border-bottom:1px solid #30363d}td{padding:6px 8px;border-bottom:1px solid #21262d;font-size:16px}
td.n{color:#e6edf3;font-weight:700}td.f{color:#8b949e;font-size:14px}.bad{color:#f85149;font-weight:700}.m{color:#8b949e;font-size:14px;margin:6px 0}
.ft{color:#8b949e;font-size:12px;margin-top:10px}
</style></head><body><div class="w"><div class="top"><div><h1>VMFA-225 Pattern Report</h1><div class="sub">%s</div></div><img class="logo" src="%s"></div>
<h2>How the pattern was flown</h2><div class="m">%s Track colour is the grade of the pass (green OK, dark green (OK), brown no grade, red cut, blue bolter, black wave-off).</div>%s
<h2>Case I pattern by pilot (median)</h2><table><tr><th>Pilot</th><th>Patterns</th><th>Initial<br>800 ft</th><th>Break<br>800 ft</th><th>Abeam<br>1-1.5 nm / 600 ft</th><th>The 90<br>450-500</th><th>The 45<br>370-400</th><th>Groove<br>15-18 s</th><th>Break past ABE</th><th>Rolled out</th><th>Most common fault</th></tr>%s</table>
<div class="ft">Kneeboard (Case I Recovery): initial 3 nm astern at 800 ft, a level break, downwind 1-1.5 nm abeam at 600 ft, the 90 at 450-500 ft, the 45 at 370-400 ft, 15-18 s in the groove. Red is off by more than 100 ft (initial, break, abeam), 50 ft (the 90 and 45) or outside the groove time. "Break past ABE" and "Rolled out" come from the flown tracks.</div></div></body></html>""" % (
        html.escape(title), logo, html.escape(note), ("<div class=\"map\">%s</div>" % overlay(trk)) if trk else "", "".join(rows) or "<tr><td colspan='11'>No Case I patterns in this period.</td></tr>")
    summary = "**Pattern report** - %s: %d Case I patterns, %d flown tracks" % (title, sum(1 for p in passes if "abeam" in (p.get("pattern") or {})), len(trk))
    return page, summary, len(rows)


def main():
    days = int(sys.argv[sys.argv.index("--days") + 1]) if "--days" in sys.argv else 30
    page, summary, pilots = build(days)
    png = gp.screenshot(page, (1000 if "Flown tracks for" in page else 400) + 36 * pilots + 120, 1500)
    out = os.path.join(gp.REPORTS, "_data", "paddles", "pattern-report.png")
    open(out, "wb").write(png)
    print(summary, "->", out)
    if "--post" in sys.argv:
        hook = gp.reports_hook()
        gp.send(hook + "?wait=true", "POST", png, summary)
        print("posted")
    return 0


if __name__ == "__main__":
    sys.exit(main())
