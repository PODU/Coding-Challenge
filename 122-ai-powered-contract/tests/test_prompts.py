from __future__ import annotations

from contract_review.models import Clause, ClauseAnalysis, ReviewDecision, ReviewReport
from contract_review.prompts import (
    ANALYSIS_SCHEMA,
    EXTRACT_SCHEMA,
    QA_NOT_ADDRESSED,
    QA_SCHEMA,
    SUMMARY_SCHEMA,
    build_analyse_prompt,
    build_extract_prompt,
    build_qa_prompt,
    build_summary_prompt,
)
from contract_review.models import Chunk


def test_extract_prompt_structure_and_schema_instruction():
    chunk = Chunk(index=2, text="1. Term\nThe contract lasts a year.", start_char=0, end_char=10, est_tokens=5)
    system, messages = build_extract_prompt(chunk)
    assert "JSON" in system
    assert "clauses" in system
    assert messages[0]["role"] == "user"
    # Chunk text is embedded verbatim.
    assert "The contract lasts a year." in messages[0]["content"]
    assert "number" in EXTRACT_SCHEMA["properties"]["clauses"]["items"]["properties"]


def test_analyse_prompt_embeds_clause_and_enum():
    clause = Clause(number=4, title="Indemnity", text="Vendor shall indemnify Buyer fully.")
    system, messages = build_analyse_prompt(clause)
    assert "JSON" in system
    assert "Vendor shall indemnify Buyer fully." in messages[0]["content"]
    assert ANALYSIS_SCHEMA["properties"]["risk"]["enum"] == ["low", "medium", "high"]


def test_summary_prompt_includes_reviewer_annotations():
    report = ReviewReport(
        clauses=[Clause(number=1, title="Term", text="One year.")],
        analyses=[ClauseAnalysis(clause_number=1, risk="medium", category="term", explanation="e", recommendation="r")],
        overall_risk="medium",
        medium_risk_count=1,
    )
    decisions = [ReviewDecision(clause_number=1, decision="annotate", note="needs liability cap")]
    system, messages = build_summary_prompt(report, decisions)
    assert "JSON" in system
    body = messages[0]["content"]
    assert "needs liability cap" in body
    assert "annotate" in body
    assert "key_terms" in SUMMARY_SCHEMA["properties"]


def test_qa_prompt_contains_sentinel_instruction_and_question():
    clauses = [Clause(number=2, title="Termination", text="Thirty days notice required.")]
    system, messages = build_qa_prompt("What is the notice period?", clauses)
    assert QA_NOT_ADDRESSED in system
    assert "What is the notice period?" in messages[0]["content"]
    assert "Thirty days notice required." in messages[0]["content"]
    assert "answer" in QA_SCHEMA["properties"]
