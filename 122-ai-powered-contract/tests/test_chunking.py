from __future__ import annotations

from contract_review import chunk_document, estimate_tokens, load_text
from contract_review.models import ContractDocument
from tests.conftest import make_numbered_contract


def test_short_input_single_chunk(short_contract):
    doc = load_text(short_contract)
    chunks = chunk_document(doc, max_tokens=6000, overlap=400)
    assert len(chunks) == 1
    assert chunks[0].start_char == 0
    assert chunks[0].text == doc.text


def test_empty_document_yields_no_chunks():
    doc = ContractDocument(text="", source_path="x")
    assert chunk_document(doc) == []


def test_multi_chunk_token_budget_never_exceeded():
    doc = load_text(make_numbered_contract(10, body_words=30))
    max_tokens = 120
    chunks = chunk_document(doc, max_tokens=max_tokens, overlap=20)
    assert len(chunks) > 1
    for c in chunks:
        assert c.est_tokens <= max_tokens
        assert estimate_tokens(c.text) <= max_tokens


def test_char_offsets_slice_back_to_source():
    doc = load_text(make_numbered_contract(8, body_words=25))
    chunks = chunk_document(doc, max_tokens=100, overlap=20)
    for c in chunks:
        assert doc.text[c.start_char : c.end_char] == c.text


def test_overlap_produces_overlapping_ranges():
    doc = load_text(make_numbered_contract(10, body_words=30))
    chunks = chunk_document(doc, max_tokens=120, overlap=70)
    assert len(chunks) > 1
    overlaps = [
        chunks[i + 1].start_char < chunks[i].end_char for i in range(len(chunks) - 1)
    ]
    assert any(overlaps), "expected at least one overlapping chunk boundary"


def test_zero_overlap_is_non_overlapping():
    doc = load_text(make_numbered_contract(8, body_words=25))
    chunks = chunk_document(doc, max_tokens=100, overlap=0)
    for i in range(len(chunks) - 1):
        assert chunks[i + 1].start_char >= chunks[i].end_char


def test_boundaries_respected_at_line_starts():
    doc = load_text(make_numbered_contract(8, body_words=25))
    chunks = chunk_document(doc, max_tokens=100, overlap=20)
    for c in chunks:
        # Every chunk starts at the document start or just after a newline,
        # i.e. never in the middle of a line/paragraph.
        assert c.start_char == 0 or doc.text[c.start_char - 1] == "\n"
