"""The corrector from Peter Norvig's "How to Write a Spelling Corrector".

Among the known words at the smallest edit distance, the most frequent one
wins. Ties go to the alphabetically first word so results don't depend on
set iteration order.
"""

from __future__ import annotations

from collections import Counter
from collections.abc import Iterable, Iterator, Mapping
from importlib import resources

from .corpus import load_frequency_csv, load_text_file, normalize

__all__ = ["SpellCorrector", "ALPHABET", "default_corrector"]

ALPHABET = "abcdefghijklmnopqrstuvwxyz"


class SpellCorrector:
    """Spelling corrector backed by a word frequency table."""

    def __init__(self, frequencies: Mapping[str, int]) -> None:
        # Our own copy, with keys normalized the same way lookups are.
        self._freq: Counter[str] = Counter()
        for word, count in frequencies.items():
            key = normalize(word)
            if key and count > 0:
                self._freq[key] += int(count)
        self._total: int = sum(self._freq.values())

    @classmethod
    def from_text_file(cls, path: str, encoding: str = "utf-8") -> "SpellCorrector":
        return cls(load_text_file(path, encoding=encoding))

    @classmethod
    def from_frequency_csv(
        cls, path: str, *, encoding: str = "utf-8", has_header: bool = True
    ) -> "SpellCorrector":
        return cls(load_frequency_csv(path, encoding=encoding, has_header=has_header))

    @property
    def total(self) -> int:
        """Sum of all word counts."""
        return self._total

    def __len__(self) -> int:
        return len(self._freq)

    def __contains__(self, word: object) -> bool:
        return isinstance(word, str) and normalize(word) in self._freq

    def frequency(self, word: str) -> int:
        return self._freq.get(normalize(word), 0)

    def is_known(self, word: str) -> bool:
        key = normalize(word)
        return bool(key) and key in self._freq

    def probability(self, word: str) -> float:
        """count / total, or 0.0 for an unknown word or an empty table."""
        if self._total == 0:
            return 0.0
        return self._freq.get(normalize(word), 0) / self._total

    @staticmethod
    def edits1(word: str) -> set[str]:
        """Every string one delete, transpose, replace or insert away."""
        splits = [(word[:i], word[i:]) for i in range(len(word) + 1)]
        deletes = [left + right[1:] for left, right in splits if right]
        transposes = [
            left + right[1] + right[0] + right[2:]
            for left, right in splits
            if len(right) > 1
        ]
        replaces = [
            left + char + right[1:]
            for left, right in splits
            if right
            for char in ALPHABET
        ]
        inserts = [left + char + right for left, right in splits for char in ALPHABET]
        return set(deletes + transposes + replaces + inserts)

    @classmethod
    def edits2(cls, word: str) -> Iterator[str]:
        """Strings two edits away. A generator, since the full set is large."""
        for first in cls.edits1(word):
            yield from cls.edits1(first)

    def known(self, words: Iterable[str]) -> set[str]:
        return {w for w in words if w in self._freq}

    def candidates(self, word: str) -> set[str]:
        """Known words at the smallest edit distance (0, 1 or 2) from *word*."""
        key = normalize(word)
        if not key:
            return set()
        if key in self._freq:
            return {key}
        edit1 = self.known(self.edits1(key))
        if edit1:
            return edit1
        return self.known(self.edits2(key))

    def edit_distance(self, word: str) -> int:
        """Distance to the nearest known word: 0, 1, 2, or -1 if none is that close."""
        key = normalize(word)
        if not key:
            return -1
        if key in self._freq:
            return 0
        if self.known(self.edits1(key)):
            return 1
        if self.known(self.edits2(key)):
            return 2
        return -1

    def correct(self, word: str) -> str:
        """Best correction for *word*, or the normalized word if nothing is close."""
        key = normalize(word)
        if not key:
            return ""
        cands = self.candidates(key)
        if not cands:
            return key
        return min(cands, key=lambda w: (-self._freq[w], w))


def default_corrector() -> SpellCorrector:
    """Corrector trained on the small corpus bundled with the package."""
    with resources.as_file(
        resources.files("spellcorrect").joinpath("data/sample_corpus.txt")
    ) as path:
        return SpellCorrector.from_text_file(str(path))
