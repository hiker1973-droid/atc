# -*- coding: utf-8 -*-
"""vSFG-7 AI ATC PHRASEOLOGY REFERENCE - builder.

Replaces the April 2026 one-off (Persian Gulf only, ReportLab, no source kept) and keeps its
format, on the operator's ask (2026-10-14, "make the document like the old one format
wise"): US Letter, white pages under a navy header strip, numbered sections, three-column
tables (Intent / Say something like / Response) with zebra rows, dark example-exchange
boxes in a monospace face, gold callout boxes, a navy cover with the group patch. Drawn
straight with PyMuPDF; Arial and Consolas from C:\\Windows\\Fonts.

What keeps it true:
  facts.json    frequencies, read out of the start_*.bat files and SkyEye's tables by
                cmd/atc/docs_test.go (TestDocFacts fails when they drift).
  phrases.json  every pilot call printed in a table here, written by this builder;
                TestDocPhrases runs each through the real SkyEye parser and fails if one no
                longer triggers what this document says.
A live reply may use another of SkyEye's two to four wordings with the same facts.

Update:  see ../README.md.
Run:     python -I docs/phraseology/build_phraseology.py      (from the repo root)
"""
import json
import os
import shutil
import subprocess

import pymupdf

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
VER = "v3.0"
DATE = "24 OCT 2026"          # change together with VER
OUT_NAME = "vSFG7_ATC_Phraseology_%s.pdf" % VER
DESKTOP_COPY = r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"
# vSFG-7 patch (operator 2026-10-22: "use this logo and update anything we have future").
LOGO = os.path.join(os.path.dirname(HERE), "vsfg7_patch.png")
FONTS = {"reg": r"C:\Windows\Fonts\arial.ttf", "bold": r"C:\Windows\Fonts\arialbd.ttf",
         "mono": r"C:\Windows\Fonts\consola.ttf"}

FACTS = json.load(open(os.path.join(HERE, "facts.json"), encoding="utf-8"))
TH = {t["key"]: t for t in FACTS}


def git_head():
    try:
        return subprocess.run(["git", "-C", REPO, "rev-parse", "--short", "HEAD"],
                              capture_output=True, text=True).stdout.strip() or "?"
    except OSError:
        return "?"


HEAD = git_head()
CS = "Raider 1-1"              # the example callsign everywhere
PHRASES = []                   # -> phrases.json


def f3(x):
    return "%.3f" % x


# =========================================================================== content
# Table rows: (intent, what the pilot says - alternatives split by "  /  ", the response,
# the intent the test checks). Each alternative is said as "<address>, Raider 1-1, <alt>."
TOWER_GROUND = [
    ("Comm / Radio check", "radio check  /  comm check  /  how copy  /  green jet", "Loud and clear", "radio_check"),
    ("Request startup", "request startup", "Startup approved", "startup"),
    ("Request taxi", "request taxi  /  ready to taxi", "Taxi to RWY XX, hold short", "taxi"),
    ("Holding short", "holding short  /  at the hold  /  holding", "Line up and wait, then cleared for takeoff", "holding_short"),
    ("Takeoff", "ready for takeoff  /  request takeoff", "Cleared for takeoff", "takeoff"),
    ("Airborne", "airborne  /  departing", "Climb angels N, expect handoff at 7 DME", "airborne"),
    ("Rolling (CTAF)", "rolling runway NN", "Same as airborne", "rolling"),
    ("7 DME / handoff (optional)", "seven DME  /  seven miles  /  request handoff",
     "Copy if already handed off, else contact Command", "dme"),
    ("Pushing Command", "pushing command", "Good day", "pushing_command"),
]
TOWER_ARRIVAL = [
    ("Inbound (initial call)", "N mile initial  /  N miles inbound", "Runway, wind, altimeter, report initial", "inbound"),
    ("Overhead", "3 mile initial  /  overhead", "Break approved, number N, report base", "overhead"),
    ("Break", "break", "Report downwind", "break"),
    ("Downwind", "downwind", "Number N, report base", "downwind"),
    ("Base", "base, gear down", "Cleared to land / affirmative", "base"),
    ("On final", "on final  /  5 mile final", "Cleared to land, RWY XX", "final"),
    ("Touch and go", "on final, touch and go", "Cleared touch and go", "final"),
    ("Straight-in", "request straight in  /  ILS approach", "Straight-in approved, report final", "straight_in"),
    ("Going around", "going around", "Climb runway heading, report downwind", "go_around"),
    ("Runway vacated", "runway vacated  /  clear of the active", "Taxi to parking", "vacated"),
]
# Multi-ship flights (pkg/controller/flight.go, 2026-10-21): one takeoff and one landing
# clearance cover the flight. The lead is CS (Raider 1-1); the wingmen follow him.
TOWER_FLIGHT = [
    ("Flight holding short", "flight of two, Raider 1-2, holding short  /  flight of three, 1-2 and 1-3, holding short",
     "Raider 1-1 flight, flight of two, line up and wait", "holding_short"),
    ("Flight takeoff", "flight of two, ready for takeoff  /  two-ship, ready for takeoff",
     "One takeoff clearance for the flight", "takeoff"),
    ("Flight inbound", "flight of two, N miles inbound full stop  /  two-ship, N mile initial",
     "Raider 1-1 flight, runway, wind, report initial", "inbound"),
    ("Flight full stop", "flight of two, full stop", "Cleared to land - the whole flight", "final"),
    ("Flight vacated", "flight of two, clear of the active", "A parking spot for every jet", "vacated"),
]
TOWER_ANYTIME = [
    ("Say again", "say again", "Repeats your last call (2 min)", "say_again"),
    ("Wind / altimeter", "wind check  /  say altimeter", "Wind and altimeter", "wind_check"),
    ("Radar check", "radar check", "Angels, range, bearing", "radar_check"),
    ("Hung ordnance", "hung ordnance", "Straight in, dearm at the end", "hung"),
    ("Emergency", "mayday mayday mayday  /  pan pan  /  declaring emergency, request runway NN",
     "The runway is yours - the one you name, if you name one", "emergency"),
    ("Souls and fuel (after an emergency)", "one soul on board, fuel state 1.5",
     "Copy, crash crew standing by, cleared to land", "souls_fuel"),
]
COMMAND = [
    ("Radio check", "radio check  /  how copy", "Loud and clear", "radio_check"),
    ("Check in", "checking in", "Loud and clear, proceed mission", "check_in"),
    ("Fence in", "fence in, state 6.2", "Copy fence in, cleared hot; Black Sea and Syria add where the GCI is", "fence_in"),
    ("On station", "on station", "Copy on station, good hunting", "on_station"),
    ("Off station", "off station", "Proceed to assigned pattern", "off_station"),
    ("Fence out", "fence out", "Copy fence out, safe passage", "fence_out"),
    ("RTB", "RTB  /  request recovery", "Expect handoff at 30 nm", "rtb"),
    ("State", "state 5.2", "Copy state; under 2.0 bingo", "state"),
    ("Bullseye", "say bullseye", "Bullseye NNN for NN", "bullseye"),
    ("Tanker", "request tanker", "Nearest tanker: bearing, range, angels; a Texaco adds contact on its frequency", "tanker"),
    ("Tasking", "request tasking", "Black Sea: contact Hitman 1-1 (61.0) or 2-1 (62.0)", "tasking"),
    ("In hot (no JTAC)", "in hot", "Cleared hot", "in_hot"),
    ("Release", "pickle  /  rifle", "Copy pickle / copy rifle", "release"),
    ("Shot / kill", "Fox 3  /  splash one MiG-29", "Copy Fox three / copy splash, good kill", "shot"),
    ("Moving", "inbound artillery battery  /  proceeding to IP Granite", "Copy", "attack_move"),
]
# Black Sea training range areas A-O (command_areas.go), and closing out a run
# (commandBrevityResponse). Section 5.
RANGE = [
    ("Going to an area", "proceeding to Alpha  /  pushing Alpha  /  inbound Alpha", "What is there, threat, bearing; report off target", "area"),
    ("Asking for an area", "request Alpha  /  permission to proceed to Alpha  /  request to work Alpha",
     "Stand by for Overlord Actual", "overlord"),
    ("Off target", "off target  /  off Alpha  /  off and egressing", "Copy off; say intentions", "off_target"),
    ("Out of weapons", "Winchester, RTB", "Copy Winchester, RTB, handoff at 30 nm", "winchester"),
    ("Fuel to get home", "Bingo, RTB", "Copy Bingo, RTB, nearest tanker", "bingo"),
    ("Fuel low, still working", "Joker", "Copy Joker", "joker"),
]
JTAC = [
    ("Check in", "checking in  /  request tasking", "target N, <name>, IP <IP>, read back", "check_in"),
    ("Readback", "target N, <IP>  /  copy", "Good readback, report IP inbound", "copy"),
    ("IP inbound", "IP inbound", "Continue (or where the IP is)", "ip_inbound"),
    ("In", "in from the south  /  in heading 330", "Cleared hot  -  or abort, abort, abort", "in"),
    ("Off", "off  /  off safe", "Egress per line 9", "off"),
    ("BDA", "request BDA", "Effects, then re-attack, correction or home", "bda"),
    ("Abort", "abort", "Copy abort, re-attack from the IP", "abort"),
    ("Radio check", "radio check", "Loud and clear", "radio_check"),
]
DECKBOSS = [
    ("Check in for launch", "request taxi  /  green jet", "Cleared to cat N, or the conga line", "taxi"),
    ("Ready on cat", "ready cat N  /  shoot", "Under tension, spin it up", "tension"),
    ("Airborne", "airborne", "Push Marshal 306.3", "airborne"),
    ("BRC", "say BRC", "Mother's BRC, magnetic", "brc"),
    ("Pushing Marshal", "pushing Marshal", "Short ack", "push"),
    ("Radio check", "radio check", "Loud and clear", "radio_check"),
]
MARSHAL = [
    ("Check in", "checking in", "Range and bearing, say marking moms", "check_in"),
    ("Marking moms", "marking moms, angels 12, state 6.5", "Weather, case, BRC, Marshal at angels N", "marking_moms"),
    ("BRC", "say BRC", "Mother's BRC, magnetic", "brc"),
    ("DME report", "20 DME", "Radar contact, continue", "dme"),
    ("Inbound 10 nm", "see you at ten", "Visual at ten, say state", "see_you_at_ten"),
    ("Fuel state", "state 5.2", "Copy state; under 2.0 expedite", "state"),
    ("Established", "established angels 2", "Signal Charlie, or hold", "established"),
    ("Commencing", "commencing", "Check in three mile initial", "commencing"),
    ("Platform (Case III)", "platform", "Final bearing NNN; at three miles Marshal hands you to Paddles", "platform"),
    ("Initial", "initial", "Contact paddles, good luck", "initial"),
    ("Clear (departure)", "clear seven miles", "Push Command 282.0", "clear"),
    ("Radar check", "radar check", "Angels, range, bearing from mother", "radar_check"),
    ("Radio check", "radio check", "Loud and clear", "radio_check"),
]


