# -*- coding: utf-8 -*-
"""SkyEye Network Architecture - network diagram + rig builder's guide (ReportLab).

Page 1 is the LAN: the three rigs (Day .220, Night .221, Foothold .222), the dev
box (.231), the router and what each talks to on the internet. Page 2 is one
role's voice loop inside a rig and the port table. Pages 3-4 are the builder's
guide: how to stand up a new rig from scratch, deploy, verify, and the usual
faults. Page 5 lists the repo's document builders.

Facts come from the start_*.bat files, check_env.bat, README_TRAINING1.md,
README_FOOTHOLD.md, CLAUDE.md, tools/dcs-hooks/vsfg7-current-mission.lua and
cmd/launcher (Update, /api/*). Change them here when the rigs change.

Run:  python -I docs/network-architecture/build_network_architecture.py   (from the repo root)
"""
import os
import shutil

from reportlab.graphics.shapes import Drawing, Line, Polygon, Rect, String
from reportlab.lib import colors
from reportlab.lib.pagesizes import landscape, letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import (KeepTogether, PageBreak, Paragraph, SimpleDocTemplate, Spacer, Table,
                                TableStyle)

HERE = os.path.dirname(os.path.abspath(__file__))
BUILD = "SkyEye 75405a7, 20 Oct 2026"
OUT_NAME = "SkyEye Network Architecture.pdf"
DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"]

NAVY = colors.HexColor("#1f3350")
SHADE = colors.HexColor("#e8edf4")
INET = colors.HexColor("#f3f0e6")
RIG = colors.HexColor("#eef3fa")
DEV = colors.HexColor("#eef7ef")
SUB = colors.white
EDGE = colors.HexColor("#9aa7b8")
VOICE = colors.HexColor("#2e8b57")   # SRS voice
API = colors.HexColor("#2f6db5")     # HTTPS to AI services
LAN = colors.HexColor("#6b7280")     # LAN HTTP :7000
GIT = colors.HexColor("#d9822b")     # git
DATA = colors.HexColor("#8e44ad")    # DCS telemetry
TXT = colors.HexColor("#1b1f24")
MUTED = colors.HexColor("#555555")

ss = getSampleStyleSheet()
title = ParagraphStyle("t", parent=ss["Title"], fontSize=17, textColor=NAVY, spaceAfter=0)
sub = ParagraphStyle("s", parent=ss["Normal"], fontSize=8.4, textColor=MUTED, alignment=1, spaceAfter=4)
h = ParagraphStyle("h", parent=ss["Heading2"], fontSize=11, textColor=NAVY, spaceBefore=5, spaceAfter=2)
cell = ParagraphStyle("c", parent=ss["Normal"], fontSize=7.6, leading=8.9)
hdr = ParagraphStyle("hd", parent=cell, textColor=colors.white, fontName="Helvetica-Bold")
note = ParagraphStyle("n", parent=ss["Normal"], fontSize=7.8, leading=9.3, spaceBefore=2)


# ---------------------------------------------------------------- drawing helpers
def box(d, x, y, w, hgt, fill, head=None, lines=(), head_color=NAVY, size=6.4, stroke=EDGE):
    d.add(Rect(x, y, w, hgt, fillColor=fill, strokeColor=stroke, strokeWidth=0.7, rx=3, ry=3))
    ty = y + hgt - 9
    if head:
        d.add(String(x + 4, ty, head, fontName="Helvetica-Bold", fontSize=size + 0.9, fillColor=head_color))
        ty -= size + 3.2
    for ln in lines:
        bold = ln.startswith("*")
        d.add(String(x + 4, ty, ln.lstrip("*"), fontName="Helvetica-Bold" if bold else "Helvetica",
                     fontSize=size, fillColor=TXT))
        ty -= size + 2.2


