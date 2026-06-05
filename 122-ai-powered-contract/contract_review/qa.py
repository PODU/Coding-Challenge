"""Answering questions about a contract from its clauses.

Clauses are ranked by plain word overlap with the question (no embeddings) and
only the best few are sent to the model, which is told to reply with a fixed
marker when they don't answer the question.
"""

from __future__ import annotations

import re
from typing import Iterable, Union

from .llm import LLMClient
from .models import Answer, Clause, ReviewReport
from .parsing import parse_qa
from .prompts import QA_SCHEMA, build_qa_prompt

_WORD = re.compile(r"[a-z0-9]+")
_STOPWORDS = {
    "the", "a", "an", "of", "to", "in", "on", "for", "and", "or", "is", "are",
    "be", "by", "with", "this", "that", "what", "which", "how", "does", "do",
    "it", "as", "at", "from", "will", "shall", "can", "may", "any", "all",
    "if", "no", "not", "there", "their", "has", "have", "was", "were", "been",
}

ClauseSource = Union[ReviewReport, Iterable[Clause]]


def _clauses_from(source: ClauseSource) -> list[Clause]:
    if isinstance(source, ReviewReport):
        return list(source.clauses)
    return list(source)


def _terms(text: str) -> list[str]:
    return [w for w in _WORD.findall(text.lower()) if len(w) > 2 and w not in _STOPWORDS]


def select_relevant(question: str, clauses: list[Clause], top_k: int = 4) -> list[Clause]:
    """Return the ``top_k`` clauses sharing the most words with the question.

    If nothing overlaps, the first clauses are returned so the model still has
    some context.
    """

    if not clauses:
        return []
    question_terms = set(_terms(question))
    scored: list[tuple[int, int, Clause]] = []
    for clause in clauses:
        overlap = sum(1 for t in _terms(f"{clause.title} {clause.text}") if t in question_terms)
        scored.append((overlap, clause.number, clause))

    # Ties go to the lower clause number so results are stable.
    scored.sort(key=lambda s: (-s[0], s[1]))
    return [c for _, _, c in scored[: max(1, top_k)]]


def answer_question(
    question: str,
    source: ClauseSource,
    llm: LLMClient,
    top_k: int = 4,
) -> Answer:
    relevant = select_relevant(question, _clauses_from(source), top_k=top_k)
    system, messages = build_qa_prompt(question, relevant)
    raw = llm.complete(system, messages, output_schema=QA_SCHEMA, intent="qa")
    return parse_qa(raw, question)
