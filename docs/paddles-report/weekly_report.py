# -*- coding: utf-8 -*-
"""The weekly report: a second picture, posted as a new message in the greenie board's channel each week.

Operator 2026-11-06: "a weekly push of a separate report to the greenieboard so we will have 2 reports".
The hourly greenie board is one message edited in place; this is a fresh post, the last seven days:
squadron numbers, highlights (best average, most improved, most traps), every pilot who flew with their
trend, 3-wire rate, most common fault and carrier-qualification progress, and the week's recoveries.
greenie_watch.py posts it on Sunday at 20:00; run it by hand with --post, or --preview to only draw it
(Reports\\_data\\paddles\\weekly-report.png).

Run:  python -I docs/paddles-report/weekly_report.py [--monthly] [--post]
The monthly report (--monthly) is the past 30 days, in the same style, kept as ONE message in the same channel that
the watcher edits each week (id in readback-greenie-monthly-message.txt) instead of posting a new one.
"""
import datetime as dt
import html
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I does not put the script's folder on the path
import greenie_post as gp  # noqa: E402
import pass_cards as pc  # noqa: E402
import pilot_stats as ps  # noqa: E402
import recovery_summary as rs  # noqa: E402

MIN_FOR_HIGHLIGHT = 3  # passes in the period (and before it, for "most improved"); 5 for the month
MONTHLY_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-monthly-message.txt")  # the monthly message's id


def arrow(delta):
    if delta is None:
        return "-"
    return "&#9650; +%.1f" % delta if delta >= 0.15 else "&#9660; %.1f" % delta if delta <= -0.15 else "&#9644; %.1f" % delta