def arrow(d, x1, y1, x2, y2, color, both=False, width=1.1, label=None, lx=None, ly=None, anchor="start"):
    d.add(Line(x1, y1, x2, y2, strokeColor=color, strokeWidth=width))

    def head(xa, ya, xb, yb):
        import math
        a = math.atan2(yb - ya, xb - xa)
        s = 4.2
        p = [xb, yb,
             xb - s * math.cos(a - 0.45), yb - s * math.sin(a - 0.45),
             xb - s * math.cos(a + 0.45), yb - s * math.sin(a + 0.45)]
        d.add(Polygon(p, fillColor=color, strokeColor=color, strokeWidth=0.4))
    head(x1, y1, x2, y2)
    if both:
        head(x2, y2, x1, y1)
    if label:
        d.add(String(lx if lx is not None else (x1 + x2) / 2 + 3, ly if ly is not None else (y1 + y2) / 2,
                     label, fontName="Helvetica", fontSize=5.8, fillColor=color, textAnchor=anchor))


# ---------------------------------------------------------------- page 1: the network
def network():
    W, H = 728, 452
    d = Drawing(W, H)
    # internet band
    d.add(Rect(0, 372, W, 80, fillColor=INET, strokeColor=EDGE, strokeWidth=0.6, rx=4, ry=4))
    d.add(String(6, 441, "INTERNET", fontName="Helvetica-Bold", fontSize=7.5, fillColor=MUTED))
    inet = [
        (8, 140, "Pilots", ["DCS client + SRS client", "fly on the rigs' DCS servers,", "talk on their SRS servers"]),
        (156, 150, "OpenAI API", ["Whisper STT gpt-4o-mini-transcribe", "TTS gpt-4o-mini-tts (ATIS, fallback)",
                                   "gpt-4o-mini (ATIS translation)"]),
        (314, 138, "ElevenLabs API", ["TTS, Flash model (towers, Command,", "Marshal, Deckboss, Paddles, Texaco)",
                                       "credit cap 127,000 -> OpenAI"]),
        (460, 128, "GitHub", ["hiker1973-droid/atc  (main)", "dev box pushes,", "rigs pull on Update"]),
        (596, 128, "Discord", ["DCSServerBot (Day): restarts DCS,", "runs Darkstar GCI; operator", "deploys / chat"]),
    ]
    for x, w, name, ls in inet:
        box(d, x, 380, w, 52, SUB, name, ls)

    # router + LAN bus
    box(d, 270, 336, 190, 24, SUB, "Home router  192.168.1.1", ["LAN 192.168.1.0/24  ·  outbound HTTPS 443"], size=6.2)
    for x, w, c in [(78, 0, VOICE), (231, 0, API), (383, 0, API), (524, 0, GIT), (660, 0, API)]:
        tx = min(max(x, 278), 452)
        d.add(Line(x, 380, x, 368, strokeColor=c, strokeWidth=1.1))
        d.add(Line(x, 368, tx, 368, strokeColor=c, strokeWidth=1.1))
        arrow(d, tx, 368, tx, 360, c, both=False)
    d.add(Line(8, 322, W - 8, 322, strokeColor=NAVY, strokeWidth=2.2))
    d.add(Line(365, 336, 365, 322, strokeColor=NAVY, strokeWidth=2.2))
    d.add(String(W - 10, 326, "LAN switch / Hyper-V 'DCS Switch' (Public profile)", fontName="Helvetica",
                 fontSize=5.8, fillColor=NAVY, textAnchor="end"))

    # rigs
    rigs = [
        (8, "DAY  ·  192.168.1.220  ·  DCS3", "Caucasus Training V1.31 (Day)",
         ["Tacview real-time :42676", "DCS-gRPC :50051", "hook -> C:\\SkyeyeATC\\current_mission.txt",
          "DCSServerBot + SkyEye Darkstar GCI 264.0"],
         "SRS server :5008", "Caucasus set (10 roles)"),
        (190, "NIGHT  ·  192.168.1.221  ·  vSFG7-Training3", "Caucasus Night Training V1.31",
         ["Tacview real-time :42676", "DCS-gRPC :50053", "hook -> current_mission.txt", ""],
         "SRS server :5004", "Caucasus set (10 roles)"),
        (372, "FOOTHOLD  ·  192.168.1.222", "Syria Foothold v1.12",
         ["Tacview real-time :42676 (off while paused)", "DCS-gRPC :50051", "hook -> current_mission.txt",
          "pauses with no players"],
         "SRS server :5002", "Syria set (10 roles)"),
    ]
    for x, head, mission, dcs, srs, roleset in rigs:
        w = 176
        box(d, x, 36, w, 276, RIG, head, size=6.6)
        box(d, x + 5, 226, w - 10, 70, SUB, "DCS World server", [mission] + dcs)
        box(d, x + 5, 194, w - 10, 28, SUB, srs, ["ExternalAudio (EAM password SRS_EAM)"])
        box(d, x + 5, 162, w - 10, 28, SUB, "launcher.exe :7000", ["/  /fleet  /logs  /api/*  ·  Update button"])
        if x == 372:
            roles = ["ATIS (Syria)", "Command 282.0", "Marshal 306.3 :6004", "Deckboss 128.6 :6005",
                     "Paddles 128.3", "Incirlik :6041  Ramat David :6042", "Akrotiri :6046  Bassel :6049",
                     "Beirut :6050"]
        else:
            roles = ["ATIS (Caucasus, 5 stations)", "Command 282.0 + Hitman 61.0/62.0 FM",
                     "+ Texaco 252.1 / 252.3", "Marshal 306.3 :6004  Deckboss 128.6 :6005", "Paddles 128.3",
                     "Batumi :6011  Kobuleti :6012  Senaki :6013", "Kutaisi :6014  Tbilisi :6015"]
        box(d, x + 5, 42, w - 10, 116, SUB, "atc.exe roles  ·  " + roleset, roles)
        d.add(Line(x + w / 2, 322, x + w / 2, 312, strokeColor=NAVY, strokeWidth=2.2))

    # dev box
    x = 556
    box(d, x, 36, 164, 276, DEV, "DEV / HOST  ·  192.168.1.231", size=6.6)
    box(d, x + 5, 238, 154, 58, SUB, "Repo C:\\SkyeyeATC", ["Go 1.26 · build.bat -> atc.exe, launcher.exe",
                                                             "go test ./... before every push",
                                                             "docs/* PDF builders (Python)"])
    box(d, x + 5, 186, 154, 48, SUB, "launcher.exe :7000", ["/fleet + /logs read every rig's :7000", "over the LAN (read-only)",
                                                              "no roles run here"])
    box(d, x + 5, 126, 154, 56, SUB, "Nightly tasks", ["00:05 Paddles report (/api/lso-log)",
                                                       "00:10 Health report (/api/day-log)",
                                                       "-> Desktop\\...\\Documentation"])
    box(d, x + 5, 42, 154, 80, SUB, "Claude Code (operator's agent)", ["read-only launcher APIs on :7000",
                                                                         "never presses Update", "pushes only when told",
                                                                         "no remote shell on the rigs"])
    d.add(Line(x + 82, 322, x + 82, 312, strokeColor=NAVY, strokeWidth=2.2))

    # legend
    lg = [(VOICE, "SRS voice / DCS multiplayer"), (API, "HTTPS to AI services"), (GIT, "git push / pull"),
          (LAN, "LAN HTTP (launcher :7000)"), (NAVY, "LAN")]
    lx = 8
    for c, t in lg:
        d.add(Line(lx, 14, lx + 18, 14, strokeColor=c, strokeWidth=2))
        d.add(String(lx + 22, 11.5, t, fontName="Helvetica", fontSize=6.4, fillColor=TXT))
        lx += 26 + len(t) * 3.3
    d.add(String(W - 4, 11.5, "Each rig is self-contained: DCS, SRS, Tacview, gRPC and its ATC roles all on localhost.",
                 fontName="Helvetica-Oblique", fontSize=6.2, fillColor=MUTED, textAnchor="end"))
    return d


