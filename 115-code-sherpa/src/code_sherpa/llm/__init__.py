"""LLM clients behind a single interface."""

from __future__ import annotations

from .base import LLMClient, LLMError, LLMResponse, Message
from .mock import MockLLMClient

__all__ = [
    "LLMClient",
    "LLMResponse",
    "Message",
    "LLMError",
    "MockLLMClient",
    "build_llm",
]


def build_llm(config) -> LLMClient:
    if config.llm == "mock":
        return MockLLMClient()
    if config.llm == "anthropic":
        from .anthropic_client import AnthropicClient

        return AnthropicClient(
            api_key=config.anthropic_api_key,
            model=config.model,
            timeout=config.llm_timeout,
            max_retries=config.llm_max_retries,
        )
    raise ValueError(f"Unknown llm: {config.llm!r}")
