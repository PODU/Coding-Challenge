"""Shared test fixtures: a temp workspace and a scripted offline LLM mock.

Principle: every test runs offline. ``MockLLMClient`` implements the ``LLMClient``
protocol and returns pre-scripted ``LLMResponse`` objects. No network, no API key,
no real Anthropic call anywhere in the suite.
"""

from __future__ import annotations

import itertools
from typing import Any

import pytest

from agent.llm import LLMResponse
from agent.permissions import PermissionPolicy
from agent.tools import ToolRegistry, default_registry
from agent.workspace import Workspace

_ids = itertools.count()


def tool_use_block(name: str, tool_input: dict[str, Any], block_id: str | None = None):
    """Build a ``tool_use`` content block as the SDK would emit it (plain dict)."""
    return {
        "type": "tool_use",
        "id": block_id or f"toolu_{next(_ids)}",
        "name": name,
        "input": tool_input,
    }


def text_block(text: str) -> dict[str, Any]:
    return {"type": "text", "text": text}


def tool_use_response(*blocks: dict[str, Any]) -> LLMResponse:
    return LLMResponse(stop_reason="tool_use", content=list(blocks))


def end_turn_response(text: str) -> LLMResponse:
    return LLMResponse(stop_reason="end_turn", content=[text_block(text)])


class MockLLMClient:
    """Returns pre-scripted responses; records the calls it received."""

    def __init__(self, scripted: list[LLMResponse]):
        self._queue = list(scripted)
        self.calls: list[dict[str, Any]] = []

    def complete(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]],
        system: str,
    ) -> LLMResponse:
        # Record a shallow copy of messages so later mutation does not rewrite history.
        self.calls.append(
            {"messages": list(messages), "tools": tools, "system": system}
        )
        if not self._queue:
            raise AssertionError("MockLLMClient ran out of scripted responses")
        return self._queue.pop(0)


@pytest.fixture
def workspace(tmp_path) -> Workspace:
    """A fresh temp-directory workspace per test."""
    return Workspace(tmp_path)


@pytest.fixture
def allow_registry() -> ToolRegistry:
    """Registry whose run_command is in 'allow' mode (no interactive prompt)."""
    return default_registry(PermissionPolicy(mode="allow"))


@pytest.fixture
def deny_registry() -> ToolRegistry:
    """Registry whose run_command is in 'deny' mode -- the safe default for tests."""
    return default_registry(PermissionPolicy(mode="deny"))
