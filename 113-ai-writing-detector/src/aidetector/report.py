"""Text and JSON rendering of a :class:`Result`."""

from __future__ import annotations

import json

from .judge import JudgeOpinion
from .scorer import Result


def to_dict(result: Result, judge: JudgeOpinion | None = None) -> dict:
    """The ``--format json`` representation of a result."""
    data: dict = {
        "score": result.score,
        "label": result.label,
        "thresholds": {
            "human": result.threshold_human,
            "ai": result.threshold_ai,
        },
        "stats": result.stats,
        "detections": [
            {
                "name": d.name,
                "step": d.step,
                "raw_score": d.raw_score,
                "cap": d.cap,
                "capped_score": d.capped_score,
                "findings": [
                    {
                        "pattern": f.pattern,
                        "evidence": f.evidence,
                        "detail": f.detail,
                        "weight": f.weight,
                    }
                    for f in d.findings
                ],
            }
            for d in result.detections
        ],
    }
    if judge is not None:
        data["judge"] = {
            "source": judge.source,
            "probability_ai": judge.probability_ai,
            "rationale": judge.rationale,
        }
    return data


def render_json(result: Result, judge: JudgeOpinion | None = None) -> str:
    return json.dumps(to_dict(result, judge), indent=2)


def render_text(
    result: Result,
    *,
    explain: bool = False,
    judge: JudgeOpinion | None = None,
) -> str:
    stats = result.stats
    lines: list[str] = []
    lines.append("AI Writing Detector — Report")
    lines.append("=" * 28)
    lines.append(
        f"Characters: {stats['characters']:,}    "
        f"Words: {stats['words']:,}    "
        f"Sentences: {stats['sentences']:,}"
    )
    lines.append("")
    lines.append(
        f"AI-likelihood score: {result.score} / 100   ->  {result.label.upper()}"
    )
    lines.append("")
    lines.append("Breakdown:")
    for d in result.detections:
        lines.append(
            f"  [{d.step}] {d.name:.<34} {d.capped_score:g} / {d.cap:g}"
        )
        if explain:
            shown = [f for f in d.findings if f.weight > 0 or f.pattern == "low_confidence"]
            if d.step == 6:
                shown = d.findings  # show every linguistic measurement, even at 0 points
            for f in shown:
                lines.append(f"        - {f.detail}")
    if judge is not None:
        lines.append("")
        lines.append(
            f"Optional judge ({judge.source}): "
            f"P(AI) = {judge.probability_ai:.0%} — {judge.rationale}"
        )
    return "\n".join(lines)
