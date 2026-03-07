"""Answer + citation handling (pure + mock LLM)."""

from __future__ import annotations

from rtfm.answer_formatter import REFUSAL_MESSAGE, format_answer
from rtfm.models import Chunk, LLMResponse, RetrievedChunk
from rtfm.prompt_builder import NOT_FOUND_SENTINEL


def _retrieved():
    return [
        RetrievedChunk(Chunk("git.md", "COMMIT", 0, "commit text"), 0.9),
        RetrievedChunk(Chunk("git.md", "LOG", 1, "log text"), 0.8),
        RetrievedChunk(Chunk("grep.md", "OPTIONS", 2, "grep text"), 0.7),
    ]


def test_citations_only_for_used_sources():
    resp = LLMResponse(text="Use commit. [1] See also [3].", stop_reason="end_turn")
    answer = format_answer(resp, _retrieved())
    assert answer.found
    files = [(c.filename, c.section) for c in answer.citations]
    assert files == [("git.md", "COMMIT"), ("grep.md", "OPTIONS")]


def test_citations_deduped_by_source():
    # [1] and [2] are both git.md but different sections -> kept; repeated [1] -> deduped.
    resp = LLMResponse(text="A [1]. B [1]. C [2].", stop_reason="end_turn")
    answer = format_answer(resp, _retrieved())
    files = [(c.filename, c.section) for c in answer.citations]
    assert files == [("git.md", "COMMIT"), ("git.md", "LOG")]


def test_not_in_docs_sentinel_yields_no_answer():
    resp = LLMResponse(text=NOT_FOUND_SENTINEL, stop_reason="end_turn")
    answer = format_answer(resp, _retrieved())
    assert answer.found is False
    assert answer.citations == []


def test_refusal_returns_decline_not_crash():
    resp = LLMResponse(text="", stop_reason="refusal")
    answer = format_answer(resp, _retrieved())
    assert answer.refused is True
    assert answer.found is False
    assert answer.text == REFUSAL_MESSAGE
    assert answer.citations == []


def test_out_of_range_citation_is_ignored():
    resp = LLMResponse(text="Answer [9].", stop_reason="end_turn")
    answer = format_answer(resp, _retrieved())
    assert answer.found
    assert answer.citations == []
