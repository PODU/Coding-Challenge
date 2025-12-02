"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``AnthropicLLMClient`` path (claude-opus-4-8) with a trivial prompt.
"""

import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_complete_round_trip():
    from chatbot.anthropic_client import AnthropicLLMClient
    from chatbot.config import Config
    from chatbot.llm import Message

    cfg = Config(api_key=os.environ["ANTHROPIC_API_KEY"], max_tokens=64)
    client = AnthropicLLMClient(cfg)
    result = client.complete(
        system="You are a terse assistant.",
        messages=[Message(role="user", content="Reply with the single word: pong")],
        max_tokens=64,
    )
    assert "pong" in result.text.lower()
    assert result.stop_reason == "end_turn"
