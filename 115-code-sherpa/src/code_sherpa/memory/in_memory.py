"""In-memory MemoryStore. Recall ranks by how many words the query shares
with an episode's question or a fact's key and value."""

from __future__ import annotations

import re
import time
from collections import defaultdict

from .base import Episode, MemoryStore, SemanticFact

_TOKEN_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")


def _tokens(text: str) -> set[str]:
    return {t.lower() for t in _TOKEN_RE.findall(text)}


def _overlap(a: set[str], b: set[str]) -> int:
    return len(a & b)


class InMemoryMemoryStore(MemoryStore):
    def __init__(self) -> None:
        self._episodes: dict[str, list[Episode]] = defaultdict(list)
        self._facts: dict[str, dict[str, SemanticFact]] = defaultdict(dict)

    def add_episode(self, episode: Episode) -> None:
        self._episodes[episode.project].append(episode)

    def recent_episodes(self, project: str, limit: int = 5) -> list[Episode]:
        eps = self._episodes.get(project, [])
        return list(reversed(eps[-limit:])) if limit > 0 else []

    def search_episodes(self, project: str, query: str, limit: int = 3) -> list[Episode]:
        q = _tokens(query)
        if not q:
            return []
        scored = [
            (_overlap(q, _tokens(ep.question)), -i, ep)
            for i, ep in enumerate(self._episodes.get(project, []))
        ]
        scored = [s for s in scored if s[0] > 0]
        scored.sort(key=lambda x: (-x[0], x[1]))
        return [ep for _, _, ep in scored[:limit]]

    def add_fact(self, fact: SemanticFact) -> None:
        self._facts[fact.project][fact.key] = fact

    def facts(self, project: str) -> list[SemanticFact]:
        return list(self._facts.get(project, {}).values())

    def relevant_facts(
        self, project: str, query: str, limit: int = 5
    ) -> list[SemanticFact]:
        q = _tokens(query)
        if not q:
            return []
        scored = []
        for fact in self._facts.get(project, {}).values():
            score = _overlap(q, _tokens(fact.key) | _tokens(fact.value))
            if score > 0:
                scored.append((score, fact))
        scored.sort(key=lambda x: (-x[0], x[1].key))
        return [f for _, f in scored[:limit]]


def make_episode(
    project: str, question: str, answer: str, citations: list[str]
) -> Episode:
    return Episode(
        project=project,
        question=question,
        answer=answer,
        citations=citations,
        timestamp=time.time(),
    )
