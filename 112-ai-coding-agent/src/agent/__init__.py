"""A small command-line coding agent: Claude plus file and shell tools.

The model sits behind :class:`agent.llm.LLMClient`, so the tests run offline
against a scripted mock.
"""

from agent.errors import (
    AgentError,
    MaxIterationsError,
    SandboxViolation,
    ToolError,
)
from agent.llm import AnthropicLLMClient, LLMClient, LLMResponse
from agent.loop import Agent
from agent.permissions import PermissionPolicy
from agent.tools import ToolRegistry, default_registry
from agent.workspace import Workspace

__all__ = [
    "Agent",
    "AgentError",
    "AnthropicLLMClient",
    "LLMClient",
    "LLMResponse",
    "MaxIterationsError",
    "PermissionPolicy",
    "SandboxViolation",
    "ToolError",
    "ToolRegistry",
    "Workspace",
    "default_registry",
]

__version__ = "0.1.0"
