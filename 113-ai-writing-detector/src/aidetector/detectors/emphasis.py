"""Excessive emphasis.

Three separately capped categories:
  * undue emphasis: ALL-CAPS words, piled-up intensifiers, '!' runs, bold
  * promotional language: "game-changer", "cutting-edge", ...
  * elegant variation: cycling through synonyms for the same thing
"""

from __future__ import annotations

import re

from .. import lexicons as lex
from ..tokenize import Document
from .base import Detection, Finding

_ALLCAPS_RE = re.compile(r"\b[A-Z][A-Z]{2,}\b")
_BOLD_RE = re.compile(r"\*\*[^*]+\*\*|__[^_]+__")
_BANG_RE = re.compile(r"!")
_PROMO_RES = [
    (t, re.compile(rf"(?<!\w){re.escape(t)}(?!\w)", re.IGNORECASE))
    for t in lex.PROMOTIONAL_TERMS
]
_INTENSIFIER_RES = [
    (t, re.compile(rf"(?<!\w){re.escape(t)}(?!\w)", re.IGNORECASE))
    for t in lex.INTENSIFIERS
]


def _capped(score: float, weight: float, cap: float) -> float:
    if score + weight > cap:
        return max(0.0, cap - score)
    return weight


class EmphasisDetector:
    name = "Excessive emphasis"
    step = 5
    cap = lex.CAP_EMPHASIS

    def run(self, doc: Document) -> Detection:
        findings = self._undue(doc) + self._promotional(doc) + self._elegant_variation(doc)
        return Detection.build(self.name, self.step, self.cap, findings)

    def _undue(self, doc: Document) -> list[Finding]:
        out: list[Finding] = []
        score = 0.0
        cap = lex.CAP_EMPHASIS_CATEGORY

        intensifier_hits = sum(
            len(rx.findall(doc.text)) for _, rx in _INTENSIFIER_RES
        )
        # One intensifier is normal; it's the pile-up that's a tell.
        excess = max(0, intensifier_hits - 1)
        if excess:
            w = _capped(score, min(excess, 3) * lex.WEIGHT_UNDUE_EMPHASIS, cap)
            score += w
            out.append(
                Finding(
                    "undue_emphasis",
                    f"{intensifier_hits} intensifiers",
                    "intensifier pile-up (very/really/extremely ...)",
                    w,
                )
            )

        caps = _ALLCAPS_RE.findall(doc.raw)
        if caps:
            w = _capped(score, min(len(caps), 2) * lex.WEIGHT_UNDUE_EMPHASIS, cap)
            score += w
            out.append(
                Finding(
                    "undue_emphasis",
                    ", ".join(caps[:3]),
                    "ALL-CAPS emphasis",
                    w,
                )
            )

        bold = _BOLD_RE.findall(doc.raw)
        bangs = _BANG_RE.findall(doc.raw)
        markers = len(bold) + max(0, len(bangs) - 1)
        if markers:
            w = _capped(score, min(markers, 2) * lex.WEIGHT_UNDUE_EMPHASIS, cap)
            score += w
            out.append(
                Finding(
                    "undue_emphasis",
                    f"{len(bold)} bold / {len(bangs)} '!'",
                    "typographic emphasis (bold / exclamation)",
                    w,
                )
            )
        return out

    def _promotional(self, doc: Document) -> list[Finding]:
        out: list[Finding] = []
        score = 0.0
        cap = lex.CAP_EMPHASIS_CATEGORY
        seen: set[str] = set()
        text = doc.lower
        for term, rx in _PROMO_RES:
            if term in seen or not rx.search(text):
                continue
            seen.add(term)
            w = _capped(score, lex.WEIGHT_PROMOTIONAL, cap)
            score += w
            out.append(
                Finding(
                    "promotional",
                    term,
                    f'promotional language: "{term}"',
                    w,
                )
            )
            if score >= cap:
                break
        return out

    def _elegant_variation(self, doc: Document) -> list[Finding]:
        out: list[Finding] = []
        score = 0.0
        cap = lex.CAP_EMPHASIS_CATEGORY
        wordset = {w.lower() for w in doc.words}
        for group in lex.ELEGANT_VARIATION_GROUPS:
            present = [g for g in group if g in wordset]
            if len(present) >= 2:
                w = _capped(score, lex.WEIGHT_ELEGANT_VARIATION, cap)
                score += w
                out.append(
                    Finding(
                        "elegant_variation",
                        ", ".join(present),
                        "elegant variation (synonym cycling)",
                        w,
                    )
                )
                if score >= cap:
                    break
        return out
