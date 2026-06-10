"""Plain data types shared by the rest of the package."""

from __future__ import annotations

from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone

from .text import sha256_hex

PageType = str  # "summary" | "entity" | "topic" | "answer"
PageStatus = str  # "active" | "stale" | "draft"


def utcnow_iso() -> str:
    """Current UTC time as ISO-8601, to the second."""
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


@dataclass
class Frontmatter:
    id: str
    type: PageType
    title: str
    project: str
    created_at: str = field(default_factory=utcnow_iso)
    updated_at: str = field(default_factory=utcnow_iso)
    sources: list[str] = field(default_factory=list)
    links: list[str] = field(default_factory=list)
    tags: list[str] = field(default_factory=list)
    status: PageStatus = "active"
    content_sha256: str = ""

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, d: dict) -> "Frontmatter":
        known = set(cls.__dataclass_fields__)  # type: ignore[attr-defined]
        return cls(**{k: v for k, v in d.items() if k in known})


@dataclass
class WikiPage:
    frontmatter: Frontmatter
    body: str

    @property
    def id(self) -> str:
        return self.frontmatter.id

    @property
    def type(self) -> str:
        return self.frontmatter.type

    def recompute_sha(self) -> str:
        self.frontmatter.content_sha256 = sha256_hex(self.body.strip())
        return self.frontmatter.content_sha256

    def embedding_text(self) -> str:
        return f"{self.frontmatter.title}\n{self.body}"


@dataclass
class Source:
    id: str
    title: str
    sha256: str
    path: str | None = None
    ingested_at: str = field(default_factory=utcnow_iso)


@dataclass
class Hit:
    page_id: str
    score: float
    via: str  # "vector" | "fulltext" | "both"


@dataclass
class Operation:
    op: str
    page_id: str | None = None
    details: dict = field(default_factory=dict)
    ts: str = field(default_factory=utcnow_iso)
    seq: int = 0

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, d: dict) -> "Operation":
        return cls(
            op=d["op"],
            page_id=d.get("page_id"),
            details=d.get("details", {}),
            ts=d.get("ts", utcnow_iso()),
            seq=d.get("seq", 0),
        )


@dataclass
class Contradiction:
    id: str
    entity: str
    page_a: str
    claim_a: str
    page_b: str
    claim_b: str
    detected_at: str = field(default_factory=utcnow_iso)

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class Finding:
    kind: str  # orphan | missing_page | broken_link | stale | data_gap | contradiction
    page_id: str | None
    message: str
    suggestion: str = ""
    severity: str = "warning"  # info | warning | error
    target: str | None = None  # e.g. the missing/broken link target

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class IngestResult:
    source_ids: list[str] = field(default_factory=list)
    pages_created: list[str] = field(default_factory=list)
    pages_updated: list[str] = field(default_factory=list)
    contradictions: list[Contradiction] = field(default_factory=list)
    ops: int = 0
    skipped: list[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        d = asdict(self)
        d["contradictions"] = [c.to_dict() for c in self.contradictions]
        return d


@dataclass
class Answer:
    project: str
    question: str
    answer: str
    citations: list[dict] = field(default_factory=list)
    retrieved: list[dict] = field(default_factory=list)
    saved_as: str | None = None

    def to_dict(self) -> dict:
        return asdict(self)


class EmbeddingDimMismatch(ValueError):
    """An embedding's length doesn't match what the store holds."""


class EmptyDocumentError(ValueError):
    """The document to ingest is empty or only whitespace."""
