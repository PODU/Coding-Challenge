"""Password generators for the two attack modes. Both are lazy so a huge
wordlist or search space uses little memory."""

from __future__ import annotations

import itertools
import string
from collections.abc import Iterator

DEFAULT_CHARSET = string.ascii_lowercase


def from_wordlist(path: str) -> Iterator[str]:
    """Yield each non-blank line of *path*.

    surrogateescape lets lines that aren't valid UTF-8 pass through unchanged.
    """
    with open(path, "r", encoding="utf-8", errors="surrogateescape", newline="") as fh:
        for line in fh:
            word = line.rstrip("\r\n")
            if word:
                yield word


def brute_force(
    min_len: int,
    max_len: int,
    charset: str = DEFAULT_CHARSET,
) -> Iterator[str]:
    """Yield every string over *charset* from min_len to max_len characters,
    shortest first."""
    if min_len < 1:
        raise ValueError("min_len must be >= 1")
    if max_len < min_len:
        raise ValueError("max_len must be >= min_len")
    if not charset:
        raise ValueError("charset must be non-empty")
    for length in range(min_len, max_len + 1):
        for combo in itertools.product(charset, repeat=length):
            yield "".join(combo)
