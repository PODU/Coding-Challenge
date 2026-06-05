from __future__ import annotations

from contract_review import Config, MockLLM, extract_clauses, load_text
from tests.conftest import extract_json, make_numbered_contract

A = "Alpha clause regarding the term and overall duration of this agreement period."
B = "Bravo clause regarding termination upon thirty days prior written notice to parties."
C = "Charlie clause regarding the limitation of liability and consequential damages."


def test_single_chunk_three_clauses(short_contract):
    llm = MockLLM(
        responses={
            "extract": extract_json(
                [(1, "Term", A), (2, "Termination", B), (3, "Liability", C)]
            )
        }
    )
    doc = load_text(short_contract)
    clauses = extract_clauses(doc, llm)
    assert len(clauses) == 3
    assert [c.number for c in clauses] == [1, 2, 3]
    # exactly one extract call per chunk (single chunk here)
    assert len(llm.calls_for("extract")) == 1


def test_multi_chunk_dedup_and_renumber():
    # Force many small chunks so the overlapping duplicate clause B appears twice.
    config = Config(provider="mock", max_chunk_tokens=80, chunk_overlap=10)
    doc = load_text(make_numbered_contract(6, body_words=40))
    llm = MockLLM(
        responses={
            "extract": [
                extract_json([(1, "Term", A), (2, "Termination", B)]),
                extract_json([(1, "Termination", B), (2, "Liability", C)]),
            ]
        }
    )
    clauses = extract_clauses(doc, llm, config)
    texts = [c.text for c in clauses]
    assert texts == [A, B, C]  # duplicate B collapsed
    assert [c.number for c in clauses] == [1, 2, 3]  # renumbered sequentially
    assert len(llm.calls_for("extract")) > 1  # genuinely multi-chunk
