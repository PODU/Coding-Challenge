"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``AnthropicJudge`` path (claude-opus-4-8) with a short sample text.
"""

import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_assess_round_trip():
    from aidetector.judge import AnthropicJudge

    opinion = AnthropicJudge().assess(
        "The quick brown fox jumps over the lazy dog near the riverbank."
    )
    assert opinion.source == "anthropic"
    assert 0.0 <= opinion.probability_ai <= 1.0
    assert opinion.rationale
