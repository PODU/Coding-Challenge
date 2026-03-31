"""Sentence, word and syllable splitting shared by the detectors.

Smart quotes, dashes and ellipses are normalised so the regexes only need to
handle one form. The raw string is kept on :class:`Document` because the
punctuation factor counts the original em-dashes and smart quotes.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

_SMART_QUOTES = {
    "“": '"',
    "”": '"',
    "‘": "'",
    "’": "'",  # also used as an apostrophe
    "„": '"',
    "‚": "'",
}
_DASHES = {
    "—": "--",
    "–": "-",
}
_ELLIPSIS = {"…": "..."}

_WORD_RE = re.compile(r"[^\W_]+(?:['’][^\W_]+)*", re.UNICODE)
# One or more of .!? (plus closing quotes/brackets) followed by space or end.
_SENTENCE_SPLIT_RE = re.compile(r"[.!?]+(?:[\"')\]]+)?(?:\s+|$)")
_VOWEL_GROUP_RE = re.compile(r"[aeiouy]+", re.IGNORECASE)


def normalize(text: str) -> str:
    for table in (_SMART_QUOTES, _DASHES, _ELLIPSIS):
        for src, dst in table.items():
            text = text.replace(src, dst)
    return text


def split_words(text: str) -> list[str]:
    """Unicode-aware word split that keeps internal apostrophes."""
    return _WORD_RE.findall(text)


def split_sentences(text: str) -> list[str]:
    return [s.strip() for s in _SENTENCE_SPLIT_RE.split(text) if s and s.strip()]


def count_syllables(word: str) -> int:
    """Rough English syllable count, at least 1 for any non-empty word."""
    w = word.lower().strip("'’")
    if not w:
        return 0
    if not re.search(r"[aeiouy]", w):
        return 1
    count = len(_VOWEL_GROUP_RE.findall(w))
    # Silent trailing 'e' (but not 'le' as in "table").
    if w.endswith("e") and not w.endswith("le") and count > 1:
        count -= 1
    return max(1, count)


@dataclass(frozen=True)
class Document:
    """An input text, tokenised once and shared by all detectors."""

    raw: str
    text: str  # normalised, for pattern matching
    words: list[str] = field(default_factory=list)
    sentences: list[str] = field(default_factory=list)
    syllables: int = 0

    @property
    def char_count(self) -> int:
        return len(self.raw)

    @property
    def word_count(self) -> int:
        return len(self.words)

    @property
    def sentence_count(self) -> int:
        return len(self.sentences)

    @property
    def is_empty(self) -> bool:
        return self.word_count == 0

    @property
    def is_short(self) -> bool:
        """Too little text for the statistical factors to mean much."""
        return self.word_count < 20 or self.sentence_count < 2

    @property
    def lower(self) -> str:
        return self.text.lower()


def build_document(raw: str) -> Document:
    text = normalize(raw)
    words = split_words(text)
    sentences = split_sentences(text)
    syllables = sum(count_syllables(w) for w in words)
    return Document(
        raw=raw,
        text=text,
        words=words,
        sentences=sentences,
        syllables=syllables,
    )
