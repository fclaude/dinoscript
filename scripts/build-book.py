#!/usr/bin/env python3
"""Lay out the print edition from the lesson data embedded in Dinoscript."""

import argparse
import hashlib
import io
from itertools import groupby
import json
import os
from pathlib import Path

from PIL import Image
from reportlab.lib.colors import HexColor
from reportlab.lib.pagesizes import letter
from reportlab.lib.utils import ImageReader
from reportlab.pdfbase.pdfmetrics import registerFont, stringWidth
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen.canvas import Canvas

BOOK = Path(__file__).resolve().parents[1] / "book"
WIDTH, HEIGHT = letter
MARGIN = 48
CONTENT = WIDTH - 2 * MARGIN

INK = "#183238"
PAPER = "#FFF9EB"
SOFT = "#536262"
WHITE = "#FFFDF5"
LINE = "#E9DDC8"
TEAL = "#125B58"
CHAPTER_COLORS = [
    ("#125B58", "#E4F1E8"),
    ("#984728", "#FAE8D9"),
    ("#5D487E", "#EEE7F7"),
    ("#345784", "#E5EEF9"),
]
EDITIONS = {
    "en": {
        "source": "book.json",
        "pdf": "Dinoscript-with-Pip.pdf",
        "cover": "Cover",
        "series": "A Dinoscript activity book",
        "with_pip": "with Pip",
        "pages": "Pages {first} to {last}",
        "license": "Copyright 2023-2026 Francisco Claude-Faust  ·  Apache License 2.0",
        "subject": "A Dinoscript workbook for kids · Apache License, Version 2.0",
        "pip": "Pip says",
        "enter": "Try entering {entries}",
        "then": ", then ",
        "output": "What you'll see",
        "try": "Try it",
        "hint": "Pip's hint",
        "notes": "Your notes",
    },
    "es-CL": {
        "source": "book.es-CL.json",
        "pdf": "Dinoscript-con-Pip.pdf",
        "cover": "Portada",
        "series": "Un cuaderno de actividades de Dinoscript",
        "with_pip": "con Pip",
        "pages": "Páginas {first} a {last}",
        "license": "Copyright 2023-2026 Francisco Claude-Faust  ·  Licencia Apache 2.0",
        "subject": "Un cuaderno de Dinoscript para niñas y niños · Licencia Apache 2.0",
        "pip": "Dice Pip",
        "enter": "Prueba ingresando {entries}",
        "then": ", luego ",
        "output": "Lo que verás",
        "try": "Ponlo a prueba",
        "hint": "Pista de Pip",
        "notes": "Tus notas",
    },
    "pl": {
        "source": "book.pl.json",
        "pdf": "Dinoscript-z-Pip.pdf",
        "cover": "Okładka",
        "series": "Zeszyt ćwiczeń do Dinoscript",
        "with_pip": "z Pip",
        "pages": "Strony {first}-{last}",
        "license": "Copyright 2023-2026 Francisco Claude-Faust  ·  Licencja Apache 2.0",
        "subject": "Zeszyt ćwiczeń z programowania dla dzieci · Licencja Apache 2.0",
        "pip": "Pip mówi",
        "enter": "Wpisz {entries}",
        "then": ", potem ",
        "output": "Co zobaczysz",
        "try": "Wypróbuj",
        "hint": "Podpowiedź od Pip",
        "notes": "Twoje notatki",
    },
}


def rectangle(c, x, y, w, h, color, radius=0):
    c.setFillColor(HexColor(color))
    if radius:
        c.roundRect(x, y, w, h, radius, stroke=0, fill=1)
    else:
        c.rect(x, y, w, h, stroke=0, fill=1)


def text(c, x, y, content, font, size, color=INK):
    c.setFont(font, size)
    c.setFillColor(HexColor(color))
    c.drawString(x, y, content)


def wrap(content, width, font, size):
    lines = []
    line = ""
    for word in content.split():
        candidate = f"{line} {word}" if line else word
        if line and stringWidth(candidate, font, size) > width:
            lines.append(line)
            line = word
        else:
            line = candidate
    if line:
        lines.append(line)
    return lines


def paragraph(c, x, top, content, width, size=14, font="Roboto", leading=19, color=INK):
    for line in wrap(content, width, font, size):
        text(c, x, top - size, line, font, size, color)
        top -= leading
    return top


