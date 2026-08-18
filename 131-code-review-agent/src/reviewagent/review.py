"""Parsing the model's reviews, merging them, and formatting the comment."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field

SEVERITIES = ("CRITICAL", "MAJOR", "MINOR")
_SEVERITY_RANK = {s: i for i, s in enumerate(SEVERITIES)}


@dataclass
class Issue:
    severity: str
    file: str
    line: int | None
    title: str
    detail: str

    @property
    def rank(self) -> int:
        return _SEVERITY_RANK.get(self.severity, len(SEVERITIES))


@dataclass
class Review:
    summary: str = ""
    issues: list[Issue] = field(default_factory=list)
    suggestions: list[str] = field(default_factory=list)
    verdict: str = "COMMENT"


class ParseError(ValueError):
    pass


def _extract_json(text: str) -> dict:
    """Find the JSON object in the model's reply.

    Models wrap it in prose or code fences despite being told not to.
    """
    text = text.strip()
    fence = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", text, re.DOTALL)
    if fence:
        text = fence.group(1)
    start = text.find("{")
    end = text.rfind("}")
    if start == -1 or end == -1 or end <= start:
        raise ParseError(f"no JSON object in model output: {text[:120]!r}")
    try:
        return json.loads(text[start : end + 1])
    except json.JSONDecodeError as exc:
        raise ParseError(f"invalid JSON from model: {exc}") from exc


def parse_review(text: str, *, default_file: str = "") -> Review:
    data = _extract_json(text)
    issues = []
    for raw in data.get("issues", []) or []:
        severity = str(raw.get("severity", "MINOR")).upper()
        if severity not in SEVERITIES:
            severity = "MINOR"
        line = raw.get("line")
        issues.append(
            Issue(
                severity=severity,
                file=str(raw.get("file") or default_file),
                line=int(line) if isinstance(line, (int, float)) else None,
                title=str(raw.get("title", "")).strip(),
                detail=str(raw.get("detail", "")).strip(),
            )
        )
    return Review(
        summary=str(data.get("summary", "")).strip(),
        issues=issues,
        suggestions=[str(s) for s in data.get("suggestions", []) or []],
        verdict=str(data.get("verdict", "COMMENT")).upper(),
    )


def merge_reviews(reviews: list[Review]) -> Review:
    """Combine the per-file reviews, most severe issues first."""
    merged = Review()
    merged.summary = " ".join(r.summary for r in reviews if r.summary)
    for r in reviews:
        merged.issues.extend(r.issues)
        merged.suggestions.extend(r.suggestions)
    merged.issues.sort(key=lambda i: (i.rank, i.file, i.line or 0))
    if any(i.severity in ("CRITICAL", "MAJOR") for i in merged.issues):
        merged.verdict = "REQUEST_CHANGES"
    elif merged.issues:
        merged.verdict = "COMMENT"
    else:
        merged.verdict = "APPROVE"
    return merged


_SEVERITY_EMOJI = {"CRITICAL": "🔴", "MAJOR": "🟠", "MINOR": "🟡"}


def format_comment(
    review: Review,
    *,
    skipped: list[tuple[str, str]] | None = None,
    reviewed_files: list[str] | None = None,
) -> str:
    """Render the review as a Markdown comment."""
    lines = ["## 🤖 Automated Code Review", ""]
    if review.summary:
        lines += [review.summary, ""]
    if review.issues:
        lines.append("### Issues found")
        for issue in review.issues:
            loc = issue.file + (f":{issue.line}" if issue.line else "")
            emoji = _SEVERITY_EMOJI.get(issue.severity, "⚪")
            lines.append(f"- {emoji} **{issue.severity}** `{loc}` — {issue.title}")
            if issue.detail:
                lines.append(f"  - {issue.detail}")
        lines.append("")
    if review.suggestions:
        lines.append("### Suggestions")
        lines += [f"- {s}" for s in review.suggestions]
        lines.append("")
    if reviewed_files:
        lines.append(f"**Reviewed:** {len(reviewed_files)} file(s)")
    if skipped:
        lines.append("")
        lines.append("<details><summary>Skipped files</summary>")
        lines.append("")
        for name, reason in skipped:
            lines.append(f"- `{name}` — {reason}")
        lines += ["", "</details>"]
    lines += ["", f"**Verdict:** {review.verdict}"]
    return "\n".join(lines)


def should_post(review: Review) -> bool:
    """Only post when there are issues; a "looks good" comment is just noise."""
    return bool(review.issues)
