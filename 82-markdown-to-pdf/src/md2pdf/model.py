"""The document model, built from markdown-it's tokens.

Nothing here depends on reportlab, so parsing can be tested without making a
PDF.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from markdown_it import MarkdownIt
from markdown_it.token import Token


@dataclass
class Inline:
    """Base class for nodes inside a line of text."""


@dataclass
class Text(Inline):
    text: str


@dataclass
class Bold(Inline):
    children: list[Inline] = field(default_factory=list)


@dataclass
class Italic(Inline):
    children: list[Inline] = field(default_factory=list)


@dataclass
class Code(Inline):
    text: str


@dataclass
class Link(Inline):
    href: str
    children: list[Inline] = field(default_factory=list)


@dataclass
class SoftBreak(Inline):
    pass


@dataclass
class HardBreak(Inline):
    pass


@dataclass
class Block:
    """Base class for paragraphs, headings, lists and so on."""


@dataclass
class Heading(Block):
    level: int
    inlines: list[Inline] = field(default_factory=list)


@dataclass
class Paragraph(Block):
    inlines: list[Inline] = field(default_factory=list)


@dataclass
class CodeBlock(Block):
    text: str
    lang: str = ""


@dataclass
class ImageBlock(Block):
    src: str
    alt: str = ""


@dataclass
class ListItem:
    blocks: list[Block] = field(default_factory=list)


@dataclass
class ListBlock(Block):
    ordered: bool
    items: list[ListItem] = field(default_factory=list)
    start: int = 1
    depth: int = 0


@dataclass
class ThematicBreak(Block):
    pass


@dataclass
class Document:
    blocks: list[Block] = field(default_factory=list)


def build_document(text: str) -> Document:
    """Parse ``text`` (CommonMark) into a :class:`Document`."""
    md = MarkdownIt("commonmark")
    tokens = md.parse(text or "")
    blocks, _ = _parse_blocks(tokens, 0, stop=None, depth=0)
    return Document(blocks=blocks)


_LIST_OPEN = {"bullet_list_open", "ordered_list_open"}


def _parse_blocks(
    tokens: list[Token], i: int, stop: str | None, depth: int
) -> tuple[list[Block], int]:
    """Parse block tokens up to the ``stop`` token, or to the end."""
    blocks: list[Block] = []
    n = len(tokens)
    while i < n:
        tok = tokens[i]
        ttype = tok.type

        if stop is not None and ttype == stop:
            return blocks, i + 1

        if ttype == "heading_open":
            level = _tag_level(tok.tag)
            inline_tok = tokens[i + 1]
            inlines = _parse_inline(inline_tok.children or [])
            blocks.append(Heading(level=level, inlines=inlines))
            i = _skip_to_close(tokens, i + 1, "heading_close")

        elif ttype == "paragraph_open":
            inline_tok = tokens[i + 1]
            blocks.extend(_paragraph_blocks(inline_tok.children or []))
            i = _skip_to_close(tokens, i + 1, "paragraph_close")

        elif ttype in _LIST_OPEN:
            list_block, i = _parse_list(tokens, i, depth)
            blocks.append(list_block)

        elif ttype == "fence" or ttype == "code_block":
            blocks.append(CodeBlock(text=tok.content.rstrip("\n"), lang=(tok.info or "").strip()))
            i += 1

        elif ttype == "blockquote_open":
            # Quotes aren't styled; just render what's inside.
            inner, i = _parse_blocks(tokens, i + 1, "blockquote_close", depth)
            blocks.extend(inner)

        elif ttype == "hr":
            blocks.append(ThematicBreak())
            i += 1

        elif ttype == "table_open":
            # Tables aren't supported; keep their text as a paragraph.
            text, i = _collect_inline_text(tokens, i + 1, "table_close")
            if text.strip():
                blocks.append(Paragraph(inlines=[Text(text.strip())]))

        elif ttype == "html_block":
            # Raw HTML is shown as text.
            stripped = tok.content.strip()
            if stripped:
                blocks.append(Paragraph(inlines=[Text(stripped)]))
            i += 1

        else:
            i += 1

    return blocks, i


def _parse_list(tokens: list[Token], i: int, depth: int) -> tuple[ListBlock, int]:
    open_tok = tokens[i]
    ordered = open_tok.type == "ordered_list_open"
    start = 1
    if ordered:
        start_attr = open_tok.attrs.get("start")
        if start_attr is not None:
            try:
                start = int(start_attr)
            except (TypeError, ValueError):
                start = 1
    close_type = "ordered_list_close" if ordered else "bullet_list_close"

    items: list[ListItem] = []
    i += 1
    n = len(tokens)
    while i < n and tokens[i].type != close_type:
        if tokens[i].type == "list_item_open":
            item_blocks, i = _parse_blocks(tokens, i + 1, "list_item_close", depth + 1)
            items.append(ListItem(blocks=item_blocks))
        else:  # pragma: no cover
            i += 1
    i += 1  # the list's close token
    return ListBlock(ordered=ordered, items=items, start=start, depth=depth), i


def _paragraph_blocks(children: list[Token]) -> list[Block]:
    """Turn a paragraph into blocks. Images become ImageBlocks of their own,
    splitting the paragraph's text around them."""
    blocks: list[Block] = []
    pending: list[Inline] = []

    def flush() -> None:
        if any(not _is_blank_inline(x) for x in pending):
            blocks.append(Paragraph(inlines=list(pending)))
        pending.clear()

    i = 0
    n = len(children)
    while i < n:
        child = children[i]
        if child.type == "image":
            flush()
            src = child.attrs.get("src", "")
            alt = child.content or _inline_plain_text(child.children or [])
            blocks.append(ImageBlock(src=str(src), alt=alt))
            i += 1
        else:
            nodes, i = _parse_one_inline(children, i)
            pending.extend(nodes)
    flush()

    if not blocks:
        blocks.append(Paragraph(inlines=[]))
    return blocks


