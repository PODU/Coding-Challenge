"""Stage 3 tests: Document/Style -> valid, paginated PDF bytes."""

from __future__ import annotations

import io

import pytest
from conftest import assert_valid_pdf, pdf_media_box, pdf_page_count, pdf_text

from md2pdf import convert_markdown
from md2pdf.style import PAGE_SIZES, Style


def test_sample_produces_valid_pdf(sample_md, in_examples):
    data = convert_markdown(sample_md, Style())
    assert_valid_pdf(data)


def test_sample_element_coverage(sample_md, in_examples):
    data = convert_markdown(sample_md, Style())
    # Collapse whitespace so line-wrapping in the PDF doesn't defeat matches.
    text = " ".join(pdf_text(data).split())
    # Headings, paragraph, list items, link text, code content, image alt.
    for expected in [
        "Sample Document",
        "Inline emphasis",
        "First bullet",
        "Nested bullet",
        "First step",
        "Sub-step",
        "link to the challenge",
        "greet",
        "Hello",
    ]:
        assert expected in text, f"missing {expected!r} in rendered PDF text"


def test_special_chars_render_without_breaking(in_examples):
    data = convert_markdown("Text with <b>& <not a tag> stays literal\n", Style())
    assert_valid_pdf(data)
    assert "not a tag" in pdf_text(data)


def test_link_annotation_present(in_examples):
    data = convert_markdown("[anchor](https://example.com/page)\n", Style())
    assert_valid_pdf(data)
    # reportlab writes the URI into a link annotation in the PDF stream.
    assert b"example.com" in data


# Pagination (L3, L4) --------------------------------------------------------
def test_long_document_paginates(long_md, in_examples):
    data = convert_markdown(long_md, Style())
    assert_valid_pdf(data)
    assert pdf_page_count(data) > 1


def test_footer_page_numbers(long_md, in_examples):
    data = convert_markdown(long_md, Style())
    n = pdf_page_count(data)
    text = pdf_text(data)
    assert f"Page 1 of {n}" in text
    assert f"Page {n} of {n}" in text


# Page size (L1) -------------------------------------------------------------
@pytest.mark.parametrize("size", ["A4", "Letter"])
def test_page_size(size, in_examples):
    data = convert_markdown("# Hi\n", Style(page_size=size))
    w, h = pdf_media_box(data)
    exp_w, exp_h = PAGE_SIZES[size]
    assert abs(w - exp_w) < 1.0 and abs(h - exp_h) < 1.0


def test_page_sizes_differ(in_examples):
    a4 = pdf_media_box(convert_markdown("# Hi\n", Style(page_size="A4")))
    letter = pdf_media_box(convert_markdown("# Hi\n", Style(page_size="Letter")))
    assert a4 != letter


# Customization (C1-C4) ------------------------------------------------------
def test_custom_font_smoke(in_examples):
    data = convert_markdown(
        "# Heading\n\nBody text.\n", Style(body_font="Times-Roman", heading_scale=1.3)
    )
    assert_valid_pdf(data)
    assert b"Times" in data  # font resource name appears in the PDF


def test_custom_colours_smoke(in_examples):
    style = Style(background="#fafafa", link_color="#cc0000", code_bg="#eeeeee")
    data = convert_markdown("[x](http://y) and `code`\n", style)
    assert_valid_pdf(data)


# Determinism ----------------------------------------------------------------
def test_determinism(sample_md, in_examples):
    a = convert_markdown(sample_md, Style())
    b = convert_markdown(sample_md, Style())
    assert pdf_page_count(a) == pdf_page_count(b)
    assert pdf_text(a) == pdf_text(b)


# Edge cases -----------------------------------------------------------------
def test_empty_file_one_page(in_examples):
    data = convert_markdown("   \n\n", Style())
    assert_valid_pdf(data)
    assert pdf_page_count(data) == 1


def test_missing_image_degrades(capsys, in_examples):
    data = convert_markdown("![my alt](does-not-exist.png)\n", Style())
    assert_valid_pdf(data)
    text = pdf_text(data)
    assert "my alt" in text  # alt-text placeholder rendered
    assert "missing image" in capsys.readouterr().err  # warning on stderr


def test_deeply_nested_list(in_examples):
    md = "- a\n  - b\n    - c\n      - d\n        - e\n"
    data = convert_markdown(md, Style())
    assert_valid_pdf(data)
    assert "e" in pdf_text(data)


def test_long_url_does_not_crash(in_examples):
    url = "https://example.com/" + "x" * 300
    data = convert_markdown(f"See {url} now\n", Style())
    assert_valid_pdf(data)


def test_image_embeds(in_examples):
    data = convert_markdown("![logo](logo.png)\n", Style())
    assert_valid_pdf(data)
    # an embedded image produces an XObject image in the PDF
    assert b"/Image" in data or b"/XObject" in data


# Offline guarantee (7.6) ----------------------------------------------------
def test_no_network_by_default(monkeypatch, sample_md, in_examples):
    import socket

    def _boom(*a, **k):
        raise AssertionError("network access attempted during render")

    monkeypatch.setattr(socket, "socket", _boom)
    monkeypatch.setattr(socket, "create_connection", _boom)
    data = convert_markdown(sample_md, Style())
    assert_valid_pdf(data)