def artwork(c, file, x, top, width):
    height = width * 2 / 3
    y = top - height
    clip = c.beginPath()
    clip.roundRect(x, y, width, height, 13)
    c.saveState()
    c.clipPath(clip, stroke=0, fill=0)
    encoded = io.BytesIO()
    with Image.open(BOOK / "art" / file) as art:
        art.convert("RGB").save(encoded, format="JPEG", quality=94, optimize=True)
    c.drawImage(ImageReader(encoded), x, y, width=width, height=height)
    c.restoreState()
    return y


def cover(c, book, chapters, edition):
    rectangle(c, 0, 0, WIDTH, HEIGHT, PAPER)
    c.bookmarkPage("cover")
    c.addOutlineEntry(edition["cover"], "cover")
    text(c, MARGIN, 714, edition["series"], "Roboto-Bold", 13, TEAL)
    text(c, MARGIN - 3, 642, "Dinoscript", "Roboto-Bold", 71)
    text(c, MARGIN - 1, 580, edition["with_pip"], "Roboto-Bold", 65, TEAL)
    subtitle_bottom = paragraph(c, MARGIN, 556, book["subtitle"], CONTENT, size=15, leading=19)
    artwork(c, book["cover_art"], MARGIN, min(519, subtitle_bottom - 8), CONTENT)
    text(c, MARGIN, 151, book["author"], "Roboto-Bold", 15)
    paragraph(c, MARGIN, 139, book["welcome"], CONTENT, size=12.8, leading=17)

    card_width = (CONTENT - 8 * (len(chapters) - 1)) / len(chapters)
    for i, (name, entries) in enumerate(chapters):
        x = MARGIN + i * (card_width + 8)
        accent, tint = CHAPTER_COLORS[i]
        number = edition["pages"].format(first=entries[0][0], last=entries[-1][0])
        rectangle(c, x, 48, card_width, 42, tint, 9)
        paragraph(c, x + 10, 87, name, card_width - 20, size=11.5, font="Roboto-Bold", leading=12, color=accent)
        text(c, x + 10, 54, number, "Roboto", 10, SOFT)
    text(c, MARGIN, 29, edition["license"], "Roboto", 8.5, SOFT)
    c.showPage()


def code_block(c, source, top, accent):
    lines = source.rstrip("\n").split("\n")
    size, spacing = (11.2, 12.5) if len(lines) > 16 else (11.6, 14.2)
    font = "DejaVuMono"
    inner = CONTENT - 34
    for line in lines:
        if stringWidth(line, font, size) > inner:
            raise ValueError(f"Code line will not fit on the page: {line}")
    height = 43 + len(lines) * spacing
    rectangle(c, MARGIN, top - height, CONTENT, height, "#213B40", 12)
    text(c, MARGIN + 16, top - 21, "DINOSCRIPT", "Roboto-Bold", 9, "#F6D985")
    baseline = top - 40
    for line in lines:
        color = "#A8D5C7" if line.lstrip().startswith("#") else "#FFF8E9"
        text(c, MARGIN + 16, baseline, line, font, size, color)
        baseline -= spacing
    return top - height


def card(c, x, top, width, label, content, accent, tint):
    line_height = 17
    height = 36 + line_height * len(wrap(content, width - 30, "Roboto", 12.5))
    rectangle(c, x, top - height, width, height, tint, 10)
    text(c, x + 15, top - 19, label, "Roboto-Bold", 11.5, accent)
    paragraph(c, x + 15, top - 26, content, width - 30, size=12.5, leading=line_height)
    return top - height


