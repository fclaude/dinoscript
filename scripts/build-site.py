#!/usr/bin/env python3
"""Build the landing page, Pip workbook, and Dinoscript playground."""

import argparse
from hashlib import sha256
from html import escape
from itertools import groupby
import json
import os
from pathlib import Path
import re
import shutil
from string import Template
import subprocess
from tempfile import TemporaryDirectory

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
SITE = ROOT / "site"
REPOSITORY = "https://github.com/fclaude/dinoscript"
PACKAGES = "https://github.com/fclaude/fclaude-packages"
SITE_URL = "https://dinoscript.org/"
EDITIONS = {
    "en": ("", "book.json", "Dinoscript-with-Pip.pdf", "English"),
    "es-CL": ("es/", "book.es-CL.json", "Dinoscript-con-Pip.pdf", "Español"),
    "pl": ("pl/", "book.pl.json", "Dinoscript-z-Pip.pdf", "Polski"),
}


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


def render_lesson(index, lesson, art, copy):
    parts = [
        f'<article class="lesson" id="lesson-{index}">',
        f'<h3>{escape(lesson["title"])}</h3>',
        f'<p class="pip-quote">“{escape(lesson["pip"])}”</p>',
    ]
    parts.extend(f"<p>{escape(paragraph)}</p>" for paragraph in lesson["body"])
    parts.append(f'<pre><code>{escape(lesson["code"].rstrip())}</code></pre>')
    parts.append(
        f'<p><a class="button secondary" href="play/?lesson={index}">{escape(copy["play_lesson"])}</a></p>'
    )
    if lesson["checks"]:
        example = lesson["checks"][0]
        parts.append(f'<details><summary>{escape(copy["example_run"])}</summary>')
        if "" in example["input"]:
            parts.append(f'<p>{escape(copy["example_press_enter"])}</p>')
        entries = copy["example_input_then"].join(answer for answer in example["input"] if answer)
        if entries:
            parts.append(f'<p>{escape(copy["example_input"].format(entries=entries))}</p>')
        parts.append(f'<pre><samp>{escape(example["output"].rstrip())}</samp></pre></details>')
    if lesson["art"]:
        source, width, height = art[lesson["art"]]
        parts.append(
            f'<img src="{escape(source)}" alt="{escape(lesson["alt"])}" '
            f'width="{width}" height="{height}" loading="lazy">'
        )
    parts.extend([
        f'<p><strong>{escape(copy["try_it"])}</strong> {escape(lesson["challenge"])}</p>',
        f'<details><summary>{escape(copy["hint"])}</summary><p>{escape(lesson["hint"])}</p></details>',
        "</article>",
    ])
    return "\n".join(parts)


def render_workbook(guide, art, copy):
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
            chapters.append(render_lesson(index, lesson, art, copy))
        contents.append("</ol>")
        chapters.append(f'<a class="contents-link" href="#contents">{escape(copy["back_to_contents"])} ↑</a></section>')
    return "\n".join(contents), "\n".join(chapters)


def render_linux_downloads(release, copy):
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
    labels = {
        "ubuntu_x64": "Ubuntu x64 DEB",
        "ubuntu_arm64": "Ubuntu ARM64 DEB",
        "fedora_x64": "Fedora x64 RPM",
        "fedora_arm64": "Fedora ARM64 RPM",
    }
    checksums = {
        "x64_checksums": "DEB/RPM x64",
        "arm64_checksums": "DEB/RPM ARM64",
        "arch_checksums": "Arch x64",
    }
    labels = {f"{key}_label": escape(copy["download_package"].format(package=package)) for key, package in labels.items()}
    labels.update({
        f"{key}_label": escape(copy["linux_checksums"].format(package=package))
        for key, package in checksums.items()
    })
    labels["arch_x64_label"] = escape(copy["download_arch_x64"])
    labels.update({key: escape(copy[key]) for key in ("linux_package", "pacman_package", "linux_install_help")})
    template = Template((SITE / "linux-downloads.html").read_text(encoding="utf-8"))
    return template.substitute({**urls, **labels}, tag=tag, version=version, release=f"{PACKAGES}/releases/tag/{tag}")


def page_links(language, page):
    routes = {key: SITE_URL + prefix + page for key, (prefix, *_) in EDITIONS.items()}
    return "\n  ".join([
        f'<link rel="canonical" href="{routes[language]}">',
        *(f'<link rel="alternate" hreflang="{key}" href="{url}">' for key, url in routes.items()),
        f'<link rel="alternate" hreflang="x-default" href="{routes["en"]}">',
    ])


def language_links(language, page, assets):
    return "\n        ".join(
        f'<a class="language-link" href="{assets}{prefix}{page}" hreflang="{key}" lang="{key}">{escape(name)}</a>'
        for key, (prefix, _, _, name) in EDITIONS.items() if key != language
    )