# Paddles (cmd/atc/lso.go heard, pkg/lso): test kinds in docs_test.go lsoDocKind.
PADDLES = [
    ("Radio check", "radio check", "Loud and clear", "radio_check"),
    ("In the break", "in the break", "Roger, call abeam", "break"),
    ("Abeam (the 180)", "abeam", "Paddles contact, Hornet in the groove, at three-quarters of a mile call the ball", "abeam"),
    ("Ball call", "Hornet ball, 5.5  /  three-quarter mile, 5.4", "Roger ball", "ball"),
    ("No ball in sight", "Clara, 5.0", "Paddles contact, keep it coming", "clara"),
    ("After the trap", "red deck", "No reply - the deck is foul, the next jet is waved off at 1/4 nm", "red_deck"),
    ("Clear of the landing area", "green deck", "Roger green deck, contact Deckboss 128.6", "green_deck"),
]
# Texaco (cmd/atc/tanker.go Response). The test runs these in order through one
# tanker net; the last field is words the reply must carry.
TEXACO = [
    ("Radio check", "comm check  /  radio check", "Loud and clear", "loud and clear"),
    ("Initial", "15 mile initial for port observation, intent to refuel",
     "Cleared port observation, angels, velocity. How much do you request?", "cleared port observation"),
    ("Give", "request 5.0  /  request full", "You are cleared for five thousand pounds / full give", "cleared for"),
    ("1 mile trail", "1 mile trail for port observation", "Continue port observation", "continue port observation"),
    ("Observation", "established port observation  /  establish portside", "Copy. Port and starboard baskets open", "baskets open"),
    ("Basket", "moving to port basket", "Cleared port basket (or standby, basket occupied)", "cleared port basket"),
    ("At the basket", "established port basket", "Copy, call ready pre-contact", "call ready pre-contact"),
    ("Ready", "ready pre-contact", "Cleared contact, port basket", "cleared contact"),
    ("Reform", "satisfied, moving to starboard reform", "Cleared starboard reform", "cleared starboard reform"),
    ("In reform", "established starboard reform  /  establish starboard side", "Copy", "copy"),
    ("Check out", "checking out, pushing 282  /  moving on to mission",
     "Cleared to depart - Fortuna super te, Viking / Stay classy, Raider / Fara í víking", "cleared to depart"),
]
# GCI, Darkstar / Overlord (cmd/atc/gci.go: gciCheckOut, gciAck, gciAliases, then SkyEye's
# own parser, pkg/gci/parser). Test kinds in docs_test.go gciDocKind. Replies from
# pkg/gci/composer and gci.go; the bullseye / BRAA numbers are examples.
GCI = [
    ("Radio check", "radio check  /  comm check", "Good copy  /  5 by 5 - off scope adds \"but I don't see you on scope\"", "radio_check"),
    ("Check in", "checking in", "Radar contact", "check_in"),
    ("Fence in", "fence in", "Copy fence in, picture follows - the picture a few seconds later", "fence_in"),
    ("Alpha check", "alpha check", "Contact, alpha check bullseye 010/48", "alpha"),
    ("Picture", "picture  /  request picture", "Up to 3 groups: bullseye, altitude, track, ID, contacts, type", "picture"),
    ("Bogey dope (nearest)", "bogey dope", "Group BRAA 045/32, 25000, hot, hostile, Flanker", "bogey"),
    ("Jets only", "any fast movers  /  bogey dope airplanes", "Nearest fixed-wing group, as bogey dope", "bogey_fast"),
    ("Helicopters only", "any slow movers  /  bogey dope helicopters", "Nearest helicopter group, as bogey dope", "bogey_slow"),
    ("Declare (bullseye)", "declare bullseye 010 for 48 at 25000", "Hostile, friendly, furball or clean", "declare"),
    ("Snaplock (BRAA)", "snaplock 125 for 10 at 8000", "Threat BRAA 125/10, 8000, hot, hostile, Flanker", "snaplock"),
    ("Spiked / strobe", "spiked 080  /  strobe 080", "Spike range 35, 16000, flank northeast, hostile - or clean 080", "spiked"),
    ("Vector", "vector to tanker", "Bearing and range to the nearest tanker", "vector"),
    ("Splash", "splash one  /  splash", "Copy splash one  /  copy splash", "splash"),
    ("Fence out", "fence out", "Copy fence out", "fence_out"),
    ("Check out", "checking out", "Copy, good day", "check_out"),
]


