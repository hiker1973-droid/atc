"""Field landing card for jets landing at an airfield (the Viper counterpart of pass_cards.py).

Same look as the carrier pass card (dashboard colours, grade badge, patch top-right), but the panels
are a runway Glide View (height against distance, 3 degree line) and a Lineup View (from above, the
runway and the flown track). Input is a dict shaped like the tower's "Approach graded" data
(pkg/fieldgrade Result) plus an optional track [(nm_out, ft_agl, metres_right_of_centreline), ...].

    python field_card.py            # renders a SAMPLE card to field-card-sample.png
"""
import datetime as dt
import html
import json
import math
import os
import re
import sys
import urllib.parse
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import greenie_post as gp  # noqa: E402
LOGO = "file:///" + os.path.join(os.path.dirname(HERE), "vsfg7_patch.png").replace("\\", "/")
NM_FT = 6076.12
GS = 3.0
AimM = 300.0  # the aim point, metres past the threshold (pkg/fieldgrade AimM)
W, H = 840, 500
RINGS = (1.5, 1.0, 0.5, 0.25)
NM_MIN = 0.35  # the axis runs this far past the threshold, down the runway
GRADE_COL = {"OK": ("#2ea043", "#fff"), "_OK_": ("#2ea043", "#fff"), "(OK)": ("#d4a72c", "#000"), "B": ("#d1832b", "#000"),
             "C": ("#d1832b", "#000"), "WO": ("#8b949e", "#000"), "CUT": ("#da3633", "#fff")}


def badge_colours(grade):
    key = grade.split(":")[0].strip()
    return GRADE_COL.get(key, ("#d4a72c", "#000"))


YEL, GRN, RED, WHITE, MUTED = "#e8b400", "#2ea043", "#f85149", "#f0f6fc", "#8b949e"  # same as pass_cards.py
HAND = 'font-family="Arial, Helvetica, sans-serif" font-style="italic"'
MILES = ((0.25, "1/4 NM"), (0.5, "1/2 NM"), (0.75, "3/4 NM"), (1.0, "1 NM"), (1.5, "1.5 NM"))
SLOPES = ((0.5, YEL, "high"), (0.0, GRN, "on slope"), (-0.7, RED, "low"))  # degrees off the 3 deg path: the Hornet card's three lines
LINEUPS = ((0.4, GRN), (0.75, YEL), (1.5, RED))  # degrees either side of the centreline, as on the Hornet card


def _txt(x, y, t, size=13, colour=MUTED, anchor="start", extra=""):
    return '<text x="%.1f" y="%.1f" fill="%s" font-size="%d" text-anchor="%s" %s>%s</text>' % (x, y, colour, size, anchor, extra, html.escape(t))


def _poly(pts, colour, width=2.6, dash=""):
    return '<polyline fill="none" stroke="%s" stroke-width="%s" stroke-dasharray="%s" points="%s"/>' % (
        colour, width, dash, " ".join("%.1f,%.1f" % q for q in pts))


def _arrow(x1, y1, x2, y2):
    """A hand-drawn style arrow from a label to the point it names (as pass_cards.arrow)."""
    dx, dy = x2 - x1, y2 - y1
    cx, cy = (x1 + x2) / 2 - dy * 0.18, (y1 + y2) / 2 + dx * 0.18
    ang = math.atan2(y2 - cy, x2 - cx)
    a = (x2 - 9 * math.cos(ang + 0.45), y2 - 9 * math.sin(ang + 0.45))
    c = (x2 - 9 * math.cos(ang - 0.45), y2 - 9 * math.sin(ang - 0.45))
    return ('<path d="M%.1f %.1f Q%.1f %.1f %.1f %.1f" fill="none" stroke="%s" stroke-width="1.6"/>'
            '<polyline points="%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="none" stroke="%s" stroke-width="1.6"/>'
            % (x1, y1, cx, cy, x2, y2, YEL, a[0], a[1], x2, y2, c[0], c[1], YEL))


def _at(trk, nm):
    """The track point nearest nm out, or None when the track never got within 0.3 nm of it."""
    best = min(((abs(d - nm), d, ft, m) for d, ft, m in trk), default=None)
    return best[1:] if best and best[0] < 0.3 else None


