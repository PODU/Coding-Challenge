"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``AnthropicClient`` streaming path (claude-opus-4-8, adaptive thinking)
with a trivial prompt.
"""

import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_complete_round_trip():
    from rtfm.config import Config
    from rtfm.llm import AnthropicClient

    client = AnthropicClient(Config(max_output_tokens=512))
    resp = client.complete(
        system="You are a terse assistant.",
        user="Reply with the single word: pong",
    )
    assert resp.stop_reason != "refusal"
    assert "pong" in resp.text.lower()
    assert resp.output_tokens > 0
