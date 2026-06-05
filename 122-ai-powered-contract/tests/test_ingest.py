from __future__ import annotations

import importlib.util

import pytest

from contract_review import load_contract, load_text, normalise_text
from contract_review.errors import MalformedDocumentError
from tests.conftest import make_simple_pdf

HAS_PYPDF = importlib.util.find_spec("pypdf") is not None


def test_normalise_collapses_blank_lines_and_line_endings():
    text = "A\r\n\r\n\r\n\r\nB   \r\nC"
    out = normalise_text(text)
    assert "\r" not in out
    assert "\n\n\n" not in out
    assert out.splitlines()[0] == "A"


def test_normalise_strips_page_number_artefacts():
    text = "Clause one.\n\nPage 2\n\nClause two.\n\n- 3 -\n\n12"
    out = normalise_text(text)
    assert "Page 2" not in out
    assert "- 3 -" not in out
    assert "Clause one." in out
    assert "Clause two." in out


def test_load_text_roundtrip():
    doc = load_text("SERVICES AGREEMENT\n\n1. Term\nText.", source_path="mem")
    assert doc.source_path == "mem"
    assert "Term" in doc.text


def test_load_text_empty_raises():
    with pytest.raises(MalformedDocumentError):
        load_text("   \n  \n ")


def test_load_contract_txt(tmp_path, short_contract):
    p = tmp_path / "contract.txt"
    p.write_text(short_contract, encoding="utf-8")
    doc = load_contract(p)
    assert doc.meta["format"] == "txt"
    assert "Termination" in doc.text


def test_load_contract_md(tmp_path):
    p = tmp_path / "contract.md"
    p.write_text("# Heading\n\n1. Term\nBody.", encoding="utf-8")
    doc = load_contract(p)
    assert doc.meta["format"] == "md"


def test_load_contract_missing_file(tmp_path):
    with pytest.raises(MalformedDocumentError):
        load_contract(tmp_path / "nope.txt")


def test_load_contract_empty_file(tmp_path):
    p = tmp_path / "empty.txt"
    p.write_text("   \n\n  ", encoding="utf-8")
    with pytest.raises(MalformedDocumentError):
        load_contract(p)


@pytest.mark.skipif(not HAS_PYPDF, reason="pypdf not installed")
def test_load_contract_pdf_text_layer(tmp_path):
    p = tmp_path / "contract.pdf"
    p.write_bytes(make_simple_pdf("Termination Clause"))
    doc = load_contract(p)
    assert doc.meta["format"] == "pdf"
    assert doc.page_count == 1
    assert "Termination" in doc.text


@pytest.mark.skipif(not HAS_PYPDF, reason="pypdf not installed")
def test_load_contract_pdf_no_text_layer(tmp_path):
    # A structurally valid PDF whose single page has no text content.
    p = tmp_path / "blank.pdf"
    p.write_bytes(make_simple_pdf(""))
    with pytest.raises(MalformedDocumentError):
        load_contract(p)
