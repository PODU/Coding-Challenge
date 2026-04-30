"""Multi-turn memory: follow-ups carry context; /reset clears; ids stay stable."""

from __future__ import annotations

from support_bot.config import Config
from support_bot.factory import build_bot
from support_bot.llm.mock import MockLLMClient
from support_bot.models import Answer
from support_bot.observability import InMemoryMetricsSink


def _bot(chunks, llm):
    return build_bot(config=Config(), llm=llm, chunks=chunks, metrics=InMemoryMetricsSink())


def test_followup_prompt_includes_prior_turns(kb_chunks):
    llm = MockLLMClient()
    bot = _bot(kb_chunks, llm)

    bot.turn("How do I build my own Redis server?")
    bot.turn("And what about the wc challenge?")

    # The prompt for the 2nd turn must include the earlier turns (multi-turn memory).
    second_prompt = llm.calls[-1]
    contents = " ".join(m.content for m in second_prompt.messages)
    assert "Redis" in contents  # the first user question carried forward
    roles = [m.role for m in second_prompt.messages]
    assert roles.count("user") >= 2 and "assistant" in roles


def test_followup_resolves_pronoun_via_history(kb_chunks):
    llm = MockLLMClient()
    bot = _bot(kb_chunks, llm)
    bot.turn("Tell me about the Redis challenge.")
    result = bot.turn("What protocol does it use?")  # "it" -> Redis, resolved via history
    assert isinstance(result, Answer)
    contents = " ".join(m.content for m in llm.calls[-1].messages)
    assert "Redis" in contents


def test_reset_clears_history_but_keeps_stable_id(kb_chunks):
    llm = MockLLMClient()
    bot = _bot(kb_chunks, llm)
    cid = bot.conversation.conversation_id

    bot.turn("How do I build my own Redis server?")
    assert bot.conversation.turns
    assert bot.conversation.conversation_id == cid  # stable within a conversation

    bot.reset()
    assert bot.conversation.turns == []
    assert bot.conversation.conversation_id == cid
