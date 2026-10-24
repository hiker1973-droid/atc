# -*- coding: utf-8 -*-
"""Nightly Paddles report: fetch a day of Paddles logs from each rig and build the PDF.

Windows Task Scheduler on the dev box runs this at 00:05 for the day before
(task "SkyEye Paddles Report"; operator 2026-10-18: PDF in Documentation,
midnight, all three rigs). Each rig's launcher serves the day from
/api/lso-log?date=; the raw lines are kept beside the PDF for tuning Paddles.
The Senaki tower's day (/api/day-log) adds the graded field landings.

Run:  python -I docs/paddles-report/run_nightly.py [--date YYYY-MM-DD]
"""
import argparse
import datetime as dt
import os
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
RIGS = [("Day", "192.168.1.220"), ("Night", "192.168.1.221"), ("Foothold", "192.168.1.222")]
# Field landings are graded by these towers (start_towers_caucasus.bat
# --approach-grade); Foothold is Syria, no Senaki.
FIELD_ROLES = [("Day", "Senaki Tower"), ("Night", "Senaki Tower")]
# One Reports folder (operator 2026-10-21): Reports\YYYY-MM\YYYY-MM-DD <Kind>.pdf,
# raw data under Reports\_data\<kind>.
REPORTS = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Reports"


def report_pdf(root, date, kind):
    """Reports\2026-10\2026-10-20 Health.pdf - every night's reports side by side, a folder a month."""
    d = os.path.join(root, date[:7])
    os.makedirs(d, exist_ok=True)
    return os.path.join(d, "%s %s.pdf" % (date, kind))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", default=(dt.date.today() - dt.timedelta(days=1)).isoformat())
    ap.add_argument("--out", default=REPORTS)
    a = ap.parse_args()
    data = os.path.join(a.out, "_data", "paddles")
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
    hosts = dict(RIGS)
    for rig, role in FIELD_ROLES:
        url = "http://%s:7000/api/day-log?name=%s&date=%s" % (hosts[rig], urllib.parse.quote(role), a.date)
        try:
            with urllib.request.urlopen(url, timeout=60) as r:
                body, ctype = r.read(), r.headers.get("Content-Type", "")
        except (urllib.error.URLError, OSError) as e:
            problems.append("%s %s: %s" % (rig, role, e))
            continue
        if "html" in ctype:
            continue
        # Not "YYYY-MM-DD Rig.log": the greenie board reads those.
        path = os.path.join(data, "field", "%s %s %s.log" % (a.date, rig, role))
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as f:
            f.write(body)
        logs += ["--field-log", "%s=%s" % (rig, path)]
    cmd = [sys.executable, "-I", os.path.join(HERE, "build_paddles_report.py"), "--date", a.date,
           "--out", data, "--no-copy", "--board-dir", data,
           "--summary-dir", os.path.join(a.out, "_summary")] + logs
    for p in problems:
        cmd += ["--problem", p]
    rc = subprocess.call(cmd)
    built = os.path.join(data, "Paddles Report %s.pdf" % a.date)
    if rc == 0 and os.path.exists(built):
        os.replace(built, report_pdf(a.out, a.date, "Paddles"))
    return rc


if __name__ == "__main__":
    sys.exit(main())
