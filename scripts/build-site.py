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


def build(tag, book_dir, output):
    guide = json.loads((book_dir / "book.json").read_text(encoding="utf-8"))
    art = render_art(book_dir, output, guide)
    cover, cover_width, cover_height = art[guide["cover_art"]]
    contents, chapters = render_workbook(guide, art)
    values = {
        "tag": tag,
        "repository": REPOSITORY,
        "release": f"{REPOSITORY}/releases/tag/{tag}",
        "download": f"{REPOSITORY}/releases/download/{tag}",
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
    shutil.copyfile(book_dir / "Dinoscript-with-Pip.pdf", output / "Dinoscript-with-Pip.pdf")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag", help="release tag such as v0.1.0")
    parser.add_argument("--book-dir", type=Path, default=ROOT / "book")
    parser.add_argument("--output", type=Path, default=ROOT / "build" / "site")
    args = parser.parse_args()
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", args.tag):
        parser.error("tag must be a version such as v0.1.0")
    build(args.tag, args.book_dir, args.output)


if __name__ == "__main__":
    main()
