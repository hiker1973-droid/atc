# -*- coding: utf-8 -*-
"""A summary of each recovery (a run of passes on one rig with no gap over 30 minutes).

Operator 2026-11-06: recovery summaries from the Tacview-fed Paddles data. For each recovery: when it ran,
the case, BRC and wind over the deck, how many passes and how they went, and the time between traps.
greenie_watch.py runs `recovery_summary.py --post` once a recovery has been quiet for 20 minutes; the
weekly report lists the week's recoveries with summarise().

Run:  python -I docs/paddles-report/recovery_summary.py [--post]
"""
import datetime as dt
import json
import os
import statistics
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I does not put the script's folder on the path
import greenie_post as gp  # noqa: E402
import pass_cards as pc  # noqa: E402
import pilot_stats as ps  # noqa: E402

GAP = dt.timedelta(minutes=30)
QUIET = dt.timedelta(minutes=20)
STATE_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-sessions.json")


def sessions(passes):
    """Recoveries: logged passes (they carry a rig) grouped by rig, split where the gap exceeds 30 minutes."""
    out = []
    by_rig = {}
    for p in sorted((p for p in passes if p.get("rig")), key=lambda p: p["t"]):
        by_rig.setdefault(p["rig"], []).append(p)
    for rig, plist in by_rig.items():
        cur = [plist[0]]
        for p in plist[1:]:
            if p["t"] - cur[-1]["t"] > GAP:
                out.append((rig, cur))
                cur = []
            cur.append(p)
        out.append((rig, cur))
    return sorted(out, key=lambda s: s[1][0]["t"])


def session_key(rig, plist):
    return "%s|%s" % (rig, plist[0]["t"].isoformat())


def summarise(rig, plist, contexts, patterns):
    """The numbers for one recovery."""
    traps = [p for p in plist if p["outcome"] == "trap"]
    st = ps.stats(plist)
    gaps = [(b["t"] - a["t"]).total_seconds() for a, b in zip(traps, traps[1:])]
    ctx = None
    brc = None
    for p in reversed(plist):
        ctx = ctx or pc.context_for(p, contexts)
        brc = brc or pc.brc_for(p, patterns)
    wod = ps.wind_over_deck(ctx, brc)
    cases = sorted({p.get("case") for p in plist if p.get("case")})
    grp, _ = ps.grouped(plist)
    best = max(((ps.stats(v)["avg"], k) for k, v in grp.items() if len(v) >= 2 and ps.stats(v)["avg"] is not None), default=None)
    return {"rig": rig, "start": plist[0]["t"], "end": plist[-1]["t"], "n": len(plist), "traps": st["traps"], "bolters": st["bolters"],
            "wos": st["wos"], "avg": st["avg"], "boarding": st["boarding"], "three": st["three"],
            "gap": statistics.median(gaps) if gaps else None, "case": "/".join(cases), "brc": brc, "ctx": ctx, "wod": wod,
            "best": best, "pilots": len({p["callsign"].lower() for p in plist})}


def clock(secs):
    return "%d:%02d" % (secs // 60, secs % 60)


def text(s, names=None):
    """The Discord message for one recovery."""
    names = names or {}
    mins = max(1, round((s["end"] - s["start"]).total_seconds() / 60))
    head = "**Recovery summary** - %s, %s %s-%s (%d min)" % (s["rig"], s["start"].strftime("%a %d %b"), s["start"].strftime("%H:%M"),
                                                            s["end"].strftime("%H:%M"), mins)
    bits = []
    if s["case"]:
        bits.append("Case " + s["case"])
    if s["brc"] is not None:
        bits.append("BRC %03d" % (round(s["brc"]) % 360 or 360))
    c = s["ctx"]
    if c and c.get("windDir") is not None:
        bits.append("wind %03d/%d" % (round(c["windDir"]) % 360 or 360, round(c.get("windKts") or 0)))
    if s["wod"]:
        bits.append("wind over deck %d kts (crosswind %d)" % (round(s["wod"][0]), abs(round(s["wod"][2]))))
    lines = [head, " | ".join(bits)] if bits else [head]
    lines.append("%d pass%s by %d pilot%s: %d traps, %d bolters, %d wave-offs - boarding rate %s" % (
        s["n"], "" if s["n"] == 1 else "es", s["pilots"], "" if s["pilots"] == 1 else "s", s["traps"], s["bolters"], s["wos"], "%d%%" % round(s["boarding"]) if s["boarding"] is not None else "-"))
    more = []
    if s["avg"] is not None:
        more.append("average %.1f" % s["avg"])
    if s["three"] is not None:
        more.append("3-wire %d%%" % round(s["three"]))
    if s["gap"]:
        more.append("median %s between traps" % clock(round(s["gap"])))
    if s["best"]:
        more.append("best %s %.1f" % (names.get(s["best"][1], s["best"][1]), s["best"][0]))
    if more:
        lines.append(" | ".join(more))
    return "\n".join(lines)


def main():
    post = "--post" in sys.argv
    days = {(dt.date.today() - dt.timedelta(days=i)).isoformat() for i in range(0, 3)}
    passes = [p for p in gp.load_passes(dt.date.today()) if p.get("rig") and p["t"].date().isoformat() in days]
    sess = sessions(passes)
    state = json.load(open(STATE_FILE, encoding="utf-8")) if os.path.exists(STATE_FILE) else None
    posted = set(state["posted"]) if state else set()
    now = dt.datetime.now().astimezone()
    names = {cs: label for cs, label, _, _ in gp.roster()}
    contexts, patterns = pc.read_context(days), pc.read_logged(days, "Paddles: pass pattern", "pattern")
    first = state is None and post
    hook = None
    for rig, plist in sess:
        key = session_key(rig, plist)
        quiet = now - plist[-1]["t"] >= QUIET
        if key in posted or not quiet:
            continue
        if first:
            posted.add(key)  # the first run only marks what already happened
            continue
        msg = text(summarise(rig, plist, contexts, patterns), names)
        print(msg)
        if post:
            if hook is None:
                hook = open(gp.CARDS_WEBHOOK_FILE if os.path.exists(gp.CARDS_WEBHOOK_FILE) else gp.WEBHOOK_FILE, encoding="utf-8").read().strip()
            import urllib.request
            req = urllib.request.Request(hook, json.dumps({"content": msg}).encode(), {"Content-Type": "application/json", "User-Agent": "ReadbackGreenie/1"})
            urllib.request.urlopen(req, timeout=30).read()
            time.sleep(1)
        posted.add(key)
    if post:
        json.dump({"posted": sorted(posted)[-500:]}, open(STATE_FILE, "w", encoding="utf-8"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
