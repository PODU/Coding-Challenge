"""Renders a Document to PDF with reportlab.

Reportlab's Platypus layer does the page layout; we only add the page
background and the "Page N of M" footer.
"""

from __future__ import annotations

import io
import os
import sys
import urllib.parse
import urllib.request
from typing import Callable

from reportlab.lib.colors import HexColor
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.utils import ImageReader
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    BaseDocTemplate,
    Flowable,
    Frame,
    Image,
    KeepTogether,
    ListFlowable,
    ListItem as RLListItem,
    PageTemplate,
    Paragraph,
    Spacer,
    XPreformatted,
)
from reportlab.pdfgen import canvas as _canvas

from .inline import escape_text, inlines_to_markup
from .model import (
    Block,
    CodeBlock,
    Document,
    Heading,
    ImageBlock,
    ListBlock,
    Paragraph as MdParagraph,
    ThematicBreak,
    build_document,
)
from .style import _FONT_VARIANTS, Style

__all__ = [
    "convert_markdown",
    "render_document",
]

_BULLET_CHARS = ["•", "◦", "▪", "⁃", "∙"]
_ORDERED_TYPES = ["1", "a", "i", "A", "I"]


def convert_markdown(
    text: str, style: Style | None = None, *, title: str | None = None
) -> bytes:
    style = style or Style()
    return render_document(build_document(text), style, title=title)


def render_document(
    document: Document, style: Style, *, title: str | None = None
) -> bytes:
    _register_custom_font(style)

    buffer = io.BytesIO()
    page_w, page_h = style.page_size_pt
    margin = style.margin_pt

    doc = BaseDocTemplate(
        buffer,
        pagesize=(page_w, page_h),
        leftMargin=margin,
        rightMargin=margin,
        topMargin=margin,
        bottomMargin=margin,
        title=title or "",
        author="md2pdf",
    )

    frame = Frame(
        margin,
        margin,
        page_w - 2 * margin,
        page_h - 2 * margin,
        id="body",
        leftPadding=0,
        rightPadding=0,
        topPadding=0,
        bottomPadding=0,
    )
    on_page = _make_background_painter(style)
    doc.addPageTemplates([PageTemplate(id="main", frames=[frame], onPage=on_page)])

    styles = _build_paragraph_styles(style)
    content_width = page_w - 2 * margin
    frame_height = page_h - 2 * margin
    flowables = _build_flowables(
        document.blocks, style, styles, content_width, frame_height
    )
    if not flowables:
        flowables = [Spacer(1, 1)]

    doc.build(flowables, canvasmaker=_make_numbered_canvas(style))
    return buffer.getvalue()


def _build_paragraph_styles(style: Style) -> dict[str, ParagraphStyle]:
    body = ParagraphStyle(
        "body",
        fontName=style.font_name(),
        fontSize=style.body_font_size,
        leading=style.body_leading,
        textColor=HexColor(style.text_color),
        alignment=TA_LEFT,
        spaceAfter=style.body_font_size * 0.6,
        wordWrap="LTR",
    )
    code = ParagraphStyle(
        "code",
        fontName=style.mono_font,
        fontSize=style.code_font_size,
        leading=style.code_font_size * 1.25,
        textColor=HexColor(style.code_color),
        backColor=HexColor(style.code_bg),
        borderPadding=6,
        spaceBefore=style.body_font_size * 0.4,
        spaceAfter=style.body_font_size * 0.6,
        wordWrap="CJK",  # lets long lines with no spaces wrap anywhere
    )
    alt = ParagraphStyle(
        "imgalt",
        parent=body,
        fontName=style.font_name(italic=True),
        textColor=HexColor("#888888"),
    )
    headings: dict[str, ParagraphStyle] = {}
    for level in range(1, 7):
        size = style.heading_size(level)
        headings[f"h{level}"] = ParagraphStyle(
            f"h{level}",
            fontName=style.font_name(bold=True),
            fontSize=size,
            leading=size * 1.2,
            textColor=HexColor(style.text_color),
            spaceBefore=size * 0.5,
            spaceAfter=size * 0.3,
            keepWithNext=True,
        )
    return {"body": body, "code": code, "imgalt": alt, **headings}


