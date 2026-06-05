from __future__ import annotations

from contract_review import build_report
from contract_review.models import Clause, ClauseAnalysis


def _clauses(n):
    return [Clause(number=i, title=f"C{i}", text=f"t{i}") for i in range(1, n + 1)]


def _analysis(n, risk):
    return ClauseAnalysis(clause_number=n, risk=risk, category="x", explanation="e", recommendation="r")


def test_overall_risk_is_max_severity():
    clauses = _clauses(3)
    analyses = [_analysis(1, "low"), _analysis(2, "medium"), _analysis(3, "high")]
    report = build_report(clauses, analyses)
    assert report.overall_risk == "high"
    assert report.high_risk_count == 1
    assert report.medium_risk_count == 1
    assert report.low_risk_count == 1


def test_overall_risk_medium_when_no_high():
    clauses = _clauses(2)
    analyses = [_analysis(1, "low"), _analysis(2, "medium")]
    report = build_report(clauses, analyses)
    assert report.overall_risk == "medium"
    assert report.high_risk_count == 0


def test_zero_clause_report_flagged():
    report = build_report([], [])
    assert report.overall_risk == "low"
    assert report.notes
    assert "No clauses" in report.notes[0]
