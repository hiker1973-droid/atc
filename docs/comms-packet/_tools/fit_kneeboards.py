# -*- coding: utf-8 -*-
"""Render every comms card as a kneeboard page in the squadron format: EXACTLY 1289 x 2190.

    python _tools/fit_kneeboards.py OUT_DIR        (run by build_skyeye_packet.py)

The house rule (operator-approved 2026-10-13, Caucasus Training NOTES.md "THE KNEEBOARD PAGE
FORMAT" and its _BUILD/fitcards.py; the F:\\OMM kneeboard pack is all 1289 x 2190): every page
is one size, and a card is RE-RENDERED at the largest type scale that still fits one page -
never shrunk as a picture. A short card gets bigger type instead of an empty bottom third.
A card that does not fit even at the floor scale must be SPLIT, and this refuses it.

Each scale runs in its own process (missioncard.SCALE is read at import by newpilot_kit),
renders every card, and reports each card's content height; a card keeps the largest scale
at which it rendered clean (no guard tripped) and fits. Operator 2026-10-22: "make sure cards
are properly formated for kneeboards".
"""
import json
import os
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
BRIEFING = r"C:\Users\afhin\Saved Games\DCS\Missions\_MISSION_TOOLKIT\briefing"
PAGE_W, PAGE_H = 1289, 2190
# missioncard ships at 1.95. Largest first; 1.20 is fitcards' floor (type stops being
# readable in the pit below it).
SCALES = [round(2.60 - 0.02 * i, 2) for i in range(int((2.60 - 1.20) / 0.02) + 1)]

STUB = r"""
import json, os, sys
sys.path.insert(0, {briefing!r}); sys.path.insert(0, {here!r})
import missioncard as MC
MC.SCALE = {scale!r}
_save, seen = MC.Card.save, {{}}
def save(self, path, min_ratio=None):
    seen[os.path.basename(path)] = int(self.y + MC.S(16) + MC.S(44))
    return _save(self, path, min_ratio={ratio!r})
MC.Card.save = save
import newpilot_kit as NK
from PIL import Image, ImageDraw
_new = NK.new_card
def new_card(eyebrow, plain, gold, sublines, page):
    # newpilot_kit does not measure the eyebrow against the page counter on its line; at a
    # large scale "... HORNET + VIPER  ·  TOWERS" ran into "24 / 29".
    d = ImageDraw.Draw(Image.new("RGB", (10, 10)))
    need = (d.textlength(eyebrow, font=MC.F(16, True)) + MC.S(24) +
            d.textlength("%d / %d" % tuple(page), font=MC.F(20, True)))
    if need > MC.W - 2 * MC.MARGIN:
        raise SystemExit("EYEBROW runs into the page counter: %r" % eyebrow)
    return _new(eyebrow, plain, gold, sublines, page)
NK.new_card = new_card
import build_skyeye_packet as B
B.CARDS = {out!r}
os.makedirs(B.CARDS, exist_ok=True)
res = {{}}
for f in B.KNEEBOARD_FUNCS:
    try:
        p = f()
        res[f.__name__] = [os.path.basename(p), seen.get(os.path.basename(p)), ""]
    except (AssertionError, SystemExit, ValueError) as e:
        res[f.__name__] = [None, None, str(e)[:160] or type(e).__name__]
print("\n@@" + json.dumps(res))
"""


def run(scale, out):
    code = STUB.format(briefing=BRIEFING, here=HERE, scale=scale, out=out,
                       ratio=(PAGE_H + 0.001) / PAGE_W)
    r = subprocess.run([sys.executable, "-c", code], capture_output=True, text=True,
                       encoding="utf-8", errors="replace", cwd=HERE)
    tag = [ln for ln in r.stdout.splitlines() if ln.startswith("@@")]
    assert tag, "renderer died at scale %s\n%s\n%s" % (scale, r.stdout[-600:], r.stderr[-1500:])
    return json.loads(tag[-1][2:])


def fit(dest):
    """{function name: (file name, scale, path)} - every card at 1289 x 2190."""
    from PIL import Image
    work = os.path.join(HERE, "_cache", "kneeboard_fit")
    best, names = {}, None
    for s in SCALES:
        tmp = os.path.join(work, "%.2f" % s)
        res = run(s, tmp)
        if names is None:
            names = list(res)
        for fn, (name, content, err) in res.items():
            if fn in best or err or content is None or content > PAGE_H:
                continue
            p = os.path.join(tmp, name)
            assert Image.open(p).size == (PAGE_W, PAGE_H), "%s @ %s is %s" % (name, s, Image.open(p).size)
            best[fn] = (name, s, p)
        if set(names) <= set(best):
            break
    missing = [n for n in names if n not in best]
    assert not missing, "%s do not fit %d px even at scale %.2f - split the card" % (missing, PAGE_H, SCALES[-1])
    os.makedirs(dest, exist_ok=True)
    out = []
    for i, fn in enumerate(names, 1):
        name, s, p = best[fn]
        dst = os.path.join(dest, "SkyEye_%02d_%s" % (i, name.split("_", 1)[1]))
        shutil.copy2(p, dst)
        out.append((os.path.basename(dst), s))
    shutil.rmtree(work, ignore_errors=True)
    return out


if __name__ == "__main__":
    for n, s in fit(sys.argv[1]):
        print("  %-36s scale %.2f" % (n, s))