def table_rows(role, rows, address, tower="", cs=CS):
    """Records every alternative for the test; returns printable rows."""
    out = []
    for intent, say, reply, want in rows:
        for alt in say.split("  /  "):
            spoken = alt.replace("N mile", "10 mile").replace("NN", "13").replace("cat N", "cat 2")
            # Hitman readback: a real card target (command_tasking.go target 4, IP Indigo;
            # training V1.25 north range - docs_test.go docReadbackTarget).
            spoken = spoken.replace("target N", "target 4").replace("<IP>", "Indigo")
            PHRASES.append({"role": role, "say": "%s, %s, %s." % (address, cs, spoken),
                            "tower": tower, "want": want})
        quoted = " ".join('"...%s"' % a for a in say.split("  /  "))
        out.append((intent, quoted, reply))
    return out


# =========================================================================== layout
W, H = 612.0, 792.0
M = 40.0                                   # side margin
TOP, BOTTOM = 62.0, H - 46.0
NAVY = (0.075, 0.125, 0.215)
NAVY2 = (0.10, 0.16, 0.27)
GOLD = (0.88, 0.66, 0.18)
INK = (0.10, 0.12, 0.16)
MUTE = (0.45, 0.48, 0.53)
ZEBRA = (0.91, 0.93, 0.96)
CALLOUT = (0.98, 0.93, 0.80)
WHITE = (1, 1, 1)
FONT = {k: pymupdf.Font(fontfile=v) for k, v in FONTS.items()}


def wrap(text, font, size, width):
    words, lines, cur = text.split(" "), [], ""
    for w in words:
        t = (cur + " " + w).strip()
        if FONT[font].text_length(t, size) <= width or not cur:
            cur = t
        else:
            lines.append(cur)
            cur = w
    if cur:
        lines.append(cur)
    return lines or [""]


