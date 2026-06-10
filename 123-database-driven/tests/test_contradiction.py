from __future__ import annotations

from llmwiki.llm import ScriptedLLM
from tests.fixtures.generate_corpus import CONTRADICTION_A, CONTRADICTION_B


def test_contradiction_detected_and_linted(wiki):
    wiki.create_project("p")
    wiki.ingest_text("p", CONTRADICTION_A, title="Ada A")
    res = wiki.ingest_text("p", CONTRADICTION_B, title="Ada B")
    # exactly one contradiction recorded (differing birth year for same entity)
    assert len(res.contradictions) == 1
    c = res.contradictions[0]
    assert "Ada Lovelace" in c.entity
    assert "1815" in (c.claim_a + c.claim_b)
    assert "1900" in (c.claim_a + c.claim_b)

    findings = wiki.lint("p")
    kinds = {f.kind for f in findings}
    assert "contradiction" in kinds


def test_agreeing_sources_no_contradiction(wiki):
    wiki.create_project("p")
    wiki.ingest_text("p", CONTRADICTION_A, title="Ada A")
    same = CONTRADICTION_A  # identical claims -> no contradiction
    res = wiki.ingest("p", same)
    assert res.contradictions == []


def test_judge_heuristic_directly():
    llm = ScriptedLLM()
    from llmwiki.agent import Agent

    a = Agent(llm)
    assert a.judge("Ada Lovelace was born in 1815", "Ada Lovelace was born in 1900") == "contradict"
    assert a.judge("Ada Lovelace was born in London", "Ada Lovelace was born in London") == "agree"
    assert a.judge("Ada Lovelace wrote algorithms", "Quantum computing uses qubits") == "unrelated"
    assert a.judge("The kernel is stable", "The kernel is not stable") == "contradict"