def _build_flowables(
    blocks: list[Block],
    style: Style,
    styles: dict[str, ParagraphStyle],
    content_width: float,
    frame_height: float,
) -> list[Flowable]:
    out: list[Flowable] = []
    for block in blocks:
        out.extend(
            _block_to_flowables(block, style, styles, content_width, frame_height)
        )
    return out


def _block_to_flowables(
    block: Block,
    style: Style,
    styles: dict[str, ParagraphStyle],
    content_width: float,
    frame_height: float,
) -> list[Flowable]:
    if isinstance(block, Heading):
        markup = inlines_to_markup(block.inlines, style)
        return [Paragraph(markup or " ", styles[f"h{min(block.level, 6)}"])]

    if isinstance(block, MdParagraph):
        markup = inlines_to_markup(block.inlines, style)
        if not markup.strip():
            return []
        return [Paragraph(markup, styles["body"])]

    if isinstance(block, CodeBlock):
        return [_code_flowable(block, styles["code"])]

    if isinstance(block, ImageBlock):
        return [_image_flowable(block, style, styles, content_width, frame_height)]

    if isinstance(block, ListBlock):
        return [_list_flowable(block, style, styles, content_width, frame_height)]

    if isinstance(block, ThematicBreak):
        return [_HRule(content_width, color=HexColor(style.text_color))]

    return []  # pragma: no cover


def _code_flowable(block: CodeBlock, code_style: ParagraphStyle) -> Flowable:
    # XPreformatted keeps whitespace and line breaks, but it also reads
    # markup, so the code has to be escaped.
    text = escape_text(block.text) if block.text else " "
    return XPreformatted(text, code_style)


def _list_flowable(
    block: ListBlock,
    style: Style,
    styles: dict[str, ParagraphStyle],
    content_width: float,
    frame_height: float,
) -> Flowable:
    depth = min(block.depth, len(_BULLET_CHARS) - 1)
    items: list[RLListItem] = []
    for item in block.items:
        flowables: list[Flowable] = []
        for child in item.blocks:
            flowables.extend(
                _block_to_flowables(child, style, styles, content_width, frame_height)
            )
        if not flowables:
            flowables = [Paragraph(" ", styles["body"])]
        items.append(RLListItem(flowables, leftIndent=0))

    common = dict(
        leftIndent=18,
        bulletFontName=style.font_name(),
        bulletFontSize=style.body_font_size,
    )
    if block.ordered:
        bullet_type = _ORDERED_TYPES[depth % len(_ORDERED_TYPES)]
        return ListFlowable(
            items,
            bulletType=bullet_type,
            start=block.start,
            bulletFormat="%s.",
            **common,
        )
    return ListFlowable(
        items,
        bulletType="bullet",
        bulletChar=_BULLET_CHARS[depth],
        **common,
    )


def _image_flowable(
    block: ImageBlock,
    style: Style,
    styles: dict[str, ParagraphStyle],
    content_width: float,
    frame_height: float,
) -> Flowable:
    data = _load_image_bytes(block.src, style)
    if data is None:
        label = block.alt.strip() or block.src or "image"
        return Paragraph(f"[image: {escape_text(label)}]", styles["imgalt"])

    try:
        reader = ImageReader(io.BytesIO(data))
        nat_w, nat_h = reader.getSize()
        if nat_w <= 0 or nat_h <= 0:
            raise ValueError("zero-sized image")
        scale = min(1.0, content_width / nat_w)
        disp_w = nat_w * scale
        disp_h = nat_h * scale
        # Leave some room so a full-height image still fits on a page.
        max_h = frame_height * 0.95
        if disp_h > max_h:
            shrink = max_h / disp_h
            disp_w *= shrink
            disp_h *= shrink
        img = Image(io.BytesIO(data), width=disp_w, height=disp_h)
        img.hAlign = "LEFT"
        return KeepTogether([img])
    except Exception as exc:  # pragma: no cover
        _warn(style, f"could not embed image {block.src!r}: {exc}")
        label = block.alt.strip() or block.src or "image"
        return Paragraph(f"[image: {escape_text(label)}]", styles["imgalt"])


