"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``ClaudeClient`` path (claude-opus-4-8, adaptive thinking) with a trivial
prompt.
"""

import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_complete_round_trip():
    from contract_review.llm import ClaudeClient

    client = ClaudeClient()
    text = client.complete(
        "You are a terse assistant.",
        [{"role": "user", "content": "Reply with the single word: pong"}],
        max_tokens=512,
    )
    assert "pong" in text.lower()
