from __future__ import annotations

import pytest

from contract_review.errors import LLMOutputError
from contract_review.parsing import (
    normalise_risk,
    parse_analysis,
    parse_clauses,
    parse_qa,
    parse_summary,
)


def test_parse_clauses_valid():
    raw = '{"clauses": [{"number": 1, "title": "Term", "text": "One year."}]}'
    clauses = parse_clauses(raw)
    assert len(clauses) == 1
    assert clauses[0].title == "Term"


def test_parse_clauses_bare_array():
    raw = '[{"number": 1, "title": "T", "text": "x"}, {"number": 2, "title": "U", "text": "y"}]'
    assert len(parse_clauses(raw)) == 2


def test_parse_clauses_strips_code_fences():
    raw = '```json\n{"clauses": [{"number": 1, "title": "T", "text": "x"}]}\n```'
    assert len(parse_clauses(raw)) == 1


def test_parse_clauses_locates_json_in_prose():
    raw = 'Sure! Here are the clauses:\n{"clauses": [{"number": 1, "title": "T", "text": "x"}]}\nHope that helps.'
    assert len(parse_clauses(raw)) == 1


def test_parse_analysis_valid_and_clause_number_default():
    raw = '{"risk": "high", "category": "liability", "explanation": "bad", "ambiguous_language": ["reasonable"], "recommendation": "cap it"}'
    analysis = parse_analysis(raw, clause_number=7)
    assert analysis.clause_number == 7
    assert analysis.risk == "high"
    assert analysis.ambiguous_language == ["reasonable"]


def test_parse_analysis_risk_enum_normalisation():
    raw = '{"clause_number": 1, "risk": "critical", "category": "x", "explanation": "e", "ambiguous_language": [], "recommendation": "r"}'
    assert parse_analysis(raw).risk == "high"


def test_normalise_risk_unknown_to_medium():
    assert normalise_risk("bananas") == "medium"
    assert normalise_risk("moderate") == "medium"
    assert normalise_risk("minimal") == "low"


def test_parse_analysis_truncated_raises():
    with pytest.raises(LLMOutputError):
        parse_analysis('{"risk": "high", "category":')


def test_parse_garbage_raises():
    with pytest.raises(LLMOutputError):
        parse_clauses("not json at all")


def test_parse_summary_valid():
    raw = '{"text": "Looks ok.", "key_terms": ["term"], "outstanding_risks": ["liability"]}'
    summary = parse_summary(raw)
    assert summary.text == "Looks ok."
    assert summary.outstanding_risks == ["liability"]


def test_parse_qa_grounded():
    answer = parse_qa('{"answer": "Thirty days.", "cited_clauses": [2]}', "notice?")
    assert answer.grounded is True
    assert answer.cited_clauses == [2]


def test_parse_qa_sentinel_not_grounded():
    answer = parse_qa('{"answer": "NOT_ADDRESSED_IN_CONTRACT", "cited_clauses": []}', "q?")
    assert answer.grounded is False
    assert "does not address" in answer.answer
