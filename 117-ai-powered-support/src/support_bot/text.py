"""Text helpers. Everything that splits text into words uses these, so the
embedder, the off-topic check and the token budgets agree on what a word is."""

from __future__ import annotations

import re

_WORD_RE = re.compile(r"[a-z0-9]+")

# Without these, words like "how" or "the" would make any question look
# on-topic.
STOPWORDS: frozenset[str] = frozenset(
    """
    a an and are as at be been being but by can could did do does doing for
    from had has have having he her here hers him his how i if in into is it
    its me my no nor not of on or our ours out over she should so some such
    than that the their theirs them then there these they this those to too
    up us was we were what when where which who whom why will with would you
    your yours
    """.split()
)


def tokenize(text: str) -> list[str]:
    """Lower-cased words, minus stop-words and single characters."""
    return [t for t in _WORD_RE.findall(text.lower()) if len(t) > 1 and t not in STOPWORDS]


def raw_tokens(text: str) -> list[str]:
    """Lower-cased words with nothing removed."""
    return _WORD_RE.findall(text.lower())


def count_tokens(text: str) -> int:
    """Approximate token count (whitespace-separated words), used for prompt
    trimming only. Real usage comes back from the API."""
    if not text:
        return 0
    return len(text.split())
