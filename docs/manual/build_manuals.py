# -*- coding: utf-8 -*-
"""Readback ATC Installation Guide and User Manual - PDF builder.

The guides are written once, as the launcher's help pages
(cmd/launcher/help/install.html and manual.html, served at /help). This
prints the same files to PDF with Microsoft Edge (headless; its print engine
keeps table rows and headings together, which PyMuPDF's HTML layout did not),
then adds a footer with the title and page numbers with PyMuPDF. The PDFs go
to cmd/launcher/help/ (the launcher serves them, the installer puts them on
the PC) and docs/manual/ (to attach to a release or send). Operator
2026-10-23: "can we put link or have a PDF installer directions and also the
user manual?"

Run:  python -I docs/manual/build_manuals.py      (from the repo root)
"""
import datetime as dt
import os
import shutil
import subprocess
import sys
import tempfile
import time

import pymupdf

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
# Every page's footer carries the version and the day it was built (operator
# 2026-10-24: "Put a date on the PDF's").
VERSION = open(os.path.join(REPO, "product_version.txt"), encoding="utf-8").read().strip()
DATED = "Version %s  -  %s" % (VERSION, dt.date.today().strftime("%d %B %Y").lstrip("0"))
HELP = os.path.join(REPO, "cmd", "launcher", "help")
OUT = os.path.join(REPO, "docs", "manual")

GUIDES = [
    ("install.html", "Readback ATC Installation Guide.pdf", "Installation Guide"),
    ("manual.html", "Readback ATC User Manual.pdf", "User Manual"),
]

EDGE = [
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
]


def edge():
    for p in EDGE:
        if os.path.exists(p):
            return p
    sys.exit("Microsoft Edge not found: it prints the guides to PDF")


def build(src, dst_name, title):
    path = os.path.join(HELP, dst_name)
    url = "file:///" + os.path.join(HELP, src).replace("\\", "/")
    with tempfile.TemporaryDirectory() as profile:  # a throwaway profile: never the user's
        subprocess.run([edge(), "--headless", "--disable-gpu", "--no-pdf-header-footer",
                        "--user-data-dir=" + profile, "--print-to-pdf=" + path, url],
                       check=True, capture_output=True, timeout=120)
        for _ in range(50):  # Edge may return before the file is complete
            if os.path.exists(path) and os.path.getsize(path) > 0:
                break
            time.sleep(0.2)

    doc = pymupdf.open(path)
    for i, page in enumerate(doc):
        r = page.rect
        y = r.height - 24
        grey = (0.48, 0.5, 0.54)
        page.insert_text((43, y), "Readback ATC - " + title, fontsize=8, fontname="helv", color=grey)
        page.insert_text((r.width / 2 - pymupdf.get_text_length(DATED, "helv", 8) / 2, y), DATED,
                         fontsize=8, fontname="helv", color=grey)
        num = f"{i + 1} / {len(doc)}"
        page.insert_text((r.width - 43 - pymupdf.get_text_length(num, "helv", 8), y), num,
                         fontsize=8, fontname="helv", color=grey)
    doc.saveIncr()
    n = len(doc)
    doc.close()
    os.makedirs(OUT, exist_ok=True)
    shutil.copyfile(path, os.path.join(OUT, dst_name))
    print(f"{dst_name}: {n} pages")


def main():
    for src, dst, title in GUIDES:
        build(src, dst, title)
    return 0


if __name__ == "__main__":
    sys.exit(main())
