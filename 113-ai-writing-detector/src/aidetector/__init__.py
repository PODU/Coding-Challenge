"""Rule-based and statistical detector for LLM-written text.

Scoring is deterministic and offline: the same input always gives the same
score, with no model or network call involved.
"""

from __future__ import annotations

from .scorer import Result, aggregate
from .tokenize import Document, build_document

__all__ = ["Document", "Result", "aggregate", "build_document", "__version__"]

__version__ = "1.0.0"
