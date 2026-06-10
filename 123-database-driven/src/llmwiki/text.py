"""Text helpers: tokenizing, sentence splitting, slugs and hashing."""

from __future__ import annotations

import hashlib
import re

_WORD_RE = re.compile(r"[A-Za-z0-9']+")
_SENT_RE = re.compile(r"(?<=[.!?])\s+")
_SLUG_STRIP_RE = re.compile(r"[^a-z0-9]+")
_NUM_RE = re.compile(r"\d+(?:\.\d+)?")

# Hard-coded so there's no NLTK download.
STOPWORDS: frozenset[str] = frozenset(
    """
    a an the and or but if then else of to in on at by for with from into over
    under is are was were be been being this that these those it its as he she
    they them his her their our your my we you i not no never can cannot could
    would should will shall may might must do does did done has have had having
    which who whom whose what when where why how than so such also about between
    """.split()
)


def tokenize(text: str) -> list[str]:
    """Lowercase word tokens with apostrophes removed."""
    return [m.group(0).replace("'", "").lower() for m in _WORD_RE.finditer(text)]


def content_tokens(text: str) -> list[str]:
    """Tokens of length >= 2 that aren't stop words."""
    return [t for t in tokenize(text) if t not in STOPWORDS and len(t) > 1]


def sentences(text: str) -> list[str]:
    return [p.strip() for p in _SENT_RE.split(text.strip()) if p.strip()]


def numbers(text: str) -> set[str]:
    return set(_NUM_RE.findall(text))


def slug(text: str, max_len: int = 80) -> str:
    """Slug that is safe in file names and URLs."""
    s = _SLUG_STRIP_RE.sub("-", text.strip().lower()).strip("-")
    if len(s) > max_len:
        s = s[:max_len].rstrip("-")
    return s or "untitled"


def sha256_hex(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def stable_hash(token: str) -> int:
    """64-bit hash that, unlike hash(), is the same on every run."""
    return int.from_bytes(hashlib.md5(token.encode("utf-8")).digest()[:8], "big")
