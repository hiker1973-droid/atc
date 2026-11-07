# -*- coding: utf-8 -*-
"""Pushes the greenie board and the pass card the moment a pass is graded.

Operator 2026-11-06: "push the cards and board once a pass has been done (instant)".
Runs all the time on the dev box (scheduled task "Readback Greenie Watch", started at
logon, restarted if it stops): every 15 seconds it counts the "Paddles: pass" lines in
each rig's log for today; when one rig has a new one it waits a few seconds, so the
pass's track, pattern and context lines are written too, then runs
greenie_post.py --refresh, which redraws the board, edits the Discord message and
posts a card for each new pass. The hourly task stays as the backstop.

Two more jobs ride on the same loop: twenty minutes after the last pass of a recovery it posts the
recovery summary (recovery_summary.py --post), and on Sunday after 20:00 it posts the weekly report
(weekly_report.py --post) once, as a new message in the greenie board's channel, and refreshes the
rolling 30-day monthly report (weekly_report.py --monthly --post), one message edited in place, and
posts the Marshal report (marshal_report.py --post) and the pattern report (pattern_report.py --post).

Run:  pythonw -I docs/paddles-report/greenie_watch.py
"""
import datetime as dt
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
RIGS = [("Day", "192.168.1.220"), ("Night", "192.168.1.221"), ("Foothold", "192.168.1.222")]
POLL_S = 15
QUIET_S = 20 * 60 + 30  # a recovery is over when no pass has come for this long
WEEK_FILE = os.path.join(os.path.expanduser("~"), "readback-greenie-weekly.json")
SETTLE_S = 8  # the pass line is the last of the pass's log lines; give the rest a moment to be written
MARK = b'"message":"Paddles: pass"'


def count(host, day):
    """The number of graded passes in this rig's Paddles log for the day, or None if the rig didn't answer."""
    try:
        with urllib.request.urlopen("http://%s:7000/api/lso-log?date=%s" % (host, day), timeout=20) as r:
            body = r.read()
            if "html" in r.headers.get("Content-Type", ""):
                return None
    except (urllib.error.URLError, OSError):
        return None
    return body.count(MARK)


def refresh():
    subprocess.run([sys.executable, "-I", "-B", os.path.join(HERE, "greenie_post.py"), "--refresh"],
                   timeout=300, capture_output=True)


def run(script, *args):
    try:
        subprocess.run([sys.executable, "-I", "-B", os.path.join(HERE, script), *args], timeout=300, capture_output=True)
    except (subprocess.SubprocessError, OSError):
        pass


def weekly_due(now):
    """Sunday from 20:00, once per ISO week."""
    week = now.strftime("%G-W%V")
    if now.weekday() != 6 or now.hour < 20:
        return None
    try:
        import json
        if json.load(open(WEEK_FILE, encoding="utf-8")).get("last") == week:
            return None
    except (OSError, ValueError):
        pass
    return week


def main():
    seen = {}
    last_pass, pending = 0.0, False
    day = dt.date.today().isoformat()
    while True:
        today = dt.date.today().isoformat()
        if today != day:  # a new day: start counting again
            day, seen = today, {}
        fresh = False
        for rig, host in RIGS:
            n = count(host, day)
            if n is None:
                continue
            if rig in seen and n > seen[rig]:
                fresh = True
            seen[rig] = n
        if fresh:
            time.sleep(SETTLE_S)
            try:
                refresh()
            except (subprocess.SubprocessError, OSError):
                pass
            last_pass, pending = time.time(), True
        if pending and time.time() - last_pass >= QUIET_S:
            # (the recovery summary post to the Trap house was dropped 2026-11-06, operator: not needed)
            run("marshal_card.py", "--post-new")  # a Marshal card for each finished approach
            pending = False
        week = weekly_due(dt.datetime.now())
        if week:
            import json
            json.dump({"last": week}, open(WEEK_FILE, "w", encoding="utf-8"))  # first, so a failed post is not retried every 15 s
            run("weekly_report.py", "--post")
            run("weekly_report.py", "--monthly", "--post")  # the rolling 30 days, edited in place
            run("marshal_report.py", "--post")  # how Marshal worked this week
            run("pattern_report.py", "--post")  # how the squadron flew the pattern (past 30 days)
        time.sleep(POLL_S)


if __name__ == "__main__":
    main()
