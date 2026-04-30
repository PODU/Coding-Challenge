"""Tries each client in turn until one succeeds."""

from __future__ import annotations

from ..models import LLMResponse, Prompt
from .base import AllProvidersFailed, LLMClient


class FallbackLLMClient:
    name = "fallback"

    def __init__(self, clients: list[LLMClient]) -> None:
        if not clients:
            raise ValueError("FallbackLLMClient requires at least one client")
        self.clients = list(clients)

    def complete(self, prompt: Prompt, *, model: str | None = None) -> LLMResponse:
        last_error: Exception | None = None
        for index, client in enumerate(self.clients):
            try:
                resp = client.complete(prompt, model=model)
            except Exception as exc:  # noqa: BLE001
                last_error = exc
                continue
            resp.fallback_used = index > 0
            resp.attempts = index + 1
            return resp
        raise AllProvidersFailed(
            f"all {len(self.clients)} provider(s) failed; last error: {last_error}"
        ) from last_error
