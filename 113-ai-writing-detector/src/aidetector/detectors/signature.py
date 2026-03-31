"""Words and phrases that LLMs overuse.

Each distinct term counts once, case-insensitively.
"""

from __future__ import annotations

import re

from .. import lexicons as lex
from ..tokenize import Document
from .base import Detection, Finding


def _term_re(term: str) -> re.Pattern[str]:
    return re.compile(rf"(?<!\w){re.escape(term)}(?!\w)", re.IGNORECASE)


_PHRASE_RES = [(p, _term_re(p)) for p in lex.SIGNATURE_PHRASES]
_WORD_RES = [(w, _term_re(w)) for w in lex.SIGNATURE_WORDS]


class SignatureDetector:
    name = "Signature words/phrases"
    step = 2
    cap = lex.CAP_SIGNATURE

    def run(self, doc: Document) -> Detection:
        text = doc.lower
        findings: list[Finding] = []
        seen: set[str] = set()

        # Phrases first; they're more specific than single words.
        for phrase, rx in _PHRASE_RES:
            if phrase in seen:
                continue
            if rx.search(text):
                seen.add(phrase)
                findings.append(
                    Finding(
                        pattern="signature_phrase",
                        evidence=phrase,
                        detail=f'signature phrase "{phrase}"',
                        weight=lex.WEIGHT_SIGNATURE_PHRASE,
                    )
                )

        for word, rx in _WORD_RES:
            if word in seen:
                continue
            if rx.search(text):
                seen.add(word)
                findings.append(
                    Finding(
                        pattern="signature_word",
                        evidence=word,
                        detail=f'signature word "{word}"',
                        weight=lex.WEIGHT_SIGNATURE_WORD,
                    )
                )

        return Detection.build(self.name, self.step, self.cap, findings)
