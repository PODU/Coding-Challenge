"""The token stream the text pipeline operates on.

Working on tokens instead of running regexes over a string is what lets the pipeline
stages compose: the spoken-command stage can turn two words into a line break, and the
list stage can then see that break without having to re-parse anything.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from enum import Enum

LEADING_PUNCT = "\"'([{"
TRAILING_PUNCT = ".,!?;:)]}\"'"

# How a punctuation mark glues to its neighbours when rendered.
ATTACH_LEFT = "left"  # . , ? ! : ; ) closing quote
ATTACH_RIGHT = "right"  # ( opening quote
ATTACH_BOTH = "both"  # - / (no space either side)

SENTENCE_END = {".", "!", "?", "..."}


class Kind(Enum):
    WORD = "word"
    PUNCT = "punct"
    BREAK = "break"  # line / paragraph / tab
    MARKER = "marker"  # a list bullet or number we inserted ourselves


@dataclass
class Token:
    kind: Kind
    text: str
    attach: str = ATTACH_LEFT
    fixed_case: bool = False
    """Set on dictionary replacements and list markers: never re-case these."""
    meta: dict = field(default_factory=dict)

    @property
    def norm(self) -> str:
        return self.text.lower().strip("'")

    @property
    def is_word(self) -> bool:
        return self.kind is Kind.WORD

    @property
    def ends_sentence(self) -> bool:
        return self.kind is Kind.PUNCT and self.text in SENTENCE_END


def word(text: str, *, fixed_case: bool = False) -> Token:
    return Token(Kind.WORD, text, fixed_case=fixed_case)


def punct(text: str, attach: str = ATTACH_LEFT) -> Token:
    return Token(Kind.PUNCT, text, attach=attach)


def brk(style: str = "line") -> Token:
    return Token(Kind.BREAK, style)


def marker(text: str) -> Token:
    return Token(Kind.MARKER, text, attach=ATTACH_RIGHT, fixed_case=True)


_WS = re.compile(r"\s+")


def tokenize(text: str) -> list[Token]:
    """Split raw ASR output into words and free-standing punctuation.

    Engines differ: whisper.cpp punctuates, vosk does not. Pulling punctuation off the
    word means every later stage sees bare words either way.
    """
    out: list[Token] = []
    for chunk in _WS.split(text.strip()):
        if not chunk:
            continue
        if chunk == "-":  # a lone dash is a real dash, not a stray hyphen
            out.append(punct("-", ATTACH_BOTH))
            continue
        lead = ""
        while chunk and chunk[0] in LEADING_PUNCT:
            lead, chunk = lead + chunk[0], chunk[1:]
        trail = ""
        while chunk and chunk[-1] in TRAILING_PUNCT:
            trail, chunk = chunk[-1] + trail, chunk[:-1]
        for ch in lead:
            out.append(punct(ch, ATTACH_RIGHT))
        if chunk:
            out.append(word(chunk))
        i = 0
        while i < len(trail):  # keep "..." together
            if trail.startswith("...", i):
                out.append(punct("..."))
                i += 3
            else:
                out.append(punct(trail[i]))
                i += 1
    return out


def render(tokens: list[Token]) -> str:
    """Tokens back to text, with the spacing a human would have typed."""
    parts: list[str] = []
    at_line_start = True
    open_pending = False  # previous token was "(" or an opening quote

    def emit(text: str, space: bool) -> None:
        nonlocal at_line_start, open_pending
        if space and not at_line_start and not open_pending:
            parts.append(" ")
        parts.append(text)
        at_line_start = False
        open_pending = False

    for tok in tokens:
        if tok.kind is Kind.BREAK:
            if tok.text == "tab":
                parts.append("\t")
                at_line_start = True
                continue
            newline = "\n\n" if tok.text == "paragraph" else "\n"
            while parts and parts[-1] in (" ", "\n", "\n\n"):
                parts.pop()  # never leave trailing space before a break
            if not parts:
                continue  # no leading blank lines
            parts.append(newline)
            at_line_start = True
            open_pending = False
        elif tok.kind is Kind.PUNCT:
            if at_line_start and tok.attach == ATTACH_LEFT:
                continue  # a clause never opens with ", " or ". "
            emit(tok.text, space=tok.attach == ATTACH_RIGHT)
            if tok.attach == ATTACH_RIGHT:
                open_pending = True
            elif tok.attach == ATTACH_BOTH:
                open_pending = True
        else:
            emit(tok.text, space=True)
    return "".join(parts).strip()


def clause_starts(tokens: list[Token]) -> list[int]:
    """Indices where a new clause begins: start of text, after a break, after . ! ?"""
    starts: list[int] = []
    expecting = True
    for i, tok in enumerate(tokens):
        if expecting and tok.kind is Kind.WORD:
            starts.append(i)  # a list marker is not the word that gets capitalised
            expecting = False
        if tok.kind is Kind.BREAK or tok.ends_sentence:
            expecting = True
    return starts