def side_view(trk, td_past_m, nm_max=1.65, ft_max=720, coarse=False):
    """Height above the runway against distance, with the Hornet card's three slope lines (3 deg +0.5 / 0 / -0.7,
    from the aim point) and the mile arcs."""
    L, R, T, B = 60, W - 24, 24, H - 50
    sx = lambda nm: R - ((nm + NM_MIN) / (nm_max + NM_MIN)) * (R - L)
    sy = lambda ft: B - (ft / ft_max) * (B - T)
    aim = -AimM / 1852.0
    o = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg" font-family="Arial">' % (W, H)]
    for ft in range(100, int(ft_max), 100):
        o.append('<line x1="%d" y1="%.0f" x2="%d" y2="%.0f" stroke="#21262d"/>' % (L, sy(ft), R, sy(ft)))
        o.append(_txt(L - 6, sy(ft) + 4, str(ft), 13, MUTED, "end"))
    for nm, lab in MILES:
        o.append('<path d="M%.1f %d q9 %d 0 %d" fill="none" stroke="#484f58" stroke-width="1"/>' % (sx(nm) - 5, T, (B - T) / 2, B - T))
        o.append(_txt(sx(nm), B + 22, lab, 13, MUTED, "middle"))
    for i, (off, col, lab) in enumerate(SLOPES):
        o.append(_txt(R - 4, 44 + 20 * i, "%s  %.1f° (%+.1f)" % (lab, GS + off, off), 14, col, "end"))
        t = math.tan(math.radians(GS + off))
        end = nm_max
        o.append(_poly([(sx(aim), sy(0)), (sx(end), sy((end - aim) * NM_FT * t))], col, 1.6, "5 4"))
    o.append('<rect x="%.1f" y="%.1f" width="%.1f" height="6" fill="#6e7681"/>' % (sx(0), sy(0), R - sx(0) + 24))
    o.append(_txt(sx(0), B + 22, "threshold", 13, MUTED, "middle"))
    o.append(_poly([(sx(d), sy(h)) for d, h, _ in trk if d <= nm_max], WHITE))
    o.append('<circle cx="%.1f" cy="%.1f" r="6" fill="%s"/>' % (sx(-td_past_m / 1852.0), sy(0), YEL))
    o.append(_txt(sx(-td_past_m / 1852.0), sy(0) - 34, "TD %d m" % td_past_m, 14, "#e6edf3", "middle"))
    one = None if coarse else _at(trk, 1.0)
    if one:  # the height at one mile, like "Alt at 1 NM" on the Hornet card
        d, ft, _ = one
        o.append(_txt(sx(d) - 150, sy(ft) + 120, "Alt at 1 NM: %d ft" % round(ft), 17, YEL, "start", HAND))
        o.append(_arrow(sx(d) - 60, sy(ft) + 106, sx(d) - 2, sy(ft) + 6))
    o.append(_txt(R, 16, "side view, glide path %.1f degrees" % GS, 12, MUTED, "end"))
    o.append('</svg>')
    return "".join(o)


def lineup_view(trk, td_right_m, td_past_m, nm_max=1.65):
    """From above: the centreline, the Hornet card's lineup lines (0.4 / 0.75 / 1.5 deg each side) and the mile arcs."""
    L, R, M = 60, W - 24, H // 2 - 10
    k = 2.0  # pixels per metre across, stretched so the lineup lines are readable
    sx = lambda nm: R - ((nm + NM_MIN) / (nm_max + NM_MIN)) * (R - L)
    sy = lambda m: M + m * k
    o = ['<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg" font-family="Arial">' % (W, H)]
    o.append('<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="#30363d"/>' % (sx(0), sy(-23), R - sx(0) + 24, sy(23) - sy(-23)))
    o.append(_poly([(L, M), (R, M)], "#c9d1d9", 1.0, "2 5"))
    for deg, col in LINEUPS:
        for sign in (1, -1):
            o.append(_poly([(sx(0), M), (sx(nm_max), sy(sign * math.tan(math.radians(deg)) * nm_max * 1852.0))], col, 1.4, "5 4"))
    for nm, lab in MILES:
        o.append('<path d="M%.1f %d q9 %d 0 %d" fill="none" stroke="#484f58" stroke-width="1"/>' % (sx(nm) - 5, 24, (H - 78) / 2, H - 78))
        o.append(_txt(sx(nm), H - 18, lab, 13, MUTED, "middle"))
    o.append(_poly([(sx(d), sy(m)) for d, _, m in trk if d <= nm_max], WHITE))
    o.append('<circle cx="%.1f" cy="%.1f" r="6" fill="%s"/>' % (sx(-td_past_m / 1852.0), sy(td_right_m), YEL))
    o.append(_txt(L, 40, "left", 14))
    o.append(_txt(L, H - 50, "right", 14))
    for i, (deg, col) in enumerate(LINEUPS):
        o.append(_txt(R - 4, 44 + 20 * i, "lineup line ±%g°" % deg, 14, col, "end"))
    o.append(_txt(R, 16, "from above, lineup", 12, MUTED, "end"))
    o.append('</svg>')
    return "".join(o)


