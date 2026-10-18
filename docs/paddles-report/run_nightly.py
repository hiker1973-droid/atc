# -*- coding: utf-8 -*-
"""Nightly Paddles report: fetch a day of Paddles logs from each rig and build the PDF.

Windows Task Scheduler on the dev box runs this at 00:05 for the day before
(task "SkyEye Paddles Report"; operator 2026-10-18: PDF in Documentation,
midnight, all three rigs). Each rig's launcher serves the day from
/api/lso-log?date=; the raw lines are kept beside the PDF for tuning Paddles.

Run:  python -I docs/paddles-report/run_nightly.py [--date YYYY-MM-DD]
"""
import argparse
import datetime as dt
import os
import subprocess
import sys
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
RIGS = [("Day", "192.168.1.220"), ("Night", "192.168.1.221"), ("Foothold", "192.168.1.222")]
OUT = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation\Paddles Reports"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", default=(dt.date.today() - dt.timedelta(days=1)).isoformat())
    ap.add_argument("--out", default=OUT)
    a = ap.parse_args()
    data = os.path.join(a.out, "data")
    os.makedirs(data, exist_ok=True)

    logs, problems = [], []
    for rig, host in RIGS:
        url = "http://%s:7000/api/lso-log?date=%s" % (host, a.date)
        try:
            with urllib.request.urlopen(url, timeout=60) as r:
                body, ctype = r.read(), r.headers.get("Content-Type", "")
        except (urllib.error.URLError, OSError) as e:
            problems.append("%s: %s" % (rig, e))
            continue
        if "html" in ctype:
            # An older launcher has no /api/lso-log and serves its dashboard.
            problems.append("%s: launcher has no /api/lso-log yet (needs an Update)" % rig)
            continue
        path = os.path.join(data, "%s %s.log" % (a.date, rig))
        with open(path, "wb") as f:
            f.write(body)
        logs += ["--log", "%s=%s" % (rig, path)]
    if not logs:
        print("no rig answered:", "; ".join(problems), file=sys.stderr)
        return 1
    cmd = [sys.executable, "-I", os.path.join(HERE, "build_paddles_report.py"), "--date", a.date,
           "--out", a.out, "--no-copy"] + logs
    for p in problems:
        cmd += ["--problem", p]
    return subprocess.call(cmd)


if __name__ == "__main__":
    sys.exit(main())
