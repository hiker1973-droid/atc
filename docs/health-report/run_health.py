# -*- coding: utf-8 -*-
"""Nightly SkyEye health report: fetch every role's day log from each rig, build the PDF.

Windows Task Scheduler on the dev box runs this at 00:10 for the day before
(task "SkyEye Health Report"; operator 2026-10-18: "also do a daily health report
on ATC/ATIS/Marshall ect"). For each rig it takes the roles of the theatre(s)
running there (launcher /api/roles), one fetch per log file (Batumi Tower and
Deckboss share atc-ugsb.log), from /api/day-log; a launcher without it gives
its 32 KB tail instead and the report says so. Raw logs stay in the data
folder beside the PDF.

Run:  python -I docs/health-report/run_health.py [--date YYYY-MM-DD]
"""
import argparse
import datetime as dt
import json
import os
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
RIGS = [("Day", "192.168.1.220"), ("Night", "192.168.1.221"), ("Foothold", "192.168.1.222")]
# One Reports folder (operator 2026-10-21): Reports\YYYY-MM\YYYY-MM-DD <Kind>.pdf,
# raw data under Reports\_data\<kind>.
REPORTS = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Reports"


def report_pdf(root, date, kind):
    """Reports\2026-10\2026-10-20 Health.pdf - every night's reports side by side, a folder a month."""
    d = os.path.join(root, date[:7])
    os.makedirs(d, exist_ok=True)
    return os.path.join(d, "%s %s.pdf" % (date, kind))


def get(url, timeout=60):
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return r.read(), r.headers.get("Content-Type", "")


def short(name):
    for s in ("vSFG-7 ", " (Caucasus)", " (Syria)", " (Germany)", " (Iraq)", " (Kola)"):
        name = name.replace(s, "")
    return name


def kind(log_file):
    k = log_file[len("atc-"):-len(".log")] if log_file.startswith("atc-") else log_file
    return k if k in ("atis", "command", "marshal", "lso") else "tower"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", default=(dt.date.today() - dt.timedelta(days=1)).isoformat())
    ap.add_argument("--out", default=REPORTS)
    a = ap.parse_args()
    data = os.path.join(a.out, "_data", "health", a.date)
    os.makedirs(data, exist_ok=True)

    man = {"date": a.date, "rigs": [], "problems": []}
    for rig, host in RIGS:
        base = "http://%s:7000" % host
        try:
            roles = json.loads(get(base + "/api/roles", 15)[0])
        except (urllib.error.URLError, OSError, ValueError) as e:
            man["problems"].append("%s: %s" % (rig, e))
            continue
        entry = {"rig": rig, "host": host, "logs": []}
        try:
            entry["version"] = json.loads(get(base + "/api/version", 15)[0]).get("short", "")
        except (urllib.error.URLError, OSError, ValueError):
            pass
        try:
            usage = json.loads(get(base + "/api/elevenlabs-usage", 15)[0])
            entry["credits"] = (usage.get("byDay") or {}).get(a.date, 0)
        except (urllib.error.URLError, OSError, ValueError):
            pass
        regions = {r["region"] for r in roles if r.get("status") == "running" and r.get("region")}
        groups = {}
        for r in roles:
            if r.get("region") in regions and r.get("logFile"):
                groups.setdefault(r["logFile"], []).append(r["name"])
        partial = False
        for log_file, names in sorted(groups.items(), key=lambda kv: (kind(kv[0]) == "tower", kv[0])):
            q = urllib.parse.urlencode({"name": names[0], "date": a.date})
            try:
                body, ctype = get(base + "/api/day-log?" + q)
            except urllib.error.HTTPError as e:
                if e.code != 404:
                    man["problems"].append("%s %s: %s" % (rig, names[0], e))
                    continue
                # An older launcher: no /api/day-log. Its tail is better than nothing.
                body, ctype = get(base + "/api/log?" + urllib.parse.urlencode({"name": names[0]}))
                partial = True
            except (urllib.error.URLError, OSError) as e:
                man["problems"].append("%s %s: %s" % (rig, names[0], e))
                continue
            path = os.path.join(data, "%s %s" % (rig, log_file))
            with open(path, "wb") as f:
                f.write(body)
            label = " + ".join(sorted({short(n) for n in names}, key=lambda n: ("Tower" not in n, n)))
            running = any(x.get("status") == "running" for x in roles if x["name"] in names)
            entry["logs"].append({"label": label, "logFile": log_file, "kind": kind(log_file), "path": path,
                                  "running": running})
        if partial:
            man["problems"].append("%s: launcher has no /api/day-log yet (needs an Update) - last 32 KB of each "
                                   "log only" % rig)
        if not regions:
            man["problems"].append("%s: no theatre running" % rig)
        man["rigs"].append(entry)
    mpath = os.path.join(data, "manifest.json")
    with open(mpath, "w", encoding="utf-8") as f:
        json.dump(man, f, indent=1)
    if not man["rigs"]:
        print("no rig answered:", "; ".join(man["problems"]), file=sys.stderr)
        return 1
    rc = subprocess.call([sys.executable, "-I", os.path.join(HERE, "build_health_report.py"),
                          "--manifest", mpath, "--out", data])
    built = os.path.join(data, "SkyEye Health Report %s.pdf" % a.date)
    if rc == 0 and os.path.exists(built):
        os.replace(built, report_pdf(a.out, a.date, "Health"))
    return rc


if __name__ == "__main__":
    sys.exit(main())