def card_html(r, trk, title, when, footer):
    fill, ink = badge_colours(r["grade"])
    segs = "".join("<tr><td>%s</td><td>%+.1f&deg;</td><td>%s</td><td>%.0f fpm</td></tr>" % (
        s["name"], s["gp"], ("%+.1f&deg;" % s["lu"]) if s.get("lu") is not None else "-", s["sink"]) for s in r["segs"])
    lines = ["%s | %s" % (r["field"], r["map"]), when, "%s | rwy %s | %s" % (r["model"], r["runway"], r["outcome"]), "",
             "Touchdown %d m past the threshold, %+d m %s of centreline, %d fpm" % (
                 r["td_past"], abs(r["td_right"]), "right" if r["td_right"] >= 0 else "left", r["td_sink"]), "", r["remark"]]
    body = "".join("<div>%s</div>" % (html.escape(x) if x else "&nbsp;") for x in lines)
    return """<!doctype html><html><head><meta charset="utf-8"><style>
body{margin:0;background:#0d1117;font-family:Arial,Helvetica,sans-serif;color:#c9d1d9}
.w{margin:8px;border:1px solid #30363d;background:#161b22;border-radius:6px;padding:14px 16px;width:1696px}
.top{display:flex;align-items:center;gap:18px;margin-bottom:10px}
.b{min-width:96px;height:58px;line-height:58px;text-align:center;border-radius:7px;font:700 32px Arial,sans-serif;background:%s;color:%s;padding:0 14px}
.n{font:700 28px Arial,sans-serif;color:#e6edf3}.cs{font:700 24px Arial,sans-serif;color:#fff;background:#0a8fd8;border-radius:7px;padding:8px 16px}
.logo{margin-left:auto;height:92px;width:auto}
.ps{display:flex;gap:16px}.pn{flex:none;width:840px;background:#0d1117;border:1px solid #30363d;border-radius:6px;overflow:hidden}
.tab{background:#1f6feb;color:#fff;font:700 22px Arial,sans-serif;text-align:center;padding:9px 0}
.row{display:flex;gap:40px;margin-top:14px}.info{font-size:20px;line-height:1.35;color:#e6edf3;flex:1}
table{border-collapse:collapse;font-size:18px;color:#e6edf3}td,th{padding:4px 16px;border-bottom:1px solid #30363d;text-align:right}th{color:#8b949e;font-weight:400}
.ft{font:12px Arial,sans-serif;color:#8b949e;margin-top:8px}
</style></head><body><div class="w"><div class="top"><div class="b">%s</div><div class="n">%s</div><div class="cs">%s</div><img class="logo" src="%s"></div>
<div class="ps"><div class="pn"><div class="tab">Glide View</div>%s</div><div class="pn"><div class="tab">Lineup View</div>%s</div></div>
<div class="row"><div class="info">%s</div><table><tr><th>Segment</th><th>Glide path</th><th>Lineup</th><th>Sink</th></tr>%s</table></div>
<div class="ft">%s</div></div></body></html>""" % (
        fill, ink, html.escape(r["grade"].split(":")[0]), html.escape(title), html.escape(r["field"].upper()), LOGO,
        side_view(trk, r["td_past"], coarse=r.get("coarse", False)), lineup_view(trk, r["td_right"], r["td_past"]), body, segs, html.escape(footer))


FAULTS = {"LUL": "lined up left", "LUR": "lined up right", "SINK": "sink rate high", "SLO": "slow", "LO": "low", "H": "high", "F": "fast",
          "LRO": "late rollout", "SHT": "touched down short", "LNG": "touched down long", "HARD": "hard landing"}
WHERE = {"X": "at the start", "IM": "in the middle", "IC": "in close", "AR": "at the ramp", "TD": "at touchdown"}
CODE_RE = re.compile(r"(__)?(\()?(LUL|LUR|SINK|SLO|LRO|SHT|LNG|HARD|LO|H|F)(\))?(__)?(X|IM|IC|AR|TD)?\b")