def build_play_assets(output):
    sources = [SITE / "play" / name for name in ("play.css", "play.js", "play-worker.js")]
    goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], cwd=ROOT, text=True).strip())
    sources.append(goroot / "lib" / "wasm" / "wasm_exec.js")
    with TemporaryDirectory() as directory:
        wasm = Path(directory) / "dinoscript.wasm"
        subprocess.run(
            ["go", "build", "-buildvcs=false", "-trimpath", "-ldflags=-s -w", "-o", str(wasm), "./cmd/dinoscript-web"],
            cwd=ROOT, env={**os.environ, "GOOS": "js", "GOARCH": "wasm"}, check=True,
        )
        sources.append(wasm)
        # Publishing a single versioned directory keeps the loader and binary in sync.
        digest = sha256()
        for source in sources:
            digest.update(sha256(source.read_bytes()).digest())
        relative = f"assets/play-{digest.hexdigest()[:16]}"
        target = output / relative
        target.mkdir(parents=True, exist_ok=True)
        for source in sources:
            shutil.copyfile(source, target / source.name)
    return relative


def render_playground(output, guide, play, values, play_assets):
    examples_dir = ROOT / "internal" / "desktop" / "examples"
    examples = {}
    options = []
    for key, (title, path) in play["examples"].items():
        examples[key] = {"filename": Path(path).name, "source": (examples_dir / path).read_text(encoding="utf-8")}
        options.append(f'<option value="{key}">{escape(title)}</option>')
    for index, lesson in enumerate(guide["pages"], 1):
        examples[f"lesson-{index}"] = {"filename": f'{play["lesson_file"]} {index}.ds', "source": lesson["code"]}
    messages = play["messages"]
    config = {"examples": examples, "starters": ["hello", "counting", "decisions", "number-magic"], "messages": messages}
    starter = examples["hello"]
    assets = "../" + values["assets"]
    language = values["language"]
    template = Template((SITE / "play" / "index.html").read_text(encoding="utf-8"))
    markup = template.substitute({
        **values,
        "assets": assets,
        "play_assets": assets + play_assets,
        "starter_filename": escape(starter["filename"]),
        "starter_source": escape(starter["source"]),
        "play_ready": escape(messages["ready"]),
        "play_empty": escape(messages["empty"]),
        "play_enter": escape(messages["enter"]),
        "initial_position": escape(messages["position"].format(line=1, column=1)),
        "play_config": json.dumps(config, ensure_ascii=True).replace("<", "\\u003c"),
        "example_options": "\n          ".join(options),
        "page_links": page_links(language, "play/"),
        "language_links": language_links(language, "play/", assets),
    })
    (output / "play").mkdir(exist_ok=True)
    (output / "play" / "index.html").write_text(markup, encoding="utf-8")


def build_edition(language, tag, book_dir, output, linux_release, play_assets):
    prefix, source, pdf, _ = EDITIONS[language]
    copy = json.loads((SITE / f"{language}.json").read_text(encoding="utf-8"))
    play = json.loads((SITE / "play" / f"{language}.json").read_text(encoding="utf-8"))
    copy.update(play["labels"])
    linux_downloads = render_linux_downloads(linux_release, copy) if linux_release is not None else ""
    if linux_release is not None and not linux_downloads:
        raise ValueError("The Linux release is missing a verified package or checksum manifest")
    if language != "en" and not (book_dir / source).is_file():
        # A site edition can launch before its first bundled app release.
        book_dir = ROOT / "book"
    guide = json.loads((book_dir / source).read_text(encoding="utf-8"))
    output = output / prefix
    art = render_art(book_dir, output, guide)
    cover, cover_width, cover_height = art[guide["cover_art"]]
    contents, chapters = render_workbook(guide, art, copy)
    greeting = guide["pages"][0]["checks"][0]["output"].rstrip("\n")
    values = {
        **{key: value if key.endswith("_html") else escape(value) for key, value in copy.items()},
        "language": language,
        "tag": tag,
        "assets": "../" if prefix else "",
        "pdf": pdf,
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
        "greeting_literal": escape(json.dumps(greeting, ensure_ascii=False)),
        "release_notes": escape(copy["release_notes"].format(tag=tag)),
        "by": escape(copy["by"].format(author=guide["author"])),
        "first_program_html": copy["first_program_html"].format(greeting=escape(greeting)),
    }
    values.update({key: escape(guide[key]) for key in ("title", "subtitle", "author", "welcome")})
    for page in ("index.html", "workbook.html"):
        template = Template((SITE / page).read_text(encoding="utf-8"))
        route = "" if page == "index.html" else page
        html = template.substitute(
            values, page_links=page_links(language, route), language_links=language_links(language, route, values["assets"]),
        )
        (output / page).write_text(html, encoding="utf-8")
    render_playground(output, guide, play, values, play_assets)
    shutil.copyfile(book_dir / pdf, output / pdf)


def build(tag, book_dir, output, linux_release):
    play_assets = build_play_assets(output)
    for language in EDITIONS:
        build_edition(language, tag, book_dir, output, linux_release, play_assets)
    shutil.copyfile(SITE / "styles.css", output / "styles.css")
    shutil.copyfile(SITE / "linux-downloads.js", output / "linux-downloads.js")
    shutil.copyfile(ROOT / "internal" / "desktop" / "pip.png", output / "pip.png")


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
