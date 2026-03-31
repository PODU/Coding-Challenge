"""The detectors, in the order they are run and reported."""

from __future__ import annotations

from .base import Detection, Detector, Finding
from .emphasis import EmphasisDetector
from .linguistic import LinguisticDetector
from .signature import SignatureDetector
from .structural import StructuralDetector
from .vague import VagueLanguageDetector

DETECTORS: tuple[Detector, ...] = (
    SignatureDetector(),
    StructuralDetector(),
    VagueLanguageDetector(),
    EmphasisDetector(),
    LinguisticDetector(),
)

__all__ = [
    "DETECTORS",
    "Detection",
    "Detector",
    "Finding",
    "SignatureDetector",
    "StructuralDetector",
    "VagueLanguageDetector",
    "EmphasisDetector",
    "LinguisticDetector",
]