def lesson(c, page, index, colors, edition, book_title):
    chapter, title = page["chapter"], page["title"]
    accent, tint = colors
    key = f"page-{index}"
    c.bookmarkPage(key)
    c.addOutlineEntry(f"{index}. {title}", key)
    rectangle(c, 0, 0, WIDTH, HEIGHT, PAPER)
    rectangle(c, 0, HEIGHT - 8, WIDTH, 8, accent)
    text(c, MARGIN, 741, chapter.upper(), "Roboto-Bold", 11, accent)
    rectangle(c, MARGIN, 723, CONTENT, 1, LINE)
    top = paragraph(c, MARGIN, 706, title, CONTENT, size=32, font="Roboto-Bold", leading=35) - 13

    pip = "“" + page["pip"] + "”"
    top = card(c, MARGIN, top, CONTENT, edition["pip"], pip, accent, tint) - 15
    for part in page["body"]:
        top = paragraph(c, MARGIN, top, part, CONTENT, size=13.5, leading=18) - 8
    top = code_block(c, page["code"], top - 1, accent) - 16

    lines = page["checks"][0]["output"].strip().split("\n")
    if not page["art"] and len(page["code"].splitlines()) <= 10 and len(lines) <= 3:
        inputs = page["checks"][0]["input"]
        label = edition["enter"].format(entries=edition["then"].join(inputs)) if inputs else edition["output"]
        text(c, MARGIN + 2, top - 10, label, "Roboto-Bold", 10.5, accent)
        top -= 18
        for line in lines:
            text(c, MARGIN + 2, top - 10, line, "DejaVuMono", 10.5, SOFT)
            top -= 14
        top -= 10

    card_width = CONTENT
    art_bottom = top
    if page["art"]:
        art_width = 202
        card_width -= art_width + 14
        art_bottom = artwork(c, page["art"], MARGIN + card_width + 14, top, art_width)
    top = card(c, MARGIN, top, card_width, edition["try"], page["challenge"], accent, WHITE) - 8
    top = card(c, MARGIN, top, card_width, edition["hint"], page["hint"], accent, tint)
    if min(top, art_bottom) < 59:
        raise ValueError(f"Lesson {index} overflows the page (bottom at {min(top, art_bottom):.1f})")

    if min(top, art_bottom) > 158:
        text(c, MARGIN, 136, edition["notes"], "Roboto-Bold", 10.5, accent)
        rectangle(c, MARGIN, 109, CONTENT, 0.7, LINE)
        rectangle(c, MARGIN, 82, CONTENT, 0.7, LINE)
    rectangle(c, MARGIN, 45, CONTENT, 1, LINE)
    text(c, MARGIN, 26, book_title.upper(), "Roboto-Bold", 8, SOFT)
    c.setFont("Roboto-Bold", 11)
    c.setFillColor(HexColor(accent))
    c.drawRightString(WIDTH - MARGIN, 26, f"{index:02}")
    c.showPage()


def build(language):
    edition = EDITIONS[language]
    source = (BOOK / edition["source"]).read_bytes()
    book = json.loads(source)
    chapters = [
        (name, list(entries))
        for name, entries in groupby(enumerate(book["pages"], 1), key=lambda entry: entry[1]["chapter"])
    ]
    digest = hashlib.sha256(source)
    for asset in sorted((BOOK / "art").glob("*.png")):
        digest.update(asset.read_bytes())
    font_dir = BOOK / "fonts"
    for font in sorted(font_dir.glob("*.ttf")):
        digest.update(font.read_bytes())
    pdf = Canvas(str(BOOK / edition["pdf"]), pagesize=letter, pageCompression=1, invariant=1,
                 lang=language if language != "en" else None)
    pdf.setTitle(book["title"])
    pdf.setAuthor(book["author"])
    pdf.setCreator("Dinoscript")
    pdf.setKeywords("source-sha256 " + digest.hexdigest())
    pdf.setSubject(edition["subject"])
    cover(pdf, book, chapters, edition)
    for colors, (_, entries) in zip(CHAPTER_COLORS, chapters, strict=True):
        for index, page in entries:
            lesson(pdf, page, index, colors, edition, book["title"])
    pdf.save()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--language", choices=EDITIONS, help="build only this edition")
    args = parser.parse_args()
    os.environ.setdefault("SOURCE_DATE_EPOCH", "1794916800")
    font_dir = BOOK / "fonts"
    registerFont(TTFont("Roboto", str(font_dir / "Roboto-Regular.ttf")))
    registerFont(TTFont("Roboto-Bold", str(font_dir / "Roboto-Bold.ttf")))
    registerFont(TTFont("DejaVuMono", str(font_dir / "DejaVuSansMono.ttf")))
    for language in [args.language] if args.language else EDITIONS:
        build(language)


if __name__ == "__main__":
    main()
