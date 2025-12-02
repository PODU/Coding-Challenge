"""A terminal chatbot built on Anthropic Claude.

All model access goes through the :class:`LLMClient` protocol, so the app can
run offline against a mock.
"""

from __future__ import annotations

from .config import Config
from .history import ConversationHistory
from .llm import (
    LLMAuthError,
    LLMClient,
    LLMEmptyResponse,
    LLMError,
    LLMRateLimitError,
    LLMResult,
    LLMServiceError,
    Message,
)
from .mock_client import MockLLMClient
from .persona import BUILT_IN, Persona
from .session import ChatSession

__all__ = [
    "Config",
    "ConversationHistory",
    "LLMClient",
    "LLMError",
    "LLMAuthError",
    "LLMRateLimitError",
    "LLMServiceError",
    "LLMEmptyResponse",
    "LLMResult",
    "Message",
    "MockLLMClient",
    "Persona",
    "BUILT_IN",
    "ChatSession",
]

__version__ = "1.0.0"