def remark_words(grade):
    """The grade's fault codes in words: "(LUL)IM" is "a little lined up left in the middle"."""
    out = []
    for m in CODE_RE.finditer(grade.split(":", 1)[1] if ":" in grade else ""):
        amount = "VERY " if m.group(1) or m.group(5) else "a little " if m.group(2) else ""
        out.append("%s%s%s" % (amount, FAULTS[m.group(3)], (" " + WHERE[m.group(6)]) if m.group(6) else ""))
    return ("; ".join(out) + ".").capitalize() if out else "Good pass."


def from_log(summary, track=""):
    """Build the card's inputs from an "Approach graded" summary and, if the rig logged one, an "Approach track" string.
    Without a track the path is drawn through the four segment averages (r["coarse"])."""
    m = re.match(r"(.+?) \((.+?)\): rwy (\S+) (.+?), grade (.+?)(?:,|;)", summary)
    cs, model, rwy, outcome, grade = m.groups()
    td = re.search(r"touched (\d+) m past the threshold, ([+-]\d+) m right, (\d+) fpm", summary)
    segs = [{"name": n, "gp": float(g), "lu": float(l) if l else None, "sink": float(k)} for n, g, l, k in
            re.findall(r"; (X|IM|IC|AR) GP ([+-][\d.]+)\S*(?: LU ([+-][\d.]+)\S*)? sink (\d+)", summary)]
    if track:
        trk = [(float(a) / 1852.0, float(b), float(c)) for a, b, c, _ in (q.split(":") for q in track.split(","))]
    else:
        mid = {"X": 1.25, "IM": 0.75, "IC": 0.375, "AR": 0.125}
        trk = [(mid[g["name"]], (mid[g["name"]] * 1852 + AimM) * math.tan(math.radians(GS + g["gp"])) * 3.28084,
                math.tan(math.radians(g["lu"] or 0.0)) * mid[g["name"]] * 1852) for g in segs]
        trk.append((-int(td.group(1)) / 1852.0, 0.0, float(td.group(2))))
    names = {"FA-18C_hornet": "F/A-18C", "F-16C_50": "F-16C", "F-16C": "F-16C"}
    r = {"field": "Senaki", "map": "Caucasus", "model": names.get(model, model.replace("_", " ")), "runway": rwy,
         "outcome": outcome, "grade": grade, "td_past": int(td.group(1)), "td_right": int(td.group(2)), "td_sink": int(td.group(3)),
         "remark": remark_words(grade), "segs": segs, "coarse": not track}
    return cs, r, trk


def sample():
    r = {"field": "Senaki", "map": "Caucasus", "model": "F-16C", "runway": "09", "outcome": "landed", "grade": "OK: (LO)IC",
         "td_past": 280, "td_right": 6, "td_sink": 540,
         "remark": "Slightly low in close, corrected. Good touchdown.",
         "segs": [{"name": "X", "gp": 0.1, "lu": 0.2, "sink": 690}, {"name": "IM", "gp": 0.4, "lu": -0.3, "sink": 700},
                  {"name": "IC", "gp": -0.6, "lu": 0.2, "sink": 640}, {"name": "AR", "gp": -0.2, "lu": 0.1, "sink": 560}]}
    def lerp(d, pts):
        for (d0, v0), (d1, v1) in zip(pts, pts[1:]):
            if d1 <= d <= d0:
                return v1 + (v0 - v1) * (d - d1) / (d0 - d1)
        return pts[-1][1]
    gp_pts = [(1.6, 0.1), (1.0, 0.3), (0.5, 0.4), (0.25, -0.6), (0.0, -0.1), (-0.3, 0.0)]   # deg off the 3 deg path
    off_pts = [(1.6, 25.0), (1.0, 6.0), (0.5, -3.0), (0.25, 2.0), (0.0, 5.0), (-0.3, 6.0)]  # metres right of centreline
    trk = []
    for i in range(0, 200):
        d = 1.6 - i * 0.01
        h = max(0.0, (d + 300 / 1852.0) * NM_FT * 0.0524 * (1 + lerp(d, gp_pts) / 3.0)) if d > -0.15 else 0.0
        trk.append((d, h, lerp(d, off_pts)))
    return r, trk



