# -*- coding: utf-8 -*-
"""Nightly Readback running-costs report: OpenAI, AWS, ElevenLabs and rig power, one PDF.

Operator 2026-10-27: "add AWS, OpenAI and power estimate spend (I want us to keep
track on gui or a nightly report) ... just for us". Dev box only; nothing here
ships in the installer.

  OpenAI      exact, from the organization costs API (admin key in
              %USERPROFILE%\\readback-openai-admin-key.txt), by day (UTC) and line item.
  AWS         exact, from Cost Explorer (profile "omm-costs" in ~/.aws/credentials,
              user readback-costs, ce:GetCostAndUsage only), by day (UTC) and service.
              It is the OMM mod library's S3 bucket. Each call costs AWS $0.01.
  ElevenLabs  the plan's monthly fee spread over the days of the month.
  Power       an estimate: each machine's average watts x 24 h, priced on Xcel
              Colorado residential time-of-use (on-peak weekdays 17-21), times a
              factor for the bill's fees and riders. All in the config file.

Config:  %USERPROFILE%\\readback-costs.json (written with defaults on the first run).
Output:  Reports\\YYYY-MM\\YYYY-MM-DD Costs.pdf, Reports\\_data\\costs\\costs.json, and a copy
         of the data at %USERPROFILE%\\readback-costs-data.json for the launcher's
         Stats page (/api/costs).

Run:  python -I docs/costs/run_costs.py [--date YYYY-MM-DD] [--days 62]
"""
import argparse
import calendar
import datetime as dt
import json
import os
import statistics
import time
import urllib.error
import urllib.request

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

HOME = os.path.expanduser("~")
REPORTS = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Reports"
CONFIG = os.path.join(HOME, "readback-costs.json")
OPENAI_KEY = os.path.join(HOME, "readback-openai-admin-key.txt")
LAUNCHER_COPY = os.path.join(HOME, "readback-costs-data.json")
AWS_PROFILE = "omm-costs"

# Operator 2026-10-27: i9-14900KF, RTX 3060 Ti, 1000 W Gold PSU. Watts are a
# guess at the wall for a DCS dedicated server plus the Readback roles (CPU
# busy on a few cores, GPU near idle); correct them with a plug-in meter.
DEFAULT_CONFIG = {
    "machines": [
        {"name": "Day (.220)", "watts": 160, "hoursPerDay": 24},
        {"name": "Night (.221)", "watts": 160, "hoursPerDay": 24},
        {"name": "Foothold (.222)", "watts": 160, "hoursPerDay": 24},
        {"name": "Dev box (.231)", "watts": 140, "hoursPerDay": 24},
    ],
    # Xcel Colorado residential time-of-use energy charges, $/kWh (2026; on-peak
    # 17:00-21:00 Mon-Fri, off-peak every other hour). feesFactor scales them to
    # the bill's all-in price: set it to (bill total / kWh) / average energy rate.
    "power": {"offPeak": 0.075, "onPeakSummer": 0.213, "onPeakWinter": 0.18,
              "summerMonths": [6, 7, 8, 9], "onPeakStart": 17, "onPeakEnd": 21, "feesFactor": 1.25},
    "elevenLabsMonthlyUSD": 22.0,
}


def load_config():
    if not os.path.exists(CONFIG):
        with open(CONFIG, "w", encoding="utf-8") as f:
            json.dump(DEFAULT_CONFIG, f, indent=2)
        return json.loads(json.dumps(DEFAULT_CONFIG))
    with open(CONFIG, encoding="utf-8") as f:
        cfg = json.load(f)
    for k, v in DEFAULT_CONFIG.items():
        cfg.setdefault(k, v)
    return cfg


# ------------------------------------------------------------------ sources
def openai_kind(item):
    """TTS / STT / other from a costs line item."""
    s = (item or "").lower()
    if "tts" in s:
        return "tts"
    if "transcribe" in s or "whisper" in s:
        return "stt"
    return "other"


