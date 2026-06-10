"""Protocols for the LLM, the embedder and the store."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Protocol, runtime_checkable

from .domain import Contradiction, Hit, Operation, WikiPage


@dataclass
class Msg:
    role: str  # "user" | "assistant"
    content: str


@dataclass
class LLMResult:
    text: str
    raw: dict = field(default_factory=dict)


@runtime_checkable
class LLMProvider(Protocol):
    def complete(
        self, system: str, messages: list[Msg], *, schema: dict | None = None
    ) -> LLMResult: ...


@runtime_checkable
class Embedder(Protocol):
    dim: int

    def embed(self, texts: list[str]) -> list[list[float]]: ...


@runtime_checkable
class Store(Protocol):
    # lifecycle
    def init_project(self, project: str) -> None: ...
    def list_projects(self) -> list[str]: ...
    def delete_project(self, project: str) -> None: ...

    # pages + embeddings
    def upsert_page(self, project: str, page: WikiPage, embedding: list[float]) -> None: ...
    def get_page(self, project: str, page_id: str) -> WikiPage | None: ...
    def all_pages(self, project: str) -> list[WikiPage]: ...
    def delete_page(self, project: str, page_id: str) -> None: ...

    # hybrid search
    def vector_search(self, project: str, query_vec: list[float], k: int) -> list[Hit]: ...
    def fulltext_search(self, project: str, query: str, k: int) -> list[Hit]: ...
    def hybrid_search(
        self, project: str, query: str, query_vec: list[float], k: int
    ) -> list[Hit]: ...

    # operation log
    def append_log(self, project: str, op: Operation) -> Operation: ...
    def read_log(self, project: str, since: datetime | None = None) -> list[Operation]: ...

    # sources, used to skip re-ingesting the same content
    def get_source_by_sha(self, project: str, sha: str) -> str | None: ...
    def add_source(self, project: str, source_id: str, title: str, sha: str, path: str | None) -> None: ...

    # contradictions
    def add_contradiction(self, project: str, c: Contradiction) -> None: ...
    def list_contradictions(self, project: str) -> list[Contradiction]: ...
