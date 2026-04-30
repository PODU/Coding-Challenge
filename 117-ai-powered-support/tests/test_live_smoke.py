"""Opt-in live smoke test against the real claude-opus-4-8.

Skipped by default. Runs only when both ANTHROPIC_API_KEY is set and RUN_LIVE=1,
so CI and the normal offline suite never touch the network.
"""

from __future__ import annotations

import os

import pytest

pytestmark = pytest.mark.skipif(
    not (os.environ.get("ANTHROPIC_API_KEY") and os.environ.get("RUN_LIVE") == "1"),
    reason="live test requires ANTHROPIC_API_KEY and RUN_LIVE=1",
)


def test_live_grounded_answer():
    from support_bot.config import Config
    from support_bot.factory import build_bot
    from support_bot.models import Answer, Escalation

    bot = build_bot(config=Config())
    result = bot.turn("How do I build my own Redis server on the platform?")
    assert isinstance(result, (Answer, Escalation))
    if isinstance(result, Answer):
        assert result.text.strip()
        assert "opus" in result.model or result.model.startswith("claude")
