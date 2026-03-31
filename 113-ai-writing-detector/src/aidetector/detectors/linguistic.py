"""Seven statistical linguistic factors.

Each factor turns a measurement into points with a piecewise-linear curve and
reports the measured value as a finding, so the report can show it. Short
documents give noisy numbers, so their points are scaled down and a
"low confidence" note is added.
"""

from __future__ import annotations

import re
import statistics

from .. import lexicons as lex
from ..tokenize import Document
from .base import Detection, Finding

def _up(x: float, zero_at: float, full_at: float, max_points: float) -> float:
    """0 points at ``zero_at`` and below, ``max_points`` at ``full_at`` and above."""
    if x <= zero_at:
        return 0.0
    if x >= full_at:
        return max_points
    return (x - zero_at) / (full_at - zero_at) * max_points


def _down(x: float, full_at: float, zero_at: float, max_points: float) -> float:
    """``max_points`` at ``full_at`` and below, 0 at ``zero_at`` and above."""
    if x <= full_at:
        return max_points
    if x >= zero_at:
        return 0.0
    return (zero_at - x) / (zero_at - full_at) * max_points


def _plateau(
    x: float,
    rise_zero: float,
    lo: float,
    hi: float,
    fall_zero: float,
    max_points: float,
) -> float:
    """Full points on [lo, hi], ramping linearly down to 0 on either side."""
    if lo <= x <= hi:
        return max_points
    if x < lo:
        return _up(x, rise_zero, lo, max_points)
    return _down(x, hi, fall_zero, max_points)


_BE = r"(?:is|are|was|were|be|been|being|gets?|got)"
_PARTICIPLE = (
    r"(?:\w+ed|written|made|done|given|taken|seen|known|shown|held|built|found|"
    r"used|based|sent|kept|told|brought|driven|grown|drawn|chosen|considered|"
    r"thrown|broken|eaten|spoken|stolen|frozen|hidden|beaten|ridden|forgotten|"
    r"begun|sung|won|lost|put|set|cut|hurt|read|left|paid|met|sold)"
)
_PASSIVE_RE = re.compile(rf"\b{_BE}\s+(?:\w+ly\s+)?{_PARTICIPLE}\b", re.IGNORECASE)

_TRANSITION_RES = [
    re.compile(rf"(?<!\w){re.escape(t)}(?!\w)", re.IGNORECASE)
    for t in lex.TRANSITION_WORDS
]
_OXFORD_RE = re.compile(r",\s+(?:and|or)\b", re.IGNORECASE)


