"""Pydantic models passed between the pipeline stages.

Validating model output into these as soon as it arrives means bad output
fails at the point it was produced.
"""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Literal, Optional

from pydantic import BaseModel, Field, field_validator

RiskLevel = Literal["low", "medium", "high"]
DecisionKind = Literal["approve", "reject", "annotate"]
SessionStatus = Literal["pending_review", "resumed", "complete"]

_RISK_ORDER: dict[str, int] = {"low": 0, "medium": 1, "high": 2}


def risk_rank(risk: str) -> int:
    """Sort key for risk levels. Unknown values rank as medium."""

    return _RISK_ORDER.get(risk, _RISK_ORDER["medium"])


def _utcnow() -> datetime:
    return datetime.now(timezone.utc)


class ContractDocument(BaseModel):
    text: str
    source_path: str
    page_count: Optional[int] = None
    meta: dict = Field(default_factory=dict)


class Chunk(BaseModel):
    index: int
    text: str
    start_char: int
    end_char: int
    est_tokens: int


class Clause(BaseModel):
    number: int
    title: str
    text: str
    source_chunk: int = 0


class ClauseAnalysis(BaseModel):
    clause_number: int
    risk: RiskLevel
    category: str = "general"
    explanation: str = ""
    ambiguous_language: list[str] = Field(default_factory=list)
    recommendation: str = ""

    @field_validator("ambiguous_language", mode="before")
    @classmethod
    def _coerce_ambiguous(cls, value: object) -> list[str]:
        if value is None:
            return []
        if isinstance(value, str):
            return [value] if value.strip() else []
        if isinstance(value, (list, tuple)):
            return [str(v) for v in value if str(v).strip()]
        return [str(value)]


class ReviewDecision(BaseModel):
    clause_number: int
    decision: DecisionKind
    note: Optional[str] = None


class ReviewReport(BaseModel):
    clauses: list[Clause] = Field(default_factory=list)
    analyses: list[ClauseAnalysis] = Field(default_factory=list)
    overall_risk: RiskLevel = "low"
    high_risk_count: int = 0
    medium_risk_count: int = 0
    low_risk_count: int = 0
    notes: list[str] = Field(default_factory=list)


class ReviewSession(BaseModel):
    id: str
    status: SessionStatus = "pending_review"
    report: ReviewReport
    decisions: list[ReviewDecision] = Field(default_factory=list)
    contract_text: str = ""
    source_path: str = ""
    created_at: datetime = Field(default_factory=_utcnow)
    updated_at: datetime = Field(default_factory=_utcnow)
    summary: Optional["Summary"] = None


class Summary(BaseModel):
    text: str
    key_terms: list[str] = Field(default_factory=list)
    outstanding_risks: list[str] = Field(default_factory=list)


class Answer(BaseModel):
    question: str
    answer: str
    grounded: bool = True
    cited_clauses: list[int] = Field(default_factory=list)


# ReviewSession refers to Summary before it is defined.
ReviewSession.model_rebuild()
