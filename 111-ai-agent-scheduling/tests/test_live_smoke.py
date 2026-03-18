"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``AnthropicLLMClient`` streaming path (claude-opus-4-8, adaptive thinking)
with a trivial prompt.
"""

import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_complete_round_trip():
    from ai_agent_scheduling.llm.client import AnthropicLLMClient

    client = AnthropicLLMClient(max_tokens=512)
    result = client.complete(
        system="You are a terse assistant.",
        task="Reply with the single word: pong",
        timeout_s=120.0,
    )
    assert "pong" in result.text.lower()
    assert result.output_tokens > 0
