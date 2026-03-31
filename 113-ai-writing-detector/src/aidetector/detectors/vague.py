"""Vague language, in three separately capped categories.

  * vague attributions ("studies show", "experts say")
  * superficial analysis ("plays a crucial role", "it is worth noting")
  * overgeneralization ("in today's world", "throughout history")
"""

from __future__ import annotations

import re

from .. import lexicons as lex
from ..tokenize import Document
from .base import Detection, Finding

_CATEGORIES: tuple[tuple[str, str, tuple[str, ...]], ...] = (
    ("vague_attribution", "vague attribution", lex.VAGUE_ATTRIBUTIONS),
    ("superficial_analysis", "superficial analysis", lex.SUPERFICIAL_ANALYSIS),
    ("overgeneralization", "overgeneralization", lex.OVERGENERALIZATION),
)


def _compile(terms: tuple[str, ...]) -> list[tuple[str, re.Pattern[str]]]:
    return [(t, re.compile(rf"(?<!\w){re.escape(t)}", re.IGNORECASE)) for t in terms]


_COMPILED = [
    (pattern, label, _compile(terms)) for pattern, label, terms in _CATEGORIES
]


class VagueLanguageDetector:
    name = "Vague language"
    step = 4
    cap = lex.CAP_VAGUE

    def run(self, doc: Document) -> Detection:
        text = doc.lower
        findings: list[Finding] = []

        for pattern, label, compiled in _COMPILED:
            category_score = 0.0
            seen: set[str] = set()
            for term, rx in compiled:
                if term in seen or not rx.search(text):
                    continue
                seen.add(term)
                # Past the category cap we still list the term, at zero weight.
                weight = lex.WEIGHT_VAGUE_ITEM
                if category_score + weight > lex.CAP_VAGUE_CATEGORY:
                    weight = max(0.0, lex.CAP_VAGUE_CATEGORY - category_score)
                category_score += weight
                findings.append(
                    Finding(
                        pattern=pattern,
                        evidence=term,
                        detail=f'{label}: "{term}"',
                        weight=weight,
                    )
                )

        return Detection.build(self.name, self.step, self.cap, findings)
