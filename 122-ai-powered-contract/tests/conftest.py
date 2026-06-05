"""Shared pytest fixtures: sample contracts, a PDF builder, and MockLLM helpers.

The whole suite runs offline: the provider is forced to ``mock`` and every LLM
call is answered by a scripted ``MockLLM``. No API key and no network are needed.
"""

from __future__ import annotations

import json

import pytest

from contract_review.models import Chunk, Clause


@pytest.fixture(autouse=True)
def _force_mock_provider(monkeypatch):
    """Force offline provider and remove any ambient API key for every test."""

    monkeypatch.setenv("CONTRACT_LLM_PROVIDER", "mock")
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)


SHORT_CONTRACT = """\
SERVICES AGREEMENT

1. Term
This Agreement begins on the Effective Date and continues for one (1) year.

2. Termination
Either party may terminate this Agreement with thirty (30) days written notice.

3. Liability
In no event shall either party be liable for indirect or consequential damages.
"""


def make_numbered_contract(n_sections: int, body_words: int = 40) -> str:
    """Build a contract with ``n_sections`` numbered sections for chunking tests."""

    para = " ".join(["lorem"] * body_words)
    parts = ["MASTER SERVICES AGREEMENT", ""]
    for i in range(1, n_sections + 1):
        parts.append(f"Section {i}. Clause Heading {i}")
        parts.append(para + ".")
        parts.append("")
    return "\n".join(parts)


@pytest.fixture
def short_contract() -> str:
    return SHORT_CONTRACT


@pytest.fixture
def sample_clauses() -> list[Clause]:
    return [
        Clause(number=1, title="Term", text="This Agreement begins on the Effective Date and continues for one year."),
        Clause(number=2, title="Termination", text="Either party may terminate with thirty (30) days written notice."),
        Clause(number=3, title="Liability", text="No party is liable for indirect or consequential damages."),
    ]


@pytest.fixture
def sample_chunk() -> Chunk:
    return Chunk(index=0, text=SHORT_CONTRACT, start_char=0, end_char=len(SHORT_CONTRACT), est_tokens=120)


def extract_json(clauses: list[tuple[int, str, str]]) -> str:
    return json.dumps(
        {"clauses": [{"number": n, "title": t, "text": x} for n, t, x in clauses]}
    )


def analysis_json(
    clause_number: int,
    risk: str = "low",
    category: str = "general",
    explanation: str = "ok",
    ambiguous: list[str] | None = None,
    recommendation: str = "none",
) -> str:
    return json.dumps(
        {
            "clause_number": clause_number,
            "risk": risk,
            "category": category,
            "explanation": explanation,
            "ambiguous_language": ambiguous or [],
            "recommendation": recommendation,
        }
    )


def summary_json(text: str, key_terms: list[str], outstanding: list[str]) -> str:
    return json.dumps(
        {"text": text, "key_terms": key_terms, "outstanding_risks": outstanding}
    )


def qa_json(answer: str, cited: list[int] | None = None) -> str:
    return json.dumps({"answer": answer, "cited_clauses": cited or []})


@pytest.fixture
def jsonbuilders():
    """Expose the JSON builders to tests as a small namespace."""

    class _NS:
        extract = staticmethod(extract_json)
        analysis = staticmethod(analysis_json)
        summary = staticmethod(summary_json)
        qa = staticmethod(qa_json)

    return _NS


def make_simple_pdf(text: str) -> bytes:
    """Build a tiny single-page PDF with an extractable text layer."""

    objs = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "
        b"/Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
    ]
    stream = b"BT /F1 24 Tf 72 700 Td (" + text.encode("latin-1") + b") Tj ET"
    objs.append(
        b"<< /Length " + str(len(stream)).encode() + b" >>\nstream\n" + stream + b"\nendstream"
    )
    objs.append(b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

    pdf = bytearray(b"%PDF-1.4\n")
    offsets = []
    for i, body in enumerate(objs, start=1):
        offsets.append(len(pdf))
        pdf += str(i).encode() + b" 0 obj\n" + body + b"\nendobj\n"
    xref_pos = len(pdf)
    size = len(objs) + 1
    pdf += b"xref\n0 " + str(size).encode() + b"\n"
    pdf += b"0000000000 65535 f \n"
    for off in offsets:
        pdf += ("%010d 00000 n \n" % off).encode()
    pdf += b"trailer\n<< /Size " + str(size).encode() + b" /Root 1 0 R >>\n"
    pdf += b"startxref\n" + str(xref_pos).encode() + b"\n%%EOF"
    return bytes(pdf)
