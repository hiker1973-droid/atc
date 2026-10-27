# -*- coding: utf-8 -*-
"""Case III Comms - two kneeboard pages + PDF of the pilot's Case III calls (Pillow).

What the pilot says to Readback ATC's Marshal and Paddles in a Case III recovery,
and what comes back, in the look of the squadron's DimOn Case III cards
(074_Case_III_Marshal / 075_Case_III_Approach, 1289 x 2190). Replies are the
first wording of the composer method Marshal calls (pkg/composer/composer.go)
on the build in BUILD; the stack, approach-time, dirty-up and bolter rules are
cmd/atc/marshal.go + marshal_case3.go.

Made 2026-10-27 (operator: "kneeboard and PDF of just Case 3 comms ... player
instruction comms, similar to these").

Run:  python -I -X utf8 docs/case3-flow/build_case3_flow.py      (from the repo root)
"""
import os
import re
import shutil

from PIL import Image, ImageDraw, ImageFont
from reportlab.pdfgen import canvas

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
BUILD = "Readback ATC main 5ce6d84, 27 OCT 2026"
OUT_PNG = ["Case_III_Comms_1_Marshal.png", "Case_III_Comms_2_Approach.png"]
OUT_PDF = "Case III Comms.pdf"
DESKTOP = [r"C:\Users\afhin\OneDrive\Desktop\SKYEYE ATC Dev\Documentation"]

W, H = 1289, 2190
M = 34  # side margin
FONTS = r"C:\Windows\Fonts"

BG = (242, 243, 243)
DARK = (28, 28, 30)
INK = (30, 30, 32)
GREY = (90, 92, 96)
LIGHT = (205, 208, 212)
YELLOW = (245, 197, 24)
TEAL = (16, 132, 132)
ORANGE = (222, 118, 12)
PURPLE = (140, 40, 220)
BLUE = (26, 110, 230)
RED = (200, 40, 40)
CHIP = (110, 112, 118)


def bahn(size, style="Regular"):
    f = ImageFont.truetype(os.path.join(FONTS, "bahnschrift.ttf"), size)
    f.set_variation_by_name(style)
    return f


def seg(name, size):
    return ImageFont.truetype(os.path.join(FONTS, name), size)


F_TITLE = bahn(58, "SemiBold")
F_KICK = bahn(27, "Bold")
F_PAGE = bahn(46, "Regular")
F_TAB = bahn(36, "Bold SemiCondensed")
F_INTRO = bahn(26, "SemiBold SemiCondensed")
F_WHEN = bahn(39, "Bold SemiCondensed")
F_LETTER = bahn(36, "Bold")
F_CHIP = bahn(22, "Bold")
F_SAY = seg("seguisbi.ttf", 35)       # what the pilot says: semibold italic, coloured
F_HEAR = seg("segoeuii.ttf", 29)      # what comes back: italic, grey
F_NOTE = bahn(29, "SemiCondensed")
F_NOTEB = bahn(29, "Bold SemiCondensed")
F_PANEL_H = bahn(32, "Bold SemiCondensed")
F_PANEL = bahn(29, "SemiCondensed")
F_PANEL_B = bahn(29, "Bold SemiCondensed")
F_PANEL_SAY = seg("seguisbi.ttf", 29)
F_FOOT = bahn(22, "SemiBold")
F_RADIO_H = bahn(24, "Bold SemiCondensed")
F_RADIO = bahn(33, "Bold")
F_RADIO_S = bahn(22, "SemiCondensed")


def text_w(d, s, f):
    return d.textlength(s, font=f)