# ---------------------------------------------------------------- page 2: the voice loop
def voice_loop():
    W, H = 728, 196
    d = Drawing(W, H)
    d.add(Rect(96, 58, 538, 132, fillColor=RIG, strokeColor=EDGE, strokeWidth=0.6, rx=4, ry=4))
    d.add(String(102, 180, "atc.exe  -  one OS process per radio role (same binary, different flags)",
                 fontName="Helvetica-Bold", fontSize=7, fillColor=NAVY))
    box(d, 2, 112, 86, 44, SUB, "Pilot", ["SRS client", "presses PTT on", "the role's frequency"])
    box(d, 2, 60, 86, 44, SUB, "SRS server", ["UDP voice", "TCP sync", ":5008 / :5004 / :5002"])
    steps = [
        (104, 128, 76, "1  SRS client", ["TCP sync + EAM,", "UDP voice in"]),
        (186, 128, 70, "2  Buffer", ["frames per radio,", "flush on PTT up"]),
        (262, 128, 80, "3  Whisper STT", ["OpenAI", "gpt-4o-mini-transcribe"]),
        (348, 128, 86, "4  Intent", ["pkg/controller", "ParseIntent, callsign"]),
        (440, 128, 88, "5  State + reply", ["pkg/state, Tacview", "pkg/composer text"]),
        (534, 128, 94, "6  TTS", ["ElevenLabs Flash, or", "OpenAI gpt-4o-mini-tts"]),
        (534, 66, 94, "7  Radio effect", ["ffmpeg radio band-pass", "+ static, levelled"]),
        (420, 66, 108, "8  DCS-SR-ExternalAudio", ["injects the clip on the", "role's freq (SRS_EAM)"]),
    ]
    for x, y, w, hd, ls in steps:
        box(d, x, y, w, 40, SUB, hd, ls)
    arrow(d, 45, 112, 45, 104, VOICE)
    arrow(d, 88, 82, 104, 140, VOICE)
    for a, b in [(180, 186), (256, 262), (342, 348), (434, 440), (528, 534)]:
        arrow(d, a, 148, b, 148, NAVY)
    arrow(d, 581, 128, 581, 106, NAVY)
    arrow(d, 534, 86, 528, 86, NAVY)
    arrow(d, 420, 80, 88, 80, VOICE, label="clip out on SRS", lx=250, ly=72)
    # side inputs
    box(d, 646, 140, 80, 48, SUB, "Tacview RT", [":42676 ACMI stream", "positions, angels,", "speeds (towers etc.)"])
    box(d, 646, 86, 80, 48, SUB, "DCS-gRPC", [":50051 / :50053", "LSO grades,", "events (Paddles)"])
    box(d, 646, 30, 80, 50, SUB, "Mission weather", ["current_mission.txt", "(DCS hook) -> .miz", "wind, QNH, ceiling"])
    for y in (164, 110, 55):   # side inputs -> one feed into state (5)
        d.add(Line(646, y, 638, y, strokeColor=DATA, strokeWidth=1.1))
    d.add(Line(638, 55, 638, 175, strokeColor=DATA, strokeWidth=1.1))
    d.add(Line(638, 175, 484, 175, strokeColor=DATA, strokeWidth=1.1))
    arrow(d, 484, 175, 484, 168, DATA)
    box(d, 96, 4, 300, 46, SUB, "launcher.exe :7000 (same box)",
        ["starts / stops roles (start_*.bat flags), reads their logs/*.log,",
         "serves /api/roles /api/log /api/day-log /api/alerts /api/version,",
         "Update: git pull -> build -> swap atc.exe/launcher.exe -> restart"], size=6.2)
    box(d, 404, 4, 230, 46, SUB, "Per-role dashboard :60xx",
        ["towers 6011-6015 / 6041-6050, Marshal 6004,", "Deckboss 6005: /status, runway, weather,",
         "proxied by the launcher as /tower/<port>/"], size=6.2)
    return d