class LinguisticDetector:
    name = "Linguistic factors"
    step = 6
    cap = lex.CAP_LINGUISTIC

    def run(self, doc: Document) -> Detection:
        findings: list[Finding] = []
        weight = lex.SHORT_TEXT_WEIGHT if doc.is_short else 1.0

        if doc.is_short:
            findings.append(
                Finding(
                    "low_confidence",
                    "short text",
                    "low confidence: too little text for reliable statistics "
                    f"({doc.word_count} words, {doc.sentence_count} sentences); "
                    "Step-6 factors down-weighted",
                    0.0,
                )
            )

        for factor in (
            self._lexical_diversity,
            self._sentence_variation,
            self._passive_voice,
            self._transition_density,
            self._flesch_kincaid,
            self._punctuation,
            self._rare_words,
        ):
            pattern, evidence, detail, points = factor(doc)
            findings.append(
                Finding(pattern, evidence, detail, round(points * weight, 4))
            )

        return Detection.build(self.name, self.step, self.cap, findings)

    def _lexical_diversity(self, doc: Document) -> tuple[str, str, str, float]:
        tokens = [w.lower() for w in doc.words]
        n = len(tokens)
        ttr = len(set(tokens)) / n if n else 0.0
        # LLM prose clusters in a mid band; very high (terse human) or very low
        # (repetitive) TTR scores fewer points.
        points = _plateau(ttr, 0.20, 0.35, 0.58, 0.72, lex.MAX_LEXICAL)
        return (
            "lexical_diversity",
            f"TTR={ttr:.2f}",
            f"lexical diversity (type/token ratio) = {ttr:.2f}",
            points,
        )

    def _sentence_variation(self, doc: Document) -> tuple[str, str, str, float]:
        lengths = [len(lex_split(s)) for s in doc.sentences]
        if len(lengths) < 2:
            cv = 1.0
        else:
            mean = statistics.fmean(lengths)
            cv = (statistics.pstdev(lengths) / mean) if mean else 0.0
        # Low variation (uniform sentence lengths) is AI-leaning.
        points = _down(cv, 0.25, 0.60, lex.MAX_SENT_VARIATION)
        return (
            "sentence_variation",
            f"cv={cv:.2f}",
            f"sentence-length variation (cv) = {cv:.2f} "
            f"({'low -> AI-leaning' if cv < 0.4 else 'human-like'})",
            points,
        )

    def _passive_voice(self, doc: Document) -> tuple[str, str, str, float]:
        sents = doc.sentences or [doc.text]
        passive = sum(1 for s in sents if _PASSIVE_RE.search(s))
        density = passive / len(sents)
        points = _up(density, 0.0, 0.40, lex.MAX_PASSIVE)
        return (
            "passive_voice",
            f"{passive}/{len(sents)} sentences",
            f"passive-voice frequency = {density:.0%} of sentences",
            points,
        )

    def _transition_density(self, doc: Document) -> tuple[str, str, str, float]:
        hits = sum(len(rx.findall(doc.text)) for rx in _TRANSITION_RES)
        per100 = (hits / doc.word_count * 100) if doc.word_count else 0.0
        points = _up(per100, 0.3, 3.0, lex.MAX_TRANSITION)
        return (
            "transition_density",
            f"{per100:.1f}/100 words",
            f"transition-word density = {per100:.1f} per 100 words ({hits} total)",
            points,
        )

    def _flesch_kincaid(self, doc: Document) -> tuple[str, str, str, float]:
        w = doc.word_count
        s = max(1, doc.sentence_count)
        if w == 0:
            grade = 0.0
        else:
            grade = 0.39 * (w / s) + 11.8 * (doc.syllables / w) - 15.59
        # LLM prose tends to land in a mid-to-high grade band.
        points = _plateau(grade, 5.0, 11.0, 16.0, 22.0, lex.MAX_FLESCH_KINCAID)
        return (
            "flesch_kincaid",
            f"grade={grade:.1f}",
            f"Flesch-Kincaid grade level = {grade:.1f}",
            points,
        )

    def _punctuation(self, doc: Document) -> tuple[str, str, str, float]:
        w = max(1, doc.word_count)
        em_dashes = doc.raw.count("—") + doc.text.count("--")
        semicolons = doc.raw.count(";")
        oxford = len(_OXFORD_RE.findall(doc.text))
        smart_quotes = sum(doc.raw.count(c) for c in "“”‘’")
        signals = em_dashes + semicolons + oxford + smart_quotes
        per100 = signals / w * 100

        has_contraction = any(c in doc.lower for c in lex.CONTRACTIONS)
        no_contraction_bonus = (
            1.0 if (not has_contraction and doc.word_count >= 30) else 0.0
        )
        points = min(
            lex.MAX_PUNCTUATION,
            _up(per100, 0.2, 2.5, lex.MAX_PUNCTUATION - 1.0) + no_contraction_bonus,
        )
        return (
            "punctuation",
            f"{signals} marks, {per100:.1f}/100w",
            f"punctuation patterns: {em_dashes} em-dash, {oxford} Oxford comma, "
            f"{semicolons} semicolon, {smart_quotes} smart-quote; "
            f"contractions {'absent' if not has_contraction else 'present'}",
            points,
        )

    def _rare_words(self, doc: Document) -> tuple[str, str, str, float]:
        content = [w.lower() for w in doc.words if len(w) >= 4 and w.isalpha()]
        if not content:
            frac = 0.0
        else:
            rare = sum(1 for w in content if w not in lex.COMMON_WORDS)
            frac = rare / len(content)
        points = _up(frac, 0.15, 0.55, lex.MAX_RARE_WORD)
        return (
            "rare_word",
            f"{frac:.0%} rare",
            f"rare-word usage = {frac:.0%} of content words outside common list",
            points,
        )


def lex_split(sentence: str) -> list[str]:
    """Word tokens in one sentence."""
    from ..tokenize import split_words

    return split_words(sentence)
