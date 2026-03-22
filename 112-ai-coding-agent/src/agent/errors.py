"""Agent exceptions.

``ToolError`` (and its subclass ``SandboxViolation``) is sent back to the model
as an error tool_result so it can try something else. ``MaxIterationsError``
ends the run.
"""

from __future__ import annotations


class AgentError(Exception):
    """Base class for all agent errors."""


class ToolError(AgentError):
    """A tool failed in a way the model can recover from (bad input, missing file)."""


class SandboxViolation(ToolError):
    """A model-supplied path resolved outside the workspace."""


class MaxIterationsError(AgentError):
    """The agentic loop hit its iteration cap without the model finishing."""

    def __init__(self, max_iterations: int) -> None:
        self.max_iterations = max_iterations
        super().__init__(
            f"agent exceeded the maximum of {max_iterations} iterations "
            "without producing a final answer"
        )
