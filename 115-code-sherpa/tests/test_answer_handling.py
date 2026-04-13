"""Answer + citation handling via the orchestrator with mock LLM."""

from __future__ import annotations

from code_sherpa.llm.mock import MockLLMClient


def test_answer_has_text_and_citations(make_sherpa, fixture_repo):
    llm = MockLLMClient(responses=["Refunds go through PaymentService.refund."])
    sherpa = make_sherpa(llm=llm)
    sherpa.ingest("demo", str(fixture_repo))

    answer = sherpa.ask("demo", "where do we handle payment failures?")

    assert answer.insufficient_context is False
    assert "Refunds" in answer.text
    assert answer.citations
    assert any("payments.py" in c.path for c in answer.citations)
    assert llm.call_count == 1


def test_insufficient_context_returns_not_found_without_llm(make_sherpa, fixture_repo):
    llm = MockLLMClient(responses=["should not be used"])
    sherpa = make_sherpa(llm=llm)
    sherpa.ingest("demo", str(fixture_repo))

    answer = sherpa.ask("demo", "kubernetes helm chart istio sidecar zzzznomatch")

    assert answer.insufficient_context is True
    assert answer.text == "I couldn't find that in this codebase."
    assert answer.citations == []
    assert llm.call_count == 0  # short-circuited, no model call


def test_llm_error_is_typed_and_retrieval_preserved(make_sherpa, fixture_repo):
    sherpa = make_sherpa(llm=MockLLMClient(error=True))
    sherpa.ingest("demo", str(fixture_repo))

    answer = sherpa.ask("demo", "how does refund work?")

    assert answer.error is not None
    assert "fail" in answer.error.lower()
    # Retrieval still surfaced despite the LLM failure.
    assert answer.citations


def test_used_memory_flag(make_sherpa, fixture_repo):
    sherpa = make_sherpa(llm=MockLLMClient(responses=["ans"]))
    sherpa.ingest("demo", str(fixture_repo))
    sherpa.add_fact("demo", "refund", "refunds are processed via the gateway")

    answer = sherpa.ask("demo", "how does refund work?")
    assert answer.used_memory is True


def test_classification_is_recorded(make_sherpa, fixture_repo):
    sherpa = make_sherpa()
    sherpa.ingest("demo", str(fixture_repo))
    answer = sherpa.ask("demo", "where is the login function?")
    assert answer.classification == "locate"


def test_episode_recorded_after_ask(make_sherpa, fixture_repo):
    sherpa = make_sherpa(llm=MockLLMClient(responses=["a"]))
    sherpa.ingest("demo", str(fixture_repo))
    sherpa.ask("demo", "how does login work?")
    eps = sherpa.memory.recent_episodes("demo")
    assert eps and eps[0].question == "how does login work?"