def rich(d, x, y, maxw, segs, lead=1.32):
    """Draw [(text, font, colour)] word-wrapped from (x, y) within maxw, on one
    baseline per line whatever the fonts; returns the y below."""
    toks = []  # (word, font, colour, space-after width)
    for s, f, col in segs:
        for t in re.findall(r"\S+|\s+", s):
            if t.isspace():
                if toks:
                    w, tf, tc, sp = toks[-1]
                    toks[-1] = (w, tf, tc, sp + text_w(d, " ", f) * len(t))
            else:
                toks.append((t, f, col, 0))
    lh = max(f.size for _, f, _ in segs) * lead
    asc = max(f.getmetrics()[0] for _, f, _ in segs)
    cx, base = x, y + asc
    for wd, f, col, sp in toks:
        ww = text_w(d, wd, f)
        if cx > x and cx + ww > x + maxw:
            cx, base = x, base + lh
        d.text((cx, base), wd, font=f, fill=col, anchor="ls")
        cx += ww + sp
    return base - asc + lh


def logo(size):
    im = Image.open(os.path.join(REPO, "docs", "vsfg7_patch.png")).convert("RGB").resize((size, size), Image.LANCZOS)
    mask = Image.new("L", (size, size), 0)
    ImageDraw.Draw(mask).ellipse((0, 0, size - 1, size - 1), fill=255)
    return im, mask


