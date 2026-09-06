"""Turn spoken ordinal runs into a real list.

"first check the logs second restart the service third tell the team" becomes

    Here are the steps:
    1. Check the logs
    2. Restart the service
    3. Tell the team

Dictated speech rarely carries the punctuation that would mark the item boundaries, so
the items are found from the ordinal words themselves rather than from sentence splits.
A run only counts when it opens at "first" and the ordinals climb by exactly one, which
keeps "I had second thoughts" out of it. `bullet point X` marks an item explicitly.
"""

from __future__ import annotations

from typing import Callable

from .tokens import Kind, Token, brk, marker, punct

ORDINALS = {
    "first": 1,
    "firstly": 1,
    "second": 2,
    "secondly": 2,
    "third": 3,
    "thirdly": 3,
    "fourth": 4,
    "fourthly": 4,
    "fifth": 5,
    "sixth": 6,
    "seventh": 7,
    "eighth": 8,
    "ninth": 9,
    "tenth": 10,
}

MIN_RUN = 2
_ITEM_LEAD_PUNCT = ",:;"


def apply_lists(tokens: list[Token], *, style: str = "numbered", bullet: str = "-") -> list[Token]:
    """Rewrite an ordinal run or a set of spoken bullets into marked list lines."""
    bullets = [i for i, t in enumerate(tokens) if t.kind is Kind.MARKER and t.meta.get("bullet")]
    if bullets:
        return _build(tokens, bullets, lambda _n: bullet)

    chain = _ordinal_chain(tokens)
    if not chain:
        return tokens
    mark = (lambda n: f"{n}.") if style == "numbered" else (lambda _n: bullet)
    return _build(tokens, chain, mark)


def _ordinal_chain(tokens: list[Token]) -> list[int]:
    """Indices of ordinal words forming an ascending 1,2,3,... run of length >= MIN_RUN."""
    chain: list[int] = []
    expected = 1
    for i, tok in enumerate(tokens):
        if tok.is_word and ORDINALS.get(tok.norm) == expected:
            chain.append(i)
            expected += 1
    if len(chain) < MIN_RUN:
        return []
    # "first line new line second line": the speaker already laid the items out by hand
    # with spoken breaks, so they wanted lines, not a list. Manual formatting wins.
    if any(tok.kind is Kind.BREAK for tok in tokens[chain[0] : chain[-1]]):
        return []
    return chain


def _build(tokens: list[Token], starts: list[int], mark: Callable[[int], str]) -> list[Token]:
    end = _list_end(tokens, starts[-1])
    intro = _trim_intro(tokens[: starts[0]])
    tail = tokens[end:]

    out: list[Token] = list(intro)
    if out and out[-1].kind is not Kind.BREAK:
        out.append(brk("line"))

    emitted = 0
    for n, start in enumerate(starts):
        stop = starts[n + 1] if n + 1 < len(starts) else end
        body = _trim_item(tokens[start + 1 : stop])
        if not body:
            continue
        emitted += 1
        out.append(marker(mark(emitted)))
        out.extend(body)
        out.append(brk("line"))

    if not emitted:
        return tokens
    while out and out[-1].kind is Kind.BREAK and not tail:
        out.pop()
    out.extend(tail)
    return out


def _list_end(tokens: list[Token], last_start: int) -> int:
    """A paragraph break closes the list; otherwise it runs to the end of the utterance."""
    for i in range(last_start, len(tokens)):
        if tokens[i].kind is Kind.BREAK and tokens[i].text == "paragraph":
            return i
    return len(tokens)


def _trim_intro(tokens: list[Token]) -> list[Token]:
    out = list(tokens)
    while out and (out[-1].kind is Kind.BREAK or (out[-1].kind is Kind.PUNCT and out[-1].text in _ITEM_LEAD_PUNCT)):
        out.pop()
    return out


def _trim_item(tokens: list[Token]) -> list[Token]:
    out = list(tokens)
    while out and out[0].kind is Kind.PUNCT and out[0].text in _ITEM_LEAD_PUNCT:
        out.pop(0)
    while out and (out[-1].kind is Kind.BREAK or (out[-1].kind is Kind.PUNCT and out[-1].attach == "left")):
        out.pop()
    return out


def punctuate_intro(tokens: list[Token]) -> list[Token]:
    """Give a real introducing clause a colon: "Here are the steps:" before the items.

    Runs after filler removal so a line that was only "um so" is not dressed up as an
    introduction it never was.
    """
    first = next((i for i, t in enumerate(tokens) if t.kind is Kind.MARKER), -1)
    if first < 2 or tokens[first - 1].kind is not Kind.BREAK:
        return tokens  # the list opens the utterance: there is no introduction
    start = 0
    for j in range(first - 2, -1, -1):
        if tokens[j].kind is Kind.BREAK:
            start = j + 1
            break
    intro = tokens[start : first - 1]
    if intro and intro[-1].is_word and sum(1 for t in intro if t.is_word) >= 2:
        return tokens[: first - 1] + [punct(":")] + tokens[first - 1 :]
    return tokens


def ends_with_list_item(tokens: list[Token]) -> bool:
    """True when the last line is a list item, which shouldn't get a final period."""
    for tok in reversed(tokens):
        if tok.kind is Kind.BREAK:
            return False
        if tok.kind is Kind.MARKER:
            return True
    return False
