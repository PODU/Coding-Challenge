"""Optional LLM second opinion, off by default.

The statistical score is always the main result; a judge only adds a note.
:class:`MockJudge` is offline and is what the tests use. :class:`AnthropicJudge`
is the only code that touches the network, and is only built when the user
passes ``--judge=anthropic`` with an API key set.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol, runtime_checkable


@dataclass(frozen=True)
class JudgeOpinion:
    """A judge's second opinion about a text."""

    probability_ai: float  # 0.0 .. 1.0
    rationale: str
    source: str            # "mock" | "anthropic"


@runtime_checkable
class Judge(Protocol):
    def assess(self, text: str) -> JudgeOpinion: ...


class MockJudge:
    """Offline judge for tests and ``--judge=mock``.

    The "probability" is a checksum of the text, so it is reproducible.
    """

    source = "mock"

    def assess(self, text: str) -> JudgeOpinion:
        stripped = text.strip()
        if not stripped:
            return JudgeOpinion(0.0, "empty input", self.source)
        checksum = sum(ord(c) for c in stripped) % 1000
        return JudgeOpinion(
            probability_ai=round(checksum / 1000.0, 3),
            rationale="deterministic mock opinion (no model called)",
            source=self.source,
        )


class AnthropicJudge:
    """Judge backed by the Anthropic API. Never used in tests.

    The ``anthropic`` SDK is imported lazily so it stays an optional extra.
    """

    source = "anthropic"
    MODEL = "claude-opus-4-8"

    def __init__(self, api_key: str | None = None) -> None:  # pragma: no cover
        import os

        try:
            import anthropic
        except ImportError as exc:  # pragma: no cover
            raise RuntimeError(
                "The 'anthropic' extra is required for --judge=anthropic: "
                "pip install 'aidetector[anthropic]'"
            ) from exc

        key = api_key or os.environ.get("ANTHROPIC_API_KEY")
        if not key:
            raise RuntimeError(
                "ANTHROPIC_API_KEY is not set; cannot use --judge=anthropic."
            )
        self._client = anthropic.Anthropic(api_key=key)

    def assess(self, text: str) -> JudgeOpinion:  # pragma: no cover - network
        prompt = (
            "Estimate the probability (0.0-1.0) that the following text was "
            "written by an AI language model. Reply with the number on the first "
            "line and a one-sentence rationale on the second.\n\n"
            f"TEXT:\n{text}"
        )
        message = self._client.messages.create(
            model=self.MODEL,
            max_tokens=200,
            messages=[{"role": "user", "content": prompt}],
        )
        body = "".join(
            block.text for block in message.content if getattr(block, "type", "") == "text"
        ).strip()
        first, _, rest = body.partition("\n")
        try:
            prob = max(0.0, min(1.0, float(first.strip().split()[0])))
        except (ValueError, IndexError):
            prob = 0.5
        return JudgeOpinion(prob, rest.strip() or body, self.source)


def make_judge(kind: str) -> Judge | None:
    """Return the judge for a ``--judge`` value, or None for ``none``."""
    if kind == "none":
        return None
    if kind == "mock":
        return MockJudge()
    if kind == "anthropic":  # pragma: no cover - requires opt-in + network
        return AnthropicJudge()
    raise ValueError(f"unknown judge kind: {kind!r}")
