"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``AnthropicLLMClient`` path (claude-opus-4-8, adaptive thinking) with a
trivial prompt and no tools.
"""

import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_complete_round_trip():
    from agent.llm import AnthropicLLMClient

    client = AnthropicLLMClient(max_tokens=512)
    resp = client.complete(
        messages=[{"role": "user", "content": "Reply with the single word: pong"}],
        tools=[],
        system="You are a terse assistant.",
    )
    assert resp.stop_reason == "end_turn"
    text = "".join(b["text"] for b in resp.content if b.get("type") == "text")
    assert "pong" in text.lower()
