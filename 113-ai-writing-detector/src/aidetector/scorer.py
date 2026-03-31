"""Combine detector scores into a 0-100 score and a label."""

from __future__ import annotations

from dataclasses import dataclass, field

from .detectors import DETECTORS, Detection
from .tokenize import Document

DEFAULT_THRESHOLD_HUMAN = 30
DEFAULT_THRESHOLD_AI = 60

LABEL_HUMAN = "likely human"
LABEL_POSSIBLE = "possibly AI"
LABEL_AI = "likely AI"


@dataclass
class Result:
    score: int
    label: str
    document: Document
    detections: list[Detection] = field(default_factory=list)
    threshold_human: int = DEFAULT_THRESHOLD_HUMAN
    threshold_ai: int = DEFAULT_THRESHOLD_AI

    @property
    def stats(self) -> dict[str, int]:
        return {
            "characters": self.document.char_count,
            "words": self.document.word_count,
            "sentences": self.document.sentence_count,
            "syllables": self.document.syllables,
        }


def classify(score: float, threshold_human: int, threshold_ai: int) -> str:
    """Each threshold is the lowest score that gets the next label up."""
    if score < threshold_human:
        return LABEL_HUMAN
    if score < threshold_ai:
        return LABEL_POSSIBLE
    return LABEL_AI


def aggregate(
    doc: Document,
    *,
    threshold_human: int = DEFAULT_THRESHOLD_HUMAN,
    threshold_ai: int = DEFAULT_THRESHOLD_AI,
) -> Result:
    detections = [d.run(doc) for d in DETECTORS]
    total = sum(d.capped_score for d in detections)
    score = int(round(max(0.0, min(100.0, total))))
    return Result(
        score=score,
        label=classify(score, threshold_human, threshold_ai),
        document=doc,
        detections=detections,
        threshold_human=threshold_human,
        threshold_ai=threshold_ai,
    )
