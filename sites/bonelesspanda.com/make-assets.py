#!/usr/bin/env -S uv run --script
# /// script
# dependencies = ["pillow", "numpy"]
# ///
"""Turn the source Tarepanda art into the web assets this page serves.

The upstream asset (a 1000x1000 RGBA cutout) carries a white outer glow: a
band of pure white wrapped around the artwork, partly transparent at its
outer edge but fully opaque against the ink. On anything but a white page
that band reads as a sticker die-cut, so it has to go.

Alpha alone cannot find it -- the opaque part of the band is indistinguishable
from the panda's own white body by pixel value. What separates them is
connectivity: the glow touches the image border, the body does not. So flood
the white from the edges inward and drop whatever the flood reaches. The ink
outline is closed, so the flood stops at it.

Usage: ./make-assets.py   (reads source/, writes public/)
"""
import pathlib
import sys
from collections import deque

import numpy as np
from PIL import Image

WHITE = 248  # luminance at or above this counts as "white" for the flood
HERE = pathlib.Path(__file__).parent
SOURCE = HERE / "source" / "tarepanda-source.png"
PUBLIC = HERE / "public"


def strip_glow(img: Image.Image) -> Image.Image:
    """Drop every white pixel reachable from the image border."""
    lum = np.asarray(img.convert("L"), dtype=np.uint8)
    alpha = np.asarray(img.convert("RGBA").split()[-1], dtype=np.uint8).copy()
    h, w = lum.shape

    # Transparent pixels read as lum 0 (the cutout's RGB is zeroed), so the
    # flood has to treat them as passable too -- they are where it starts.
    white = (lum >= WHITE) | (alpha == 0)
    seen = np.zeros_like(white)
    queue = deque()

    for x in range(w):
        for y in (0, h - 1):
            if white[y, x] and not seen[y, x]:
                seen[y, x] = True
                queue.append((y, x))
    for y in range(h):
        for x in (0, w - 1):
            if white[y, x] and not seen[y, x]:
                seen[y, x] = True
                queue.append((y, x))

    while queue:
        y, x = queue.popleft()
        for dy, dx in ((1, 0), (-1, 0), (0, 1), (0, -1)):
            ny, nx = y + dy, x + dx
            if 0 <= ny < h and 0 <= nx < w and white[ny, nx] and not seen[ny, nx]:
                seen[ny, nx] = True
                queue.append((ny, nx))

    kept_before = int((alpha > 0).sum())
    alpha[seen] = 0
    kept_after = int((alpha > 0).sum())
    print(f"  flood removed {kept_before - kept_after} px, {kept_after} remain")
    if kept_after < kept_before * 0.35:
        sys.exit("flood leaked into the artwork -- raise WHITE or check the outline")

    # The art is graphite, so RGB carries no colour: LA halves the pixel data.
    return Image.merge("LA", (Image.fromarray(lum), Image.fromarray(alpha)))


def main() -> None:
    src = Image.open(SOURCE).convert("RGBA")
    print(f"source: {src.size}")

    panda = strip_glow(src)
    panda = panda.crop(panda.split()[-1].getbbox())
    print(f"cropped: {panda.size}")
    panda.save(PUBLIC / "tarepanda.png", optimize=True)

    # Square favicon: the whole panda, padded, so the tab icon is recognisable.
    side, pad = 256, 12
    fit = panda.copy()
    fit.thumbnail((side - 2 * pad, side - 2 * pad), Image.LANCZOS)
    icon = Image.new("LA", (side, side), (0, 0))
    icon.paste(fit, ((side - fit.width) // 2, (side - fit.height) // 2))
    icon.save(PUBLIC / "favicon.png", optimize=True)

    for name in ("tarepanda.png", "favicon.png"):
        print(f"  {name}: {(PUBLIC / name).stat().st_size} bytes")


if __name__ == "__main__":
    main()