# ---------------------------------------------------------------- tables
def table(head, rows, widths, size=None):
    st = cell if size is None else ParagraphStyle("cx", parent=cell, fontSize=size, leading=size + 1.3)
    data = [[Paragraph(x, hdr) for x in head]] + [[Paragraph(x, st) for x in r] for r in rows]
    t = Table(data, colWidths=[w * inch for w in widths], repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, SHADE]),
        ("GRID", (0, 0), (-1, -1), 0.4, EDGE),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("TOPPADDING", (0, 0), (-1, -1), 1.8), ("BOTTOMPADDING", (0, 0), (-1, -1), 1.8),
    ]))
    return t


def section(heading, tbl, *notes):
    return KeepTogether([Paragraph(heading, h), tbl] + [Paragraph(n, note) for n in notes])


PORTS = [
    ["SRS server", "TCP+UDP 5008 (Day) · 5004 (Night) · 5002 (Foothold)", "pilots (internet), every atc.exe role",
     "<b>SKYEYE_SRS</b> = localhost:&lt;port&gt; on the rig"],
    ["Tacview real-time", "TCP 42676", "towers, Marshal, Deckboss, Paddles, Command, launcher",
     "<b>SKYEYE_TACVIEW</b>. DCS: Options &gt; Special &gt; Tacview &gt; real-time telemetry"],
    ["DCS-gRPC", "TCP 50051 (Day, Foothold) · 50053 (Night)", "Paddles (LSO grades, events)",
     "start_lso_*.bat picks it; <b>SKYEYE_GRPC</b> overrides"],
    ["Launcher", "TCP 7000", "LAN browsers, the dev box, the nightly reports",
     "firewall rule must allow the <b>Public</b> profile on the Hyper-V switch"],
    ["Role dashboards", "TCP 6004-6005, 6011-6015, 6041-6050 (others per theatre 6021-6054)",
     "the launcher (proxy /tower/&lt;port&gt;/), operators", "--dashboard-port in each start_*.bat"],
    ["pprof (Command)", "TCP 7770, localhost only", "debugging goroutines", "--pprof-port"],
    ["OpenAI API", "HTTPS 443 out", "every role (STT), ATIS + fallback TTS", "<b>OPENAI_API_KEY</b>"],
    ["ElevenLabs API", "HTTPS 443 out", "TTS for the ElevenLabs voices; credit reads every 10 min",
     "<b>ELEVENLABS_API_KEY</b>; --elevenlabs-stop-at 127000"],
    ["GitHub", "HTTPS 443 out", "Update (git pull --ff-only); dev box push", "repo github.com/hiker1973-droid/atc"],
]

