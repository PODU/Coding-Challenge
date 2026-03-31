"""Types shared by the detectors."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Protocol, runtime_checkable

from ..tokenize import Document


@dataclass(frozen=True)
class Finding:
    """A single piece of evidence emitted by a detector."""

    pattern: str   # machine key, e.g. "signature_word"
    evidence: str  # the matched span, e.g. "delve into"
    detail: str    # human-readable explanation
    weight: float  # points this finding contributes (pre-cap)


@dataclass
class Detection:
    """The result of running one detector over a document."""

    name: str
    step: int
    raw_score: float
    cap: float
    capped_score: float
    findings: list[Finding] = field(default_factory=list)

    @classmethod
    def build(
        cls,
        name: str,
        step: int,
        cap: float,
        findings: list[Finding],
    ) -> "Detection":
        raw = round(sum(f.weight for f in findings), 4)
        return cls(
            name=name,
            step=step,
            raw_score=raw,
            cap=cap,
            capped_score=round(min(raw, cap), 4),
            findings=findings,
        )


@runtime_checkable
class Detector(Protocol):
    """Detectors are stateless and apply their own cap."""

    name: str
    step: int
    cap: float

    def run(self, doc: Document) -> Detection: ...
