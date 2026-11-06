# -*- coding: utf-8 -*-
"""The Marshal report: a picture of how Marshal worked for the squadron, posted weekly.

Operator 2026-11-06: "a report on marshal". From each rig's Marshal log (the launcher's /api/day-log) it builds
  1. the recovery timelines - each jet's check-in, hold, commence, platform / dirty-up and hand-off to Paddles;
  2. Marshal's accuracy - what it heard, answered, missed (intent misses) and ignored;
  5. stack and timing - how long jets held, whether they pushed on their approach time, how deep the stack got;
  6. the link to the landings - for each approach through Marshal, the Paddles pass that followed and its grade.
Posted as a new message in the greenie board's channel by greenie_watch.py (Sunday, after the weekly report).

Run:  python -I docs/paddles-report/marshal_report.py [--days 7] [--post]
"""
import collections
import datetime as dt
import html
import json
import math
import os
import statistics
import sys
import urllib.error
import urllib.parse
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I does not put the script's folder on the path
import greenie_post as gp  # noqa: E402
import pilot_stats as ps  # noqa: E402

RIGS = gp.RIGS
DATA = os.path.join(gp.REPORTS, "_data", "marshal")
CASE_WORDS = {"One": "I", "Two": "II", "Three": "III", "1": "I", "2": "II", "3": "III"}
HAND_OFF = dt.timedelta(minutes=25)  # a pass this soon after the hand-off is that approach's landing


def get(url, timeout=60):
    try:
        with urllib.request.urlopen(url, timeout=timeout) as r:
            if "html" in r.headers.get("Content-Type", ""):
                return None
            return r.read()
    except (urllib.error.URLError, OSError):
        return None


def marshal_roles(host):
    body = get("http://%s:7000/api/roles" % host, 15)
    if not body:
        return []
    try:
        roles = json.loads(body)
    except ValueError:
        return []
    roles = roles.get("roles", roles) if isinstance(roles, dict) else roles
    return [r.get("name") for r in roles if isinstance(r, dict) and str(r.get("name", "")).startswith("Marshal")]


def fetch(days):
    """Save each rig's Marshal log for the days (the last two days are fetched again; older days only once)."""
    os.makedirs(DATA, exist_ok=True)
    today = dt.date.today()
    for rig, host in RIGS:
        for role in marshal_roles(host):
            for i in range(days + 1):
                day = (today - dt.timedelta(days=i)).isoformat()
                path = os.path.join(DATA, "%s %s %s.log" % (day, rig, role.replace(" ", "_").replace("(", "").replace(")", "")))
                if os.path.exists(path) and i > 1:
                    continue
                body = get("http://%s:7000/api/day-log?name=%s&date=%s" % (host, urllib.parse.quote(role), day))
                if body is not None:
                    open(path, "wb").write(body)


def read_events(days):
    """[(time, rig, message, fields)] from the saved logs, oldest first."""
    since = dt.date.today() - dt.timedelta(days=days)
    out, seen = [], set()
    for f in sorted(os.listdir(DATA)) if os.path.isdir(DATA) else []:
        day, rig = f[:10], f.split(" ")[1]
        if day < since.isoformat():
            continue
        for line in open(os.path.join(DATA, f), encoding="utf-8", errors="replace"):
            try:
                o = json.loads(line)
                t = dt.datetime.fromisoformat(o["time"])
            except (ValueError, KeyError):
                continue
            key = (rig, o["time"], o.get("message"), o.get("text"), o.get("callsign"))
            if key in seen:
                continue
            seen.add(key)
            out.append((t, rig, o.get("message", ""), o))
    return sorted(out, key=lambda e: e[0])


def add_fix(rec, kind, t, o):
    """A real radar position from Marshal's log: range (NM) and magnetic bearing from the carrier, and angels."""
    if o.get("fixFound") and o.get("fixDistNm"):
        rec["fixes"].append((kind, t, o["fixDistNm"], o.get("fixBearingMag"), o.get("fixAngels")))