def _load_image_bytes(src: str, style: Style) -> bytes | None:
    if not src:
        return None
    lower = src.lower()
    if lower.startswith(("http://", "https://")):
        if not style.allow_remote_images:
            _warn(style, f"skipping remote image (offline mode): {src}")
            return None
        try:  # pragma: no cover - exercised only with network access
            with urllib.request.urlopen(src, timeout=10) as resp:
                return resp.read()
        except Exception as exc:  # pragma: no cover
            _warn(style, f"failed to download image {src}: {exc}")
            return None
    if lower.startswith("data:"):
        return _decode_data_uri(src, style)
    if not os.path.isfile(src):
        _warn(style, f"missing image file: {src}")
        return None
    try:
        with open(src, "rb") as fh:
            return fh.read()
    except OSError as exc:
        _warn(style, f"could not read image {src}: {exc}")
        return None


def _decode_data_uri(src: str, style: Style) -> bytes | None:
    import base64

    try:
        header, _, payload = src.partition(",")
        if ";base64" in header:
            return base64.b64decode(payload)
        return urllib.parse.unquote_to_bytes(payload)
    except Exception as exc:  # pragma: no cover
        _warn(style, f"bad data-uri image: {exc}")
        return None


def _make_background_painter(style: Style) -> Callable[[object, object], None]:
    bg = HexColor(style.background)
    page_w, page_h = style.page_size_pt

    def _paint(canvas: object, doc: object) -> None:
        canvas.saveState()  # type: ignore[attr-defined]
        canvas.setFillColor(bg)  # type: ignore[attr-defined]
        canvas.rect(0, 0, page_w, page_h, stroke=0, fill=1)  # type: ignore[attr-defined]
        canvas.restoreState()  # type: ignore[attr-defined]

    return _paint


def _make_numbered_canvas(style: Style):
    """Return a Canvas class that adds the footer. The total page count isn't
    known until the end, so pages are held back and drawn in save()."""
    margin = style.margin_pt
    page_w, _page_h = style.page_size_pt
    footer_tpl = style.footer_text
    footer_font = style.font_name()
    footer_size = max(8.0, style.body_font_size - 2)
    footer_color = HexColor(style.text_color)

    class NumberedCanvas(_canvas.Canvas):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, **kwargs)
            self._saved_states: list[dict] = []

        def showPage(self) -> None:  # noqa: N802 (reportlab API)
            self._saved_states.append(dict(self.__dict__))
            self._startPage()

        def save(self) -> None:
            total = len(self._saved_states)
            for state in self._saved_states:
                self.__dict__.update(state)
                self._draw_footer(total)
                super().showPage()
            super().save()

        def _draw_footer(self, total: int) -> None:
            text = footer_tpl.format(page=self._pageNumber, total=total)
            self.saveState()
            self.setFont(footer_font, footer_size)
            self.setFillColor(footer_color)
            self.drawCentredString(page_w / 2.0, margin * 0.45, text)
            self.restoreState()

    return NumberedCanvas


class _HRule(Flowable):
    """A horizontal line, for ``---``."""

    def __init__(self, width: float, color, thickness: float = 0.6):
        super().__init__()
        self.width = width
        self.color = color
        self.thickness = thickness
        self.height = 8.0

    def wrap(self, avail_w, avail_h):  # noqa: N802
        self.width = avail_w
        return avail_w, self.height

    def draw(self) -> None:
        self.canv.saveState()
        self.canv.setStrokeColor(self.color)
        self.canv.setLineWidth(self.thickness)
        self.canv.line(0, self.height / 2, self.width, self.height / 2)
        self.canv.restoreState()


def _register_custom_font(style: Style) -> None:
    """If ``body_font`` is a font file, register it and point the style at it."""
    name = style.body_font
    if name in _FONT_VARIANTS:
        return
    if os.path.isfile(name) and name.lower().endswith((".ttf", ".otf")):
        family = os.path.splitext(os.path.basename(name))[0]
        try:
            pdfmetrics.registerFont(TTFont(family, name))
        except Exception as exc:  # pragma: no cover
            _warn(style, f"could not register font {name}: {exc}; using Helvetica")
            style.body_font = "Helvetica"
            return
        # We only have the one file, so bold and italic use it too.
        _FONT_VARIANTS[family] = {
            "normal": family,
            "bold": family,
            "italic": family,
            "bolditalic": family,
        }
        style.body_font = family
    else:
        _warn(style, f"unknown font {name!r}; falling back to Helvetica")
        style.body_font = "Helvetica"


def _warn(style: Style, message: str) -> None:
    style._warnings.append(message)
    print(f"md2pdf: warning: {message}", file=sys.stderr)