class Doc:
    def __init__(self):
        self.pdf = pymupdf.open()
        self.page = None
        self.y = TOP
        self.n = 0

    # -- primitives
    def text(self, x, y, s, font="reg", size=9.5, color=INK, align="l"):
        f = FONT[font]
        if align == "c":
            x -= f.text_length(s, size) / 2
        elif align == "r":
            x -= f.text_length(s, size)
        tw = pymupdf.TextWriter(self.page.rect)
        tw.append((x, y), s, font=f, fontsize=size)
        tw.write_text(self.page, color=color)

    def rect(self, r, fill, width=0, stroke=None):
        self.page.draw_rect(pymupdf.Rect(*r), color=stroke, fill=fill, width=width)

    # -- pages
    def new_page(self, chrome=True):
        self.page = self.pdf.new_page(width=W, height=H)
        self.n += 1
        self.y = TOP
        if chrome:
            self.rect((M, 26, W - M, 44), NAVY)
            self.text(M + 8, 38.5, "vSFG-7", "bold", 9, GOLD)
            self.text(M + 48, 38.5, "AI ATC Pilot Phraseology Reference", "reg", 8.5, WHITE)
            self.text(W - M - 8, 38.5, "All theatres  ·  %s" % VER, "reg", 8.5, (0.7, 0.75, 0.82), "r")
        self.text(W / 2, H - 24, "vSFG-7 Virtual Strike Fighter Group Seven    ·    Page %d" % self.n,
                  "reg", 7.5, MUTE, "c")

    def need(self, h):
        if self.y + h > BOTTOM:
            self.new_page()

    # -- blocks
    def h1(self, s):
        self.need(70)
        if self.y > TOP + 1:
            self.y += 10
        self.y += 20
        self.text(M, self.y, s, "bold", 17, INK)
        self.y += 6
        self.page.draw_line((M, self.y), (W - M, self.y), color=GOLD, width=1.2)
        self.y += 16

    def h2(self, s):
        self.need(50)
        self.y += 6
        self.text(M, self.y + 10, s, "bold", 11.5, INK)
        self.y += 20

    def para(self, s, size=9.5, color=INK, font="reg"):
        lines = wrap(s, font, size, W - 2 * M)
        self.need(len(lines) * (size + 3.5) + 4)
        for ln in lines:
            self.y += size + 3.5
            self.text(M, self.y, ln, font, size, color)
        self.y += 6

    def table(self, headers, rows, cols):
        """cols: column widths as fractions of the text width."""
        tw = W - 2 * M
        xs, x = [], M
        for c in cols:
            xs.append(x)
            x += c * tw
        pad, size, lead = 7.0, 8.6, 11.2

        def header():
            self.rect((M, self.y, W - M, self.y + 20), NAVY)
            for i, h in enumerate(headers):
                self.text(xs[i] + pad, self.y + 13.5, h, "bold", 8.8, WHITE)
            self.y += 20

        self.need(20 + 24)
        header()
        for ri, row in enumerate(rows):
            cells = [wrap(str(v), "reg", size, cols[i] * tw - 2 * pad) for i, v in enumerate(row)]
            h = max(len(c) for c in cells) * lead + 10
            if self.y + h > BOTTOM:
                self.new_page()
                header()
            if ri % 2 == 1:
                self.rect((M, self.y, W - M, self.y + h), ZEBRA)
            for i, c in enumerate(cells):
                for li, ln in enumerate(c):
                    self.text(xs[i] + pad, self.y + 14 + li * lead, ln, "reg", size, INK)
            self.y += h
        self.y += 12

    def dialogue(self, lines):
        """lines: (speaker, text). Dark box, monospace, like the April example blocks."""
        size, lead, who_w = 8.2, 11.0, 58.0
        body = [(w, wrap('"%s"' % t, "mono", size, W - 2 * M - who_w - 24)) for w, t in lines]
        h = sum(len(b) for _, b in body) * lead + (len(body) - 1) * 4 + 18
        self.need(h + 10)
        self.rect((M, self.y, W - M, self.y + h), NAVY2)
        y = self.y + 9 + size
        for who, b in body:
            self.text(M + 10, y, who + ":", "mono", size, GOLD)
            for ln in b:
                self.text(M + 10 + who_w, y, ln, "mono", size, (0.86, 0.89, 0.93))
                y += lead
            y += 4
        self.y += h + 14

    def callout(self, title, lines):
        size, lead = 8.8, 11.6
        body = [wrap(s, "reg", size, W - 2 * M - 30) for s in lines]
        h = 26 + sum(len(b) for b in body) * lead + 8
        self.need(h + 8)
        self.rect((M, self.y, W - M, self.y + h), CALLOUT)
        self.rect((M, self.y, M + 3.5, self.y + h), GOLD)
        self.text(M + 14, self.y + 17, title, "bold", 9.3, INK)
        y = self.y + 17 + lead + 2
        for b in body:
            for ln in b:
                self.text(M + 14, y, ln, "reg", size, INK)
                y += lead
        self.y += h + 14

    def tips(self, items):
        size, lead = 9.0, 12.0
        for bold, rest in items:
            lines = wrap(bold + " " + rest, "reg", size, W - 2 * M - 16)
            self.need(len(lines) * lead + 8)
            self.y += lead
            self.page.draw_rect(pymupdf.Rect(M + 2, self.y - 6.5, M + 7, self.y - 1.5), color=GOLD, fill=GOLD)
            # bold lead-in on the first line, the rest regular
            first, x = lines[0], M + 16
            if first.startswith(bold):
                self.text(x, self.y, bold, "bold", size, INK)
                self.text(x + FONT["bold"].text_length(bold + " ", size), self.y, first[len(bold):].strip(), "reg", size, INK)
            else:
                self.text(x, self.y, first, "reg", size, INK)
            for ln in lines[1:]:
                self.y += lead
                self.text(x, self.y, ln, "reg", size, INK)
            self.y += 5
        self.y += 8

    # -- cover
    def cover(self):
        self.new_page(chrome=False)
        top, bot = 70.0, 430.0
        self.rect((M, top, W - M, bot), NAVY)
        self.page.draw_line((M + 4, top + 16), (W / 2 + 20, top + 16), color=GOLD, width=2, dashes="[2 1] 0")
        self.text(W / 2, top + 72, "vSFG-7", "bold", 40, GOLD, "c")
        self.text(W / 2, top + 88, "VIRTUAL STRIKE FIGHTER GROUP SEVEN", "bold", 8.5, (0.62, 0.68, 0.76), "c")
        if os.path.exists(LOGO):
            s = 150.0
            pix = pymupdf.Pixmap(LOGO)
            while pix.width > 600:          # the patch is print-size; 600 px is plenty at 150 pt
                pix.shrink(1)
            self.page.insert_image(pymupdf.Rect(W / 2 - s / 2, top + 100, W / 2 + s / 2, top + 100 + s),
                                   stream=pix.tobytes("png"))
        self.text(W / 2, top + 284, "AI ATC PHRASEOLOGY REFERENCE", "bold", 20, WHITE, "c")
        self.text(W / 2, top + 303, "SkyEye on SRS  ·  every theatre", "reg", 10, (0.62, 0.68, 0.76), "c")
        self.page.draw_line((W / 2 - 110, top + 316), (W / 2 + 110, top + 316), color=GOLD, width=1)
        self.text(W / 2, top + 334, "VMFA-225 Vikings  ·  421st FS Black Widows", "bold", 9.5, GOLD, "c")
        self.text(W / 2, top + 349, "F/A-18C Hornet  ·  F-16C Viper", "reg", 9, (0.62, 0.68, 0.76), "c")
        self.text(W / 2, bot + 30, "%s  ·  %s" % (VER, DATE), "bold", 11, INK, "c")
        self.text(W / 2, bot + 46, "Built from SkyeyeATC %s. Frequencies are read from the start scripts; every call in "
                  "the tables is tested against SkyEye." % HEAD, "reg", 8, MUTE, "c")


