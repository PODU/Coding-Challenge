"""The :class:`LLMClient` protocol and the message and tool types it uses.

``AnthropicClient`` implements it for real; ``MockLLMClient`` for tests and the
offline demo.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Protocol, Sequence


@dataclass(frozen=True)
class Message:
    """``role`` is ``user``, ``assistant`` or ``tool``. Tool calls and their
    results share a ``tool_call_id``."""

    role: str
    content: str
    tool_call_id: str | None = None


@dataclass(frozen=True)
class ToolCall:
    id: str
    name: str
    arguments: dict


@dataclass(frozen=True)
class ToolSpec:
    name: str
    description: str
    parameters: dict


@dataclass(frozen=True)
class Usage:
    input_tokens: int = 0
    output_tokens: int = 0


@dataclass(frozen=True)
class LLMResult:
    """Either final text or tool calls to run before asking again."""

    text: str = ""
    tool_calls: tuple[ToolCall, ...] = field(default_factory=tuple)
    stop_reason: str = "end_turn"
    usage: Usage = field(default_factory=Usage)

    @property
    def wants_tools(self) -> bool:
        return bool(self.tool_calls)


class LLMError(Exception):
    """Base class for client failures."""


class LLMEmptyResponse(LLMError):
    """The model returned no usable content."""


class LLMClient(Protocol):
    def complete(
        self,
        *,
        system: str,
        messages: Sequence[Message],
        tools: Sequence[ToolSpec] = (),
        model: str = "",
        max_tokens: int = 1024,
    ) -> LLMResult: ...
