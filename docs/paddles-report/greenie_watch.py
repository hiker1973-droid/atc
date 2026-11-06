# -*- coding: utf-8 -*-
"""Pushes the greenie board and the pass card the moment a pass is graded.

Operator 2026-11-06: "push the cards and board once a pass has been done (instant)".
Runs all the time on the dev box (scheduled task "Readback Greenie Watch", started at
logon, restarted if it stops): every 15 seconds it counts the "Paddles: pass" lines in
each rig's log for today; when one rig has a new one it waits a few seconds, so the
pass's track, pattern and context lines are written too, then runs
greenie_post.py --refresh, which redraws the board, edits the Discord message and
posts a card for each new pass. The hourly task stays as the backstop.

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


def main():
    seen = {}
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
        time.sleep(POLL_S)


if __name__ == "__main__":
    main()
