"""Opt-in live smoke test against the real Anthropic API.

Skipped unless RUN_LIVE=1 and ANTHROPIC_API_KEY are both set. Exercises the
real ``AnthropicLLM`` path (claude-opus-4-8, adaptive thinking) and its JSON
envelope extraction with a trivial prompt.
"""

import json
import os

import pytest

pytestmark = pytest.mark.skipif(
    os.environ.get("RUN_LIVE") != "1" or not os.environ.get("ANTHROPIC_API_KEY"),
    reason="live smoke test: set RUN_LIVE=1 and ANTHROPIC_API_KEY to run",
)


def test_live_complete_round_trip():
    from llmwiki.interfaces import Msg
    from llmwiki.llm import AnthropicLLM

    client = AnthropicLLM(max_tokens=1024)
    result = client.complete(
        'TASK: echo\nReturn a JSON object of the form {"answer": "<word>"}.',
        [Msg(role="user", content='Echo back exactly: {"answer": "pong"}')],
    )
    data = json.loads(result.text)
    assert data["answer"].lower() == "pong"
