"""Filler removal, stutter collapse, custom dictionary and casing.

Each function is a plain transform on a token list. There is no model involved; the
only grammar work done is punctuation and casing, which simple rules handle well.
"""

from __future__ import annotations

from .tokens import ATTACH_LEFT, Kind, Token, clause_starts, word

# Words that legitimately double in English, so a repeat is not a stutter.
REPEATABLE = {"had", "that", "no", "very", "ha", "very"}

# Never re-cased by the sentence-casing pass: the speaker/dictionary meant them.
_ALWAYS_CAPS = {"i"}


def remove_fillers(tokens: list[Token], fillers: list[str], phrases: list[str]) -> list[Token]:
    """Drop "um"/"uh" style words and multi-word verbal tics."""
    filler_set = {f.lower() for f in fillers}
    phrase_keys = sorted((tuple(p.lower().split()) for p in phrases), key=len, reverse=True)

    out: list[Token] = []
    i = 0
    n = len(tokens)
    while i < n:
        tok = tokens[i]
        if tok.is_word:
            hit = _phrase_at(tokens, i, phrase_keys)
            if hit:
                i += hit
                continue
            if tok.norm in filler_set:
                i += 1
                continue
        out.append(tok)
        i += 1
    return normalize_punctuation(out)


def _phrase_at(tokens: list[Token], start: int, phrases: list[tuple[str, ...]]) -> int:
    for key in phrases:
        end = start + len(key)
        if end > len(tokens):
            continue
        window = tokens[start:end]
        if all(t.is_word for t in window) and tuple(t.norm for t in window) == key:
            return len(key)
    return 0


# Which mark wins when removing a filler leaves two marks adjacent ("so, , then").
_PUNCT_RANK = {",": 0, ";": 1, ":": 1}


def normalize_punctuation(tokens: list[Token]) -> list[Token]:
    """Drop marks left dangling by a removed word: doubled commas, clause-leading marks."""
    out: list[Token] = []
    for tok in tokens:
        if tok.kind is Kind.PUNCT and tok.attach == ATTACH_LEFT:
            if not out or out[-1].kind is Kind.BREAK:
                continue  # a clause never opens with a comma
            prev = out[-1]
            if prev.kind is Kind.PUNCT and prev.attach == ATTACH_LEFT:
                if _PUNCT_RANK.get(tok.text, 2) > _PUNCT_RANK.get(prev.text, 2):
                    out[-1] = tok
                continue
        out.append(tok)
    return out


def collapse_repeats(tokens: list[Token]) -> list[Token]:
    """"the the plan" -> "the plan". Words that genuinely double are left alone."""
    out: list[Token] = []
    for tok in tokens:
        prev = out[-1] if out else None
        if (
            tok.is_word
            and prev is not None
            and prev.is_word
            and prev.norm == tok.norm
            and tok.norm not in REPEATABLE
        ):
            continue
        out.append(tok)
    return out


def apply_dictionary(tokens: list[Token], dictionary: dict[str, str]) -> list[Token]:
    """Replace dictionary phrases: whole tokens, case-insensitive, longest phrase first."""
    if not dictionary:
        return tokens
    keys = sorted((tuple(k.lower().split()), v) for k, v in dictionary.items() if k.strip())
    keys.sort(key=lambda kv: len(kv[0]), reverse=True)

    out: list[Token] = []
    i = 0
    n = len(tokens)
    while i < n:
        replaced = False
        for key, value in keys:
            end = i + len(key)
            if end > n:
                continue
            window = tokens[i:end]
            if all(t.is_word for t in window) and tuple(t.norm for t in window) == key:
                out.extend(word(part, fixed_case=True) for part in value.split())
                i = end
                replaced = True
                break
        if not replaced:
            out.append(tokens[i])
            i += 1
    return out


def apply_casing(tokens: list[Token]) -> list[Token]:
    """Capitalise sentence openers and the pronoun "I"; leave everything else alone."""
    starts = set(clause_starts(tokens))
    out: list[Token] = []
    for i, tok in enumerate(tokens):
        if not tok.is_word or tok.fixed_case:
            out.append(tok)
            continue
        text = tok.text
        if text.lower() in _ALWAYS_CAPS or text.lower().startswith("i'"):
            text = text[0].upper() + text[1:]
        elif i in starts and text[:1].islower():
            text = text[0].upper() + text[1:]
        out.append(Token(tok.kind, text, tok.attach, tok.fixed_case, dict(tok.meta)))
    return out


def ensure_terminal_punctuation(tokens: list[Token]) -> list[Token]:
    """A dictated sentence that just stops gets a full stop."""
    trimmed = list(tokens)
    while trimmed and trimmed[-1].kind is Kind.BREAK:
        trimmed.pop()
    if not trimmed:
        return trimmed
    last = trimmed[-1]
    if last.kind is Kind.PUNCT and last.text in {".", "!", "?", "...", ":", ";", ","}:
        if last.text == ",":
            trimmed[-1] = Token(Kind.PUNCT, ".")
        return trimmed
    if last.kind is Kind.PUNCT and last.text in {'"', ")"}:
        trimmed.insert(len(trimmed) - 1, Token(Kind.PUNCT, "."))
        return trimmed
    trimmed.append(Token(Kind.PUNCT, "."))
    return trimmed
