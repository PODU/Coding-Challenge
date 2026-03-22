"""The agent loop.

The Messages API is stateless, so we resend the whole history each turn. The
API is strict about its shape: the assistant turn goes in unchanged, then one
user message holding a tool_result for every tool_use in it.
"""

from __future__ import annotations

from typing import Any

from agent.errors import MaxIterationsError
from agent.llm import LLMClient
from agent.tools import ToolRegistry
from agent.workspace import Workspace

DEFAULT_SYSTEM_PROMPT = (
    "You are a command-line coding agent operating inside a sandboxed workspace "
    "directory. Use the provided tools to read, write, and list files and to run "
    "commands. Read a file before editing it. All paths are relative to the "
    "workspace root; you cannot access anything outside it. When the task is "
    "complete, stop calling tools and give a short summary of what you did."
)


class Agent:
    """Drives an LLM through tool-use until it finishes or hits a safety cap."""

    def __init__(
        self,
        llm: LLMClient,
        registry: ToolRegistry,
        workspace: Workspace,
        system: str = DEFAULT_SYSTEM_PROMPT,
        max_iterations: int = 25,
    ) -> None:
        if max_iterations < 1:
            raise ValueError("max_iterations must be >= 1")
        self.llm = llm
        self.registry = registry
        self.workspace = workspace
        self.system = system
        self.max_iterations = max_iterations

    def run(self, task: str) -> str:
        """Run the loop for ``task`` and return the model's final text.

        Raises :class:`MaxIterationsError` if the model never stops calling tools.
        """
        messages: list[dict[str, Any]] = [{"role": "user", "content": task}]

        for _ in range(self.max_iterations):
            resp = self.llm.complete(messages, self.registry.schemas(), self.system)

            messages.append({"role": "assistant", "content": resp.content})

            if resp.stop_reason != "tool_use":
                return _final_text(resp.content)

            tool_results: list[dict[str, Any]] = []
            for block in resp.content:
                if not isinstance(block, dict) or block.get("type") != "tool_use":
                    continue
                result, is_error = self.registry.dispatch(
                    block.get("name", ""),
                    block.get("input", {}) or {},
                    self.workspace,
                )
                tool_results.append(
                    {
                        "type": "tool_result",
                        "tool_use_id": block.get("id"),
                        "content": result,
                        "is_error": is_error,
                    }
                )

            messages.append({"role": "user", "content": tool_results})

        raise MaxIterationsError(self.max_iterations)


def _final_text(content: list[dict[str, Any]]) -> str:
    """Concatenate the text blocks of a terminal assistant turn."""
    parts = [
        block.get("text", "")
        for block in content
        if isinstance(block, dict) and block.get("type") == "text"
    ]
    return "\n".join(p for p in parts if p).strip()
