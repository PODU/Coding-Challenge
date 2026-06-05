"""Risk analysis for each clause, run in parallel.

If the model's answer for a clause can't be parsed, that clause is marked high
risk for manual review instead of failing the whole run.
"""

from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
from typing import Optional

from .config import Config
from .errors import LLMOutputError
from .llm import LLMClient
from .models import Clause, ClauseAnalysis
from .obs import RunRecorder
from .parsing import parse_analysis
from .prompts import ANALYSIS_SCHEMA, build_analyse_prompt


def _placeholder(clause: Clause, reason: str) -> ClauseAnalysis:
    return ClauseAnalysis(
        clause_number=clause.number,
        risk="high",
        category="unanalysed",
        explanation=f"Could not analyse this clause automatically: {reason}",
        ambiguous_language=[],
        recommendation="Manual review required.",
    )


def analyse_clause(clause: Clause, llm: LLMClient) -> ClauseAnalysis:
    system, messages = build_analyse_prompt(clause)
    try:
        raw = llm.complete(
            system, messages, output_schema=ANALYSIS_SCHEMA, intent="analyse"
        )
        analysis = parse_analysis(raw, clause_number=clause.number)
        # Don't trust the model to echo the right number back.
        if analysis.clause_number != clause.number:
            analysis = analysis.model_copy(update={"clause_number": clause.number})
        return analysis
    except LLMOutputError as exc:
        return _placeholder(clause, str(exc))


def analyse_all(
    clauses: list[Clause],
    llm: LLMClient,
    config: Optional[Config] = None,
    recorder: Optional[RunRecorder] = None,
) -> list[ClauseAnalysis]:
    """Analyse every clause, returning results in the same order as ``clauses``."""

    config = config or Config.from_env()
    if not clauses:
        return []

    workers = max(1, min(config.analysis_workers, len(clauses)))
    results: dict[int, ClauseAnalysis] = {}

    if workers == 1:
        for clause in clauses:
            results[clause.number] = analyse_clause(clause, llm)
    else:
        with ThreadPoolExecutor(max_workers=workers) as pool:
            futures = {
                pool.submit(analyse_clause, clause, llm): clause for clause in clauses
            }
            for future, clause in futures.items():
                results[clause.number] = future.result()

    ordered = [results[c.number] for c in clauses]
    if recorder is not None:
        high = sum(1 for a in ordered if a.risk == "high")
        recorder.emit("analyse_all", clauses=len(ordered), high_risk=high, workers=workers)
    return ordered