class Page:
    def __init__(self, title, n, gap=0):
        self.gap = gap
        self.im = Image.new("RGB", (W, H), BG)
        self.d = ImageDraw.Draw(self.im)
        d = self.d
        # header
        d.rectangle((0, 0, W, 150), fill=DARK)
        lg, mk = logo(122)
        self.im.paste(lg, (30, 14), mk)
        d.ellipse((29, 13, 152, 136), outline=(120, 90, 40), width=3)
        d.text((178, 30), "vSFG-7  ·  READBACK ATC  ·  CASE III", font=F_KICK, fill=YELLOW)
        d.text((176, 62), title, font=F_TITLE, fill=(240, 240, 240))
        pg = "%d / 2" % n
        d.text((W - M - text_w(d, pg, F_PAGE), 52), pg, font=F_PAGE, fill=(150, 150, 150))
        d.rectangle((0, 150, W, 158), fill=YELLOW)
        # tab row
        d.polygon([(0, 172), (430, 172), (400, 232), (0, 232)], fill=DARK)
        d.text((22, 182), "PILOT CALLS", font=F_TAB, fill=(255, 255, 255))
        badge = "F/A-18C"
        bx = W - M - 300
        d.polygon([(bx, 186), (bx + 160, 186), (bx + 150, 222), (bx - 10, 222)], fill=CHIP)
        d.text((bx + 14, 188), badge, font=bahn(30, "Bold"), fill=(255, 255, 255))
        d.text((bx + 172, 188), "Readback", font=bahn(30, "SemiBold"), fill=GREY)
        d.text((M - 14, 246), "Night (1800 - 0600 local)  ·  Day: ceiling under 1,500 ft or visibility under 3 nm.  Marshal picks the case.",
               font=F_INTRO, fill=INK)
        d.line((0, 290, W, 290), fill=INK, width=3)
        self.y = 306

    def radios(self, items):
        d, y = self.d, self.y
        n = len(items)
        gap = 14
        bw = (W - 2 * M - gap * (n - 1)) / n
        for i, (head, freq, sub, col) in enumerate(items):
            x = M + i * (bw + gap)
            d.rounded_rectangle((x, y, x + bw, y + 112), radius=10, fill=(255, 255, 255), outline=col, width=3)
            d.rectangle((x, y, x + bw, y + 32), fill=col)
            d.text((x + 12, y + 3), head, font=F_RADIO_H, fill=(255, 255, 255))
            d.text((x + 12, y + 38), freq, font=F_RADIO, fill=col)
            d.text((x + 12, y + 80), sub, font=F_RADIO_S, fill=GREY)
        self.y = y + 112 + 22

    def chip(self, x, y, label, col):
        d = self.d
        w = text_w(d, label, F_CHIP) + 16
        d.rounded_rectangle((x, y + 7, x + w, y + 36), radius=5, fill=col)
        d.text((x + 8, y + 9), label, font=F_CHIP, fill=(255, 255, 255))
        return w

    def step(self, letter, col, when, lines, notes=(), last=False):
        """lines: [(kind, text)] with kind SAY / HEAR / READ BACK / THEN."""
        d = self.d
        y0 = self.y
        cx, cy, r = M + 30, y0 + 24, 27
        tx = M + 82
        maxw = W - M - tx
        d.text((tx, y0 + 4), when, font=F_WHEN, fill=col)
        y = y0 + 58
        for kind, txt in lines:
            say = kind in ("SAY", "READ BACK", "OR SAY")
            ccol = col if say else CHIP
            cw = self.chip(tx, y, kind, ccol)
            f, fc = (F_SAY, col) if say else (F_HEAR, GREY)
            y = rich(d, tx + cw + 12, y, maxw - cw - 12, [(txt, f, fc)], lead=1.28) + 4
        for nt in notes:
            if isinstance(nt, tuple):
                b, rest = nt
                segs = [("•  " + b + " ", F_NOTEB, INK), (rest, F_NOTE, INK)]
            else:
                segs = [("•  " + nt, F_NOTE, INK)]
            y = rich(d, tx, y, maxw, segs, lead=1.25)
        y += 10
        # the letter on a dotted spine
        if not last:
            for yy in range(int(cy + r + 6), int(y + 8 + self.gap), 12):
                d.ellipse((cx - 3, yy, cx + 3, yy + 6), fill=col)
        d.ellipse((cx - r, cy - r, cx + r, cy + r), fill=(255, 255, 255), outline=col, width=5)
        lw = text_w(d, letter, F_LETTER)
        d.text((cx - lw / 2, cy - 22), letter, font=F_LETTER, fill=col)
        self.y = y + 8 + self.gap

    def panel(self, title, rows, col=YELLOW):
        """rows: [[(text, kind)]] kind: n normal, b bold, s say (italic)."""
        d = self.d
        self.y -= self.gap  # no spread below the last step
        y0 = self.y + 6
        bottom = H - 70
        d.rectangle((0, y0, W, bottom), fill=(36, 36, 40))
        tw = text_w(d, title, F_PANEL_H)
        d.rectangle((W - M - tw - 30, y0 - 22, W - M, y0 + 26), fill=(70, 70, 76))
        d.text((W - M - tw - 15, y0 - 18), title, font=F_PANEL_H, fill=(255, 255, 255))
        y = y0 + 34
        fonts = {"n": (F_PANEL, (225, 225, 225)), "b": (F_PANEL_B, (255, 255, 255)),
                 "s": (F_PANEL_SAY, col), "g": (F_PANEL, (160, 160, 165))}
        for row in rows:
            segs = [(t, fonts[k][0], fonts[k][1]) for t, k in row]
            y = rich(d, M, y, W - 2 * M, segs, lead=1.3) + 6
        self.panel_bottom = y
        return y

    def footer(self, right):
        d = self.d
        d.rectangle((0, H - 70, W, H), fill=DARK)
        d.text((M - 8, H - 50), "Replies: " + BUILD + "  ·  first of 2 - 4 wordings shown",
               font=F_FOOT, fill=YELLOW)
        d.text((W - M - text_w(d, right, F_FOOT), H - 50), right, font=F_FOOT, fill=(150, 150, 150))


RADIOS = [
    ("MARSHAL", "306.300 AM", "COMM 1  CH 3  ·  to the deck", TEAL),
    ("PADDLES (LSO)", "128.300 AM", "COMM 2  ·  at 3 miles", BLUE),
    ("EXAMPLE", "BRC 275", "final bearing 266  ·  radial 086", CHIP),
]


