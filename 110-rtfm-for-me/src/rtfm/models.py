"""Dataclasses shared across the package."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

# Plain lists so callers don't need numpy; the in-memory store converts
# internally.
Vector = list[float]


@dataclass(frozen=True)
class Chunk:
    """A slice of a source document."""

    filename: str
    section: str
    position: int  # index of the chunk within its file
    text: str
    chunk_id: str = ""
    embedding: Optional[Vector] = None

    def with_embedding(self, embedding: Vector) -> "Chunk":
        return Chunk(
            filename=self.filename,
            section=self.section,
            position=self.position,
            text=self.text,
            chunk_id=self.chunk_id or f"{self.filename}#{self.position}",
            embedding=list(embedding),
        )


@dataclass(frozen=True)
class RetrievedChunk:
    chunk: Chunk
    score: float


@dataclass(frozen=True)
class Citation:
    filename: str
    section: str
    position: int

    @property
    def label(self) -> str:
        if self.section:
            return f"{self.filename} ({self.section})"
        return self.filename


@dataclass
class Answer:
    text: str
    citations: list[Citation] = field(default_factory=list)
    found: bool = True  # False when the docs don't cover the question
    refused: bool = False  # the model declined to answer


@dataclass(frozen=True)
class Turn:
    question: str
    answer: str


@dataclass
class CacheStats:
    hits: int = 0
    misses: int = 0
    # Rough estimates, for reporting only.
    latency_saved_ms: float = 0.0
    cost_saved_usd: float = 0.0

    @property
    def total(self) -> int:
        return self.hits + self.misses

    @property
    def hit_rate(self) -> float:
        return self.hits / self.total if self.total else 0.0


@dataclass
class AskResult:
    answer: Answer
    cached: bool = False
    degraded: list[str] = field(default_factory=list)  # components that failed and were skipped
    retrieved: list[RetrievedChunk] = field(default_factory=list)


@dataclass(frozen=True)
class LLMResponse:
    text: str
    stop_reason: str
    input_tokens: int = 0
    output_tokens: int = 0
