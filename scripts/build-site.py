#!/usr/bin/env python3
"""Build the release landing page and online edition of the Pip workbook."""

import argparse
from html import escape
from itertools import groupby
import json
from pathlib import Path
import re
import shutil
from string import Template

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
SITE = ROOT / "site"
REPOSITORY = "https://github.com/fclaude/dinoscript"
PACKAGES = "https://github.com/fclaude/fclaude-packages"


def render_art(book_dir, output, guide):
    art_dir = output / "art"
    art_dir.mkdir(parents=True, exist_ok=True)
    names = {guide["cover_art"], *(lesson["art"] for lesson in guide["pages"] if lesson["art"])}
    art = {}
    for name in sorted(names):
        filename = Path(name).with_suffix(".webp").name
        with Image.open(book_dir / "art" / name) as image:
            image.thumbnail((1200, 800), Image.Resampling.LANCZOS)
            image.save(art_dir / filename, quality=86, method=6)
            art[name] = (f"art/{filename}", image.width, image.height)
    return art


def render_lesson(index, lesson, art):
    parts = [
        f'<article class="lesson" id="lesson-{index}">',
        f'<h3>{escape(lesson["title"])}</h3>',
        f'<p class="pip-quote">“{escape(lesson["pip"])}”</p>',
    ]
    parts.extend(f"<p>{escape(paragraph)}</p>" for paragraph in lesson["body"])
    parts.append(f'<pre><code>{escape(lesson["code"].rstrip())}</code></pre>')
    if lesson["checks"]:
        example = lesson["checks"][0]
        parts.append("<details><summary>Example run</summary>")
        if example["input"]:
            entries = ", then ".join(escape(entry) for entry in example["input"])
            parts.append(f"<p>Try entering {entries}.</p>")
        parts.append(f'<pre><samp>{escape(example["output"].rstrip())}</samp></pre></details>')
    if lesson["art"]:
        source, width, height = art[lesson["art"]]
        parts.append(
            f'<img src="{escape(source)}" alt="{escape(lesson["alt"])}" '
            f'width="{width}" height="{height}" loading="lazy">'
        )
    parts.extend([
        f'<p><strong>Try it.</strong> {escape(lesson["challenge"])}</p>',
        f'<details><summary>Pip’s hint</summary><p>{escape(lesson["hint"])}</p></details>',
        "</article>",
    ])
    return "\n".join(parts)


def render_workbook(guide, art):
    contents, chapters = [], []
    lessons = enumerate(guide["pages"], 1)
    groups = groupby(lessons, key=lambda entry: entry[1]["chapter"])
    for chapter_index, (name, group) in enumerate(groups, 1):
        entries = list(group)
        chapter_id = f"chapter-{chapter_index}"
        name = escape(name)
        contents.extend([
            f'<h3><a href="#{chapter_id}">{name}</a></h3>',
            f'<ol start="{entries[0][0]}">',
        ])
        chapters.extend([
            f'<section class="chapter" aria-labelledby="{chapter_id}">',
            f'<h2 id="{chapter_id}">{name}</h2>',
        ])
        for index, lesson in entries:
            contents.append(f'<li><a href="#lesson-{index}">{escape(lesson["title"])}</a></li>')
            chapters.append(render_lesson(index, lesson, art))
        contents.append("</ol>")
        chapters.append('<a class="contents-link" href="#contents">Back to contents ↑</a></section>')
    return "\n".join(contents), "\n".join(chapters)


def render_linux_downloads(release):
    tag = release.get("tag_name", "")
    if not re.fullmatch(r"packages-[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]+", tag):
        return ""
    assets = {asset["name"] for asset in release.get("assets", []) if asset["state"] == "uploaded"}
    versions = [
        match[1] for name in assets
        if (match := re.fullmatch(r"dinoscript_([0-9]+\.[0-9]+\.[0-9]+-[0-9]+)_amd64\.deb", name))
    ]
    if len(versions) != 1:
        return ""
    version, = versions
    files = {
        "ubuntu_x64": f"dinoscript_{version}_amd64.deb",
        "ubuntu_arm64": f"dinoscript_{version}_arm64.deb",
        "fedora_x64": f"dinoscript-{version}.x86_64.rpm",
        "fedora_arm64": f"dinoscript-{version}.aarch64.rpm",
        "arch_x64": f"dinoscript-{version}-x86_64.pkg.tar.zst",
        "x64_checksums": "SHA256SUMS-linux-amd64",
        "arm64_checksums": "SHA256SUMS-linux-arm64",
        "arch_checksums": "SHA256SUMS-archlinux-x86_64",
    }
    if not set(files.values()) <= assets:
        return ""
    download = f"{PACKAGES}/releases/download/{tag}"
    urls = {key: f"{download}/{name}" for key, name in files.items()}
    template = Template((SITE / "linux-downloads.html").read_text(encoding="utf-8"))
    return template.substitute(urls, tag=tag, version=version, release=f"{PACKAGES}/releases/tag/{tag}")


def build(tag, book_dir, output, linux_release):
    linux_downloads = render_linux_downloads(linux_release) if linux_release is not None else ""
    if linux_release is not None and not linux_downloads:
        raise ValueError("The Linux release is missing a verified package or checksum manifest")
    guide = json.loads((book_dir / "book.json").read_text(encoding="utf-8"))
    art = render_art(book_dir, output, guide)
    cover, cover_width, cover_height = art[guide["cover_art"]]
    contents, chapters = render_workbook(guide, art)
    values = {
        "tag": tag,
        "repository": REPOSITORY,
        "release": f"{REPOSITORY}/releases/tag/{tag}",
        "download": f"{REPOSITORY}/releases/download/{tag}",
        "linux_releases": f"{PACKAGES}/releases/latest",
        "linux_downloads": linux_downloads,
        "cover": escape(cover),
        "cover_alt": escape(guide["cover_alt"]),
        "cover_width": cover_width,
        "cover_height": cover_height,
        "contents": contents,
        "chapters": chapters,
    }
    values.update({key: escape(guide[key]) for key in ("title", "subtitle", "author", "welcome")})
    for page in ("index.html", "workbook.html"):
        template = Template((SITE / page).read_text(encoding="utf-8"))
        (output / page).write_text(template.substitute(values), encoding="utf-8")
    shutil.copyfile(SITE / "styles.css", output / "styles.css")
    shutil.copyfile(SITE / "linux-downloads.js", output / "linux-downloads.js")
    shutil.copyfile(book_dir / "Dinoscript-with-Pip.pdf", output / "Dinoscript-with-Pip.pdf")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag", help="release tag such as v0.1.0")
    parser.add_argument("--book-dir", type=Path, default=ROOT / "book")
    parser.add_argument("--output", type=Path, default=ROOT / "build" / "site")
    parser.add_argument("--linux-release", type=Path, help="GitHub API metadata for the published Linux package set")
    args = parser.parse_args()
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", args.tag):
        parser.error("tag must be a version such as v0.1.0")
    linux_release = json.loads(args.linux_release.read_text(encoding="utf-8")) if args.linux_release else None
    build(args.tag, args.book_dir, args.output, linux_release)


if __name__ == "__main__":
    main()
