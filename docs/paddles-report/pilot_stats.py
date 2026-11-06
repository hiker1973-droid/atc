# -*- coding: utf-8 -*-
"""Per-pilot numbers for the weekly report and the pass cards (operator 2026-11-06: carrier-qualification
tracker, trends, wind over deck, recovery summaries).

Everything here is computed from the same passes the greenie board draws (greenie_post.load_passes: the
Paddles logs, the day summaries, the hand-added passes and the corrections), so the numbers always agree
with the board.

CQ requirement: 6 day and 4 night traps (the usual Navy numbers), counted from "since" on. Change it in
%USERPROFILE%\\readback-cq.json:  {"day": 6, "night": 4, "since": "2026-10-17"}
A trap flown in Case III is a night trap; anything else counts as day (older passes carry no case).
"""
import collections
import datetime as dt
import json
import math
import os
import statistics

import greenie_post as gp

CQ_FILE = os.path.join(os.path.expanduser("~"), "readback-cq.json")
HORNET_ON_SPEED_AOA = 8.1  # degrees (pkg/lso hornetOnSpeed)


def cq_config():
    cfg = {"day": 6, "night": 4, "since": "2026-10-17"}  # 17 Oct = the first Paddles logs
    if os.path.exists(CQ_FILE):
        try:
            cfg.update(json.load(open(CQ_FILE, encoding="utf-8")))
        except ValueError:
            pass
    return cfg


def is_night(p):
    return bool(p.get("case3")) or p.get("case") == "III"


def grouped(passes):
    """{callsign key: [passes, oldest first]} for roster pilots only, and the roster's labels."""
    names = {cs: label for cs, label, _, _ in gp.roster()}
    canon = {a: cs for cs, _, _, al in gp.roster() for a in al if cs}
    out = collections.defaultdict(list)
    for p in sorted(passes, key=lambda p: p["t"]):
        key = canon.get(p["callsign"].lower(), p["callsign"].lower())
        if names and key not in names:
            continue
        out[key].append(p)
    return out, names


def stats(plist):
    """Numbers for one pilot's passes (oldest first)."""
    scored = [x for x in (gp.points(p) for p in plist) if x is not None]
    traps = [p for p in plist if p["outcome"] == "trap"]
    bolters = [p for p in plist if p["outcome"] == "bolter"]
    wos = [p for p in plist if p["outcome"] == "wave off" and gp.board_key(p) != "WOFD"]
    last10 = [x for x in (gp.points(p) for p in plist[-10:]) if x is not None]
    wires = [p for p in traps if p.get("wire")]
    return {
        "n": len(plist), "traps": len(traps), "bolters": len(bolters), "wos": len(wos),
        "avg": statistics.mean(scored) if scored else None,
        "avg10": statistics.mean(last10) if last10 else None,
        "boarding": 100.0 * len(traps) / (len(traps) + len(bolters)) if traps or bolters else None,
        "three": 100.0 * sum(1 for p in wires if p["wire"] == 3) / len(wires) if wires else None,
        "fault": top_fault(plist),
    }


def top_fault(plist):
    """The most common LSO fault in these passes, in words ("low in close"), or ""."""
    import pass_cards as pc
    count = collections.Counter()
    for p in plist:
        for m in pc.REMARK_RE.finditer(p.get("comments") or ""):
            count[(pc.WORDS[m.group(3)], pc.WHERE[m.group(6)].split(" (")[0])] += 1
    if not count:
        return ""
    (word, where), n = count.most_common(1)[0]
    return "%s %s (%d)" % (word, where, n)


def cq_progress(plist, cfg=None):
    """(day traps, night traps) since the CQ start date."""
    cfg = cfg or cq_config()
    since = dt.date.fromisoformat(cfg["since"])
    day = night = 0
    for p in plist:
        if p["outcome"] == "trap" and p["t"].date() >= since:
            if is_night(p):
                night += 1
            else:
                day += 1
    return day, night


def cq_text(plist, cfg=None):
    cfg = cfg or cq_config()
    d, n = cq_progress(plist, cfg)
    return "CQ day %d/%d, night %d/%d" % (min(d, cfg["day"]), cfg["day"], min(n, cfg["night"]), cfg["night"])


def wind_over_deck(ctx, brc):
    """(wind over deck, headwind, crosswind) in knots from the ship's speed, the wind and the BRC; None without them.
    Crosswind is positive from starboard."""
    if not ctx or brc is None or ctx.get("windDir") is None:
        return None
    ship = ctx.get("shipKts") or 0
    rel = math.radians((ctx.get("windDir") or 0) - brc)
    kts = ctx.get("windKts") or 0
    head, cross = kts * math.cos(rel), kts * math.sin(rel)
    return ship + head, head, cross


def pilot_line(label, plist):
    """One line for a pass card: "Raider 302 | Jedi: 31 passes, avg 2.4 (last 10 2.9), boarding 83%, CQ day 4/6, night 1/4"."""
    s = stats(plist)
    if not s["n"]:
        return ""
    bits = ["%d passes" % s["n"]]
    if s["avg"] is not None:
        bits.append("avg %.1f%s" % (s["avg"], " (last 10: %.1f)" % s["avg10"] if s["avg10"] is not None and s["n"] > 10 else ""))
    if s["boarding"] is not None:
        bits.append("boarding %d%%" % round(s["boarding"]))
    return "%s: %s, %s" % (label, ", ".join(bits), cq_text(plist))
