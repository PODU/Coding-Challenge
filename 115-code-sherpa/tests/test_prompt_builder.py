"""Prompt building: deterministic structure, citations, memory, no-context."""

from __future__ import annotations

from code_sherpa.memory.base import Episode, SemanticFact
from code_sherpa.models import CodeChunk, ScoredChunk
from code_sherpa.prompt.builder import PromptBuilder
from code_sherpa.prompt.select import ContextSelector, SelectedContext
from code_sherpa.prompt.templates import NO_CONTEXT_MARKER, SYSTEM_RULES


def _ctx():
    chunk = CodeChunk.make(
        "p", "src/payments.py", "python", "method", "PaymentService.refund",
        20, 22, "def refund(self, txn):\n    return self.gateway.refund(txn)", "sha",
    )
    return ContextSelector().select([ScoredChunk(chunk=chunk, score=1.0)])


def test_messages_have_system_and_user():
    msgs = PromptBuilder().build_messages("How do refunds work?", _ctx())
    assert len(msgs) == 2
    assert msgs[0].role == "system"
    assert msgs[0].content == SYSTEM_RULES
    assert msgs[1].role == "user"


def test_user_text_includes_citation_marker_and_question():
    text = PromptBuilder().build_user_text("How do refunds work?", _ctx())
    assert "[src/payments.py:20-22]" in text
    assert "PaymentService.refund" in text
    assert "```python" in text
    assert "Question: How do refunds work?" in text


def test_insufficient_context_branch():
    empty = SelectedContext(insufficient_context=True)
    text = PromptBuilder().build_user_text("anything?", empty)
    assert NO_CONTEXT_MARKER in text
    assert "I couldn't find that in this codebase." in text


def test_memory_injection():
    facts = [SemanticFact("p", "gateway", "we use Stripe")]
    episodes = [Episode("p", "what about refunds?", "...", [], 0.0)]
    text = PromptBuilder().build_user_text("refunds?", _ctx(), facts, episodes)
    assert "we use Stripe" in text
    assert "earlier Q: what about refunds?" in text


def test_prompt_is_deterministic():
    a = PromptBuilder().build_user_text("Q?", _ctx())
    b = PromptBuilder().build_user_text("Q?", _ctx())
    assert a == b