def fetch_openai(start, end):
    """{date: {tts, stt, other}} for [start, end), UTC days."""
    with open(OPENAI_KEY, encoding="utf-8") as f:
        key = f.read().strip()
    out, page = {}, None
    st = calendar.timegm(start.timetuple())
    et = calendar.timegm(end.timetuple())
    while True:
        url = ("https://api.openai.com/v1/organization/costs?start_time=%d&end_time=%d&bucket_width=1d"
               "&limit=180&group_by=line_item" % (st, et))
        if page:
            url += "&page=" + page
        req = urllib.request.Request(url, headers={"Authorization": "Bearer " + key})
        with urllib.request.urlopen(req, timeout=60) as r:
            data = json.load(r)
        for b in data.get("data", []):
            day = dt.datetime.fromtimestamp(b["start_time"], dt.timezone.utc).date().isoformat()
            row = out.setdefault(day, {"tts": 0.0, "stt": 0.0, "other": 0.0})
            for res in b.get("results", []):
                row[openai_kind(res.get("line_item"))] += float(res.get("amount", {}).get("value", 0) or 0)
        if not data.get("has_more"):
            return out
        page = data.get("next_page")


def fetch_aws(start, end):
    """{date: {service: usd}} for [start, end), UTC days."""
    import boto3
    ce = boto3.Session(profile_name=AWS_PROFILE).client("ce", region_name="us-east-1")
    out, token = {}, None
    while True:
        kw = dict(TimePeriod={"Start": start.isoformat(), "End": end.isoformat()}, Granularity="DAILY",
                  Metrics=["UnblendedCost"], GroupBy=[{"Type": "DIMENSION", "Key": "SERVICE"}])
        if token:
            kw["NextPageToken"] = token
        r = ce.get_cost_and_usage(**kw)
        for p in r["ResultsByTime"]:
            row = out.setdefault(p["TimePeriod"]["Start"], {})
            for g in p.get("Groups", []):
                amt = float(g["Metrics"]["UnblendedCost"]["Amount"])
                if amt:
                    row[g["Keys"][0]] = row.get(g["Keys"][0], 0.0) + amt
        token = r.get("NextPageToken")
        if not token:
            return out


def power_day(cfg, day):
    """({machine: usd}, kWh) for one local day."""
    p = cfg["power"]
    on_rate = p["onPeakSummer"] if day.month in p["summerMonths"] else p["onPeakWinter"]
    on_hours = (p["onPeakEnd"] - p["onPeakStart"]) if day.weekday() < 5 else 0
    per, kwh = {}, 0.0
    for m in cfg["machines"]:
        hrs = min(24, m.get("hoursPerDay", 24))
        kw = m["watts"] / 1000.0
        on = min(on_hours, hrs) * kw
        off = (hrs * kw) - on
        per[m["name"]] = (on * on_rate + off * p["offPeak"]) * p["feesFactor"]
        kwh += hrs * kw
    return per, kwh


# ------------------------------------------------------------------ data
def build_days(cfg, start, end, openai, aws, errors):
    days = {}
    d = start
    while d < end:
        k = d.isoformat()
        o = openai.get(k) if openai is not None else None
        a = aws.get(k) if aws is not None else None
        pw, kwh = power_day(cfg, d)
        el = cfg["elevenLabsMonthlyUSD"] / calendar.monthrange(d.year, d.month)[1]
        row = {
            "openai": None if o is None else dict(o, total=sum(o.values())),
            "aws": None if a is None else {"services": a, "total": sum(a.values())},
            "power": {"machines": pw, "total": sum(pw.values()), "kwh": kwh},
            "elevenlabs": el,
        }
        row["total"] = sum(x for x in (
            row["openai"]["total"] if row["openai"] else 0,
            row["aws"]["total"] if row["aws"] else 0,
            row["power"]["total"], el))
        days[k] = row
        d += dt.timedelta(days=1)
    return days


def merge(old, new):
    """Keep earlier days we no longer fetch; a source that failed tonight keeps last night's numbers."""
    out = dict(old)
    for k, row in new.items():
        prev = out.get(k, {})
        for src in ("openai", "aws"):
            if row[src] is None and prev.get(src):
                row[src] = prev[src]
        row["total"] = sum(x for x in (
            row["openai"]["total"] if row["openai"] else 0,
            row["aws"]["total"] if row["aws"] else 0,
            row["power"]["total"], row["elevenlabs"]))
        out[k] = row
    return dict(sorted(out.items()))


CATS = [("OpenAI", lambda r: r["openai"]["total"] if r.get("openai") else 0),
        ("AWS (OMM S3)", lambda r: r["aws"]["total"] if r.get("aws") else 0),
        ("ElevenLabs", lambda r: r["elevenlabs"]),
        ("Power", lambda r: r["power"]["total"])]