def page1(gap=0):
    p = Page("Case III Comms 1: Marshal", 1, gap)
    p.radios(RADIOS)
    p.step("A", TEAL, "INSIDE 50 nm: CHECK IN WITH MARKING MOMS", [
        ("SAY", "Marshal, 302, marking moms 340 for 30, angels 12, state 6.5"),
        ("HEAR", "302, Union Marshal, radar contact, angels 12, range 30 miles from the Abe, bearing three four "
                 "zero, mother's weather ..., Case Three recovery, BRC 275, altimeter two niner niner two, marshal on "
                 "the zero eight six, two one DME, angels six, expected approach time is one five, report established."),
        ("READ BACK", "302, marshal on the 086, 21 DME, angels 6, approach time 15"),
    ], [("Marking moms", "is your bearing and range FROM the carrier (TACAN radial and DME)."),
        "No reply to the readback - that is normal. Say the modex first on every call."])
    p.step("B", TEAL, "AT YOUR ANGELS ON THE RADIAL: ESTABLISHED", [
        ("SAY", "302, established angels 6, state 6.1"),
        ("HEAR", "302, Union Marshal, roger established angels six, marshal on the zero eight six, two one DME, "
                 "expected approach time is one five."),
    ], [("No Charlie in Case III.", "Your approach time is your clearance to leave the stack."),
        "Hold on the radial: inbound leg toward the ship, turns away. Fly the shape that makes your minute."])
    p.step("C", TEAL, "LOST YOUR TIME? ASK FOR IT", [
        ("SAY", "Marshal, 302, request push time"),
        ("HEAR", "302, Union Marshal, your push time is one five."),
    ], [("Time already gone?", "Marshal gives you a new one at least 2 minutes out."),
        "Late in the stack by more than a few seconds - tell Marshal before you push."])
    p.step("D", ORANGE, "AT YOUR MINUTE: COMMENCING", [
        ("SAY", "302, commencing, altimeter 29.92, state 5.2"),
        ("HEAR", "302, Union Marshal, copy commencing, radar contact, descend to platform five thousand, "
                 "final bearing two six six, confirm BRC two seven five, report platform."),
    ], [("Descend 4,000 fpm at 250 kts", "on the final bearing. Set the final bearing on the TACAN course line."),
        "Continued on page 2: platform, dirty up, needles, Paddles, bolter."], last=True)
    p.panel("THE STACK", [
        [("Marshal radial", "b"), (" = final bearing + 180.    ", "n"), ("Final bearing", "b"),
         (" = BRC - 9 (angled deck).  All magnetic.", "n")],
        [("First jet: angels 6 at 21 DME.", "b"), ("  Each jet after takes the next angels up.  DME = angels + 15.", "n")],
        [("Approach times are 10 minutes apart.", "b"),
         ("  The first is about 10 minutes after your marking moms.", "n")],
        [("\"One five\"", "s"), (" = 15 minutes past the hour.  Commence on the minute, inside 10 seconds.", "n")],
        [("Readbacks and \"copy\" get no reply.", "b"),
         ("  After commencing you can drop \"Marshal\":  ", "n"), ("302, platform", "s"), ("  is heard.", "n")],
        [("Case changes while you are in the stack:  ", "n"),
         ("\"All marshal aircraft, recovery is now Case Three, acknowledge on next push.\"", "g")],
    ])
    p.footer("vSFG-7  Case III Comms 1/2")
    return p


