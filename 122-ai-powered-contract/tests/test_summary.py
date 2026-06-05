from __future__ import annotations

from contract_review import MockLLM, build_report, build_summary, stream_summary
from contract_review.models import Clause, ClauseAnalysis, ReviewDecision
from tests.conftest import summary_json


def _report():
    clauses = [Clause(number=1, title="Liability", text="liability text")]
    analyses = [ClauseAnalysis(clause_number=1, risk="high", category="liability", explanation="e", recommendation="cap")]
    return build_report(clauses, analyses)


def test_build_summary_returns_validated_model():
    raw = summary_json("Overall the contract is high risk.", ["liability"], ["uncapped liability"])
    llm = MockLLM(responses={"summary": raw})
    summary = build_summary(_report(), [], llm)
    assert summary.text.startswith("Overall")
    assert summary.key_terms == ["liability"]
    assert summary.outstanding_risks == ["uncapped liability"]


def test_summary_prompt_includes_reviewer_annotations():
    raw = summary_json("text", [], [])
    llm = MockLLM(responses={"summary": raw})
    decisions = [ReviewDecision(clause_number=1, decision="annotate", note="MISSING_CAP_FLAG")]
    build_summary(_report(), decisions, llm)
    call = llm.calls_for("summary")[0]
    assert "MISSING_CAP_FLAG" in call["messages"][0]["content"]


def test_stream_summary_yields_tokens_and_reconstructs():
    raw = summary_json("Streamed summary body here.", ["term"], [])
    llm = MockLLM(responses={"summary": raw})
    tokens = list(stream_summary(_report(), [], llm))
    assert len(tokens) > 1  # genuinely streamed in pieces
    assert "".join(tokens) == raw


def test_build_summary_stream_mode_parses_streamed_text():
    raw = summary_json("Streamed and parsed.", ["a"], ["b"])
    llm = MockLLM(responses={"summary": raw})
    summary = build_summary(_report(), [], llm, stream=True)
    assert summary.text == "Streamed and parsed."
    assert llm.calls[-1]["method"] == "stream"
