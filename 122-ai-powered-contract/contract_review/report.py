"""Roll clause analyses up into a ReviewReport."""

from __future__ import annotations

from .models import Clause, ClauseAnalysis, ReviewReport, risk_rank


def build_report(clauses: list[Clause], analyses: list[ClauseAnalysis]) -> ReviewReport:
    """The overall risk is the worst risk of any single clause."""

    high = sum(1 for a in analyses if a.risk == "high")
    medium = sum(1 for a in analyses if a.risk == "medium")
    low = sum(1 for a in analyses if a.risk == "low")

    if analyses:
        overall = max((a.risk for a in analyses), key=risk_rank)
    else:
        overall = "low"

    notes: list[str] = []
    if not clauses:
        notes.append("No clauses identified in this contract.")

    return ReviewReport(
        clauses=clauses,
        analyses=analyses,
        overall_risk=overall,
        high_risk_count=high,
        medium_risk_count=medium,
        low_risk_count=low,
        notes=notes,
    )
