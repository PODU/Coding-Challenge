"""Per-turn metrics and cost estimates. Every turn emits one ``TurnLog``."""

from __future__ import annotations

import json
import sys
from collections import Counter
from dataclasses import asdict, dataclass
from typing import Protocol, TextIO, runtime_checkable

from .config import PRICES


def estimate_cost_usd(model: str, input_tokens: int, output_tokens: int) -> float:
    price_in, price_out = PRICES.get(model, (5.0, 25.0))
    return (input_tokens / 1_000_000) * price_in + (output_tokens / 1_000_000) * price_out


@dataclass
class TurnLog:
    request_id: str
    timestamp: str
    provider: str
    model: str
    input_tokens: int
    output_tokens: int
    latency_ms: float
    estimated_cost_usd: float
    fallback_used: bool
    finish_reason: str
    escalated: bool
    escalation_reason: str | None
    classification: str | None


@runtime_checkable
class MetricsSink(Protocol):
    def emit(self, log: TurnLog) -> None: ...


class JsonlMetricsSink:
    """Writes each log as a JSON line, to stderr by default."""

    def __init__(self, stream: TextIO | None = None) -> None:
        self.stream = stream or sys.stderr

    def emit(self, log: TurnLog) -> None:
        self.stream.write(json.dumps(asdict(log)) + "\n")
        self.stream.flush()


class InMemoryMetricsSink:
    """Keeps logs in memory; backs the /stats command and the tests."""

    def __init__(self) -> None:
        self.logs: list[TurnLog] = []

    def emit(self, log: TurnLog) -> None:
        self.logs.append(log)

    def stats(self) -> dict:
        n = len(self.logs)
        escalated = sum(1 for x in self.logs if x.escalated)
        return {
            "turns": n,
            "answers": n - escalated,
            "escalations": escalated,
            "escalation_rate": (escalated / n) if n else 0.0,
            "fallback_turns": sum(1 for x in self.logs if x.fallback_used),
            "total_input_tokens": sum(x.input_tokens for x in self.logs),
            "total_output_tokens": sum(x.output_tokens for x in self.logs),
            "total_cost_usd": round(sum(x.estimated_cost_usd for x in self.logs), 6),
            "avg_latency_ms": (sum(x.latency_ms for x in self.logs) / n) if n else 0.0,
            "by_model": dict(Counter(x.model for x in self.logs)),
            "by_reason": dict(
                Counter(x.escalation_reason for x in self.logs if x.escalation_reason)
            ),
            "by_classification": dict(
                Counter(x.classification for x in self.logs if x.classification)
            ),
        }
