"""ELIZA's response generator.

The only state is the per-rule reassembly cursors and the memory queue, so
with the default "cycle" mode the replies are fully deterministic.
"""

from __future__ import annotations

import random
import re
from collections import deque
from typing import NamedTuple

from .reflect import reflect
from .script import XNONE, Keyword, Script

__all__ = ["ElizaEngine"]

_CLAUSE_SPLIT = re.compile(r"[.,;!?]")
_PLACEHOLDER = re.compile(r"\((\d+)\)")
_STRIP_EDGES = " .,;:!?\"'"


class _Hit(NamedTuple):
    name: str
    rank: int
    appearance: int


class ElizaEngine:
    """Generates replies from a script.

    ``reassembly="cycle"`` steps through each rule's templates in turn;
    ``"random"`` picks one with ``rng``.
    """

    def __init__(
        self,
        script: Script,
        *,
        rng: random.Random | None = None,
        reassembly: str = "cycle",
    ) -> None:
        if reassembly not in ("cycle", "random"):
            raise ValueError("reassembly must be 'cycle' or 'random'")
        self.script = script
        self.mode = reassembly
        self.rng = rng if rng is not None else random.Random()
        # (keyword, rule index) -> index of the next template to use
        self.cursors: dict[tuple[str, int], int] = {}
        self.memory_cursors: dict[str, int] = {}
        self.memory: deque[str] = deque(maxlen=script.memory_size)

    def greeting(self) -> str:
        return self.script.initial[0]

    def farewell(self) -> str:
        return self.script.final[0]

    def is_quit(self, text: str) -> bool:
        return text.strip().lower() in self.script.quit

    def respond(self, text: str) -> str:
        found = self._first_keyword_clause(text)
        if found is None:
            return self._postprocess(self._fallback())

        tokens, hits = found
        token_str = " ".join(tokens)
        for hit in sorted(hits, key=lambda h: (-h.rank, h.appearance)):
            result = self._apply_keyword(hit.name, token_str, set())
            if result is not None:
                return self._postprocess(result)
        return self._postprocess(self._fallback())

    def _first_keyword_clause(
        self, text: str
    ) -> tuple[list[str], list[_Hit]] | None:
        """Return the tokens and keyword hits of the first clause with a keyword."""
        for clause in _CLAUSE_SPLIT.split(text):
            tokens = self._tokenize(clause)
            if not tokens:
                continue
            hits = self._scan(tokens)
            if hits:
                return tokens, hits
        return None

    def _tokenize(self, clause: str) -> list[str]:
        words = (raw.strip(_STRIP_EDGES) for raw in clause.lower().split())
        return [self.script.pre.get(word, word) for word in words if word]

    def _scan(self, tokens: list[str]) -> list[_Hit]:
        seen: set[str] = set()
        hits: list[_Hit] = []
        for i, tok in enumerate(tokens):
            if tok == XNONE or tok in seen:
                continue
            kw = self.script.keywords.get(tok)
            if kw is not None:
                hits.append(_Hit(tok, kw.rank, i))
                seen.add(tok)
        return hits

    def _apply_keyword(
        self, name: str, token_str: str, visited: set[str]
    ) -> str | None:
        kw = self.script.keywords[name]
        for idx, rule in enumerate(kw.rules):
            match = rule.regex.fullmatch(token_str)
            if match is None:
                continue
            if rule.delegate is not None:
                if rule.delegate in visited:
                    continue
                delegated = self._apply_keyword(
                    rule.delegate, token_str, visited | {name}
                )
                if delegated is None:
                    continue
                self._push_memory(kw, match)
                return delegated
            template = self._choose((name, idx), rule.reassemb)
            self._push_memory(kw, match)
            return self._fill(template, match)
        return None

    def _choose(self, key: tuple[str, int], options: list[str]) -> str:
        if self.mode == "random":
            return self.rng.choice(options)
        i = self.cursors.get(key, 0)
        self.cursors[key] = (i + 1) % len(options)
        return options[i]

    def _push_memory(self, kw: Keyword, match: re.Match[str]) -> None:
        if not kw.memory:
            return
        if self.mode == "random":
            tmpl = self.rng.choice(kw.memory)
        else:
            i = self.memory_cursors.get(kw.keyword, 0)
            self.memory_cursors[kw.keyword] = (i + 1) % len(kw.memory)
            tmpl = kw.memory[i]
        self.memory.append(self._fill(tmpl, match))

    def _fill(self, template: str, match: re.Match[str]) -> str:
        def repl(m: re.Match[str]) -> str:
            return self._reflect_fragment(match.group(int(m.group(1))) or "")

        return _PLACEHOLDER.sub(repl, template)

    def _reflect_fragment(self, fragment: str) -> str:
        reflected = reflect(fragment.split(), self.script.reflections)
        return " ".join(reflected).strip(_STRIP_EDGES)

    def _fallback(self) -> str:
        if self.memory:
            return self.memory.popleft()
        # xnone's "*" rule matches anything, including "".
        result = self._apply_keyword(XNONE, "", set())
        assert result is not None
        return result

    @staticmethod
    def _postprocess(text: str) -> str:
        text = re.sub(r"\s+", " ", text).strip()
        text = re.sub(r"\s+([.,;:!?])", r"\1", text)
        if text:
            text = text[0].upper() + text[1:]
        return text
