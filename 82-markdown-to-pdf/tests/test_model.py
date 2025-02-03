"""Stage 2 tests: markdown-it token stream -> Document IR (no PDF involved)."""

from __future__ import annotations

from md2pdf.model import (
    Bold,
    Code,
    CodeBlock,
    Heading,
    ImageBlock,
    Italic,
    Link,
    ListBlock,
    Paragraph,
    Text,
    build_document,
)


def _first(doc, cls):
    for b in doc.blocks:
        if isinstance(b, cls):
            return b
    raise AssertionError(f"no {cls.__name__} in document")


def _flatten_text(nodes) -> str:
    out = []
    for n in nodes:
        if isinstance(n, Text):
            out.append(n.text)
        elif isinstance(n, (Bold, Italic, Link)):
            out.append(_flatten_text(n.children))
        elif isinstance(n, Code):
            out.append(n.text)
    return "".join(out)


# R1 -------------------------------------------------------------------------
def test_headings_levels():
    doc = build_document("# H1\n\n## H2\n\n### H3\n\n###### H6\n")
    headings = [b for b in doc.blocks if isinstance(b, Heading)]
    assert [h.level for h in headings] == [1, 2, 3, 6]
    assert _flatten_text(headings[0].inlines) == "H1"


# R2 -------------------------------------------------------------------------
def test_paragraph():
    doc = build_document("Hello world.\n\nSecond para.\n")
    paras = [b for b in doc.blocks if isinstance(b, Paragraph)]
    assert len(paras) == 2
    assert _flatten_text(paras[0].inlines) == "Hello world."


# R3/R4/R5 -------------------------------------------------------------------
def test_bold_italic_bolditalic():
    doc = build_document("**b** and *i* and ***bi***\n")
    p = _first(doc, Paragraph)
    kinds = [type(n).__name__ for n in p.inlines]
    assert "Bold" in kinds
    assert "Italic" in kinds
    # ***bi*** nests strong>em (or em>strong); ensure both present somewhere.
    flat = "".join(kinds)
    assert "Bold" in flat and "Italic" in flat


def test_emphasis_disambiguation():
    for src, top in [
        ("*a*", Italic),
        ("**a**", Bold),
        ("_a_", Italic),
        ("__a__", Bold),
    ]:
        doc = build_document(src + "\n")
        p = _first(doc, Paragraph)
        assert any(isinstance(n, top) for n in p.inlines), src


# R6 -------------------------------------------------------------------------
def test_unordered_list_nested():
    doc = build_document("- a\n- b\n  - b1\n  - b2\n")
    lst = _first(doc, ListBlock)
    assert lst.ordered is False
    assert len(lst.items) == 2
    nested = [b for b in lst.items[1].blocks if isinstance(b, ListBlock)]
    assert nested and nested[0].depth == 1 and nested[0].ordered is False


# R7 -------------------------------------------------------------------------
def test_ordered_list_and_start():
    doc = build_document("3. three\n4. four\n")
    lst = _first(doc, ListBlock)
    assert lst.ordered is True
    assert lst.start == 3
    assert len(lst.items) == 2


def test_mixed_nested_list_types():
    doc = build_document("1. one\n   - bullet\n2. two\n")
    lst = _first(doc, ListBlock)
    assert lst.ordered is True
    nested = [b for b in lst.items[0].blocks if isinstance(b, ListBlock)]
    assert nested and nested[0].ordered is False


# R8 -------------------------------------------------------------------------
def test_inline_code():
    doc = build_document("use `print()` here\n")
    p = _first(doc, Paragraph)
    codes = [n for n in p.inlines if isinstance(n, Code)]
    assert codes and codes[0].text == "print()"


# R9 -------------------------------------------------------------------------
def test_fenced_code_block_preserves_content():
    doc = build_document("```py\nx = 1\n  y = 2\n```\n")
    cb = _first(doc, CodeBlock)
    assert cb.lang == "py"
    assert cb.text == "x = 1\n  y = 2"


# R10 ------------------------------------------------------------------------
def test_link():
    doc = build_document("[click](https://example.com)\n")
    p = _first(doc, Paragraph)
    links = [n for n in p.inlines if isinstance(n, Link)]
    assert links and links[0].href == "https://example.com"
    assert _flatten_text(links[0].children) == "click"


# R11 ------------------------------------------------------------------------
def test_image_block():
    doc = build_document("![alt text](pic.png)\n")
    img = _first(doc, ImageBlock)
    assert img.src == "pic.png"
    assert img.alt == "alt text"


def test_special_chars_kept_literal():
    doc = build_document("5 < 10 & 10 > 5\n")
    p = _first(doc, Paragraph)
    assert "<" in _flatten_text(p.inlines)
    assert "&" in _flatten_text(p.inlines)


def test_empty_document():
    doc = build_document("")
    assert doc.blocks == []


def test_blockquote_degrades_to_paragraph():
    doc = build_document("> quoted text\n")
    p = _first(doc, Paragraph)
    assert "quoted text" in _flatten_text(p.inlines)
