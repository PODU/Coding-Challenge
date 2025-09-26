"""Load training data into a word -> count :class:`~collections.Counter`.

Two formats are read: a plain-text corpus like Norvig's ``big.txt``, which is
tokenized on ``[a-z]+``, and a ``word,count`` CSV like Kaggle's English Word
Frequency dataset.
"""

from __future__ import annotations

import csv
import re
import unicodedata
from collections import Counter

__all__ = [
    "WORD_RE",
    "normalize",
    "tokenize",
    "count_text",
    "load_text_file",
    "load_frequency_csv",
]

WORD_RE = re.compile(r"[a-z]+")


def normalize(word: str) -> str:
    """Lowercase *word* and keep only ``a-z``, stripping accents first.

    ``café`` becomes ``cafe``. A word with no ASCII letters becomes ``""``.
    """
    decomposed = unicodedata.normalize("NFKD", word)
    ascii_only = decomposed.encode("ascii", "ignore").decode("ascii")
    return "".join(WORD_RE.findall(ascii_only.lower()))


def tokenize(text: str) -> list[str]:
    return WORD_RE.findall(text.lower())


def count_text(text: str) -> Counter[str]:
    return Counter(tokenize(text))


def load_text_file(path: str, encoding: str = "utf-8") -> Counter[str]:
    with open(path, encoding=encoding, errors="replace") as handle:
        return count_text(handle.read())


def load_frequency_csv(
    path: str,
    *,
    encoding: str = "utf-8",
    has_header: bool = True,
) -> Counter[str]:
    """Load a ``word,count`` CSV.

    Duplicate words are summed. Rows without a usable word or a numeric count
    are skipped.
    """
    table: Counter[str] = Counter()
    with open(path, encoding=encoding, newline="") as handle:
        reader = csv.reader(handle)
        if has_header:
            next(reader, None)
        for row in reader:
            if len(row) < 2:
                continue
            word = normalize(row[0])
            if not word:
                continue
            try:
                count = int(float(row[1].strip()))
            except (ValueError, AttributeError):
                continue
            if count > 0:
                table[word] += count
    return table