def summarise(days, day):
    """Tiles: the day, month to date, projected month, last month - per category and total."""
    first = day.replace(day=1)
    dim = calendar.monthrange(day.year, day.month)[1]
    last_end = first - dt.timedelta(days=1)
    last_first = last_end.replace(day=1)

    def span(a, b):
        rows = [r for k, r in days.items() if a.isoformat() <= k <= b.isoformat()]
        return {name: sum(f(r) for r in rows) for name, f in CATS}, len(rows)

    yday, _ = span(day, day)
    mtd, n = span(first, day)
    last, ln = span(last_first, last_end)
    proj = {k: (v / n * dim if n else 0) for k, v in mtd.items()}
    for t in (yday, mtd, last, proj):
        t["Total"] = sum(v for k, v in t.items() if k != "Total")
    recent = [r["total"] for k, r in days.items() if k < day.isoformat()][-14:]
    med = statistics.median(recent) if recent else 0
    spike = bool(med and yday["Total"] > 2 * med)
    return {"day": day.isoformat(), "yesterday": yday, "monthToDate": mtd, "projectedMonth": proj,
            "lastMonth": last, "lastMonthFull": ln == calendar.monthrange(last_first.year, last_first.month)[1],
            "median14": med, "spike": spike, "monthly": monthly(days, day)}


# ATIS replay (Readback 0.3.2, 0355edb): ATIS is recorded once per weather and
# runway and replayed, and the light-wind runway changes once a day instead of
# every 4 h. Expected to take off about three quarters of the OpenAI voice bill
# once the rigs run it; the 7-day rate shows the real figure a week after.
ATIS_REPLAY_CUT = 0.75


def monthly(days, day):
    """A month of running at the last 7 days' rate, per source - the projection the
    Stats page shows (operator 2026-10-27: "a monthly projection for cost based on
    changes"). While OpenAI voice still runs over $1 a day, a second column estimates
    the month with the ATIS replay in place."""
    keys = [k for k in days if k <= day.isoformat()][-7:]
    if not keys:
        return None
    n = len(keys)
    rate = {name: sum(f(days[k]) for k in keys) / n for name, f in CATS}
    tts = sum((days[k].get("openai") or {}).get("tts", 0) for k in keys) / n
    month = {k: v * 30 for k, v in rate.items()}
    month["Total"] = sum(month.values())
    out = {"days": n, "from": keys[0], "to": keys[-1], "perDay": rate, "month": month, "ttsPerDay": tts}
    if tts > 1.0:
        after = dict(month)
        after["OpenAI"] = month["OpenAI"] - tts * 30 * ATIS_REPLAY_CUT
        after["Total"] = sum(v for k, v in after.items() if k != "Total")
        out["withAtisReplay"] = after
    return out


# ------------------------------------------------------------------ PDF
ss = getSampleStyleSheet()
NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
GREY = colors.HexColor("#555555")
title = ParagraphStyle("t", parent=ss["Title"], fontSize=16, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.2, textColor=GREY, alignment=1, spaceAfter=4)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=10.5, textColor=NAVY, spaceBefore=6, spaceAfter=2)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.6, leading=9)
num = ParagraphStyle("n", parent=cell, alignment=2)
hdr = ParagraphStyle("hd", parent=cell, textColor=colors.white, fontName="Helvetica-Bold")
hdrn = ParagraphStyle("hdn", parent=hdr, alignment=2)
note = ParagraphStyle("no", parent=ss["Normal"], fontSize=7.4, leading=8.8, spaceBefore=1)
big = ParagraphStyle("b", parent=ss["Normal"], fontSize=15, leading=17, textColor=NAVY, alignment=1, fontName="Helvetica-Bold")
small = ParagraphStyle("sm", parent=cell, alignment=1, textColor=GREY)


def usd(v):
    return "$%.2f" % v if v < 1000 else "${:,.0f}".format(v)


def table(head, rows, widths, numeric_from=1):
    data = [[Paragraph(x, hdrn if i >= numeric_from else hdr) for i, x in enumerate(head)]]
    data += [[Paragraph(x, num if i >= numeric_from else cell) for i, x in enumerate(r)] for r in rows]
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#9aa7b8")),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("TOPPADDING", (0, 0), (-1, -1), 1.8), ("BOTTOMPADDING", (0, 0), (-1, -1), 1.8),
    ]))
    return t


