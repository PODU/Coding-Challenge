"""Memory interface: past Q&A turns (episodes) and user-supplied facts."""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass, field


@dataclass(frozen=True)
class Episode:
    """One past Q&A turn for a project."""

    project: str
    question: str
    answer: str
    citations: list[str]
    timestamp: float


@dataclass(frozen=True)
class SemanticFact:
    """A developer-provided fact / glossary entry for a project."""

    project: str
    key: str
    value: str


class MemoryStore(ABC):
    """Stores episodes and facts per project."""

    @abstractmethod
    def add_episode(self, episode: Episode) -> None: ...

    @abstractmethod
    def recent_episodes(self, project: str, limit: int = 5) -> list[Episode]: ...

    @abstractmethod
    def search_episodes(self, project: str, query: str, limit: int = 3) -> list[Episode]:
        """Past turns whose questions share words with ``query``."""

    @abstractmethod
    def add_fact(self, fact: SemanticFact) -> None: ...

    @abstractmethod
    def facts(self, project: str) -> list[SemanticFact]: ...

    @abstractmethod
    def relevant_facts(self, project: str, query: str, limit: int = 5) -> list[SemanticFact]:
        ...