def _parse_inline(children: list[Token]) -> list[Inline]:
    out: list[Inline] = []
    i = 0
    n = len(children)
    while i < n:
        tok = children[i]
        ttype = tok.type
        if ttype == "text":
            out.append(Text(tok.content))
            i += 1
        elif ttype == "strong_open":
            inner, i = _inline_until(children, i + 1, "strong_close")
            out.append(Bold(children=inner))
        elif ttype == "em_open":
            inner, i = _inline_until(children, i + 1, "em_close")
            out.append(Italic(children=inner))
        elif ttype == "link_open":
            href = str(tok.attrs.get("href", ""))
            inner, i = _inline_until(children, i + 1, "link_close")
            out.append(Link(href=href, children=inner))
        elif ttype == "code_inline":
            out.append(Code(tok.content))
            i += 1
        elif ttype == "image":
            # Images nested inside links or emphasis are shown as their alt text.
            alt = tok.content or _inline_plain_text(tok.children or [])
            if alt:
                out.append(Text(alt))
            i += 1
        elif ttype == "softbreak":
            out.append(SoftBreak())
            i += 1
        elif ttype == "hardbreak":
            out.append(HardBreak())
            i += 1
        elif ttype in ("s_open", "s_close"):  # strikethrough isn't rendered
            i += 1
        else:
            if tok.content:
                out.append(Text(tok.content))
            i += 1
    return out


def _inline_until(
    children: list[Token], i: int, stop: str
) -> tuple[list[Inline], int]:
    """Parse inline tokens up to ``stop``. Returns the nodes and the index just
    past ``stop``."""
    out: list[Inline] = []
    n = len(children)
    while i < n and children[i].type != stop:
        sub, i = _parse_one_inline(children, i)
        out.extend(sub)
    return out, i + 1


def _parse_one_inline(children: list[Token], i: int) -> tuple[list[Inline], int]:
    tok = children[i]
    ttype = tok.type
    if ttype == "strong_open":
        inner, j = _inline_until(children, i + 1, "strong_close")
        return [Bold(children=inner)], j
    if ttype == "em_open":
        inner, j = _inline_until(children, i + 1, "em_close")
        return [Italic(children=inner)], j
    if ttype == "link_open":
        href = str(tok.attrs.get("href", ""))
        inner, j = _inline_until(children, i + 1, "link_close")
        return [Link(href=href, children=inner)], j
    return _parse_inline([tok]), i + 1


def _tag_level(tag: str) -> int:
    if tag and len(tag) == 2 and tag[0] in "hH" and tag[1].isdigit():
        return int(tag[1])
    return 1


def _skip_to_close(tokens: list[Token], i: int, close_type: str) -> int:
    n = len(tokens)
    while i < n and tokens[i].type != close_type:
        i += 1
    return i + 1


def _collect_inline_text(
    tokens: list[Token], i: int, stop: str
) -> tuple[str, int]:
    parts: list[str] = []
    n = len(tokens)
    while i < n and tokens[i].type != stop:
        if tokens[i].type == "inline":
            parts.append(_inline_plain_text(tokens[i].children or []))
        i += 1
    return " ".join(p for p in parts if p), i + 1


def _inline_plain_text(children: list[Token]) -> str:
    parts: list[str] = []
    for tok in children:
        if tok.children:
            parts.append(_inline_plain_text(tok.children))
        elif tok.content:
            parts.append(tok.content)
    return "".join(parts)


def _is_blank_inline(node: Inline) -> bool:
    if isinstance(node, Text):
        return node.text.strip() == ""
    return isinstance(node, SoftBreak)