def build_pdf(path, days, s, cfg, errors):
    day = dt.date.fromisoformat(s["day"])
    doc = SimpleDocTemplate(path, pagesize=letter, leftMargin=0.55 * inch, rightMargin=0.55 * inch,
                            topMargin=0.5 * inch, bottomMargin=0.5 * inch, title="Readback Costs %s" % s["day"])
    S = [Paragraph("Readback Running Costs", title),
         Paragraph("%s &middot; OpenAI + AWS exact, ElevenLabs plan fee, power estimated &middot; internal" %
                   day.strftime("%a %d %b %Y"), sub)]
    tiles = [("Yesterday", s["yesterday"]["Total"]), ("Month to date", s["monthToDate"]["Total"]),
             ("Projected %s" % day.strftime("%b"), s["projectedMonth"]["Total"]),
             ("Last month%s" % ("" if s["lastMonthFull"] else " (part)"), s["lastMonth"]["Total"])]
    tt = Table([[Paragraph(usd(v), big) for _, v in tiles], [Paragraph(n, small) for n, _ in tiles]],
               colWidths=[1.85 * inch] * 4)
    tt.setStyle(TableStyle([("BOX", (0, 0), (-1, -1), 0.6, NAVY), ("INNERGRID", (0, 0), (-1, -1), 0.3, colors.HexColor("#c5cfdc")),
                            ("BACKGROUND", (0, 0), (-1, -1), SHADE)]))
    S.append(tt)
    if s["spike"]:
        S.append(Paragraph("<font color='#c62828'><b>Spike:</b></font> yesterday %s is more than twice the 14-day median %s." %
                           (usd(s["yesterday"]["Total"]), usd(s["median14"])), note))
    for e in errors:
        S.append(Paragraph("<font color='#d98e04'><b>%s</b></font>" % e, note))

    S.append(Paragraph("By source", h))
    rows = [[n, usd(s["yesterday"][n]), usd(s["monthToDate"][n]), usd(s["projectedMonth"][n]), usd(s["lastMonth"][n])]
            for n, _ in CATS] + [["<b>Total</b>"] + ["<b>%s</b>" % usd(s[k]["Total"]) for k in
                                                      ("yesterday", "monthToDate", "projectedMonth", "lastMonth")]]
    S.append(table(["Source", "Yesterday", "Month to date", "Projected month", "Last month"], rows, [2.4, 1.15, 1.15, 1.25, 1.15]))

    m = s.get("monthly")
    if m:
        S.append(Paragraph("Monthly projection (30 days at the last %d days' rate, %s to %s)" % (m["days"], m["from"], m["to"]), h))
        after = m.get("withAtisReplay")
        head = ["Source", "Per day", "Per month"] + (["With ATIS replay (est.)"] if after else [])
        rows = [[n, usd(m["perDay"][n]), usd(m["month"][n])] + ([usd(after[n])] if after else []) for n, _ in CATS]
        rows.append(["<b>Total</b>", "<b>%s</b>" % usd(sum(m["perDay"].values())), "<b>%s</b>" % usd(m["month"]["Total"])] +
                    (["<b>%s</b>" % usd(after["Total"])] if after else []))
        S.append(table(head, rows, [2.4, 1.2, 1.3] + ([1.9] if after else [])))
        if after:
            S.append(Paragraph("The last column takes %d%% off the OpenAI voice bill (%s a day now) for the ATIS replay in "
                               "Readback 0.3.2, once the rigs are updated. A week after that the 7-day rate shows the real "
                               "figure and this column goes." % (ATIS_REPLAY_CUT * 100, usd(m["ttsPerDay"])), note))

    S.append(Paragraph("Last 14 days", h))
    keys = [k for k in days if k <= s["day"]][-14:]
    rows = []
    for k in reversed(keys):
        r = days[k]
        o = r.get("openai") or {}
        rows.append([dt.date.fromisoformat(k).strftime("%a %d %b"),
                     usd(o.get("tts", 0)) if o else "-", usd(o.get("stt", 0)) if o else "-",
                     usd(o.get("other", 0)) if o else "-",
                     usd(r["aws"]["total"]) if r.get("aws") else "-",
                     usd(r["elevenlabs"]), usd(r["power"]["total"]), "<b>%s</b>" % usd(r["total"])])
    S.append(table(["Day", "OpenAI voice", "OpenAI hearing", "OpenAI other", "AWS", "ElevenLabs", "Power", "Total"],
                   rows, [1.15, 0.95, 0.95, 0.9, 0.8, 0.85, 0.8, 0.85]))

    aws_mtd = {}
    for k, r in days.items():
        if k[:7] == s["day"][:7] and r.get("aws"):
            for svc, v in r["aws"]["services"].items():
                aws_mtd[svc] = aws_mtd.get(svc, 0) + v
    if aws_mtd:
        S.append(Paragraph("AWS this month by service", h))
        S.append(table(["Service", "Month to date"], [[k, usd(v)] for k, v in sorted(aws_mtd.items(), key=lambda x: -x[1])],
                       [4.0, 1.4]))

    p = cfg["power"]
    S.append(Paragraph("Power estimate", h))
    rows = []
    for m in cfg["machines"]:
        kwh_day = m["watts"] * min(24, m.get("hoursPerDay", 24)) / 1000.0
        mtd = sum(days[k]["power"]["machines"].get(m["name"], 0) for k in days if k[:7] == s["day"][:7] and k <= s["day"])
        rows.append([m["name"], "%d W" % m["watts"], "%g h" % m.get("hoursPerDay", 24), "%.1f kWh" % kwh_day, usd(mtd)])
    S.append(table(["Machine", "Average draw", "Hours a day", "Per day", "Month to date"], rows, [2.4, 1.1, 1.0, 1.1, 1.2]))
    S.append(Paragraph(
        "Xcel Colorado time-of-use: off-peak $%.3f, on-peak $%.3f summer / $%.3f winter per kWh (%02d:00-%02d:00 weekdays), "
        "x %.2f for fees and riders. The draw is a guess for an i9-14900KF / RTX 3060 Ti running a DCS server and the "
        "Readback roles - measure with a plug-in meter and set it in %s." %
        (p["offPeak"], p["onPeakSummer"], p["onPeakWinter"], p["onPeakStart"], p["onPeakEnd"], p["feesFactor"], CONFIG), note))
    S.append(Spacer(1, 4))
    S.append(Paragraph(
        "OpenAI and AWS days are UTC days as their billing reports them; power and ElevenLabs are local days. OpenAI voice "
        "is gpt-4o-mini-tts (ATIS and any role not on ElevenLabs); hearing is transcription. ElevenLabs is the plan's "
        "$%.2f a month spread over the month; the credits it buys are on the Stats page." % cfg["elevenLabsMonthlyUSD"], note))
    doc.build(S)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", default=(dt.date.today() - dt.timedelta(days=1)).isoformat())
    ap.add_argument("--days", type=int, default=62)
    ap.add_argument("--out", default=REPORTS)
    a = ap.parse_args()
    cfg = load_config()
    day = dt.date.fromisoformat(a.date)
    end = day + dt.timedelta(days=1)
    start = min(end - dt.timedelta(days=a.days), (day.replace(day=1) - dt.timedelta(days=1)).replace(day=1))

    errors, openai, aws = [], None, None
    try:
        openai = fetch_openai(start, end)
    except urllib.error.HTTPError as e:
        errors.append("OpenAI costs unavailable: HTTP %d (admin key?)" % e.code)
    except Exception as e:
        errors.append("OpenAI costs unavailable: %s" % type(e).__name__)
    try:
        aws = fetch_aws(start, end)
        if not any(aws.values()):
            errors.append("AWS reported $0 for every day - Cost Explorer may still be filling in.")
    except Exception as e:
        errors.append("AWS costs unavailable: %s" % type(e).__name__)

    data_dir = os.path.join(a.out, "_data", "costs")
    os.makedirs(data_dir, exist_ok=True)
    store = os.path.join(data_dir, "costs.json")
    old = {}
    if os.path.exists(store):
        with open(store, encoding="utf-8") as f:
            old = json.load(f).get("days", {})
    days = merge(old, build_days(cfg, start, end, openai, aws, errors))
    s = summarise(days, day)
    blob = {"updated": dt.datetime.now().isoformat(timespec="seconds"), "summary": s, "errors": errors,
            "machines": cfg["machines"], "days": days}
    for path in (store, LAUNCHER_COPY):
        with open(path, "w", encoding="utf-8") as f:
            json.dump(blob, f, indent=1)

    d = os.path.join(a.out, a.date[:7])
    os.makedirs(d, exist_ok=True)
    pdf = os.path.join(d, "%s Costs.pdf" % a.date)
    build_pdf(pdf, days, s, cfg, errors)
    print("wrote", pdf)
    print("yesterday %s  month to date %s  projected %s" % (usd(s["yesterday"]["Total"]), usd(s["monthToDate"]["Total"]),
                                                          usd(s["projectedMonth"]["Total"])))
    for e in errors:
        print("note:", e)


if __name__ == "__main__":
    main()
