"""Tools the agent can call, by name."""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Callable

from ..llm import ToolSpec


@dataclass(frozen=True)
class Tool:
    spec: ToolSpec
    fn: Callable[..., object]


class ToolError(Exception):
    """Raised by tools; the agent reports it to the model as the tool result."""


class ToolRegistry:
    def __init__(self) -> None:
        self._tools: dict[str, Tool] = {}

    def register(self, spec: ToolSpec, fn: Callable[..., object]) -> None:
        self._tools[spec.name] = Tool(spec, fn)

    @property
    def specs(self) -> list[ToolSpec]:
        return [t.spec for t in self._tools.values()]

    def names(self) -> list[str]:
        return list(self._tools)

    def call(self, name: str, arguments: dict) -> str:
        """Run a tool and return its result as text for the model. Errors are
        returned as JSON too, so the model can see what went wrong."""
        tool = self._tools.get(name)
        if tool is None:
            return json.dumps({"error": f"unknown tool: {name}"})
        try:
            result = tool.fn(**arguments)
        except ToolError as exc:
            return json.dumps({"error": str(exc)})
        except TypeError as exc:  # the model passed the wrong arguments
            return json.dumps({"error": f"bad arguments for {name}: {exc}"})
        if isinstance(result, str):
            return result
        return json.dumps(result, default=str)