def approaches(events):
    """Each jet's trip through Marshal: a record per check-in, filled in as its later calls arrive."""
    recs, open_rec = [], {}
    for t, rig, msg, o in events:
        cs = o.get("callsign")
        if "aircraft checking in" in msg and cs:
            rec = {"rig": rig, "callsign": cs, "checkin": t, "position": o.get("position"), "angels": o.get("stackAngels"),
                   "eat": dt.datetime.fromisoformat(o["eat"]) if o.get("eat") else None, "case": CASE_WORDS.get((o.get("case") or "").replace("Case ", ""), (o.get("case") or "").replace("Case ", "")),
                   "radial": o.get("radial"), "brc": o.get("brcMag"), "fixes": [], "hold": None, "commence": None, "platform": None, "dirty": None, "handoff": None, "eat_missed": 0, "misses": 0}
            add_fix(rec, "check-in", t, o)
            recs.append(rec)
            open_rec[(rig, cs.lower())] = rec
            continue
        rec = open_rec.get((rig, (cs or "").lower()))
        if not rec:
            continue
        if "established in hold" in msg:
            rec["hold"] = t
            add_fix(rec, "hold", t, o)
        elif "commencing" in msg:
            rec["commence"] = t
            add_fix(rec, "commence", t, o)
        elif "platform call" in msg:
            rec["platform"] = t
            add_fix(rec, "platform", t, o)
        elif "dirty up" in msg:
            rec["dirty"] = t
        elif "handing to Paddles" in msg or "handing off to LSO" in msg:
            rec["handoff"] = t
        elif "approach time had passed" in msg:
            rec["eat_missed"] += 1
        elif "intent miss" in msg:
            rec["misses"] += 1
    return recs


def accuracy(events):
    """What Marshal heard, answered, missed and ignored."""
    c = collections.Counter()
    for t, rig, msg, o in events:
        if msg == "Marshal heard":
            c["heard"] += 1
        elif msg == "Marshal TX":
            c["answered"] += 1
        elif msg == "Marshal intent miss":
            c["miss"] += 1
        elif "not addressed to Marshal" in msg:
            c["ignored"] += 1
        elif "readback, no reply" in msg:
            c["readbacks"] += 1
        elif "SRS disconnected" in msg:
            c["srs"] += 1
        elif "recovery case changed" in msg:
            c["cases"] += 1
    return c


def link_passes(recs, passes):
    """Attach the Paddles pass that followed each approach (same callsign, soon after the hand-off or the commence call)."""
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    by_cs = collections.defaultdict(list)
    for p in passes:
        by_cs[canon.get(p["callsign"].lower(), p["callsign"].lower())].append(p)
    for r in recs:
        key = canon.get(r["callsign"].lower(), r["callsign"].lower())
        start = r["handoff"] or r["commence"] or r["checkin"]
        r["pass"] = next((p for p in sorted(by_cs.get(key, []), key=lambda p: p["t"]) if start <= p["t"] <= start + HAND_OFF), None)
    return recs


def steps(r):
    """How far through the procedure the jet got: hold, commence, hand-off."""
    return sum(1 for k in ("hold", "commence", "handoff") if r[k])


def mins(a, b):
    return (b - a).total_seconds() / 60 if a and b else None


