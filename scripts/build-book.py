#!/usr/bin/env python3
"""Lay out the print edition from the lesson data embedded in Dinoscript."""

import hashlib
import io
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
CHAPTERS = {
    "Start coding": ("#125B58", "#E4F1E8"),
    "Text games": ("#984728", "#FAE8D9"),
    "Math puzzles": ("#5D487E", "#EEE7F7"),
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


def cover(c, book):
    rectangle(c, 0, 0, WIDTH, HEIGHT, PAPER)
    c.bookmarkPage("cover")
    c.addOutlineEntry("Cover", "cover")
    text(c, MARGIN, 714, "A Dinoscript activity book", "Roboto-Bold", 13, TEAL)
    text(c, MARGIN - 3, 642, "Dinoscript", "Roboto-Bold", 71)
    text(c, MARGIN - 1, 580, "with Pip", "Roboto-Bold", 65, TEAL)
    paragraph(c, MARGIN, 556, book["subtitle"], CONTENT, size=15, leading=19)
    artwork(c, book["cover_art"], MARGIN, 519, CONTENT)
    text(c, MARGIN, 151, book["author"], "Roboto-Bold", 15)
    paragraph(c, MARGIN, 139, book["welcome"], CONTENT, size=12.8, leading=17)

    chapters = [("Start coding", "Pages 1 to 8"), ("Text games", "Pages 9 to 11"), ("Math puzzles", "Pages 12 to 14")]
    card_width = (CONTENT - 16) / 3
    for i, (name, number) in enumerate(chapters):
        x = MARGIN + i * (card_width + 8)
        accent, tint = CHAPTERS[name]
        rectangle(c, x, 48, card_width, 42, tint, 9)
        text(c, x + 11, 72, name, "Roboto-Bold", 12, accent)
        text(c, x + 11, 57, number, "Roboto", 10, SOFT)
    text(c, MARGIN, 29, "Copyright 2023-2026 Francisco Claude-Faust  ·  Apache License 2.0",
         "Roboto", 8.5, SOFT)
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


def lesson(c, page, index):
    chapter, title = page["chapter"], page["title"]
    accent, tint = CHAPTERS[chapter]
    key = f"page-{index}"
    c.bookmarkPage(key)
    c.addOutlineEntry(f"{index}. {title}", key)
    rectangle(c, 0, 0, WIDTH, HEIGHT, PAPER)
    rectangle(c, 0, HEIGHT - 8, WIDTH, 8, accent)
    text(c, MARGIN, 741, chapter.upper(), "Roboto-Bold", 11, accent)
    rectangle(c, MARGIN, 723, CONTENT, 1, LINE)
    top = paragraph(c, MARGIN, 706, title, CONTENT, size=32, font="Roboto-Bold", leading=35) - 13

    pip = "“" + page["pip"] + "”"
    top = card(c, MARGIN, top, CONTENT, "Pip says", pip, accent, tint) - 15
    for part in page["body"]:
        top = paragraph(c, MARGIN, top, part, CONTENT, size=13.5, leading=18) - 8
    top = code_block(c, page["code"], top - 1, accent) - 16

    lines = page["checks"][0]["output"].strip().split("\n")
    if not page["art"] and len(page["code"].splitlines()) <= 10 and len(lines) <= 3:
        label = "Try entering " + ", then ".join(page["checks"][0]["input"]) if page["checks"][0]["input"] else "What you'll see"
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
    top = card(c, MARGIN, top, card_width, "Try it", page["challenge"], accent, WHITE) - 8
    top = card(c, MARGIN, top, card_width, "Pip's hint", page["hint"], accent, tint)
    if min(top, art_bottom) < 59:
        raise ValueError(f"Lesson {index} overflows the page (bottom at {min(top, art_bottom):.1f})")

    if min(top, art_bottom) > 158:
        text(c, MARGIN, 136, "Your notes", "Roboto-Bold", 10.5, accent)
        rectangle(c, MARGIN, 109, CONTENT, 0.7, LINE)
        rectangle(c, MARGIN, 82, CONTENT, 0.7, LINE)
    rectangle(c, MARGIN, 45, CONTENT, 1, LINE)
    text(c, MARGIN, 26, "DINOSCRIPT WITH PIP", "Roboto-Bold", 8, SOFT)
    c.setFont("Roboto-Bold", 11)
    c.setFillColor(HexColor(accent))
    c.drawRightString(WIDTH - MARGIN, 26, f"{index:02}")
    c.showPage()


def main():
    os.environ.setdefault("SOURCE_DATE_EPOCH", "1794916800")
    source = (BOOK / "book.json").read_bytes()
    book = json.loads(source)
    digest = hashlib.sha256(source)
    for asset in sorted((BOOK / "art").glob("*.png")):
        digest.update(asset.read_bytes())
    font_dir = BOOK / "fonts"
    for font in sorted(font_dir.glob("*.ttf")):
        digest.update(font.read_bytes())
    registerFont(TTFont("Roboto", str(font_dir / "Roboto-Regular.ttf")))
    registerFont(TTFont("Roboto-Bold", str(font_dir / "Roboto-Bold.ttf")))
    registerFont(TTFont("DejaVuMono", str(font_dir / "DejaVuSansMono.ttf")))
    pdf = Canvas(str(BOOK / "Dinoscript-with-Pip.pdf"), pagesize=letter, pageCompression=1, invariant=1)
    pdf.setTitle(book["title"])
    pdf.setAuthor(book["author"])
    pdf.setCreator("Dinoscript")
    pdf.setKeywords("source-sha256 " + digest.hexdigest())
    pdf.setSubject("A Dinoscript workbook for kids · Apache License, Version 2.0")
    cover(pdf, book)
    for index, page in enumerate(book["pages"], 1):
        lesson(pdf, page, index)
    pdf.save()


if __name__ == "__main__":
    main()
