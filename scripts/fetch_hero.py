#!/usr/bin/env python3
"""Fetch a CC0/public-domain hero image for the blog.

Searches Openverse with a hard license filter (CC0 + public domain mark),
downloads the best candidate, converts it to the blog's hero format
(1280x480 grayscale JPEG), registers it in the image library manifest,
and optionally attaches it to a post's front matter.

Runs in GitHub Actions (.github/workflows/fetch-hero.yml), where the
network is unrestricted; sandboxed Claude Code sessions trigger that
workflow instead of running this directly.
"""

import argparse
import datetime
import io
import json
import re
from pathlib import Path

import requests
from PIL import Image, ImageEnhance, ImageOps

API = "https://api.openverse.org/v1/images/"
HEADERS = {
    "User-Agent": "experiments-blog-hero-fetcher/1.0 (https://github.com/Packetslave/experiments)"
}
W, H = 1280, 480
LICENSE_LABELS = {"cc0": "CC0", "pdm": "public domain"}


def search(query):
    """Return Openverse results, best candidates first. License filter is
    enforced server-side (cc0,pdm) and re-checked per result later."""
    r = requests.get(
        API,
        params={"q": query, "license": "cc0,pdm", "page_size": 20},
        headers=HEADERS,
        timeout=30,
    )
    r.raise_for_status()
    results = r.json().get("results", [])

    def score(x):
        w, h = x.get("width") or 0, x.get("height") or 0
        if not w or not h:
            return -1
        landscape = 1.0 if w >= h else 0.5
        size = 1.0 if (w >= 1000 and h >= 500) else (0.6 if w >= 700 else 0.2)
        return landscape * size * min(w * h, 12_000_000)

    return sorted(results, key=score, reverse=True)


def download(result):
    url = result.get("url")
    if not url:
        return None
    try:
        r = requests.get(url, headers=HEADERS, timeout=60)
        r.raise_for_status()
        im = Image.open(io.BytesIO(r.content))
        im.load()
        return im
    except Exception as e:  # dead links and broken files are common; skip
        print(f"  skipping {url}: {e}")
        return None


def process(im):
    """Grayscale, autocontrast, crop to 8:3, resize to 1280x480."""
    im = im.convert("L")
    im = ImageOps.autocontrast(im, cutoff=1)
    w, h = im.size
    target = W / H
    if w / h > target:
        nw = int(h * target)
        x = (w - nw) // 2
        im = im.crop((x, 0, x + nw, h))
    else:
        nh = int(w / target)
        y = int((h - nh) * 0.4)  # bias the crop slightly above center
        im = im.crop((0, y, w, y + nh))
    if im.width < W:
        print(f"  note: upscaling from {im.width}px wide")
    im = im.resize((W, H), Image.LANCZOS)
    im = ImageEnhance.Contrast(im).enhance(1.05)
    return im.convert("RGB")


def slugify(text, max_words=4):
    words = re.findall(r"[a-z0-9]+", (text or "").lower())[:max_words]
    return "-".join(words) or "image"


def patch_front_matter(post_path, image_path, credit):
    text = post_path.read_text()
    m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
    if not m:
        raise SystemExit(f"no front matter found in {post_path}")
    fm = m.group(1)
    fm = re.sub(r"^image:.*$\n?", "", fm, flags=re.M)
    fm = re.sub(r"^imageCredit:.*$\n?", "", fm, flags=re.M)
    fm = fm.rstrip("\n") + f'\nimage: "{image_path}"\nimageCredit: "{credit}"'
    post_path.write_text(f"---\n{fm}\n---\n" + text[m.end() :])


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--query", required=True, help="search terms (literal visual subjects work best)")
    ap.add_argument("--slug", default="", help="post slug to attach to; blank = only stock the library")
    ap.add_argument("--repo-root", default=".")
    args = ap.parse_args()

    root = Path(args.repo_root)
    libdir = root / "blog/static/images/library"
    manifest_path = libdir / "manifest.json"
    libdir.mkdir(parents=True, exist_ok=True)
    manifest = json.loads(manifest_path.read_text()) if manifest_path.exists() else []

    post_path = None
    if args.slug:
        matches = sorted((root / "blog/content/posts").glob(f"*{args.slug}*.md"))
        if not matches:
            raise SystemExit(f"no post file matches slug '{args.slug}'")
        post_path = matches[-1]

    print(f"searching Openverse for: {args.query}")
    im, chosen = None, None
    for result in search(args.query):
        lic = LICENSE_LABELS.get((result.get("license") or "").lower())
        if not lic:
            continue  # never accept anything outside CC0 / public domain
        print(f"trying: {result.get('title')!r} by {result.get('creator')!r} [{result.get('license')}]")
        im = download(result)
        if im:
            chosen = result
            break
    if im is None:
        raise SystemExit("no usable image found; try different search terms")

    lic = LICENSE_LABELS[chosen["license"].lower()]
    creator = chosen.get("creator") or "unknown photographer"
    credit = f"Photo: {creator}, {lic} (via Openverse)"

    existing = next((e for e in manifest if e.get("openverse_id") == chosen.get("id")), None)
    if existing:
        rel = existing["file"]
        print(f"already in library as {rel}")
    else:
        fname = f"{slugify(chosen.get('title') or args.query)}-{chosen['id'][:8]}.jpg"
        out = libdir / fname
        process(im).save(out, quality=82, optimize=True)
        rel = f"images/library/{fname}"
        print(f"saved {out} ({out.stat().st_size // 1024} KB)")

        tags = [t.get("name") for t in (chosen.get("tags") or [])[:8] if t.get("name")]
        keywords = sorted(set(re.findall(r"[a-z0-9]+", args.query.lower()) + tags))
        manifest.append(
            {
                "file": rel,
                "title": chosen.get("title") or "",
                "creator": creator,
                "license": lic,
                "source": chosen.get("foreign_landing_url") or chosen.get("url") or "",
                "openverse_id": chosen.get("id"),
                "keywords": keywords,
                "added": datetime.date.today().isoformat(),
            }
        )
        manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
        print(f"library now has {len(manifest)} entries")

    if post_path:
        patch_front_matter(post_path, rel, credit)
        print(f"attached to {post_path.name}")


if __name__ == "__main__":
    main()