def stack_map(recs, names, canon):
    """An overview map (north up, carrier in the middle): each Case III approach's hold fix on its marshal radial
    (angels + 15 DME, as Marshal gives it), the way in along the radial, and the dirty-up and hand-off points; the
    marker is coloured by the grade of the pass that followed. Returns (svg, lines of text beside it)."""
    c3 = [r for r in recs if r["case"] == "III" and r.get("radial") is not None]
    size, cx, cy = 600, 300, 300
    far = max([(r["angels"] or 6) + 15 for r in c3] + [f[2] for r in c3 for f in r["fixes"]] + [20]) + 8
    far = int(math.ceil(far / 10.0) * 10)
    k = 270.0 / far  # pixels per nautical mile

    def pt(brg, nm):
        a = math.radians(brg)
        return cx + nm * k * math.sin(a), cy - nm * k * math.cos(a)
    s = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">' % (size, size)]
    for nm in range(10, far + 1, 10):
        s.append('<circle cx="%d" cy="%d" r="%.1f" fill="none" stroke="#30363d" stroke-width="1.2"/>' % (cx, cy, nm * k))
        s.append('<text x="%.1f" y="%.1f" fill="#6e7681" font-size="12">%d NM</text>' % (cx + 4, cy - nm * k - 3, nm))
    for brg, lab in ((0, "N"), (90, "E"), (180, "S"), (270, "W")):
        x, y = pt(brg, far + 2.2)
        s.append('<text x="%.1f" y="%.1f" fill="#8b949e" font-size="14" text-anchor="middle">%s</text>' % (x, y + 5, lab))
    text, seen = [], {}
    for r in c3:
        ang = r["angels"] or 6
        d = ang + 15
        hx, hy = pt(r["radial"], d)
        real = [f for f in r["fixes"] if f[3] is not None]
        hold = next((f for f in real if f[0] == "hold"), None) or next((f for f in real if f[0] == "check-in"), None)
        if hold:  # a real position beats the formula: where the jet was when it was in the hold
            hx, hy = pt(hold[3], hold[2])
        grade = gp.board_key(r["pass"]) if r.get("pass") else None
        fill = gp.COLOURS[grade][0] if grade else "#6e7681"
        if len(real) > 1:  # the jet's own track through Marshal: check-in, hold, commence, platform
            s.append('<polyline fill="none" stroke="#58a6ff" stroke-width="2" points="%s"/>' % " ".join("%.1f,%.1f" % pt(f[3], f[2]) for f in real))
            for f in real:
                x, y = pt(f[3], f[2])
                s.append('<circle cx="%.1f" cy="%.1f" r="4" fill="#0d1117" stroke="#58a6ff" stroke-width="1.6"/>' % (x, y))
        s.append('<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#58a6ff" stroke-width="1.6" stroke-dasharray="6 5" opacity=".6"/>' % (hx, hy, cx, cy))
        for nm, lab in ((8, "dirty up"), (3, "Paddles")):
            x, y = pt(r["radial"], nm)
            s.append('<circle cx="%.1f" cy="%.1f" r="3.2" fill="#e8b400"/>' % (x, y))
            if (round(r["radial"]), nm) not in seen:
                seen[(round(r["radial"]), nm)] = 1
                s.append('<text x="%.1f" y="%.1f" fill="#e8b400" font-size="11">%s</text>' % (x + 4, y - 7, lab))
        s.append('<circle cx="%.1f" cy="%.1f" r="9" fill="%s" stroke="#e6edf3" stroke-width="1.6"/>' % (hx, hy, fill))
        who = names.get(canon.get(r["callsign"].lower(), r["callsign"].lower()), r["callsign"]).split(" | ")[-1]
        s.append('<text x="%.1f" y="%.1f" fill="#e6edf3" font-size="13" text-anchor="middle">A%d %s</text>' % (hx, hy - 14, ang, html.escape(who)))
        p = r.get("pass")
        text.append("%s - %s: %03d radial, %d DME, angels %d%s%s%s" % (
            r["checkin"].strftime("%a %d %b %H:%M"), who, round(r["radial"]) % 360 or 360, d, ang,
            ", held at %d NM on %03d" % (hold[2], hold[3]) if hold else "",
            ", pushed %s" % r["commence"].strftime("%H:%M") if r["commence"] else "",
            ", %s%s" % (p["grade"], " %d wire" % p["wire"] if p.get("wire") else "") if p else ""))
    # the carrier: a ship symbol along the BRC of the last approach, at the centre
    brc = next((r["brc"] for r in reversed(c3) if r.get("brc") is not None), None)
    rot = -90 if brc is None else brc - 90  # the symbol is drawn pointing east (right)
    s.append('<g transform="translate(%d %d) rotate(%.1f)"><path d="M14 0 L-8 -6 L-12 -6 L-12 6 L-8 6 Z" fill="#8b949e"/></g>' % (cx, cy, rot))
    s.append('<text x="%d" y="%d" fill="#e6edf3" font-size="13" text-anchor="middle">ABE%s</text>' % (cx, cy + 40, " BRC %03d" % (round(brc) % 360 or 360) if brc is not None else ""))
    if not c3:
        s.append('<text x="%d" y="%d" fill="#8b949e" font-size="16" text-anchor="middle">No Case III approaches in this period</text>' % (cx, cy - 60))
    s.append("</svg>")
    return "".join(s), text