def page2(gap=0):
    p = Page("Case III Comms 2: Approach", 2, gap)
    p.radios(RADIOS)
    p.step("E", ORANGE, "PASSING 5,000 ft: PLATFORM", [
        ("SAY", "302, platform"),
        ("HEAR", "302, Union Marshal, copy platform, final bearing two six six."),
        ("OR SAY", "302, checking in, 15 miles"),
        ("HEAR", "302, Union Marshal, radar contact, 15 DME, continue."),
    ], [("Slow to 2,000 fpm.", "Level at 1,200 ft by 10 miles, on the final bearing by 12.")])
    p.step("F", ORANGE, "8 MILES: MARSHAL CALLS DIRTY UP", [
        ("HEAR", "302, Union Marshal, eight miles, dirty up."),
    ], [("Gear, full flaps, hook.", "On speed by 6 miles at 1,200 ft. No reply needed.")])
    p.step("G", PURPLE, "NEEDLES (ACLS) - IF YOU HAVE THEM", [
        ("SAY", "302, needles up and left"),
        ("HEAR", "302, Union Marshal, roger needles."),
        ("OR SAY", "302, negative needles"),
        ("HEAR", "302, Union Marshal, roger negative needles, fly bullseye.   (fly the ICLS)"),
    ], [("Coupled / command control:", "\"302, coupled\" gets \"roger coupled.\" Optional calls.")])
    p.step("H", BLUE, "3 MILES: MARSHAL HANDS YOU TO PADDLES", [
        ("HEAR", "302, Union Marshal, three miles, contact paddles, call the ball."),
    ], [("Switch COMM 2 to 128.300.", "No three-mile initial call in Case III. Keep Marshal on COMM 1.")])
    p.step("J", BLUE, "3/4 MILE: BALL CALL TO PADDLES", [
        ("SAY", "302, Hornet ball, 5.0"),
        ("HEAR", "Roger ball.   Then fly his calls: \"a little high\", \"power\", \"right for lineup\"..."),
        ("OR SAY", "302, Clara, 5.0      (no ball)"),
        ("HEAR", "Paddles contact, keep it coming."),
    ], [("Trapped:", "on 128.3 say \"red deck\", then \"green deck\" once clear - Paddles sends you to Deckboss.")],
        last=True)
    p.panel("WAVE-OFF / BOLTER  ·  MARSHAL, COMM 1", [
        [("1  ", "b"), ("Gear up, flaps half, 150 kts.", "b"),
         ("  Straight ahead on the final bearing, climb to 1,200 ft. Marshal calls when radar sees you go round, "
          "or say  ", "n"), ("302, bolter", "s"), ("  /  ", "n"), ("302, wave off", "s")],
        [("   ", "n"), ("302, Union Marshal, climb to one thousand two hundred, at four miles turn downwind, report abeam.", "g")],
        [("2  ", "b"), ("At 4 miles upwind turn downwind,", "b"),
         (" 2 nm offset, 1,200 ft, 150 kts. Abeam the ship say  ", "n"), ("302, abeam", "s")],
        [("   ", "n"), ("302, Union Marshal, roger abeam, four miles past abeam turn final, final bearing two six six.", "g")],
        [("3  ", "b"), ("Dirty up, turn final,", "b"),
         (" intercept the final bearing. At 3 miles Marshal sends you to Paddles again - ball call as before.", "n")],
    ], col=YELLOW)
    p.footer("vSFG-7  Case III Comms 2/2")
    return p


def fit(build, steps):
    """Spread the steps so the panel ends just above the footer."""
    slack = (H - 70 - 24) - build(0).panel_bottom
    return build(max(0, slack / (steps - 1))).im


def main():
    pages = [fit(page1, 4), fit(page2, 5)]
    for im, name in zip(pages, OUT_PNG):
        im.save(os.path.join(HERE, name), optimize=True)
    pdf = os.path.join(HERE, OUT_PDF)
    pw, ph = W * 72 / 200, H * 72 / 200  # 200 dpi
    c = canvas.Canvas(pdf, pagesize=(pw, ph))
    c.setTitle("Case III Comms")
    for name in OUT_PNG:
        c.drawImage(os.path.join(HERE, name), 0, 0, pw, ph)
        c.showPage()
    c.save()
    for dst in DESKTOP:
        if os.path.isdir(dst):
            for name in OUT_PNG + [OUT_PDF]:
                shutil.copy2(os.path.join(HERE, name), os.path.join(dst, name))
    print("wrote", ", ".join(OUT_PNG + [OUT_PDF]))


if __name__ == "__main__":
    main()
