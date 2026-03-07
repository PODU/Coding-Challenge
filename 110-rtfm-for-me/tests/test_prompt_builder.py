"""Prompt construction (pure + round-trip through the mock LLM)."""

from __future__ import annotations

from rtfm.answer_formatter import format_answer
from rtfm.llm import MockLLMClient
from rtfm.models import Chunk, RetrievedChunk, Turn
from rtfm.prompt_builder import NOT_FOUND_SENTINEL, build_prompt


def _rc(filename, section, position, text, score) -> RetrievedChunk:
    return RetrievedChunk(
        chunk=Chunk(filename=filename, section=section, position=position, text=text),
        score=score,
    )


def _retrieved():
    return [
        _rc("git.md", "COMMIT", 0, "git commit records changes", 0.9),
        _rc("grep.md", "OPTIONS", 1, "the -i flag ignores case", 0.7),
    ]


def test_prompt_contains_every_chunk_text_and_source_label_and_grounding():
    retrieved = _retrieved()
    p = build_prompt("How do I commit?", retrieved)
    for rc in retrieved:
        assert rc.chunk.text in p.user
        assert rc.chunk.filename in p.user
        assert rc.chunk.section in p.user
    # Grounding instruction + the not-found sentinel live in the system prompt.
    assert "ONLY" in p.system
    assert NOT_FOUND_SENTINEL in p.system


def test_chunks_appear_in_rank_order_and_question_after_context():
    retrieved = _retrieved()
    p = build_prompt("How do I commit?", retrieved)
    pos_first = p.user.index("git commit records changes")
    pos_second = p.user.index("the -i flag ignores case")
    pos_question = p.user.index("Question:")
    assert pos_first < pos_second < pos_question
    # Sources are numbered for citation.
    assert "[1]" in p.user and "[2]" in p.user


def test_memory_injected_when_present_and_absent_when_not():
    retrieved = _retrieved()
    turns = [Turn(question="what is git?", answer="a vcs")]
    mems = ["prefers go"]
    with_mem = build_prompt("How?", retrieved, recent_turns=turns, long_term_memories=mems)
    assert "prefers go" in with_mem.user
    assert "what is git?" in with_mem.user

    without = build_prompt("How?", retrieved)
    assert "long-term memory" not in without.user.lower()
    assert "Recent conversation" not in without.user


def test_no_assistant_prefill_constructed():
    # build_prompt yields only a system + user pair; there is no assistant turn.
    p = build_prompt("How?", _retrieved())
    assert set(vars(p).keys()) == {"system", "user"}


def test_round_trip_through_mock_llm():
    retrieved = _retrieved()
    p = build_prompt("How do I commit?", retrieved)
    mock = MockLLMClient("Run git commit -m. [1]")
    resp = mock.complete(system=p.system, user=p.user)

    assert len(mock.calls) == 1
    assert mock.calls[0] == (p.system, p.user)
    answer = format_answer(resp, retrieved)
    assert answer.text == "Run git commit -m. [1]"
    assert answer.found