GUIDE = [
    ["1", "Box and network",
     "Windows 10/11 or Server, static IP on 192.168.1.0/24 (gateway 192.168.1.1). Note the hostname - two boxes "
     "already answer \"DCS3\", so always identify a rig by IP."],
    ["2", "DCS World dedicated server",
     "Install, load the mission set (Caucasus Training / Night Training V1.31, or Foothold Syria). Turn on "
     "<b>Tacview real-time telemetry on 42676</b>. For Paddles: DCS-gRPC enabled (MissionScripting.lua desanitised, "
     "gRPC on :50051, or :50053 on a second server on the same box)."],
    ["3", "Mission hook",
     "Copy <b>tools/dcs-hooks/vsfg7-current-mission.lua</b> to "
     "<i>Saved Games\\DCS.dcs_serverrelease\\Scripts\\Hooks\\</i>, restart DCS once. Each load writes "
     "C:\\SkyeyeATC\\current_mission.txt and re-pins SKYEYE_MIZ; every role follows the weather live."],
    ["4", "SRS server",
     "Install DCS-SimpleRadio Standalone, pick the port (5008 / 5004 / 5002 are taken by the current rigs), turn on "
     "the External Audio Module and set its password. ExternalAudio must be at "
     "<i>C:\\Program Files\\Eagle Dynamics\\SRS\\ExternalAudio\\DCS-SR-ExternalAudio.exe</i> (or pass --external-audio)."],
    ["5", "Tools",
     "Git, <b>Go 1.26</b> (winget install GoLang.Go), ffmpeg (default C:\\ffmpeg-master-latest-win64-gpl\\bin\\ffmpeg.exe, "
     "or pass --ffmpeg - a wrong path fails quietly: clean audio, no radio effect), Python 3 only for the docs."],
    ["6", "Code",
     "<b>git clone https://github.com/hiker1973-droid/atc C:\\SkyeyeATC</b> (every bat assumes this path), then "
     "<b>build.bat</b> -> atc.exe + launcher.exe. Build from a real clone, not a git worktree, or /api/version says "
     "\"unknown\"."],
    ["7", "Environment (setx, then a new window)",
     "<b>OPENAI_API_KEY</b>, <b>ELEVENLABS_API_KEY</b>, <b>SRS_EAM</b> (EAM password), <b>SKYEYE_SRS</b> = localhost:&lt;srs "
     "port&gt;, <b>SKYEYE_TACVIEW</b> = localhost:42676, optional SKYEYE_GRPC. SKYEYE_MIZ is set by the hook. Run "
     "<b>check_env.bat</b>: all set, SRS and Tacview REACHABLE."],
    ["8", "Firewall and AV",
     "Inbound TCP 7000 (and the dashboard ports if wanted) on the LAN, for <b>Public</b> as well as Private. Add an "
     "antivirus exception for C:\\SkyeyeATC - Bitdefender has quarantined launcher.exe before and then blocked "
     "writes to that path."],
    ["9", "Start",
     "<b>start_launcher.bat</b>, open http://&lt;ip&gt;:7000/, start the theatre (Start region), or run "
     "start_region_&lt;theatre&gt;.bat. Each role opens its own console window. Never start regions on the dev box "
     "(no SRS_EAM, bats pause)."],
    ["10", "Join the fleet",
     "Add the rig to the host launcher's <b>--fleet</b> list (name@ip:7000) so /fleet, /logs and the nightly reports "
     "see it. Add it to rigcheck / run_health / run_nightly host lists."],
]