def build(days=7):
    now = dt.datetime.now().astimezone()
    events = read_events(days)
    recs = approaches(events)
    passes = [p for p in gp.load_passes(now.date()) if p["t"] >= now - dt.timedelta(days=days)]
    link_passes(recs, passes)
    acc = accuracy(events)
    names = {cs: label for cs, label, _, _ in gp.roster()}
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    holds = [x for x in (mins(r["checkin"], r["commence"]) for r in recs) if x is not None]
    eats = [mins(r["eat"], r["commence"]) for r in recs if r["eat"] and r["commence"]]
    on_time = sum(1 for e in eats if abs(e) <= 2)
    c3 = [r for r in recs if r["case"] == "III"]  # only Case III logs every step (hold, commence, platform, dirty up, hand-off)
    complete = sum(1 for r in c3 if steps(r) == 3)
    tiles = [("Check-ins", len(recs)), ("Case III completed the procedure", "%d of %d" % (complete, len(c3)) if c3 else "-"),
             ("Calls heard", acc["heard"]), ("Answered", acc["answered"]), ("Intent misses", acc["miss"]), ("Ignored (not addressed)", acc["ignored"])]
    timing = [
        ("Median check-in to commence", "%.0f min" % statistics.median(holds) if holds else "-"),
        ("Pushed within 2 min of the approach time", "%d of %d" % (on_time, len(eats)) if eats else "-"),
        ("Approach time passed (new time given)", sum(r["eat_missed"] for r in recs)),
        ("Deepest stack", max((r["position"] or 0 for r in recs), default=0) or "-"),
        ("Case changes (all rigs)", acc["cases"]), ("SRS drops (all rigs)", acc["srs"]),
    ]
    # per pilot
    per = collections.defaultdict(list)
    for r in recs:
        per[canon.get(r["callsign"].lower(), r["callsign"].lower())].append(r)
    rows = []
    for k, rl in sorted(per.items(), key=lambda kv: -len(kv[1])):
        h = [x for x in (mins(r["checkin"], r["commence"]) for r in rl) if x is not None]
        grades = [x for x in (gp.points(r["pass"]) for r in rl if r["pass"]) if x is not None]
        rows.append("<tr><td class='n'>%s</td><td>%d</td><td>%d of %d Case III</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>" % (
            html.escape(names.get(k, rl[0]["callsign"])), len(rl), sum(1 for r in rl if r["case"] == "III" and steps(r) == 3),
            sum(1 for r in rl if r["case"] == "III"),
            "%.0f min" % statistics.median(h) if h else "-", sum(r["eat_missed"] for r in rl), sum(r["misses"] for r in rl),
            "%.1f (%d)" % (statistics.mean(grades), len(grades)) if grades else "-"))
    # the latest approaches, jet by jet
    ticks = lambda r: "".join('<span class="%s">%s</span>' % ("y" if r[k] else "n", lab) for k, lab in (("hold", "hold"), ("commence", "commence"), ("platform", "platform"), ("dirty", "dirty up"), ("handoff", "Paddles")))
    last = []
    for r in recs[-12:][::-1]:
        p = r["pass"]
        last.append("<tr><td>%s</td><td class='n'>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>" % (
            r["checkin"].strftime("%a %d %b %H:%M"), html.escape(names.get(canon.get(r["callsign"].lower(), r["callsign"].lower()), r["callsign"])),
            ("Case " + r["case"]) if r["case"] else "-", ticks(r),
            html.escape("%s%s" % (p["grade"], " %d wire" % p["wire"] if p.get("wire") else "")) if p else "<span class='m'>no pass seen</span>"))
    map_svg, map_text = stack_map(recs, names, canon)
    logo = "file:///" + os.path.join(os.path.dirname(gp.HERE), "viking_head_white.png").replace("\\", "/")
    title = "%s - %s" % ((now - dt.timedelta(days=days)).strftime("%d %b"), now.strftime("%d %b %Y"))
    page = """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:16px 20px;width:1460px}
.top{display:flex;align-items:center;gap:18px}h1{margin:0;font-size:34px;color:#58a6ff}.sub{color:#8b949e;font-size:17px;margin-top:4px}.logo{margin-left:auto;height:84px}
.tiles{display:flex;gap:12px;margin:16px 0}.tile{flex:1;background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:10px 14px}.tile b{display:block;font-size:30px;color:#e6edf3}.tile span{color:#8b949e;font-size:14px}
.tm{display:flex;gap:12px;margin-bottom:6px}.tm div{flex:1;background:#1c2330;border-radius:6px;padding:9px 12px;font-size:17px}.tm small{display:block;color:#8b949e;font-size:12px}
h2{font-size:18px;margin:14px 0 6px;color:#e6edf3;border-bottom:1px solid #30363d;padding-bottom:4px}
table{border-collapse:collapse;width:100%%}th{color:#8b949e;font-size:13px;text-align:left;padding:5px 8px;border-bottom:1px solid #30363d}td{padding:6px 8px;border-bottom:1px solid #21262d;font-size:16px}
td.n{color:#e6edf3;font-weight:700}.y{background:#1f6b00;color:#fff;border-radius:3px;padding:1px 7px;margin-right:4px;font-size:13px}.n{}span.n{background:#30363d;color:#8b949e;border-radius:3px;padding:1px 7px;margin-right:4px;font-size:13px}.m{color:#8b949e}
.mp{display:flex;gap:20px;align-items:flex-start}.mp ul{margin:6px 0 0 16px;padding:0;font-size:16px;line-height:1.7}
.ft{color:#8b949e;font-size:12px;margin-top:10px}
</style></head><body><div class="w"><div class="top"><div><h1>VMFA-225 Marshal Report</h1><div class="sub">%s</div></div><img class="logo" src="%s"></div>
<div class="tiles">%s</div><div class="tm">%s</div>
<h2>Pilots through Marshal</h2><table><tr><th>Pilot</th><th>Check-ins</th><th>Case III completed (hold, commence, hand-off)</th><th>Median check-in to commence</th><th>Approach times passed</th><th>Intent misses</th><th>Average grade after (passes)</th></tr>%s</table>
<h2>Stack map - Case III holds (the jet's real position where Marshal logged it, otherwise its assigned radial and DME)</h2><div class="mp"><div>%s</div><ul>%s</ul></div>
<h2>Latest approaches</h2><table><tr><th>Checked in</th><th>Pilot</th><th>Case</th><th>Steps Marshal logged</th><th>Paddles pass that followed</th></tr>%s</table>
<div class="ft">From the Marshal logs of every rig. An intent miss is a call Marshal heard but could not act on. "Ignored" calls were not addressed to Marshal (often heard as Deckboss or another name). A pass counts for an approach if it came within 25 minutes of the hand-off.</div></div></body></html>""" % (
        html.escape(title), logo,
        "".join("<div class='tile'><b>%s</b><span>%s</span></div>" % (v, html.escape(k)) for k, v in tiles),
        "".join("<div><small>%s</small>%s</div>" % (html.escape(k), v) for k, v in timing),
        "".join(rows) or "<tr><td colspan='7'>No check-ins in this period.</td></tr>", map_svg,
        "".join("<li>%s</li>" % html.escape(t) for t in map_text) or "<li>No Case III approaches in this period.</li>",
        "".join(last) or "<tr><td colspan='5'>No approaches in this period.</td></tr>")
    summary = "**Marshal report** - %s: %d check-ins, %d calls heard, %d intent misses" % (title, len(recs), acc["heard"], acc["miss"])
    return page, summary, len(rows), min(len(recs), 12)


def main():
    days = int(sys.argv[sys.argv.index("--days") + 1]) if "--days" in sys.argv else 7
    fetch(days)
    page, summary, pilots, last = build(days)
    png = gp.screenshot(page, 1090 + 36 * pilots + 40 * last, 1500)
    out = os.path.join(gp.REPORTS, "_data", "paddles", "marshal-report.png")
    open(out, "wb").write(png)
    print(summary, "->", out)
    if "--post" in sys.argv:
        hook = open(gp.WEBHOOK_FILE, encoding="utf-8").read().strip()
        gp.send(hook + "?wait=true", "POST", png, summary)
        print("posted")
    return 0


if __name__ == "__main__":
    sys.exit(main())
