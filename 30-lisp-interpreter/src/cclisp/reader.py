"""Turn tokens into nested lists and atoms."""

from __future__ import annotations

from .tokenizer import tokenize
from .types import LispSyntaxError, Symbol

# AST node type: Symbol | int | float | str | bool | list
AST = object

_ESCAPES = {
    "n": "\n",
    "t": "\t",
    "r": "\r",
    "\\": "\\",
    '"': '"',
    "0": "\0",
}


def _decode_string(token: str) -> str:
    """Decode a string token, quotes and all, into its value."""
    inner = token[1:-1]
    out: list[str] = []
    i = 0
    n = len(inner)
    while i < n:
        ch = inner[i]
        if ch == "\\" and i + 1 < n:
            nxt = inner[i + 1]
            out.append(_ESCAPES.get(nxt, nxt))
            i += 2
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def atom(token: str) -> AST:
    """Convert a non-list token into its value."""
    if token.startswith('"'):
        return _decode_string(token)
    if token == "t":
        return True
    if token == "nil":
        return False
    try:
        return int(token)
    except ValueError:
        pass
    try:
        return float(token)
    except ValueError:
        pass
    return Symbol(token)


def read_from_tokens(tokens: list[str]) -> AST:
    """Pop one complete expression off the front of *tokens*."""
    if not tokens:
        raise LispSyntaxError("unexpected EOF while reading")

    token = tokens.pop(0)

    if token == "(":
        lst: list[AST] = []
        while True:
            if not tokens:
                raise LispSyntaxError("unexpected EOF while reading: missing )")
            if tokens[0] == ")":
                tokens.pop(0)
                return lst
            lst.append(read_from_tokens(tokens))

    if token == ")":
        raise LispSyntaxError("unexpected )")

    if token == "'":
        # 'x  ->  (quote x)
        return [Symbol("quote"), read_from_tokens(tokens)]

    return atom(token)


def parse(program: str) -> AST:
    """Read the single expression in *program*. Anything after it is an error."""
    tokens = tokenize(program)
    if not tokens:
        raise LispSyntaxError("unexpected EOF while reading: empty input")
    expr = read_from_tokens(tokens)
    if tokens:
        raise LispSyntaxError(f"unexpected trailing tokens: {tokens!r}")
    return expr


def parse_all(program: str) -> list[AST]:
    """Read every top-level form in *program*."""
    tokens = tokenize(program)
    forms: list[AST] = []
    while tokens:
        forms.append(read_from_tokens(tokens))
    return forms