VERIFY = [
    ["/api/version", "build commit (e.g. 7047a8a) - the BUILD, not what was pulled; modified=true is normal after Update"],
    ["/api/roles", "every role of the region status=running (Caucasus: 10)"],
    ["/api/log?name=&lt;role&gt;", "\"ATC online\" / \"Command channel starting\"; tankers: \"Tanker net starting\"; "
                                   "\"Weather loaded from .miz\" naming the loaded mission"],
    ["/api/miz-weather", "mizName matches the mission DCS is running"],
    ["/api/alerts", "null, or only known items (Foothold paused: Tacview unreachable)"],
    ["On the radio", "radio check on a tower and on Command: loud and clear in the expected voice"],
]

FAULTS = [
    ["TACVIEW OFFLINE on every role", "Real-time telemetry off in DCS, or the mission is paused (Foothold with no players - normal)."],
    ["SRS TCP failed for ~1 min, then fine", "Mission rotation restarting DCS/SRS. Roles re-register on their own."],
    ["Dashboard :7000 dead, ATC still talking", "launcher.exe died; start_launcher.bat (roles keep running). "
              "install_launcher_watchdog.bat (once per rig) restarts it automatically within ~60 s."],
    ["Radio effect missing (clean audio)", "ffmpeg path wrong - pass --ffmpeg."],
    ["Voices switched to OpenAI", "ElevenLabs credit cap (127,000) reached; resets on the 14th, 16:31Z."],
    ["Roles on the wrong weather", "Hook not installed / current_mission.txt stale; check /api/miz-weather."],
    ["ATIS ExternalAudio crashes, 45 s TTS timeouts", "Known, left alone (ruling 2026-10-19); ATIS resends."],
]

UPDATE = [
    ["1", "Developer: tests pass, commit, <b>git push</b> to main (dev box)."],
    ["2", "Operator: /fleet <b>Update</b> (per rig) or Update all rigs."],
    ["3", "Launcher: git pull --ff-only, build atc.new.exe + launcher.new.exe, re-read keys from the registry."],
    ["4", "Stop running regions, swap atc.exe (old kept as atc.prev.exe), start the regions again (~30 s off air)."],
    ["5", "Swap launcher.exe, restart it; result in update_last.json. Verify with the checklist below."],
]

