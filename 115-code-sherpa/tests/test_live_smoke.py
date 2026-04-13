"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``AnthropicClient`` path (claude-opus-4-8) with a trivial prompt.
"""

import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_complete_round_trip():
    from code_sherpa.llm.anthropic_client import AnthropicClient
    from code_sherpa.llm.base import Message

    client = AnthropicClient(api_key=os.environ["ANTHROPIC_API_KEY"])
    resp = client.complete(
        [
            Message(role="system", content="You are a terse assistant."),
            Message(role="user", content="Reply with the single word: pong"),
        ],
        max_tokens=64,
    )
    assert "pong" in resp.text.lower()
    assert resp.stop_reason == "end_turn"
