"""LLM clients: ``AnthropicClient`` for real use and ``MockLLMClient`` for tests."""

from __future__ import annotations

from typing import Callable, Protocol, runtime_checkable

from .config import Config
from .models import LLMResponse


@runtime_checkable
class LLMClient(Protocol):
    def complete(self, *, system: str, user: str) -> LLMResponse: ...


class MockLLMClient:
    """Returns scripted replies and records every call.

    ``scripted`` is a string or a ``(system, user) -> str`` callable. Use
    ``stop_reason="refusal"`` to simulate the model declining.
    """

    def __init__(
        self,
        scripted: str | Callable[[str, str], str] = "",
        *,
        stop_reason: str = "end_turn",
    ) -> None:
        self.calls: list[tuple[str, str]] = []
        self._scripted = scripted
        self._stop_reason = stop_reason

    def complete(self, *, system: str, user: str) -> LLMResponse:
        self.calls.append((system, user))
        if self._stop_reason == "refusal":
            return LLMResponse(text="", stop_reason="refusal")
        text = self._scripted(system, user) if callable(self._scripted) else self._scripted
        return LLMResponse(
            text=text,
            stop_reason=self._stop_reason,
            input_tokens=len(user.split()),
            output_tokens=len(text.split()),
        )


class AnthropicClient:
    """Client for the Anthropic API. SDK errors are left to propagate."""

    def __init__(self, cfg: Config, *, client=None) -> None:
        self._cfg = cfg
        if client is not None:
            self._client = client
        else:  # pragma: no cover - requires the anthropic SDK + a key
            try:
                import anthropic  # type: ignore
            except Exception as exc:
                raise RuntimeError(
                    "AnthropicClient requires the optional 'anthropic' dependency. "
                    "Install it with: pip install 'rtfm[anthropic]'."
                ) from exc
            # The SDK picks up ANTHROPIC_API_KEY itself.
            self._client = anthropic.Anthropic()

    def complete(self, *, system: str, user: str) -> LLMResponse:  # pragma: no cover - network
        # Streaming avoids HTTP timeouts on long answers. This model only takes
        # adaptive thinking; budget_tokens and temperature are rejected.
        with self._client.messages.stream(
            model=self._cfg.model,
            max_tokens=self._cfg.max_output_tokens,
            thinking={"type": "adaptive"},
            system=system,
            messages=[{"role": "user", "content": user}],
        ) as stream:
            msg = stream.get_final_message()

        usage = getattr(msg, "usage", None)
        in_tok = int(getattr(usage, "input_tokens", 0) or 0)
        out_tok = int(getattr(usage, "output_tokens", 0) or 0)

        if msg.stop_reason == "refusal":
            return LLMResponse(text="", stop_reason="refusal", input_tokens=in_tok, output_tokens=0)

        text = next((b.text for b in msg.content if getattr(b, "type", None) == "text"), "")
        return LLMResponse(
            text=text,
            stop_reason=msg.stop_reason or "end_turn",
            input_tokens=in_tok,
            output_tokens=out_tok,
        )