BUILDERS = [
    ["Comms Packet (16:9 PDF, 26 pages)", "docs/comms-packet/_tools/build_skyeye_packet.py",
     "python \"_tools\\build_skyeye_packet.py\" (from docs/comms-packet)", "VER / DATE / build stamp; NOTES.md entry"],
    ["Phraseology Reference", "docs/phraseology/build_phraseology.py", "python -I docs/phraseology/build_phraseology.py",
     "facts.json / phrases.json; drift tests TestDocFacts / TestDocPhrases"],
    ["Texaco Tanker Comms Flow", "docs/tanker-flow/build_tanker_flow.py", "python -I docs/tanker-flow/build_tanker_flow.py",
     "one page; copies to Documentation"],
    ["Paddles LSO Comms Flow", "docs/paddles-flow/build_paddles_flow.py", "python -I docs/paddles-flow/build_paddles_flow.py", ""],
    ["Hitman JTAC Comms Flow", "docs/hitman-flow/build_hitman_flow.py", "python -I docs/hitman-flow/build_hitman_flow.py", ""],
    ["Paddles daily report", "docs/paddles-report/run_nightly.py -> build_paddles_report.py",
     "task \"SkyEye Paddles Report\" 00:05", "reads /api/lso-log from each rig"],
    ["Health daily report", "docs/health-report/run_health.py -> build_health_report.py",
     "task \"SkyEye Health Report\" 00:10", "reads /api/day-log, /api/version, ElevenLabs usage"],
    ["This document", "docs/network-architecture/build_network_architecture.py",
     "python -I docs/network-architecture/build_network_architecture.py", "edit the tables when a rig changes"],
]


def footer(c, doc):
    c.saveState()
    c.setFont("Helvetica", 6.8)
    c.setFillColor(MUTED)
    c.drawString(0.45 * inch, 0.25 * inch, "vSFG-7 SkyEye ATC  ·  Network Architecture  ·  %s" % BUILD)
    c.drawRightString(landscape(letter)[0] - 0.45 * inch, 0.25 * inch, "page %d" % doc.page)
    c.restoreState()


S = [
    Paragraph("SkyEye ATC Network Architecture", title),
    Paragraph("vSFG-7 · three DCS rigs and the dev box on one home LAN · %s" % BUILD, sub),
    network(),
    PageBreak(),
    Paragraph("Inside a rig: one role's voice loop", h),
    Paragraph("Every radio role (each tower, ATIS, Command with Hitman and Texaco, Marshal, Deckboss, Paddles) is its "
              "own <b>atc.exe</b> process, started by a start_*.bat with that role's flags. All of them talk to the rig's "
              "own SRS, Tacview and DCS-gRPC on localhost, and to OpenAI / ElevenLabs over HTTPS.", note),
    voice_loop(),
    section("Ports and services", table(["Service", "Port", "Used by", "Set by"], PORTS, [1.25, 2.55, 2.75, 3.45], size=7.2)),
    PageBreak(),
    Paragraph("Builder's guide: stand up a new rig", title),
    Paragraph("From a bare Windows box to a rig on the fleet. Steps 1-4 are DCS and SRS, 5-10 are SkyEye.", sub),
    section("Build", table(["#", "Step", "What to do"], GUIDE, [0.3, 1.55, 8.15])),
    section("Deploying a change (the Update button)", table(["#", "Step"], UPDATE, [0.3, 9.7])),
    section("Verify a rig (read-only, from any LAN browser: http://&lt;ip&gt;:7000)",
            table(["Check", "Expect"], VERIFY, [1.6, 8.4])),
    section("Known faults", table(["Symptom", "Cause / fix"], FAULTS, [2.6, 7.4]),
            "No remote shell reaches the rigs (WinRM / SSH off): pulls, builds and restarts happen through the "
            "Update button or at the box. The dev box only reads the launchers."),
    section("Document builders in the repo", table(["Document", "Builder", "Run", "Notes"], BUILDERS,
                                                   [1.9, 3.0, 3.1, 2.0], size=7.0)),
]

out = os.path.join(HERE, OUT_NAME)
doc = SimpleDocTemplate(out, pagesize=landscape(letter), leftMargin=0.45 * inch, rightMargin=0.45 * inch,
                        topMargin=0.35 * inch, bottomMargin=0.4 * inch,
                        title="SkyEye ATC Network Architecture", author="vSFG7")
doc.build(S, onFirstPage=footer, onLaterPages=footer)
for d in DESKTOP:
    if os.path.isdir(d):
        shutil.copy2(out, os.path.join(d, OUT_NAME))
print("ok", out)
