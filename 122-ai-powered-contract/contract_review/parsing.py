"""Turning model replies into validated models.

Models don't always return bare JSON, so this strips code fences, digs the JSON
out of surrounding prose, and maps risk words like "critical" onto
low/medium/high. Anything that still can't be parsed raises ``LLMOutputError``.
"""

from __future__ import annotations

import json
import re
from typing import Any

from .errors import LLMOutputError
from .models import Answer, Clause, ClauseAnalysis, Summary
from .prompts import QA_NOT_ADDRESSED

_FENCE = re.compile(r"```(?:json)?\s*(.*?)\s*```", re.DOTALL | re.IGNORECASE)

_RISK_ALIASES = {
    "critical": "high",
    "severe": "high",
    "serious": "high",
    "elevated": "high",
    "moderate": "medium",
    "med": "medium",
    "minor": "low",
    "minimal": "low",
    "none": "low",
    "negligible": "low",
}


def _strip_fences(text: str) -> str:
    match = _FENCE.search(text)
    if match:
        return match.group(1).strip()
    return text.strip()


def _locate_span(text: str) -> str:
    """Return the text from the first { or [ to the last matching closer."""

    starts = [i for i in (text.find("{"), text.find("[")) if i != -1]
    if not starts:
        raise LLMOutputError("No JSON object or array found in model output.")
    start = min(starts)
    opener = text[start]
    closer = "}" if opener == "{" else "]"
    end = text.rfind(closer)
    if end == -1 or end < start:
        raise LLMOutputError("Unbalanced JSON in model output.")
    return text[start : end + 1]


def loads(text: str) -> Any:
    """Parse JSON that may be fenced or surrounded by prose."""

    if text is None:
        raise LLMOutputError("Empty model output.")
    candidate = _strip_fences(text)
    try:
        return json.loads(candidate)
    except json.JSONDecodeError:
        pass
    try:
        return json.loads(_locate_span(candidate))
    except (json.JSONDecodeError, LLMOutputError) as exc:
        raise LLMOutputError(f"Could not parse JSON from model output: {exc}") from exc


def normalise_risk(value: Any) -> str:
    """Map a risk value onto low/medium/high, defaulting to medium."""

    if not isinstance(value, str):
        return "medium"
    key = value.strip().lower()
    if key in {"low", "medium", "high"}:
        return key
    return _RISK_ALIASES.get(key, "medium")


def parse_clauses(text: str) -> list[Clause]:
    data = loads(text)
    if isinstance(data, dict):
        items = data.get("clauses", data.get("items"))
        if items is None and "number" in data:  # a single clause object
            items = [data]
    elif isinstance(data, list):
        items = data
    else:
        items = None
    if not isinstance(items, list):
        raise LLMOutputError("Expected a list of clauses in model output.")

    clauses: list[Clause] = []
    for raw in items:
        if not isinstance(raw, dict):
            continue
        try:
            clauses.append(
                Clause(
                    number=int(raw.get("number", len(clauses) + 1)),
                    title=str(raw.get("title", "")).strip() or "Untitled clause",
                    text=str(raw.get("text", "")).strip(),
                    source_chunk=int(raw.get("source_chunk", 0)),
                )
            )
        except (ValueError, TypeError) as exc:
            raise LLMOutputError(f"Invalid clause object: {exc}") from exc
    return clauses


def parse_analysis(text: str, clause_number: int | None = None) -> ClauseAnalysis:
    data = loads(text)
    if isinstance(data, list):
        data = data[0] if data else {}
    if not isinstance(data, dict):
        raise LLMOutputError("Expected a JSON object for clause analysis.")
    data = dict(data)
    data["risk"] = normalise_risk(data.get("risk"))
    if clause_number is not None:
        data.setdefault("clause_number", clause_number)
    try:
        return ClauseAnalysis.model_validate(data)
    except Exception as exc:  # pydantic ValidationError
        raise LLMOutputError(f"Invalid clause analysis: {exc}") from exc


def parse_summary(text: str) -> Summary:
    data = loads(text)
    if not isinstance(data, dict):
        raise LLMOutputError("Expected a JSON object for the summary.")
    try:
        return Summary.model_validate(data)
    except Exception as exc:
        raise LLMOutputError(f"Invalid summary: {exc}") from exc


def parse_qa(text: str, question: str) -> Answer:
    """Parse a Q&A reply. An empty answer or the not-addressed marker gives
    an ungrounded Answer."""

    data = loads(text)
    if not isinstance(data, dict):
        raise LLMOutputError("Expected a JSON object for the answer.")
    answer_text = str(data.get("answer", "")).strip()
    cited = data.get("cited_clauses", []) or []
    if not isinstance(cited, list):
        cited = []
    cited_clauses = [int(c) for c in cited if str(c).strip().lstrip("-").isdigit()]

    if QA_NOT_ADDRESSED in answer_text or not answer_text:
        return Answer(
            question=question,
            answer="This contract does not address that.",
            grounded=False,
            cited_clauses=[],
        )
    return Answer(
        question=question,
        answer=answer_text,
        grounded=True,
        cited_clauses=cited_clauses,
    )
