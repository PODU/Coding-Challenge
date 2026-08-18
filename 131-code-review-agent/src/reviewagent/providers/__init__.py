"""Model providers, all with ``complete(system, prompt) -> Completion``.

Which one is used is set by REVIEW_PROVIDER, REVIEW_MODEL and REVIEW_API_KEY.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol


@dataclass
class Completion:
    text: str
    input_tokens: int = 0
    output_tokens: int = 0
    model: str = ""


class Provider(Protocol):
    def complete(self, system: str, prompt: str) -> Completion: ...


def make_provider(
    name: str,
    *,
    api_key: str = "",
    model: str = "",
    base_url: str = "",
    timeout: float = 120.0,
    transport=None,
) -> Provider:
    name = (name or "mock").lower()
    kwargs = dict(api_key=api_key, model=model, timeout=timeout)
    if transport is not None:
        kwargs["transport"] = transport
    if name == "anthropic":
        from .anthropic import AnthropicProvider

        return AnthropicProvider(**kwargs)
    if name == "openai":
        from .openai import OpenAIProvider

        if base_url:
            kwargs["base_url"] = base_url
        return OpenAIProvider(**kwargs)
    if name == "ollama":
        from .ollama import OllamaProvider

        kwargs.pop("api_key")
        if base_url:
            kwargs["base_url"] = base_url
        return OllamaProvider(**kwargs)
    if name == "mock":
        from .mock import MockProvider

        return MockProvider()
    raise ValueError(f"unknown provider '{name}' (anthropic|openai|ollama|mock)")
