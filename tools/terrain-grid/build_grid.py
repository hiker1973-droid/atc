"""Build a terrain height grid for the GCI's terrain masking (pkg/terrain).

Source: AWS Open Data "Terrain Tiles" (Tilezen / Mapzen, terrarium PNG
encoding, public bucket elevation-tiles-prod, no account), built from SRTM,
GMTED, ETOPO and national DEMs. The DCS maps here are real geography, so the
ground matches DCS closely; at zoom 8 a pixel is ~430 m at 44 N.

    python -I tools/terrain-grid/build_grid.py caucasus            (from the repo root)

Downloads the tiles once into tools/terrain-grid/_tiles/<zoom>/ (git-ignored),
samples them onto a regular 0.01-degree grid and writes
pkg/terrain/grids/<theatre>.grid.gz: gzip(JSON header line + int16 LE metres,
row 0 = the south edge). Sea and below-sea land are stored as 0.
"""
import gzip
import io
import json
import math
import os
import sys
import urllib.request

import numpy as np
from PIL import Image

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "..", "..", "pkg", "terrain", "grids")
URL = "https://s3.amazonaws.com/elevation-tiles-prod/terrarium/{z}/{x}/{y}.png"
ZOOM = 8
STEP = 0.01  # degrees

# DCS map extents (approximate, generous) - lat_min, lat_max, lon_min, lon_max.
THEATRES = {
    "caucasus": (40.0, 48.0, 27.0, 48.0),
    "syria": (31.0, 38.0, 31.0, 41.0),
    "kola": (64.0, 72.0, 10.0, 36.0),
}


def tile_xy(lat, lon, z):
    n = 2 ** z
    x = (lon + 180.0) / 360.0 * n
    y = (1.0 - math.asinh(math.tan(math.radians(lat))) / math.pi) / 2.0 * n
    return x, y


def tile(z, x, y):
    path = os.path.join(HERE, "_tiles", str(z), "%d_%d.png" % (x, y))
    if not os.path.exists(path):
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with urllib.request.urlopen(URL.format(z=z, x=x, y=y), timeout=60) as r:
            data = r.read()
        with open(path, "wb") as f:
            f.write(data)
    img = np.asarray(Image.open(path).convert("RGB"), dtype=np.float64)
    # terrarium: height = R*256 + G + B/256 - 32768 (metres)
    return img[:, :, 0] * 256 + img[:, :, 1] + img[:, :, 2] / 256 - 32768


def build(name):
    lat0, lat1, lon0, lon1 = THEATRES[name]
    rows = int(round((lat1 - lat0) / STEP)) + 1
    cols = int(round((lon1 - lon0) / STEP)) + 1
    lats = lat0 + STEP * np.arange(rows)
    lons = lon0 + STEP * np.arange(cols)
    # Every tile the grid touches.
    xa, ya = tile_xy(lat1, lon0, ZOOM)
    xb, yb = tile_xy(lat0, lon1, ZOOM)
    tiles = {}
    for tx in range(int(xa), int(xb) + 1):
        for ty in range(int(ya), int(yb) + 1):
            tiles[(tx, ty)] = tile(ZOOM, tx, ty)
    print("%s: %d tiles at zoom %d, grid %d x %d" % (name, len(tiles), ZOOM, rows, cols))
    out = np.zeros((rows, cols), dtype=np.int16)
    for r, lat in enumerate(lats):
        for c, lon in enumerate(lons):
            fx, fy = tile_xy(lat, lon, ZOOM)
            tx, ty = int(fx), int(fy)
            px = min(int((fx - tx) * 256), 255)
            py = min(int((fy - ty) * 256), 255)
            h = tiles[(tx, ty)][py, px]
            out[r, c] = max(0, min(int(round(h)), 9000))
    os.makedirs(OUT, exist_ok=True)
    path = os.path.join(OUT, "%s.grid.gz" % name)
    header = json.dumps({"name": name, "latMin": lat0, "lonMin": lon0, "step": STEP, "rows": rows, "cols": cols})
    raw = io.BytesIO()
    raw.write(header.encode() + b"\n")
    raw.write(out.astype("<i2").tobytes())
    with gzip.open(path, "wb", compresslevel=9) as f:
        f.write(raw.getvalue())
    print("wrote %s (%.1f MB), max %d m" % (path, os.path.getsize(path) / 1e6, out.max()))


if __name__ == "__main__":
    for n in sys.argv[1:] or ["caucasus"]:
        build(n)
