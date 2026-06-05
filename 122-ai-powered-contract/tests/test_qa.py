from __future__ import annotations

from contract_review import MockLLM, answer_question, build_report, select_relevant
from tests.conftest import qa_json


def test_select_relevant_picks_termination_clause(sample_clauses):
    selected = select_relevant("What is the termination notice period?", sample_clauses, top_k=1)
    assert len(selected) == 1
    assert selected[0].title == "Termination"


def test_answerable_question_is_grounded(sample_clauses):
    report = build_report(sample_clauses, [])
    llm = MockLLM(responses={"qa": qa_json("Thirty days written notice.", [2])})
    answer = answer_question("What is the termination notice period?", report, llm)
    assert answer.grounded is True
    assert answer.cited_clauses == [2]
    assert "Thirty days" in answer.answer


def test_absent_answer_not_grounded(sample_clauses):
    report = build_report(sample_clauses, [])
    llm = MockLLM(responses={"qa": qa_json("NOT_ADDRESSED_IN_CONTRACT", [])})
    answer = answer_question("What is the governing law?", report, llm)
    assert answer.grounded is False
    assert "does not address" in answer.answer
    assert answer.cited_clauses == []


def test_answer_question_accepts_clause_list(sample_clauses):
    llm = MockLLM(responses={"qa": qa_json("Indemnity is mutual.", [3])})
    answer = answer_question("indemnity?", sample_clauses, llm)
    assert answer.grounded is True
