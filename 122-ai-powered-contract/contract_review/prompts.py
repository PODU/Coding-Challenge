"""Prompts and JSON schemas for each LLM call.

Each builder returns ``(system, messages)``. The fixed instructions go in the
system prompt so they can be cached; the clause or question text goes in the
user message.
"""

from __future__ import annotations

from typing import Any

from .models import Chunk, Clause, ReviewDecision, ReviewReport

# The model is told to answer with exactly this when the clauses don't cover
# the question; parse_qa looks for it.
QA_NOT_ADDRESSED = "NOT_ADDRESSED_IN_CONTRACT"

PromptPair = tuple[str, list[dict[str, Any]]]

EXTRACT_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "clauses": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "number": {"type": "integer"},
                    "title": {"type": "string"},
                    "text": {"type": "string"},
                },
                "required": ["number", "title", "text"],
                "additionalProperties": False,
            },
        }
    },
    "required": ["clauses"],
    "additionalProperties": False,
}

ANALYSIS_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "clause_number": {"type": "integer"},
        "risk": {"type": "string", "enum": ["low", "medium", "high"]},
        "category": {"type": "string"},
        "explanation": {"type": "string"},
        "ambiguous_language": {"type": "array", "items": {"type": "string"}},
        "recommendation": {"type": "string"},
    },
    "required": [
        "clause_number",
        "risk",
        "category",
        "explanation",
        "ambiguous_language",
        "recommendation",
    ],
    "additionalProperties": False,
}

SUMMARY_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "text": {"type": "string"},
        "key_terms": {"type": "array", "items": {"type": "string"}},
        "outstanding_risks": {"type": "array", "items": {"type": "string"}},
    },
    "required": ["text", "key_terms", "outstanding_risks"],
    "additionalProperties": False,
}

QA_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "answer": {"type": "string"},
        "cited_clauses": {"type": "array", "items": {"type": "integer"}},
    },
    "required": ["answer", "cited_clauses"],
    "additionalProperties": False,
}

_EXTRACT_SYSTEM = (
    "You are a meticulous contract-analysis assistant. Split the supplied "
    "contract text into individual, logically distinct clauses. Preserve the "
    "original wording of each clause verbatim. Number the clauses sequentially "
    "starting at 1 and give each a short descriptive title. "
    "Respond ONLY with JSON matching this schema: "
    '{"clauses": [{"number": int, "title": string, "text": string}]}. '
    "Do not include any commentary outside the JSON."
)


def build_extract_prompt(chunk: Chunk) -> PromptPair:
    user = (
        "Extract the clauses from the following contract excerpt "
        f"(chunk {chunk.index}). Return JSON only.\n\n"
        "---BEGIN CONTRACT EXCERPT---\n"
        f"{chunk.text}\n"
        "---END CONTRACT EXCERPT---"
    )
    return _EXTRACT_SYSTEM, [{"role": "user", "content": user}]


_ANALYSE_SYSTEM = (
    "You are a contract-risk analyst. Analyse a single contract clause and "
    "assess its risk to the receiving party. Respond ONLY with JSON matching "
    "this schema: {\"clause_number\": int, \"risk\": one of "
    '"low"|"medium"|"high", "category": string (e.g. termination, liability, '
    'indemnity, payment, confidentiality), "explanation": plain-English string, '
    '"ambiguous_language": [string], "recommendation": string}. '
    "Flag any vague or ambiguous phrases in ambiguous_language. "
    "Do not include any commentary outside the JSON."
)


def build_analyse_prompt(clause: Clause) -> PromptPair:
    user = (
        f"Analyse clause number {clause.number} titled \"{clause.title}\". "
        "Return JSON only.\n\n"
        "---BEGIN CLAUSE---\n"
        f"{clause.text}\n"
        "---END CLAUSE---"
    )
    return _ANALYSE_SYSTEM, [{"role": "user", "content": user}]


_SUMMARY_SYSTEM = (
    "You are a senior contracts attorney writing a professional review summary. "
    "Fold the per-clause risk analyses and the reviewer's decisions/annotations "
    "into a clear executive summary. Respond ONLY with JSON matching this "
    'schema: {"text": string, "key_terms": [string], "outstanding_risks": '
    "[string]}. Reflect any reviewer rejections or annotations in the "
    "outstanding_risks. Do not include commentary outside the JSON."
)


def _format_decisions(decisions: list[ReviewDecision]) -> str:
    if not decisions:
        return "(no reviewer decisions recorded)"
    lines = []
    for d in decisions:
        note = f" — note: {d.note}" if d.note else ""
        lines.append(f"Clause {d.clause_number}: {d.decision}{note}")
    return "\n".join(lines)


def build_summary_prompt(report: ReviewReport, decisions: list[ReviewDecision]) -> PromptPair:
    analysis_lines = []
    by_number = {c.number: c for c in report.clauses}
    for a in report.analyses:
        clause = by_number.get(a.clause_number)
        title = clause.title if clause else f"Clause {a.clause_number}"
        analysis_lines.append(
            f"Clause {a.clause_number} ({title}) [{a.risk} risk / {a.category}]: "
            f"{a.explanation} Recommendation: {a.recommendation}"
        )
    user = (
        "Overall risk: "
        f"{report.overall_risk} (high={report.high_risk_count}, "
        f"medium={report.medium_risk_count}).\n\n"
        "Per-clause analyses:\n"
        + ("\n".join(analysis_lines) or "(none)")
        + "\n\nReviewer decisions:\n"
        + _format_decisions(decisions)
        + "\n\nWrite the review summary as JSON only."
    )
    return _SUMMARY_SYSTEM, [{"role": "user", "content": user}]


_QA_SYSTEM = (
    "You are a contract Q&A assistant. Answer the user's question using ONLY "
    "the supplied contract clauses. Cite the clause numbers you relied on. "
    "If the answer is not contained in the supplied clauses, set the answer "
    f"field to exactly \"{QA_NOT_ADDRESSED}\" and return an empty cited_clauses "
    "list. Never fabricate terms that are not present. Respond ONLY with JSON "
    'matching: {"answer": string, "cited_clauses": [int]}.'
)


def build_qa_prompt(question: str, relevant_clauses: list[Clause]) -> PromptPair:
    if relevant_clauses:
        clause_block = "\n\n".join(
            f"[Clause {c.number} — {c.title}]\n{c.text}" for c in relevant_clauses
        )
    else:
        clause_block = "(no clauses supplied)"
    user = (
        "Question: "
        f"{question}\n\n"
        "---BEGIN CONTRACT CLAUSES---\n"
        f"{clause_block}\n"
        "---END CONTRACT CLAUSES---\n\n"
        "Answer as JSON only."
    )
    return _QA_SYSTEM, [{"role": "user", "content": user}]