def build(now=None, days=7, kind="Weekly"):
    """(html, summary text, number of pilots, number of recoveries) for the last `days` days up to now."""
    now = now or dt.datetime.now().astimezone()
    start = now - dt.timedelta(days=days)
    minp = MIN_FOR_HIGHLIGHT if days <= 7 else 5
    passes = gp.load_passes(now.date())
    week = [p for p in passes if p["t"] >= start]
    before = [p for p in passes if p["t"] < start]
    gw, names = ps.grouped(week)
    gb, _ = ps.grouped(before)
    ga, _ = ps.grouped(passes)
    cfg = ps.cq_config()
    total = ps.stats([p for plist in gw.values() for p in plist])
    night = sum(1 for plist in gw.values() for p in plist if ps.is_night(p))
    dayset = {(now.date() - dt.timedelta(days=i)).isoformat() for i in range(0, days + 1)}
    contexts = pc.read_context(dayset)
    patterns = pc.read_logged(dayset, "Paddles: pass pattern", "pattern")
    sess = [(r, pl) for r, pl in rs.sessions([p for p in week if p.get("rig")]) if len(pl) >= 2]
    older = max(0, len(sess) - 10)
    sess_all, sess = sess, sess[-10:]  # the list shows the latest ten recoveries

    rows, stat = [], {}
    for k, plist in gw.items():
        s = ps.stats(plist)
        sb = ps.stats(gb.get(k, []))
        delta = s["avg"] - sb["avg"] if s["avg"] is not None and sb["avg"] is not None and sb["n"] >= minp else None
        stat[k] = (s, delta)
        rows.append((s["avg"] if s["avg"] is not None else -1, k))
    rows.sort(reverse=True)

    def hl(items):
        return html.escape(names[items[1]]) + " <b>%s</b>" % items[2] if items else "-"
    elig = [(stat[k][0]["avg"], k) for k in gw if stat[k][0]["n"] >= minp and stat[k][0]["avg"] is not None]
    best = max(elig, default=None)
    improved = max(((stat[k][1], k) for k in gw if stat[k][1] is not None and stat[k][0]["n"] >= minp), default=None)
    most = max(((stat[k][0]["traps"], k) for k in gw), default=None)
    highlights = [
        ("Best average", "%s <b>%.1f</b>" % (html.escape(names[best[1]]), best[0]) if best else "-"),
        ("Most improved", "%s <b>%+.1f</b>" % (html.escape(names[improved[1]]), improved[0]) if improved and improved[0] > 0 else "-"),
        ("Most traps", "%s <b>%d</b>" % (html.escape(names[most[1]]), most[0]) if most and most[0] else "-"),
    ]

    tiles = [("Passes", len(week)), ("Traps", total["traps"]), ("Boarding rate", "%d%%" % round(total["boarding"]) if total["boarding"] is not None else "-"),
             ("Average grade", "%.1f" % total["avg"] if total["avg"] is not None else "-"), ("3-wire", "%d%%" % round(total["three"]) if total["three"] is not None else "-"),
             ("Case III passes", night), ("Recoveries", len(sess_all))]

    body = []
    for _, k in rows:
        s, delta = stat[k]
        d, n = ps.cq_progress(ga[k], cfg)
        cq = lambda have, need: '<span class="%s">%d/%d</span>' % ("ok" if have >= need else "", min(have, need), need)
        body.append("<tr><td class='n'>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class='f'>%s</td><td>%s</td><td>%s</td></tr>" % (
            html.escape(names[k]), s["n"], s["traps"], s["bolters"], s["wos"], "%d%%" % round(s["boarding"]) if s["boarding"] is not None else "-",
            "%.1f" % s["avg"] if s["avg"] is not None else "-", arrow(delta), "%d%%" % round(s["three"]) if s["three"] is not None else "-",
            html.escape(s["fault"]) or "-", cq(d, cfg["day"]), cq(n, cfg["night"])))
    recs = []
    for rig, plist in sess:
        r = rs.summarise(rig, plist, contexts, patterns)
        bits = ["%s %s" % (rig, r["start"].strftime("%a %d %b %H:%M")), "%d passes, %d traps, %d bolters, %d wave-offs" % (r["n"], r["traps"], r["bolters"], r["wos"])]
        if r["case"]:
            bits.append("Case " + r["case"])
        if r["wod"]:
            bits.append("wind over deck %d kts" % round(r["wod"][0]))
        if r["gap"]:
            bits.append("%s between traps" % rs.clock(round(r["gap"])))
        recs.append("<li>%s</li>" % html.escape(" | ".join(bits)))
    # carrier qualification for everyone on the roster (not Reserve unless they have passes)
    reserve = {cs for cs, _, res, _ in gp.roster() if res}
    cqs = []
    for k in sorted(names, key=lambda c: names[c]):
        if k in reserve and k not in ga:
            continue
        d, n = ps.cq_progress(ga.get(k, []), cfg)
        done = d >= cfg["day"] and n >= cfg["night"]
        cqs.append('<span class="chip%s">%s <b>D %d/%d  N %d/%d</b></span>' % (" ok" if done else "", html.escape(names[k]), min(d, cfg["day"]), cfg["day"], min(n, cfg["night"]), cfg["night"]))

    logo = "file:///" + os.path.join(os.path.dirname(gp.HERE), "viking_head_white.png").replace("\\", "/")
    title = "%s - %s%s" % (start.strftime("%d %b"), now.strftime("%d %b %Y"), " (the past %d days)" % days if days > 7 else "")
    page = """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:16px 20px;width:1460px}
.top{display:flex;align-items:center;gap:18px}h1{margin:0;font-size:34px;color:#58a6ff}.sub{color:#8b949e;font-size:17px;margin-top:4px}.logo{margin-left:auto;height:84px}
.tiles{display:flex;gap:12px;margin:16px 0}.tile{flex:1;background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:10px 14px}.tile b{display:block;font-size:30px;color:#e6edf3}.tile span{color:#8b949e;font-size:14px}
.hl{display:flex;gap:12px;margin-bottom:14px}.hl div{flex:1;background:#1c2330;border-radius:6px;padding:10px 14px;font-size:19px}.hl small{display:block;color:#8b949e;font-size:13px}
h2{font-size:18px;margin:14px 0 6px;color:#e6edf3;border-bottom:1px solid #30363d;padding-bottom:4px}
table{border-collapse:collapse;width:100%%}th{color:#8b949e;font-size:13px;text-align:left;padding:5px 8px;border-bottom:1px solid #30363d}td{padding:6px 8px;border-bottom:1px solid #21262d;font-size:17px}
td.n{color:#e6edf3;font-weight:700}td.f{color:#8b949e;font-size:15px}.ok{color:#3fb950;font-weight:700}
ul{margin:4px 0 0 18px;padding:0;font-size:16px;line-height:1.6}.chip{display:inline-block;background:#1c2330;border:1px solid #30363d;border-radius:12px;padding:3px 12px;margin:3px;font-size:15px}.chip.ok{border-color:#3fb950}
.ft{color:#8b949e;font-size:12px;margin-top:10px}
</style></head><body><div class="w"><div class="top"><div><h1>VMFA-225 %s Report</h1><div class="sub">%s</div></div><img class="logo" src="%s"></div>
<div class="tiles">%s</div><div class="hl">%s</div>
<h2>Pilots</h2><table><tr><th>Pilot</th><th>Passes</th><th>Traps</th><th>Bolters</th><th>WO</th><th>Boarding</th><th>Avg</th><th>vs before</th><th>3-wire</th><th>Most common fault</th><th>CQ day</th><th>CQ night</th></tr>%s</table>
<h2>Recoveries</h2><ul>%s</ul>
<h2>Carrier qualification (%d day, %d night traps since %s)</h2><div>%s</div>
<div class="ft">Grades from Paddles; an average counts a foul-deck wave-off as no score. A Case III trap is a night trap; others count as day.</div></div></body></html>""" % (
        html.escape(kind), html.escape(title), logo,
        "".join("<div class='tile'><b>%s</b><span>%s</span></div>" % (v, html.escape(label)) for label, v in tiles),
        "".join("<div><small>%s</small>%s</div>" % (html.escape(a), b) for a, b in highlights),
        "".join(body) or "<tr><td colspan='12'>No passes in this period.</td></tr>", ("".join(recs) + ("<li>and %d earlier</li>" % older if older else "")) or "<li>No recoveries in this period.</li>",
        cfg["day"], cfg["night"], cfg["since"], "".join(cqs))
    summary = "**%s report** - %s: %d passes, %d traps, boarding rate %s" % (kind, title, len(week), total["traps"], tiles[2][1])
    return page, summary, len(rows), len(sess)


def main():
    monthly = "--monthly" in sys.argv
    page, summary, pilots, sess = build(days=30, kind="Monthly") if monthly else build()
    png = gp.screenshot(page, 330 + 52 * pilots + 34 * sess + 70, 1500)
    out = os.path.join(gp.REPORTS, "_data", "paddles", "monthly-report.png" if monthly else "weekly-report.png")
    open(out, "wb").write(png)
    print(summary, "->", out)
    if "--post" in sys.argv:
        hook = gp.reports_hook()
        if monthly:  # one message, edited each week
            print(gp.post(hook, png, summary, MONTHLY_FILE))
        else:
            gp.send(hook + "?wait=true", "POST", png, summary)
            print("posted")
    return 0


if __name__ == "__main__":
    sys.exit(main())