# =========================================================================== document
def build():
    d = Doc()
    d.cover()
    d.new_page()

    # 1. Frequency map
    d.h1("1. Frequency Map")
    d.para("Where SkyEye listens, read from the start scripts. All AM, except Hitman (FM). "
           "Mission preset channels differ - check the frequency, not only the channel.", 9, MUTE)
    for t in FACTS:
        d.h2(t["title"])
        rows = []
        for tw in t["towers"]:
            rows.append((tw["name"], f3(tw["freq"]), "Two-way", "Callsign: %s" % tw["name"]))
        rows.append(("Command", f3(t["command"]), "Two-way", "Callsign: vSFG-7-Command"))
        if t.get("jtac"):
            note = "9-line targets 1-3" if t.get("jtac2") else "9-line training targets"
            rows.append(("Hitman 1-1  -  JTAC", f3(t["jtac"]) + " FM", "Two-way", note))
        if t.get("jtac2"):
            rows.append(("Hitman 2-1  -  JTAC", f3(t["jtac2"]) + " FM", "Two-way", "9-line targets 4-5"))
        if t.get("deckboss"):
            rows.append(("Deckboss  -  Carrier", f3(t["deckboss"]), "Two-way", "CVN-72 launch ops"))
        if t.get("marshal"):
            rows.append(("Marshal  -  Carrier", f3(t["marshal"]), "Two-way", "Recovery stack"))
        if t.get("lso"):
            rows.append(("Paddles  -  LSO", f3(t["lso"]), "Two-way", "Groove calls, ball call, deck calls"))
        if t.get("tanker"):
            rows.append(("Texaco 1-1  -  Tanker", f3(t["tanker"]), "Two-way", "KC-130, TACAN 51Y, angels 13"))
        if t.get("tanker2"):
            rows.append(("Texaco 3-1  -  Tanker", f3(t["tanker2"]), "Two-way", "KC-135 MPRS, TACAN 53Y, angels 24"))
        if t.get("gci"):
            note = "Picture, threats, bogey dope"
            if t["key"] == "kola":
                note = "Confirm on the Kola card"
            rows.append(("%s  -  GCI" % t["gci_name"], f3(t["gci"]), "Two-way", note))
        for a in t["atis"]:
            rows.append(("ATIS  -  " + a["name"].replace(" ATIS", ""), f3(a["freq"]), "Listen only", "Updates with the weather"))
        d.table(("Channel", "Freq (MHz)", "Type", "Notes"), rows, (0.34, 0.17, 0.15, 0.34))
    d.callout("Carrier presets", [
        "Marshal 306.300 is Hornet COMM 1 CH 3 \"CVN-72 AI MARSHALL\" - that is SkyEye. "
        "306.200 \"CVN-72 MARSHAL\" is kept for a live human Marshal.",
        "Deckboss 128.600 is shared with the DCS AI carrier controller - you may hear both."])

    # 2. Addressing
    d.new_page()
    d.h1("2. Addressing the Tower")
    d.para("The tower only responds when addressed. Any of these forms work:")
    d.table(("Format", "Example"), [
        ("Field name + Tower", "\"Batumi Tower, Raider 1-1, ...\""),
        ("Field name alone", "\"Batumi, Raider 1-1, ...\""),
        ("Field name + Traffic (CTAF)", "\"Senaki Traffic, Raider 1-1, rolling runway 09\""),
        ("Bare - Tower", "\"Tower, Raider 1-1, ...\""),
    ], (0.36, 0.64))
    d.callout("Whisper mishear auto-corrections", [
        "Field names: a letter or two off still counts - helmand, el minhad -> Minhad; ldaf, delta for -> Dhafra; "
        "alan, el ain -> Al Ain. Mafraq also answers to King Hussein.",
        "Callsigns: Reader / Radar / Rater -> Raider  ·  Vino / Venue / Demon -> Venom."])

    # 3. Tower
    d.h1("3. Tower - Request Phraseology")
    d.h2("3.1 Ground & Departure")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("tower", TOWER_GROUND, "Batumi Tower", "Batumi Tower"), (0.24, 0.42, 0.34))
    d.dialogue([("Pilot", "Batumi Tower, Raider 1-1, request taxi."),
                ("Tower", "Raider 1-1, Batumi Tower, taxi to runway one three, altimeter two niner niner two, hold short."),
                ("Pilot", "Batumi Tower, Raider 1-1, holding short."),
                ("Tower", "Raider 1-1, runway one three, line up and wait."),
                ("Tower", "Raider 1-1, wind one two zero at eight, runway one three, cleared for takeoff."),
                ("Pilot", "Batumi Tower, Raider 1-1, airborne."),
                ("Tower", "Raider 1-1, Batumi Tower, proceed to angels four, expect handoff at seven DME."),
                ("Tower", "Raider 1-1, Batumi Departure, seven miles, angels four, leaving my airspace. "
                          "Contact command, two eight two point zero, channel one. Good day.")])
    d.callout("If you are held", [
        "Traffic inside 20 nm, or less than 60 s since the last departure, holds you. Stay put - the tower "
        "clears you when it is clear; you do not need to call again.",
        "\"seven miles\" on its own is the departure call. Inbound, always say \"inbound\" or \"initial\" with a distance."])
    d.callout("The 7-mile handoff", [
        "At 7 miles the tower calls you from Tacview - \"<field> Departure ... leaving my airspace, contact command\". "
        "You do not need to call 7 DME; if you do, the tower answers \"copy\". Call before 7 miles and you get the handoff then.",
        "After takeoff \"clear the active\" counts as your airborne call. Fields with an Australian controller sign off \"g'day\"."])
    d.h2("3.2 Arrival & Pattern")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("tower", TOWER_ARRIVAL, "Batumi Tower", "Batumi Tower"), (0.24, 0.42, 0.34))
    d.dialogue([("Pilot", "Batumi Tower, Raider 1-1, 10 mile initial."),
                ("Tower", "Raider 1-1, Batumi Tower, runway one three, wind one two zero at eight, report initial."),
                ("Pilot", "Batumi Tower, Raider 1-1, 3 mile initial."),
                ("Tower", "Raider 1-1, left break runway one three, number one, report base."),
                ("Pilot", "Batumi Tower, Raider 1-1, base, gear down."),
                ("Tower", "Raider 1-1, runway one three, cleared to land."),
                ("Pilot", "Batumi Tower, Raider 1-1, clear of the active."),
                ("Tower", "Raider 1-1, taxi to parking.")])
    d.callout("Good to know", [
        "Past 15 nm you hear \"continue inbound, report N miles\".",
        "Say \"gear down\" in your base or final call, or the tower adds \"check wheels down\".",
        "IMC (ceiling under 1,500 ft or visibility under 3 nm): no overhead - straight in only."])
    d.h2("3.3 Multi-Ship Flights")
    d.para("The lead talks for the flight. Say the size and name the wingmen on the first call; one takeoff "
           "clearance and one landing clearance cover every jet, and the tower answers \"<lead> flight\".")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("tower", TOWER_FLIGHT, "Batumi Tower", "Batumi Tower"), (0.24, 0.42, 0.34))
    d.dialogue([("Pilot", "Batumi Tower, Raider 1-1, flight of three, 1-2 and 1-3, holding short."),
                ("Tower", "Raider 1-1 flight, Batumi Tower, flight of three, runway one three, line up and wait."),
                ("Tower", "Raider 1-1 flight, Batumi Tower, wind one two zero at eight, runway one three, cleared for takeoff."),
                ("Pilot", "Batumi Tower, Raider 1-1 flight, airborne."),
                ("Pilot", "Batumi Tower, Raider 1-1, flight of two, Raider 1-2, 10 miles inbound full stop."),
                ("Tower", "Raider 1-1 flight, Batumi Tower, flight of two, runway one three, wind one two zero at "
                          "eight, report initial."),
                ("Pilot", "Batumi Tower, Raider 1-1 flight, clear of the active."),
                ("Ground", "Raider 1-1 flight, taxi to parking, Raider 1-1 spot 12, Raider 1-2 spot 13.")])
    d.callout("Flights", [
        "Size: \"flight of two\", \"two-ship\" or \"three FA-18s\". Wingmen right after the lead: \"1-2 and 1-3\", "
        "\"302 and 303\" or \"Raider 302, Raider 303\".",
        "A wingman may call too: named, or flying within 2 nm of the lead (Tacview), he is answered as the flight.",
        "The next departure waits the usual 60 s plus 20 s for each extra jet. Every jet gets a parking spot; "
        "unnamed wingmen get \"spots 12, 13 and 14\"."])
    d.h2("3.4 Any Time")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("tower", TOWER_ANYTIME, "Batumi Tower", "Batumi Tower"), (0.24, 0.42, 0.34))
    d.callout("Emergencies", [
        "\"Declaring emergency, request runway 09\": you get the runway you name, even against the active, for the "
        "rest of your approach. No runway named: the active.",
        "The tower asks for souls on board and fuel state. Answer \"one soul on board, fuel state 1.5\" (or "
        "\"fuel state zero\"): the tower copies it, crash crew standing by, cleared to land."])
    d.table(("The tower calls you when", "Call"), [
        ("Over 350 kt inside 10 nm", "Speed check"),
        ("Inside 5 nm while a jet is rolling", "Go around"),
        ("A wind shift changes the runway", "Runway in use is now NN"),
        ("You were held and the traffic has cleared", "Cleared for takeoff")], (0.6, 0.4))

    # 4. Command
    d.h1("4. Command Channel - %s MHz" % f3(TH["pg"]["command"]))
    d.para("The squadron net in every theatre. Lead with \"Command\" and your callsign.")
    d.table(("Intent", "Say something like", "Example Response"),
            table_rows("command", COMMAND, "Command"), (0.2, 0.4, 0.4))
    d.dialogue([("Pilot", "Command, Raider 1-1, fence in, state 6.2."),
                ("Cmd", "Raider 1-1, Command, copy fence in, you are cleared hot."),
                ("Pilot", "Command, Raider 1-1, request tasking."),
                ("Cmd", "Raider 1-1, Command, contact Hitman one one on six one point zero zero zero for tasking.")])
    d.callout("The automatic handoff", [
        "Once you have talked to Command it watches you on Tacview. Inside 30 nm of your recovery - a field, or the "
        "carrier if you launched from it - it calls you once with the tower or Marshal frequency.",
        "Bullseye and tanker need the radar picture; without it you hear \"unable\". Tasking is Black Sea training only.",
        "No JTAC up? After fence in, Command copies your attack calls: \"in hot\" gets cleared hot, \"pickle\" / "
        "\"rifle\" / \"Fox 3\" / \"splash\" get a copy. Pilots often use the flight number alone (\"302, Fox 3\") - "
        "Command finds the full callsign on Tacview."])

    # 5. Range areas (command_areas.go)
    d.h1("5. Range Areas and Overlord Actual - Command")
    d.para("Black Sea training. The lettered areas A - O on the kneeboard's TARGET RANGES card. Tell Command where you "
           "are going and it tells you what is there; ask, and Overlord Actual (a second voice on Command's frequency) "
           "clears you. The 9-line areas D, H and I go to Hitman.")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("command", RANGE, "Command"), (0.2, 0.44, 0.36))
    d.dialogue([("Pilot", "Command, Raider 1-1, permission to proceed to Alpha."),
                ("Cmd", "Raider 1-1, Command, stand by for Overlord Actual."),
                ("Ovrld", "Raider 1-1, Overlord Actual, you are Irene for Alpha, report copy."),
                ("Pilot", "Raider 1-1, copy Irene."),
                ("Cmd", "Raider 1-1, Command, copy Irene. Alpha, Maykop airfield, SA-6 site and parked aircraft. "
                        "Threat SA-6. Alpha bears three-four-zero for 85. Report off target."),
                ("Pilot", "Command, Raider 1-1, off Alpha, Winchester, RTB."),
                ("Cmd", "Raider 1-1, Command, copy off Alpha, Winchester, RTB. Good work. "
                        "Expect recovery handoff at three zero miles.")])
    d.callout("How the areas work", [
        "Overlord comes up about 10 s after Command's stand by. He says \"you are Irene\" or \"you are cleared to "
        "engage\" - both mean go. \"Negative\" means another flight is working it: ask for another area.",
        "Command gives the threat from what is actually there: E is an SA-10 site, G an SA-2 site, M the cruiser "
        "Moskva's group, K the Kuznetsov with SA-15 corvettes. The kneeboard card under-reports E, G and M.",
        "Another flight on the same area is called out (\"be advised, Venom 2-1 working Alpha\"). Call off target "
        "when done so the area frees up.",
        "Bingo: Command adds the nearest tanker when the rig has Tacview. The bearing needs Tacview too."])

    # 6. Hitman
    d.h1("6. Hitman 1-1 / 2-1 - %s and %s FM (JTAC)" % (f3(TH["caucasus"]["jtac"]), f3(TH["caucasus"]["jtac2"])))
    d.para("Black Sea training 9-lines, 100-127 nm north of Senaki past the main ridge. Hitman 1-1 (61.0, Hornet "
           "COMM 2 CH 12, Viper COMM 2 CH 8) runs Kuban Valley, targets 1-3; Hitman 2-1 (62.0, dial it - no preset "
           "yet) runs Cherkessk, targets 4-5. Command sends you to whichever has a free target. Lead with \"Hitman\".")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("jtac", JTAC, "Hitman"), (0.18, 0.4, 0.42))
    d.dialogue([("Pilot", "Hitman 1-1, Raider 1-1, checking in."),
                ("Hitman", "Raider 1-1, Hitman one one, target 4, Fuel Depot, IP Indigo. Read back."),
                ("Pilot", "Hitman, Raider 1-1, target 4, Indigo."),
                ("Hitman", "Raider 1-1, Hitman one one, good readback. Report IP inbound."),
                ("Pilot", "Hitman, Raider 1-1, IP inbound."),
                ("Hitman", "Raider 1-1, Hitman one one, continue."),
                ("Pilot", "Hitman, Raider 1-1, in from the east."),
                ("Hitman", "Raider 1-1, Hitman one one, cleared hot."),
                ("Hitman", "Raider 1-1, Hitman one one, BDA. Impact on the aim point. Partial effects. Large tanks, "
                           "2 of 4 destroyed. 3 of 14 on target destroyed. Re-attack approved, report IP inbound from Indigo.")])
    d.callout("What Hitman checks (Tacview, against your 9-line card)", [
        "IP inbound: within 20 nm of the IP, nose on the target. Off the run-in line you hear \"continue, run in heading NNN\".",
        "In call: inside the card's final attack heading and 0.5 - 15 nm from the target - otherwise abort, abort, abort.",
        "After cleared hot: a run that swings 10 deg outside the window before release is aborted.",
        "BDA about 15 s after impact, one of four: good effects (go home or ask for more), partial effects (re-attack), "
        "no effects (a correction, e.g. \"correct south 300\", then re-attack), or unable to assess - the bridges - "
        "when Hitman asks \"say your observed effects\": answer \"bridge dropped\" or \"no effects\".",
        "No radar picture of you: no checks, and you are still cleared hot. JTAC LIVE 60.000 is a human JTAC."])

    # 7. Deckboss
    d.h1("7. Deckboss - %s MHz (Carrier Launch)" % f3(TH["pg"]["deckboss"]))
    d.para("Manages cats 1-4 on CVN-72. If all cats are taken you join the conga line (six). Lead with \"Deckboss\"; "
           "the airborne call needs no address.")
    d.table(("Intent", "Say something like", "What Happens"),
            table_rows("deckboss", DECKBOSS, "Deckboss"), (0.22, 0.4, 0.38))
    d.dialogue([("Pilot", "Deckboss, Raider 1-1, request taxi."),
                ("Deck", "Raider 1-1, Deckboss, cleared to cat two."),
                ("Pilot", "Deckboss, Raider 1-1, ready cat two."),
                ("Deck", "Raider 1-1, Deckboss, under tension cat two, spin it up, shooter's discretion."),
                ("Pilot", "Raider 1-1, airborne."),
                ("Deck", "Raider 1-1, Deckboss, copy airborne, push Marshal, three zero six point three.")])

    # 8. Marshal
    d.h1("8. Marshal - %s MHz (Carrier Recovery)" % f3(TH["pg"]["marshal"]))
    d.para("Manages the recovery stack. Marshal ignores a call that does not start with \"Marshal\" or has no number "
           "in your callsign. The case (I, II or III) is set from time and weather.")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("marshal", MARSHAL, "Marshal"), (0.2, 0.4, 0.4))
    d.dialogue([("Pilot", "Marshal, Raider 1-1, marking moms 340 for 30, angels 12, state 6.5."),
                ("Marshal", "Raider 1-1, Union Marshal, radar contact, angels 12, range 30 miles, bearing three four zero "
                            "from mother, expect Case One recovery, BRC 275, altimeter two niner niner two, Marshal at angels two, "
                            "report see me at ten."),
                ("Pilot", "Marshal, Raider 1-1, established angels two."),
                ("Marshal", "Raider 1-1, Union Marshal, signal Charlie.")])
    d.callout("Case III (night, or ceiling under 1,500 ft / visibility under 3 nm)", [
        "Marking moms gets a holding fix and a push time: marshal on the NNN, DD DME, angels N, expected approach time is MM. "
        "The stack starts at angels 6; the DME is angels + 15 (angels 6 at 21 DME).",
        "Commencing: descend to platform 5,000, final bearing NNN. Call \"platform\" passing 5,000 ft. "
        "At three miles Marshal says \"contact paddles, call the ball\": switch and make the ball call at three-quarters.",
        "Needles, coupled and command control reports get a \"roger\"; negative needles gets \"fly bullseye\". "
        "Calls after commencing may start with your modex alone.",
        "Marshal radial = final bearing + 180. Final bearing = BRC - 9. All magnetic."])

    # 9. Paddles
    cau = TH["caucasus"]
    d.h1("9. Paddles - %s MHz (LSO)" % f3(cau["lso"]))
    d.para("SkyEye's LSO on CVN-72 Abe (Black Sea Day / Night and Foothold Syria). Hornet COMM 2 CH 4. Marshal sends "
           "you here at the initial (\"contact paddles\"). Paddles talks you down the glideslope from Tacview: you fly, "
           "he calls. Give your modex or full callsign; the examples use Raider 302.")
    d.table(("Intent", "Say something like", "Response"),
            table_rows("lso", PADDLES, "Paddles", cs="Raider 302"), (0.22, 0.38, 0.40))
    d.dialogue([("Pilot", "Paddles, Raider 302, abeam."),
                ("Paddles", "Paddles contact, Raider 302, Hornet in the groove, at three-quarters of a mile call the ball."),
                ("Pilot", "302, Hornet ball, 5.5."),
                ("Paddles", "Roger ball."),
                ("Paddles", "You're high, bring it down."),
                ("Pilot", "302, red deck."),
                ("Pilot", "302, green deck."),
                ("Paddles", "302, Paddles, roger green deck, contact Deckboss, one two eight point six.")])
    d.callout("In the groove (no readback)", [
        "Calls start about 0.8 nm (1.3 nm after your ball call): a little high, you're high, you're low - power, "
        "POWER, right for lineup, come left, you're fast / slow (Hornet AoA). After your ball call, a call every second "
        "while you are off.",
        "No ball call by about 0.6 nm: \"call the ball\". Wave off, wave off, wave off: mandatory - power, climb straight ahead.",
        "Foul deck at 1/4 nm (no green deck from the jet ahead, or Tacview shows the landing area foul): "
        "\"302, foul deck, wave off, wave off, proceed to bolter pattern\". A red deck lapses after 3 minutes."])

    # 10. Texaco
    d.h1("10. Texaco - %s / %s MHz (Tankers)" % (f3(cau["tanker"]), f3(cau["tanker2"])))
    d.para("Black Sea Day / Night. Texaco 1-1: KC-130, %s AM (Hornet COMM 2 CH 8), TACAN 51Y, angels 13, about "
           "260 KIAS. Texaco 3-1: KC-135 MPRS, %s AM (COMM 2 CH 10), TACAN 53Y, angels 24, about 265 KIAS. "
           "Two baskets each; the same calls work on both (\"Texaco 1\" or \"Texaco 3\"). The calls follow the "
           "vSFG-7 AAQ Aerial Refueling Procedure: observation is always port, reform always starboard."
           % (f3(cau["tanker"]), f3(cau["tanker2"])))
    d.table(("Step", "Say something like", "Texaco"),
            table_rows("tanker", TEXACO, "Texaco 1-1"), (0.18, 0.44, 0.38))
    d.dialogue([("Pilot", "Command, Raider 1-1, request tanker."),
                ("Cmd", "Raider 1-1, Command, Texaco 1-1 bears zero four five for 32, angels 13. Contact Texaco one one "
                        "on two five two point one."),
                ("Pilot", "Texaco 1-1, Raider 1-1, 15 mile initial for port observation, intent to refuel."),
                ("Texaco", "Raider 11, Texaco one one, cleared port observation, one three thousand, velocity two six "
                           "zero. How much do you request?"),
                ("Pilot", "Texaco 1-1, Raider 1-1, request 5.0."),
                ("Texaco", "Raider 11, Texaco, you are cleared for five thousand pounds."),
                ("Pilot", "Texaco 1-1, Raider 1-1, moving on to mission."),
                ("Texaco", "Raider 11, Texaco, cleared to depart. Stay classy, Raider.")])
    d.callout("Tanker rules", [
        "Lead makes the initial, trail, observation and check-out calls; each jet its own basket and reform calls. "
        "Give your amount in the initial (\"... intent to refuel, request 6.0\") and Texaco copies it instead of asking.",
        "One jet per basket: a held basket gets \"standby, basket occupied\" - it frees when that jet calls reform, "
        "or after 10 minutes. A bare \"ready pre-contact\" counts when you are the only jet on the tanker.",
        "Altitude and velocity come live from Tacview. SkyEye is radio only: DCS still needs the AI comms menu "
        "Ready pre-contact before fuel flows."])

    # 11. GCI
    syr = TH["syria"]
    d.h1("11. GCI - %s %s / %s %s MHz" % (cau["gci_name"], f3(cau["gci"]), syr["gci_name"], f3(syr["gci"])))
    d.para("SkyEye's GCI (AWACS) controller in our voice, a British woman. Blue coalition. Black Sea Day / Night: "
           "%s on %s AM (Hornet COMM 1 CH 5). Syria (Foothold): %s on %s AM - dial it, the Foothold cards number "
           "channels per airframe. Lead with \"%s\" (or \"%s\", or \"Anyface\"), then your callsign, then the "
           "request. Your fence check goes here and the picture follows. Command still copies fence in and adds "
           "\"%s is up on two six four point zero for picture\" (Syria: %s, two six six)."
           % (cau["gci_name"], f3(cau["gci"]), syr["gci_name"], f3(syr["gci"]), cau["gci_name"], syr["gci_name"],
              cau["gci_name"], syr["gci_name"]))
    d.table(("Intent", "Say something like", "Response"),
            table_rows("gci", GCI, cau["gci_name"], cau["gci_name"], cs="Raider 302"), (0.2, 0.4, 0.4))
    d.dialogue([("Pilot", "Darkstar, Raider 302, checking in."),
                ("GCI", "Raider 3 0 2, Darkstar, radar contact."),
                ("Pilot", "Darkstar, Raider 302, fence in."),
                ("GCI", "Raider 3 0 2, Darkstar, copy fence in, picture follows."),
                ("GCI", "Darkstar, 2 groups. Group bullseye zero one zero, 48, 25000, track southwest, hostile, "
                        "2 contacts, Flanker. ..."),
                ("Pilot", "Darkstar, Raider 302, bogey dope."),
                ("GCI", "Raider 3 0 2, group BRAA zero four five, 32, 25000, hot, hostile, Flanker."),
                ("Pilot", "Darkstar, Raider 302, checking out."),
                ("GCI", "Raider 3 0 2, Darkstar, copy, good day.")])
    d.callout("Your name must carry your callsign", [
        "DCS name \"Raider 302 | Name\" or \"Venom 212 | Name\" - the GCI finds Raider / Venom and the number "
        "anywhere in it. Your SRS name must be the same as your DCS name, or threat and merged calls never reach you.",
        "Bearings: say the three digits (\"zero six five\"); ranges and altitudes as normal numbers. No \"niner\" / \"tree\"."])
    d.table(("The GCI calls you, unasked (only if you are on its frequency)", "Call"), [
        ("Every 5 minutes", "Picture"),
        ("A hostile inside 25 nm (repeats every 3 min)", "Threat"),
        ("Inside 3 nm of a hostile (fixed wing)", "Merged"),
        ("A hostile off scope for 30 s", "Faded"),
        ("The GCI comes online (mid-mission = it restarted)", "Sunrise"),
        ("No AWACS airborne / back up (\"All players, %s, off station / on station\")" % cau["gci_name"],
         "Off / on station")], (0.7, 0.3))
    d.callout("What the GCI can see", [
        "Only what the mission's own AWACS could: the E-3 out to about 217 nm, the E-2 about 200 nm, and nothing below "
        "its radar horizon - a bandit on the deck far out is invisible until it climbs or closes, and one that drops out "
        "of coverage is called faded. Friendlies are always seen.",
        "Terrain hides you too (Black Sea, Syria, Kola): a jet low in a valley behind a ridge from every AWACS drops "
        "off the picture until it climbs or an AWACS gets an angle on it.",
        "No AWACS airborne (not up yet, or shot down): off station. Picture, bogey dope and declare get \"off station, "
        "no picture\"; radio check, fence, splash and check in / out still work."])
    d.callout("The flow", [
        "Flow: Command check-in -> %s check-in, fence in -> fly (picture, bogey dope, declare) -> fence out, checking "
        "out -> Command RTB -> Marshal or tower. Overlord Actual on Command (section 5) is the range officer, not the "
        "GCI." % cau["gci_name"]])

    # 12. Tips
    d.h1("12. General Tips")
    d.tips([
        ("Always say your callsign.", "Address first, then your full callsign with its number. Raider and Venom are "
         "auto-corrected from common mishearings."),
        ("Keep transmissions under 20 seconds.", "Long calls are cut off. Two short calls beat one long one."),
        ("Pause half a second after keying,", "then speak; wait 2 - 3 seconds for the answer and do not re-key over it."),
        ("CTAF style works.", "\"[Field] traffic, [callsign], [intent]\" is recognised like a direct tower call."),
        ("Emergencies bypass sequencing.", "\"mayday\", \"pan pan\" or \"declaring emergency\" puts you at the top "
         "of the queue; name a runway and it is yours."),
        ("ATIS letters only advance on a weather change.", "Information Alpha three broadcasts running is expected."),
        ("Position calls need Tacview.", "Radar check, bullseye, Hitman's checks and the 30 nm handoff need the rig's Tacview feed."),
    ])
    d.callout("If the ATC doesn't respond", [
        "1.  Addressed the right controller - field name, Command, Hitman, Deckboss, Marshal, Paddles, Texaco or Darkstar?",
        "2.  Included your full callsign with its number?",
        "3.  Used a call from the tables in this reference?",
        "4.  Right frequency (section 1), AM - or FM for Hitman? Encryption off?",
        "5.  SRS volume and mic - an open mic blocks reception. Still nothing: post the exact words you said."])

    json.dump(PHRASES, open(os.path.join(HERE, "phrases.json"), "w", encoding="utf-8", newline="\n"), indent=1)
    out = os.path.join(HERE, OUT_NAME)
    d.pdf.set_metadata({"title": "vSFG-7 AI ATC Phraseology Reference %s" % VER, "author": "vSFG-7",
                        "subject": "SkyEye ATC pilot calls, built from SkyeyeATC %s" % HEAD})
    for f in os.listdir(HERE):
        if f.startswith("vSFG7_ATC_Phraseology_") and f.endswith(".pdf") and f != OUT_NAME:
            os.remove(os.path.join(HERE, f))       # older versions live in git history
    d.pdf.save(out, deflate=True, garbage=3)
    if os.path.isdir(DESKTOP_COPY):
        shutil.copy2(out, os.path.join(DESKTOP_COPY, OUT_NAME))
        retire_old_versions(DESKTOP_COPY, "vSFG7_ATC_Phraseology_", OUT_NAME)
    print("PDF %d pages, %d phrases -> %s (%.1f MB)" % (d.n, len(PHRASES), out, os.path.getsize(out) / 1e6))


def retire_old_versions(folder, prefix, keep):
    """Move older versions of this PDF in the Desktop Documentation folder to its Old\
    (what was done by hand at every version; operator 2026-10-21 tidy)."""
    old = os.path.join(folder, "Old")
    for f in os.listdir(folder):
        if f.startswith(prefix) and f.endswith(".pdf") and f != keep:
            os.makedirs(old, exist_ok=True)
            dst = os.path.join(old, f)
            if os.path.exists(dst):
                os.remove(os.path.join(folder, f))
            else:
                shutil.move(os.path.join(folder, f), dst)


if __name__ == "__main__":
    build()