FIELD_WEBHOOK_FILE = os.path.join(os.path.expanduser("~"), "readback-field-cards-webhook.txt")  # the field cards' channel (Venom + Raider)
FIELD_STATE_FILE = os.path.join(os.path.expanduser("~"), "readback-field-cards.json")           # passes already posted
FIELD_FLIGHTS = ("Venom", "Raider")
FIELD_RIGS = (("Day", "192.168.1.220"), ("Night", "192.168.1.221"))


def pilot_names():
    out = {}
    try:
        for line in open(os.path.join(os.path.expanduser("~"), "readback-pilots.csv"), encoding="utf-8-sig"):
            c = line.rstrip("\r\n").split(",")
            if len(c) > 1:
                out[c[0].strip().lower()] = c[1].strip()
    except OSError:
        pass
    return out


def day_passes(date):
    """(time, callsign, summary, track) for each graded Venom / Raider landing in the Senaki Tower day logs of the rigs."""
    out = []
    for rig, host in FIELD_RIGS:
        try:
            url = "http://%s:7000/api/day-log?name=%s&date=%s" % (host, urllib.parse.quote("Senaki Tower"), date)
            lines = urllib.request.urlopen(url, timeout=60).read().decode("utf-8", "replace").splitlines()
        except OSError as e:
            print("%s: no Senaki Tower log (%s)" % (rig, e))
            continue
        recs = []
        for line in lines:
            try:
                recs.append(json.loads(line))
            except ValueError:
                pass
        tracks = [j for j in recs if j.get("message") == "Approach track"]
        for j in recs:
            if j.get("message") == "Approach graded" and j["callsign"].startswith(FIELD_FLIGHTS) and "landed" in j["summary"]:
                t = dt.datetime.fromisoformat(j["time"])
                # the track line is logged right after its graded line
                tk = next((x["track"] for x in tracks if x["callsign"] == j["callsign"] and
                           abs((dt.datetime.fromisoformat(x["time"]) - t).total_seconds()) < 5), "")
                out.append((j["time"], j["callsign"], j["summary"], tk))
    return sorted(out)


def post_field_cards(date, dry=False):
    hook = open(FIELD_WEBHOOK_FILE, encoding="utf-8").read().strip()
    try:
        done = set(json.load(open(FIELD_STATE_FILE)))
    except (OSError, ValueError):
        done = set()
    names = pilot_names()
    n = 0
    for when, cs, summary, track in day_passes(date):
        key = "%s|%s" % (when, cs)
        if key in done:
            continue
        _, r, trk = from_log(summary, track)
        t = dt.datetime.fromisoformat(when)
        title = "%s | %s" % (cs, names[cs.lower()]) if cs.lower() in names else cs
        foot = "Senaki field landings - " + ("segment averages only: logged before tracks" if r["coarse"] else "track from the tower")
        png = gp.screenshot(card_html(r, trk, title, t.strftime("%A %B %d, %Y %H:%M:%S") + " (real time)", foot), 960, 1730)
        if dry:
            open(os.path.join(os.path.expanduser("~"), "field-card-%s.png" % cs.replace(" ", "")), "wb").write(png)
        else:
            gp.send(hook + "?wait=true", "POST", png, "**%s** - Senaki rwy %s: %s" % (title, r["runway"], r["grade"]))
            done.add(key)
            json.dump(sorted(done), open(FIELD_STATE_FILE, "w"))
        n += 1
        print("%s %s %s" % ("drew" if dry else "posted", cs, r["grade"]))
    return n


if __name__ == "__main__" and sys.argv[1:2] == ["--post"]:
    post_field_cards(sys.argv[2] if len(sys.argv) > 2 else dt.date.today().isoformat(), dry="--dry" in sys.argv)
elif __name__ == "__main__" and len(sys.argv) > 2:
    lines = open(sys.argv[2], encoding="utf-8").read().splitlines()
    cs, r, trk = from_log(lines[0], lines[1])
    open(sys.argv[1], "wb").write(gp.screenshot(card_html(r, trk, cs, "Saturday November 07, 2026 (SIMULATED PASS)", "Senaki field landings - simulated pass through the real grader"), 960, 1730))
    print(sys.argv[1])
elif __name__ == "__main__":
    r, trk = sample()
    png = gp.screenshot(card_html(r, trk, "Sample Viper 1-1", "Saturday November 07, 2026 14:20:00 (SAMPLE DATA)",
                                  "Senaki field landings - SAMPLE DATA, not a real pass"), 960, 1730)
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.expanduser("~"), "field-card-sample.png")
    open(out, "wb").write(png)
    print(out)
