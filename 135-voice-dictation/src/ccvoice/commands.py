"""Spoken formatting commands.

"new line" becomes an actual line break, "period" becomes ".", and so on. Matching is
longest-phrase-first over the token stream, and `literally <phrase>` escapes back to the
plain words for when you really do want to dictate the word "period".
"""

from __future__ import annotations

from typing import Callable

from .tokens import ATTACH_BOTH, ATTACH_LEFT, ATTACH_RIGHT, Kind, Token, brk, punct

ESCAPE_WORDS = {"literally", "literal"}

# phrase (as spoken, lowercase) -> token factory
COMMANDS: dict[tuple[str, ...], Callable[[], Token]] = {
    ("new", "line"): lambda: brk("line"),
    ("newline",): lambda: brk("line"),
    ("next", "line"): lambda: brk("line"),
    ("new", "paragraph"): lambda: brk("paragraph"),
    ("tab",): lambda: brk("tab"),
    ("period",): lambda: punct("."),
    ("full", "stop"): lambda: punct("."),
    ("comma",): lambda: punct(","),
    ("question", "mark"): lambda: punct("?"),
    ("exclamation", "mark"): lambda: punct("!"),
    ("exclamation", "point"): lambda: punct("!"),
    ("colon",): lambda: punct(":"),
    ("semicolon",): lambda: punct(";"),
    ("semi", "colon"): lambda: punct(";"),
    ("ellipsis",): lambda: punct("..."),
    ("dot", "dot", "dot"): lambda: punct("..."),
    ("dash",): lambda: punct("-", ATTACH_BOTH),
    ("hyphen",): lambda: punct("-", ATTACH_BOTH),
    ("slash",): lambda: punct("/", ATTACH_BOTH),
    ("open", "quote"): lambda: punct('"', ATTACH_RIGHT),
    ("close", "quote"): lambda: punct('"', ATTACH_LEFT),
    ("open", "paren"): lambda: punct("(", ATTACH_RIGHT),
    ("open", "parenthesis"): lambda: punct("(", ATTACH_RIGHT),
    ("close", "paren"): lambda: punct(")", ATTACH_LEFT),
    ("close", "parenthesis"): lambda: punct(")", ATTACH_LEFT),
    ("bullet", "point"): lambda: Token(Kind.MARKER, "", meta={"bullet": True}),
}

MAX_PHRASE = max(len(k) for k in COMMANDS)


def apply_commands(tokens: list[Token]) -> list[Token]:
    """Replace spoken command phrases with the punctuation/breaks they name."""
    out: list[Token] = []
    i = 0
    n = len(tokens)
    while i < n:
        tok = tokens[i]
        if not tok.is_word:
            out.append(tok)
            i += 1
            continue

        if tok.norm in ESCAPE_WORDS:
            phrase_len = _match_len(tokens, i + 1)
            if phrase_len:  # "literally new line" -> the words "new line"
                out.extend(tokens[i + 1 : i + 1 + phrase_len])
                i += 1 + phrase_len
                continue

        phrase_len = _match_len(tokens, i)
        if phrase_len:
            key = tuple(tokens[i + k].norm for k in range(phrase_len))
            out.append(COMMANDS[key]())
            i += phrase_len
            continue

        out.append(tok)
        i += 1
    return out


def _match_len(tokens: list[Token], start: int) -> int:
    """Length of the longest command phrase starting at `start`, or 0."""
    for size in range(min(MAX_PHRASE, len(tokens) - start), 0, -1):
        window = tokens[start : start + size]
        if any(not t.is_word for t in window):
            continue
        if tuple(t.norm for t in window) in COMMANDS:
            return size
    return 0


def describe() -> list[tuple[str, str]]:
    """(spoken phrase, what it inserts) pairs, for `ccvoice doctor`."""
    rows = []
    for phrase, factory in COMMANDS.items():
        tok = factory()
        if tok.kind is Kind.BREAK:
            shown = {"line": "\\n", "paragraph": "\\n\\n", "tab": "\\t"}[tok.text]
        elif tok.kind is Kind.MARKER:
            shown = "list bullet"
        else:
            shown = tok.text
        rows.append((" ".join(phrase), shown))
    return sorted(rows)


__all__ = ["apply_commands", "describe", "COMMANDS", "ESCAPE_WORDS"]
